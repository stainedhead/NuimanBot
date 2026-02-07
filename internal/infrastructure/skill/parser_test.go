package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
user-invocable: true
allowed-tools:
  - calculator
  - datetime
---

# Test Skill

This is the body of the skill.
`

	parser := NewSkillParser()
	fm, body, err := parser.ParseFrontmatter([]byte(content))

	if err != nil {
		t.Fatalf("ParseFrontmatter() unexpected error: %v", err)
	}

	if fm == nil {
		t.Fatal("ParseFrontmatter() frontmatter is nil")
	}

	if fm.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "test-skill")
	}

	if fm.Description != "A test skill" {
		t.Errorf("Description = %q, want %q", fm.Description, "A test skill")
	}

	if fm.UserInvocable == nil || !*fm.UserInvocable {
		t.Error("UserInvocable should be true")
	}

	if len(fm.AllowedTools) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(fm.AllowedTools))
	}

	if !strings.Contains(body, "# Test Skill") {
		t.Errorf("Body missing expected content, got: %s", body)
	}
}

func TestParseFrontmatter_MissingDelimiters(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "No delimiters",
			content: "name: test\ndescription: test",
		},
		{
			name:    "Only one delimiter",
			content: "---\nname: test\ndescription: test",
		},
		{
			name:    "Empty content",
			content: "",
		},
	}

	parser := NewSkillParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parser.ParseFrontmatter([]byte(tt.content))
			if err == nil {
				t.Error("ParseFrontmatter() expected error, got nil")
			}
			if !strings.Contains(err.Error(), "missing frontmatter delimiters") {
				t.Errorf("Expected delimiter error, got: %v", err)
			}
		})
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
invalid-yaml-here: [unclosed
---

Body
`

	parser := NewSkillParser()
	_, _, err := parser.ParseFrontmatter([]byte(content))

	if err == nil {
		t.Fatal("ParseFrontmatter() expected error for invalid YAML, got nil")
	}

	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("Expected YAML error, got: %v", err)
	}
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	content := `---
---

Body
`

	parser := NewSkillParser()
	_, _, err := parser.ParseFrontmatter([]byte(content))

	if err == nil {
		t.Fatal("ParseFrontmatter() expected error for empty frontmatter, got nil")
	}

	if !strings.Contains(err.Error(), "frontmatter is empty") {
		t.Errorf("Expected empty frontmatter error, got: %v", err)
	}
}

func TestParseFrontmatter_UnknownFields(t *testing.T) {
	content := `---
name: test-skill
description: A test skill
unknown-field: value
future-field: another-value
---

Body
`

	parser := NewSkillParser()
	fm, _, err := parser.ParseFrontmatter([]byte(content))

	if err != nil {
		t.Fatalf("ParseFrontmatter() should tolerate unknown fields, got error: %v", err)
	}

	if fm.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "test-skill")
	}
}

func TestValidateFrontmatter_Valid(t *testing.T) {
	fm := &domain.SkillFrontmatter{
		Name:        "test-skill",
		Description: "A test skill",
	}

	parser := NewSkillParser()
	err := parser.ValidateFrontmatter(fm, "test-skill")

	if err != nil {
		t.Errorf("ValidateFrontmatter() unexpected error: %v", err)
	}
}

func TestValidateFrontmatter_NameMismatch(t *testing.T) {
	fm := &domain.SkillFrontmatter{
		Name:        "skill-one",
		Description: "A test skill",
	}

	parser := NewSkillParser()
	err := parser.ValidateFrontmatter(fm, "skill-two")

	if err == nil {
		t.Fatal("ValidateFrontmatter() expected error for name mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "does not match directory name") {
		t.Errorf("Expected name mismatch error, got: %v", err)
	}
}

func TestValidateFrontmatter_EmptyExpectedName(t *testing.T) {
	fm := &domain.SkillFrontmatter{
		Name:        "test-skill",
		Description: "A test skill",
	}

	parser := NewSkillParser()
	err := parser.ValidateFrontmatter(fm, "")

	if err != nil {
		t.Errorf("ValidateFrontmatter() should skip name check when expectedName is empty, got error: %v", err)
	}
}

func TestValidateFrontmatter_InvalidFrontmatter(t *testing.T) {
	fm := &domain.SkillFrontmatter{
		Name: "", // Invalid: empty name
	}

	parser := NewSkillParser()
	err := parser.ValidateFrontmatter(fm, "")

	if err == nil {
		t.Fatal("ValidateFrontmatter() expected error for invalid frontmatter, got nil")
	}
}

func TestParse_ValidFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Create SKILL.md
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: test-skill
description: A test skill
---

# Test Skill

This is the body.
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse
	parser := NewSkillParser()
	skill, err := parser.Parse(skillFile, domain.ScopeUser)

	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "test-skill")
	}

	if skill.Scope != domain.ScopeUser {
		t.Errorf("Scope = %v, want %v", skill.Scope, domain.ScopeUser)
	}

	if skill.Priority != domain.ScopeUser.Priority() {
		t.Errorf("Priority = %d, want %d", skill.Priority, domain.ScopeUser.Priority())
	}

	if skill.ID != "user:test-skill" {
		t.Errorf("ID = %q, want %q", skill.ID, "user:test-skill")
	}

	if skill.Directory != skillDir {
		t.Errorf("Directory = %q, want %q", skill.Directory, skillDir)
	}

	if skill.FilePath != skillFile {
		t.Errorf("FilePath = %q, want %q", skill.FilePath, skillFile)
	}
}

func TestParse_FileNotFound(t *testing.T) {
	parser := NewSkillParser()
	_, err := parser.Parse("/nonexistent/path/SKILL.md", domain.ScopeProject)

	if err == nil {
		t.Fatal("Parse() expected error for nonexistent file, got nil")
	}

	if _, ok := err.(domain.ErrSkillParseError); !ok {
		t.Errorf("Expected ErrSkillParseError, got %T", err)
	}
}

func TestParse_EmptyBody(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "empty-body")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Create SKILL.md with empty body
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: empty-body
description: A skill with empty body
---
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse
	parser := NewSkillParser()
	skill, err := parser.Parse(skillFile, domain.ScopeProject)

	if err != nil {
		t.Fatalf("Parse() should allow empty body, got error: %v", err)
	}

	if skill.BodyMD != "" {
		t.Errorf("BodyMD = %q, want empty string", skill.BodyMD)
	}
}

func TestParse_NameMismatch(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skill-one")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Create SKILL.md with different name
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: skill-two
description: Name mismatch test
---

Body
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse
	parser := NewSkillParser()
	_, err := parser.Parse(skillFile, domain.ScopeProject)

	if err == nil {
		t.Fatal("Parse() expected error for name mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "does not match directory name") {
		t.Errorf("Expected name mismatch error, got: %v", err)
	}
}

func TestParse_InvalidSkill(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "invalid-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Create SKILL.md with invalid frontmatter (missing description)
	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: invalid-skill
---

Body
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse
	parser := NewSkillParser()
	_, err := parser.Parse(skillFile, domain.ScopeProject)

	if err == nil {
		t.Fatal("Parse() expected error for invalid skill, got nil")
	}
}
