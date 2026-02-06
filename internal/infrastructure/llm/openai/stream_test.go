package openai_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/llm/openai"
)

func TestStream_WithMockServer(t *testing.T) {
	// Create mock server that returns streaming SSE responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a streaming request
		if !strings.Contains(r.URL.Path, "/chat/completions") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Send streaming responses
		responses := []string{
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, resp := range responses {
			w.Write([]byte(resp + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	// Create client with mock server
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      server.URL + "/v1",
		DefaultModel: "gpt-4",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Stream
	stream, err := client.Stream(ctx, domain.LLMProviderOpenAI, req)
	if err != nil {
		t.Fatalf("Stream() returned error: %v", err)
	}

	// Collect chunks
	var chunks []domain.StreamChunk
	for chunk := range stream {
		chunks = append(chunks, chunk)
	}

	// Verify we got some chunks
	if len(chunks) == 0 {
		t.Error("Expected to receive stream chunks")
	}

	// Check for done chunk
	foundDone := false
	for _, chunk := range chunks {
		if chunk.Done {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Error("Expected to receive done chunk")
	}
}

func TestStream_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      server.URL + "/v1",
		DefaultModel: "gpt-4",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Should return error immediately
	_, err := client.Stream(ctx, domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error from server, got nil")
	}
}

func TestComplete_WithMockServer(t *testing.T) {
	// Create mock server that returns completion response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if !strings.Contains(r.URL.Path, "/chat/completions") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		// Return completion response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1694268190,
			"model": "gpt-4",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help you?"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 8,
				"total_tokens": 18
			}
		}`))
	}))
	defer server.Close()

	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      server.URL + "/v1",
		DefaultModel: "gpt-4",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Complete
	resp, err := client.Complete(ctx, domain.LLMProviderOpenAI, req)
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}

	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("Content = %s, want 'Hello! How can I help you?'", resp.Content)
	}

	if resp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}

	if resp.Usage.CompletionTokens != 8 {
		t.Errorf("CompletionTokens = %d, want 8", resp.Usage.CompletionTokens)
	}
}

func TestListModels_WithMockServer(t *testing.T) {
	// Create mock server that returns models list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/models") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "gpt-4", "object": "model", "created": 1687882410, "owned_by": "openai"},
				{"id": "gpt-3.5-turbo", "object": "model", "created": 1677610602, "owned_by": "openai"}
			]
		}`))
	}))
	defer server.Close()

	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test-key"),
		BaseURL:      server.URL + "/v1",
		DefaultModel: "gpt-4",
	}

	client := openai.New(cfg)
	ctx := context.Background()

	models, err := client.ListModels(ctx, domain.LLMProviderOpenAI)
	if err != nil {
		t.Fatalf("ListModels() returned error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}

	if models[0].ID != "gpt-4" {
		t.Errorf("Model 0 ID = %s, want gpt-4", models[0].ID)
	}

	if models[1].ID != "gpt-3.5-turbo" {
		t.Errorf("Model 1 ID = %s, want gpt-3.5-turbo", models[1].ID)
	}
}

func parseSSE(data string) map[string]interface{} {
	// Simple SSE parser for testing
	result := make(map[string]interface{})
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			result["data"] = strings.TrimPrefix(line, "data: ")
		}
	}
	return result
}
