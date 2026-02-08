package bedrock

import (
	"context"
	"errors"
	"fmt"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Client implements domain.LLMService for AWS Bedrock.
type Client struct {
	client *bedrockruntime.Client
	cfg    *config.BedrockProviderConfig
}

// NewClient creates a new Bedrock client with AWS credential resolution.
// It uses the AWS SDK default credential chain which checks:
// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
// 2. Shared credentials file (~/.aws/credentials)
// 3. IAM role for EC2/ECS/Lambda
func NewClient(cfg *config.BedrockProviderConfig) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	// Load AWS configuration from default credential chain
	awsCfg, err := loadAWSConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return NewClientWithConfig(cfg, awsCfg), nil
}

// NewClientWithConfig creates a client with a custom AWS config (for testing).
func NewClientWithConfig(cfg *config.BedrockProviderConfig, awsCfg aws.Config) *Client {
	brClient := bedrockruntime.NewFromConfig(awsCfg)

	return &Client{
		client: brClient,
		cfg:    cfg,
	}
}

// Complete sends a completion request to Bedrock using the Converse API.
func (c *Client) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if provider != domain.LLMProviderBedrock {
		return nil, errors.New("provider type must be bedrock")
	}

	// Build Converse input
	input, err := c.buildConverseInput(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build converse input: %w", err)
	}

	// Call Bedrock Converse API
	output, err := c.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse failed: %w", err)
	}

	// Convert response to domain format
	response := convertResponse(output)
	return response, nil
}

// Stream sends a streaming completion request to Bedrock.
func (c *Client) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	if provider != domain.LLMProviderBedrock {
		return nil, errors.New("provider type must be bedrock")
	}

	// Build Converse input (same as Complete)
	input, err := c.buildConverseStreamInput(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build converse stream input: %w", err)
	}

	// Call Bedrock ConverseStream API
	output, err := c.client.ConverseStream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse stream failed: %w", err)
	}

	// Create output channel
	chunkChan := make(chan domain.StreamChunk, 10)

	// Process stream in goroutine
	go c.processStreamEvents(output.GetStream(), chunkChan)

	return chunkChan, nil
}

// ListModels returns available Bedrock models.
func (c *Client) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	if provider != domain.LLMProviderBedrock {
		return nil, errors.New("provider type must be bedrock")
	}

	// Return known Claude models available on Bedrock
	// Note: Bedrock doesn't have a simple ListModels API like other providers
	// This returns the models we support
	models := []domain.ModelInfo{
		{
			ID:            "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
			Name:          "Claude 3.5 Sonnet (v2)",
			Provider:      "bedrock",
			ContextWindow: 200000,
		},
		{
			ID:            "anthropic.claude-3-5-sonnet-20240620-v1:0",
			Name:          "Claude 3.5 Sonnet (v1)",
			Provider:      "bedrock",
			ContextWindow: 200000,
		},
		{
			ID:            "anthropic.claude-3-opus-20240229-v1:0",
			Name:          "Claude 3 Opus",
			Provider:      "bedrock",
			ContextWindow: 200000,
		},
		{
			ID:            "anthropic.claude-3-sonnet-20240229-v1:0",
			Name:          "Claude 3 Sonnet",
			Provider:      "bedrock",
			ContextWindow: 200000,
		},
		{
			ID:            "anthropic.claude-3-haiku-20240307-v1:0",
			Name:          "Claude 3 Haiku",
			Provider:      "bedrock",
			ContextWindow: 200000,
		},
	}

	return models, nil
}

// validateConfig validates the Bedrock configuration.
func validateConfig(cfg *config.BedrockProviderConfig) error {
	if cfg.AWSRegion == "" {
		return errors.New("AWS region is required")
	}
	return nil
}

// loadAWSConfig loads AWS configuration with credential chain resolution.
func loadAWSConfig(ctx context.Context, cfg *config.BedrockProviderConfig) (aws.Config, error) {
	opts := buildLoadOptions(cfg)

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS default config: %w", err)
	}

	return awsCfg, nil
}

// buildLoadOptions builds AWS SDK load options from Bedrock config.
func buildLoadOptions(cfg *config.BedrockProviderConfig) []func(*awsconfig.LoadOptions) error {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
	}

	if cfg.AWSProfile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.AWSProfile))
	}

	return opts
}

// buildConverseInput builds a ConverseInput from domain.LLMRequest.
func (c *Client) buildConverseInput(req *domain.LLMRequest) (*bedrockruntime.ConverseInput, error) {
	// Translate model ID
	modelID := translateModelID(req.Model)

	// Convert messages and system prompt
	messages, systemBlocks := convertMessages(req.Messages, req.SystemPrompt)

	// Build inference configuration
	inferenceConfig := &types.InferenceConfiguration{}
	if req.MaxTokens > 0 {
		maxTokens := int32(req.MaxTokens)
		inferenceConfig.MaxTokens = &maxTokens
	}
	if req.Temperature > 0 {
		temperature := float32(req.Temperature)
		inferenceConfig.Temperature = &temperature
	}

	// Build input
	input := &bedrockruntime.ConverseInput{
		ModelId:         aws.String(modelID),
		Messages:        messages,
		InferenceConfig: inferenceConfig,
	}

	// Add system blocks if present
	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		tools := convertTools(req.Tools)
		if len(tools) > 0 {
			input.ToolConfig = &types.ToolConfiguration{
				Tools: tools,
			}
		}
	}

	return input, nil
}

// buildConverseStreamInput builds a ConverseStreamInput from domain.LLMRequest.
func (c *Client) buildConverseStreamInput(req *domain.LLMRequest) (*bedrockruntime.ConverseStreamInput, error) {
	// Translate model ID
	modelID := translateModelID(req.Model)

	// Convert messages and system prompt
	messages, systemBlocks := convertMessages(req.Messages, req.SystemPrompt)

	// Build inference configuration
	inferenceConfig := &types.InferenceConfiguration{}
	if req.MaxTokens > 0 {
		maxTokens := int32(req.MaxTokens)
		inferenceConfig.MaxTokens = &maxTokens
	}
	if req.Temperature > 0 {
		temperature := float32(req.Temperature)
		inferenceConfig.Temperature = &temperature
	}

	// Build input
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:         aws.String(modelID),
		Messages:        messages,
		InferenceConfig: inferenceConfig,
	}

	// Add system blocks if present
	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		tools := convertTools(req.Tools)
		if len(tools) > 0 {
			input.ToolConfig = &types.ToolConfiguration{
				Tools: tools,
			}
		}
	}

	return input, nil
}

// processStreamEvents processes the Bedrock stream events.
func (c *Client) processStreamEvents(stream *bedrockruntime.ConverseStreamEventStream, chunkChan chan<- domain.StreamChunk) {
	defer close(chunkChan)

	for event := range stream.Events() {
		switch e := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			// Handle content delta
			if delta := e.Value.Delta; delta != nil {
				if textDelta, ok := delta.(*types.ContentBlockDeltaMemberText); ok {
					chunkChan <- domain.StreamChunk{
						Delta: textDelta.Value,
						Done:  false,
					}
				}
			}

		case *types.ConverseStreamOutputMemberMessageStop:
			// Message completed
			chunkChan <- domain.StreamChunk{
				Done: true,
			}
			return

		case *types.ConverseStreamOutputMemberMetadata:
			// Metadata event - continue processing
			continue
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		chunkChan <- domain.StreamChunk{
			Error: fmt.Errorf("stream error: %w", err),
			Done:  true,
		}
	}
}
