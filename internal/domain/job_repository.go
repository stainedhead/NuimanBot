package domain

import "context"

// JobRepository defines persistence operations for Job entities. As with
// ProjectRepository, Get/Delete take ownerUserID explicitly and behave as
// ErrNotFound (never ErrForbidden) for cross-owner access — see
// project_repository.go's doc comment for the full rationale.
type JobRepository interface {
	// SaveJob creates or updates a Job.
	SaveJob(ctx context.Context, j *Job) error

	// GetJob retrieves a Job by ID, scoped to its owner.
	// Returns ErrNotFound if no such Job exists for ownerUserID.
	GetJob(ctx context.Context, ownerUserID, jobID string) (*Job, error)

	// ListJobs returns all Jobs owned by ownerUserID.
	// Returns an empty slice (never nil) if none exist.
	ListJobs(ctx context.Context, ownerUserID string) ([]*Job, error)

	// DeleteJob removes a Job by ID, scoped to its owner.
	// Returns ErrNotFound if no such Job exists for ownerUserID.
	DeleteJob(ctx context.Context, ownerUserID, jobID string) error

	// UpdateStatus updates just the Status (and UpdatedAt) of a Job,
	// scoped to its owner. Returns ErrNotFound if no such Job exists for
	// ownerUserID.
	UpdateStatus(ctx context.Context, ownerUserID, jobID string, status JobStatus) error
}
