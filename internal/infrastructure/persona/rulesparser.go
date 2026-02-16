package persona

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"nuimanbot/internal/domain"
)

// RulesParser parses YAML frontmatter from RULES.md Markdown files.
type RulesParser struct{}

// NewRulesParser creates a new RulesParser.
func NewRulesParser() *RulesParser {
	return &RulesParser{}
}

// ParseMarkdownWithFrontmatter extracts YAML frontmatter and Markdown body
// from RULES.md content. Frontmatter must appear at the very start of the
// file, delimited by "---" lines.
//
// Returns the parsed RulesConfig, the Markdown body (trimmed), and any error.
// If no frontmatter is found, returns an empty RulesConfig and the full
// content as body with no error (graceful degradation).
func (p *RulesParser) ParseMarkdownWithFrontmatter(content string) (domain.RulesConfig, string, error) {
	var config domain.RulesConfig

	if content == "" {
		return config, "", nil
	}

	// Frontmatter must start at the beginning of the file with "---"
	if !strings.HasPrefix(content, "---") {
		return config, strings.TrimSpace(content), nil
	}

	// Split on "---" into at most 3 parts: before first delimiter, YAML, body
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		// Only one delimiter found — not valid frontmatter
		return config, strings.TrimSpace(content), nil
	}

	// parts[0] should be empty (content starts with "---")
	// parts[1] is the YAML frontmatter
	// parts[2] is the Markdown body
	yamlContent := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	// Empty frontmatter (---\n---) is valid — return empty config
	if yamlContent == "" {
		return config, body, nil
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		return domain.RulesConfig{}, "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	// Preserve raw YAML after unmarshal so it can't be overwritten by parsed fields
	config.RawYAML = yamlContent

	return config, body, nil
}
