package memoryv2

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/tracing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// initTestTracing initializes tracing for tests and returns a cleanup function.
func initTestTracing(t *testing.T) func() {
	t.Helper()
	err := tracing.Initialize(tracing.Config{
		Enabled:     true,
		ServiceName: "memoryv2-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}
	return func() { _ = tracing.Shutdown(context.Background()) }
}

// MockLLMClient is a mock LLM client for testing
type MockLLMClient struct {
	ResponseJSON string
	Error        error
	CallCount    int
}

func (m *MockLLMClient) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, responseSchema interface{}) (string, error) {
	m.CallCount++
	if m.Error != nil {
		return "", m.Error
	}
	return m.ResponseJSON, nil
}

// MockCellRepository is a mock memory cell repository for testing
type MockCellRepository struct {
	Cells     []*memoryv2.MemoryCell
	CreateErr error
	ListCalls int
}

func (m *MockCellRepository) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Cells = append(m.Cells, cell)
	return nil
}

func (m *MockCellRepository) Get(ctx context.Context, id string) (*memoryv2.MemoryCell, error) {
	for _, cell := range m.Cells {
		if cell.ID == id {
			return cell, nil
		}
	}
	return nil, memoryv2.ErrNotFound
}

func (m *MockCellRepository) Update(ctx context.Context, cell *memoryv2.MemoryCell) error {
	for i, c := range m.Cells {
		if c.ID == cell.ID {
			m.Cells[i] = cell
			return nil
		}
	}
	return memoryv2.ErrNotFound
}

func (m *MockCellRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockCellRepository) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	m.ListCalls++
	results := []*memoryv2.MemoryCell{}
	for _, cell := range m.Cells {
		if filter.Scene != "" && cell.Scene != filter.Scene {
			continue
		}
		if filter.ConversationID != "" && cell.ConversationID != filter.ConversationID {
			continue
		}
		results = append(results, cell)
	}
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return results, nil
}

func (m *MockCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *MockCellRepository) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	results := []*memoryv2.MemoryCell{}
	for _, cell := range m.Cells {
		if cell.Scene == scene {
			results = append(results, cell)
		}
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (m *MockCellRepository) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *MockCellRepository) DeleteExpired(ctx context.Context) (int, error) {
	return 0, nil
}

// MockSceneRepository is a mock memory scene repository for testing
type MockSceneRepository struct {
	Scenes    []*memoryv2.MemoryScene
	UpsertErr error
}

func (m *MockSceneRepository) Upsert(ctx context.Context, scene *memoryv2.MemoryScene) error {
	if m.UpsertErr != nil {
		return m.UpsertErr
	}
	// Find and update or append
	for i, s := range m.Scenes {
		if s.Scene == scene.Scene {
			m.Scenes[i] = scene
			return nil
		}
	}
	m.Scenes = append(m.Scenes, scene)
	return nil
}

func (m *MockSceneRepository) Get(ctx context.Context, scene string) (*memoryv2.MemoryScene, error) {
	for _, s := range m.Scenes {
		if s.Scene == scene {
			return s, nil
		}
	}
	return nil, memoryv2.ErrNotFound
}

func (m *MockSceneRepository) List(ctx context.Context) ([]*memoryv2.MemoryScene, error) {
	return m.Scenes, nil
}

func (m *MockSceneRepository) Delete(ctx context.Context, scene string) error {
	return nil
}

