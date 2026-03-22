package memoryv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// MockDomainLLMService is a mock for domain.LLMService
type MockDomainLLMService struct {
	CompleteFunc func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
}

func (m *MockDomainLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, provider, req)
	}
	return &domain.LLMResponse{Content: `{"test": true}`}, nil
}

func (m *MockDomainLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *MockDomainLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	return nil, nil
}

// Tests for truncate function
func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"empty string", "", 5, ""},
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly limit", "hello", 5, "hello"},
		{"longer than limit", "hello world", 5, "hello..."},
		{"zero limit", "hello", 0, "..."},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

// Tests for LLMServiceAdapter
func TestLLMServiceAdapter_NewAndGenerateJSON(t *testing.T) {
	t.Parallel()

	t.Run("successful_call", func(t *testing.T) {
		t.Parallel()
		mockSvc := &MockDomainLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: `{"result": "ok"}`}, nil
			},
		}
		adapter := NewLLMServiceAdapter(mockSvc, domain.LLMProviderAnthropic, "claude-3-haiku")
		if adapter == nil {
			t.Fatal("NewLLMServiceAdapter returned nil")
		}

		result, err := adapter.GenerateJSON(context.Background(), "system", "user", map[string]string{"key": "val"})
		if err != nil {
			t.Fatalf("GenerateJSON failed: %v", err)
		}
		if result == "" {
			t.Error("Expected non-empty result")
		}
	})

	t.Run("llm_error", func(t *testing.T) {
		t.Parallel()
		mockSvc := &MockDomainLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return nil, errors.New("LLM unavailable")
			},
		}
		adapter := NewLLMServiceAdapter(mockSvc, domain.LLMProviderAnthropic, "claude-3-haiku")
		_, err := adapter.GenerateJSON(context.Background(), "system", "user", nil)
		if err == nil {
			t.Error("Expected error when LLM fails")
		}
	})

	t.Run("empty_response", func(t *testing.T) {
		t.Parallel()
		mockSvc := &MockDomainLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: ""}, nil
			},
		}
		adapter := NewLLMServiceAdapter(mockSvc, domain.LLMProviderAnthropic, "claude-3-haiku")
		_, err := adapter.GenerateJSON(context.Background(), "system", "user", nil)
		if err == nil {
			t.Error("Expected error for empty response")
		}
	})

	t.Run("invalid_json_response", func(t *testing.T) {
		t.Parallel()
		mockSvc := &MockDomainLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "not valid json"}, nil
			},
		}
		adapter := NewLLMServiceAdapter(mockSvc, domain.LLMProviderAnthropic, "claude-3-haiku")
		_, err := adapter.GenerateJSON(context.Background(), "system", "user", nil)
		if err == nil {
			t.Error("Expected error for invalid JSON response")
		}
	})
}

// Tests for memory_alerts.go alert functions
func TestAlertFunctions(t *testing.T) {
	// These functions use slog and alerting.SendAlert (fire-and-forget).
	// We just verify they don't panic.

	ctx := context.Background()

	t.Run("alertExtractionFailed_does_not_panic", func(t *testing.T) {
		alertExtractionFailed(ctx, "conv-123", errors.New("test error"))
	})

	t.Run("alertConsolidationFailed_does_not_panic", func(t *testing.T) {
		alertConsolidationFailed(ctx, "scene-test", errors.New("test error"))
	})

	t.Run("alertSlowRecall_does_not_panic", func(t *testing.T) {
		alertSlowRecall(ctx, "conv-123", 200*time.Millisecond)
	})

	t.Run("logSlowFTSQuery_does_not_panic", func(t *testing.T) {
		logSlowFTSQuery(ctx, "test query", 100*time.Millisecond)
	})
}

