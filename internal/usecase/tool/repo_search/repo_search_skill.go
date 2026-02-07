package repo_search

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/skill/common"
	"nuimanbot/internal/usecase/skill/executor"
)

const (
	defaultMaxResults      = 50
	defaultSearchTimeout   = 30 * time.Second
	defaultContextLines    = 2
	ripgrepNoMatchExitCode = 1
)

// RepoSearchSkill provides fast codebase search using ripgrep
type RepoSearchSkill struct {
	config    domain.SkillConfig
	executor  executor.ExecutorService
	pathVal   *common.PathValidator
	sanitizer *common.OutputSanitizer
}

// SearchResult represents a single search match
type SearchResult struct {
	File          string   `json:"file"`
	Line          int      `json:"line"`
	Match         string   `json:"match"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

// SearchOutput represents the complete search results
type SearchOutput struct {
	Results      []SearchResult `json:"results"`
	TotalMatches int            `json:"total_matches"`
	Truncated    bool           `json:"truncated"`
}

// NewRepoSearchSkill creates a new RepoSearchSkill instance
func NewRepoSearchSkill(
	config domain.SkillConfig,
	executor executor.ExecutorService,
	pathVal *common.PathValidator,
	sanitizer *common.OutputSanitizer,
) *RepoSearchSkill {
	return &RepoSearchSkill{
		config:    config,
		executor:  executor,
		pathVal:   pathVal,
		sanitizer: sanitizer,
	}
}

// Name returns the skill identifier
func (s *RepoSearchSkill) Name() string {
	return "repo_search"
}

// Description returns a human-readable description
func (s *RepoSearchSkill) Description() string {
	return "Fast codebase search using ripgrep (rg). Search for code patterns, file names, and content across your repositories with workspace restriction enforcement."
}

// RequiredPermissions returns the permissions needed
func (s *RepoSearchSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionRead}
}

// Config returns the skill configuration
func (s *RepoSearchSkill) Config() domain.SkillConfig {
	return s.config
}

// InputSchema returns the JSON schema for parameters
func (s *RepoSearchSkill) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (regex or literal)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search (default: current workspace)",
			},
			"file_type": map[string]any{
				"type":        "string",
				"description": "Filter by file extension (e.g., 'go', 'js')",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"default":     defaultMaxResults,
				"description": "Maximum number of results to return",
			},
			"context_lines": map[string]any{
				"type":        "integer",
				"default":     defaultContextLines,
				"description": "Number of context lines before/after match",
			},
		},
		"required": []string{"query"},
	}
}

// Execute runs the codebase search
func (s *RepoSearchSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	query, searchPath, err := s.validateAndExtractParams(params)
	if err != nil {
		return nil, err
	}

	execResult, err := s.executeRipgrep(ctx, query, searchPath, params)
	if err != nil {
		return nil, err
	}

	output := s.formatOutput(execResult.Stdout, params)

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"query":       query,
			"search_path": searchPath,
			"exit_code":   execResult.ExitCode,
		},
	}, nil
}

// validateAndExtractParams validates and extracts required parameters
func (s *RepoSearchSkill) validateAndExtractParams(params map[string]any) (query string, searchPath string, err error) {
	// Extract and validate query
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", "", fmt.Errorf("query is required")
	}

	// Get search path (default to current directory)
	searchPath = "."
	if path, ok := params["path"].(string); ok && path != "" {
		searchPath = path
	}

	// Validate path if pathValidator is configured
	if s.pathVal != nil {
		if err := s.pathVal.ValidatePath(searchPath); err != nil {
			return "", "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	return query, searchPath, nil
}

// executeRipgrep runs ripgrep and returns the execution result
func (s *RepoSearchSkill) executeRipgrep(ctx context.Context, query, searchPath string, params map[string]any) (*executor.ExecutionResult, error) {
	args := s.buildRipgrepArgs(query, params)

	execReq := executor.ExecutionRequest{
		Command:    "rg",
		Args:       args,
		WorkingDir: searchPath,
		Timeout:    defaultSearchTimeout,
	}

	execResult, err := s.executor.Execute(ctx, execReq)
	if err != nil {
		return nil, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	// ripgrep returns exit code 1 when no matches found (not an error)
	if execResult.ExitCode != 0 && execResult.ExitCode != ripgrepNoMatchExitCode {
		return nil, fmt.Errorf("ripgrep failed with exit code %d: %s", execResult.ExitCode, execResult.Stderr)
	}

	return execResult, nil
}

// formatOutput parses ripgrep output and applies sanitization
func (s *RepoSearchSkill) formatOutput(rawOutput string, params map[string]any) string {
	results := s.parseRipgrepOutput(rawOutput, params)

	if s.sanitizer != nil {
		return s.sanitizer.SanitizeOutput(results)
	}

	return results
}

// getIntParam extracts an integer parameter from params (handles both int and float64)
func getIntParam(params map[string]any, key string) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case int64:
			return int(v)
		}
	}
	return 0
}

// buildRipgrepArgs constructs the ripgrep command arguments
func (s *RepoSearchSkill) buildRipgrepArgs(query string, params map[string]any) []string {
	args := []string{
		"--line-number", // Show line numbers
		"--no-heading",  // Don't group by file
		"--color=never", // No color output
	}

	// Add context lines if specified
	if contextLines := getIntParam(params, "context_lines"); contextLines > 0 {
		args = append(args, "-C", strconv.Itoa(contextLines))
	}

	// Add max count if specified
	if maxResults := getIntParam(params, "max_results"); maxResults > 0 {
		args = append(args, "--max-count", strconv.Itoa(maxResults))
	}

	// Add file type filter if specified
	if fileType, ok := params["file_type"].(string); ok && fileType != "" {
		args = append(args, "--type", fileType)
	}

	// Add the search query
	args = append(args, query)

	return args
}

// parseRipgrepOutput parses ripgrep output into structured results
func (s *RepoSearchSkill) parseRipgrepOutput(output string, params map[string]any) string {
	if output == "" {
		return s.emptyResults()
	}

	results := s.parseLines(strings.Split(strings.TrimSpace(output), "\n"))
	maxResults := s.getMaxResults(params)

	searchOutput := SearchOutput{
		Results:      results,
		TotalMatches: len(results),
		Truncated:    len(results) >= maxResults,
	}

	return s.marshalResults(searchOutput)
}

// emptyResults returns an empty JSON result
func (s *RepoSearchSkill) emptyResults() string {
	return `{"results":[],"total_matches":0,"truncated":false}`
}

// parseLines parses ripgrep output lines into SearchResult structs
func (s *RepoSearchSkill) parseLines(lines []string) []SearchResult {
	results := make([]SearchResult, 0, len(lines))

	for _, line := range lines {
		if result, ok := s.parseLine(line); ok {
			results = append(results, result)
		}
	}

	return results
}

// parseLine parses a single ripgrep output line (format: file:line:content)
func (s *RepoSearchSkill) parseLine(line string) (SearchResult, bool) {
	if line == "" {
		return SearchResult{}, false
	}

	parts := strings.SplitN(line, ":", 3)
	if len(parts) < 3 {
		return SearchResult{}, false
	}

	lineNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return SearchResult{}, false
	}

	return SearchResult{
		File:  parts[0],
		Line:  lineNum,
		Match: parts[2],
	}, true
}

// getMaxResults gets the max_results parameter with default
func (s *RepoSearchSkill) getMaxResults(params map[string]any) int {
	maxResults := getIntParam(params, "max_results")
	if maxResults == 0 {
		return defaultMaxResults
	}
	return maxResults
}

// marshalResults converts SearchOutput to JSON string
func (s *RepoSearchSkill) marshalResults(output SearchOutput) string {
	jsonOutput, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal results: %s"}`, err.Error())
	}
	return string(jsonOutput)
}