func TestMemoryCuratorService_ExtractCells(t *testing.T) {
	t.Run("successful_extraction", func(t *testing.T) {
		// Setup mock LLM response
		extractionResp := ExtractionResponse{
			Cells: []ExtractedCell{
				{
					Scene:    "project-setup",
					CellType: "decision",
					Salience: 0.85,
					Content:  "User decided to use TDD methodology",
					Source:   []string{"msg-1"},
				},
				{
					Scene:    "user-preferences",
					CellType: "preference",
					Salience: 0.9,
					Content:  "User prefers dark mode",
					Source:   []string{"msg-2"},
				},
			},
		}
		responseJSON, _ := json.Marshal(extractionResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{
			Enabled:               true,
			ExtractionModel:       "claude-3-haiku-20240307",
			MaxCellsPerExtraction: 10,
			RetryOnInvalidJSON:    true,
		}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "I want to use TDD and dark mode",
			AssistantReply: "Great choices! TDD ensures quality code.",
			MessageIDs:     []string{"msg-1", "msg-2"},
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		result, err := curator.ExtractCells(ctx, interaction)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.CellsCreated != 2 {
			t.Errorf("Expected 2 cells created, got %d", result.CellsCreated)
		}

		if len(mockCellRepo.Cells) != 2 {
			t.Errorf("Expected 2 cells in repository, got %d", len(mockCellRepo.Cells))
		}

		// Verify cell properties
		firstCell := mockCellRepo.Cells[0]
		if firstCell.Scene != "project-setup" {
			t.Errorf("Expected scene 'project-setup', got '%s'", firstCell.Scene)
		}
		if firstCell.CellType != memoryv2.CellTypeDecision {
			t.Errorf("Expected cell type Decision, got %v", firstCell.CellType)
		}
		if firstCell.Salience != 0.85 {
			t.Errorf("Expected salience 0.85, got %f", firstCell.Salience)
		}
	})

	t.Run("handles_llm_error", func(t *testing.T) {
		mockLLM := &MockLLMClient{Error: errors.New("LLM API error")}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: true}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "Test message",
			AssistantReply: "Test reply",
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		_, err := curator.ExtractCells(ctx, interaction)
		if err == nil {
			t.Error("Expected error for LLM failure, got nil")
		}
	})

	t.Run("handles_invalid_json", func(t *testing.T) {
		mockLLM := &MockLLMClient{ResponseJSON: "invalid json {{}"}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{
			Enabled:            true,
			RetryOnInvalidJSON: false, // Disable retry for this test
		}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "Test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		_, err := curator.ExtractCells(ctx, interaction)
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}
	})

	t.Run("skips_extraction_when_disabled", func(t *testing.T) {
		mockLLM := &MockLLMClient{}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: false}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "Test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		result, err := curator.ExtractCells(ctx, interaction)
		if err != nil {
			t.Fatalf("Expected no error when disabled, got: %v", err)
		}

		if result.CellsCreated != 0 {
			t.Errorf("Expected 0 cells when disabled, got %d", result.CellsCreated)
		}

		if mockLLM.CallCount != 0 {
			t.Error("Expected no LLM calls when disabled")
		}
	})
}

