package cli

import (
	"bytes"
	"context"
	"errors"
	"nuimanbot/internal/domain"
	"strings"
	"testing"
)

// Mock LifecycleManager for testing
type mockLifecycleManager struct {
	started        []domain.SubagentContext
	cancelled      []string
	getStatusCalls []string
	statusResults  map[string]*domain.SubagentResult
}

func (m *mockLifecycleManager) Start(ctx context.Context, subagentCtx domain.SubagentContext) error {
	m.started = append(m.started, subagentCtx)
	return nil
}

func (m *mockLifecycleManager) Cancel(ctx context.Context, subagentID string) error {
	m.cancelled = append(m.cancelled, subagentID)
	return nil
}

func (m *mockLifecycleManager) GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	m.getStatusCalls = append(m.getStatusCalls, subagentID)
	if result, ok := m.statusResults[subagentID]; ok {
		return result, nil
	}
	return &domain.SubagentResult{
		SubagentID: subagentID,
		Status:     domain.SubagentStatusRunning,
	}, nil
}

func (m *mockLifecycleManager) ListRunning(ctx context.Context) []string {
	var running []string
	for id := range m.statusResults {
		running = append(running, id)
	}
	return running
}

func (m *mockLifecycleManager) SetMonitoringHook(hook func(string, domain.SubagentStatus)) {}

func (m *mockLifecycleManager) Shutdown(ctx context.Context) error {
	return nil
}

// TestSkillCommand_Execute_ForkContext tests executing a skill with context: fork
func TestSkillCommand_Execute_ForkContext(t *testing.T) {
	registry := &mockSkillRegistry{
		skills: map[string]*domain.Skill{
			"fork-skill": {
				Name:        "fork-skill",
				Description: "A skill that forks",
				Frontmatter: domain.SkillFrontmatter{
					Name:        "fork-skill",
					Description: "Test",
					Context:     "fork",
				},
				BodyMD: "Execute this in a subagent",
			},
		},
	}

	renderer := &mockSkillRenderer{
		renderResults: map[string]*domain.RenderedSkill{
			"fork-skill": {
				Prompt:       "Execute this in a subagent",
				AllowedTools: []string{"read", "write"},
			},
		},
	}

	lifecycle := &mockLifecycleManager{
		statusResults: make(map[string]*domain.SubagentResult),
	}

	output := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, output)
	cmd.SetLifecycleManager(lifecycle)

	ctx := context.Background()
	result, err := cmd.Execute(ctx, "fork-skill", []string{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// For fork skills, result should indicate background execution
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Verify subagent was started
	if len(lifecycle.started) != 1 {
		t.Fatalf("Expected 1 subagent started, got %d", len(lifecycle.started))
	}

	started := lifecycle.started[0]
	if started.SkillName != "fork-skill" {
		t.Errorf("Started skill = %v, want fork-skill", started.SkillName)
	}

	if len(started.AllowedTools) != 2 {
		t.Errorf("AllowedTools count = %d, want 2", len(started.AllowedTools))
	}
}

// TestSkillCommand_Execute_InlineContext tests normal inline execution
func TestSkillCommand_Execute_InlineContext(t *testing.T) {
	registry := &mockSkillRegistry{
		skills: map[string]*domain.Skill{
			"inline-skill": {
				Name:        "inline-skill",
				Description: "A normal skill",
				Frontmatter: domain.SkillFrontmatter{
					Name:        "inline-skill",
					Description: "Test",
					// Context not set, defaults to inline
				},
				BodyMD: "Execute this inline",
			},
		},
	}

	renderer := &mockSkillRenderer{
		renderResults: map[string]*domain.RenderedSkill{
			"inline-skill": {
				Prompt:       "Execute this inline",
				AllowedTools: []string{},
			},
		},
	}

	lifecycle := &mockLifecycleManager{
		statusResults: make(map[string]*domain.SubagentResult),
	}

	output := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, output)
	cmd.SetLifecycleManager(lifecycle)

	ctx := context.Background()
	result, err := cmd.Execute(ctx, "inline-skill", []string{})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Should NOT start a subagent
	if len(lifecycle.started) != 0 {
		t.Errorf("Expected 0 subagents started for inline skill, got %d", len(lifecycle.started))
	}

	// Should return rendered skill
	if result.Prompt != "Execute this inline" {
		t.Errorf("Prompt = %v, want 'Execute this inline'", result.Prompt)
	}
}

// TestSkillCommand_GetSubagentStatus tests getting subagent status
func TestSkillCommand_GetSubagentStatus(t *testing.T) {
	lifecycle := &mockLifecycleManager{
		statusResults: map[string]*domain.SubagentResult{
			"test-subagent": {
				SubagentID: "test-subagent",
				Status:     domain.SubagentStatusComplete,
				Output:     "Done!",
			},
		},
	}

	output := &bytes.Buffer{}
	cmd := NewSkillCommand(nil, nil, output)
	cmd.SetLifecycleManager(lifecycle)

	ctx := context.Background()
	status, err := cmd.GetSubagentStatus(ctx, "test-subagent")

	if err != nil {
		t.Fatalf("GetSubagentStatus() error = %v", err)
	}

	if status.Status != domain.SubagentStatusComplete {
		t.Errorf("Status = %v, want complete", status.Status)
	}

	if status.Output != "Done!" {
		t.Errorf("Output = %v, want 'Done!'", status.Output)
	}
}

// TestSkillCommand_ListRunningSubagents tests listing running subagents
func TestSkillCommand_ListRunningSubagents(t *testing.T) {
	lifecycle := &mockLifecycleManager{
		statusResults: map[string]*domain.SubagentResult{
			"subagent-1": {SubagentID: "subagent-1", Status: domain.SubagentStatusRunning},
			"subagent-2": {SubagentID: "subagent-2", Status: domain.SubagentStatusRunning},
		},
	}

	output := &bytes.Buffer{}
	cmd := NewSkillCommand(nil, nil, output)
	cmd.SetLifecycleManager(lifecycle)

	ctx := context.Background()
	err := cmd.ListRunningSubagents(ctx)

	if err != nil {
		t.Fatalf("ListRunningSubagents() error = %v", err)
	}

	// Check output contains subagent IDs
	outputStr := output.String()
	if !strings.Contains(outputStr, "subagent-1") || !strings.Contains(outputStr, "subagent-2") {
		t.Errorf("Output missing subagent IDs, got: %s", outputStr)
	}
}

// Mock implementations for existing interfaces

type mockSkillRegistry struct {
	skills map[string]*domain.Skill
}

func (m *mockSkillRegistry) Get(name string) (*domain.Skill, error) {
	if skill, ok := m.skills[name]; ok {
		return skill, nil
	}
	return nil, errors.New("skill not found")
}

func (m *mockSkillRegistry) UserInvocableCatalog() []domain.SkillCatalogEntry {
	return []domain.SkillCatalogEntry{}
}

type mockSkillRenderer struct {
	renderResults map[string]*domain.RenderedSkill
}

func (m *mockSkillRenderer) Render(skill *domain.Skill, args []string) (*domain.RenderedSkill, error) {
	if result, ok := m.renderResults[skill.Name]; ok {
		return result, nil
	}
	return &domain.RenderedSkill{
		Prompt:       skill.BodyMD,
		AllowedTools: skill.Frontmatter.AllowedTools,
	}, nil
}
