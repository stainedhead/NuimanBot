package chat

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// Test streaming with no tool calls - simple text response
func TestProcessMessageStream_NoToolCalls(t *testing.T) {
	llmService := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			ch := make(chan domain.StreamChunk, 3)
			go func() {
				defer close(ch)
				ch <- domain.StreamChunk{Delta: "Hello"}
				ch <- domain.StreamChunk{Delta: " world"}
				ch <- domain.StreamChunk{Delta: "!", Done: true}
			}()
			return ch, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	// Create test message
	msg := &domain.IncomingMessage{
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-456",
		Text:        "test message",
	}

	// Process streaming message
	ch, err := service.ProcessMessageStream(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessMessageStream failed: %v", err)
	}

	// Collect chunks
	var chunks []string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("Stream error: %v", chunk.Error)
		}
		if chunk.Delta != "" {
			chunks = append(chunks, chunk.Delta)
		}
	}

	// Verify chunks received
	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunks))
	}

	expectedChunks := []string{"Hello", " world", "!"}
	for i, expected := range expectedChunks {
		if i < len(chunks) && chunks[i] != expected {
			t.Errorf("Chunk %d: expected '%s', got '%s'", i, expected, chunks[i])
		}
	}
}

// Test streaming with tool calls - should return error (not yet supported)
func TestProcessMessageStream_WithToolCalls(t *testing.T) {
	llmService := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			ch := make(chan domain.StreamChunk, 2)
			go func() {
				defer close(ch)
				// Simulate tool call in stream
				ch <- domain.StreamChunk{
					ToolCall: &domain.ToolCall{
						ToolName:  "calculator",
						Arguments: map[string]any{"expression": "2+2"},
					},
					Done: true,
				}
			}()
			return ch, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	msg := &domain.IncomingMessage{
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-456",
		Text:        "What is 2+2?",
	}

	ch, err := service.ProcessMessageStream(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessMessageStream failed: %v", err)
	}

	// Collect chunks - should get error about tool calls not supported
	var gotError bool
	for chunk := range ch {
		if chunk.Error != nil {
			gotError = true
			// Verify it's an error about tool calls not being supported
			if chunk.Error.Error() != "streaming with tool calls not yet supported - use ProcessMessage instead" {
				t.Errorf("Unexpected error: %v", chunk.Error)
			}
		}
	}

	if !gotError {
		t.Error("Expected error when tool calls are present in stream")
	}
}

// Test streaming with error
func TestProcessMessageStream_LLMError(t *testing.T) {
	llmService := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			ch := make(chan domain.StreamChunk, 2)
			go func() {
				defer close(ch)
				ch <- domain.StreamChunk{Delta: "Start"}
				// Simulate error mid-stream
				time.Sleep(10 * time.Millisecond)
				ch <- domain.StreamChunk{Error: domain.ErrLLMUnavailable}
			}()
			return ch, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	msg := &domain.IncomingMessage{
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-456",
		Text:        "test message",
	}

	ch, err := service.ProcessMessageStream(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessMessageStream failed: %v", err)
	}

	// Collect chunks until error
	errorReceived := false
	for chunk := range ch {
		if chunk.Error != nil {
			errorReceived = true
			if chunk.Error != domain.ErrLLMUnavailable {
				t.Errorf("Expected ErrLLMUnavailable, got %v", chunk.Error)
			}
		}
	}

	if !errorReceived {
		t.Error("Expected to receive error in stream")
	}
}

// Test that stream respects context cancellation
func TestProcessMessageStream_ContextCancellation(t *testing.T) {
	llmService := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			ch := make(chan domain.StreamChunk)
			go func() {
				defer close(ch)
				for i := 0; i < 10; i++ {
					select {
					case <-ctx.Done():
						return
					case ch <- domain.StreamChunk{Delta: "chunk"}:
						time.Sleep(50 * time.Millisecond)
					}
				}
			}()
			return ch, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(llmService, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	msg := &domain.IncomingMessage{
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-456",
		Text:        "test message",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := service.ProcessMessageStream(ctx, msg)
	if err != nil {
		t.Fatalf("ProcessMessageStream failed: %v", err)
	}

	// Read a couple chunks then cancel
	chunkCount := 0
	for chunk := range ch {
		if chunk.Error != nil {
			break
		}
		chunkCount++
		if chunkCount == 2 {
			cancel()
		}
	}

	// Verify we didn't read all 10 chunks
	if chunkCount >= 10 {
		t.Error("Context cancellation did not stop stream early")
	}
}
