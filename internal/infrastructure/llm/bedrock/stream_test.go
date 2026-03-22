package bedrock

import (
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// newTestClient creates a test bedrock client with a mock AWS config.
func newTestClient() *Client {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	return NewClientWithConfig(cfg, awsCfg)
}

// TestConverseStreamChunk_ContentDelta verifies domain.StreamChunk structure.
func TestConverseStreamChunk_ContentDelta(t *testing.T) {
	chunk := domain.StreamChunk{
		Delta: "hello",
		Done:  false,
	}

	if chunk.Delta != "hello" {
		t.Errorf("Expected delta 'hello', got %s", chunk.Delta)
	}
	if chunk.Done {
		t.Error("Expected Done=false")
	}
}

// TestConverseStreamChunk_Done verifies Done stream chunk.
func TestConverseStreamChunk_Done(t *testing.T) {
	chunk := domain.StreamChunk{
		Done: true,
	}

	if !chunk.Done {
		t.Error("Expected Done=true")
	}
	if chunk.Delta != "" {
		t.Errorf("Expected empty delta, got %s", chunk.Delta)
	}
}

// TestBuildConverseInput_NoOptions tests buildConverseInput without optional fields.
func TestBuildConverseInput_NoOptions(t *testing.T) {
	// MaxTokens=0 and Temperature=0 should not set inference config fields
	client := newTestClient()
	input, err := client.buildConverseInput(&domain.LLMRequest{
		Model:     "unknown-model",
		MaxTokens: 0, // Should not set MaxTokens in InferenceConfig
		Messages:  []domain.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("buildConverseInput error: %v", err)
	}
	if input.InferenceConfig.MaxTokens != nil {
		t.Error("Expected MaxTokens to be nil when not set")
	}
	if input.InferenceConfig.Temperature != nil {
		t.Error("Expected Temperature to be nil when not set")
	}
	if input.ToolConfig != nil {
		t.Error("Expected nil ToolConfig when no tools")
	}
}

// TestBuildConverseStreamInput_NoOptions tests buildConverseStreamInput without optional fields.
func TestBuildConverseStreamInput_NoOptions(t *testing.T) {
	client := newTestClient()
	input, err := client.buildConverseStreamInput(&domain.LLMRequest{
		Model:     "unknown-model",
		MaxTokens: 0,
		Messages:  []domain.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("buildConverseStreamInput error: %v", err)
	}
	if input.InferenceConfig.MaxTokens != nil {
		t.Error("Expected MaxTokens to be nil when not set")
	}
	if input.ToolConfig != nil {
		t.Error("Expected nil ToolConfig when no tools")
	}
}
