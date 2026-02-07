package fallback

import (
	"context"
	"fmt"
	"log/slog"

	"nuimanbot/internal/domain"
)

// Service wraps an LLMService and provides automatic fallback to alternative providers.
type Service struct {
	underlying    domain.LLMService
	providerChain []domain.LLMProvider // Ordered list of providers to try
}

// NewFallbackService creates a new fallback service.
// The providerChain defines the order of providers to try (first is primary).
func NewFallbackService(underlying domain.LLMService, providerChain []domain.LLMProvider) *Service {
	return &Service{
		underlying:    underlying,
		providerChain: providerChain,
	}
}

// Complete attempts to complete a request with automatic fallback.
func (s *Service) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	// Try providers in order
	var lastErr error
	for _, p := range s.providerChain {
		slog.Info("Attempting LLM request", "provider", p)

		resp, err := s.underlying.Complete(ctx, p, req)
		if err == nil {
			// Success
			if p != provider {
				slog.Info("Fallback successful", "used", p, "instead_of", provider)
			}
			return resp, nil
		}

		// Log failure and continue to next provider
		slog.Warn("Provider failed", "provider", p, "error", err)
		lastErr = err
	}

	// All providers failed
	if lastErr != nil {
		return nil, fmt.Errorf("all LLM providers failed")
	}

	return nil, fmt.Errorf("no providers available")
}

// Stream attempts to stream a response with automatic fallback.
func (s *Service) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	// Try providers in order
	var lastErr error
	for _, p := range s.providerChain {
		slog.Info("Attempting streaming LLM request", "provider", p)

		ch, err := s.underlying.Stream(ctx, p, req)
		if err == nil {
			// Success
			if p != provider {
				slog.Info("Fallback successful", "used", p, "instead_of", provider)
			}
			return ch, nil
		}

		// Log failure and continue to next provider
		slog.Warn("Provider failed", "provider", p, "error", err)
		lastErr = err
	}

	// All providers failed
	if lastErr != nil {
		return nil, fmt.Errorf("all LLM providers failed")
	}

	return nil, fmt.Errorf("no providers available")
}

// ListModels lists available models for a provider (no fallback for this operation).
func (s *Service) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	return s.underlying.ListModels(ctx, provider)
}
