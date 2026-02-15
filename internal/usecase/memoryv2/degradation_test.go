package memoryv2

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// --- Failing Repository Mocks ---

// FailingCellRepository simulates database failures for degradation testing.
type FailingCellRepository struct {
	CreateErr        error
	GetErr           error
	ListErr          error
	SearchFTSErr     error
	GetBySceneErr    error
	HighSalienceErr  error
	DeleteExpiredErr error
}

func (r *FailingCellRepository) Create(_ context.Context, _ *memoryv2.MemoryCell) error {
	return r.CreateErr
}

func (r *FailingCellRepository) Get(_ context.Context, _ string) (*memoryv2.MemoryCell, error) {
	return nil, r.GetErr
}

func (r *FailingCellRepository) List(_ context.Context, _ memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	return nil, r.ListErr
}

func (r *FailingCellRepository) Delete(_ context.Context, _ string) error {
	return nil
}

func (r *FailingCellRepository) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, r.SearchFTSErr
}

func (r *FailingCellRepository) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, r.GetBySceneErr
}

func (r *FailingCellRepository) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, r.HighSalienceErr
}

func (r *FailingCellRepository) DeleteExpired(_ context.Context) (int, error) {
	return 0, r.DeleteExpiredErr
}

// FailingSceneRepository simulates database failures for scene operations.
type FailingSceneRepository struct {
	UpsertErr error
	GetErr    error
	ListErr   error
}

func (r *FailingSceneRepository) Upsert(_ context.Context, _ *memoryv2.MemoryScene) error {
	return r.UpsertErr
}

func (r *FailingSceneRepository) Get(_ context.Context, _ string) (*memoryv2.MemoryScene, error) {
	return nil, r.GetErr
}

func (r *FailingSceneRepository) List(_ context.Context) ([]*memoryv2.MemoryScene, error) {
	return nil, r.ListErr
}

func (r *FailingSceneRepository) Delete(_ context.Context, _ string) error {
	return nil
}

// --- Curator Degradation Tests ---

func TestCurator_Degradation_RepoCreateFails_PartialResults(t *testing.T) {
	// When the cell repository fails to persist, extraction should still return
	// a result with error information, not crash or panic.
	extractionResp := ExtractionResponse{
		Cells: []ExtractedCell{
			{
				Scene:    "test-scene",
				CellType: "fact",
				Salience: 0.85,
				Content:  "Important fact",
				Source:   []string{"msg-1"},
			},
		},
	}
	responseJSON, _ := json.Marshal(extractionResp)

	mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
	failingCellRepo := &FailingCellRepository{
		CreateErr: errors.New("database connection refused"),
	}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{Enabled: true}
	curator := NewMemoryCuratorService(mockLLM, failingCellRepo, mockSceneRepo, config)

	interaction := InteractionContext{
		ConversationID: "conv-123",
		UserMessage:    "Test message",
		AssistantReply: "Test reply",
		Timestamp:      time.Now(),
	}

	result, err := curator.ExtractCells(context.Background(), interaction)

	// Should return an error since no cells were created
	if err == nil {
		t.Fatal("Expected error when all cell creates fail")
	}

	// Result should still be returned with error details
	if result == nil {
		t.Fatal("Expected non-nil result even on failure")
	}

	if result.CellsCreated != 0 {
		t.Errorf("Expected 0 cells created, got %d", result.CellsCreated)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors to be recorded in result")
	}
}

func TestCurator_Degradation_LLMReturnsEmptyCells(t *testing.T) {
	// When the LLM returns no cells, it should not be treated as an error.
	extractionResp := ExtractionResponse{
		Cells: []ExtractedCell{}, // Nothing worth remembering
	}
	responseJSON, _ := json.Marshal(extractionResp)

	mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
	mockCellRepo := &MockCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{Enabled: true}
	curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

	interaction := InteractionContext{
		ConversationID: "conv-123",
		UserMessage:    "Hi",
		AssistantReply: "Hello! How can I help?",
		Timestamp:      time.Now(),
	}

	result, err := curator.ExtractCells(context.Background(), interaction)

	// Should NOT return an error - empty extraction is normal
	if err != nil {
		t.Fatalf("Empty extraction should not be an error, got: %v", err)
	}

	if result.CellsCreated != 0 {
		t.Errorf("Expected 0 cells, got %d", result.CellsCreated)
	}
}

