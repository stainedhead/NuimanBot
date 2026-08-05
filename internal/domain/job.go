package domain

import "time"

// JobStatus is the lifecycle state of a Job (or, shared with Chore runs, a
// Run — see RunStatus). Defined once here and reused by RunStatus to avoid
// two parallel enumerations drifting apart; see run.go.
type JobStatus string

const (
	// JobStatusQueued means the Job is waiting in the FIFO queue for a
	// worker (FR-027).
	JobStatusQueued JobStatus = "queued"
	// JobStatusRunning means a worker is actively executing the Job.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted means the Job's most recent run finished
	// successfully.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed means the Job's most recent run ended in failure
	// (including a worker crash — recorded via the run's Error field).
	JobStatusFailed JobStatus = "failed"
)

// ContextType identifies what a Job (or Chore) runs in the context of
// (FR-026).
type ContextType string

const (
	// ContextTypeChat means the Job/Chore runs in the context of a Chat;
	// no working directory is implied.
	ContextTypeChat ContextType = "chat"
	// ContextTypeProject means the Job/Chore runs in the context of a
	// Project; that Project's OutputDirectory is the default working
	// directory (FR-026).
	ContextTypeProject ContextType = "project"
)

// Job is a one-time agent task, queued and executed via the shared worker
// pool (FR-024–FR-030).
type Job struct {
	// ID is the unique identifier (UUID).
	ID string
	// OwnerUserID is the owning user (FR-009/FR-010).
	OwnerUserID string
	// Title is the user-supplied Job title.
	Title string
	// Description is the Job's task description, also persisted as
	// JOB-DESCRIPTION.md in HiddenDirectory (FR-025).
	Description string
	// HiddenDirectory holds JOB-DESCRIPTION.md and other agent-managed
	// files for this Job.
	HiddenDirectory string
	// ContextType and ContextID identify what this Job runs against
	// (FR-026): a Chat (ContextTypeChat) or a Project (ContextTypeProject).
	ContextType ContextType
	ContextID   string
	// WorkingDirectory is the filesystem directory the run executes
	// against. Defaults to the referenced Project's OutputDirectory when
	// ContextType == ContextTypeProject (FR-026); empty for
	// ContextTypeChat, since Chats expose no working directory.
	WorkingDirectory string
	// Status is the Job's current lifecycle state, mirroring its most
	// recent Run's status.
	Status JobStatus
	// PendingDeletion marks a Job soft-deleted while a run is still active
	// (spec.md Edge Case #3): the record is hidden from creation of new
	// runs but its current in-flight run is allowed to finish before the
	// record is actually removed.
	PendingDeletion bool
	// CreatedAt is when the Job was created.
	CreatedAt time.Time
	// UpdatedAt is the Job's last-modification time.
	UpdatedAt time.Time
}
