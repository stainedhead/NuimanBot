package openai_test

import (
	"context"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/llm/openai"
)

func TestNew(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
		Organization: "org-test",
	}

	client := openai.New(cfg)

	if client == nil {
		t.Fatal("New() returned nil client")
	}
}

func TestComplete(t *testing.T) {
	// This test requires a real API key to run
	// For now, we test that the method exists and returns an error with invalid key
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-invalid-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "gpt-4o",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// We expect this to fail with invalid API key
	_, err := client.Complete(ctx, domain.LLMProviderOpenAI, req)

	// With invalid key, we should get an error
	if err == nil {
		t.Error("Expected error with invalid API key, got nil")
	}
}

func TestStream(t *testing.T) {
	// Test that Stream method exists and returns error with invalid key
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-invalid-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "gpt-4o",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Call Stream - expect error with invalid API key
	_, err := client.Stream(ctx, domain.LLMProviderOpenAI, req)

	// With invalid key, we should get an error
	if err == nil {
		t.Error("Expected error with invalid API key, got nil")
	}
}
