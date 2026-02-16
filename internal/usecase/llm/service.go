package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
// It supports dynamic provider routing based on model name, config mappings, or explicit provider selection.
type Service struct {
	cfg             *config.LLMConfig
	providerClients sync.Map // Map[domain.LLMProvider]ProviderClient
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
// If provider is empty, it auto-resolves from the request's Model field.
func (s *Service) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	resolved, err := s.resolveProvider(provider, req.Model)
	if err != nil {
		return nil, err
	}
	client, err := s.GetClientForProvider(resolved)
	if err != nil {
		return nil, err
	}
	return client.Complete(ctx, resolved, req)
}

// Stream performs a streaming completion request by routing to the appropriate provider.
// If provider is empty, it auto-resolves from the request's Model field.
func (s *Service) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	resolved, err := s.resolveProvider(provider, req.Model)
	if err != nil {
		return nil, err
	}
	client, err := s.GetClientForProvider(resolved)
	if err != nil {
		return nil, err
	}
	return client.Stream(ctx, resolved, req)
}

// ListModels lists available models for a given provider.
func (s *Service) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	client, err := s.GetClientForProvider(provider)
	if err != nil {
		return nil, err
	}
	return client.ListModels(ctx, provider)
}

// resolveProvider returns the provider as-is if non-empty, or auto-resolves from the model name.
func (s *Service) resolveProvider(provider domain.LLMProvider, model string) (domain.LLMProvider, error) {
	if provider != "" {
		return provider, nil
	}
	return s.ResolveProviderForModel(model)
}

// ResolveProviderForModel determines which LLM provider handles a given model.
// Resolution order:
//  1. Explicit model mapping in config (cfg.Models)
//  2. Provider prefix format "provider/model" (e.g., "anthropic/claude-3-sonnet")
//  3. Default provider from cfg.DefaultModel.Primary
func (s *Service) ResolveProviderForModel(model string) (domain.LLMProvider, error) {
	// 1. Check explicit model mappings
	if s.cfg.Models != nil {
		if modelCfg, ok := s.cfg.Models[model]; ok {
			provider, err := s.findProviderByConfigID(modelCfg.ProviderConfigID)
			if err == nil {
				return provider, nil
			}
		}
	}

	// 2. Check "provider/model" format
	if provider, ok := parseProviderPrefix(model); ok {
		return provider, nil
	}

	// 3. Fall back to default
	return s.DefaultProvider()
}

// DefaultProvider resolves the default provider from config or registered clients.
func (s *Service) DefaultProvider() (domain.LLMProvider, error) {
	primary := s.cfg.DefaultModel.Primary
	if primary != "" {
		// Try provider prefix format
		if provider, ok := parseProviderPrefix(primary); ok {
			return provider, nil
		}
		// Try models map
		if s.cfg.Models != nil {
			if modelCfg, ok := s.cfg.Models[primary]; ok {
				return s.findProviderByConfigID(modelCfg.ProviderConfigID)
			}
		}
	}

	// Fall back to first registered provider
	var firstProvider domain.LLMProvider
	found := false
	s.providerClients.Range(func(key, _ any) bool {
		firstProvider = key.(domain.LLMProvider)
		found = true
		return false
	})
	if found {
		return firstProvider, nil
	}

	return "", fmt.Errorf("no providers registered and no default configured")
}

// findProviderByConfigID finds the LLMProvider type for a given provider config ID.
func (s *Service) findProviderByConfigID(configID string) (domain.LLMProvider, error) {
	for _, p := range s.cfg.Providers {
		if p.ID == configID {
			return p.Type, nil
		}
	}
	return "", fmt.Errorf("provider config %q not found", configID)
}

// parseProviderPrefix extracts a provider from "provider/model" format.
func parseProviderPrefix(model string) (domain.LLMProvider, bool) {
	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	provider := domain.LLMProvider(parts[0])
	switch provider {
	case domain.LLMProviderAnthropic, domain.LLMProviderOpenAI, domain.LLMProviderOllama, domain.LLMProviderBedrock:
		return provider, true
	}
	return "", false
}
