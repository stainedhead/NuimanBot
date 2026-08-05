package domain

import (
	"context"
	"time"
)

// ChoreRepository defines persistence operations for Chore entities.
// Get/Delete take ownerUserID explicitly and behave as ErrNotFound (never
// ErrForbidden) for cross-owner access — see project_repository.go's doc
// comment for the full rationale.
type ChoreRepository interface {
	// SaveChore creates or updates a Chore.
	SaveChore(ctx context.Context, c *Chore) error

	// GetChore retrieves a Chore by ID, scoped to its owner.
	// Returns ErrNotFound if no such Chore exists for ownerUserID.
	GetChore(ctx context.Context, ownerUserID, choreID string) (*Chore, error)

	// ListChores returns all Chores owned by ownerUserID.
	// Returns an empty slice (never nil) if none exist.
	ListChores(ctx context.Context, ownerUserID string) ([]*Chore, error)

	// DeleteChore removes a Chore by ID, scoped to its owner.
	// Returns ErrNotFound if no such Chore exists for ownerUserID.
	DeleteChore(ctx context.Context, ownerUserID, choreID string) error

	// UpdateNextFireTime updates just the NextFireTime (and UpdatedAt) of
	// a Chore, scoped to its owner. Returns ErrNotFound if no such Chore
	// exists for ownerUserID.
	UpdateNextFireTime(ctx context.Context, ownerUserID, choreID string, next time.Time) error

	// ListAllDue returns every confirmed, non-pending-deletion Chore across
	// all users whose NextFireTime has arrived as of now. This is the one
	// intentionally cross-user query in this repository: the cron
	// evaluator is a system-wide process, not acting on behalf of a single
	// requesting user, so per-user scoping does not apply here. Per-user
	// isolation is still preserved because the caller (the scheduler) never
	// exposes this result to any individual user's UI/API surface — only
	// each due Chore's own OwnerUserID's Run is created.
	ListAllDue(ctx context.Context, now time.Time) ([]*Chore, error)
}
