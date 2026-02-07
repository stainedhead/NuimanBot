package e2e

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	cliadapter "nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
	skillinfra "nuimanbot/internal/infrastructure/skill"
	skillusecase "nuimanbot/internal/usecase/skill"
)

// TestSkillE2E_RenderAndIntegration tests the complete skill workflow
// from discovery → rendering → chat integration (without actual LLM).
func TestSkillE2E_RenderAndIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Create temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-e2e-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 2. Create a real skill file
	skillContent := `---
name: test-e2e-skill
description: E2E test skill for integration testing
user-invocable: true
allowed-tools:
  - calculator
  - datetime
---

# E2E Test Skill

Please analyze the following arguments: $ARGUMENTS

Specifically:
- First arg: $0
- Second arg: $1

Use only the allowed tools to complete this task.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// 3. Initialize skill system components
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	// 4. Scan and load skills
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(ctx, roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 5. Verify skill was discovered
	skills := registry.List()
	if len(skills) == 0 {
		t.Fatal("Expected skill to be discovered, got none")
	}

	foundSkill := false
	for _, skill := range skills {
		if skill.Name == "test-e2e-skill" {
			foundSkill = true
			break
		}
	}
	if !foundSkill {
		t.Fatal("Expected to find 'test-e2e-skill'")
	}

	// 6. Create skill command
	skillCmd := cliadapter.NewSkillCommand(registry, renderer, os.Stdout)

	// 7. Execute skill with arguments
	args := []string{"value1", "value2"}
	rendered, err := skillCmd.Execute(ctx, "test-e2e-skill", args)
	if err != nil {
		t.Fatalf("Failed to execute skill: %v", err)
	}

	// 8. Verify rendered output
	if rendered.SkillName != "test-e2e-skill" {
		t.Errorf("Expected skill name 'test-e2e-skill', got '%s'", rendered.SkillName)
	}

	// 9. Verify argument substitution
	if !skillContains(rendered.Prompt, "value1 value2") {
		t.Error("Expected prompt to contain 'value1 value2' ($ARGUMENTS)")
	}
	if !skillContains(rendered.Prompt, "First arg: value1") {
		t.Error("Expected prompt to contain 'First arg: value1' ($0)")
	}
	if !skillContains(rendered.Prompt, "Second arg: value2") {
		t.Error("Expected prompt to contain 'Second arg: value2' ($1)")
	}

	// 10. Verify tool restrictions
	if len(rendered.AllowedTools) != 2 {
		t.Errorf("Expected 2 allowed tools, got %d", len(rendered.AllowedTools))
	}
	expectedTools := map[string]bool{"calculator": false, "datetime": false}
	for _, tool := range rendered.AllowedTools {
		if _, exists := expectedTools[tool]; exists {
			expectedTools[tool] = true
		}
	}
	for tool, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool '%s' in AllowedTools, but it was not found", tool)
		}
	}
}

// TestSkillE2E_ChatIntegration tests skill handler with message handler integration.
func TestSkillE2E_ChatIntegration(t *testing.T) {
	ctx := context.Background()

	// 1. Create temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "chat-integration-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 2. Create skill file
	skillContent := `---
name: chat-integration-skill
description: Test skill for chat integration
user-invocable: true
---

# Chat Integration Test

The user wants to know: $ARGUMENTS
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// 3. Initialize skill system
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(ctx, roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 4. Create skill command and handler
	skillCmd := cliadapter.NewSkillCommand(registry, renderer, os.Stdout)
	skillHandler := cli.NewSkillHandler(skillCmd, os.Stdout)

	// 5. Create mock message handler to capture skill-generated messages
	var capturedMessage *domain.IncomingMessage
	messageHandler := func(ctx context.Context, msg domain.IncomingMessage) error {
		capturedMessage = &msg
		return nil
	}

	// 6. Set message handler on skill handler
	skillHandler.SetMessageHandler(messageHandler, domain.PlatformCLI, "test_user")

	// 7. Execute skill through handler
	err := skillHandler.Execute(ctx, "chat-integration-skill", []string{"what is 2+2"})
	if err != nil {
		t.Fatalf("Failed to execute skill: %v", err)
	}

	// 8. Verify message was captured
	if capturedMessage == nil {
		t.Fatal("Expected message handler to be called, but it wasn't")
	}

	// 9. Verify message content
	if !skillContains(capturedMessage.Text, "what is 2+2") {
		t.Errorf("Expected message text to contain 'what is 2+2', got: %s", capturedMessage.Text)
	}

	// 10. Verify metadata
	if capturedMessage.Metadata == nil {
		t.Fatal("Expected metadata to be set")
	}

	skillName, ok := capturedMessage.Metadata["skill_name"].(string)
	if !ok || skillName != "chat-integration-skill" {
		t.Errorf("Expected skill_name metadata to be 'chat-integration-skill', got: %v", skillName)
	}

	isSkillInvoke, ok := capturedMessage.Metadata["is_skill_invoke"].(bool)
	if !ok || !isSkillInvoke {
		t.Error("Expected is_skill_invoke metadata to be true")
	}
}

