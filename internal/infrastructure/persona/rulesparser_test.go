package persona

import (
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

func TestParseMarkdownWithFrontmatter_ValidComplete(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
  - credential_use
blocked_tools:
  - shell_exec
privacy:
  never_store:
    - api_keys
    - passwords
---

# Rules

- Never send messages to third parties without asking me first.
- Prefer concise answers unless I ask for deep detail.
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.RequiresConfirmation) != 2 {
		t.Errorf("RequiresConfirmation len = %d, want 2", len(config.RequiresConfirmation))
	}
	if config.RequiresConfirmation[0] != "external_message" {
		t.Errorf("RequiresConfirmation[0] = %q, want %q", config.RequiresConfirmation[0], "external_message")
	}
	if config.RequiresConfirmation[1] != "credential_use" {
		t.Errorf("RequiresConfirmation[1] = %q, want %q", config.RequiresConfirmation[1], "credential_use")
	}

	if len(config.BlockedTools) != 1 {
		t.Errorf("BlockedTools len = %d, want 1", len(config.BlockedTools))
	}
	if config.BlockedTools[0] != "shell_exec" {
		t.Errorf("BlockedTools[0] = %q, want %q", config.BlockedTools[0], "shell_exec")
	}

	if len(config.Privacy.NeverStore) != 2 {
		t.Errorf("Privacy.NeverStore len = %d, want 2", len(config.Privacy.NeverStore))
	}
	if config.Privacy.NeverStore[0] != "api_keys" {
		t.Errorf("Privacy.NeverStore[0] = %q, want %q", config.Privacy.NeverStore[0], "api_keys")
	}

	if !strings.Contains(body, "# Rules") {
		t.Errorf("Body should contain '# Rules', got: %s", body)
	}
	if !strings.Contains(body, "Never send messages") {
		t.Errorf("Body should contain rules content, got: %s", body)
	}
}

func TestParseMarkdownWithFrontmatter_NoFrontmatter(t *testing.T) {
	content := `# Rules

Just plain markdown without frontmatter.
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Should return empty config
	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty, got %v", config.RequiresConfirmation)
	}
	if len(config.BlockedTools) != 0 {
		t.Errorf("BlockedTools should be empty, got %v", config.BlockedTools)
	}
	if len(config.Privacy.NeverStore) != 0 {
		t.Errorf("Privacy.NeverStore should be empty, got %v", config.Privacy.NeverStore)
	}

	// Should return full content as body
	if !strings.Contains(body, "# Rules") {
		t.Errorf("Body should contain full content, got: %s", body)
	}
}

func TestParseMarkdownWithFrontmatter_EmptyContent(t *testing.T) {
	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter("")
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty, got %v", config.RequiresConfirmation)
	}
	if body != "" {
		t.Errorf("Body should be empty, got: %q", body)
	}
}

func TestParseMarkdownWithFrontmatter_InvalidYAML(t *testing.T) {
	content := `---
requires_confirmation:
  - valid_item
blocked_tools: [unclosed
---

# Rules
`

	parser := NewRulesParser()
	_, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err == nil {
		t.Fatal("ParseMarkdownWithFrontmatter() expected error for invalid YAML, got nil")
	}

	if !strings.Contains(err.Error(), "invalid YAML frontmatter") {
		t.Errorf("Expected YAML error message, got: %v", err)
	}
}

func TestParseMarkdownWithFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := `---
---

# Rules

Some content.
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Empty YAML should produce empty config
	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty, got %v", config.RequiresConfirmation)
	}
	if !strings.Contains(body, "# Rules") {
		t.Errorf("Body should contain markdown content, got: %s", body)
	}
}

func TestParseMarkdownWithFrontmatter_FrontmatterOnly(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
---
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.RequiresConfirmation) != 1 {
		t.Errorf("RequiresConfirmation len = %d, want 1", len(config.RequiresConfirmation))
	}
	if config.RequiresConfirmation[0] != "external_message" {
		t.Errorf("RequiresConfirmation[0] = %q, want %q", config.RequiresConfirmation[0], "external_message")
	}

	if body != "" {
		t.Errorf("Body should be empty, got: %q", body)
	}
}

func TestParseMarkdownWithFrontmatter_RawYAMLPreserved(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
blocked_tools:
  - shell_exec
---

Body
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if config.RawYAML == "" {
		t.Error("RawYAML should be preserved, got empty string")
	}
	if !strings.Contains(config.RawYAML, "requires_confirmation") {
		t.Errorf("RawYAML should contain original YAML, got: %s", config.RawYAML)
	}
}

