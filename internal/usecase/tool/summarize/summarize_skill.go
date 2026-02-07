package summarize

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/skill/executor"
)

const (
	defaultTimeout   = 90 * time.Second
	maxWebPageSize   = 10 * 1024 * 1024 // 10MB
	maxContentLength = 50000            // Max chars to send to LLM
)

// SummarizeSkill provides URL and YouTube video summarization
type SummarizeSkill struct {
	config     domain.SkillConfig
	llmService domain.LLMService
	executor   executor.ExecutorService
	httpClient *http.Client
}

// SummaryOutput represents the structured summary output
type SummaryOutput struct {
	Summary       string   `json:"summary"`
	Title         string   `json:"title,omitempty"`
	Author        string   `json:"author,omitempty"`
	PublishedDate string   `json:"published_date,omitempty"`
	SourceType    string   `json:"source_type"`
	ReadingTime   string   `json:"reading_time,omitempty"`
	KeyQuotes     []string `json:"key_quotes,omitempty"`
	URL           string   `json:"url"`
}

// NewSummarizeSkill creates a new SummarizeSkill instance
func NewSummarizeSkill(
	config domain.SkillConfig,
	llmService domain.LLMService,
	executor executor.ExecutorService,
	httpClient *http.Client,
) *SummarizeSkill {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}

	return &SummarizeSkill{
		config:     config,
		llmService: llmService,
		executor:   executor,
		httpClient: httpClient,
	}
}

// Name returns the skill identifier
func (s *SummarizeSkill) Name() string {
	return "summarize"
}

// Description returns a human-readable description
func (s *SummarizeSkill) Description() string {
	return "Summarize external URLs, web pages, and YouTube videos using LLM"
}

// RequiredPermissions returns the permissions needed
func (s *SummarizeSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the skill configuration
func (s *SummarizeSkill) Config() domain.SkillConfig {
	return s.config
}

// InputSchema returns the JSON schema for parameters
func (s *SummarizeSkill) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to summarize (HTTP, HTTPS, or YouTube)",
			},
			"format": map[string]any{
				"type":        "string",
				"enum":        []string{"brief", "detailed", "bullet_points"},
				"default":     "brief",
				"description": "Output format",
			},
			"include_quotes": map[string]any{
				"type":        "boolean",
				"default":     false,
				"description": "Include key quotes from source",
			},
		},
		"required": []string{"url"},
	}
}

// Execute runs the URL summarization
func (s *SummarizeSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	urlStr, err := s.validateURL(params)
	if err != nil {
		return nil, err
	}

	content, sourceType, err := s.fetchContent(ctx, urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	summary, err := s.generateSummary(ctx, content, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	output := s.formatOutput(summary, urlStr, sourceType, params)

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"url":         urlStr,
			"source_type": sourceType,
		},
	}, nil
}

// validateURL validates and extracts the URL parameter
func (s *SummarizeSkill) validateURL(params map[string]any) (string, error) {
	urlStr, ok := params["url"].(string)
	if !ok || urlStr == "" {
		return "", fmt.Errorf("url is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Security checks
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("only HTTP and HTTPS URLs are supported")
	}

	// Reject localhost and private IPs
	if strings.Contains(parsedURL.Host, "localhost") ||
		strings.Contains(parsedURL.Host, "127.0.0.1") ||
		strings.Contains(parsedURL.Host, "0.0.0.0") {
		return "", fmt.Errorf("localhost and private IPs are not allowed")
	}

	return urlStr, nil
}

// fetchContent fetches content from URL (web page or YouTube)
func (s *SummarizeSkill) fetchContent(ctx context.Context, urlStr string) (string, string, error) {
	// Check if it's a YouTube URL
	if s.isYouTubeURL(urlStr) {
		content, err := s.fetchYouTubeTranscript(ctx, urlStr)
		return content, "youtube", err
	}

	// Regular web page
	content, err := s.fetchWebPage(ctx, urlStr)
	return content, "webpage", err
}

// isYouTubeURL checks if the URL is a YouTube video
func (s *SummarizeSkill) isYouTubeURL(urlStr string) bool {
	return strings.Contains(urlStr, "youtube.com/watch") ||
		strings.Contains(urlStr, "youtu.be/")
}

// fetchYouTubeTranscript fetches YouTube transcript using yt-dlp
func (s *SummarizeSkill) fetchYouTubeTranscript(ctx context.Context, urlStr string) (string, error) {
	if s.executor == nil {
		return "", fmt.Errorf("executor not available for YouTube transcript extraction")
	}

	execReq := executor.ExecutionRequest{
		Command: "yt-dlp",
		Args: []string{
			"--skip-download",
			"--write-auto-sub",
			"--sub-lang", "en",
			"--sub-format", "txt",
			"--print", "%(subtitles)s",
			urlStr,
		},
		Timeout: defaultTimeout,
	}

	execResult, err := s.executor.Execute(ctx, execReq)
	if err != nil {
		return "", fmt.Errorf("yt-dlp execution failed: %w", err)
	}

	if execResult.ExitCode != 0 {
		return "", fmt.Errorf("yt-dlp failed: %s", execResult.Stderr)
	}

	return execResult.Stdout, nil
}

// fetchWebPage fetches web page content via HTTP
func (s *SummarizeSkill) fetchWebPage(ctx context.Context, urlStr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	// Set user agent
	userAgent := "NuimanBot/1.0"
	if ua, ok := s.config.Params["user_agent"].(string); ok {
		userAgent = ua
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read with size limit
	limitedReader := io.LimitReader(resp.Body, int64(maxWebPageSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// generateSummary generates a summary using the LLM service
func (s *SummarizeSkill) generateSummary(ctx context.Context, content string, params map[string]any) (string, error) {
	// Truncate content if too long
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + "..."
	}

	format := "brief"
	if f, ok := params["format"].(string); ok {
		format = f
	}

	includeQuotes := false
	if iq, ok := params["include_quotes"].(bool); ok {
		includeQuotes = iq
	}

	prompt := s.buildSummaryPrompt(content, format, includeQuotes)

	llmReq := &domain.LLMRequest{
		Messages: []domain.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   2000,
		Temperature: 0.3,
	}

	resp, err := s.llmService.Complete(ctx, domain.LLMProviderAnthropic, llmReq)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// buildSummaryPrompt builds the prompt for the LLM
func (s *SummarizeSkill) buildSummaryPrompt(content, format string, includeQuotes bool) string {
	var prompt strings.Builder

	prompt.WriteString("Please summarize the following content")

	switch format {
	case "detailed":
		prompt.WriteString(" in detail (2-3 paragraphs)")
	case "bullet_points":
		prompt.WriteString(" as bullet points (5-10 key points)")
	default:
		prompt.WriteString(" briefly (1-2 paragraphs)")
	}

	if includeQuotes {
		prompt.WriteString(". Include 2-3 key quotes from the source")
	}

	prompt.WriteString(".\n\nContent:\n")
	prompt.WriteString(content)

	return prompt.String()
}

// formatOutput formats the summary output as JSON
func (s *SummarizeSkill) formatOutput(summary, urlStr, sourceType string, params map[string]any) string {
	output := SummaryOutput{
		Summary:    summary,
		URL:        urlStr,
		SourceType: sourceType,
	}

	jsonOutput, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to format output: %s"}`, err.Error())
	}

	return string(jsonOutput)
}
