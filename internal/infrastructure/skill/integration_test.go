package skill

import (
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

func TestIntegration_ParseValidSkill(t *testing.T) {
	parser := NewSkillParser()
	skillFile := filepath.Join("testdata", "valid-skill", "SKILL.md")

	skill, err := parser.Parse(skillFile, domain.ScopeProject)
	if err != nil {
		t.Fatalf("Parse() failed for valid-skill: %v", err)
	}

	if skill.Name != "valid-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "valid-skill")
	}

	if skill.Description != "A valid test skill for integration testing" {
		t.Errorf("Description = %q, unexpected value", skill.Description)
	}

	if len(skill.AllowedTools()) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(skill.AllowedTools()))
	}

	if !skill.CanBeInvokedByUser() {
		t.Error("Expected skill to be user-invocable")
	}
}

func TestIntegration_ParseInvalidYAML(t *testing.T) {
	parser := NewSkillParser()
	skillFile := filepath.Join("testdata", "invalid-yaml", "SKILL.md")

	_, err := parser.Parse(skillFile, domain.ScopeProject)
	if err == nil {
		t.Fatal("Parse() should fail for invalid-yaml, got nil error")
	}

	if _, ok := err.(domain.ErrSkillParseError); !ok {
		t.Errorf("Expected ErrSkillParseError, got %T", err)
	}
}

func TestIntegration_ParseMissingName(t *testing.T) {
	parser := NewSkillParser()
	skillFile := filepath.Join("testdata", "missing-name", "SKILL.md")

	_, err := parser.Parse(skillFile, domain.ScopeProject)
	if err == nil {
		t.Fatal("Parse() should fail for missing-name, got nil error")
	}

	if _, ok := err.(domain.ErrSkillParseError); !ok {
		t.Errorf("Expected ErrSkillParseError, got %T", err)
	}
}

func TestIntegration_ParseNameMismatch(t *testing.T) {
	parser := NewSkillParser()
	skillFile := filepath.Join("testdata", "name-mismatch", "SKILL.md")

	_, err := parser.Parse(skillFile, domain.ScopeProject)
	if err == nil {
		t.Fatal("Parse() should fail for name-mismatch, got nil error")
	}

	if _, ok := err.(domain.ErrSkillParseError); !ok {
		t.Errorf("Expected ErrSkillParseError, got %T", err)
	}
}

func TestIntegration_ScanTestdata(t *testing.T) {
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: "testdata", Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	// Should only find valid-skill (others are invalid or missing SKILL.md)
	if len(skills) != 1 {
		t.Errorf("Scan() found %d skills, want 1 (only valid-skill should pass)", len(skills))
	}

	if len(skills) > 0 && skills[0].Name != "valid-skill" {
		t.Errorf("Expected to find valid-skill, got %q", skills[0].Name)
	}
}

func TestIntegration_LoadValidSkill(t *testing.T) {
	repo := NewFilesystemSkillRepository()
	skillFile := filepath.Join("testdata", "valid-skill", "SKILL.md")

	skill, err := repo.Load(skillFile)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if skill.Name != "valid-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "valid-skill")
	}

	// Verify body was loaded
	if skill.BodyMD == "" {
		t.Error("BodyMD should not be empty")
	}

	// Verify frontmatter was parsed
	if skill.Frontmatter.Name != "valid-skill" {
		t.Errorf("Frontmatter.Name = %q, want %q", skill.Frontmatter.Name, "valid-skill")
	}
}
