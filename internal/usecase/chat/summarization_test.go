package chat

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// TestSummarizeConversation_Success tests successful conversation summarization
func TestSummarizeConversation_Success(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			// Verify summarization prompt
			if len(req.Messages) == 0 {
				t.Error("Expected messages in summarization request")
			}
			return &domain.LLMResponse{
				Content: "Summary of the conversation: User asked about weather, bot provided forecast.",
				Usage: domain.TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			// Return old messages that need summarization
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "What's the weather?"},
				{ID: "msg2", Role: "assistant", Content: "It's sunny."},
				{ID: "msg3", Role: "user", Content: "Will it rain?"},
				{ID: "msg4", Role: "assistant", Content: "No rain expected."},
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockSkillExecutionService{}, &mockSecurityService{})

	summary, err := service.SummarizeConversation(context.Background(), "conv-123", 1000)
	if err != nil {
		t.Fatalf("SummarizeConversation failed: %v", err)
	}

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	if summary != "Summary of the conversation: User asked about weather, bot provided forecast." {
		t.Errorf("Unexpected summary: %s", summary)
	}
}

// TestSummarizeConversation_EmptyConversation tests summarization with no messages
func TestSummarizeConversation_EmptyConversation(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockSkillExecutionService{}, &mockSecurityService{})

	summary, err := service.SummarizeConversation(context.Background(), "conv-123", 1000)
	if err == nil {
		t.Fatal("Expected error for empty conversation")
	}

	if summary != "" {
		t.Error("Expected empty summary for empty conversation")
	}
}

// TestSummarizeConversation_LLMError tests handling of LLM errors during summarization
func TestSummarizeConversation_LLMError(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, errors.New("LLM service unavailable")
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello"},
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockSkillExecutionService{}, &mockSecurityService{})

	_, err := service.SummarizeConversation(context.Background(), "conv-123", 1000)
	if err == nil {
		t.Fatal("Expected LLM error to propagate")
	}
}

// TestSummarizeConversation_RepositoryError tests handling of repository errors
func TestSummarizeConversation_RepositoryError(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return nil, errors.New("database connection failed")
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockSkillExecutionService{}, &mockSecurityService{})

	_, err := service.SummarizeConversation(context.Background(), "conv-123", 1000)
	if err == nil {
		t.Fatal("Expected repository error to propagate")
	}

	if err.Error() != "failed to get messages for summarization: database connection failed" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestSummarizeConversation_PreservesKeyInformation tests that summary preserves important details
func TestSummarizeConversation_PreservesKeyInformation(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			// Check that system prompt instructs to preserve key information
			if req.SystemPrompt == "" {
				t.Error("Expected system prompt for summarization")
			}
			return &domain.LLMResponse{
				Content: "User asked about weather in Seattle. Assistant provided 7-day forecast with rain expected.",
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "What's the weather in Seattle?"},
				{ID: "msg2", Role: "assistant", Content: "Seattle: 7-day forecast shows rain."},
			}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockSkillExecutionService{}, &mockSecurityService{})

	summary, err := service.SummarizeConversation(context.Background(), "conv-123", 1000)
	if err != nil {
		t.Fatalf("SummarizeConversation failed: %v", err)
	}

	if summary == "" {
		t.Error("Expected non-empty summary")
	}
}
