package doc_summarize

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocSummarizeSkill_Config covers the Config() 0.0% function
func TestDocSummarizeSkill_Config(t *testing.T) {
	t.Parallel()
	config := domain.ToolConfig{Enabled: true}
	skill := NewDocSummarizeSkill(config, nil, nil)
	got := skill.Config()
	assert.Equal(t, config.Enabled, got.Enabled)
}

// TestCountWords covers the countWords function
func TestCountWords(t *testing.T) {
	t.Parallel()
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	tests := []struct {
		text  string
		count int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"one two three four five", 5},
		{"  spaces  between  words  ", 3},
	}

	for _, tt := range tests {
		got := skill.countWords(tt.text)
		assert.Equal(t, tt.count, got, "countWords(%q)", tt.text)
	}
}

// TestBuildSummaryPrompt covers the buildSummaryPrompt function
func TestBuildSummaryPrompt(t *testing.T) {
	t.Parallel()
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	t.Run("without_focus", func(t *testing.T) {
		t.Parallel()
		prompt := skill.buildSummaryPrompt("test content", 200, "")
		assert.Contains(t, prompt, "200")
		assert.Contains(t, prompt, "test content")
	})

	t.Run("with_focus", func(t *testing.T) {
		t.Parallel()
		prompt := skill.buildSummaryPrompt("test content", 200, "security aspects")
		assert.Contains(t, prompt, "security aspects")
		assert.Contains(t, prompt, "test content")
	})
}

// TestFormatOutput covers the formatOutput function
func TestFormatOutput(t *testing.T) {
	t.Parallel()
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	output := skill.formatOutput("Test summary with multiple words", "test-source", map[string]any{})
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Test summary")
	assert.Contains(t, output, "test-source")
}

// TestGenerateSummary covers the generateSummary function
func TestGenerateSummary(t *testing.T) {
	t.Parallel()

	t.Run("success_with_defaults", func(t *testing.T) {
		t.Parallel()
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "Generated summary"}, nil
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		summary, err := skill.generateSummary(context.Background(), "test content", map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "Generated summary", summary)
	})

	t.Run("with_max_words_int", func(t *testing.T) {
		t.Parallel()
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "Summary"}, nil
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		_, err := skill.generateSummary(context.Background(), "content", map[string]any{"max_words": 100})
		require.NoError(t, err)
	})

	t.Run("with_max_words_float64", func(t *testing.T) {
		t.Parallel()
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "Summary"}, nil
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		_, err := skill.generateSummary(context.Background(), "content", map[string]any{"max_words": float64(150)})
		require.NoError(t, err)
	})

	t.Run("with_focus", func(t *testing.T) {
		t.Parallel()
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "Focused summary"}, nil
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		_, err := skill.generateSummary(context.Background(), "content", map[string]any{"focus": "API changes"})
		require.NoError(t, err)
	})

	t.Run("llm_error", func(t *testing.T) {
		t.Parallel()
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return nil, errors.New("LLM service unavailable")
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		_, err := skill.generateSummary(context.Background(), "content", map[string]any{})
		require.Error(t, err)
	})

	t.Run("content_truncated_when_too_long", func(t *testing.T) {
		t.Parallel()
		var receivedContent string
		mockLLM := &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				for _, msg := range req.Messages {
					receivedContent = msg.Content
				}
				return &domain.LLMResponse{Content: "Summary"}, nil
			},
		}
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, mockLLM, nil)

		// Create content larger than maxContentLength
		longContent := make([]byte, maxContentLength+100)
		for i := range longContent {
			longContent[i] = 'a'
		}

		_, err := skill.generateSummary(context.Background(), string(longContent), map[string]any{})
		require.NoError(t, err)
		assert.Less(t, len(receivedContent), maxContentLength+200, "Content should be truncated")
	})
}

// TestReadFile covers the readFile function
func TestReadFile(t *testing.T) {
	t.Parallel()
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	t.Run("file_not_found", func(t *testing.T) {
		t.Parallel()
		_, err := skill.readFile("/nonexistent/path/to/file.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("reads_existing_file", func(t *testing.T) {
		t.Parallel()
		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.md")
		content := "# Test Document\n\nThis is test content."
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		result, err := skill.readFile(tmpFile)
		require.NoError(t, err)
		assert.Equal(t, content, result)
	})
}

// TestFetchURL covers the fetchURL function
func TestFetchURL(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("document content"))
		}))
		defer server.Close()

		skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, server.Client())
		content, err := skill.fetchURL(context.Background(), server.URL+"/test.md")
		require.NoError(t, err)
		assert.Equal(t, "document content", content)
	})

	t.Run("non_200_status", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, server.Client())
		_, err := skill.fetchURL(context.Background(), server.URL+"/notfound.md")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})
}

// TestGetAllowedDomains covers the getAllowedDomains function
func TestGetAllowedDomains(t *testing.T) {
	t.Parallel()

	t.Run("no_allowed_domains_configured", func(t *testing.T) {
		t.Parallel()
		skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)
		domains := skill.getAllowedDomains()
		assert.Empty(t, domains)
	})

	t.Run("with_allowed_domains", func(t *testing.T) {
		t.Parallel()
		config := domain.ToolConfig{
			Params: map[string]interface{}{
				"allowed_domains": []interface{}{"github.com", "docs.example.com"},
			},
		}
		skill := NewDocSummarizeSkill(config, nil, nil)
		domains := skill.getAllowedDomains()
		assert.Len(t, domains, 2)
		assert.Contains(t, domains, "github.com")
	})
}
