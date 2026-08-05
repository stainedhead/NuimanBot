package domain

import "context"

// ProjectRepository defines persistence operations for Project entities.
//
// Unlike ConversationRepository (which resolves GetConversation/
// DeleteConversation by ID alone, relying on the usecase layer to check
// ownership), Get/Delete here take ownerUserID explicitly and MUST behave
// as ErrNotFound — never ErrForbidden — when projectID exists but is owned
// by a different user. This is a deliberate strengthening for this feature:
// spec.md's Security NFR requires per-user isolation "enforced at the
// data-access layer, not just hidden in the UI", and Edge Case #10 requires
// cross-user access by ID to return 404 (existence never disclosed, even to
// admins). See implementation-notes.md for the rationale.
type ProjectRepository interface {
	// SaveProject creates or updates a Project.
	SaveProject(ctx context.Context, p *Project) error

	// GetProject retrieves a Project by ID, scoped to its owner.
	// Returns ErrNotFound if no such Project exists for ownerUserID
	// (including when projectID exists but belongs to a different user).
	GetProject(ctx context.Context, ownerUserID, projectID string) (*Project, error)

	// ListProjects returns all Projects owned by ownerUserID.
	// Returns an empty slice (never nil) if none exist.
	ListProjects(ctx context.Context, ownerUserID string) ([]*Project, error)

	// DeleteProject removes a Project by ID, scoped to its owner.
	// Returns ErrNotFound if no such Project exists for ownerUserID.
	DeleteProject(ctx context.Context, ownerUserID, projectID string) error
}
