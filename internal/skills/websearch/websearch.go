package websearch

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	searchClient "nuimanbot/internal/infrastructure/search"
)

// WebSearch implements the domain.Skill interface for web search.
type WebSearch struct {
	client *searchClient.Client
	config domain.SkillConfig
}

// NewWebSearch creates a new WebSearch skill.
func NewWebSearch(timeoutSeconds int) *WebSearch {
	return &WebSearch{
		client: searchClient.NewClient(timeoutSeconds),
		config: domain.SkillConfig{
			Enabled: true,
		},
	}
}

// Name returns the skill name.
func (w *WebSearch) Name() string {
	return "websearch"
}

// Description returns the skill description.
func (w *WebSearch) Description() string {
	return "Perform web searches using DuckDuckGo and return relevant results"
}

// InputSchema returns the JSON schema for the skill's input parameters.
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
func (w *WebSearch) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	// Extract and validate query
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return &domain.SkillResult{
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
		return &domain.SkillResult{
			Error: "limit must be between 1 and 50",
		}, nil
	}

	// Perform search
	results, err := w.client.Search(ctx, query, limit)
	if err != nil {
		return &domain.SkillResult{
			Error: fmt.Sprintf("search failed: %v", err),
		}, nil
	}

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

	return &domain.SkillResult{
		Output: output.String(),
		Metadata: map[string]any{
			"query":   query,
			"count":   len(results),
			"results": resultsData,
		},
	}, nil
}

// RequiredPermissions returns the permissions required for this skill.
func (w *WebSearch) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the skill's configuration.
func (w *WebSearch) Config() domain.SkillConfig {
	return w.config
}
