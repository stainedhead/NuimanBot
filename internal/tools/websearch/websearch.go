package websearch

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	searchClient "nuimanbot/internal/infrastructure/search"
	"nuimanbot/internal/usecase/security"
)

// WebSearch implements the domain.Tool interface for web search.
type WebSearch struct {
	client          *searchClient.Client
	config          domain.ToolConfig
	outputValidator security.OutputValidator
}

// NewWebSearch creates a new WebSearch tool.
func NewWebSearch(timeoutSeconds int) *WebSearch {
	return &WebSearch{
		client: searchClient.NewClient(timeoutSeconds),
		config: domain.ToolConfig{
			Enabled: true,
		},
		outputValidator: security.NewDefaultOutputValidator(),
	}
}

// SetOutputValidator overrides the OutputValidator used to scan search results
// for prompt-injection patterns before they're returned as tool output.
// Optional: if never called, NewWebSearch's default (fail-closed reject) is used.
func (w *WebSearch) SetOutputValidator(v security.OutputValidator) {
	w.outputValidator = v
}

// Name returns the tool name.
func (w *WebSearch) Name() string {
	return "websearch"
}

// Description returns the tool description.
func (w *WebSearch) Description() string {
	return "Perform web searches using DuckDuckGo and return relevant results"
}

// InputSchema returns the JSON schema for the tool's input parameters.
func (w *WebSearch) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Number of results to return (1-50)",
				"default":     5,
				"minimum":     1,
				"maximum":     50,
			},
		},
		"required": []string{"query"},
	}
}

// Execute performs the web search operation.
func (w *WebSearch) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	// Extract and validate query
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return &domain.ExecutionResult{
			Error: "missing query parameter",
		}, nil
	}

	// Extract limit (optional, default to 5)
	limit := 5
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := params["limit"].(int); ok {
		limit = l
	}

	// Validate limit
	if limit < 1 || limit > 50 {
		return &domain.ExecutionResult{
			Error: "limit must be between 1 and 50",
		}, nil
	}

	// Perform search
	rawResults, err := w.client.Search(ctx, query, limit)
	if err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	// Scan each result for prompt-injection patterns before returning it as
	// tool output. Results are independent snippets, so a flagged individual
	// result is dropped (or annotated, per config) rather than failing the
	// whole search; if every result is flagged and dropped, an empty/safe
	// result set is returned rather than an error.
	//
	// FR-007/FR-R07 (specs/260803-improve-nuimanbot-security-auto-review):
	// review flagged that this is a deliberate, tool-specific reading of the
	// "reject fails the tool call" config semantics — `summarize`,
	// `doc_summarize`, and the MCP bridge each return exactly one piece of
	// content, so for them "reject" and "fail the whole call" are the same
	// thing. `websearch` returns N independent snippets from N independent
	// sources; failing the entire search because one of several results was
	// flagged would discard clean, useful results for no added security
	// benefit (the flagged snippet is dropped either way, satisfying the
	// fail-closed NFR at the level that actually matters: no
	// flagged/unvalidated content ever reaches the model). This variance was
	// reviewed and is intentionally kept as-is rather than changed to match
	// the other three tools; see spec.md's FR-007 entry and
	// implementation-notes.md for the full rationale. Locked in by
	// TestExecute_RejectedResult_DropsOnlyFlaggedResult_CallStillSucceeds in
	// websearch_filter_test.go so a future change here is deliberate, not
	// accidental.
	results, flaggedCount, matchedPatterns := w.filterFlaggedResults(ctx, rawResults)

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))

	if len(results) == 0 {
		output.WriteString("No results found.")
	} else {
		for i, result := range results {
			output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
			output.WriteString(fmt.Sprintf("   %s\n", result.URL))
			if result.Snippet != "" {
				output.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
			}
			output.WriteString("\n")
		}
	}

	// Convert results for metadata
	resultsData := make([]map[string]any, len(results))
	for i, result := range results {
		resultsData[i] = map[string]any{
			"title":   result.Title,
			"url":     result.URL,
			"snippet": result.Snippet,
		}
	}

	metadata := map[string]any{
		"query":   query,
		"count":   len(results),
		"results": resultsData,
	}
	if flaggedCount > 0 {
		metadata["injection_flagged"] = true
		metadata["matched_patterns"] = matchedPatterns
	}

	return &domain.ExecutionResult{
		Output:   output.String(),
		Metadata: metadata,
	}, nil
}

// filterFlaggedResults scans each search result's title+snippet for
// prompt-injection patterns via OutputValidator. A flagged result is either
// dropped (reject, the default) or annotated with a visible warning marker
// (annotate) and kept; clean results always pass through unchanged. A nil
// outputValidator disables scanning and returns results unchanged.
//
// A validator error (the OutputValidator interface explicitly allows one,
// even though DefaultOutputValidator never returns one in practice) is
// treated the same as a flagged/rejected result: the result is dropped, never
// passed through unvalidated. This fails closed per the feature's NFR — if
// the validator can't tell us whether content is safe, we must not assume it
// is. If every result in the batch errors, the natural result is the same
// safe empty-result outcome used when every result is flagged, rather than a
// generic error that might leak internal details.
func (w *WebSearch) filterFlaggedResults(ctx context.Context, results []searchClient.SearchResult) (filtered []searchClient.SearchResult, flaggedCount int, matchedPatterns []string) {
	if w.outputValidator == nil {
		return results, 0, nil
	}

	filtered = make([]searchClient.SearchResult, 0, len(results))
	for _, r := range results {
		content := r.Title + "\n" + r.Snippet
		result, err := w.outputValidator.ValidateToolOutput(ctx, r.URL, content)
		if err != nil {
			// Fail closed: an errored validation is not a "clean" result, so
			// drop it exactly like a flagged/rejected one rather than passing
			// unvalidated content through.
			flaggedCount++
			continue
		}
		if !result.Flagged {
			filtered = append(filtered, r)
			continue
		}

		flaggedCount++
		matchedPatterns = append(matchedPatterns, result.MatchedPatterns...)

		if result.Action == security.ValidationActionAnnotate {
			r.Snippet = security.AnnotateFlaggedContent(r.Snippet)
			filtered = append(filtered, r)
		}
		// Fail closed (default reject action): drop this result, keep the rest.
	}

	return filtered, flaggedCount, matchedPatterns
}

// RequiredPermissions returns the permissions required for this tool.
func (w *WebSearch) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the tool's configuration.
func (w *WebSearch) Config() domain.ToolConfig {
	return w.config
}
