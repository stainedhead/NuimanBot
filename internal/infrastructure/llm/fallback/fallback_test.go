package fallback

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// Test: Primary provider succeeds, no fallback
func TestFallbackService_PrimarySucceeds(t *testing.T) {
	primary := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "primary response"}, nil
		},
	}

	// Create fallback chain: primary (Anthropic) -> OpenAI -> Ollama
	service := NewFallbackService(primary, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	resp, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Content != "primary response" {
		t.Errorf("Expected 'primary response', got '%s'", resp.Content)
	}
}

// Test: Primary fails, fallback to next provider
func TestFallbackService_PrimaryFailsFallbackSucceeds(t *testing.T) {
	callCount := 0
	primary := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				// First call (Anthropic) fails
				return nil, errors.New("anthropic unavailable")
			}
			// Second call (OpenAI) succeeds
			return &domain.LLMResponse{Content: "fallback response"}, nil
		},
	}

	service := NewFallbackService(primary, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	resp, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Content != "fallback response" {
		t.Errorf("Expected 'fallback response', got '%s'", resp.Content)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 provider attempts, got %d", callCount)
	}
}

// Test: Multiple fallbacks
func TestFallbackService_MultipleFallbacks(t *testing.T) {
	callCount := 0
	primary := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount <= 2 {
				// First two calls fail (Anthropic, OpenAI)
				return nil, errors.New("provider unavailable")
			}
			// Third call (Ollama) succeeds
			return &domain.LLMResponse{Content: "ollama response"}, nil
		},
	}

	service := NewFallbackService(primary, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	resp, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.Content != "ollama response" {
		t.Errorf("Expected 'ollama response', got '%s'", resp.Content)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 provider attempts, got %d", callCount)
	}
}

// Test: All providers fail
func TestFallbackService_AllProvidersFail(t *testing.T) {
	primary := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, errors.New("provider unavailable")
		},
	}

	service := NewFallbackService(primary, []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)

	if err == nil {
		t.Fatal("Expected error when all providers fail")
	}

	if err.Error() != "all LLM providers failed" {
		t.Errorf("Expected 'all LLM providers failed', got '%s'", err.Error())
	}
}

// Test: No fallback providers configured
func TestFallbackService_NoFallbacks(t *testing.T) {
	primary := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, errors.New("primary unavailable")
		},
	}

	service := NewFallbackService(primary, []domain.LLMProvider{
		domain.LLMProviderAnthropic, // Only one provider
	})

	req := &domain.LLMRequest{
		Model:    "claude-3-sonnet",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := service.Complete(context.Background(), domain.LLMProviderAnthropic, req)

	if err == nil {
		t.Fatal("Expected error when primary fails and no fallbacks configured")
	}
}

// mockLLMService is a mock implementation for testing
type mockLLMService struct {
	completeFunc   func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
	streamFunc     func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error)
	listModelsFunc func(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error)
}

func (m *mockLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, provider, req)
	}
	return &domain.LLMResponse{}, nil
}

func (m *mockLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, provider, req)
	}
	ch := make(chan domain.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	if m.listModelsFunc != nil {
		return m.listModelsFunc(ctx, provider)
	}
	return []domain.ModelInfo{}, nil
}