// Tests for FormatMemoryForInjection
func TestFormatMemoryForInjection(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mockCellRepo := &MockFTSCellRepository{}
	mockSceneRepo := &MockSceneRepository{}
	config := RecallConfig{FTSResultLimit: 10, SalienceThreshold: 0.8, TokenBudget: 500}
	recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

	t.Run("empty_response_returns_empty_string", func(t *testing.T) {
		t.Parallel()
		response := &RecallResponse{
			Cells:  []*memoryv2.MemoryCell{},
			Scenes: []*memoryv2.MemoryScene{},
		}
		result := recall.FormatMemoryForInjection(response)
		if result != "" {
			t.Errorf("Expected empty string for empty response, got: %q", result)
		}
	})

	t.Run("formats_cells_and_scenes", func(t *testing.T) {
		t.Parallel()
		response := &RecallResponse{
			Cells: []*memoryv2.MemoryCell{
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
			},
			Scenes: []*memoryv2.MemoryScene{
				{
					Scene:      "test-scene",
					Summary:    "Summary of test scene",
					TokenCount: 10,
					UpdatedAt:  now,
				},
			},
			TotalTokens: 20,
		}
		result := recall.FormatMemoryForInjection(response)
		if result == "" {
			t.Error("Expected non-empty result")
		}
		if len(result) == 0 {
			t.Error("Expected formatted output")
		}
		// Should contain scene name and summary
		if !containsString(result, "test-scene") {
			t.Error("Expected scene name in output")
		}
		if !containsString(result, "Summary of test scene") {
			t.Error("Expected scene summary in output")
		}
		if !containsString(result, "Important fact") {
			t.Error("Expected cell content in output")
		}
	})

	t.Run("only_scenes_no_cells", func(t *testing.T) {
		t.Parallel()
		response := &RecallResponse{
			Cells: []*memoryv2.MemoryCell{},
			Scenes: []*memoryv2.MemoryScene{
				{
					Scene:      "lone-scene",
					Summary:    "Lone scene summary",
					TokenCount: 5,
					UpdatedAt:  now,
				},
			},
			TotalTokens: 5,
		}
		result := recall.FormatMemoryForInjection(response)
		if result == "" {
			t.Error("Expected non-empty result with scenes but no cells")
		}
	})
}

// containsString checks if haystack contains needle
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		func() bool {
			for i := 0; i <= len(haystack)-len(needle); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}())
}

