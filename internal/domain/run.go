package domain

import "time"

// RunStatus is the lifecycle state of a single Job or Chore execution.
// Deliberately has no "Cancelled" state — no FR describes user-initiated
// cancellation of an in-flight run (spec.md Non-Goals) — and reuses
// JobStatus's Queued/Running/Completed/Failed values plus one Chore-only
// addition, RunStatusSkipped (FR-035).
type RunStatus string

const (
	RunStatusQueued    RunStatus = RunStatus(JobStatusQueued)
	RunStatusRunning   RunStatus = RunStatus(JobStatusRunning)
	RunStatusCompleted RunStatus = RunStatus(JobStatusCompleted)
	RunStatusFailed    RunStatus = RunStatus(JobStatusFailed)
	// RunStatusSkipped marks a Chore run that was skipped because the
	// previous run was still active when this one came due (FR-035).
	RunStatusSkipped RunStatus = "skipped"
)

// IsTerminal reports whether status represents a run that has finished and
// will not transition further.
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusCompleted, RunStatusFailed, RunStatusSkipped:
		return true
	default:
		return false
	}
}

// SourceType identifies whether a Run originated from a Job or a Chore.
type SourceType string

const (
	SourceTypeJob   SourceType = "job"
	SourceTypeChore SourceType = "chore"
)

// Run is a single execution record for a Job or Chore (FR-028, FR-036,
// FR-040–FR-044). Every field needed for History's list/filter/detail views
// and the Observability NFR lives here.
type Run struct {
	// ID is the unique identifier (UUID).
	ID string
	// OwnerUserID is the owning user (FR-009/FR-010) — always the owner of
	// the source Job/Chore, denormalized here so History can list/filter
	// runs without joining back to the source on every query.
	OwnerUserID string
	// SourceType and SourceID identify which Job or Chore produced this
	// Run.
	SourceType SourceType
	SourceID   string
	// Status is this run's current lifecycle state.
	Status RunStatus
	// StartedAt/EndedAt are nil until the run reaches that stage.
	StartedAt *time.Time
	EndedAt   *time.Time
	// LogPath is the durable, on-disk location of this run's full
	// processing log (Observability NFR) — captured and retrievable via
	// History even after the run completes, subject to History retention.
	LogPath string
	// ResultsPath is the on-disk location of this run's RESULTS.md — the
	// agent's final summary/output for the run (data-dictionary.md's
	// resolved scope: a markdown file, not a structured field).
	ResultsPath string
	// SkipReason is set when Status == RunStatusSkipped (FR-035), e.g.
	// "skipped — previous run still active".
	SkipReason *string
	// Error records a worker crash/failure against the run rather than
	// silently dropping it (Observability NFR), including provider/LLM
	// failures (spec.md Edge Case #13) and stale-Project-reference
	// failures (Edge Case #2).
	Error *string
	// NotifiedAt drives the History notification badge's clear-on-view
	// behavior (FR-044). Also set (not left nil) when a retention sweep
	// deletes an unviewed run, so the badge count never references a
	// deleted run (spec.md Edge Case #7) — see RunRepository's retention
	// sweep contract.
	NotifiedAt *time.Time
	// CreatedAt is when the Run record was created (i.e., enqueued).
	CreatedAt time.Time
}

// Duration returns the run's elapsed execution time. Returns 0 if the run
// has not started, or has started but not yet ended.
func (r *Run) Duration() time.Duration {
	if r.StartedAt == nil || r.EndedAt == nil {
		return 0
	}
	return r.EndedAt.Sub(*r.StartedAt)
}

// IsUnviewed reports whether this completed run should still count toward
// the History notification badge (FR-044): terminal status and not yet
// notified/viewed.
func (r *Run) IsUnviewed() bool {
	return r.Status.IsTerminal() && r.NotifiedAt == nil
}

// RunFilter narrows a ListRuns query (FR-041): by source (Job/Chore), date
// range, and status. Zero-value fields are "no filter" for that dimension.
type RunFilter struct {
	SourceType *SourceType
	SourceID   *string
	Status     *RunStatus
	Since      *time.Time
	Until      *time.Time
}
