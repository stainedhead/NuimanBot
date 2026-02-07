package cli_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain"
	skillinfra "nuimanbot/internal/infrastructure/skill"
	skillusecase "nuimanbot/internal/usecase/skill"
)

// TestSkillIntegration_EndToEnd tests the complete skill workflow:
// 1. Skill discovery from filesystem
// 2. Registry initialization
// 3. Skill invocation via CLI command
// 4. Argument substitution and rendering
func TestSkillIntegration_EndToEnd(t *testing.T) {
	ctx := context.Background()

	// 1. Setup: Get test fixture directory
	fixtureDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get fixture directory: %v", err)
	}

	// 2. Create infrastructure layer: Filesystem repository
	repo := skillinfra.NewFilesystemSkillRepository()

	// 3. Create use case layer: Registry and renderer
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	// 4. Initialize registry by scanning test fixtures
	roots := []domain.SkillRoot{
		{Path: fixtureDir, Scope: domain.ScopeProject},
	}
	err = registry.Initialize(ctx, roots)
	if err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// 5. Verify skill was discovered
	skills := registry.List()
	if len(skills) == 0 {
		t.Fatal("Expected at least one skill to be discovered, got none")
	}

	foundTestSkill := false
	for _, skill := range skills {
		if skill.Name == "test-skill" {
			foundTestSkill = true
			break
		}
	}
	if !foundTestSkill {
		t.Fatal("Expected to find 'test-skill', but it was not discovered")
	}

	// 6. Create adapter layer: CLI command handler
	skillCmd := cli.NewSkillCommand(registry, renderer, io.Discard)

	// 7. Execute skill with arguments
	args := []string{"arg1", "arg2"}
	rendered, err := skillCmd.Execute(ctx, "test-skill", args)
	if err != nil {
		t.Fatalf("Failed to execute skill: %v", err)
	}

	// 8. Verify rendered output
	if rendered == nil {
		t.Fatal("Expected non-nil rendered skill")
	}

	if rendered.SkillName != "test-skill" {
		t.Errorf("Expected skill name 'test-skill', got '%s'", rendered.SkillName)
	}

	// 9. Verify argument substitution worked correctly
	expectedSubstitutions := map[string]bool{
		"arg1 arg2":             false, // $ARGUMENTS
		"First argument: arg1":  false, // $0
		"Second argument: arg2": false, // $1
	}

	for expected := range expectedSubstitutions {
		if contains(rendered.Prompt, expected) {
			expectedSubstitutions[expected] = true
		}
	}

	for expected, found := range expectedSubstitutions {
		if !found {
			t.Errorf("Expected rendered prompt to contain '%s', but it was not found", expected)
		}
	}

	// 10. Verify original placeholders were replaced
	if contains(rendered.Prompt, "$ARGUMENTS") {
		t.Error("Prompt should not contain $ARGUMENTS placeholder after rendering")
	}
	if contains(rendered.Prompt, "$0") {
		t.Error("Prompt should not contain $0 placeholder after rendering")
	}
	if contains(rendered.Prompt, "$1") {
		t.Error("Prompt should not contain $1 placeholder after rendering")
	}
}

// TestSkillIntegration_UserInvocableFilter tests that only user-invocable skills
// are included in the user catalog
func TestSkillIntegration_UserInvocableFilter(t *testing.T) {
	ctx := context.Background()

	// Setup
	fixtureDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get fixture directory: %v", err)
	}

	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: fixtureDir, Scope: domain.ScopeProject},
	}
	err = registry.Initialize(ctx, roots)
	if err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Get user-invocable catalog
	userCatalog := registry.UserInvocableCatalog()

	// Verify test-skill is in user catalog (it's marked user-invocable: true)
	foundTestSkill := false
	for _, entry := range userCatalog {
		if entry.Name == "test-skill" {
			foundTestSkill = true
			break
		}
	}

	if !foundTestSkill {
		t.Error("Expected 'test-skill' to be in user-invocable catalog")
	}
}

// TestSkillIntegration_ListCommand tests the List command with real skills
func TestSkillIntegration_ListCommand(t *testing.T) {
	ctx := context.Background()

	// Setup
	fixtureDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get fixture directory: %v", err)
	}

	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: fixtureDir, Scope: domain.ScopeProject},
	}
	err = registry.Initialize(ctx, roots)
	if err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Create skill command with buffer to capture output
	var output testBuffer
	skillCmd := cli.NewSkillCommand(registry, renderer, &output)

	// Execute list command
	err = skillCmd.List(ctx)
	if err != nil {
		t.Fatalf("List command failed: %v", err)
	}

	// Verify output contains skill information
	outputStr := output.String()
	if !contains(outputStr, "test-skill") {
		t.Errorf("List output should contain 'test-skill', got: %s", outputStr)
	}
	if !contains(outputStr, "integration testing") {
		t.Errorf("List output should contain skill description, got: %s", outputStr)
	}
}

// TestSkillIntegration_DescribeCommand tests the Describe command with real skills
func TestSkillIntegration_DescribeCommand(t *testing.T) {
	ctx := context.Background()

	// Setup
	fixtureDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("Failed to get fixture directory: %v", err)
	}

	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	roots := []domain.SkillRoot{
		{Path: fixtureDir, Scope: domain.ScopeProject},
	}
	err = registry.Initialize(ctx, roots)
	if err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Create skill command with buffer to capture output
	var output testBuffer
	skillCmd := cli.NewSkillCommand(registry, renderer, &output)

	// Execute describe command
	err = skillCmd.Describe(ctx, "test-skill")
	if err != nil {
		t.Fatalf("Describe command failed: %v", err)
	}

	// Verify output contains detailed skill information
	outputStr := output.String()
	if !contains(outputStr, "test-skill") {
		t.Errorf("Describe output should contain skill name, got: %s", outputStr)
	}
	if !contains(outputStr, "User-invocable: true") {
		t.Errorf("Describe output should show user-invocable status, got: %s", outputStr)
	}
	if !contains(outputStr, "$ARGUMENTS") {
		t.Errorf("Describe output should contain skill body with placeholders, got: %s", outputStr)
	}
}

// testBuffer is a simple string buffer for testing output
type testBuffer struct {
	data []byte
}

func (b *testBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *testBuffer) String() string {
	return string(b.data)
}

// contains checks if haystack contains needle
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || len(haystack) > len(needle) &&
			indexOf(haystack, needle) >= 0)
}

// indexOf returns the index of needle in haystack, or -1 if not found
func indexOf(haystack, needle string) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
