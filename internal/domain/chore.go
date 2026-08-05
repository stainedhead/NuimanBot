package domain

import "time"

// Chore is a recurring, cron-scheduled agent task (FR-031–FR-038). Unlike
// Job, a Chore has no terminal "Completed"/"Failed" Status of its own — each
// firing produces its own Run, and the Chore itself remains Active until
// deleted (see spec.md's Non-Goals: no "paused" state exists in this spec).
type Chore struct {
	// ID is the unique identifier (UUID).
	ID string
	// OwnerUserID is the owning user (FR-009/FR-010).
	OwnerUserID string
	// Title is the user-supplied Chore title.
	Title string
	// Description is the Chore's task description, persisted the same way
	// as a Job's JOB-DESCRIPTION.md (FR-031).
	Description string
	// HiddenDirectory holds JOB-DESCRIPTION.md and other agent-managed
	// files for this Chore.
	HiddenDirectory string
	// WorkingDirectory is optional (FR-031) — a Chore need not be tied to
	// a Project.
	WorkingDirectory string
	// ContextType/ContextID mirror Job's context model when the Chore is
	// tied to a Project (FR-031); ContextType is empty when the Chore has
	// no Project context.
	ContextType ContextType
	ContextID   string
	// Schedule is this Chore's cron-style recurrence (FR-032/FR-034).
	Schedule Schedule
	// ScheduleConfirmed is false for an agent-proposed schedule pending
	// user confirmation (FR-033). A Chore with ScheduleConfirmed == false
	// never fires — see NextFireTime's doc comment and spec.md Edge Case
	// #6 ("pending confirmation" never silently expires).
	ScheduleConfirmed bool
	// NextFireTime is this Chore's next scheduled firing, persisted so it
	// survives a server restart (Reliability NFR). Meaningless/ignored
	// while ScheduleConfirmed is false.
	NextFireTime time.Time
	// PendingDeletion marks a Chore soft-deleted while a run is still
	// active (spec.md Edge Case #3), mirroring Job.PendingDeletion.
	PendingDeletion bool
	// CreatedAt is when the Chore was created.
	CreatedAt time.Time
	// UpdatedAt is the Chore's last-modification time.
	UpdatedAt time.Time
}

// IsDue reports whether this Chore should fire as of now: it must be
// confirmed, not pending deletion, and its NextFireTime must have arrived.
// Callers are also responsible for the FR-035 skip-if-still-running check,
// which requires knowledge of in-flight runs this entity does not carry.
func (c *Chore) IsDue(now time.Time) bool {
	if !c.ScheduleConfirmed || c.PendingDeletion {
		return false
	}
	return !c.NextFireTime.After(now)
}