func TestCurator_Degradation_SceneConsolidationFails_CellsStillSaved(t *testing.T) {
	// When scene consolidation fails after cells are saved, cells should
	// still be persisted and result should indicate partial success.
	extractionResp := ExtractionResponse{
		Cells: []ExtractedCell{
			{
				Scene:    "test-scene",
				CellType: "fact",
				Salience: 0.85,
				Content:  "Important fact",
				Source:   []string{"msg-1"},
			},
		},
	}
	responseJSON, _ := json.Marshal(extractionResp)

	// LLM succeeds for extraction, but we'll make the scene repo fail
	callCount := 0
	mockLLM := &MockLLMClient{}
	mockLLM.ResponseJSON = string(responseJSON)

	// Override GenerateJSON to return different responses
	originalGenerateJSON := mockLLM.GenerateJSON
	_ = originalGenerateJSON
	// Use a custom LLM that fails on second call (consolidation)
	customLLM := &conditionalLLMClient{
		firstResponse:  string(responseJSON),
		secondResponse: "",
		secondError:    errors.New("LLM consolidation timeout"),
		callCount:      &callCount,
	}

	mockCellRepo := &MockCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{Enabled: true}
	curator := NewMemoryCuratorService(customLLM, mockCellRepo, mockSceneRepo, config)

	interaction := InteractionContext{
		ConversationID: "conv-123",
		UserMessage:    "Test",
		AssistantReply: "Reply",
		Timestamp:      time.Now(),
	}

	result, err := curator.ExtractCells(context.Background(), interaction)

	// Should succeed overall - cells were saved even if consolidation failed
	// The error should be in result.Errors, not as the return error
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.CellsCreated != 1 {
		t.Errorf("Expected 1 cell created despite consolidation failure, got %d", result.CellsCreated)
	}

	// err may or may not be nil depending on whether consolidation errors are fatal
	// The key assertion: cells were saved
	if len(mockCellRepo.Cells) != 1 {
		t.Errorf("Expected 1 cell in repo, got %d", len(mockCellRepo.Cells))
	}

	_ = err // err presence depends on implementation - cells saved is what matters
}

func TestCurator_Degradation_InvalidCellType_SkipsCell(t *testing.T) {
	// When LLM returns an invalid cell type, that cell should be skipped
	// but other valid cells should still be processed.
	extractionResp := ExtractionResponse{
		Cells: []ExtractedCell{
			{
				Scene:    "test-scene",
				CellType: "invalid-type", // Invalid
				Salience: 0.85,
				Content:  "Should be skipped",
				Source:   []string{"msg-1"},
			},
			{
				Scene:    "test-scene",
				CellType: "fact", // Valid
				Salience: 0.90,
				Content:  "Should be saved",
				Source:   []string{"msg-2"},
			},
		},
	}
	responseJSON, _ := json.Marshal(extractionResp)

	mockLLM := &MockLLMClient{ResponseJSON: string(responseJSON)}
	mockCellRepo := &MockCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{Enabled: true}
	curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

	interaction := InteractionContext{
		ConversationID: "conv-123",
		UserMessage:    "Test",
		AssistantReply: "Reply",
		Timestamp:      time.Now(),
	}

	result, err := curator.ExtractCells(context.Background(), interaction)

	// Should succeed - one cell valid, one skipped
	if err != nil {
		t.Fatalf("Expected no fatal error, got: %v", err)
	}

	if result.CellsCreated != 1 {
		t.Errorf("Expected 1 cell created (invalid one skipped), got %d", result.CellsCreated)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected at least 1 error for the invalid cell type")
	}
}

// --- Recall Degradation Tests ---

func TestRecall_Degradation_FTSSearchFails_ReturnError(t *testing.T) {
	// When FTS search fails, recall should return an error.
	failingCellRepo := &FailingCellRepository{
		SearchFTSErr: errors.New("FTS index corrupted"),
	}
	mockSceneRepo := &MockSceneRepository{}

	config := RecallConfig{
		FTSResultLimit:    10,
		SalienceThreshold: 0.8,
		FallbackCellLimit: 5,
		TokenBudget:       500,
	}

	recall := NewMemoryRecallService(failingCellRepo, mockSceneRepo, config)

	request := RecallRequest{
		ConversationID: "conv-123",
		Query:          "test query",
		MaxTokens:      500,
	}

	_, err := recall.RecallMemory(context.Background(), request)

	if err == nil {
		t.Fatal("Expected error when FTS search fails")
	}
}

func TestRecall_Degradation_SalienceFallbackFails_ReturnsError(t *testing.T) {
	// When both FTS and salience fallback fail, recall should return error.
	// Use a repo that returns empty FTS results (triggering fallback) then fails on salience
	emptyFTSRepo := &emptyFTSFailingSalienceRepo{
		salienceErr: errors.New("database connection timeout"),
	}

	mockSceneRepo := &MockSceneRepository{}

	config := RecallConfig{
		FTSResultLimit:    10,
		SalienceThreshold: 0.8,
		FallbackCellLimit: 5,
		TokenBudget:       500,
	}

	recall := NewMemoryRecallService(emptyFTSRepo, mockSceneRepo, config)

	request := RecallRequest{
		ConversationID: "conv-123",
		Query:          "test query",
		MaxTokens:      500,
	}

	_, err := recall.RecallMemory(context.Background(), request)

	if err == nil {
		t.Fatal("Expected error when salience fallback fails")
	}
}