// Tests for getScenes with MaxScenes default
func TestGetScenes_DefaultMaxScenes(t *testing.T) {
	t.Parallel()
	now := time.Now()

	// Create many cells from different scenes
	cells := make([]*memoryv2.MemoryCell, 15)
	scenes := make([]*memoryv2.MemoryScene, 15)
	for i := 0; i < 15; i++ {
		sceneName := "scene-" + string(rune('a'+i))
		cells[i] = &memoryv2.MemoryCell{
			ID:             "cell-" + string(rune('a'+i)),
			ConversationID: "conv-123",
			Scene:          sceneName,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.9,
			Content:        "cell content",
			Source:         `["msg-1"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		scenes[i] = &memoryv2.MemoryScene{
			Scene:      sceneName,
			Summary:    "scene summary",
			TokenCount: 5,
			UpdatedAt:  now,
		}
	}

	mockCellRepo := &MockFTSCellRepository{}
	mockSceneRepo := &MockSceneRepository{Scenes: scenes}

	// MaxScenes = 0 triggers the default of 10
	config := RecallConfig{MaxScenes: 0}
	recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

	retrievedScenes, _ := recall.getScenes(context.Background(), cells)
	if len(retrievedScenes) > 10 {
		t.Errorf("Expected at most 10 scenes with default MaxScenes, got %d", len(retrievedScenes))
	}
}

// Tests for buildExtractionUserPrompt
func TestBuildExtractionUserPrompt(t *testing.T) {
	t.Parallel()

	mockLLM := &MockLLMClient{}
	mockCellRepo := &MockCellRepository{}
	mockSceneRepo := &MockSceneRepository{}
	config := CuratorConfig{Enabled: true}
	curator := NewMemoryCuratorService(mockLLM, mockCellRepo, mockSceneRepo, config)

	t.Run("without_tool_outputs", func(t *testing.T) {
		t.Parallel()
		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "Hello",
			AssistantReply: "Hi there",
			ToolOutputs:    nil,
		}
		prompt := curator.buildExtractionUserPrompt(interaction)
		if !containsString(prompt, "Hello") {
			t.Error("Expected user message in prompt")
		}
		if !containsString(prompt, "Hi there") {
			t.Error("Expected assistant reply in prompt")
		}
	})

	t.Run("with_tool_outputs", func(t *testing.T) {
		t.Parallel()
		interaction := InteractionContext{
			ConversationID: "conv-123",
			UserMessage:    "What time is it?",
			AssistantReply: "It is 3pm",
			ToolOutputs:    []string{"tool result 1", "tool result 2"},
		}
		prompt := curator.buildExtractionUserPrompt(interaction)
		if !containsString(prompt, "Tool Outputs") {
			t.Error("Expected tool outputs section in prompt")
		}
		if !containsString(prompt, "tool result 1") {
			t.Error("Expected first tool output in prompt")
		}
	})
}

// Tests for FTS recall error paths
func TestRecallMemory_ErrorPaths(t *testing.T) {
	t.Run("fts_search_error", func(t *testing.T) {
		mockCellRepo := &MockFTSCellRepositoryWithErrors{
			FTSError: errors.New("FTS search failed"),
		}
		mockSceneRepo := &MockSceneRepository{}

		config := RecallConfig{FTSResultLimit: 10, SalienceThreshold: 0.8, TokenBudget: 500}
		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "test query",
			MaxTokens:      500,
			MaxCells:       10,
		}

		_, err := recall.RecallMemory(context.Background(), request)
		if err == nil {
			t.Error("Expected error when FTS search fails")
		}
	})

	t.Run("fallback_salience_error", func(t *testing.T) {
		mockCellRepo := &MockFTSCellRepositoryWithErrors{
			FTSResults:    []*memoryv2.MemoryCell{}, // empty FTS
			SalienceError: errors.New("salience search failed"),
		}
		mockSceneRepo := &MockSceneRepository{}

		config := RecallConfig{FTSResultLimit: 10, SalienceThreshold: 0.8, TokenBudget: 500}
		recall := NewMemoryRecallService(mockCellRepo, mockSceneRepo, config)

		request := RecallRequest{
			ConversationID: "conv-123",
			Query:          "test query",
			MaxTokens:      500,
			MaxCells:       10,
		}

		_, err := recall.RecallMemory(context.Background(), request)
		if err == nil {
			t.Error("Expected error when fallback search fails")
		}
	})
}

// MockFTSCellRepositoryWithErrors allows testing error paths
type MockFTSCellRepositoryWithErrors struct {
	FTSResults    []*memoryv2.MemoryCell
	FTSError      error
	SalienceError error
}

func (m *MockFTSCellRepositoryWithErrors) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	if m.FTSError != nil {
		return nil, m.FTSError
	}
	return m.FTSResults, nil
}

func (m *MockFTSCellRepositoryWithErrors) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	if m.SalienceError != nil {
		return nil, m.SalienceError
	}
	return []*memoryv2.MemoryCell{}, nil
}

func (m *MockFTSCellRepositoryWithErrors) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	return nil
}

func (m *MockFTSCellRepositoryWithErrors) Get(ctx context.Context, id string) (*memoryv2.MemoryCell, error) {
	return nil, memoryv2.ErrNotFound
}

func (m *MockFTSCellRepositoryWithErrors) Update(ctx context.Context, cell *memoryv2.MemoryCell) error {
	return nil
}

func (m *MockFTSCellRepositoryWithErrors) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockFTSCellRepositoryWithErrors) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *MockFTSCellRepositoryWithErrors) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *MockFTSCellRepositoryWithErrors) DeleteExpired(ctx context.Context) (int, error) {
	return 0, nil
}
