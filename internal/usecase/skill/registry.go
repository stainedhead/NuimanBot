package skill

import (
	"context"
	"nuimanbot/internal/domain"
)

// SkillRegistry defines the interface for managing skills (discovery, registration, retrieval).
type SkillRegistry interface {
	// Register adds a skill to the registry.
	Register(skill domain.Skill) error

	// Get retrieves a skill by its unique name.
	Get(name string) (domain.Skill, error)

	// List returns all registered skills.
	List() []domain.Skill

	// ListForUser returns skills available to a specific user, considering their permissions/allowlist.
	ListForUser(ctx context.Context, userID string) ([]domain.Skill, error)
}
