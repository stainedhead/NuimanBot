package resilience

import (
	"context"
	"log/slog"
	"time"

	"nuimanbot/internal/domain"
)

// ResilientLLMService wraps an LLM service with circuit breaker and retry logic.
type ResilientLLMService struct {
	inner          domain.LLMService
	circuitBreaker *CircuitBreaker
	retryPolicy    *RetryPolicy
}

// NewResilientLLMService creates a new resilient LLM service wrapper.
//
// inner: The underlying LLM service to wrap.
// maxFailures: Circuit breaker threshold (number of consecutive failures before opening).
// resetTimeout: Circuit breaker reset timeout (duration before attempting recovery).
// maxRetries: Maximum number of retry attempts.
// retryDelay: Initial delay between retries (exponential backoff).
func NewResilientLLMService(
	inner domain.LLMService,
	maxFailures uint32,
	resetTimeout time.Duration,
	maxRetries int,
	retryDelay time.Duration,
) *ResilientLLMService {
	return &ResilientLLMService{
		inner:          inner,
		circuitBreaker: NewCircuitBreaker(maxFailures, resetTimeout),
		retryPolicy:    NewRetryPolicy(maxRetries, retryDelay),
	}
}

// Complete performs an LLM completion with circuit breaker and retry protection.
func (r *ResilientLLMService) Complete(
	ctx context.Context,
	provider domain.LLMProvider,
	req *domain.LLMRequest,
) (*domain.LLMResponse, error) {
	var response *domain.LLMResponse

	// Retry with exponential backoff
	err := r.retryPolicy.Retry(func() error {
		// Check circuit breaker
		return r.circuitBreaker.Call(func() error {
			var err error
			response, err = r.inner.Complete(ctx, provider, req)
			if err != nil {
				slog.Warn("LLM completion failed",
					"provider", provider,
					"model", req.Model,
					"error", err,
				)
				return err
			}
			return nil
		})
	})

	if err != nil {
		slog.Error("LLM completion failed after retries",
			"provider", provider,
			"model", req.Model,
			"error", err,
			"circuit_state", r.circuitBreaker.Stats().State,
		)
		return nil, err
	}

	return response, nil
}

// Stream performs streaming LLM completion with circuit breaker protection.
//
// Note: Retry is not applied to streaming as it's not idempotent and harder to recover.
// Only circuit breaker protection is used.
func (r *ResilientLLMService) Stream(
	ctx context.Context,
	provider domain.LLMProvider,
	req *domain.LLMRequest,
) (<-chan domain.StreamChunk, error) {
	var chunks <-chan domain.StreamChunk

	// Only use circuit breaker for streaming (no retry)
	err := r.circuitBreaker.Call(func() error {
		var err error
		chunks, err = r.inner.Stream(ctx, provider, req)
		if err != nil {
			slog.Warn("LLM streaming failed",
				"provider", provider,
				"model", req.Model,
				"error", err,
			)
			return err
		}
		return nil
	})

	if err != nil {
		slog.Error("LLM streaming failed",
			"provider", provider,
			"model", req.Model,
			"error", err,
			"circuit_state", r.circuitBreaker.Stats().State,
		)
		return nil, err
	}

	return chunks, nil
}

// ListModels lists available models with circuit breaker and retry protection.
func (r *ResilientLLMService) ListModels(
	ctx context.Context,
	provider domain.LLMProvider,
) ([]domain.ModelInfo, error) {
	var models []domain.ModelInfo

	// Retry with exponential backoff
	err := r.retryPolicy.Retry(func() error {
		// Check circuit breaker
		return r.circuitBreaker.Call(func() error {
			var err error
			models, err = r.inner.ListModels(ctx, provider)
			if err != nil {
				slog.Warn("LLM list models failed",
					"provider", provider,
					"error", err,
				)
				return err
			}
			return nil
		})
	})

	if err != nil {
		slog.Error("LLM list models failed after retries",
			"provider", provider,
			"error", err,
			"circuit_state", r.circuitBreaker.Stats().State,
		)
		return nil, err
	}

	return models, nil
}

// CircuitBreakerStats returns circuit breaker statistics.
func (r *ResilientLLMService) CircuitBreakerStats() Stats {
	return r.circuitBreaker.Stats()
}

// RetryStats returns retry policy statistics.
func (r *ResilientLLMService) RetryStats() RetryStats {
	return r.retryPolicy.Stats()
}
