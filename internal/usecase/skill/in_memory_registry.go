package skill

import (
	"context"
	"fmt"
	"sync"

	"nuimanbot/internal/domain"
)

// InMemoryRegistry is a simple in-memory implementation of the SkillRegistry.
type InMemoryRegistry struct {
	mu     sync.RWMutex
	skills map[string]domain.Skill
}

// NewInMemoryRegistry creates a new InMemoryRegistry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		skills: make(map[string]domain.Skill),
	}
}

// Register adds a skill to the registry.
func (r *InMemoryRegistry) Register(skill domain.Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if skill == nil {
		return fmt.Errorf("cannot register nil skill")
	}

	if _, exists := r.skills[skill.Name()]; exists {
		return fmt.Errorf("skill with name '%s' already registered", skill.Name())
	}
	r.skills[skill.Name()] = skill
	return nil
}

// Get retrieves a skill by its unique name.
func (r *InMemoryRegistry) Get(name string) (domain.Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}
	return skill, nil
}

// List returns all registered skills.
func (r *InMemoryRegistry) List() []domain.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skills := make([]domain.Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		skills = append(skills, skill)
	}
	return skills
}

// ListForUser returns skills available to a specific user.
// For this simple implementation, it returns all skills.
func (r *InMemoryRegistry) ListForUser(ctx context.Context, userID string) ([]domain.Skill, error) {
	return r.List(), nil
}
