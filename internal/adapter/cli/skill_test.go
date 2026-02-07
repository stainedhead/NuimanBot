package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// MockSkillRegistry is a mock implementation of SkillRegistry for testing
type MockSkillRegistry struct {
	skills map[string]*domain.Skill
}

func NewMockSkillRegistry() *MockSkillRegistry {
	return &MockSkillRegistry{
		skills: make(map[string]*domain.Skill),
	}
}

func (m *MockSkillRegistry) Register(skill domain.Skill) error {
	m.skills[skill.Name] = &skill
	return nil
}

func (m *MockSkillRegistry) RegisterMany(skills []domain.Skill) error {
	for _, skill := range skills {
		m.skills[skill.Name] = &skill
	}
	return nil
}

func (m *MockSkillRegistry) Get(name string) (*domain.Skill, error) {
	skill, exists := m.skills[name]
	if !exists {
		return nil, domain.ErrSkillNotFound{SkillName: name}
	}
	return skill, nil
}

func (m *MockSkillRegistry) List() []domain.Skill {
	skills := make([]domain.Skill, 0, len(m.skills))
	for _, skill := range m.skills {
		skills = append(skills, *skill)
	}
	return skills
}

func (m *MockSkillRegistry) Catalog() []domain.SkillCatalogEntry {
	catalog := make([]domain.SkillCatalogEntry, 0)
	for _, skill := range m.skills {
		catalog = append(catalog, domain.SkillCatalogEntry{
			Name:        skill.Name,
			Description: skill.Description,
			Scope:       skill.Scope,
			Priority:    skill.Priority,
		})
	}
	return catalog
}

func (m *MockSkillRegistry) UserInvocableCatalog() []domain.SkillCatalogEntry {
	catalog := make([]domain.SkillCatalogEntry, 0)
	for _, skill := range m.skills {
		if skill.CanBeInvokedByUser() {
			catalog = append(catalog, domain.SkillCatalogEntry{
				Name:        skill.Name,
				Description: skill.Description,
				Scope:       skill.Scope,
				Priority:    skill.Priority,
			})
		}
	}
	return catalog
}

func (m *MockSkillRegistry) ModelInvocableCatalog() []domain.SkillCatalogEntry {
	catalog := make([]domain.SkillCatalogEntry, 0)
	for _, skill := range m.skills {
		if skill.CanBeSelectedByModel() {
			catalog = append(catalog, domain.SkillCatalogEntry{
				Name:        skill.Name,
				Description: skill.Description,
				Scope:       skill.Scope,
				Priority:    skill.Priority,
			})
		}
	}
	return catalog
}

func (m *MockSkillRegistry) Initialize(ctx context.Context, roots []domain.SkillRoot) error {
	return nil
}

func (m *MockSkillRegistry) Reload(ctx context.Context, roots []domain.SkillRoot) error {
	return nil
}

// MockSkillRenderer is a mock implementation of SkillRenderer for testing
type MockSkillRenderer struct {
	renderErr error
}

func NewMockSkillRenderer() *MockSkillRenderer {
	return &MockSkillRenderer{}
}

func (m *MockSkillRenderer) Render(skill *domain.Skill, args []string) (*domain.RenderedSkill, error) {
	if m.renderErr != nil {
		return nil, m.renderErr
	}
	return &domain.RenderedSkill{
		SkillName:    skill.Name,
		Prompt:       skill.BodyMD,
		AllowedTools: skill.AllowedTools(),
	}, nil
}

func (m *MockSkillRenderer) SubstituteArguments(body string, args []string) string {
	return body
}

// TestExecute_ValidSkill tests executing a valid user-invocable skill
func TestExecute_ValidSkill(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	// Create a user-invocable skill
	userInvocable := true
	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		BodyMD:      "Test prompt",
		Frontmatter: domain.SkillFrontmatter{
			Name:          "test-skill",
			Description:   "A test skill",
			UserInvocable: &userInvocable,
		},
	}
	registry.Register(skill)

	// Execute skill
	rendered, err := cmd.Execute(context.Background(), "test-skill", []string{"arg1", "arg2"})

	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if rendered == nil {
		t.Fatal("Execute() returned nil rendered skill")
	}

	if rendered.SkillName != "test-skill" {
		t.Errorf("Expected skill name 'test-skill', got '%s'", rendered.SkillName)
	}
}

