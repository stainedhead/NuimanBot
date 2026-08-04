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
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool/common"
	"nuimanbot/internal/usecase/tool/executor"
)

const (
	defaultTimeout   = 90 * time.Second
	maxWebPageSize   = 10 * 1024 * 1024 // 10MB
	maxContentLength = 50000            // Max chars to send to LLM
)

// SummarizeSkill provides URL and YouTube video summarization
type SummarizeSkill struct {
	config          domain.ToolConfig
	llmService      domain.LLMService
	executor        executor.ExecutorService
	httpClient      *http.Client
	outputValidator security.OutputValidator
	// ssrfProtectionDisabled inverts the SetSSRFProtection(enabled) API so the
	// zero value (a bare struct literal, as used by several white-box tests)
	// keeps SSRF protection ON by default — fail-closed, matching the rest of
	// this security spec's defaults.
	ssrfProtectionDisabled bool
	// urlValidationOpts configures common.ResolveValidatedIP's host resolution
	// (see validateURL). The zero value uses net.DefaultResolver; tests inject
	// a fake IPResolver to exercise DNS-rebinding scenarios deterministically.
	urlValidationOpts common.URLValidationOptions
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
	config domain.ToolConfig,
	llmService domain.LLMService,
	executor executor.ExecutorService,
	httpClient *http.Client,
) *SummarizeSkill {
	if httpClient == nil {
		httpClient = defaultSSRFSafeHTTPClient(defaultTimeout)
	}

	return &SummarizeSkill{
		config:          config,
		llmService:      llmService,
		executor:        executor,
		httpClient:      httpClient,
		outputValidator: security.NewDefaultOutputValidator(),
	}
}

// defaultSSRFSafeHTTPClient builds the http.Client used when no client is
// injected by the caller. Every hop of a fetch — the initial request
// (validateURL resolves and validates the target via
// common.ResolveValidatedIP and pins the result into the request's context
// with common.WithResolvedIP *before* fetchWebPage ever constructs the
// http.Request) and every subsequent redirect hop (re-validated and
// re-pinned by common.NewCheckRedirect) — is dialed at the exact resolved IP
// that was validated, not a second, independent DNS lookup that could return
// something different. This closes the DNS-rebinding TOCTOU window across
// the *entire* fetch flow, not just redirects (see
// common.NewSSRFSafeTransport / common.NewCheckRedirect /
// common.ResolveValidatedIP).
func defaultSSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: common.NewSSRFSafeTransport(nil),
		CheckRedirect: common.NewCheckRedirect(
			common.FetchPolicy{SSRFProtection: true, FollowRedirects: true},
			common.URLValidationOptions{},
		),
	}
}

// SetSSRFProtection enables or disables IP-resolution-based SSRF validation
// of the fetch target (see common.ValidateFetchURL). Defaults to enabled;
// wired from security.fetch.ssrf_protection by cmd/nuimanbot's DI so an
// operator can opt out without a code change.
func (s *SummarizeSkill) SetSSRFProtection(enabled bool) {
	s.ssrfProtectionDisabled = !enabled
}

// SetOutputValidator overrides the OutputValidator used to scan fetched content
// for prompt-injection patterns before it enters the summarization sub-prompt.
// Optional: if never called, NewSummarizeSkill's default (fail-closed reject) is used.
func (s *SummarizeSkill) SetOutputValidator(v security.OutputValidator) {
	s.outputValidator = v
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
func (s *SummarizeSkill) Config() domain.ToolConfig {
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
func (s *SummarizeSkill) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	urlStr, ctx, err := s.validateURL(ctx, params)
	if err != nil {
		return nil, err
	}

	content, sourceType, err := s.fetchContent(ctx, urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	// Validate fetched content for prompt-injection patterns BEFORE it enters
	// the summarization sub-prompt. Third-party web/YouTube content is
	// untrusted; a flagged result fails the tool call closed by default.
	content, flagged, matchedPatterns, err := s.validateFetchedContent(ctx, urlStr, content)
	if err != nil {
		return nil, err
	}

	summary, err := s.generateSummary(ctx, content, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	output := s.formatOutput(summary, urlStr, sourceType, params)

	metadata := map[string]any{
		"url":         urlStr,
		"source_type": sourceType,
	}
	if flagged {
		metadata["injection_flagged"] = true
		metadata["matched_patterns"] = matchedPatterns
	}

	return &domain.ExecutionResult{
		Output:   output,
		Metadata: metadata,
	}, nil
}

// validateFetchedContent scans fetched content for prompt-injection patterns via
// OutputValidator. Clean content is returned unchanged. Flagged content is
// handled per the validator's configured action: annotate wraps it with a
// visible warning marker; reject (the default) returns a *security.FlaggedOutputError
// and empty content so the caller fails the tool call closed. A nil
// outputValidator (e.g. a bare struct literal in tests) disables scanning and
// passes content through unchanged.
func (s *SummarizeSkill) validateFetchedContent(ctx context.Context, source, content string) (validated string, flagged bool, matchedPatterns []string, err error) {
	if s.outputValidator == nil {
		return content, false, nil, nil
	}

	result, err := s.outputValidator.ValidateToolOutput(ctx, source, content)
	if err != nil {
		return "", false, nil, fmt.Errorf("output validation failed: %w", err)
	}
	if !result.Flagged {
		return content, false, nil, nil
	}

	if result.Action == security.ValidationActionAnnotate {
		return security.AnnotateFlaggedContent(content), true, result.MatchedPatterns, nil
	}

	// Fail closed (default reject action).
	return "", true, result.MatchedPatterns, &security.FlaggedOutputError{
		Source:          source,
		MatchedPatterns: result.MatchedPatterns,
	}
}

// validateURL validates and extracts the URL parameter. SSRF protection
// resolves the host to its IP address(es) and rejects loopback, RFC 1918
// private, link-local (including cloud metadata), and multicast/reserved
// targets — see common.ResolveValidatedIP. This replaces the previous
// substring-only check against "localhost"/"127.0.0.1"/"0.0.0.0", which never
// caught IPv6 loopback (::1), cloud-metadata addresses, or other private
// ranges.
//
// Unlike a bare validity check, this uses common.ResolveValidatedIP (not the
// discard-the-IP common.ValidateFetchURL) and pins the validated IP into the
// returned context via common.WithResolvedIP. Callers MUST use the returned
// context — not the one passed in — for the fetch that follows: fetchWebPage
// constructs its http.Request with that context, so pinnedDialContext (see
// common/ssrf_transport.go) dials the exact address that was validated
// instead of letting the real dialer perform an independent second DNS
// lookup at connect time, which is what closes the DNS-rebinding TOCTOU
// window on the initial (non-redirect) request — the same mechanism
// NewCheckRedirect already applies per redirect hop.
func (s *SummarizeSkill) validateURL(ctx context.Context, params map[string]any) (string, context.Context, error) {
	urlStr, ok := params["url"].(string)
	if !ok || urlStr == "" {
		return "", ctx, fmt.Errorf("url is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", ctx, fmt.Errorf("invalid URL: %w", err)
	}

	// Security checks
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", ctx, fmt.Errorf("only HTTP and HTTPS URLs are supported")
	}

	if !s.ssrfProtectionDisabled {
		ip, err := common.ResolveValidatedIP(ctx, urlStr, s.urlValidationOpts)
		if err != nil {
			return "", ctx, fmt.Errorf("url rejected by SSRF protection: %w", err)
		}
		ctx = common.WithResolvedIP(ctx, ip)
	}

	return urlStr, ctx, nil
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
	defer func() { _ = resp.Body.Close() }()

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
