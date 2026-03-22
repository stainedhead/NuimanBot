package persona_test

import (
	"testing"

	"nuimanbot/internal/infrastructure/persona"
)

// TestNewRulesParserAdapter tests the constructor.
func TestNewRulesParserAdapter(t *testing.T) {
	adapter := persona.NewRulesParserAdapter()
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}
}

// TestRulesParserAdapter_ParseMarkdownWithFrontmatter tests parsing through the adapter.
func TestRulesParserAdapter_ParseMarkdownWithFrontmatter(t *testing.T) {
	adapter := persona.NewRulesParserAdapter()

	content := `---
version: "1.0"
---
# Rules

Some rules here.
`

	config, body, err := adapter.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() error = %v", err)
	}

	_ = config // Version is parsed
	if body == "" {
		t.Error("Expected non-empty body")
	}
}

// TestRulesParserAdapter_ParseEmpty tests parsing empty content.
func TestRulesParserAdapter_ParseEmpty(t *testing.T) {
	adapter := persona.NewRulesParserAdapter()

	config, body, err := adapter.ParseMarkdownWithFrontmatter("")
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() error = %v", err)
	}
	if body != "" {
		t.Errorf("Expected empty body, got %q", body)
	}
	_ = config
}
