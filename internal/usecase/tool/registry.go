package tool

import (
	"context"
	"nuimanbot/internal/domain"
)

// ToolRegistry defines the interface for managing tools (discovery, registration, retrieval).
type ToolRegistry interface {
	// Register adds a tool to the registry.
	Register(tool domain.Tool) error

	// Get retrieves a tool by its unique name.
	Get(name string) (domain.Tool, error)

	// List returns all registered tools.
	List() []domain.Tool

	// ListForUser returns tools available to a specific user, considering their permissions/allowlist.
	ListForUser(ctx context.Context, userID string) ([]domain.Tool, error)
}