// TestExecute_SkillNotFound tests executing a non-existent skill
func TestExecute_SkillNotFound(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	_, err := cmd.Execute(context.Background(), "nonexistent", []string{})

	if err == nil {
		t.Fatal("Execute() should return error for non-existent skill")
	}

	if !errors.Is(err, domain.ErrSkillNotFound{}) {
		var notFoundErr domain.ErrSkillNotFound
		if !errors.As(err, &notFoundErr) {
			t.Errorf("Expected ErrSkillNotFound, got: %v", err)
		}
	}
}

// TestExecute_NotUserInvocable tests executing a skill that is not user-invocable
func TestExecute_NotUserInvocable(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	// Create a non-user-invocable skill
	userInvocable := false
	skill := domain.Skill{
		Name:        "model-only",
		Description: "Model-only skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		BodyMD:      "Test prompt",
		Frontmatter: domain.SkillFrontmatter{
			Name:          "model-only",
			Description:   "Model-only skill",
			UserInvocable: &userInvocable,
		},
	}
	registry.Register(skill)

	_, err := cmd.Execute(context.Background(), "model-only", []string{})

	if err == nil {
		t.Fatal("Execute() should return error for non-user-invocable skill")
	}
}

// TestList_WithSkills tests listing user-invocable skills
func TestList_WithSkills(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	// Add user-invocable skills
	userInvocable := true
	skill1 := domain.Skill{
		Name:        "skill-one",
		Description: "First skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		Frontmatter: domain.SkillFrontmatter{
			Name:          "skill-one",
			Description:   "First skill",
			UserInvocable: &userInvocable,
		},
	}
	skill2 := domain.Skill{
		Name:        "skill-two",
		Description: "Second skill",
		Scope:       domain.ScopeUser,
		Priority:    200,
		Frontmatter: domain.SkillFrontmatter{
			Name:          "skill-two",
			Description:   "Second skill",
			UserInvocable: &userInvocable,
		},
	}
	registry.Register(skill1)
	registry.Register(skill2)

	err := cmd.List(context.Background())

	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	output := buf.String()
	if !contains(output, "skill-one") {
		t.Errorf("List output should contain 'skill-one', got: %s", output)
	}
	if !contains(output, "skill-two") {
		t.Errorf("List output should contain 'skill-two', got: %s", output)
	}
	if !contains(output, "First skill") {
		t.Errorf("List output should contain 'First skill', got: %s", output)
	}
}

// TestList_NoSkills tests listing when no user-invocable skills exist
func TestList_NoSkills(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	err := cmd.List(context.Background())

	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}

	output := buf.String()
	if !contains(output, "No user-invocable skills found") {
		t.Errorf("List output should indicate no skills found, got: %s", output)
	}
}

// TestDescribe_ValidSkill tests describing a valid skill
func TestDescribe_ValidSkill(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	userInvocable := true
	skill := domain.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Scope:       domain.ScopeProject,
		Priority:    100,
		BodyMD:      "Test prompt body",
		Frontmatter: domain.SkillFrontmatter{
			Name:          "test-skill",
			Description:   "A test skill",
			UserInvocable: &userInvocable,
			AllowedTools:  []string{"tool1", "tool2"},
		},
	}
	registry.Register(skill)

	err := cmd.Describe(context.Background(), "test-skill")

	if err != nil {
		t.Fatalf("Describe() returned error: %v", err)
	}

	output := buf.String()
	if !contains(output, "test-skill") {
		t.Errorf("Describe output should contain skill name, got: %s", output)
	}
	if !contains(output, "A test skill") {
		t.Errorf("Describe output should contain description, got: %s", output)
	}
	if !contains(output, "Test prompt body") {
		t.Errorf("Describe output should contain body, got: %s", output)
	}
}

// TestDescribe_SkillNotFound tests describing a non-existent skill
func TestDescribe_SkillNotFound(t *testing.T) {
	registry := NewMockSkillRegistry()
	renderer := NewMockSkillRenderer()
	buf := &bytes.Buffer{}
	cmd := NewSkillCommand(registry, renderer, buf)

	err := cmd.Describe(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("Describe() should return error for non-existent skill")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
