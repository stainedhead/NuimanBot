package fallback

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// TestFallbackService_Stream_PrimarySucceeds tests successful streaming with primary provider.
func TestFallbackService_Stream_PrimarySucceeds(t *testing.T) {
	ch := make(chan domain.StreamChunk, 2)
	ch <- domain.StreamChunk{Delta: "hello"}
	ch <- domain.StreamChunk{Done: true}
	close(ch)

	mock := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			return ch, nil
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	result, err := service.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil channel")
	}

	// Drain the channel
	var chunks []domain.StreamChunk
	for c := range result {
		chunks = append(chunks, c)
	}
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunks))
	}
}

// TestFallbackService_Stream_PrimaryFailsFallbackSucceeds tests fallback on stream failure.
func TestFallbackService_Stream_PrimaryFailsFallbackSucceeds(t *testing.T) {
	callCount := 0
	ch := make(chan domain.StreamChunk, 1)
	ch <- domain.StreamChunk{Delta: "fallback response"}
	close(ch)

	mock := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("primary stream error")
			}
			return ch, nil
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	result, err := service.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil channel")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 stream attempts, got %d", callCount)
	}
}

// TestFallbackService_Stream_AllFail tests all providers failing in streaming mode.
func TestFallbackService_Stream_AllFail(t *testing.T) {
	mock := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			return nil, errors.New("stream unavailable")
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := service.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error when all providers fail")
	}
	if err.Error() != "all LLM providers failed" {
		t.Errorf("Expected 'all LLM providers failed', got '%s'", err.Error())
	}
}

// TestFallbackService_Stream_EmptyProviders tests streaming with no providers configured.
func TestFallbackService_Stream_EmptyProviders(t *testing.T) {
	mock := &mockLLMService{}
	service := NewFallbackService(mock, []domain.LLMProvider{})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := service.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error with no providers")
	}
	if err.Error() != "no providers available" {
		t.Errorf("Expected 'no providers available', got '%s'", err.Error())
	}
}

// TestFallbackService_Stream_FallbackProvider tests that using a different provider logs fallback.
func TestFallbackService_Stream_FallbackProvider(t *testing.T) {
	callCount := 0
	ch := make(chan domain.StreamChunk)
	close(ch)

	mock := &mockLLMService{
		streamFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
			callCount++
			if provider == domain.LLMProviderAnthropic {
				return nil, errors.New("anthropic unavailable")
			}
			return ch, nil
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	// Request with Anthropic, but Anthropic fails, should use OpenAI
	result, err := service.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil channel")
	}
}

// TestFallbackService_Complete_EmptyProviders tests Complete with no providers configured.
func TestFallbackService_Complete_EmptyProviders(t *testing.T) {
	mock := &mockLLMService{}
	service := NewFallbackService(mock, []domain.LLMProvider{})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error with no providers")
	}
	if err.Error() != "no providers available" {
		t.Errorf("Expected 'no providers available', got '%s'", err.Error())
	}
}

// TestFallbackService_ListModels tests ListModels delegation.
func TestFallbackService_ListModels(t *testing.T) {
	expectedModels := []domain.ModelInfo{
		{ID: "model-1", Name: "Model 1", Provider: "anthropic"},
		{ID: "model-2", Name: "Model 2", Provider: "anthropic"},
	}

	mock := &mockLLMService{
		listModelsFunc: func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
			return expectedModels, nil
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{domain.LLMProviderAnthropic})

	models, err := service.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d models, got %d", len(expectedModels), len(models))
	}
}

// TestFallbackService_ListModels_Error tests ListModels error propagation.
func TestFallbackService_ListModels_Error(t *testing.T) {
	mock := &mockLLMService{
		listModelsFunc: func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
			return nil, errors.New("models unavailable")
		},
	}

	service := NewFallbackService(mock, []domain.LLMProvider{domain.LLMProviderAnthropic})

	_, err := service.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err == nil {
		t.Fatal("Expected error from ListModels")
	}
}
