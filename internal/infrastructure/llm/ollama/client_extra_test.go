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

// TestComplete_WrongProvider tests provider validation.
func TestComplete_WrongProvider(t *testing.T) {
	cfg := &config.OllamaProviderConfig{
		BaseURL:      "http://localhost:11434",
		DefaultModel: "llama2",
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Fatal("Expected error for wrong provider")
	}
}

// TestComplete_HTTPError tests HTTP error handling.
func TestComplete_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama2",
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Complete(context.Background(), domain.LLMProviderOllama, req)
	if err == nil {
		t.Fatal("Expected error for non-200 status")
	}
}

// TestComplete_InvalidJSON tests response with invalid JSON.
func TestComplete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{"))
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama2",
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Complete(context.Background(), domain.LLMProviderOllama, req)
	if err == nil {
		t.Fatal("Expected error for invalid JSON response")
	}
}

// TestComplete_WithSystemPrompt tests request with system prompt.
func TestComplete_WithSystemPrompt(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)

		resp := map[string]any{
			"model":   "llama2",
			"message": map[string]string{"role": "assistant", "content": "Hello!"},
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

	req := &domain.LLMRequest{
		Model:        "llama2",
		SystemPrompt: "You are helpful",
		Messages:     []domain.Message{{Role: "user", Content: "hello"}},
	}

	resp, err := client.Complete(context.Background(), domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Content != "Hello!" {
		t.Errorf("Expected content 'Hello!', got %q", resp.Content)
	}

	// Verify system prompt was added
	messages, ok := capturedBody["messages"].([]interface{})
	if !ok || len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages (system + user), got %v", capturedBody["messages"])
	}
}

// TestComplete_DefaultModel tests that the default model is used when none specified.
func TestComplete_DefaultModel(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)

		resp := map[string]any{
			"model":   "default-model",
			"message": map[string]string{"role": "assistant", "content": "Hello!"},
			"done":    true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "default-model",
	}
	client := ollama.New(cfg)

	// Request with empty model - should use default
	req := &domain.LLMRequest{
		Model:    "", // empty model
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Complete(context.Background(), domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if capturedBody["model"] != "default-model" {
		t.Errorf("Expected default model 'default-model', got %v", capturedBody["model"])
	}
}

// TestListModels_Success tests successful model listing.
func TestListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected /api/tags, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama2", "size": 3825819519},
				{"name": "mistral", "size": 4109865544},
			},
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

	models, err := client.ListModels(context.Background(), domain.LLMProviderOllama)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}
	if models[0].ID != "llama2" {
		t.Errorf("Expected model ID 'llama2', got %s", models[0].ID)
	}
	if models[0].Provider != "ollama" {
		t.Errorf("Expected provider 'ollama', got %s", models[0].Provider)
	}
}

// TestListModels_WrongProvider tests ListModels provider validation.
func TestListModels_WrongProvider(t *testing.T) {
	cfg := &config.OllamaProviderConfig{
		BaseURL: "http://localhost:11434",
	}
	client := ollama.New(cfg)

	_, err := client.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err == nil {
		t.Fatal("Expected error for wrong provider")
	}
}

// TestListModels_HTTPError tests ListModels with HTTP error.
func TestListModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL: server.URL,
	}
	client := ollama.New(cfg)

	_, err := client.ListModels(context.Background(), domain.LLMProviderOllama)
	if err == nil {
		t.Fatal("Expected error for non-200 status")
	}
}

// TestListModels_InvalidJSON tests ListModels with invalid JSON response.
func TestListModels_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL: server.URL,
	}
	client := ollama.New(cfg)

	_, err := client.ListModels(context.Background(), domain.LLMProviderOllama)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

// TestStream_WrongProvider tests Stream provider validation.
func TestStream_WrongProvider(t *testing.T) {
	cfg := &config.OllamaProviderConfig{
		BaseURL: "http://localhost:11434",
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Stream(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Fatal("Expected error for wrong provider in Stream")
	}
}

// TestStream_HTTPError tests Stream with HTTP error.
func TestStream_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL: server.URL,
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	_, err := client.Stream(context.Background(), domain.LLMProviderOllama, req)
	if err == nil {
		t.Fatal("Expected error for non-200 status in Stream")
	}
}

// TestStream_InvalidJSON tests Stream with invalid JSON in stream.
func TestStream_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json\n"))
		// Don't flush - let the body close naturally
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL: server.URL,
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	stream, err := client.Stream(context.Background(), domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Collect chunks and expect an error
	var hasError bool
	for chunk := range stream {
		if chunk.Error != nil {
			hasError = true
		}
	}

	if !hasError {
		t.Error("Expected error chunk for invalid JSON in stream")
	}
}

// TestStream_StreamEOF tests Stream handling when stream ends with EOF (no done=true).
func TestStream_StreamEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)
		// Send chunks without a done=true final chunk
		chunks := []string{
			`{"model":"llama2","message":{"role":"assistant","content":"Hello"},"done":false}`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
			flusher.Flush()
		}
		// Close without sending done=true, which will cause EOF
	}))
	defer server.Close()

	cfg := &config.OllamaProviderConfig{
		BaseURL:      server.URL,
		DefaultModel: "llama2",
	}
	client := ollama.New(cfg)

	req := &domain.LLMRequest{
		Model:    "llama2",
		Messages: []domain.Message{{Role: "user", Content: "hello"}},
	}

	stream, err := client.Stream(context.Background(), domain.LLMProviderOllama, req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var doneReceived bool
	var content string
	for chunk := range stream {
		if chunk.Delta != "" {
			content += chunk.Delta
		}
		if chunk.Done {
			doneReceived = true
		}
	}

	// Should have received some content and eventually a Done signal
	if content == "" {
		t.Error("Expected some content")
	}
	if !doneReceived {
		t.Error("Expected Done signal after EOF")
	}
}
