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
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool/common"
)

const (
	defaultMaxWords   = 300
	defaultTimeout    = 60 * time.Second
	defaultMaxDocSize = 5 * 1024 * 1024 // 5MB
	maxContentLength  = 50000           // Max chars to send to LLM
)

// DocSummarizeSkill provides document summarization capabilities
type DocSummarizeSkill struct {
	config          domain.ToolConfig
	llmService      domain.LLMService
	httpClient      *http.Client
	outputValidator security.OutputValidator
	// ssrfProtectionDisabled inverts the SetSSRFProtection(enabled) API so the
	// zero value (a bare struct literal) keeps SSRF protection ON by default —
	// fail-closed, matching the rest of this security spec's defaults.
	ssrfProtectionDisabled bool
	// urlValidationOpts configures common.ResolveValidatedIP's host resolution
	// (see validateURL). The zero value uses net.DefaultResolver; tests inject
	// a fake IPResolver to exercise DNS-rebinding scenarios deterministically.
	urlValidationOpts common.URLValidationOptions
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
		httpClient = defaultSSRFSafeHTTPClient(defaultTimeout)
	}

	return &DocSummarizeSkill{
		config:          config,
		llmService:      llmService,
		httpClient:      httpClient,
		outputValidator: security.NewDefaultOutputValidator(),
	}
}

// defaultSSRFSafeHTTPClient builds the http.Client used when no client is
// injected by the caller. Every hop of a fetch — the initial request
// (validateURL resolves and validates the target via
// common.ResolveValidatedIP and pins the result into the request's context
// with common.WithResolvedIP *before* fetchURL ever constructs the
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
// of the fetch target (see common.ValidateFetchURL), layered under the
// (optional) domain allowlist. Defaults to enabled; wired from
// security.fetch.ssrf_protection by cmd/nuimanbot's DI so an operator can opt
// out without a code change.
func (s *DocSummarizeSkill) SetSSRFProtection(enabled bool) {
	s.ssrfProtectionDisabled = !enabled
}

// SetOutputValidator overrides the OutputValidator used to scan fetched content
// for prompt-injection patterns before it enters the summarization sub-prompt.
// Optional: if never called, NewDocSummarizeSkill's default (fail-closed reject) is used.
func (s *DocSummarizeSkill) SetOutputValidator(v security.OutputValidator) {
	s.outputValidator = v
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
	source, ctx, err := s.validateSource(ctx, params)
	if err != nil {
		return nil, err
	}

	content, err := s.fetchContent(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	// Validate fetched content for prompt-injection patterns BEFORE it enters
	// the summarization sub-prompt. Third-party file/URL content is untrusted;
	// a flagged result fails the tool call closed by default.
	content, flagged, matchedPatterns, err := s.validateFetchedContent(ctx, source, content)
	if err != nil {
		return nil, err
	}

	summary, err := s.generateSummary(ctx, content, params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	output := s.formatOutput(summary, source, params)

	metadata := map[string]any{
		"source":     source,
		"word_count": s.countWords(summary),
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
// outputValidator disables scanning and passes content through unchanged.
func (s *DocSummarizeSkill) validateFetchedContent(ctx context.Context, source, content string) (validated string, flagged bool, matchedPatterns []string, err error) {
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

// validateSource validates and extracts the source parameter. For a URL
// source, the returned context carries the pinned, validated IP (see
// validateURL) that callers MUST use for the subsequent fetch instead of the
// context passed in. A file-path source returns the input context unchanged.
func (s *DocSummarizeSkill) validateSource(ctx context.Context, params map[string]any) (string, context.Context, error) {
	source, ok := params["source"].(string)
	if !ok || source == "" {
		return "", ctx, fmt.Errorf("source is required")
	}

	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return s.validateURL(ctx, source)
	}

	// It's a file path
	return source, ctx, nil
}

// validateURL validates the URL, checks the (optional) domain allowlist, and
// then always applies IP-resolution-based SSRF validation (see
// common.ResolveValidatedIP), layered UNDER the domain allowlist: even when
// no allowlist is configured — previously "unrestricted" — or when the
// target domain IS allowlisted, loopback/private/link-local/multicast
// targets are rejected. doc_summarize previously had no SSRF check at all
// beyond this optional allowlist.
//
// Unlike a bare validity check, this uses common.ResolveValidatedIP (not the
// discard-the-IP common.ValidateFetchURL) and pins the validated IP into the
// returned context via common.WithResolvedIP. Callers MUST use the returned
// context — not the one passed in — for the fetch that follows: fetchURL
// constructs its http.Request with that context, so pinnedDialContext (see
// common/ssrf_transport.go) dials the exact address that was validated
// instead of letting the real dialer perform an independent second DNS
// lookup at connect time, which is what closes the DNS-rebinding TOCTOU
// window on the initial (non-redirect) request — the same mechanism
// NewCheckRedirect already applies per redirect hop.
func (s *DocSummarizeSkill) validateURL(ctx context.Context, urlStr string) (string, context.Context, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", ctx, fmt.Errorf("invalid URL: %w", err)
	}

	// Check domain allowlist if configured. Matching is exact-or-subdomain
	// only (host == domain, or host is a subdomain of domain) — NOT a plain
	// substring match. A substring match (strings.Contains) would let a host
	// like "notgithub.com.attacker.net" or "github.com.evil.example" sail
	// through an allowlist entry of "github.com" (FR-R08/FR-008, P1).
	if allowedDomains := s.getAllowedDomains(); len(allowedDomains) > 0 {
		host := parsedURL.Host
		allowed := false
		for _, domain := range allowedDomains {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", ctx, fmt.Errorf("domain %s not in allowed list", parsedURL.Host)
		}
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
	defer func() { _ = resp.Body.Close() }()

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
