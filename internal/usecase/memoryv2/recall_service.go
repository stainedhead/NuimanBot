package memoryv2

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/tracing"
)

// MemoryRecallService handles memory retrieval and ranking
type MemoryRecallService struct {
	cellRepo  memoryv2.MemoryCellRepository
	sceneRepo memoryv2.MemorySceneRepository
	config    RecallConfig
}

// NewMemoryRecallService creates a new memory recall service
func NewMemoryRecallService(
	cellRepo memoryv2.MemoryCellRepository,
	sceneRepo memoryv2.MemorySceneRepository,
	config RecallConfig,
) *MemoryRecallService {
	return &MemoryRecallService{
		cellRepo:  cellRepo,
		sceneRepo: sceneRepo,
		config:    config,
	}
}

// RecallMemory retrieves relevant memory for a query
func (s *MemoryRecallService) RecallMemory(ctx context.Context, request RecallRequest) (*RecallResponse, error) {
	ctx, _ = tracing.StartSpan(ctx, "memory.recall")
	defer tracing.EndSpan(ctx)
	tracing.AddAttribute(ctx, "conversation_id", request.ConversationID)
	tracing.AddAttribute(ctx, "query", request.Query)
	tracing.AddAttribute(ctx, "max_tokens", request.MaxTokens)

	startTime := time.Now()

	slog.Info("Starting memory recall",
		"conversation_id", request.ConversationID,
		"query", request.Query,
		"max_tokens", request.MaxTokens,
	)

	// Step 1: Try FTS search first
	ftsStart := time.Now()
	ftsCtx, _ := tracing.StartSpan(ctx, "memory.fts_search")
	tracing.AddAttribute(ftsCtx, "query", request.Query)
	cells, err := s.cellRepo.SearchFTS(ftsCtx, request.Query, s.config.FTSResultLimit)
	ftsDuration := time.Since(ftsStart)
	metrics.MemoryFTSQueryDuration.Observe(ftsDuration.Seconds())
	if err != nil {
		tracing.RecordError(ftsCtx, err.Error())
		tracing.EndSpan(ftsCtx)
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryRecallTotal.WithLabelValues("error", "fts").Inc()
		alertRecallFailed(ctx, request.ConversationID, request.Query, err)
		return nil, fmt.Errorf("FTS search failed: %w", err)
	}
	if isSlowFTSQuery(ftsDuration) {
		logSlowFTSQuery(ctx, request.Query, ftsDuration)
	}
	tracing.AddAttribute(ftsCtx, "result_count", len(cells))
	tracing.EndSpan(ftsCtx)

	ftsMatchCount := len(cells)
	fallbackUsed := false

	// Step 2: Fallback to salience if FTS yields too few results
	if len(cells) == 0 {
		fallbackUsed = true
		slog.Info("FTS returned no results, using salience fallback",
			"conversation_id", request.ConversationID,
			"query", request.Query,
		)
		tracing.AddAttribute(ctx, "fallback_used", true)
		cells, err = s.cellRepo.GetHighSalience(ctx, request.ConversationID, s.config.SalienceThreshold, s.config.FallbackCellLimit)
		if err != nil {
			alertRecallFailed(ctx, request.ConversationID, request.Query, err)
			tracing.RecordError(ctx, err.Error())
			metrics.MemoryRecallTotal.WithLabelValues("error", "fallback").Inc()
			return nil, fmt.Errorf("salience fallback failed: %w", err)
		}
	}

	// Step 3: Get scenes for cells first (to know scene token counts)
	allScenes, _ := s.getScenes(ctx, cells)

	// Step 4: Apply token budget (including scene tokens)
	cells, scenes, totalTokens := s.applyBudgetWithScenes(cells, allScenes, request.MaxTokens)

	// Calculate retrieval time
	retrievalTimeMs := time.Since(startTime).Milliseconds()

	response := &RecallResponse{
		Cells:           cells,
		Scenes:          scenes,
		TotalTokens:     totalTokens,
		FTSMatchCount:   ftsMatchCount,
		FallbackUsed:    fallbackUsed,
		RetrievalTimeMs: retrievalTimeMs,
	}

	tracing.AddAttribute(ctx, "cell_count", len(response.Cells))
	tracing.AddAttribute(ctx, "scene_count", len(response.Scenes))
	tracing.AddAttribute(ctx, "total_tokens", response.TotalTokens)
	tracing.AddAttribute(ctx, "fts_match_count", response.FTSMatchCount)
	tracing.AddAttribute(ctx, "retrieval_time_ms", response.RetrievalTimeMs)

	// Record recall metrics
	queryType := "fts"
	if response.FallbackUsed {
		queryType = "fallback"
	}
	metrics.MemoryRecallTotal.WithLabelValues("success", queryType).Inc()
	metrics.MemoryRecallDuration.Observe(time.Since(startTime).Seconds())
	if len(response.Cells) > 0 {
		metrics.MemoryRecallCellsTotal.Add(float64(len(response.Cells)))
	}

	// Alert if retrieval is slow
	recallDuration := time.Since(startTime)
	if isSlowRecall(recallDuration) {
		alertSlowRecall(ctx, request.ConversationID, recallDuration)
	}

	logRecallComplete(request.ConversationID, len(response.Cells), len(response.Scenes),
		response.TotalTokens, response.FTSMatchCount, response.FallbackUsed, recallDuration)

	return response, nil
}

