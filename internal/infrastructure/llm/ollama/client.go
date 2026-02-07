package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// Client wraps an HTTP client for Ollama API calls.
type Client struct {
	httpClient *http.Client
	config     *config.OllamaProviderConfig
}

// New creates a new Ollama client with the provided configuration.
func New(cfg *config.OllamaProviderConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Ollama can be slow for large models
		},
		config: cfg,
	}
}

// ollamaChatRequest represents an Ollama /api/chat request
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

// ollamaMessage represents a message in Ollama format
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatResponse represents an Ollama /api/chat response
type ollamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// Complete performs a completion request to the Ollama API.
func (c *Client) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if provider != domain.LLMProviderOllama {
		return nil, fmt.Errorf("Ollama client cannot handle provider: %s", provider)
	}

	// Convert domain.LLMRequest to Ollama format
	ollamaReq := c.convertRequest(req)

	// Make HTTP POST to /api/chat
	url := fmt.Sprintf("%s/api/chat", c.config.BaseURL)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort read for error message
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to domain.LLMResponse
	return c.convertResponse(&ollamaResp), nil
}

// convertRequest converts domain.LLMRequest to Ollama format
func (c *Client) convertRequest(req *domain.LLMRequest) ollamaChatRequest {
	// Convert messages
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Add conversation messages
	for _, msg := range req.Messages {
		messages = append(messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Use default model if not specified
	model := req.Model
	if model == "" && c.config.DefaultModel != "" {
		model = c.config.DefaultModel
	}

	// Build request
	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	// Set options
	options := make(map[string]any)
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}
	if len(options) > 0 {
		ollamaReq.Options = options
	}

	return ollamaReq
}

// convertResponse converts Ollama response to domain.LLMResponse
func (c *Client) convertResponse(resp *ollamaChatResponse) *domain.LLMResponse {
	return &domain.LLMResponse{
		Content: resp.Message.Content,
		// Ollama doesn't provide token usage in non-streaming mode
		Usage: domain.TokenUsage{},
		// Ollama doesn't provide finish_reason in this format
		FinishReason: "stop",
	}
}

// Stream performs a streaming completion request to the Ollama API.
func (c *Client) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	if provider != domain.LLMProviderOllama {
		return nil, fmt.Errorf("Ollama client cannot handle provider: %s", provider)
	}

	// Convert domain.LLMRequest to Ollama format
	ollamaReq := c.convertRequest(req)
	ollamaReq.Stream = true // Enable streaming

	// Make HTTP POST to /api/chat
	url := fmt.Sprintf("%s/api/chat", c.config.BaseURL)
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort read for error message
		resp.Body.Close()
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Create output channel
	outChan := make(chan domain.StreamChunk, 10)

	// Process stream in goroutine
	go func() {
		defer close(outChan)
		defer func() { _ = resp.Body.Close() }()

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk ollamaChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					outChan <- domain.StreamChunk{Done: true}
					return
				}
				outChan <- domain.StreamChunk{Error: fmt.Errorf("stream decode error: %w", err)}
				return
			}

			// Send content delta
			if chunk.Message.Content != "" {
				outChan <- domain.StreamChunk{Delta: chunk.Message.Content}
			}

			// Check if done
			if chunk.Done {
				outChan <- domain.StreamChunk{Done: true}
				return
			}
		}
	}()

	return outChan, nil
}

// ListModels returns available models from Ollama.
func (c *Client) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	if provider != domain.LLMProviderOllama {
		return nil, fmt.Errorf("Ollama client cannot handle provider: %s", provider)
	}

	// List models from Ollama API
	url := fmt.Sprintf("%s/api/tags", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list Ollama models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) //nolint:errcheck // Best effort read for error message
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to domain.ModelInfo
	models := make([]domain.ModelInfo, len(result.Models))
	for i, model := range result.Models {
		models[i] = domain.ModelInfo{
			ID:       model.Name,
			Name:     model.Name,
			Provider: "ollama",
			// Ollama doesn't provide context window info via API
			ContextWindow: 0,
		}
	}

	return models, nil
}
