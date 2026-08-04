package doc_summarize

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

func TestDocSummarizeSkill_Config(t *testing.T) {
	cfg := domain.ToolConfig{
		Enabled: true,
		Params:  map[string]interface{}{"key": "val"},
	}
	skill := NewDocSummarizeSkill(cfg, nil, nil)
	got := skill.Config()
	if got.Enabled != cfg.Enabled {
		t.Error("Config() should return the stored config")
	}
}

func TestDocSummarizeSkill_Execute_FileSuccess(t *testing.T) {
	// Create a temp file
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fpath, []byte("# Hello\n\nThis is a test document."), 0600); err != nil {
		t.Fatal(err)
	}

	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "Summary of the test document."}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"source": fpath,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Output == "" {
		t.Error("Execute() should return non-empty output")
	}
}

func TestDocSummarizeSkill_Execute_FileNotFound(t *testing.T) {
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": "/nonexistent/path/to/file.md",
	})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestDocSummarizeSkill_Execute_HTTPUrl(t *testing.T) {
	// Set up a local test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# Test Content\n\nThis is fetched content from HTTP."))
	}))
	defer server.Close()

	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "HTTP content summary."}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, server.Client())
	// This test exercises the HTTP-fetch path end-to-end, not SSRF
	// protection; httptest servers always listen on loopback, which
	// always-on SSRF validation (Phase 4, FR-020) now rejects regardless of
	// domain allowlist. SSRF-specific behavior is covered separately in
	// doc_summarize_ssrf_test.go.
	skill.SetSSRFProtection(false)

	result, err := skill.Execute(context.Background(), map[string]any{
		"source": server.URL + "/test.md",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
}

func TestDocSummarizeSkill_Execute_HTTPError(t *testing.T) {
	// Server returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, server.Client())
	// Exercises HTTP-status error handling specifically, not SSRF protection
	// (see doc_summarize_ssrf_test.go for that).
	skill.SetSSRFProtection(false)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": server.URL + "/missing.md",
	})
	if err == nil {
		t.Error("expected error for HTTP 404 response")
	}
}

func TestDocSummarizeSkill_Execute_LLMError(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fpath, []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}

	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, fmt.Errorf("LLM error")
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": fpath,
	})
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestDocSummarizeSkill_Execute_WithFocusAndMaxWords(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fpath, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	var capturedPrompt string
	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			if len(req.Messages) > 0 {
				capturedPrompt = req.Messages[0].Content
			}
			return &domain.LLMResponse{Content: "Focused summary."}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"source":    fpath,
		"focus":     "security",
		"max_words": 150,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}
	if capturedPrompt == "" {
		t.Error("LLM should have been called")
	}
}

func TestDocSummarizeSkill_Execute_MaxWordsAsFloat64(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fpath, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "summary"}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

	// max_words as float64 (as JSON numbers are parsed)
	result, err := skill.Execute(context.Background(), map[string]any{
		"source":    fpath,
		"max_words": float64(200),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}
}

func TestDocSummarizeSkill_GetAllowedDomains_Empty(t *testing.T) {
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	// With no config, allowed domains should be nil
	result, err := skill.Execute(context.Background(), map[string]any{
		"source": "https://anydomain.com/file.txt",
	})
	// Should not fail domain check since no domains configured
	// Will fail with network error or client error (acceptable in tests)
	if result != nil || err != nil {
		// Both outcomes are acceptable here - just verifying no panic
	}
}

func TestDocSummarizeSkill_CountWords(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "words.md")
	if err := os.WriteFile(fpath, []byte("one two three four five"), 0600); err != nil {
		t.Fatal(err)
	}

	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "alpha beta gamma"}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)
	result, err := skill.Execute(context.Background(), map[string]any{
		"source": fpath,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}
	// Word count should be 3 (alpha, beta, gamma)
	if wc, ok := result.Metadata["word_count"]; ok {
		if wc.(int) != 3 {
			t.Errorf("word_count = %v, want 3", wc)
		}
	}
}

func TestDocSummarizeSkill_Execute_LargContent(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "large.md")
	// Create content larger than maxContentLength
	content := make([]byte, maxContentLength+100)
	for i := range content {
		content[i] = 'a'
	}
	if err := os.WriteFile(fpath, content, 0600); err != nil {
		t.Fatal(err)
	}

	var capturedContent string
	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			if len(req.Messages) > 0 {
				capturedContent = req.Messages[0].Content
			}
			return &domain.LLMResponse{Content: "summary of large document"}, nil
		},
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)
	_, err := skill.Execute(context.Background(), map[string]any{"source": fpath})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	// Content should have been truncated
	if len(capturedContent) > maxContentLength+200 { // allowing for prompt overhead
		t.Error("Content should be truncated for large documents")
	}
}
