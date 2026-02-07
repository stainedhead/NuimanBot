package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/skill"
	"nuimanbot/internal/infrastructure/subagent"
	skillUsecase "nuimanbot/internal/usecase/skill"
)

// TestSubagentE2E_ForkSkillExecution tests end-to-end subagent execution
func TestSubagentE2E_ForkSkillExecution(t *testing.T) {
	// Create temp directory for test skills
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-fork-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create a test skill with context: fork
	skillContent := `---
name: test-fork-skill
description: A test skill that executes in a forked context
context: fork
allowed-tools:
  - read
  - write
---

# Test Fork Skill

This skill should execute in a forked subagent context.

Execute the following task: Analyze the test scenario.
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Initialize components
	repo := skill.NewFilesystemSkillRepository()
	registry := skillUsecase.NewInMemorySkillRegistry(repo)

	// Add test skill directory to registry
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Create mock executor for testing
	mockExecutor := &mockSubagentExecutor{
		results: make(map[string]*domain.SubagentResult),
	}

	// Create lifecycle manager
	lifecycleManager := subagent.NewLifecycleManager(mockExecutor)

	// Create CLI command
	renderer := skillUsecase.NewDefaultSkillRenderer()
	output := &testOutput{}
	skillCmd := cli.NewSkillCommand(registry, renderer, output)
	skillCmd.SetLifecycleManager(lifecycleManager)

	// Execute the fork skill
	ctx := context.Background()
	result, err := skillCmd.Execute(ctx, "test-fork-skill", []string{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify output contains notification
	found := false
	for _, msg := range output.messages {
		if contains(string(msg), "Started subagent") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Output missing 'Started subagent' notification")
	}

	// Give executor a moment to start
	time.Sleep(50 * time.Millisecond)

	// Verify subagent was started
	if len(mockExecutor.executeCalls) != 1 {
		t.Fatalf("Expected 1 Execute call, got %d", len(mockExecutor.executeCalls))
	}

	executed := mockExecutor.executeCalls[0]
	if executed.SkillName != "test-fork-skill" {
		t.Errorf("Executed skill = %s, want test-fork-skill", executed.SkillName)
	}

	if len(executed.AllowedTools) != 2 {
		t.Errorf("AllowedTools count = %d, want 2", len(executed.AllowedTools))
	}
}

// TestSubagentE2E_InlineSkillExecution tests that normal skills execute inline
func TestSubagentE2E_InlineSkillExecution(t *testing.T) {
	// Create temp directory for test skills
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-inline-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	// Create a test skill WITHOUT context: fork
	skillContent := `---
name: test-inline-skill
description: A normal skill that executes inline
---

# Test Inline Skill

This skill should execute normally (inline).
`

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		t.Fatalf("Failed to write skill file: %v", err)
	}

	// Initialize components
	repo := skill.NewFilesystemSkillRepository()
	registry := skillUsecase.NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}
	if err := registry.Initialize(context.Background(), roots); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	mockExecutor := &mockSubagentExecutor{
		results: make(map[string]*domain.SubagentResult),
	}

	lifecycleManager := subagent.NewLifecycleManager(mockExecutor)

	renderer := skillUsecase.NewDefaultSkillRenderer()
	output := &testOutput{}
	skillCmd := cli.NewSkillCommand(registry, renderer, output)
	skillCmd.SetLifecycleManager(lifecycleManager)

	// Execute the inline skill
	ctx := context.Background()
	result, err := skillCmd.Execute(ctx, "test-inline-skill", []string{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify NO subagent was started
	time.Sleep(50 * time.Millisecond)
	if len(mockExecutor.executeCalls) != 0 {
		t.Errorf("Expected 0 Execute calls for inline skill, got %d", len(mockExecutor.executeCalls))
	}

	// Verify output does NOT contain subagent notification
	for _, msg := range output.messages {
		if contains(string(msg), "Started subagent") {
			t.Error("Output should not contain 'Started subagent' for inline skill")
			break
		}
	}
}

// TestSubagentE2E_LifecycleOperations tests subagent lifecycle operations
func TestSubagentE2E_LifecycleOperations(t *testing.T) {
	mockExecutor := &mockSubagentExecutor{
		results: make(map[string]*domain.SubagentResult),
		delay:   200 * time.Millisecond,
	}

	lifecycleManager := subagent.NewLifecycleManager(mockExecutor)

	// Create and start a subagent
	subagentCtx := domain.SubagentContext{
		ID:              "e2e-test-subagent",
		ParentContextID: "parent",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Test"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	if err := lifecycleManager.Start(ctx, subagentCtx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Get status - should be running
	status, err := lifecycleManager.GetStatus(ctx, "e2e-test-subagent")
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Status != domain.SubagentStatusRunning && status.Status != domain.SubagentStatusComplete {
		t.Errorf("Status = %v, want running or complete", status.Status)
	}

	// List running
	running := lifecycleManager.ListRunning(ctx)
	if len(running) == 0 && status.Status == domain.SubagentStatusRunning {
		t.Error("ListRunning() should return the running subagent")
	}

	// Cancel
	if err := lifecycleManager.Cancel(ctx, "e2e-test-subagent"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	// Verify cancelled
	time.Sleep(100 * time.Millisecond)
	status, _ = lifecycleManager.GetStatus(ctx, "e2e-test-subagent")
	if status.Status != domain.SubagentStatusCancelled {
		t.Errorf("Status after cancel = %v, want cancelled", status.Status)
	}
}

// Mock executor for E2E tests
type mockSubagentExecutor struct {
	executeCalls []domain.SubagentContext
	results      map[string]*domain.SubagentResult
	delay        time.Duration
}

func (m *mockSubagentExecutor) Execute(ctx context.Context, subagentCtx domain.SubagentContext) (*domain.SubagentResult, error) {
	m.executeCalls = append(m.executeCalls, subagentCtx)

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return &domain.SubagentResult{
				SubagentID:   subagentCtx.ID,
				Status:       domain.SubagentStatusCancelled,
				ErrorMessage: "cancelled",
			}, nil
		}
	}

	return &domain.SubagentResult{
		SubagentID: subagentCtx.ID,
		Status:     domain.SubagentStatusComplete,
		Output:     "Mock execution complete",
	}, nil
}

func (m *mockSubagentExecutor) Cancel(ctx context.Context, subagentID string) error {
	return nil
}

func (m *mockSubagentExecutor) GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	if result, ok := m.results[subagentID]; ok {
		return result, nil
	}
	return &domain.SubagentResult{
		SubagentID: subagentID,
		Status:     domain.SubagentStatusRunning,
	}, nil
}

// Test output capture
type testOutput struct {
	messages [][]byte
}

func (t *testOutput) Write(p []byte) (n int, err error) {
	// Make a copy of the bytes
	copied := make([]byte, len(p))
	copy(copied, p)
	t.messages = append(t.messages, copied)
	return len(p), nil
}
