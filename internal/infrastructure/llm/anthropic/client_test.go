package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// TestComplete_Success tests successful completion without tools
func TestComplete_Success(t *testing.T) {
	// Create mock Anthropic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Check API key header
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-api-key" {
			t.Errorf("Expected x-api-key header with test-api-key, got %s", apiKey)
		}

		// Return mock response
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Hello! How can I help you today?",
				},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test Complete()
	req := &domain.LLMRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: 0.7,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	// Verify response
	if response.Content != "Hello! How can I help you today?" {
		t.Errorf("Expected content 'Hello! How can I help you today?', got %s", response.Content)
	}
	if response.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 20 {
		t.Errorf("Expected 20 completion tokens, got %d", response.Usage.CompletionTokens)
	}
	if response.FinishReason != "end_turn" {
		t.Errorf("Expected finish reason 'end_turn', got %s", response.FinishReason)
	}
}

// TestComplete_WithSystemPrompt tests completion with system prompt
func TestComplete_WithSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to verify system prompt
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Check if system prompt is present
		system, ok := reqBody["system"]
		if !ok {
			t.Error("Expected system field in request")
		}

		// Verify system prompt format
		if systemSlice, ok := system.([]interface{}); ok {
			if len(systemSlice) > 0 {
				if block, ok := systemSlice[0].(map[string]interface{}); ok {
					if block["type"] != "text" {
						t.Errorf("Expected system type 'text', got %v", block["type"])
					}
					if block["text"] != "You are a helpful assistant." {
						t.Errorf("Expected system text 'You are a helpful assistant.', got %v", block["text"])
					}
				}
			}
		}

		// Return mock response
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{"type": "text", "text": "I'm here to help!"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  15,
				"output_tokens": 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:        "claude-3-sonnet-20240229",
		MaxTokens:    1024,
		Temperature:  0.7,
		SystemPrompt: "You are a helpful assistant.",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() with system prompt failed: %v", err)
	}

	if response.Content != "I'm here to help!" {
		t.Errorf("Expected content 'I'm here to help!', got %s", response.Content)
	}
}

// TestComplete_WithTools tests completion with tool definitions
func TestComplete_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to verify tools
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Check if tools are present
		tools, ok := reqBody["tools"]
		if !ok {
			t.Error("Expected tools field in request")
		}

		toolsSlice, ok := tools.([]interface{})
		if !ok || len(toolsSlice) == 0 {
			t.Error("Expected non-empty tools array")
		}

		// Return response with tool use
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   "tool_123",
					"name": "calculator",
					"input": map[string]interface{}{
						"operation": "add",
						"a":         5,
						"b":         3,
					},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  20,
				"output_tokens": 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: 0.7,
		Messages: []domain.Message{
			{Role: "user", Content: "What is 5 + 3?"},
		},
		Tools: []domain.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Performs basic arithmetic operations",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"operation": map[string]any{"type": "string"},
						"a":         map[string]any{"type": "number"},
						"b":         map[string]any{"type": "number"},
					},
				},
			},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() with tools failed: %v", err)
	}

	// Verify tool call
	if len(response.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}
	if response.ToolCalls[0].ToolName != "calculator" {
		t.Errorf("Expected tool name 'calculator', got %s", response.ToolCalls[0].ToolName)
	}
	if response.FinishReason != "tool_use" {
		t.Errorf("Expected finish reason 'tool_use', got %s", response.FinishReason)
	}
}

// TestComplete_APIError tests error handling
func TestComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("invalid-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	_, err = client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Expected error for invalid API key, got nil")
	}
}

// TestComplete_InvalidProvider tests provider validation
func TestComplete_InvalidProvider(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:     "gpt-4",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	// Try with wrong provider
	_, err = client.Complete(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error for wrong provider, got nil")
	}
}

// TestListModels_Success tests model listing
func TestListModels_Success(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	models, err := client.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	// Should return at least Claude models
	if len(models) == 0 {
		t.Error("Expected at least one model, got none")
	}

	// Check for expected models
	foundSonnet := false
	for _, model := range models {
		if model.ID == "claude-3-5-sonnet-20241022" || model.ID == "claude-3-sonnet-20240229" {
			foundSonnet = true
			if model.Provider != "anthropic" {
				t.Errorf("Expected provider 'anthropic', got %s", model.Provider)
			}
			if model.ContextWindow <= 0 {
				t.Error("Expected positive context window")
			}
		}
	}
	if !foundSonnet {
		t.Error("Expected to find Claude Sonnet model in list")
	}
}

// TestNewClient_InvalidConfig tests client creation validation
func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.LLMProviderConfig
		wantErr bool
	}{
		{
			name: "missing API key",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderAnthropic,
				APIKey: domain.NewSecureStringFromString(""),
			},
			wantErr: true,
		},
		{
			name: "wrong provider type",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderOpenAI,
				APIKey: domain.NewSecureStringFromString("test-key"),
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderAnthropic,
				APIKey: domain.NewSecureStringFromString("test-key"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
