package bedrock

import (
	"context"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// TestNewClient_Success tests successful client creation with valid config
func TestNewClient_Success(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion:      "us-east-1",
		DefaultModel:   "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
		MaxRetries:     3,
		RequestTimeout: 120,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.cfg != cfg {
		t.Error("Client config not set correctly")
	}
}

// TestNewClient_MissingRegion tests error when AWS region is missing
func TestNewClient_MissingRegion(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "", // Missing region
	}

	client, err := NewClient(cfg)
	if err == nil {
		t.Fatal("Expected error for missing AWS region, got nil")
	}

	if client != nil {
		t.Error("Expected nil client on error")
	}

	expectedMsg := "AWS region is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestNewClientWithConfig tests client creation with custom AWS config
func TestNewClientWithConfig(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion:    "us-west-2",
		DefaultModel: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
	}

	// Create a mock AWS config
	awsCfg := aws.Config{
		Region: "us-west-2",
	}

	client := NewClientWithConfig(cfg, awsCfg)
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.cfg != cfg {
		t.Error("Client config not set correctly")
	}
}

// TestComplete_WrongProvider tests rejection when wrong provider type is passed
func TestComplete_WrongProvider(t *testing.T) {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}

	awsCfg := aws.Config{Region: "us-east-1"}
	client := NewClientWithConfig(cfg, awsCfg)

	req := &domain.LLMRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Pass wrong provider type
	_, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error for wrong provider type, got nil")
	}

	expectedMsg := "provider type must be bedrock"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
