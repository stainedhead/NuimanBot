package anthropic

import (
	"context"
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

const (
	// Event types for streaming
	eventTypeContentBlockDelta = "content_block_delta"
	eventTypeMessageStop       = "message_stop"

	// Delta types
	deltaTypeText = "text_delta"
)

// Client implements domain.LLMService for the Anthropic API.
type Client struct {
	client *anthropicsdk.Client
	cfg    *config.LLMProviderConfig
}

// NewClient creates a new Anthropic LLM client.
func NewClient(cfg *config.LLMProviderConfig) (*Client, error) {
	if cfg.Type != domain.LLMProviderAnthropic {
		return nil, fmt.Errorf("invalid LLM provider type for Anthropic client: %s", cfg.Type)
	}
	if cfg.APIKey.Value() == "" {
		return nil, errors.New("Anthropic API key is required")
	}

	anthropicClient := anthropicsdk.NewClient(
		option.WithAPIKey(cfg.APIKey.Value()),
	)

	return &Client{
		client: &anthropicClient,
		cfg:    cfg,
	}, nil
}

// NewClientWithBaseURL creates a client with a custom base URL (for testing).
func NewClientWithBaseURL(cfg *config.LLMProviderConfig, baseURL string) (*Client, error) {
	if cfg.Type != domain.LLMProviderAnthropic {
		return nil, fmt.Errorf("invalid LLM provider type for Anthropic client: %s", cfg.Type)
	}
	if cfg.APIKey.Value() == "" {
		return nil, errors.New("Anthropic API key is required")
	}

	anthropicClient := anthropicsdk.NewClient(
		option.WithAPIKey(cfg.APIKey.Value()),
		option.WithBaseURL(baseURL),
	)

	return &Client{
		client: &anthropicClient,
		cfg:    cfg,
	}, nil
}

// Complete performs a completion request to the Anthropic API.
func (c *Client) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if provider != domain.LLMProviderAnthropic {
		return nil, fmt.Errorf("Anthropic client cannot handle provider: %s", provider)
	}

	// Build message parameters
	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  convertMessages(req.Messages),
	}

	// Add temperature if non-zero
	if req.Temperature > 0 {
		params.Temperature = anthropicsdk.Float(req.Temperature)
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{
				Type: "text",
				Text: req.SystemPrompt,
			},
		}
	}

	// Add tool definitions if provided
	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	// Make API call
	response, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API call failed: %w", err)
	}

	// Convert response to domain format
	return convertResponse(response), nil
}

// Stream performs a streaming completion to the Anthropic API.
func (c *Client) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	if provider != domain.LLMProviderAnthropic {
		return nil, fmt.Errorf("Anthropic client cannot handle provider: %s", provider)
	}

	// Build message parameters
	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  convertMessages(req.Messages),
	}

	// Add temperature if non-zero
	if req.Temperature > 0 {
		params.Temperature = anthropicsdk.Float(req.Temperature)
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{
				Type: "text",
				Text: req.SystemPrompt,
			},
		}
	}

	// Add tool definitions if provided
	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	// Create streaming request
	stream := c.client.Messages.NewStreaming(ctx, params)

	// Create output channel
	out := make(chan domain.StreamChunk, 10)

	// Start goroutine to process stream
	go func() {
		defer close(out)

		// Process stream events
		for stream.Next() {
			event := stream.Current()

			// Handle text deltas
			if event.Type == eventTypeContentBlockDelta {
				if event.Delta.Type == deltaTypeText {
					out <- domain.StreamChunk{
						Delta: event.Delta.Text,
					}
				}
			}

			// Handle completion
			if event.Type == eventTypeMessageStop {
				out <- domain.StreamChunk{
					Done: true,
				}
				return
			}
		}

		// Check for errors
		if err := stream.Err(); err != nil {
			out <- domain.StreamChunk{
				Error: fmt.Errorf("streaming error: %w", err),
			}
		}
	}()

	return out, nil
}

// ListModels returns available models for Anthropic.
func (c *Client) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	if provider != domain.LLMProviderAnthropic {
		return nil, fmt.Errorf("Anthropic client cannot handle provider: %s", provider)
	}

	// Anthropic doesn't have a models API endpoint yet,
	// so we return a hardcoded list of known models
	models := []domain.ModelInfo{
		{
			ID:            "claude-3-5-sonnet-20241022",
			Name:          "Claude 3.5 Sonnet",
			Provider:      "anthropic",
			ContextWindow: 200000,
		},
		{
			ID:            "claude-3-opus-20240229",
			Name:          "Claude 3 Opus",
			Provider:      "anthropic",
			ContextWindow: 200000,
		},
		{
			ID:            "claude-3-sonnet-20240229",
			Name:          "Claude 3 Sonnet",
			Provider:      "anthropic",
			ContextWindow: 200000,
		},
		{
			ID:            "claude-3-haiku-20240307",
			Name:          "Claude 3 Haiku",
			Provider:      "anthropic",
			ContextWindow: 200000,
		},
	}

	return models, nil
}
