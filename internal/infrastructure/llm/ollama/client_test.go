package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/llm/ollama"
)

func TestNew(t *testing.T) {
	cfg := &config.OllamaProviderConfig{
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama2",
	}

	client := ollama.New(cfg)

	if client == nil {
		t.Fatal("New() returned nil client")
	}
}

func TestComplete(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected /api/chat path, got %s", r.URL.Path)
		}

		// Send mock response
		resp := map[string]any{
			"model":   "llama2",
			"message": map[string]string{"role": "assistant", "content": "Hello! How can I help you?"},
			"done":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama2",
	}

	client := ollama.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "llama2",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := client.Complete(ctx, domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Complete() returned nil response")
	}

	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("Expected content 'Hello! How can I help you?', got '%s'", resp.Content)
	}
}

func TestStream(t *testing.T) {
	// Create mock HTTP server that returns line-delimited JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected /api/chat path, got %s", r.URL.Path)
		}

		// Send streaming response (line-delimited JSON)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		// Send chunks
		flusher := w.(http.Flusher)
		chunks := []string{
			`{"model":"llama2","message":{"role":"assistant","content":"Hello"},"done":false}`,
			`{"model":"llama2","message":{"role":"assistant","content":" there"},"done":false}`,
			`{"model":"llama2","message":{"role":"assistant","content":"!"},"done":true}`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama2",
	}

	client := ollama.New(cfg)
	ctx := context.Background()

	req := &domain.LLMRequest{
		Model: "llama2",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	stream, err := client.Stream(ctx, domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Stream() returned error: %v", err)
	}

	if stream == nil {
		t.Fatal("Stream() returned nil channel")
	}

	// Collect chunks
	var content string
	doneReceived := false
	for chunk := range stream {
		if chunk.Error != nil {
			t.Errorf("Received error chunk: %v", chunk.Error)
		}
		if chunk.Delta != "" {
			content += chunk.Delta
		}
		if chunk.Done {
			doneReceived = true
		}
	}

	expectedContent := "Hello there!"
	if content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
	}

	if !doneReceived {
		t.Error("Never received done signal")
	}
}
