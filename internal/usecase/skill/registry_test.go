package skill

import (
	"context"
	"sync"
	"testing"

	"nuimanbot/internal/domain"
)

// mockSkillRepository is a mock implementation for testing
type mockSkillRepository struct {
	skills []domain.Skill
	err    error
}

func (m *mockSkillRepository) Scan(roots []domain.SkillRoot) ([]domain.Skill, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.skills, nil
}

func (m *mockSkillRepository) Load(skillPath string) (*domain.Skill, error) {
	for _, skill := range m.skills {
		if skill.FilePath == skillPath {
			return &skill, nil
		}
	}
	return nil, domain.ErrSkillNotFound{SkillName: skillPath}
}

func TestRegister_ValidSkill(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "A test skill",
		},
	}

	err := registry.Register(skill)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Verify skill was registered
	retrieved, err := registry.Get("test-skill")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.Name != "test-skill" {
		t.Errorf("Retrieved skill name = %q, want %q", retrieved.Name, "test-skill")
	}
}

func TestRegister_InvalidSkill(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skill := domain.Skill{
		Name: "", // Invalid: empty name
		Frontmatter: domain.SkillFrontmatter{
			Name: "",
		},
	}

	err := registry.Register(skill)
	if err == nil {
		t.Fatal("Register() should fail for invalid skill, got nil error")
	}
}

func TestRegister_Conflict_HigherPriorityWins(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	// Register project-scoped skill (priority 100)
	skill1 := domain.Skill{
		Name:        "test-skill",
		Description: "Project skill",
		Scope:       domain.ScopeProject,
		Priority:    domain.ScopeProject.Priority(),
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "Project skill",
		},
	}

	err := registry.Register(skill1)
	if err != nil {
		t.Fatalf("Register() failed for skill1: %v", err)
	}

	// Register user-scoped skill with same name (priority 200 - higher)
	skill2 := domain.Skill{
		Name:        "test-skill",
		Description: "User skill",
		Scope:       domain.ScopeUser,
		Priority:    domain.ScopeUser.Priority(),
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "User skill",
		},
	}

	err = registry.Register(skill2)
	if err != nil {
		t.Fatalf("Register() failed for skill2: %v", err)
	}

	// Get should return the higher priority skill (user)
	retrieved, err := registry.Get("test-skill")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.Scope != domain.ScopeUser {
		t.Errorf("Retrieved skill scope = %v, want %v (higher priority should win)", retrieved.Scope, domain.ScopeUser)
	}

	if retrieved.Description != "User skill" {
		t.Errorf("Retrieved skill description = %q, want %q", retrieved.Description, "User skill")
	}
}

func TestRegisterMany_Success(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skills := []domain.Skill{
		{
			Name:        "skill-one",
			Description: "First skill",
			Scope:       domain.ScopeProject,
			Priority:    100,
			Frontmatter: domain.SkillFrontmatter{
				Name:        "skill-one",
				Description: "First skill",
			},
		},
		{
			Name:        "skill-two",
			Description: "Second skill",
			Scope:       domain.ScopeProject,
			Priority:    100,
			Frontmatter: domain.SkillFrontmatter{
				Name:        "skill-two",
				Description: "Second skill",
			},
		},
	}

	err := registry.RegisterMany(skills)
	if err != nil {
		t.Fatalf("RegisterMany() failed: %v", err)
	}

	// Verify both skills were registered
	list := registry.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d skills, want 2", len(list))
	}
}

func TestRegisterMany_PartialFailure(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skills := []domain.Skill{
		{
			Name:        "valid-skill",
			Description: "A valid skill",
			Scope:       domain.ScopeProject,
			Priority:    100,
			Frontmatter: domain.SkillFrontmatter{
				Name:        "valid-skill",
				Description: "A valid skill",
			},
		},
		{
			Name: "", // Invalid skill
			Frontmatter: domain.SkillFrontmatter{
				Name: "",
			},
		},
	}

	err := registry.RegisterMany(skills)
	if err == nil {
		t.Fatal("RegisterMany() should fail when one skill is invalid, got nil error")
	}

	// First skill should have been registered before failure
	_, err2 := registry.Get("valid-skill")
	if err2 != nil {
		t.Errorf("First skill should have been registered before failure: %v", err2)
	}
}

func TestGet_ExistingSkill(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "A test skill",
		},
	}

	_ = registry.Register(skill)

	retrieved, err := registry.Get("test-skill")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if retrieved.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "test-skill")
	}
}

func TestGet_NonExistentSkill(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() should return error for nonexistent skill, got nil")
	}

	if _, ok := err.(domain.ErrSkillNotFound); !ok {
		t.Errorf("Expected ErrSkillNotFound, got %T", err)
	}
}

func TestList_AllSkills(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skill1 := domain.Skill{
		Name:        "skill-one",
		Description: "First",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "skill-one",
			Description: "First",
		},
	}

	skill2 := domain.Skill{
		Name:        "skill-two",
		Description: "Second",
		Scope:       domain.ScopeUser,
		Priority:    200,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "skill-two",
			Description: "Second",
		},
	}

	_ = registry.Register(skill1)
	_ = registry.Register(skill2)

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d skills, want 2", len(list))
	}
}

