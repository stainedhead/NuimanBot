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

func TestComplete_WrongProvider(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
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
	}

	// Try with wrong provider
	_, err := client.Complete(ctx, domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Expected error with wrong provider, got nil")
	}
}

func TestStream_WrongProvider(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
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
	}

	// Try with wrong provider
	_, err := client.Stream(ctx, domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Expected error with wrong provider, got nil")
	}
}

func TestListModels_WrongProvider(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	// Try with wrong provider
	_, err := client.ListModels(ctx, domain.LLMProviderAnthropic)
	if err == nil {
		t.Error("Expected error with wrong provider, got nil")
	}
}

func TestNew_WithCustomConfig(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      "https://custom.openai.com/v1",
		DefaultModel: "gpt-4-turbo",
		Organization: "org-custom",
	}

	client := openai.New(cfg)

	if client == nil {
		t.Fatal("New() returned nil client with custom config")
	}
}

func TestNew_WithMinimalConfig(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)

	if client == nil {
		t.Fatal("New() returned nil client with minimal config")
	}
}

func TestComplete_WithTools(t *testing.T) {
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
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
		Tools: []domain.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
		MaxTokens:   200,
		Temperature: 0.5,
	}

	// Will fail with invalid API key, but tests the conversion path
	_, err := client.Complete(ctx, domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestStream_WithTools(t *testing.T) {
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
			{Role: "user", Content: "Calculate 5+3"},
		},
		Tools: []domain.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Perform calculations",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{"type": "string"},
					},
				},
			},
		},
		MaxTokens:   150,
		Temperature: 0.3,
	}

	// Will fail with invalid API key, but tests the setup path
	_, err := client.Stream(ctx, domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestListModels(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-invalid-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	// Will fail with invalid API key
	_, err := client.ListModels(ctx, domain.LLMProviderOpenAI)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestComplete_WithSystemPrompt(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-invalid-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model:        "gpt-4o",
		SystemPrompt: "You are a helpful assistant",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Will fail with invalid API key, but tests system prompt conversion
	_, err := client.Complete(ctx, domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestStream_WithSystemPrompt(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-invalid-key"),
		BaseURL:      "https://api.openai.com/v1",
		DefaultModel: "gpt-4o",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model:        "gpt-4o",
		SystemPrompt: "You are a helpful assistant",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	// Will fail with invalid API key, but tests system prompt conversion
	_, err := client.Stream(ctx, domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}
