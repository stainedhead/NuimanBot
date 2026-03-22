package bedrock

import (
	"context"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// TestListModels_Success tests successful model listing.
func TestListModels_Success(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	models, err := client.ListModels(context.Background(), domain.LLMProviderBedrock)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) == 0 {
		t.Error("Expected at least one model")
	}

	// Verify all models have required fields
	for _, m := range models {
		if m.ID == "" {
			t.Error("Model ID should not be empty")
		}
		if m.Provider != "bedrock" {
			t.Errorf("Expected provider 'bedrock', got %s", m.Provider)
		}
	}
}

// TestListModels_WrongProvider tests ListModels with wrong provider.
func TestListModels_WrongProvider(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	_, err := client.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err == nil {
		t.Fatal("Expected error for wrong provider")
	}
	if err.Error() != "provider type must be bedrock" {
		t.Errorf("Expected 'provider type must be bedrock', got '%s'", err.Error())
	}
}

// TestStream_WrongProvider tests Stream with wrong provider.
func TestStream_WrongProvider(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet",
		MaxTokens: 100,
		Messages:  []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error for wrong provider")
	}
	if err.Error() != "provider type must be bedrock" {
		t.Errorf("Expected 'provider type must be bedrock', got '%s'", err.Error())
	}
}

// TestBuildConverseInput_WithOptions tests buildConverseInput with MaxTokens and Temperature.
func TestBuildConverseInput_WithOptions(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:        "claude-3-5-sonnet-20241022",
		MaxTokens:    2048,
		Temperature:  0.7,
		SystemPrompt: "You are helpful",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	input, err := client.buildConverseInput(req)
	if err != nil {
		t.Fatalf("buildConverseInput() error = %v", err)
	}

	// Verify model ID was translated
	if *input.ModelId != "us.anthropic.claude-3-5-sonnet-20241022-v2:0" {
		t.Errorf("Expected translated model ID, got %s", *input.ModelId)
	}

	// Verify inference config
	if input.InferenceConfig.MaxTokens == nil {
		t.Fatal("Expected MaxTokens to be set")
	}
	if *input.InferenceConfig.MaxTokens != 2048 {
		t.Errorf("Expected MaxTokens=2048, got %d", *input.InferenceConfig.MaxTokens)
	}
	if input.InferenceConfig.Temperature == nil {
		t.Fatal("Expected Temperature to be set")
	}

	// Verify system blocks
	if len(input.System) != 1 {
		t.Errorf("Expected 1 system block, got %d", len(input.System))
	}
}

// TestBuildConverseInput_WithTools tests buildConverseInput with tools.
func TestBuildConverseInput_WithTools(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "What is 5+3?"}},
		Tools: []domain.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Perform math",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"a": map[string]any{"type": "number"},
					},
				},
			},
		},
	}

	input, err := client.buildConverseInput(req)
	if err != nil {
		t.Fatalf("buildConverseInput() error = %v", err)
	}

	if input.ToolConfig == nil {
		t.Fatal("Expected ToolConfig to be set")
	}
	if len(input.ToolConfig.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(input.ToolConfig.Tools))
	}
}

// TestBuildConverseStreamInput_Basic tests buildConverseStreamInput basic usage.
func TestBuildConverseStreamInput_Basic(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 512,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	input, err := client.buildConverseStreamInput(req)
	if err != nil {
		t.Fatalf("buildConverseStreamInput() error = %v", err)
	}

	if *input.ModelId != "anthropic.claude-3-sonnet-20240229-v1:0" {
		t.Errorf("Expected translated model ID, got %s", *input.ModelId)
	}
}

// TestBuildConverseStreamInput_WithOptionsAndTools tests stream input with all options.
func TestBuildConverseStreamInput_WithOptionsAndTools(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:        "claude-3-haiku-20240307",
		MaxTokens:    256,
		Temperature:  0.5,
		SystemPrompt: "Be concise",
		Messages:     []domain.Message{{Role: "user", Content: "Hello"}},
		Tools: []domain.ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"query": map[string]any{"type": "string"}},
				},
			},
		},
	}

	input, err := client.buildConverseStreamInput(req)
	if err != nil {
		t.Fatalf("buildConverseStreamInput() error = %v", err)
	}

	if len(input.System) != 1 {
		t.Errorf("Expected 1 system block, got %d", len(input.System))
	}
	if input.ToolConfig == nil {
		t.Fatal("Expected ToolConfig to be set")
	}
	if input.InferenceConfig.MaxTokens == nil {
		t.Fatal("Expected MaxTokens to be set")
	}
	if input.InferenceConfig.Temperature == nil {
		t.Fatal("Expected Temperature to be set")
	}
}