func TestCatalog_AllSkills(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "test-skill",
			Description: "A test skill",
		},
	}

	_ = registry.Register(skill)

	catalog := registry.Catalog()
	if len(catalog) != 1 {
		t.Fatalf("Catalog() returned %d entries, want 1", len(catalog))
	}

	entry := catalog[0]
	if entry.Name != "test-skill" {
		t.Errorf("Entry name = %q, want %q", entry.Name, "test-skill")
	}

	if entry.Description != "A test skill" {
		t.Errorf("Entry description = %q, want %q", entry.Description, "A test skill")
	}

	if entry.Scope != domain.ScopeProject {
		t.Errorf("Entry scope = %v, want %v", entry.Scope, domain.ScopeProject)
	}

	if entry.Priority != 100 {
		t.Errorf("Entry priority = %d, want 100", entry.Priority)
	}
}

func TestUserInvocableCatalog_Filtered(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	// User-invocable skill
	skill1 := domain.Skill{
		Name:        "user-skill",
		Description: "User invocable",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:          "user-skill",
			Description:   "User invocable",
			UserInvocable: boolPtr(true),
		},
	}

	// Non user-invocable skill
	skill2 := domain.Skill{
		Name:        "non-user-skill",
		Description: "Not user invocable",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:          "non-user-skill",
			Description:   "Not user invocable",
			UserInvocable: boolPtr(false),
		},
	}

	_ = registry.Register(skill1)
	_ = registry.Register(skill2)

	catalog := registry.UserInvocableCatalog()
	if len(catalog) != 1 {
		t.Errorf("UserInvocableCatalog() returned %d entries, want 1", len(catalog))
	}

	if len(catalog) > 0 && catalog[0].Name != "user-skill" {
		t.Errorf("Entry name = %q, want %q", catalog[0].Name, "user-skill")
	}
}

func TestModelInvocableCatalog_Filtered(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	// Model-invocable skill
	skill1 := domain.Skill{
		Name:        "model-skill",
		Description: "Model invocable",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:                   "model-skill",
			Description:            "Model invocable",
			DisableModelInvocation: false,
		},
	}

	// Non model-invocable skill
	skill2 := domain.Skill{
		Name:        "non-model-skill",
		Description: "Not model invocable",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:                   "non-model-skill",
			Description:            "Not model invocable",
			DisableModelInvocation: true,
		},
	}

	_ = registry.Register(skill1)
	_ = registry.Register(skill2)

	catalog := registry.ModelInvocableCatalog()
	if len(catalog) != 1 {
		t.Errorf("ModelInvocableCatalog() returned %d entries, want 1", len(catalog))
	}

	if len(catalog) > 0 && catalog[0].Name != "model-skill" {
		t.Errorf("Entry name = %q, want %q", catalog[0].Name, "model-skill")
	}
}

func TestInitialize_Success(t *testing.T) {
	mockSkills := []domain.Skill{
		{
			Name:        "skill-one",
			Description: "First",
			Scope:       domain.ScopeProject,
			Priority:    100,
			Frontmatter: domain.SkillFrontmatter{
				Name:        "skill-one",
				Description: "First",
			},
		},
	}

	repo := &mockSkillRepository{skills: mockSkills}
	registry := NewInMemorySkillRegistry(repo)

	roots := []domain.SkillRoot{
		{Path: "/path/to/skills", Scope: domain.ScopeProject},
	}

	err := registry.Initialize(context.Background(), roots)
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Verify skills were registered
	list := registry.List()
	if len(list) != 1 {
		t.Errorf("Initialize() registered %d skills, want 1", len(list))
	}
}

func TestReload_ClearsAndRescans(t *testing.T) {
	mockSkills := []domain.Skill{
		{
			Name:        "skill-one",
			Description: "First",
			Scope:       domain.ScopeProject,
			Priority:    100,
			Frontmatter: domain.SkillFrontmatter{
				Name:        "skill-one",
				Description: "First",
			},
		},
	}

	repo := &mockSkillRepository{skills: mockSkills}
	registry := NewInMemorySkillRegistry(repo)

	// Initial registration
	skill := domain.Skill{
		Name:        "old-skill",
		Description: "Old",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:        "old-skill",
			Description: "Old",
		},
	}
	_ = registry.Register(skill)

	// Reload should clear old skills and load new ones
	roots := []domain.SkillRoot{
		{Path: "/path/to/skills", Scope: domain.ScopeProject},
	}

	err := registry.Reload(context.Background(), roots)
	if err != nil {
		t.Fatalf("Reload() failed: %v", err)
	}

	// Old skill should be gone
	_, err = registry.Get("old-skill")
	if err == nil {
		t.Error("Reload() should have cleared old-skill")
	}

	// New skill should be present
	_, err = registry.Get("skill-one")
	if err != nil {
		t.Errorf("Reload() should have loaded skill-one: %v", err)
	}
}

func TestRegistry_ThreadSafety(t *testing.T) {
	repo := &mockSkillRepository{}
	registry := NewInMemorySkillRegistry(repo)

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			skill := domain.Skill{
				Name:        "skill",
				Description: "Test",
				Scope:       domain.ScopeProject,
				Priority:    n,
				Frontmatter: domain.SkillFrontmatter{
					Name:        "skill",
					Description: "Test",
				},
			}
			_ = registry.Register(skill)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.List()
			_, _ = registry.Get("skill")
		}()
	}

	wg.Wait()

	// Should not panic and should have registered the skill
	retrieved, err := registry.Get("skill")
	if err != nil {
		t.Errorf("Get() after concurrent operations failed: %v", err)
	}

	if retrieved.Name != "skill" {
		t.Error("Concurrent operations corrupted data")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
