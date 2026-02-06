package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/resilience"
)

type mockLLMService struct {
	completeCalls int
	streamCalls   int
	listCalls     int
	shouldFail    bool
	failCount     int
}

func (m *mockLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	m.completeCalls++
	if m.shouldFail {
		if m.failCount > 0 {
			m.failCount--
			return nil, errors.New("temporary failure")
		}
	}
	return &domain.LLMResponse{
		Content: "test response",
	}, nil
}

func (m *mockLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	m.streamCalls++
	if m.shouldFail {
		return nil, errors.New("stream failure")
	}
	ch := make(chan domain.StreamChunk, 1)
	ch <- domain.StreamChunk{Delta: "test"}
	close(ch)
	return ch, nil
}

func (m *mockLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	m.listCalls++
	if m.shouldFail {
		if m.failCount > 0 {
			m.failCount--
			return nil, errors.New("temporary failure")
		}
	}
	return []domain.ModelInfo{{ID: "test-model"}}, nil
}

func TestResilientLLMService_Complete_Success(t *testing.T) {
	mock := &mockLLMService{}
	resilient := resilience.NewResilientLLMService(mock, 3, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	resp, err := resilient.Complete(context.Background(), provider, req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp == nil || resp.Content != "test response" {
		t.Error("Expected valid response")
	}

	if mock.completeCalls != 1 {
		t.Errorf("Expected 1 call, got %d", mock.completeCalls)
	}
}

func TestResilientLLMService_Complete_RetrySuccess(t *testing.T) {
	mock := &mockLLMService{
		shouldFail: true,
		failCount:  2, // Fail twice, then succeed
	}
	resilient := resilience.NewResilientLLMService(mock, 5, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	resp, err := resilient.Complete(context.Background(), provider, req)

	if err != nil {
		t.Errorf("Expected success after retries, got %v", err)
	}

	if resp == nil {
		t.Error("Expected valid response after retries")
	}

	if mock.completeCalls != 3 {
		t.Errorf("Expected 3 calls (2 failures + 1 success), got %d", mock.completeCalls)
	}
}

func TestResilientLLMService_Complete_CircuitBreaker(t *testing.T) {
	mock := &mockLLMService{shouldFail: true, failCount: 100} // Always fail
	resilient := resilience.NewResilientLLMService(mock, 2, 100*time.Millisecond, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	// First call: will retry 3 times, then fail
	resilient.Complete(context.Background(), provider, req)

	// Circuit should now be open (3 retries hit the 2 failure threshold)
	stats := resilient.CircuitBreakerStats()
	if stats.State != "open" {
		t.Errorf("Expected circuit to be open, got %s", stats.State)
	}

	// Next call should fail immediately without calling the inner service
	callsBefore := mock.completeCalls
	_, err := resilient.Complete(context.Background(), provider, req)

	if err != resilience.ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	if mock.completeCalls != callsBefore {
		t.Error("Circuit breaker should have prevented the call")
	}
}

func TestResilientLLMService_Stream_Success(t *testing.T) {
	mock := &mockLLMService{}
	resilient := resilience.NewResilientLLMService(mock, 3, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	chunks, err := resilient.Stream(context.Background(), provider, req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if chunks == nil {
		t.Error("Expected valid channel")
	}

	if mock.streamCalls != 1 {
		t.Errorf("Expected 1 call, got %d", mock.streamCalls)
	}
}

func TestResilientLLMService_Stream_CircuitBreaker(t *testing.T) {
	mock := &mockLLMService{shouldFail: true}
	resilient := resilience.NewResilientLLMService(mock, 2, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	// Fail twice to open circuit (no retry for streaming)
	resilient.Stream(context.Background(), provider, req)
	resilient.Stream(context.Background(), provider, req)

	// Circuit should be open
	_, err := resilient.Stream(context.Background(), provider, req)

	if err != resilience.ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestResilientLLMService_ListModels_Success(t *testing.T) {
	mock := &mockLLMService{}
	resilient := resilience.NewResilientLLMService(mock, 3, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI

	models, err := resilient.ListModels(context.Background(), provider)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(models) != 1 || models[0].ID != "test-model" {
		t.Error("Expected valid models list")
	}

	if mock.listCalls != 1 {
		t.Errorf("Expected 1 call, got %d", mock.listCalls)
	}
}

func TestResilientLLMService_Stats(t *testing.T) {
	mock := &mockLLMService{shouldFail: true, failCount: 2}
	resilient := resilience.NewResilientLLMService(mock, 5, 5*time.Second, 3, 10*time.Millisecond)

	provider := domain.LLMProviderOpenAI
	req := &domain.LLMRequest{Model: "gpt-4"}

	// Make a call that retries
	resilient.Complete(context.Background(), provider, req)

	// Check stats
	cbStats := resilient.CircuitBreakerStats()
	retryStats := resilient.RetryStats()

	if cbStats.SuccessCount != 1 {
		t.Errorf("Expected 1 success in circuit breaker, got %d", cbStats.SuccessCount)
	}

	if retryStats.TotalRetries < 1 {
		t.Errorf("Expected at least 1 retry, got %d", retryStats.TotalRetries)
	}
}
