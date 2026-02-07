package skill

import (
	"context"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// TestPreprocessRenderer_Render tests basic preprocessing
func TestPreprocessRenderer_Render(t *testing.T) {
	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `# Test Skill

Current directory contents:

!command
ls

Done.`,
	}

	result, err := renderer.Render(context.Background(), skill, []string{})

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if result == nil {
		t.Fatal("Render() returned nil")
	}

	// Should have substituted the command with output
	if strings.Contains(result.Prompt, "!command") {
		t.Error("Prompt still contains !command marker")
	}

	if !strings.Contains(result.Prompt, "MOCK OUTPUT") {
		t.Error("Prompt should contain command output")
	}
}

// TestPreprocessRenderer_MultipleCommands tests multiple command blocks
func TestPreprocessRenderer_MultipleCommands(t *testing.T) {
	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `# Test Skill

First command:

!command
git status

Second command:

!command
ls

Done.`,
	}

	result, err := renderer.Render(context.Background(), skill, []string{})

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Both commands should be substituted
	if strings.Contains(result.Prompt, "!command") {
		t.Error("Prompt still contains !command markers")
	}

	// Should have two outputs
	count := strings.Count(result.Prompt, "MOCK OUTPUT")
	if count != 2 {
		t.Errorf("Prompt should contain 2 command outputs, got %d", count)
	}
}

// TestPreprocessRenderer_CommandFailure tests error handling
func TestPreprocessRenderer_CommandFailure(t *testing.T) {
	executor := NewMockCommandExecutor()
	executor.ShouldFail = true

	renderer := NewPreprocessRenderer(executor)

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `!command
invalid-command`,
	}

	result, err := renderer.Render(context.Background(), skill, []string{})

	// Should handle error gracefully
	if err != nil {
		t.Fatalf("Render() should not error on command failure: %v", err)
	}

	// Should include error message in output
	if !strings.Contains(result.Prompt, "ERROR") && !strings.Contains(result.Prompt, "failed") {
		t.Error("Prompt should contain error message")
	}
}

// TestPreprocessRenderer_NoCommands tests skill without preprocessing
func TestPreprocessRenderer_NoCommands(t *testing.T) {
	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD:      "# Test Skill\n\nNo commands here.",
	}

	result, err := renderer.Render(context.Background(), skill, []string{})

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should return original content
	if result.Prompt != skill.BodyMD {
		t.Error("Prompt should match original body when no commands")
	}
}

// TestPreprocessRenderer_ArgumentSubstitution tests combining preprocessing with args
func TestPreprocessRenderer_ArgumentSubstitution(t *testing.T) {
	renderer := NewPreprocessRenderer(NewMockCommandExecutor())

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `# Test: $0

!command
ls

Processing: $0`,
	}

	args := []string{"myfile.txt"}
	result, err := renderer.Render(context.Background(), skill, args)

	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Should have both argument substitution and command substitution
	if !strings.Contains(result.Prompt, "myfile.txt") {
		t.Error("Prompt should contain substituted argument")
	}

	if !strings.Contains(result.Prompt, "MOCK OUTPUT") {
		t.Error("Prompt should contain command output")
	}

	if strings.Contains(result.Prompt, "$0") {
		t.Error("Prompt should not contain $0 placeholder")
	}

	if strings.Contains(result.Prompt, "!command") {
		t.Error("Prompt should not contain !command marker")
	}
}

// TestPreprocessRenderer_Caching tests result caching (optional)
func TestPreprocessRenderer_Caching(t *testing.T) {
	t.Skip("Caching not yet implemented")

	executor := NewMockCommandExecutor()
	renderer := NewPreprocessRenderer(executor)

	skill := &domain.Skill{
		Name:        "test-skill",
		Description: "Test",
		BodyMD: `!command
git status`,
	}

	// First render
	_, err := renderer.Render(context.Background(), skill, []string{})
	if err != nil {
		t.Fatalf("First render error = %v", err)
	}

	firstCallCount := executor.CallCount

	// Second render
	_, err = renderer.Render(context.Background(), skill, []string{})
	if err != nil {
		t.Fatalf("Second render error = %v", err)
	}

	// Should have used cache (same call count)
	if executor.CallCount != firstCallCount {
		t.Error("Should have used cached result")
	}
}

// MockCommandExecutor for testing
type MockCommandExecutor struct {
	ShouldFail bool
	CallCount  int
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{}
}

func (m *MockCommandExecutor) Execute(ctx context.Context, cmd domain.PreprocessCommand) (*domain.CommandResult, error) {
	m.CallCount++

	if m.ShouldFail {
		return &domain.CommandResult{
			ExitCode: 1,
			Error:    "command failed",
			Output:   "ERROR: Mock command failed",
		}, nil
	}

	return &domain.CommandResult{
		ExitCode: 0,
		Output:   "MOCK OUTPUT: " + cmd.Command,
	}, nil
}
