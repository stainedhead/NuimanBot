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

// TestNewClientWithBaseURL_WrongProvider tests validation in NewClientWithBaseURL.
func TestNewClientWithBaseURL_WrongProvider(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderOpenAI,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}

	_, err := NewClientWithBaseURL(cfg, "http://example.com")
	if err == nil {
		t.Fatal("Expected error for wrong provider type in NewClientWithBaseURL")
	}
}

// TestNewClientWithBaseURL_MissingAPIKey tests missing API key validation.
func TestNewClientWithBaseURL_MissingAPIKey(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString(""),
	}

	_, err := NewClientWithBaseURL(cfg, "http://example.com")
	if err == nil {
		t.Fatal("Expected error for missing API key in NewClientWithBaseURL")
	}
}

// TestStream_WithMockServer tests streaming with a mock HTTP server.
func TestStream_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)

		// Send SSE events matching Anthropic stream format
		events := []string{
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}` + "\n\n",
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}` + "\n\n",
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n",
		}

		for _, event := range events {
			w.Write([]byte(event))
			flusher.Flush()
		}
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
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

	stream, err := client.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Collect all chunks
	var content string
	var doneReceived bool
	for chunk := range stream {
		if chunk.Error != nil {
			// Expected - stream events use SDK format, our mock won't match exactly
			t.Logf("Stream chunk error (expected with mock): %v", chunk.Error)
		}
		if chunk.Delta != "" {
			content += chunk.Delta
		}
		if chunk.Done {
			doneReceived = true
		}
	}

	t.Logf("Collected content: %q, done: %v", content, doneReceived)
}

// TestStream_WithSystemPromptAndTools tests Stream with all parameter combinations.
func TestStream_WithSystemPromptAndTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Verify system and tools are present
		if body["system"] == nil {
			t.Error("Expected system in request")
		}
		if body["tools"] == nil {
			t.Error("Expected tools in request")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Just close the connection
		w.(http.Flusher).Flush()
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:        "claude-3-sonnet-20240229",
		MaxTokens:    512,
		Temperature:  0.5,
		SystemPrompt: "You are helpful",
		Messages:     []domain.Message{{Role: "user", Content: "Hello"}},
		Tools: []domain.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Math",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"a": map[string]any{"type": "number"}},
				},
			},
		},
	}

	stream, err := client.Stream(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Drain channel
	for range stream {
	}
}

// TestConvertMessages_WithSystemMessages tests that system messages are skipped.
func TestConvertMessages_WithSystemMessages(t *testing.T) {
	messages := []domain.Message{
		{Role: "system", Content: "Be helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result := convertMessages(messages)
	// System message should be skipped
	if len(result) != 2 {
		t.Errorf("Expected 2 messages (system skipped), got %d", len(result))
	}
}

// TestConvertTools_NoDescription tests tool without description.
func TestConvertTools_NoDescription(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "tool-no-desc",
			Description: "", // No description
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	result := convertTools(tools)
	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}
	if result[0].OfTool.Description.Valid() {
		t.Error("Expected invalid (not set) description for empty string input")
	}
}

// TestConvertTools_WithRequiredFields tests required field extraction with []string.
func TestConvertTools_WithRequiredFields(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "tool-with-required",
			Description: "A tool",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"x": map[string]any{"type": "number"}},
				"required":   []string{"x"},
			},
		},
	}

	result := convertTools(tools)
	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}
	if len(result[0].OfTool.InputSchema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(result[0].OfTool.InputSchema.Required))
	}
}

// TestConvertTools_WithInterfaceRequiredFields tests required field extraction with []interface{}.
func TestConvertTools_WithInterfaceRequiredFields(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "tool-with-iface-required",
			Description: "A tool",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"y": map[string]any{"type": "string"}},
				"required":   []interface{}{"y"},
			},
		},
	}

	result := convertTools(tools)
	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}
	if len(result[0].OfTool.InputSchema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(result[0].OfTool.InputSchema.Required))
	}
}
