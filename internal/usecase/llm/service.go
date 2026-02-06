package llm

import (
	"context"
	"fmt"
	"sync" // For sync.Map

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// ProviderClient represents a concrete LLM client implementation (e.g., Anthropic, OpenAI).
// This interface is essentially the domain.LLMService interface, but defined here to avoid circular dependencies
// if the domain.LLMService interface requires types from usecase.
// However, in our Clean Architecture setup, domain should define interfaces, and usecase should implement them or orchestrate.
// So, we will use domain.LLMService here as the contract for concrete clients.
type ProviderClient interface {
	domain.LLMService // Embed the domain LLMService interface
}

// Service implements the domain.LLMService interface by orchestrating calls to specific LLM providers.

type Service struct {
	cfg *config.LLMConfig

	providerClients sync.Map // Map[domain.LLMProvider]ProviderClient

	// defaultLLMClient ProviderClient // Client for the default primary model - TODO: Implement default client selection logic.

}

// NewService creates a new LLM orchestration service.
func NewService(cfg *config.LLMConfig) *Service {
	return &Service{
		cfg:             cfg,
		providerClients: sync.Map{},
	}
}

// RegisterProviderClient registers an LLM client for a specific provider.
func (s *Service) RegisterProviderClient(provider domain.LLMProvider, client ProviderClient) {
	s.providerClients.Store(provider, client)
}

// GetClientForProvider retrieves an LLM client for a specific provider.
func (s *Service) GetClientForProvider(provider domain.LLMProvider) (ProviderClient, error) {
	if client, ok := s.providerClients.Load(provider); ok {
		return client.(ProviderClient), nil
	}
	return nil, fmt.Errorf("LLM client for provider %s not registered", provider)
}

// Complete performs a completion request by routing to the appropriate provider.
func (s *Service) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	client, err := s.GetClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	return client.Complete(ctx, provider, req)
}

// Stream performs a streaming completion request by routing to the appropriate provider.
func (s *Service) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	client, err := s.GetClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	return client.Stream(ctx, provider, req)
}

// ListModels lists available models for a given provider.
func (s *Service) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	client, err := s.GetClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	return client.ListModels(ctx, provider)
}

// TODO: Implement logic to select the correct provider based on req.Model and cfg.DefaultModel/Models map.
// This basic implementation assumes 'provider' is explicitly passed.
