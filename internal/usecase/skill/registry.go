package skill

import (
	"context"
	"fmt"
	"sync"

	"nuimanbot/internal/domain"
)

// SkillRegistry manages registered skills with priority-based resolution.
// It provides catalog views filtered by invocation permissions.
type SkillRegistry interface {
	// Register adds a skill to the registry
	Register(skill domain.Skill) error

	// RegisterMany adds multiple skills (batch operation)
	RegisterMany(skills []domain.Skill) error

	// Get retrieves a skill by name (priority resolution if conflicts)
	Get(name string) (*domain.Skill, error)

	// List returns all registered skills (highest priority per name)
	List() []domain.Skill

	// Catalog returns lightweight catalog entries for all skills
	Catalog() []domain.SkillCatalogEntry

	// UserInvocableCatalog returns catalog filtered for user-invocable skills
	UserInvocableCatalog() []domain.SkillCatalogEntry

	// ModelInvocableCatalog returns catalog filtered for model-invocable skills
	ModelInvocableCatalog() []domain.SkillCatalogEntry

	// Initialize scans and registers skills from repository
	Initialize(ctx context.Context, roots []domain.SkillRoot) error

	// Reload clears and re-scans all skills
	Reload(ctx context.Context, roots []domain.SkillRoot) error
}

// InMemorySkillRegistry is an in-memory implementation of SkillRegistry.
// It is thread-safe and uses priority-based conflict resolution.
//
// When multiple skills have the same name, the skill with highest priority wins.
// Priority order: Enterprise (300) > User (200) > Project (100) > Plugin (50)
type InMemorySkillRegistry struct {
	skills     map[string]domain.Skill // name -> skill (highest priority wins)
	allSkills  []domain.Skill          // all skills including conflicts
	repository domain.SkillRepository
	mu         sync.RWMutex
}

// NewInMemorySkillRegistry creates a new in-memory skill registry.
func NewInMemorySkillRegistry(repo domain.SkillRepository) *InMemorySkillRegistry {
	return &InMemorySkillRegistry{
		skills:     make(map[string]domain.Skill),
		allSkills:  make([]domain.Skill, 0),
		repository: repo,
	}
}

// Register adds a skill to the registry.
// If a skill with the same name already exists, priority determines which is kept.
// Higher priority skills replace lower priority ones.
func (r *InMemorySkillRegistry) Register(skill domain.Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := skill.Validate(); err != nil {
		return err
	}

	// Check if skill already exists
	if existing, exists := r.skills[skill.Name]; exists {
		// Priority resolution: higher priority wins
		if skill.Priority > existing.Priority {
			r.skills[skill.Name] = skill
		}
		// Keep track of conflict (add to allSkills regardless)
	} else {
		r.skills[skill.Name] = skill
	}

	r.allSkills = append(r.allSkills, skill)

	return nil
}

// RegisterMany adds multiple skills in a batch operation.
// If any skill fails validation, the operation stops and returns an error.
// Skills successfully registered before the error remain in the registry.
func (r *InMemorySkillRegistry) RegisterMany(skills []domain.Skill) error {
	for _, skill := range skills {
		if err := r.Register(skill); err != nil {
			return fmt.Errorf("failed to register skill %s: %w", skill.Name, err)
		}
	}
	return nil
}

// Get retrieves a skill by name.
// If multiple skills with the same name exist, returns the highest priority one.
func (r *InMemorySkillRegistry) Get(name string) (*domain.Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[name]
	if !exists {
		return nil, domain.ErrSkillNotFound{SkillName: name}
	}

	return &skill, nil
}

// List returns all registered skills (highest priority per name).
func (r *InMemorySkillRegistry) List() []domain.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]domain.Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// Catalog returns lightweight catalog entries for all registered skills.
func (r *InMemorySkillRegistry) Catalog() []domain.SkillCatalogEntry {
	skills := r.List()
	catalog := make([]domain.SkillCatalogEntry, len(skills))
	for i, skill := range skills {
		catalog[i] = domain.SkillCatalogEntry{
			Name:        skill.Name,
			Description: skill.Description,
			Scope:       skill.Scope,
			Priority:    skill.Priority,
		}
	}
	return catalog
}

// UserInvocableCatalog returns catalog entries filtered for user-invocable skills.
// These are skills that can be invoked directly by users via /skill-name.
func (r *InMemorySkillRegistry) UserInvocableCatalog() []domain.SkillCatalogEntry {
	skills := r.List()
	catalog := make([]domain.SkillCatalogEntry, 0)
	for _, skill := range skills {
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

// ModelInvocableCatalog returns catalog entries filtered for model-invocable skills.
// These are skills that the LLM can automatically select and use.
func (r *InMemorySkillRegistry) ModelInvocableCatalog() []domain.SkillCatalogEntry {
	skills := r.List()
	catalog := make([]domain.SkillCatalogEntry, 0)
	for _, skill := range skills {
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

// Initialize scans and registers skills from the repository.
// This is typically called once at application startup.
func (r *InMemorySkillRegistry) Initialize(ctx context.Context, roots []domain.SkillRoot) error {
	skills, err := r.repository.Scan(roots)
	if err != nil {
		return fmt.Errorf("failed to scan skills: %w", err)
	}

	return r.RegisterMany(skills)
}

// Reload clears all registered skills and re-scans from the repository.
// This can be used to pick up changes to skill files without restarting the application.
func (r *InMemorySkillRegistry) Reload(ctx context.Context, roots []domain.SkillRoot) error {
	r.mu.Lock()
	r.skills = make(map[string]domain.Skill)
	r.allSkills = make([]domain.Skill, 0)
	r.mu.Unlock()

	return r.Initialize(ctx, roots)
}
