package chat

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// TestBuildContextWindow_FitsInWindow tests that all messages fit in context window
func TestBuildContextWindow_FitsInWindow(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
				{ID: "msg2", Role: "assistant", Content: "Hi there!", TokenCount: 15},
				{ID: "msg3", Role: "user", Content: "How are you?", TokenCount: 20},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages in context, got %d", len(messages))
	}

	if totalTokens != 45 {
		t.Errorf("Expected 45 total tokens, got %d", totalTokens)
	}
}

// TestBuildContextWindow_ExceedsLimit tests that old messages are dropped when limit exceeded
func TestBuildContextWindow_ExceedsLimit(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Old message 1", TokenCount: 100},
				{ID: "msg2", Role: "assistant", Content: "Old response 1", TokenCount: 150},
				{ID: "msg3", Role: "user", Content: "Old message 2", TokenCount: 200},
				{ID: "msg4", Role: "assistant", Content: "Old response 2", TokenCount: 250},
				{ID: "msg5", Role: "user", Content: "Recent message", TokenCount: 50},
				{ID: "msg6", Role: "assistant", Content: "Recent response", TokenCount: 75},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	// Limit to 400 tokens (should drop oldest messages)
	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 400)

	// Should include only recent messages that fit
	if len(messages) > 3 {
		t.Errorf("Expected at most 3 messages within limit, got %d", len(messages))
	}

	if totalTokens > 400 {
		t.Errorf("Expected tokens <= 400, got %d", totalTokens)
	}

	// Most recent message should always be included
	if len(messages) > 0 && messages[len(messages)-1].Content != "Recent response" {
		t.Error("Expected most recent message to be included")
	}
}

// TestBuildContextWindow_ProviderLimits tests different provider token limits
func TestBuildContextWindow_ProviderLimits(t *testing.T) {
	tests := []struct {
		name          string
		provider      domain.LLMProvider
		expectedLimit int
	}{
		{"Anthropic Claude", domain.LLMProviderAnthropic, 200000},
		{"OpenAI GPT-4", domain.LLMProviderOpenAI, 128000},
		{"Ollama", domain.LLMProviderOllama, 32000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := createTestService(&mockLLMService{}, &mockMemoryRepository{
				getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
					// Verify correct limit is used
					if maxTokens != tt.expectedLimit-2000 {
						t.Errorf("Expected maxTokens=%d (limit-reserve), got %d", tt.expectedLimit-2000, maxTokens)
					}
					return []domain.StoredMessage{}, nil
				},
			}, &mockToolExecutionService{}, &mockSecurityService{})

			service.BuildContextWindow(context.Background(), "conv-123", tt.provider, tt.expectedLimit)
		})
	}
}

// TestBuildContextWindow_EmptyConversation tests handling of empty conversation
func TestBuildContextWindow_EmptyConversation(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000)

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages for empty conversation, got %d", len(messages))
	}

	if totalTokens != 0 {
		t.Errorf("Expected 0 total tokens, got %d", totalTokens)
	}
}

// TestBuildContextWindow_SingleMessage tests context with single message
func TestBuildContextWindow_SingleMessage(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10, Timestamp: time.Now()},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000)

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Role != "user" {
		t.Errorf("Expected user message, got %s", messages[0].Role)
	}

	if totalTokens != 10 {
		t.Errorf("Expected 10 tokens, got %d", totalTokens)
	}
}