// TestSkillE2E_NonUserInvocable tests that non-user-invocable skills are rejected.
func TestSkillE2E_NonUserInvocable(t *testing.T) {
	ctx := context.Background()

	// 1. Create temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "model-only-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 2. Create non-user-invocable skill
	skillContent := `---
name: model-only-skill
description: This skill can only be invoked by the model
user-invocable: false
---

# Model Only Skill

This skill should not be invocable by users.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// 3. Initialize skill system
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(ctx, roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 4. Create skill command
	skillCmd := cliadapter.NewSkillCommand(registry, renderer, os.Stdout)

	// 5. Attempt to execute non-user-invocable skill (should fail)
	_, err := skillCmd.Execute(ctx, "model-only-skill", []string{})
	if err == nil {
		t.Error("Expected error when executing non-user-invocable skill, got nil")
	}

	if !skillContains(err.Error(), "not user-invocable") {
		t.Errorf("Expected error to mention 'not user-invocable', got: %v", err)
	}
}

// TestSkillE2E_MessageHandlerError tests error handling when message handler fails.
func TestSkillE2E_MessageHandlerError(t *testing.T) {
	ctx := context.Background()

	// 1. Create temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "error-test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 2. Create skill file
	skillContent := `---
name: error-test-skill
description: Test skill for error handling
user-invocable: true
---

# Error Test Skill

Test content.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// 3. Initialize skill system
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(ctx, roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 4. Create skill command and handler
	skillCmd := cliadapter.NewSkillCommand(registry, renderer, os.Stdout)
	skillHandler := cli.NewSkillHandler(skillCmd, os.Stdout)

	// 5. Create failing message handler
	messageHandler := func(ctx context.Context, msg domain.IncomingMessage) error {
		return errors.New("simulated chat service error")
	}

	skillHandler.SetMessageHandler(messageHandler, domain.PlatformCLI, "test_user")

	// 6. Execute skill (should propagate error from message handler)
	err := skillHandler.Execute(ctx, "error-test-skill", []string{})
	if err == nil {
		t.Fatal("Expected error from message handler, got nil")
	}

	if !skillContains(err.Error(), "failed to process skill through chat service") {
		t.Errorf("Expected error message to mention chat service failure, got: %v", err)
	}
}

// TestSkillE2E_WithoutMessageHandler tests fallback behavior when no message handler is set.
func TestSkillE2E_WithoutMessageHandler(t *testing.T) {
	ctx := context.Background()

	// 1. Create temporary skill directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "fallback-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// 2. Create skill file
	skillContent := `---
name: fallback-skill
description: Test skill for fallback behavior
user-invocable: true
---

# Fallback Test

This skill should display without processing through chat.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// 3. Initialize skill system
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(ctx, roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 4. Create skill handler WITHOUT setting message handler
	skillCmd := cliadapter.NewSkillCommand(registry, renderer, os.Stdout)
	skillHandler := cli.NewSkillHandler(skillCmd, os.Stdout)

	// 5. Execute skill (should work in fallback mode)
	err := skillHandler.Execute(ctx, "fallback-skill", []string{})
	if err != nil {
		t.Fatalf("Expected fallback execution to succeed, got error: %v", err)
	}

	// Success - no error means fallback mode worked
}

// skillContains checks if haystack skillContains needle (local version to avoid redeclaration)
func skillContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && skillIndexOf(haystack, needle) >= 0
}

// skillIndexOf returns the index of needle in haystack, or -1 if not found
func skillIndexOf(haystack, needle string) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
