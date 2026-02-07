package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	openai "github.com/sashabaranov/go-openai"
)

// Client wraps the OpenAI SDK client and implements the LLM provider interface.
type Client struct {
	client *openai.Client
	config *config.OpenAIProviderConfig
}

// New creates a new OpenAI client with the provided configuration.
func New(cfg *config.OpenAIProviderConfig) *Client {
	clientConfig := openai.DefaultConfig(cfg.APIKey.Value())

	// Set custom base URL if provided
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	// Set organization if provided
	if cfg.Organization != "" {
		clientConfig.OrgID = cfg.Organization
	}

	return &Client{
		client: openai.NewClientWithConfig(clientConfig),
		config: cfg,
	}
}

// Complete performs a completion request to the OpenAI API.
func (c *Client) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if provider != domain.LLMProviderOpenAI {
		return nil, fmt.Errorf("OpenAI client cannot handle provider: %s", provider)
	}

	// Convert domain.LLMRequest to openai.ChatCompletionRequest
	oaiReq := c.convertRequest(req)

	// Make API call
	resp, err := c.client.CreateChatCompletion(ctx, oaiReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	// Convert response to domain.LLMResponse
	return c.convertResponse(&resp), nil
}

// Stream performs a streaming completion request to the OpenAI API.
func (c *Client) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	if provider != domain.LLMProviderOpenAI {
		return nil, fmt.Errorf("OpenAI client cannot handle provider: %s", provider)
	}

	// Convert domain.LLMRequest to openai.ChatCompletionRequest
	oaiReq := c.convertRequest(req)
	oaiReq.Stream = true // Enable streaming

	// Create stream
	stream, err := c.client.CreateChatCompletionStream(ctx, oaiReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API streaming error: %w", err)
	}

	// Create output channel
	outChan := make(chan domain.StreamChunk, 10)

	// Process stream in goroutine
	go func() {
		defer close(outChan)
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if err != nil {
				// Check if stream is done (io.EOF)
				if errors.Is(err, io.EOF) {
					outChan <- domain.StreamChunk{Done: true}
					return
				}
				// Send error chunk
				outChan <- domain.StreamChunk{Error: fmt.Errorf("stream error: %w", err)}
				return
			}

			// Extract delta from response
			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta

				// Send content delta if present
				if delta.Content != "" {
					outChan <- domain.StreamChunk{Delta: delta.Content}
				}

				// Send tool call delta if present
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						// Parse arguments if present
						var args map[string]any
						if tc.Function.Arguments != "" {
							if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
								args = map[string]any{"_raw": tc.Function.Arguments}
							}
						}

						outChan <- domain.StreamChunk{
							ToolCall: &domain.ToolCall{
								ToolName:  tc.Function.Name,
								Arguments: args,
							},
						}
					}
				}

				// Check finish reason
				if resp.Choices[0].FinishReason != "" {
					outChan <- domain.StreamChunk{Done: true}
					return
				}
			}
		}
	}()

	return outChan, nil
}

// ListModels returns available models for OpenAI.
func (c *Client) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	if provider != domain.LLMProviderOpenAI {
		return nil, fmt.Errorf("OpenAI client cannot handle provider: %s", provider)
	}

	// List models from OpenAI API
	models, err := c.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list OpenAI models: %w", err)
	}

	// Convert to domain.ModelInfo
	result := make([]domain.ModelInfo, 0, len(models.Models))
	for _, model := range models.Models {
		result = append(result, domain.ModelInfo{
			ID:       model.ID,
			Name:     model.ID, // OpenAI doesn't provide separate name
			Provider: "openai",
			// Context window info not provided by ListModels API
			ContextWindow: 0,
		})
	}

	return result, nil
}

// convertRequest converts domain.LLMRequest to openai.ChatCompletionRequest
func (c *Client) convertRequest(req *domain.LLMRequest) openai.ChatCompletionRequest {
	// Convert messages
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: req.SystemPrompt,
		})
	}

	// Add conversation messages
	for _, msg := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
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
	oaiReq := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	// Set optional parameters
	if req.MaxTokens > 0 {
		oaiReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		oaiReq.Temperature = float32(req.Temperature)
	}

	// Convert tools if provided
	if len(req.Tools) > 0 {
		oaiReq.Tools = c.convertTools(req.Tools)
	}

	return oaiReq
}

// convertTools converts domain.ToolDefinition to openai.Tool
func (c *Client) convertTools(tools []domain.ToolDefinition) []openai.Tool {
	oaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		oaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}
	return oaiTools
}

// convertResponse converts openai.ChatCompletionResponse to domain.LLMResponse
func (c *Client) convertResponse(resp *openai.ChatCompletionResponse) *domain.LLMResponse {
	result := &domain.LLMResponse{
		Usage: domain.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Extract content and tool calls from first choice
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Content = choice.Message.Content
		result.FinishReason = string(choice.FinishReason)

		// Convert tool calls if present
		if len(choice.Message.ToolCalls) > 0 {
			result.ToolCalls = c.convertToolCalls(choice.Message.ToolCalls)
		}
	}

	return result
}

// convertToolCalls converts OpenAI tool calls to domain.ToolCall
func (c *Client) convertToolCalls(toolCalls []openai.ToolCall) []domain.ToolCall {
	result := make([]domain.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		// Parse arguments JSON to map
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			// If parsing fails, store the raw string
			args = map[string]any{"_raw": tc.Function.Arguments, "_parse_error": err.Error()}
		}

		result[i] = domain.ToolCall{
			ToolName:  tc.Function.Name,
			Arguments: args,
		}
	}
	return result
}
