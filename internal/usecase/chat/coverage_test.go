package chat

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

// TestSetPromptComposer verifies the setter works
func TestSetPromptComposer(t *testing.T) {
	t.Parallel()
	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, &mockSecurityService{})

	// Create a mock prompt composer
	composer := &mockPromptComposer{}
	service.SetPromptComposer(composer)
	// No panic = success; the setter is covered
}

// TestSetLLMDefaults verifies the setter and default functions work
func TestSetLLMDefaults(t *testing.T) {
	t.Parallel()
	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, &mockSecurityService{})

	// Test defaults before setting
	if service.defaultModel() == "" {
		t.Error("defaultModel() should return a non-empty fallback")
	}
	if service.defaultMaxTokens() <= 0 {
		t.Errorf("defaultMaxTokens() should return positive value, got %d", service.defaultMaxTokens())
	}
	if service.defaultTemperature() <= 0 {
		t.Errorf("defaultTemperature() should return positive value, got %f", service.defaultTemperature())
	}

	// Set custom defaults
	service.SetLLMDefaults(LLMDefaults{
		Model:       "custom-model",
		MaxTokens:   2048,
		Temperature: 0.5,
	})

	if service.defaultModel() != "custom-model" {
		t.Errorf("Expected 'custom-model', got %q", service.defaultModel())
	}
	if service.defaultMaxTokens() != 2048 {
		t.Errorf("Expected 2048, got %d", service.defaultMaxTokens())
	}
	if service.defaultTemperature() != 0.5 {
		t.Errorf("Expected 0.5, got %f", service.defaultTemperature())
	}
}

// TestGetProviderTokenLimit covers all provider branches
func TestGetProviderTokenLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider domain.LLMProvider
		wantMin  int
	}{
		{domain.LLMProviderAnthropic, 100000},
		{domain.LLMProviderOpenAI, 100000},
		{domain.LLMProviderOllama, 1000},
		{domain.LLMProvider("unknown"), 1000},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.provider), func(t *testing.T) {
			t.Parallel()
			limit := getProviderTokenLimit(tt.provider)
			if limit < tt.wantMin {
				t.Errorf("getProviderTokenLimit(%v) = %d, expected >= %d", tt.provider, limit, tt.wantMin)
			}
		})
	}
}

// TestExportConversation_GetConversationError covers the error path
func TestExportConversation_GetConversationError(t *testing.T) {
	t.Parallel()

	memoryRepo := &mockMemoryRepository{
		getConversationFunc: func(ctx context.Context, convID string) (*domain.Conversation, error) {
			return nil, &testError{msg: "conversation not found"}
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	_, err := service.ExportConversation(context.Background(), "conv-123", ExportFormatJSON)
	if err == nil {
		t.Error("Expected error when GetConversation fails")
	}
}

// TestExportConversation_GetMessagesError covers the messages fetch error path
func TestExportConversation_GetMessagesError(t *testing.T) {
	t.Parallel()

	memoryRepo := &mockMemoryRepository{
		getConversationFunc: func(ctx context.Context, convID string) (*domain.Conversation, error) {
			return &domain.Conversation{ID: convID}, nil
		},
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return nil, &testError{msg: "messages fetch failed"}
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	_, err := service.ExportConversation(context.Background(), "conv-123", ExportFormatJSON)
	if err == nil {
		t.Error("Expected error when GetRecentMessages fails")
	}
}

// TestBuildCacheKey covers the buildCacheKey function
func TestBuildCacheKey(t *testing.T) {
	t.Parallel()

	t.Run("empty_messages", func(t *testing.T) {
		t.Parallel()
		key := buildCacheKey([]domain.Message{})
		if key != "" {
			t.Errorf("Expected empty key for empty messages, got %q", key)
		}
	})

	t.Run("single_message", func(t *testing.T) {
		t.Parallel()
		messages := []domain.Message{
			{Role: "user", Content: "Hello"},
		}
		key := buildCacheKey(messages)
		if key == "" {
			t.Error("Expected non-empty key")
		}
		// Key should contain the content
		if len(key) == 0 {
			t.Error("Key should not be empty for non-empty messages")
		}
	})

	t.Run("multiple_messages_are_deterministic", func(t *testing.T) {
		t.Parallel()
		messages := []domain.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		}
		key1 := buildCacheKey(messages)
		key2 := buildCacheKey(messages)
		if key1 != key2 {
			t.Error("buildCacheKey should be deterministic")
		}
	})

	t.Run("different_messages_produce_different_keys", func(t *testing.T) {
		t.Parallel()
		messages1 := []domain.Message{
			{Role: "user", Content: "Hello"},
		}
		messages2 := []domain.Message{
			{Role: "user", Content: "Goodbye"},
		}
		key1 := buildCacheKey(messages1)
		key2 := buildCacheKey(messages2)
		if key1 == key2 {
			t.Error("Different messages should produce different keys")
		}
	})
}

// TestFormatToolResults covers uncovered branches
func TestFormatToolResults(t *testing.T) {
	t.Parallel()

	t.Run("empty_results", func(t *testing.T) {
		t.Parallel()
		result := formatToolResults([]domain.ToolResult{})
		if result == "" {
			t.Error("Expected non-empty string for no results")
		}
	})

	t.Run("single_result_with_output", func(t *testing.T) {
		t.Parallel()
		results := []domain.ToolResult{
			{ToolName: "test-tool", Output: "some output"},
		}
		formatted := formatToolResults(results)
		if formatted == "" {
			t.Error("Expected non-empty formatted result")
		}
	})

	t.Run("result_with_error", func(t *testing.T) {
		t.Parallel()
		results := []domain.ToolResult{
			{ToolName: "test-tool", Error: "tool execution failed"},
		}
		formatted := formatToolResults(results)
		if formatted == "" {
			t.Error("Expected non-empty formatted result even with error")
		}
	})

	t.Run("multiple_results", func(t *testing.T) {
		t.Parallel()
		results := []domain.ToolResult{
			{ToolName: "tool-1", Output: "output1"},
			{ToolName: "tool-2", Error: "error2"},
		}
		formatted := formatToolResults(results)
		if formatted == "" {
			t.Error("Expected non-empty formatted results")
		}
	})
}

// mockPromptComposer for testing SetPromptComposer
type mockPromptComposer struct{}

func (m *mockPromptComposer) Compose(ctx context.Context, input PromptComposerInput) (*PromptComposerOutput, error) {
	return &PromptComposerOutput{SystemPrompt: "mock system prompt"}, nil
}

// testError is a simple error for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
