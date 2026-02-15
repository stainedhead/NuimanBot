package chat

import (
	"context"
	"errors"
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

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "")

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
	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 400, "")

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
					// Verify correct limit is used (minus reserved tokens)
					expectedAvailable := tt.expectedLimit - ReservedTokens
					if maxTokens != expectedAvailable {
						t.Errorf("Expected maxTokens=%d (limit-reserve), got %d", expectedAvailable, maxTokens)
					}
					return []domain.StoredMessage{}, nil
				},
			}, &mockToolExecutionService{}, &mockSecurityService{})

			service.BuildContextWindow(context.Background(), "conv-123", tt.provider, tt.expectedLimit, "")
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

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "")

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

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "")

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

// --- Memory Recall Integration Tests ---

// TestBuildContextWindow_WithMemoryRecall tests that recalled memories are prepended
func TestBuildContextWindow_WithMemoryRecall(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "What was our project about?", TokenCount: 20},
			}, nil
		},
	}

	recaller := &mockMemoryRecaller{
		recallFunc: func(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
			return "### Relevant Long-Term Memory\n\n- User prefers Go for backend\n- Project uses SQLite\n", nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "What was our project about?")

	// Should have 2 messages: system memory message + user message
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages (memory + user), got %d", len(messages))
	}

	// First message should be system with memory content
	if messages[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got %q", messages[0].Role)
	}
	if messages[0].Content == "" {
		t.Error("Expected non-empty memory content in system message")
	}

	// Second message should be the user message
	if messages[1].Role != "user" {
		t.Errorf("Expected second message role 'user', got %q", messages[1].Role)
	}

	// Total tokens should include memory tokens
	if totalTokens <= 20 {
		t.Errorf("Expected total tokens > 20 (user tokens + memory tokens), got %d", totalTokens)
	}

	// Verify the recaller was called with correct params
	if !recaller.called {
		t.Error("Expected recaller to be called")
	}
	if recaller.lastQuery != "What was our project about?" {
		t.Errorf("Expected query 'What was our project about?', got %q", recaller.lastQuery)
	}
}

// TestBuildContextWindow_MemoryRecallEmpty tests that empty recall result adds no system message
func TestBuildContextWindow_MemoryRecallEmpty(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
			}, nil
		},
	}

	recaller := &mockMemoryRecaller{
		recallFunc: func(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
			return "", nil // No memories found
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "Hello")

	// Should have only the user message - no system memory message
	if len(messages) != 1 {
		t.Errorf("Expected 1 message (no memory), got %d", len(messages))
	}

	if totalTokens != 10 {
		t.Errorf("Expected 10 tokens, got %d", totalTokens)
	}
}

// TestBuildContextWindow_MemoryRecallFailsGracefully tests graceful degradation on recall error
func TestBuildContextWindow_MemoryRecallFailsGracefully(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
				{ID: "msg2", Role: "assistant", Content: "Hi!", TokenCount: 5},
			}, nil
		},
	}

	recaller := &mockMemoryRecaller{
		recallFunc: func(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
			return "", errors.New("database connection refused")
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	// Should NOT return error - degrade gracefully
	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "Hello")

	// Should still have conversation messages despite recall failure
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages despite recall failure, got %d", len(messages))
	}

	if totalTokens != 15 {
		t.Errorf("Expected 15 tokens, got %d", totalTokens)
	}
}

// TestBuildContextWindow_NoRecallerSet tests that no recall happens without recaller
func TestBuildContextWindow_NoRecallerSet(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	// No recaller set

	messages, totalTokens := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "Hello")

	if len(messages) != 1 {
		t.Errorf("Expected 1 message without recaller, got %d", len(messages))
	}

	if totalTokens != 10 {
		t.Errorf("Expected 10 tokens, got %d", totalTokens)
	}
}

// TestBuildContextWindow_EmptyQuerySkipsRecall tests that empty query skips recall
func TestBuildContextWindow_EmptyQuerySkipsRecall(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
			}, nil
		},
	}

	recaller := &mockMemoryRecaller{}
	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	messages, _ := service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "")

	// Recaller should NOT be called with empty query
	if recaller.called {
		t.Error("Expected recaller NOT to be called with empty query")
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
}

// TestBuildContextWindow_MemoryTokensBudget tests that memory tokens reduce available budget
func TestBuildContextWindow_MemoryTokensBudget(t *testing.T) {
	// Memory recall returns ~100 tokens worth of content
	memoryContent := "### Relevant Long-Term Memory\n\n**Scene: project-setup**\n" +
		"Summary: Go project with SQLite\n\n" +
		"**Key Facts:**\n- [fact, salience=0.90] User prefers Go for backend\n" +
		"- [fact, salience=0.85] Project uses SQLite for storage\n\n" +
		"*Retrieved 2 cells from 1 scenes (100 tokens)*\n"

	var capturedMaxTokens int
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			capturedMaxTokens = maxTokens
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", TokenCount: 10},
			}, nil
		},
	}

	recaller := &mockMemoryRecaller{
		recallFunc: func(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
			return memoryContent, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	service.BuildContextWindow(context.Background(), "conv-123", domain.LLMProviderAnthropic, 1000, "test query")

	// The available tokens for messages should be reduced by memory token estimate
	// Memory content is ~300 chars, estimate ~75 tokens (300/4)
	// Available = 1000 - memory_tokens
	if capturedMaxTokens >= 1000 {
		t.Errorf("Expected reduced token budget for messages after memory recall, got %d", capturedMaxTokens)
	}
}

// TestBuildContextWindow_MemoryRecallConversationID tests correct conversationID is passed
func TestBuildContextWindow_MemoryRecallConversationID(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	var capturedConvID string
	recaller := &mockMemoryRecaller{
		recallFunc: func(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
			capturedConvID = conversationID
			return "", nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})
	service.SetMemoryRecaller(recaller)

	service.BuildContextWindow(context.Background(), "conv-specific-456", domain.LLMProviderAnthropic, 1000, "test")

	if capturedConvID != "conv-specific-456" {
		t.Errorf("Expected conversationID 'conv-specific-456', got %q", capturedConvID)
	}
}
