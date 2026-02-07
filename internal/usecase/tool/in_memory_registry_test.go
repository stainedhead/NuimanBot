package tool

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

// mockTool implements domain.Tool for testing
type mockTool struct {
	name        string
	description string
	permissions []domain.Permission
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{"type": "string"},
		},
	}
}

func (m *mockTool) RequiredPermissions() []domain.Permission {
	return m.permissions
}

func (m *mockTool) Config() domain.ToolConfig {
	return domain.ToolConfig{
		Enabled: true,
	}
}

func (m *mockTool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	return &domain.ExecutionResult{
		Output: "mock output",
	}, nil
}

// TestNewInMemoryRegistry tests registry creation
func TestNewInMemoryRegistry(t *testing.T) {
	registry := NewInMemoryRegistry()
	if registry == nil {
		t.Fatal("NewInMemoryRegistry() returned nil")
	}
}

// TestRegister tests skill registration
func TestRegister(t *testing.T) {
	registry := NewInMemoryRegistry()

	skill := &mockTool{
		name:        "test_skill",
		description: "Test skill",
		permissions: []domain.Permission{},
	}

	err := registry.Register(skill)
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}

	// Verify skill was registered
	retrieved, err := registry.Get("test_skill")
	if err != nil {
		t.Fatalf("Get() returned error after registration: %v", err)
	}
	if retrieved.Name() != skill.Name() {
		t.Error("Get() returned different skill than registered")
	}
}

// TestRegister_DuplicateName tests registering skill with duplicate name
func TestRegister_DuplicateName(t *testing.T) {
	registry := NewInMemoryRegistry()

	skill1 := &mockTool{name: "duplicate", description: "First"}
	skill2 := &mockTool{name: "duplicate", description: "Second"}

	err := registry.Register(skill1)
	if err != nil {
		t.Fatalf("First Register() returned error: %v", err)
	}

	err = registry.Register(skill2)
	if err == nil {
		t.Error("Register() should error for duplicate skill name")
	}
}

// TestRegister_NilSkill tests registering nil skill
func TestRegister_NilSkill(t *testing.T) {
	registry := NewInMemoryRegistry()

	err := registry.Register(nil)
	if err == nil {
		t.Error("Register() should error for nil skill")
	}
}

// TestGet tests retrieving registered skills
func TestGet(t *testing.T) {
	registry := NewInMemoryRegistry()

	skill := &mockTool{name: "test_skill", description: "Test"}
	registry.Register(skill)

	// Test successful retrieval
	retrieved, err := registry.Get("test_skill")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if retrieved.Name() != "test_skill" {
		t.Errorf("Get() returned skill with name %s, want test_skill", retrieved.Name())
	}
}

// TestGet_NotFound tests retrieving non-existent skill
func TestGet_NotFound(t *testing.T) {
	registry := NewInMemoryRegistry()

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Error("Get() should error for non-existent skill")
	}

	expectedMsg := "tool not found: nonexistent"
	if err.Error() != expectedMsg {
		t.Errorf("Get() error = %v, want %v", err.Error(), expectedMsg)
	}
}

// TestList tests listing all registered skills
func TestList(t *testing.T) {
	registry := NewInMemoryRegistry()

	// Register multiple skills
	skills := []*mockTool{
		{name: "skill1", description: "First skill"},
		{name: "skill2", description: "Second skill"},
		{name: "skill3", description: "Third skill"},
	}

	for _, skill := range skills {
		registry.Register(skill)
	}

	// List all skills
	listed := registry.List()
	if len(listed) != 3 {
		t.Fatalf("List() returned %d skills, want 3", len(listed))
	}

	// Verify all skills are present
	skillNames := make(map[string]bool)
	for _, skill := range listed {
		skillNames[skill.Name()] = true
	}

	for _, expected := range skills {
		if !skillNames[expected.name] {
			t.Errorf("List() missing skill %s", expected.name)
		}
	}
}

// TestList_Empty tests listing with no registered skills
func TestList_Empty(t *testing.T) {
	registry := NewInMemoryRegistry()

	listed := registry.List()
	if len(listed) != 0 {
		t.Errorf("List() returned %d skills for empty registry, want 0", len(listed))
	}
}

// TestListForUser tests listing skills for a specific user
func TestListForUser(t *testing.T) {
	registry := NewInMemoryRegistry()
	ctx := context.Background()

	// Register multiple skills
	skill1 := &mockTool{name: "skill1", description: "First"}
	skill2 := &mockTool{name: "skill2", description: "Second"}

	registry.Register(skill1)
	registry.Register(skill2)

	// ListForUser currently returns all skills (permission filtering is done at service layer)
	listed, err := registry.ListForUser(ctx, "user1")
	if err != nil {
		t.Fatalf("ListForUser() returned error: %v", err)
	}

	if len(listed) != 2 {
		t.Errorf("ListForUser() returned %d skills, want 2", len(listed))
	}
}

// TestListForUser_Empty tests listing for user when no skills registered
func TestListForUser_Empty(t *testing.T) {
	registry := NewInMemoryRegistry()
	ctx := context.Background()

	listed, err := registry.ListForUser(ctx, "user1")
	if err != nil {
		t.Fatalf("ListForUser() returned error: %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("ListForUser() returned %d skills for empty registry, want 0", len(listed))
	}
}

// TestRegisterMultipleSkills tests registering and retrieving multiple skills
func TestRegisterMultipleSkills(t *testing.T) {
	registry := NewInMemoryRegistry()

	skills := []string{"calculator", "weather", "search", "notes", "datetime"}

	for _, name := range skills {
		skill := &mockTool{name: name, description: name + " skill"}
		err := registry.Register(skill)
		if err != nil {
			t.Fatalf("Failed to register skill %s: %v", name, err)
		}
	}

	// Verify all can be retrieved
	for _, name := range skills {
		skill, err := registry.Get(name)
		if err != nil {
			t.Errorf("Failed to get skill %s: %v", name, err)
		}
		if skill.Name() != name {
			t.Errorf("Get(%s) returned skill with name %s", name, skill.Name())
		}
	}

	// Verify List returns all
	listed := registry.List()
	if len(listed) != len(skills) {
		t.Errorf("List() returned %d skills, want %d", len(listed), len(skills))
	}
}