func TestMemoryCuratorService_ConsolidateScene(t *testing.T) {
	t.Run("consolidate_new_scene", func(t *testing.T) {
		// Setup mock cells for the scene
		now := time.Now()
		mockCells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-123",
				Scene:          "project-setup",
				CellType:       memoryv2.CellTypeDecision,
				Salience:       0.85,
				Content:        "User decided to use TDD",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "cell-2",
				ConversationID: "conv-123",
				Scene:          "project-setup",
				CellType:       memoryv2.CellTypePreference,
				Salience:       0.9,
				Content:        "User prefers dark mode",
				Source:         `["msg-2"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		// Setup mock LLM response
		consolidationResp := SceneConsolidationResponse{
			Summary:    "User is setting up a new project with TDD methodology and dark mode preferences",
			TokenCount: 15,
		}
		responseJSON, _ := json.Marshal(consolidationResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{Cells: mockCells}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{
			Enabled:               true,
			ConsolidationModel:    "claude-3-haiku-20240307",
			SceneSummaryMaxTokens: 500,
		}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		ctx := context.Background()
		err := curator.ConsolidateScene(ctx, "project-setup")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify scene was created in repository
		if len(mockSceneRepo.Scenes) != 1 {
			t.Errorf("Expected 1 scene in repository, got %d", len(mockSceneRepo.Scenes))
		}

		scene := mockSceneRepo.Scenes[0]
		if scene.Scene != "project-setup" {
			t.Errorf("Expected scene name 'project-setup', got '%s'", scene.Scene)
		}
		if scene.TokenCount != 15 {
			t.Errorf("Expected token count 15, got %d", scene.TokenCount)
		}
	})

	t.Run("update_existing_scene", func(t *testing.T) {
		// Start with existing scene and cells
		now := time.Now()
		mockCells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-3",
				ConversationID: "conv-123",
				Scene:          "project-setup",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.75,
				Content:        "New fact about the project",
				Source:         `["msg-3"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		existingScene := &memoryv2.MemoryScene{
			Scene:      "project-setup",
			Summary:    "Old summary",
			TokenCount: 10,
			UpdatedAt:  time.Now().Add(-1 * time.Hour),
		}

		consolidationResp := SceneConsolidationResponse{
			Summary:    "Updated summary with new information",
			TokenCount: 20,
		}
		responseJSON, _ := json.Marshal(consolidationResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{Cells: mockCells}
		mockSceneRepo := &MockSceneRepository{Scenes: []*memoryv2.MemoryScene{existingScene}}

		config := CuratorConfig{
			Enabled:               true,
			ConsolidationModel:    "claude-3-haiku-20240307",
			SceneSummaryMaxTokens: 500,
		}

		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		ctx := context.Background()
		err := curator.ConsolidateScene(ctx, "project-setup")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify scene was updated (not duplicated)
		if len(mockSceneRepo.Scenes) != 1 {
			t.Errorf("Expected 1 scene (updated, not duplicated), got %d", len(mockSceneRepo.Scenes))
		}

		scene := mockSceneRepo.Scenes[0]
		if scene.Summary != "Updated summary with new information" {
			t.Errorf("Expected updated summary")
		}
		if scene.TokenCount != 20 {
			t.Errorf("Expected updated token count 20, got %d", scene.TokenCount)
		}
	})
}

func TestExtractCells_WithTracing(t *testing.T) {
	cleanup := initTestTracing(t)
	defer cleanup()

	t.Run("successful_extraction_with_tracing", func(t *testing.T) {
		extractionResp := ExtractionResponse{
			Cells: []ExtractedCell{
				{
					Scene:    "test-scene",
					CellType: "fact",
					Salience: 0.8,
					Content:  "Test fact with tracing",
					Source:   []string{"msg-1"},
				},
			},
		}
		responseJSON, _ := json.Marshal(extractionResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: true}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		// Start parent span to verify nested span propagation
		ctx, parentSpan := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		interaction := InteractionContext{
			ConversationID: "conv-traced",
			UserMessage:    "Test with tracing",
			AssistantReply: "Traced reply",
			Timestamp:      time.Now(),
		}

		result, err := curator.ExtractCells(ctx, interaction)
		if err != nil {
			t.Fatalf("Expected no error with tracing, got: %v", err)
		}

		if result.CellsCreated != 1 {
			t.Errorf("Expected 1 cell created, got %d", result.CellsCreated)
		}

		// Verify tracing is active (parent span has valid trace ID)
		if parentSpan.TraceID == "" {
			t.Error("Expected non-empty trace ID on parent span")
		}
	})

	t.Run("extraction_error_with_tracing", func(t *testing.T) {
		mockLLM := &MockLLMClient{Error: errors.New("LLM API error")}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: true}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		ctx, _ := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		interaction := InteractionContext{
			ConversationID: "conv-error-traced",
			UserMessage:    "Error test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		_, err := curator.ExtractCells(ctx, interaction)
		if err == nil {
			t.Error("Expected error for LLM failure with tracing, got nil")
		}
	})

	t.Run("disabled_extraction_with_tracing", func(t *testing.T) {
		mockLLM := &MockLLMClient{}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: false}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		ctx, _ := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		interaction := InteractionContext{
			ConversationID: "conv-disabled",
			UserMessage:    "Disabled test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		result, err := curator.ExtractCells(ctx, interaction)
		if err != nil {
			t.Fatalf("Expected no error when disabled with tracing, got: %v", err)
		}
		if result.CellsCreated != 0 {
			t.Errorf("Expected 0 cells when disabled, got %d", result.CellsCreated)
		}
	})
}

func TestConsolidateScene_WithTracing(t *testing.T) {
	cleanup := initTestTracing(t)
	defer cleanup()

	t.Run("consolidation_with_tracing", func(t *testing.T) {
		now := time.Now()
		mockCells := []*memoryv2.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-123",
				Scene:          "test-scene",
				CellType:       memoryv2.CellTypeDecision,
				Salience:       0.85,
				Content:        "Test decision",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		consolidationResp := SceneConsolidationResponse{
			Summary:    "Traced scene summary",
			TokenCount: 10,
		}
		responseJSON, _ := json.Marshal(consolidationResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{Cells: mockCells}
		mockSceneRepo := &MockSceneRepository{}

		config := CuratorConfig{Enabled: true, SceneSummaryMaxTokens: 500}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		ctx, _ := tracing.StartSpan(context.Background(), "test.parent")
		defer tracing.EndSpan(ctx)

		err := curator.ConsolidateScene(ctx, "test-scene")
		if err != nil {
			t.Fatalf("Expected no error with tracing, got: %v", err)
		}

		if len(mockSceneRepo.Scenes) != 1 {
			t.Errorf("Expected 1 scene, got %d", len(mockSceneRepo.Scenes))
		}
	})
}

func TestExtractCells_Metrics(t *testing.T) {
	t.Run("successful_extraction_increments_counters", func(t *testing.T) {
		extractionResp := ExtractionResponse{
			Cells: []ExtractedCell{
				{
					Scene:    "metrics-scene",
					CellType: "fact",
					Salience: 0.8,
					Content:  "Fact for metrics test",
					Source:   []string{"msg-1"},
				},
				{
					Scene:    "metrics-scene",
					CellType: "decision",
					Salience: 0.9,
					Content:  "Decision for metrics test",
					Source:   []string{"msg-2"},
				},
			},
		}
		responseJSON, _ := json.Marshal(extractionResp)

		// Record initial metric values
		initialSuccess := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("success"))
		initialCells := testutil.ToFloat64(metrics.MemoryCellsCreatedTotal)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}
		config := CuratorConfig{Enabled: true}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-metrics",
			UserMessage:    "Test metrics",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		result, err := curator.ExtractCells(context.Background(), interaction)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if result.CellsCreated != 2 {
			t.Errorf("Expected 2 cells, got %d", result.CellsCreated)
		}

		// Verify metrics incremented
		newSuccess := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("success"))
		if newSuccess != initialSuccess+1 {
			t.Errorf("Expected extraction success counter to increment by 1, got delta %f", newSuccess-initialSuccess)
		}

		newCells := testutil.ToFloat64(metrics.MemoryCellsCreatedTotal)
		if newCells != initialCells+2 {
			t.Errorf("Expected cells created to increment by 2, got delta %f", newCells-initialCells)
		}
	})

	t.Run("error_increments_error_counter", func(t *testing.T) {
		initialError := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("error"))

		mockLLM := &MockLLMClient{Error: errors.New("LLM failure")}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}
		config := CuratorConfig{Enabled: true}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-err",
			UserMessage:    "Test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		_, err := curator.ExtractCells(context.Background(), interaction)
		if err == nil {
			t.Fatal("Expected error")
		}

		newError := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("error"))
		if newError != initialError+1 {
			t.Errorf("Expected error counter to increment by 1, got delta %f", newError-initialError)
		}
	})

	t.Run("disabled_increments_skipped_counter", func(t *testing.T) {
		initialSkipped := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("skipped"))

		mockLLM := &MockLLMClient{}
		mockCellRepo := &MockCellRepository{}
		mockSceneRepo := &MockSceneRepository{}
		config := CuratorConfig{Enabled: false}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		interaction := InteractionContext{
			ConversationID: "conv-skip",
			UserMessage:    "Test",
			AssistantReply: "Reply",
			Timestamp:      time.Now(),
		}

		_, err := curator.ExtractCells(context.Background(), interaction)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		newSkipped := testutil.ToFloat64(metrics.MemoryExtractionTotal.WithLabelValues("skipped"))
		if newSkipped != initialSkipped+1 {
			t.Errorf("Expected skipped counter to increment by 1, got delta %f", newSkipped-initialSkipped)
		}
	})
}