// TestBuildLoadOptions_WithProfile tests buildLoadOptions with a profile set.
func TestBuildLoadOptions_WithProfile(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion:  "eu-west-1",
		AWSProfile: "my-profile",
	}

	opts := buildLoadOptions(cfg)
	// Should have 2 options: region + profile
	if len(opts) != 2 {
		t.Errorf("Expected 2 load options, got %d", len(opts))
	}
}

// TestBuildLoadOptions_WithoutProfile tests buildLoadOptions without a profile.
func TestBuildLoadOptions_WithoutProfile(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-west-2",
	}

	opts := buildLoadOptions(cfg)
	// Should have 1 option: region only
	if len(opts) != 1 {
		t.Errorf("Expected 1 load option, got %d", len(opts))
	}
}

// TestConvertMessages_SystemMessage tests that system messages are skipped.
func TestConvertMessages_SystemMessage(t *testing.T) {
	messages := []domain.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
	}

	result, system := convertMessages(messages, "")
	// System role messages are skipped in convertMessages
	if len(result) != 1 {
		t.Errorf("Expected 1 message (system skipped), got %d", len(result))
	}
	if len(system) != 0 {
		t.Errorf("Expected 0 system blocks from messages, got %d", len(system))
	}
}

// TestConvertUsage_NilUsage tests convertUsage with nil.
func TestConvertUsage_NilUsage(t *testing.T) {
	usage := convertUsage(nil)
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 {
		t.Error("Expected zero usage for nil input")
	}
}

// TestConvertResponse_NoOutput tests convertResponse with no Output member.
func TestConvertResponse_NoOutput(t *testing.T) {
	output := &bedrockruntime.ConverseOutput{
		StopReason: types.StopReasonEndTurn,
		Usage:      nil,
	}

	result := convertResponse(output)
	if result.Content != "" {
		t.Errorf("Expected empty content, got %q", result.Content)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("Expected no tool calls, got %d", len(result.ToolCalls))
	}
}

// TestNormalizeStopReason tests stop reason normalization.
func TestNormalizeStopReason(t *testing.T) {
	tests := []struct {
		reason   types.StopReason
		expected string
	}{
		{types.StopReasonEndTurn, "end_turn"},
		{types.StopReasonToolUse, "tool_use"},
		{types.StopReasonMaxTokens, "max_tokens"},
	}

	for _, tt := range tests {
		result := normalizeStopReason(tt.reason)
		if result != tt.expected {
			t.Errorf("normalizeStopReason(%s) = %s, want %s", tt.reason, result, tt.expected)
		}
	}
}

// TestParseToolUseBlock_NoName tests parseToolUseBlock with nil name.
func TestParseToolUseBlock_NoName(t *testing.T) {
	block := types.ToolUseBlock{
		Name:  nil, // No name
		Input: nil, // No input
	}

	result := parseToolUseBlock(block)
	if result.ToolName != "" {
		t.Errorf("Expected empty tool name, got %q", result.ToolName)
	}
	if len(result.Arguments) != 0 {
		t.Errorf("Expected no arguments, got %v", result.Arguments)
	}
}

// TestComplete_WrongProvider_IsChecked verifies provider check in Complete.
func TestComplete_WrongProvider_BedrockOnly(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet",
		MaxTokens: 100,
		Messages:  []domain.Message{{Role: "user", Content: "hello"}},
	}

	for _, p := range []domain.LLMProvider{
		domain.LLMProviderAnthropic,
		domain.LLMProviderOpenAI,
		domain.LLMProviderOllama,
	} {
		_, err := client.Complete(context.Background(), p, req)
		if err == nil {
			t.Errorf("Expected error for provider %s", p)
		}
	}
}
