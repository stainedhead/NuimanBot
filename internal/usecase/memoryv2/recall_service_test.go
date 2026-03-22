package memoryv2

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/tracing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// MockFTSCellRepository extends MockCellRepository with FTS capability
type MockFTSCellRepository struct {
	MockCellRepository
	FTSResults []*memoryv2.MemoryCell
	FTSQuery   string
}

func (m *MockFTSCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	m.FTSQuery = query
	results := m.FTSResults
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (m *MockFTSCellRepository) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	results := []*memoryv2.MemoryCell{}
	for _, cell := range m.Cells {
		matchesConv := conversationID == "" || cell.ConversationID == conversationID
		if matchesConv && cell.Salience >= threshold {
			results = append(results, cell)
		}
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func TestMemoryRecallService_RecallMemory(t *testing.T) {
	t.Run("fts_search_returns_results", func(t *testing.T) {
		now := time.Now()

		// Setup FTS results
		ftsResults := []*memoryv2.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-123",
				Scene:          "authentication",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.9,
				Content:        "User configured OAuth2 authentication",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "cell-2",
				ConversationID: "conv-123",
				Scene:          "authentication",
				CellType:       memoryv2.CellTypeDecision,
				Salience:       0.85,
				Content:        "Decided to use JWT tokens with 24-hour expiry",
				Source:         `["msg-2"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "authentication",
				Summary:    "User is configuring OAuth2 authentication with JWT tokens",
				TokenCount: 12,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{
			FTSResults: ftsResults,
		}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			TokenBudget:       500,
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "How do I configure authentication?",
			MaxTokens:      500,
			MaxCells:       10,
		}

		ctx := context.Background()
		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(response.Cells) != 2 {
			t.Errorf("Expected 2 cells, got %d", len(response.Cells))
		}

		if response.FTSMatchCount != 2 {
			t.Errorf("Expected 2 FTS matches, got %d", response.FTSMatchCount)
		}

		if response.FallbackUsed {
			t.Error("Expected no fallback when FTS returns results")
		}

		if len(response.Scenes) != 1 {
			t.Errorf("Expected 1 scene, got %d", len(response.Scenes))
		}

		if response.Scenes[0].Scene != "authentication" {
			t.Errorf("Expected authentication scene, got %s", response.Scenes[0].Scene)
		}
	})

	t.Run("fallback_to_salience_when_fts_empty", func(t *testing.T) {
		now := time.Now()

		// No FTS results, but cells available for fallback
		highSalienceCells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-3",
				ConversationID: "conv-123",
				Scene:          "project-setup",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.95,
				Content:        "User prefers TDD workflow",
				Source:         `["msg-3"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "project-setup",
				Summary:    "User is setting up development workflow",
				TokenCount: 10,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{
			FTSResults: []*memoryv2.MemoryCell{}, // Empty FTS
			MockCellRepository: MockCellRepository{
				Cells: highSalienceCells,
			},
		}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			FallbackCellLimit: 5,
			TokenBudget:       500,
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "some query",
			MaxTokens:      500,
			MaxCells:       10,
		}

		ctx := context.Background()
		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !response.FallbackUsed {
			t.Error("Expected fallback to be used when FTS is empty")
		}

		if response.FTSMatchCount != 0 {
			t.Errorf("Expected 0 FTS matches, got %d", response.FTSMatchCount)
		}

		if len(response.Cells) != 1 {
			t.Errorf("Expected 1 cell from fallback, got %d", len(response.Cells))
		}
	})

	t.Run("respects_token_budget", func(t *testing.T) {
		now := time.Now()

		// Create many cells that would exceed token budget
		cells := make([]*memoryv2.MemoryCell, 10)
		for i := 0; i < 10; i++ {
			cells[i] = &memoryv2.MemoryCell{
				ID:             string(rune('a' + i)),
				ConversationID: "conv-123",
				Scene:          "test-scene",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.9,
				Content:        "This is a test cell with some content that takes tokens",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "test-scene",
				Summary:    "Test scene summary",
				TokenCount: 5,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{FTSResults: cells}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			TokenBudget:       100, // Small budget
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "test query",
			MaxTokens:      100,
			MaxCells:       10,
		}

		ctx := context.Background()
		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Should have fewer cells due to token budget
		if len(response.Cells) >= 10 {
			t.Error("Expected token budget to limit cell count")
		}

		if response.TotalTokens > 100 {
			t.Errorf("Expected total tokens <= 100, got %d", response.TotalTokens)
		}
	})

	t.Run("deduplicates_scenes", func(t *testing.T) {
		now := time.Now()

		// Multiple cells from same scene
		cells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-123",
				Scene:          "authentication",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.9,
				Content:        "Cell 1",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "cell-2",
				ConversationID: "conv-123",
				Scene:          "authentication",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.85,
				Content:        "Cell 2",
				Source:         `["msg-2"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "authentication",
				Summary:    "Authentication scene",
				TokenCount: 5,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{FTSResults: cells}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			TokenBudget:       500,
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "test",
			MaxTokens:      500,
			MaxCells:       10,
		}

		ctx := context.Background()
		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Should have only 1 scene despite 2 cells from same scene
		if len(response.Scenes) != 1 {
			t.Errorf("Expected 1 deduplicated scene, got %d", len(response.Scenes))
		}
	})
}

func TestRecallMemory_WithTracing(t *testing.T) {
	cleanup := initTestTracing(t)
	defer cleanup()

	t.Run("fts_search_with_tracing", func(t *testing.T) {
		now := time.Now()

		ftsResults := []*memoryv2.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-123",
				Scene:          "test-scene",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.9,
				Content:        "Traced FTS result",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "test-scene",
				Summary:    "Test scene",
				TokenCount: 5,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{FTSResults: ftsResults}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			TokenBudget:       500,
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		// Start parent span
		ctx, parentSpan := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "test query with tracing",
			MaxTokens:      500,
			MaxCells:       10,
		}

		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error with tracing, got: %v", err)
		}

		if len(response.Cells) != 1 {
			t.Errorf("Expected 1 cell, got %d", len(response.Cells))
		}

		if response.FTSMatchCount != 1 {
			t.Errorf("Expected 1 FTS match, got %d", response.FTSMatchCount)
		}

		// Verify tracing is active
		if parentSpan.TraceID == "" {
			t.Error("Expected non-empty trace ID")
		}
	})

	t.Run("fallback_with_tracing", func(t *testing.T) {
		now := time.Now()

		highSalienceCells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-2",
				ConversationID: "conv-123",
				Scene:          "fallback-scene",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.95,
				Content:        "High salience cell",
				Source:         `["msg-2"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{
				Scene:      "fallback-scene",
				Summary:    "Fallback scene",
				TokenCount: 5,
				UpdatedAt:  now,
			},
		}

		mockCellRepo := &MockFTSCellRepository{
			FTSResults:         []*memoryv2.MemoryCell{}, // Empty FTS
			MockCellRepository: MockCellRepository{Cells: highSalienceCells},
		}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{
			FTSResultLimit:    10,
			SalienceThreshold: 0.8,
			FallbackCellLimit: 5,
			TokenBudget:       500,
		}

		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		ctx, _ := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "fallback test",
			MaxTokens:      500,
			MaxCells:       10,
		}

		response, err := recall.RecallMemory(ctx, request)
		if err != nil {
			t.Fatalf("Expected no error with tracing, got: %v", err)
		}

		if !response.FallbackUsed {
			t.Error("Expected fallback to be used")
		}

		if len(response.Cells) != 1 {
			t.Errorf("Expected 1 cell from fallback, got %d", len(response.Cells))
		}
	})
}

func TestRecallMemory_Metrics(t *testing.T) {
	t.Run("fts_recall_increments_counters", func(t *testing.T) {
		now := time.Now()

		initialFTSSuccess := testutil.ToFloat64(metrics.MemoryRecallTotal.WithLabelValues("success", "fts"))
		initialCellsTotal := testutil.ToFloat64(metrics.MemoryRecallCellsTotal)

		ftsResults := []*memoryv2.MemoryCell{
			{
				ID: "cell-m1", ConversationID: "conv-123", Scene: "metrics-scene",
				CellType: memoryv2.CellTypeFact, Salience: 0.9, Content: "FTS result",
				Source: `["msg-1"]`, CreatedAt: now, UpdatedAt: now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{Scene: "metrics-scene", Summary: "Scene", TokenCount: 5, UpdatedAt: now},
		}

		mockCellRepo := &MockFTSCellRepository{FTSResults: ftsResults}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{FTSResultLimit: 10, SalienceThreshold: 0.8, TokenBudget: 500}
		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{ConversationID: "conv-123", Query: "metrics test", MaxTokens: 500, MaxCells: 10}
		response, err := recall.RecallMemory(context.Background(), request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(response.Cells) != 1 {
			t.Errorf("Expected 1 cell, got %d", len(response.Cells))
		}

		newFTSSuccess := testutil.ToFloat64(metrics.MemoryRecallTotal.WithLabelValues("success", "fts"))
		if newFTSSuccess != initialFTSSuccess+1 {
			t.Errorf("Expected FTS success counter to increment by 1, got delta %f", newFTSSuccess-initialFTSSuccess)
		}

		newCellsTotal := testutil.ToFloat64(metrics.MemoryRecallCellsTotal)
		if newCellsTotal != initialCellsTotal+1 {
			t.Errorf("Expected cells total to increment by 1, got delta %f", newCellsTotal-initialCellsTotal)
		}
	})

	t.Run("fallback_recall_increments_fallback_counter", func(t *testing.T) {
		now := time.Now()

		initialFallback := testutil.ToFloat64(metrics.MemoryRecallTotal.WithLabelValues("success", "fallback"))

		highSalienceCells := []*memoryv2.MemoryCell{
			{
				ID: "cell-m2", ConversationID: "conv-123", Scene: "fallback-scene",
				CellType: memoryv2.CellTypeFact, Salience: 0.95, Content: "High salience",
				Source: `["msg-2"]`, CreatedAt: now, UpdatedAt: now,
			},
		}

		scenes := []*memoryv2.MemoryScene{
			{Scene: "fallback-scene", Summary: "Fallback", TokenCount: 5, UpdatedAt: now},
		}

		mockCellRepo := &MockFTSCellRepository{
			FTSResults:         []*memoryv2.MemoryCell{},
			MockCellRepository: MockCellRepository{Cells: highSalienceCells},
		}
		mockSceneRepo := &MockSceneRepository{Scenes: scenes}

		config := RecallConfig{FTSResultLimit: 10, SalienceThreshold: 0.8, FallbackCellLimit: 5, TokenBudget: 500}
		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{ConversationID: "conv-123", Query: "fallback metrics", MaxTokens: 500, MaxCells: 10}
		response, err := recall.RecallMemory(context.Background(), request)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if !response.FallbackUsed {
			t.Error("Expected fallback to be used")
		}

		newFallback := testutil.ToFloat64(metrics.MemoryRecallTotal.WithLabelValues("success", "fallback"))
		if newFallback != initialFallback+1 {
			t.Errorf("Expected fallback success counter to increment by 1, got delta %f", newFallback-initialFallback)
		}
	})
}