func TestConsolidateScene_Metrics(t *testing.T) {
	t.Run("successful_consolidation_increments_counter", func(t *testing.T) {
		initialSuccess := testutil.ToFloat64(metrics.MemoryConsolidationTotal.WithLabelValues("success"))

		now := time.Now()
		mockCells := []*memoryv2.MemoryCell{
			{
				ID: "cell-m1", ConversationID: "conv-123", Scene: "metrics-scene",
				CellType: memoryv2.CellTypeFact, Salience: 0.8, Content: "Fact",
				Source: `["msg-1"]`, CreatedAt: now, UpdatedAt: now,
			},
		}

		consolidationResp := SceneConsolidationResponse{Summary: "Summary", TokenCount: 5}
		responseJSON, _ := json.Marshal(consolidationResp)

		mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
		mockCellRepo := &MockCellRepository{Cells: mockCells}
		mockSceneRepo := &MockSceneRepository{}
		config := CuratorConfig{Enabled: true, SceneSummaryMaxTokens: 500}
		curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

		err := curator.ConsolidateScene(context.Background(), "metrics-scene")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		newSuccess := testutil.ToFloat64(metrics.MemoryConsolidationTotal.WithLabelValues("success"))
		if newSuccess != initialSuccess+1 {
			t.Errorf("Expected consolidation success counter to increment by 1, got delta %f", newSuccess-initialSuccess)
		}
	})
}

