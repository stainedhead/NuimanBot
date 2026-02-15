package memoryv2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/alerting"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/tracing"

	"github.com/google/uuid"
)

// LLMClient defines the contract for LLM operations
type LLMClient interface {
	// GenerateJSON calls the LLM and expects a JSON response matching the schema
	GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, responseSchema interface{}) (string, error)
}

// MemoryCuratorService handles memory extraction and scene consolidation
type MemoryCuratorService struct {
	llm       LLMClient
	cellRepo  memoryv2.MemoryCellRepository
	sceneRepo memoryv2.MemorySceneRepository
	config    CuratorConfig
}

// NewMemoryCuratorService creates a new memory curator service
func NewMemoryCuratorService(
	llm LLMClient,
	cellRepo memoryv2.MemoryCellRepository,
	sceneRepo memoryv2.MemorySceneRepository,
	config CuratorConfig,
) *MemoryCuratorService {
	return &MemoryCuratorService{
		llm:       llm,
		cellRepo:  cellRepo,
		sceneRepo: sceneRepo,
		config:    config,
	}
}

// ExtractCells extracts memory cells from a completed interaction
func (s *MemoryCuratorService) ExtractCells(ctx context.Context, interaction InteractionContext) (*CurationResult, error) {
	ctx, _ = tracing.StartSpan(ctx, "memory.extract_cells")
	defer tracing.EndSpan(ctx)
	tracing.AddAttribute(ctx, "conversation_id", interaction.ConversationID)

	extractionStart := time.Now()
	defer func() {
		metrics.MemoryExtractionDuration.Observe(time.Since(extractionStart).Seconds())
	}()

	// Skip extraction if disabled
	if !s.config.Enabled {
		slog.Debug("Memory extraction skipped: disabled",
			"conversation_id", interaction.ConversationID,
		)
		tracing.AddAttribute(ctx, "skipped", true)
		metrics.MemoryExtractionTotal.WithLabelValues("skipped").Inc()
		return &CurationResult{
			CellsCreated:  0,
			ScenesUpdated: 0,
		}, nil
	}

	slog.Info("Starting memory extraction",
		"conversation_id", interaction.ConversationID,
	)

	// Build LLM prompt
	systemPrompt := s.buildExtractionSystemPrompt()
	userPrompt := s.buildExtractionUserPrompt(interaction)

	// Call LLM for extraction
	var extractionResp ExtractionResponse
	responseJSON, err := s.llm.GenerateJSON(ctx, systemPrompt, userPrompt, &extractionResp)
	if err != nil {
		slog.Error("LLM extraction failed",
			"conversation_id", interaction.ConversationID,
			"error", err,
		)
		alerting.SendAlert(ctx, alerting.Alert{
			Severity: alerting.SeverityError,
			Title:    "Memory extraction LLM failure",
			Message:  fmt.Sprintf("LLM extraction failed for conversation %s: %v", interaction.ConversationID, err),
			Tags:     map[string]string{"component": "memory", "operation": "extraction"},
		})
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryExtractionTotal.WithLabelValues("error").Inc()
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse response
	err = json.Unmarshal([]byte(responseJSON), &extractionResp)
	if err != nil {
		slog.Error("Invalid JSON from LLM extraction",
			"conversation_id", interaction.ConversationID,
			"error", err,
		)
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryExtractionTotal.WithLabelValues("error").Inc()
		if s.config.RetryOnInvalidJSON {
			// TODO: Implement retry logic with repair prompt
			return nil, fmt.Errorf("invalid JSON response (retry not yet implemented): %w", err)
		}
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Validate and persist cells
	result := &CurationResult{
		Errors: []error{},
	}

	touchedScenes := make(map[string]bool)

	for _, extractedCell := range extractionResp.Cells {
		cell, err := s.convertToMemoryCell(extractedCell, interaction)
		if err != nil {
			slog.Warn("Invalid extracted cell",
				"conversation_id", interaction.ConversationID,
				"scene", extractedCell.Scene,
				"cell_type", extractedCell.CellType,
				"error", err,
			)
			result.Errors = append(result.Errors, fmt.Errorf("invalid cell: %w", err))
			continue
		}

		err = s.cellRepo.Create(ctx, cell)
		if err != nil {
			slog.Error("Failed to persist memory cell",
				"conversation_id", interaction.ConversationID,
				"cell_id", cell.ID,
				"scene", cell.Scene,
				"error", err,
			)
			result.Errors = append(result.Errors, fmt.Errorf("failed to create cell: %w", err))
			continue
		}

		result.CellsCreated++
		touchedScenes[cell.Scene] = true
	}

	// Consolidate touched scenes
	for scene := range touchedScenes {
		err := s.ConsolidateScene(ctx, scene)
		if err != nil {
			slog.Error("Scene consolidation failed",
				"conversation_id", interaction.ConversationID,
				"scene", scene,
				"error", err,
			)
			result.Errors = append(result.Errors, fmt.Errorf("failed to consolidate scene %s: %w", scene, err))
		} else {
			result.ScenesUpdated++
		}
	}

	tracing.AddAttribute(ctx, "cells_created", result.CellsCreated)
	tracing.AddAttribute(ctx, "scenes_updated", result.ScenesUpdated)
	tracing.AddAttribute(ctx, "error_count", len(result.Errors))

	// Record metrics for created cells
	if result.CellsCreated > 0 {
		metrics.MemoryCellsCreatedTotal.Add(float64(result.CellsCreated))
	}

	// Return error if no cells were created but we had errors
	if result.CellsCreated == 0 && len(result.Errors) > 0 {
		slog.Error("Extraction produced no cells",
			"conversation_id", interaction.ConversationID,
			"error_count", len(result.Errors),
		)
		alerting.SendAlert(ctx, alerting.Alert{
			Severity: alerting.SeverityWarning,
			Title:    "Memory extraction produced no cells",
			Message:  fmt.Sprintf("Extraction for conversation %s failed: 0 cells, %d errors", interaction.ConversationID, len(result.Errors)),
			Tags:     map[string]string{"component": "memory", "operation": "extraction"},
		})
		metrics.MemoryExtractionTotal.WithLabelValues("error").Inc()
		return result, fmt.Errorf("extraction failed: no cells created, %d errors", len(result.Errors))
	}

	extractionDuration := time.Since(extractionStart)
	logExtractionComplete(interaction.ConversationID, result.CellsCreated, result.ScenesUpdated, len(result.Errors), extractionDuration)

	if isSlowExtraction(extractionDuration) {
		alertExtractionFailed(ctx, interaction.ConversationID, fmt.Errorf("extraction exceeded latency threshold: %v", extractionDuration))
	}

	metrics.MemoryExtractionTotal.WithLabelValues("success").Inc()
	return result, nil
}

// ConsolidateScene generates or updates a scene summary based on all cells in the scene
func (s *MemoryCuratorService) ConsolidateScene(ctx context.Context, sceneName string) error {
	ctx, _ = tracing.StartSpan(ctx, "memory.consolidate_scene")
	defer tracing.EndSpan(ctx)
	tracing.AddAttribute(ctx, "scene_name", sceneName)

	consolidationStart := time.Now()
	defer func() {
		metrics.MemoryConsolidationDuration.Observe(time.Since(consolidationStart).Seconds())
	}()

	// Get all cells for this scene
	cells, err := s.cellRepo.GetByScene(ctx, sceneName, 0) // 0 = no limit
	if err != nil {
		slog.Error("Failed to get cells for scene consolidation",
			"scene", sceneName,
			"error", err,
		)
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryConsolidationTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("failed to get cells for scene: %w", err)
	}

	tracing.AddAttribute(ctx, "cell_count", len(cells))

	if len(cells) == 0 {
		// No cells, nothing to consolidate
		return nil
	}

	// Get existing scene (if any)
	existingScene, err := s.sceneRepo.Get(ctx, sceneName)
	var existingSummary string
	if err == nil {
		existingSummary = existingScene.Summary
	}

	// Build consolidation prompt
	systemPrompt := s.buildConsolidationSystemPrompt()
	userPrompt := s.buildConsolidationUserPrompt(sceneName, cells, existingSummary)

	// Call LLM for consolidation
	var consolidationResp SceneConsolidationResponse
	responseJSON, err := s.llm.GenerateJSON(ctx, systemPrompt, userPrompt, &consolidationResp)
	if err != nil {
		slog.Error("LLM consolidation failed",
			"scene", sceneName,
			"cell_count", len(cells),
			"error", err,
		)
		alerting.SendAlert(ctx, alerting.Alert{
			Severity: alerting.SeverityError,
			Title:    "Scene consolidation LLM failure",
			Message:  fmt.Sprintf("LLM consolidation failed for scene %s: %v", sceneName, err),
			Tags:     map[string]string{"component": "memory", "operation": "consolidation"},
		})
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryConsolidationTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("LLM consolidation failed: %w", err)
	}

	// Parse response
	err = json.Unmarshal([]byte(responseJSON), &consolidationResp)
	if err != nil {
		slog.Error("Invalid JSON from LLM consolidation",
			"scene", sceneName,
			"error", err,
		)
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryConsolidationTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	// Create or update scene
	scene := &memoryv2.MemoryScene{
		Scene:      sceneName,
		Summary:    consolidationResp.Summary,
		TokenCount: consolidationResp.TokenCount,
		UpdatedAt:  time.Now(),
	}

	err = s.sceneRepo.Upsert(ctx, scene)
	if err != nil {
		slog.Error("Failed to upsert scene",
			"scene", sceneName,
			"error", err,
		)
		tracing.RecordError(ctx, err.Error())
		metrics.MemoryConsolidationTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("failed to upsert scene: %w", err)
	}

	consolidationDuration := time.Since(consolidationStart)
	slog.Info("Scene consolidation completed",
		"component", "memory_curator",
		"scene", sceneName,
		"cell_count", len(cells),
		"token_count", consolidationResp.TokenCount,
		"duration_ms", consolidationDuration.Milliseconds(),
	)

	if isSlowConsolidation(consolidationDuration) {
		alertConsolidationFailed(ctx, sceneName, fmt.Errorf("consolidation exceeded latency threshold: %v", consolidationDuration))
	}

	metrics.MemoryConsolidationTotal.WithLabelValues("success").Inc()
	return nil
}

// convertToMemoryCell converts an extracted cell to a domain MemoryCell
func (s *MemoryCuratorService) convertToMemoryCell(extracted ExtractedCell, interaction InteractionContext) (*memoryv2.MemoryCell, error) {
	// Parse cell type
	cellType, err := memoryv2.ParseCellType(extracted.CellType)
	if err != nil {
		return nil, fmt.Errorf("invalid cell type: %w", err)
	}

	// Marshal source as JSON
	sourceJSON, err := json.Marshal(extracted.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source: %w", err)
	}

	now := time.Now()

	cell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: interaction.ConversationID,
		Scene:          extracted.Scene,
		CellType:       cellType,
		Salience:       extracted.Salience,
		Content:        extracted.Content,
		Source:         string(sourceJSON),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Validate
	err = cell.Validate()
	if err != nil {
		return nil, err
	}

	return cell, nil
}

// buildExtractionSystemPrompt creates the system prompt for memory extraction
func (s *MemoryCuratorService) buildExtractionSystemPrompt() string {
	return `You are a memory extraction assistant. Your job is to analyze completed interactions and extract structured memory cells.

Memory cells are typed knowledge units:
- **fact**: Objective information, statements, or observations
- **decision**: Choices made or preferences expressed
- **task**: Action items, TODOs, or goals
- **preference**: User preferences, likes/dislikes, or patterns
- **plan**: Future plans, strategies, or roadmaps
- **risk**: Warnings, concerns, or potential issues

Each cell has:
- **scene**: Topic/context bucket (e.g., "project-setup", "user-preferences", "debugging")
- **cell_type**: One of the types above
- **salience**: Importance score 0.0-1.0 (1.0 = critical, 0.5 = moderate, 0.0 = trivial)
- **content**: Clear, self-contained description (avoid "they", "it", use names/specifics)
- **source**: Array of message IDs this cell was extracted from

Guidelines:
- Extract only important, actionable, or memorable information
- Use clear, specific language (avoid pronouns like "they", "it")
- Assign appropriate salience (don't over-inflate importance)
- Group related cells into coherent scenes
- Skip trivial chit-chat or transient information
- Return empty array if nothing worth remembering

Return JSON in this format:
{
  "cells": [
    {
      "scene": "scene-name",
      "cell_type": "fact",
      "salience": 0.85,
      "content": "Clear, specific description",
      "source": ["msg-1", "msg-2"]
    }
  ]
}`
}

// buildExtractionUserPrompt creates the user prompt for memory extraction
func (s *MemoryCuratorService) buildExtractionUserPrompt(interaction InteractionContext) string {
	prompt := fmt.Sprintf("Extract memory cells from this interaction:\n\n**User:**\n%s\n\n**Assistant:**\n%s",
		interaction.UserMessage,
		interaction.AssistantReply,
	)

	if len(interaction.ToolOutputs) > 0 {
		prompt += "\n\n**Tool Outputs:**\n"
		for i, output := range interaction.ToolOutputs {
			prompt += fmt.Sprintf("%d. %s\n", i+1, output)
		}
	}

	prompt += "\n\nExtract memory cells (return empty array if nothing worth remembering)."

	return prompt
}

// buildConsolidationSystemPrompt creates the system prompt for scene consolidation
func (s *MemoryCuratorService) buildConsolidationSystemPrompt() string {
	return `You are a scene summary consolidation assistant. Your job is to create concise, stable summaries of topic-organized memory scenes.

Scene summaries should:
- Be clear, self-contained, and informative
- Avoid ephemeral phrasing like "recently", "this week", or "just now"
- Focus on what is known, decided, or planned (not when)
- Be stable across updates (incremental, not regenerated from scratch)
- Respect token budget constraints

Return JSON in this format:
{
  "summary": "Clear summary of the scene",
  "token_count": 15
}`
}

// buildConsolidationUserPrompt creates the user prompt for scene consolidation
func (s *MemoryCuratorService) buildConsolidationUserPrompt(sceneName string, cells []*memoryv2.MemoryCell, existingSummary string) string {
	prompt := fmt.Sprintf("Consolidate summary for scene: **%s**\n\n", sceneName)

	if existingSummary != "" {
		prompt += fmt.Sprintf("**Existing Summary:**\n%s\n\n", existingSummary)
	}

	prompt += "**Memory Cells:**\n"
	for i, cell := range cells {
		prompt += fmt.Sprintf("%d. [%s, salience=%.2f] %s\n", i+1, cell.CellType.String(), cell.Salience, cell.Content)
	}

	maxTokens := s.config.SceneSummaryMaxTokens
	if maxTokens == 0 {
		maxTokens = 500 // Default
	}

	prompt += fmt.Sprintf("\n\nGenerate a consolidated summary (max %d tokens). Avoid ephemeral phrasing.", maxTokens)

	return prompt
}
