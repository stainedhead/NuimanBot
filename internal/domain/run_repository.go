package domain

import "context"

// RunRepository defines persistence operations for Run entities. Get/List
// take ownerUserID explicitly and behave as ErrNotFound (never
// ErrForbidden) for cross-owner access — see project_repository.go's doc
// comment for the full rationale.
type RunRepository interface {
	// SaveRun creates or updates a Run.
	SaveRun(ctx context.Context, r *Run) error

	// GetRun retrieves a Run by ID, scoped to its owner.
	// Returns ErrNotFound if no such Run exists for ownerUserID.
	GetRun(ctx context.Context, ownerUserID, runID string) (*Run, error)

	// ListRuns returns Runs owned by ownerUserID matching filter, most
	// recent first. Returns an empty slice (never nil) if none match.
	ListRuns(ctx context.Context, ownerUserID string, filter RunFilter) ([]*Run, error)

	// AppendLog appends a chunk of log content to runID's durable log,
	// scoped to its owner. Returns ErrNotFound if no such Run exists for
	// ownerUserID.
	AppendLog(ctx context.Context, ownerUserID, runID string, chunk string) error

	// MarkNotified sets NotifiedAt on runID (FR-044's badge clear-on-view),
	// scoped to its owner. Returns ErrNotFound if no such Run exists for
	// ownerUserID.
	MarkNotified(ctx context.Context, ownerUserID, runID string) error

	// CountUnnotified returns the number of terminal, unviewed runs for
	// ownerUserID (the History notification badge count, FR-044).
	CountUnnotified(ctx context.Context, ownerUserID string) (int, error)

	// DeleteRun removes a Run by ID, scoped to its owner. Used by the
	// retention sweep (FR-043); per spec.md Edge Case #7, a caller sweeping
	// an unviewed run must not simply delete it out from under a still-live
	// badge count — see the usecase-layer retention sweep, which calls
	// MarkNotified before DeleteRun for any unviewed run being swept.
	DeleteRun(ctx context.Context, ownerUserID, runID string) error
}