func TestParseMarkdownWithFrontmatter_OnlyOneDelimiter(t *testing.T) {
	content := `---
Some content that looks like frontmatter but only has one delimiter.
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Should treat as no frontmatter
	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty, got %v", config.RequiresConfirmation)
	}
	if body == "" {
		t.Error("Body should contain the content")
	}
}

func TestParseMarkdownWithFrontmatter_ContentBeforeFirstDelimiter(t *testing.T) {
	content := `Some content before frontmatter
---
requires_confirmation:
  - external_message
---

# Rules
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Frontmatter must start at the beginning of the file; content before delimiter
	// means this is not valid frontmatter
	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty when content precedes first delimiter, got %v", config.RequiresConfirmation)
	}
	if !strings.Contains(body, "Some content before frontmatter") {
		t.Errorf("Body should contain all content, got: %s", body)
	}
}

func TestParseMarkdownWithFrontmatter_WhitespaceHandling(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
---


   # Rules with leading whitespace

Some body content.

`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.RequiresConfirmation) != 1 {
		t.Errorf("RequiresConfirmation len = %d, want 1", len(config.RequiresConfirmation))
	}

	// Body should be trimmed
	if strings.HasPrefix(body, " ") || strings.HasPrefix(body, "\n") {
		t.Errorf("Body should be trimmed, got: %q", body)
	}
	if strings.HasSuffix(body, " ") || strings.HasSuffix(body, "\n") {
		t.Errorf("Body should be trimmed, got: %q", body)
	}
}

func TestParseMarkdownWithFrontmatter_UnknownFields(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
unknown_field: some_value
another_unknown:
  - item1
  - item2
---

# Rules
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() should tolerate unknown fields, got error: %v", err)
	}

	if len(config.RequiresConfirmation) != 1 {
		t.Errorf("RequiresConfirmation len = %d, want 1", len(config.RequiresConfirmation))
	}
}

func TestParseMarkdownWithFrontmatter_PartialConfig(t *testing.T) {
	content := `---
blocked_tools:
  - shell_exec
---

# Rules
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Only blocked_tools should be populated
	if len(config.BlockedTools) != 1 {
		t.Errorf("BlockedTools len = %d, want 1", len(config.BlockedTools))
	}
	if len(config.RequiresConfirmation) != 0 {
		t.Errorf("RequiresConfirmation should be empty, got %v", config.RequiresConfirmation)
	}
	if len(config.Privacy.NeverStore) != 0 {
		t.Errorf("Privacy.NeverStore should be empty, got %v", config.Privacy.NeverStore)
	}
}

func TestParseMarkdownWithFrontmatter_PrivacyOnly(t *testing.T) {
	content := `---
privacy:
  never_store:
    - passwords
    - api_keys
    - tokens
---

# Privacy Rules
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.Privacy.NeverStore) != 3 {
		t.Errorf("Privacy.NeverStore len = %d, want 3", len(config.Privacy.NeverStore))
	}
	if config.Privacy.NeverStore[0] != "passwords" {
		t.Errorf("Privacy.NeverStore[0] = %q, want %q", config.Privacy.NeverStore[0], "passwords")
	}
}

func TestParseMarkdownWithFrontmatter_ValidateReturnsNil(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
blocked_tools:
  - shell_exec
---

Body
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Validate should pass for well-formed configs
	if err := config.Validate(); err != nil {
		t.Errorf("Validate() should pass for parsed config, got: %v", err)
	}
}

func TestNewRulesParser(t *testing.T) {
	parser := NewRulesParser()
	if parser == nil {
		t.Fatal("NewRulesParser() returned nil")
	}
}

// Ensure the function result satisfies the domain type.
func TestParseMarkdownWithFrontmatter_ReturnsDomainType(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
---

Body
`

	parser := NewRulesParser()
	config, _, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	// Verify the returned config is a proper domain.RulesConfig
	var _ domain.RulesConfig = config
}

func TestParseMarkdownWithFrontmatter_DashesInBody(t *testing.T) {
	content := `---
requires_confirmation:
  - external_message
---

# Rules

- Rule one
- Rule two

---

## Another section after horizontal rule

Some more content.
`

	parser := NewRulesParser()
	config, body, err := parser.ParseMarkdownWithFrontmatter(content)
	if err != nil {
		t.Fatalf("ParseMarkdownWithFrontmatter() unexpected error: %v", err)
	}

	if len(config.RequiresConfirmation) != 1 {
		t.Errorf("RequiresConfirmation len = %d, want 1", len(config.RequiresConfirmation))
	}

	// Body should contain everything after second frontmatter delimiter,
	// including horizontal rules (---) in the markdown body
	if !strings.Contains(body, "---") {
		t.Errorf("Body should preserve horizontal rules (---), got: %s", body)
	}
	if !strings.Contains(body, "Another section") {
		t.Errorf("Body should contain content after horizontal rule, got: %s", body)
	}
}