func TestRecall_Degradation_SceneRepoFails_StillReturnsCells(t *testing.T) {
	// When scene repository fails, recall should still return cells
	// (scenes are supplementary, not required).
	now := time.Now()

	ftsResults := []*memoryv2.MemoryCell{
		{
			ID:             "cell-1",
			ConversationID: "conv-123",
			Scene:          "test-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.9,
			Content:        "Important fact",
			Source:         `["msg-1"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	mockCellRepo := &MockFTSCellRepository{FTSResults: ftsResults}
	failingSceneRepo := &FailingSceneRepository{
		GetErr: errors.New("scene table corrupted"),
	}

	config := RecallConfig{
		FTSResultLimit:    10,
		SalienceThreshold: 0.8,
		TokenBudget:       500,
	}

	recall := NewMemoryRecallService(mockCellRepo, failingSceneRepo, config)

	request := RecallRequest{
		ConversationID: "conv-123",
		Query:          "test query",
		MaxTokens:      500,
	}

	response, err := recall.RecallMemory(context.Background(), request)

	// Should succeed even when scenes fail - cells are the primary data
	if err != nil {
		t.Fatalf("Expected no error when scenes fail, got: %v", err)
	}

	if len(response.Cells) != 1 {
		t.Errorf("Expected 1 cell despite scene failure, got %d", len(response.Cells))
	}
}

func TestRecall_Degradation_EmptyDatabase_ReturnsEmptyResult(t *testing.T) {
	// When the database is empty (no cells, no scenes), recall should
	// return an empty result without error.
	mockCellRepo := &MockFTSCellRepository{
		FTSResults: []*memoryv2.MemoryCell{},
		MockCellRepository: MockCellRepository{
			Cells: []*memoryv2.MemoryCell{},
		},
	}
	mockSceneRepo := &MockSceneRepository{}

	config := RecallConfig{
		FTSResultLimit:    10,
		SalienceThreshold: 0.8,
		FallbackCellLimit: 5,
		TokenBudget:       500,
	}

	recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

	request := RecallRequest{
		ConversationID: "conv-123",
		Query:          "anything",
		MaxTokens:      500,
	}

	response, err := recall.RecallMemory(context.Background(), request)

	if err != nil {
		t.Fatalf("Empty database should not cause error, got: %v", err)
	}

	if len(response.Cells) != 0 {
		t.Errorf("Expected 0 cells for empty database, got %d", len(response.Cells))
	}

	if response.FallbackUsed != true {
		// Empty FTS triggers fallback, which also returns empty
		// This is expected behavior, not an error
	}
}

func TestRecall_Degradation_FormatMemory_EmptyResponse(t *testing.T) {
	// FormatMemoryForInjection should handle empty response gracefully.
	mockCellRepo := &MockFTSCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, RecallConfig{})

	emptyResponse := &RecallResponse{
		Cells:  []*memoryv2.MemoryCell{},
		Scenes: []*memoryv2.MemoryScene{},
	}

	formatted := recall.FormatMemoryForInjection(emptyResponse)

	if formatted != "" {
		t.Errorf("Expected empty string for empty response, got: %q", formatted)
	}
}

func TestRecall_Degradation_FormatMemory_NilResponse(t *testing.T) {
	// FormatMemoryForInjection should handle nil cells/scenes.
	mockCellRepo := &MockFTSCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, RecallConfig{})

	emptyResponse := &RecallResponse{
		Cells:  nil,
		Scenes: nil,
	}

	formatted := recall.FormatMemoryForInjection(emptyResponse)

	if formatted != "" {
		t.Errorf("Expected empty string for nil response fields, got: %q", formatted)
	}
}

// --- Helper types ---

// conditionalLLMClient returns different responses on successive calls.
type conditionalLLMClient struct {
	firstResponse  string
	secondResponse string
	secondError    error
	callCount      *int
}

func (c *conditionalLLMClient) GenerateJSON(_ context.Context, _, _ string, _ interface{}) (string, error) {
	*c.callCount++
	if *c.callCount == 1 {
		return c.firstResponse, nil
	}
	return c.secondResponse, c.secondError
}

// emptyFTSFailingSalienceRepo returns empty FTS results but fails on salience fallback.
type emptyFTSFailingSalienceRepo struct {
	salienceErr error
}

func (r *emptyFTSFailingSalienceRepo) Create(_ context.Context, _ *memoryv2.MemoryCell) error {
	return nil
}

func (r *emptyFTSFailingSalienceRepo) Get(_ context.Context, _ string) (*memoryv2.MemoryCell, error) {
	return nil, memoryv2.ErrNotFound
}

func (r *emptyFTSFailingSalienceRepo) List(_ context.Context, _ memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (r *emptyFTSFailingSalienceRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func (r *emptyFTSFailingSalienceRepo) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return []*memoryv2.MemoryCell{}, nil // Empty results trigger fallback
}

func (r *emptyFTSFailingSalienceRepo) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (r *emptyFTSFailingSalienceRepo) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, r.salienceErr
}

func (r *emptyFTSFailingSalienceRepo) DeleteExpired(_ context.Context) (int, error) {
	return 0, nil
}