// applyBudgetWithScenes applies token budget including scene tokens
func (s *MemoryRecallService) applyBudgetWithScenes(
	cells []*memoryv2.MemoryCell,
	scenes []*memoryv2.MemoryScene,
	maxTokens int,
) ([]*memoryv2.MemoryCell, []*memoryv2.MemoryScene, int) {
	if len(cells) == 0 {
		return cells, scenes, 0
	}

	// Calculate scene tokens
	sceneTokenMap := make(map[string]int)
	totalSceneTokens := 0
	for _, scene := range scenes {
		sceneTokenMap[scene.Scene] = scene.TokenCount
		totalSceneTokens += scene.TokenCount
	}

	// Start with scene tokens in budget
	totalTokens := totalSceneTokens
	result := []*memoryv2.MemoryCell{}
	usedScenes := make(map[string]bool)

	for _, cell := range cells {
		// Estimate tokens for this cell
		cellTokens := s.estimateTokens(cell.Content)

		// Check if adding this cell would exceed budget
		if maxTokens > 0 && (totalTokens+cellTokens) > maxTokens {
			break
		}

		result = append(result, cell)
		totalTokens += cellTokens
		usedScenes[cell.Scene] = true
	}

	// Filter scenes to only those actually used
	usedSceneList := []*memoryv2.MemoryScene{}
	for _, scene := range scenes {
		if usedScenes[scene.Scene] {
			usedSceneList = append(usedSceneList, scene)
		}
	}

	return result, usedSceneList, totalTokens
}

// getScenes retrieves unique scenes for the given cells
func (s *MemoryRecallService) getScenes(ctx context.Context, cells []*memoryv2.MemoryCell) ([]*memoryv2.MemoryScene, int) {
	// Deduplicate scene names
	sceneNames := make(map[string]bool)
	for _, cell := range cells {
		sceneNames[cell.Scene] = true
	}

	// Limit number of scenes
	maxScenes := s.config.MaxScenes
	if maxScenes == 0 {
		maxScenes = 10 // Default
	}

	scenes := []*memoryv2.MemoryScene{}
	totalTokens := 0

	for sceneName := range sceneNames {
		if len(scenes) >= maxScenes {
			break
		}

		scene, err := s.sceneRepo.Get(ctx, sceneName)
		if err != nil {
			// Skip scenes that don't exist (shouldn't happen, but be defensive)
			continue
		}

		scenes = append(scenes, scene)
		totalTokens += scene.TokenCount
	}

	return scenes, totalTokens
}

// estimateTokens estimates token count for text
// Simple heuristic: ~4 characters per token
func (s *MemoryRecallService) estimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// FormatMemoryForInjection formats recalled memory for context injection
func (s *MemoryRecallService) FormatMemoryForInjection(response *RecallResponse) string {
	if len(response.Cells) == 0 && len(response.Scenes) == 0 {
		return ""
	}

	output := "### Relevant Long-Term Memory (Curated)\n\n"

	// Group cells by scene
	cellsByScene := make(map[string][]*memoryv2.MemoryCell)
	for _, cell := range response.Cells {
		cellsByScene[cell.Scene] = append(cellsByScene[cell.Scene], cell)
	}

	// Output scene summaries with their cells
	for _, scene := range response.Scenes {
		output += fmt.Sprintf("**Scene: %s**\n", scene.Scene)
		output += fmt.Sprintf("Summary: %s\n\n", scene.Summary)

		// Output cells for this scene
		if cells, ok := cellsByScene[scene.Scene]; ok {
			output += "**Key Facts:**\n"
			for _, cell := range cells {
				output += fmt.Sprintf("- [%s, salience=%.2f] %s\n",
					cell.CellType.String(),
					cell.Salience,
					cell.Content,
				)
			}
			output += "\n"
		}
	}

	output += fmt.Sprintf("*Retrieved %d cells from %d scenes (%d tokens)*\n",
		len(response.Cells),
		len(response.Scenes),
		response.TotalTokens,
	)

	return output
}
