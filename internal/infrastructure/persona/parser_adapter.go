package persona

import "nuimanbot/internal/domain"

// RulesParserAdapter adapts RulesParser to the FrontmatterParser interface.
type RulesParserAdapter struct {
	parser *RulesParser
}

// NewRulesParserAdapter creates a new adapter.
func NewRulesParserAdapter() *RulesParserAdapter {
	return &RulesParserAdapter{
		parser: NewRulesParser(),
	}
}

// ParseMarkdownWithFrontmatter implements the FrontmatterParser interface.
func (a *RulesParserAdapter) ParseMarkdownWithFrontmatter(content string) (domain.RulesConfig, string, error) {
	return a.parser.ParseMarkdownWithFrontmatter(content)
}
