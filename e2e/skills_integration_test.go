package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/skill"
	usecaseskill "nuimanbot/internal/usecase/skill"
)

// TestSkillsSystem_E2E tests the entire skills system end-to-end
func TestSkillsSystem_E2E(t *testing.T) {
	// Get project root
	projectRoot := filepath.Join("..", ".") // e2e is in project root

	// Create skills config
	skillsConfig := &config.SkillsConfig{
		Enabled: true,
		Roots: []config.SkillRootConfig{
			{
				Path:  filepath.Join(projectRoot, ".claude/skills"),
				Scope: domain.ScopeProject,
			},
		},
	}

	// Get skill roots
	roots, err := skillsConfig.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots() failed: %v", err)
	}

	// Create infrastructure components
	repo := skill.NewFilesystemSkillRepository()
	registry := usecaseskill.NewInMemorySkillRegistry(repo)
	renderer := usecaseskill.NewDefaultSkillRenderer()

	// Initialize registry (scan and load skills)
	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Registry.Initialize() failed: %v", err)
	}

	// Verify test-skill was loaded
	testSkill, err := registry.Get("test-skill")
	if err != nil {
		t.Fatalf("Registry.Get('test-skill') failed: %v", err)
	}

	if testSkill.Name != "test-skill" {
		t.Errorf("Skill name = %q, want 'test-skill'", testSkill.Name)
	}

	t.Logf("✓ Successfully loaded skill: %s", testSkill.Name)
	t.Logf("  Description: %s", testSkill.Description)
	t.Logf("  Scope: %s", testSkill.Scope)
	t.Logf("  User-invocable: %v", testSkill.CanBeInvokedByUser())
	t.Logf("  Model-invocable: %v", testSkill.CanBeSelectedByModel())

	// Test rendering with arguments
	args := []string{"file1.go", "file2.go", "file3.go"}
	rendered, err := renderer.Render(testSkill, args)
	if err != nil {
		t.Fatalf("Renderer.Render() failed: %v", err)
	}

	// Verify arguments were substituted
	if rendered.SkillName != "test-skill" {
		t.Errorf("RenderedSkill.SkillName = %q, want 'test-skill'", rendered.SkillName)
	}

	// Check that $0, $1, and $ARGUMENTS were substituted
	prompt := rendered.Prompt
	if !contains(prompt, "file1.go") {
		t.Error("Rendered prompt should contain 'file1.go' (from $0)")
	}
	if !contains(prompt, "file2.go") {
		t.Error("Rendered prompt should contain 'file2.go' (from $1)")
	}
	if !contains(prompt, "file1.go file2.go file3.go") {
		t.Error("Rendered prompt should contain full arguments (from $ARGUMENTS)")
	}

	t.Logf("✓ Successfully rendered skill with arguments")
	t.Logf("  Arguments: %v", args)
	t.Logf("  Allowed tools: %v", rendered.AllowedTools)
	t.Logf("  Prompt length: %d bytes", len(rendered.Prompt))

	// Test catalog functionality
	catalog := registry.Catalog()
	if len(catalog) == 0 {
		t.Error("Catalog should contain at least the test-skill")
	}

	t.Logf("✓ Catalog contains %d skill(s)", len(catalog))
	for _, entry := range catalog {
		t.Logf("  - %s: %s (scope: %s)", entry.Name, entry.Description, entry.Scope)
	}

	// Test user-invocable catalog
	userCatalog := registry.UserInvocableCatalog()
	foundTestSkill := false
	for _, entry := range userCatalog {
		if entry.Name == "test-skill" {
			foundTestSkill = true
			break
		}
	}

	if !foundTestSkill {
		t.Error("test-skill should be in user-invocable catalog")
	}

	t.Logf("✓ User-invocable catalog works correctly")

	// Success message
	t.Log("\n=== Skills System E2E Test PASSED ===")
	t.Log("All components working together:")
	t.Log("  ✓ Configuration")
	t.Log("  ✓ Filesystem scanning")
	t.Log("  ✓ YAML parsing")
	t.Log("  ✓ Skill registration")
	t.Log("  ✓ Skill rendering")
	t.Log("  ✓ Catalog generation")
}

// TestSkillsSystem_PriorityResolution tests skill priority resolution
func TestSkillsSystem_PriorityResolution(t *testing.T) {
	// Create two roots with different scopes for the same skill
	tmpDir := t.TempDir()

	// Create project-scoped skill
	projectSkillDir := filepath.Join(tmpDir, "project", "test-priority")
	if err := createTestSkill(projectSkillDir, "test-priority", "Project version", domain.ScopeProject); err != nil {
		t.Fatalf("Failed to create project skill: %v", err)
	}

	// Create user-scoped skill (higher priority)
	userSkillDir := filepath.Join(tmpDir, "user", "test-priority")
	if err := createTestSkill(userSkillDir, "test-priority", "User version", domain.ScopeUser); err != nil {
		t.Fatalf("Failed to create user skill: %v", err)
	}

	// Create registry with both roots
	repo := skill.NewFilesystemSkillRepository()
	registry := usecaseskill.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: filepath.Join(tmpDir, "project"), Scope: domain.ScopeProject},
		{Path: filepath.Join(tmpDir, "user"), Scope: domain.ScopeUser},
	}

	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Get the skill - should be user version (higher priority)
	retrieved, err := registry.Get("test-priority")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.Scope != domain.ScopeUser {
		t.Errorf("Retrieved skill scope = %v, want %v (higher priority should win)", retrieved.Scope, domain.ScopeUser)
	}

	if retrieved.Description != "User version" {
		t.Errorf("Retrieved skill description = %q, want 'User version'", retrieved.Description)
	}

	t.Log("✓ Priority resolution works correctly - User scope overrides Project scope")
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper to create a test skill
func createTestSkill(dir, name, description string, scope domain.SkillScope) error {
	if err := mkdirAll(dir); err != nil {
		return err
	}

	content := `---
name: ` + name + `
description: ` + description + `
user-invocable: true
---

# ` + name + `

This is a test skill for ` + description + `.
Scope: ` + scope.String() + `
`

	return writeFile(filepath.Join(dir, "SKILL.md"), []byte(content))
}

// Helper functions
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