func TestMemoryCuratorService_Deduplication(t *testing.T) {
	cleanup := initTestTracing(t)
	defer cleanup()

	// Existing cell in the scene
	now := time.Now()
	existingCell := &memoryv2.MemoryCell{
		ID:             "existing-cell-1",
		ConversationID: "conv-prev",
		Scene:          "project-setup",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        "User prefers tabs over spaces",
		Source:         `["msg-old"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	mockCellRepo := &MockCellRepository{Cells: []*memoryv2.MemoryCell{existingCell}}
	mockSceneRepo := &MockSceneRepository{}

	// LLM returns a cell with the same content but higher salience
	responseJSON := `{"cells":[{"scene":"project-setup","cell_type":"fact","salience":0.9,"content":"User prefers tabs over spaces","source":["msg-1"]}]}`
	mockLLM := &MockLLMClient{ResponseJSON: responseJSON}

	curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, CuratorConfig{Enabled: true})

	result, err := curator.ExtractCells(context.Background(), InteractionContext{
		ConversationID: "conv-123",
		UserMessage:    "What do you prefer for indentation?",
		AssistantReply: "Tabs.",
		Timestamp:      now,
	})
	if err != nil {
		t.Fatalf("ExtractCells: %v", err)
	}

	// The duplicate should be skipped (not created)
	if result.CellsCreated != 0 {
		t.Errorf("expected 0 cells created (all duplicates), got %d", result.CellsCreated)
	}
	if result.CellsSkipped != 1 {
		t.Errorf("expected 1 cell skipped, got %d", result.CellsSkipped)
	}

	// Existing cell's salience should be updated to 0.9
	if existingCell.Salience != 0.9 {
		t.Errorf("expected existing cell salience updated to 0.9, got %f", existingCell.Salience)
	}

	// No new cells should have been added to the repository
	if len(mockCellRepo.Cells) != 1 {
		t.Errorf("expected 1 cell in repo (no new cells), got %d", len(mockCellRepo.Cells))
	}
}

func TestNormalizeContent(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"  spaces  around  ", "spaces around"},
		{"tabs\there", "tabs here"},
		{"newline\nhere", "newline here"},
		{"multiple   spaces", "multiple spaces"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeContent(c.input)
		if got != c.expected {
			t.Errorf("normalizeContent(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}
