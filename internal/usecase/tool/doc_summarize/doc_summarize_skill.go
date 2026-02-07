package doc_summarize

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

const (
	defaultMaxWords   = 300
	defaultTimeout    = 60 * time.Second
	defaultMaxDocSize = 5 * 1024 * 1024 // 5MB
	maxContentLength  = 50000           // Max chars to send to LLM
)

// DocSummarizeSkill provides document summarization capabilities
type DocSummarizeSkill struct {
	config     domain.ToolConfig
	llmService domain.LLMService
	httpClient *http.Client
}

// SummaryOutput represents the structured summary output
type SummaryOutput struct {
	Summary   string   `json:"summary"`
	Source    string   `json:"source"`
	WordCount int      `json:"word_count"`
	KeyTopics []string `json:"key_topics,omitempty"`
	Timestamp string   `json:"timestamp"`
}

// NewDocSummarizeSkill creates a new DocSummarizeSkill instance
func NewDocSummarizeSkill(
	config domain.ToolConfig,
	llmService domain.LLMService,
	httpClient *http.Client,
) *DocSummarizeSkill {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}

	return &DocSummarizeSkill{
		config:     config,
		llmService: llmService,
		httpClient: httpClient,
	}
}

// Name returns the skill identifier
func (s *DocSummarizeSkill) Name() string {
	return "doc_summarize"
}

// Description returns a human-readable description
func (s *DocSummarizeSkill) Description() string {
	return "Summarize documentation files and links using LLM. Supports local files, Git URLs, and HTTP/HTTPS URLs."
}

// RequiredPermissions returns the permissions needed
func (s *DocSummarizeSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{
		domain.PermissionRead,
		domain.PermissionNetwork,
	}
}

// Config returns the skill configuration
func (s *DocSummarizeSkill) Config() domain.ToolConfig {
	return s.config
}

// InputSchema returns the JSON schema for parameters
func (s *DocSummarizeSkill) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source": map[string]any{
				"type":        "string",
				"description": "File path, Git URL, or HTTP/HTTPS URL",
			},
			"max_words": map[string]any{
				"type":        "integer",
				"default":     defaultMaxWords,
				"description": "Target summary length in words",
			},
			"focus": map[string]any{
				"type":        "string",
				"description": "Optional focus area (e.g., 'API changes', 'security')",
			},
		},
		"required": []string{"source"},
	}
}

// Execute runs the document summarization
func (s *DocSummarizeSkill) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	source, err := s.validateSource(params)
	if err != nil {
		return nil, err
	}

	content, err := s.fetchContent(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	summary, err := s.generateSummary(ctx, content, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	output := s.formatOutput(summary, source, params)

	return &domain.ExecutionResult{
		Output: output,
		Metadata: map[string]any{
			"source":     source,
			"word_count": s.countWords(summary),
		},
	}, nil
}

// validateSource validates and extracts the source parameter
func (s *DocSummarizeSkill) validateSource(params map[string]any) (string, error) {
	source, ok := params["source"].(string)
	if !ok || source == "" {
		return "", fmt.Errorf("source is required")
	}

	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return s.validateURL(source)
	}

	// It's a file path
	return source, nil
}

// validateURL validates the URL and checks domain allowlist
func (s *DocSummarizeSkill) validateURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Check domain allowlist if configured
	if allowedDomains := s.getAllowedDomains(); len(allowedDomains) > 0 {
		allowed := false
		for _, domain := range allowedDomains {
			if strings.Contains(parsedURL.Host, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("domain %s not in allowed list", parsedURL.Host)
		}
	}

	return urlStr, nil
}

// getAllowedDomains gets the allowed domains from config
func (s *DocSummarizeSkill) getAllowedDomains() []string {
	if domains, ok := s.config.Params["allowed_domains"].([]interface{}); ok {
		result := make([]string, 0, len(domains))
		for _, d := range domains {
			if str, ok := d.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// fetchContent fetches content from source (file or URL)
func (s *DocSummarizeSkill) fetchContent(ctx context.Context, source string) (string, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return s.fetchURL(ctx, source)
	}
	return s.readFile(source)
}

// fetchURL fetches content from HTTP/HTTPS URL
func (s *DocSummarizeSkill) fetchURL(ctx context.Context, urlStr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read with size limit
	limitedReader := io.LimitReader(resp.Body, int64(defaultMaxDocSize))
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// readFile reads content from local file
func (s *DocSummarizeSkill) readFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("file not found or inaccessible: %w", err)
	}

	if info.Size() > int64(defaultMaxDocSize) {
		return "", fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), defaultMaxDocSize)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// generateSummary generates a summary using the LLM service
func (s *DocSummarizeSkill) generateSummary(ctx context.Context, content string, params map[string]any) (string, error) {
	// Truncate content if too long
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + "..."
	}

	maxWords := defaultMaxWords
	if mw, ok := params["max_words"].(int); ok {
		maxWords = mw
	} else if mw, ok := params["max_words"].(float64); ok {
		maxWords = int(mw)
	}

	focus := ""
	if f, ok := params["focus"].(string); ok {
		focus = f
	}

	prompt := s.buildSummaryPrompt(content, maxWords, focus)

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

	// Use default provider (Anthropic) - can be configured later
	resp, err := s.llmService.Complete(ctx, domain.LLMProviderAnthropic, llmReq)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// buildSummaryPrompt builds the prompt for the LLM
func (s *DocSummarizeSkill) buildSummaryPrompt(content string, maxWords int, focus string) string {
	prompt := fmt.Sprintf("Please summarize the following document in approximately %d words", maxWords)

	if focus != "" {
		prompt += fmt.Sprintf(", focusing on: %s", focus)
	}

	prompt += ".\n\nDocument:\n" + content

	return prompt
}

// formatOutput formats the summary output as JSON
func (s *DocSummarizeSkill) formatOutput(summary, source string, params map[string]any) string {
	output := SummaryOutput{
		Summary:   summary,
		Source:    source,
		WordCount: s.countWords(summary),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	jsonOutput, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to format output: %s"}`, err.Error())
	}

	return string(jsonOutput)
}

// countWords counts words in text
func (s *DocSummarizeSkill) countWords(text string) int {
	return len(strings.Fields(text))
}
