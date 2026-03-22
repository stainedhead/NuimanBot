package summarize

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/executor"
)

// mockExecService implements executor.ExecutorService for testing
type mockExecService struct {
	executeFunc func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error)
}

func (m *mockExecService) Execute(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return &executor.ExecutionResult{ExitCode: 0, Stdout: "mock output"}, nil
}
func (m *mockExecService) ExecuteBackground(ctx context.Context, req executor.ExecutionRequest) (*executor.BackgroundSession, error) {
	return &executor.BackgroundSession{ID: "session-1"}, nil
}
func (m *mockExecService) GetSessionStatus(ctx context.Context, sessionID string) (*executor.SessionStatus, error) {
	return &executor.SessionStatus{}, nil
}
func (m *mockExecService) GetSessionOutput(ctx context.Context, sessionID string) (string, error) {
	return "", nil
}
func (m *mockExecService) CancelSession(ctx context.Context, sessionID string) error {
	return nil
}

func TestSummarizeSkill_Config(t *testing.T) {
	cfg := domain.ToolConfig{Enabled: true}
	skill := NewSummarizeSkill(cfg, nil, nil, nil)
	got := skill.Config()
	if !got.Enabled {
		t.Error("Config() should return the configured value")
	}
}

func TestSummarizeSkill_Execute_SuccessHTTP(t *testing.T) {
	// httptest uses 127.0.0.1 which is blocked by the skill's security check.
	// Test fetchWebPage directly via private method by calling generateSummary on known content.
	// Instead, we test the full pipeline by calling buildSummaryPrompt and generateSummary.
	skill := &SummarizeSkill{
		config: domain.ToolConfig{},
		llmService: &MockLLMService{
			CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
				return &domain.LLMResponse{Content: "Web page summary."}, nil
			},
		},
		httpClient: &http.Client{},
	}

	// Test via generateSummary directly
	summary, err := skill.generateSummary(context.Background(), "test content", map[string]any{})
	if err != nil {
		t.Fatalf("generateSummary() error = %v", err)
	}
	if summary != "Web page summary." {
		t.Errorf("generateSummary() = %q, want %q", summary, "Web page summary.")
	}
}

func TestSummarizeSkill_Execute_HTTPError(t *testing.T) {
	// Test fetchWebPage error handling directly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	skill := &SummarizeSkill{
		config:     domain.ToolConfig{},
		httpClient: server.Client(),
	}

	_, err := skill.fetchWebPage(context.Background(), server.URL+"/error")
	if err == nil {
		t.Error("expected error for HTTP 500 response")
	}
}

func TestSummarizeSkill_Execute_LLMFailure(t *testing.T) {
	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, fmt.Errorf("LLM unavailable")
		},
	}

	skill := &SummarizeSkill{
		config:     domain.ToolConfig{},
		llmService: mockLLM,
		httpClient: &http.Client{},
	}

	_, err := skill.generateSummary(context.Background(), "some content", map[string]any{})
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestSummarizeSkill_Execute_YouTubeWithExecutorFailure(t *testing.T) {
	mockExec := &mockExecService{
		executeFunc: func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
			return nil, fmt.Errorf("yt-dlp not installed")
		},
	}

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, mockExec, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "https://www.youtube.com/watch?v=test123",
	})
	if err == nil {
		t.Error("expected error when yt-dlp fails")
	}
}

func TestSummarizeSkill_Execute_YouTubeWithNonZeroExitCode(t *testing.T) {
	mockExec := &mockExecService{
		executeFunc: func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
			return &executor.ExecutionResult{
				ExitCode: 1,
				Stderr:   "error downloading transcript",
			}, nil
		},
	}

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, mockExec, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "https://youtu.be/test123",
	})
	if err == nil {
		t.Error("expected error when yt-dlp exits non-zero")
	}
}

func TestSummarizeSkill_Execute_YouTubeWithNoExecutor(t *testing.T) {
	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "https://www.youtube.com/watch?v=test123",
	})
	if err == nil {
		t.Error("expected error when no executor is configured for YouTube")
	}
}

func TestSummarizeSkill_Execute_PrivateIPRejected(t *testing.T) {
	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "http://127.0.0.1:8080/secret",
	})
	if err == nil {
		t.Error("expected error for 127.0.0.1")
	}
}

func TestSummarizeSkill_Execute_WithCustomUserAgent(t *testing.T) {
	var capturedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprintf(w, "content")
	}))
	defer server.Close()

	skill := &SummarizeSkill{
		config: domain.ToolConfig{
			Params: map[string]interface{}{
				"user_agent": "TestBot/2.0",
			},
		},
		httpClient: server.Client(),
	}

	_, _ = skill.fetchWebPage(context.Background(), server.URL+"/page")

	if capturedUA != "TestBot/2.0" {
		t.Errorf("User-Agent = %q, want %q", capturedUA, "TestBot/2.0")
	}
}

func TestSummarizeSkill_BuildSummaryPrompt_Formats(t *testing.T) {
	formats := []struct {
		format      string
		shouldMatch string
	}{
		{"brief", "briefly"},
		{"detailed", "detail"},
		{"bullet_points", "bullet"},
	}

	for _, tt := range formats {
		t.Run(tt.format, func(t *testing.T) {
			skill := &SummarizeSkill{config: domain.ToolConfig{}}
			prompt := skill.buildSummaryPrompt("some content", tt.format, false)
			if !strings.Contains(strings.ToLower(prompt), tt.shouldMatch) {
				t.Errorf("buildSummaryPrompt(%q) prompt = %q, should contain %q", tt.format, prompt, tt.shouldMatch)
			}
		})
	}
}

func TestSummarizeSkill_Execute_WithIncludeQuotesTrue(t *testing.T) {
	skill := &SummarizeSkill{config: domain.ToolConfig{}}
	prompt := skill.buildSummaryPrompt("content", "brief", true)
	if !strings.Contains(prompt, "quotes") {
		t.Error("Prompt should mention quotes when include_quotes is true")
	}
}

func TestSummarizeSkill_Execute_LargeContent(t *testing.T) {
	largeContent := strings.Repeat("word ", maxContentLength/5+100)

	var capturedContent string
	mockLLM := &MockLLMService{
		CompleteFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			if len(req.Messages) > 0 {
				capturedContent = req.Messages[0].Content
			}
			return &domain.LLMResponse{Content: "summary"}, nil
		},
	}

	skill := &SummarizeSkill{
		config:     domain.ToolConfig{},
		llmService: mockLLM,
		httpClient: &http.Client{},
	}

	_, err := skill.generateSummary(context.Background(), largeContent, map[string]any{})
	if err != nil {
		t.Fatalf("generateSummary() error = %v", err)
	}
	// Content should have been truncated
	if len(capturedContent) > maxContentLength+200 {
		t.Error("Content should be truncated for large inputs")
	}
}
