// Package jobs orchestrates the web admin's Jobs environment (FR-024–
// FR-030): create/list/get/delete a one-time agent task, persisting its
// Description as JOB-DESCRIPTION.md in a hidden directory (FR-025),
// resolving a Project-context Job's default working directory (FR-026),
// and enqueuing every created Job onto the shared worker pool in FIFO order
// (FR-027). Modeled on internal/usecase/chats and internal/usecase/projects
// for consistency, per spec.md's guidance to model net-new entities on
// existing patterns.
package jobs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
)

// jobDescriptionFileName is the file a Job's Description is persisted to
// within its HiddenDirectory (FR-025).
const jobDescriptionFileName = "JOB-DESCRIPTION.md"

// EnqueueRequest is the minimal shape Service needs to hand a newly-created
// Job's Run off to the shared worker pool (FR-027).
type EnqueueRequest struct {
	RunID       string
	OwnerUserID string
	SourceID    string
}

// RunEnqueuer is the interface Service depends on to enqueue a Run onto the
// shared worker pool (FR-027, FR-039). Defined here rather than imported
// from internal/infrastructure/scheduler: AGENTS.md's Clean Architecture
// layering forbids the usecase layer depending on infrastructure
// implementations — only on interfaces it defines itself, which
// infrastructure then implements. A concrete adapter wrapping
// scheduler.WorkerPool is wired centrally in cmd/nuimanbot/main.go, not
// here — see internal/adapter/web/jobs_handler.go's doc comment for the
// same convention applied one layer up.
type RunEnqueuer interface {
	Enqueue(ctx context.Context, req EnqueueRequest) error
}

// ProjectDirectoryLookup resolves a Project's OutputDirectory by ID, scoped
// to ownerUserID, so Service can default a Project-context Job's
// WorkingDirectory (FR-026) without a hard dependency on
// internal/usecase/projects. A nil ProjectDirectoryLookup is a deliberate
// test-only convenience (production always wires one — see
// cmd/nuimanbot/extended_context.go): with no lookup configured at all,
// verification is skipped and WorkingDirectory is simply left unresolved.
//
// A *non-nil* lookup that errors (contextID doesn't resolve for
// ownerUserID) now REJECTS Job creation (FR-002, auto-review fix pass) —
// this is a deliberate behavior change from this feature's original pass.
// The implementation backing this interface
// (cmd/nuimanbot/extended_context.go's projectDirectoryLookupAdapter, via
// domain.ProjectRepository.GetProject) folds "belongs to a different
// owner" and "never existed / was deleted" into the identical
// domain.ErrNotFound by design (an anti-IDOR defense — see
// FileProjectRepository's doc comment), so CreateJob cannot distinguish
// them and does not try: any unresolved-for-this-owner contextID is
// rejected. A Project deleted *after* a Job was created is unaffected by
// this — that case is still tolerated at run time, by
// internal/infrastructure/scheduler/stub_executor.go's checkProjectExists.
type ProjectDirectoryLookup interface {
	OutputDirectoryFor(ctx context.Context, ownerUserID, projectID string) (string, error)
}

// ChatOwnershipCheck verifies that a Chat exists and is owned by
// ownerUserID, so Service can reject a --chat contextID belonging to a
// different user (FR-002, auto-review fix pass) without a hard dependency
// on internal/usecase/chats — mirrors ProjectDirectoryLookup's role for
// ContextTypeProject, but Chat has no working directory to resolve, so this
// is a pure ownership check.
//
// Unlike ProjectDirectoryLookup, a nil ChatOwnershipCheck does NOT
// silently skip verification for a non-empty --chat contextID: this
// interface has no pre-existing "nil is fine" contract to preserve (it's
// introduced fresh by FR-002), so it fails closed instead — see
// verifyContextOwnership's doc comment and implementation-notes.md's
// Research Question 1 resolution. Production always wires a non-nil
// implementation (see cmd/nuimanbot/extended_context.go).
type ChatOwnershipCheck interface {
	VerifyChatOwnership(ctx context.Context, ownerUserID, chatID string) error
}

// Service orchestrates Job create/list/get/delete.
type Service struct {
	jobs          domain.JobRepository
	runs          domain.RunRepository
	enqueuer      RunEnqueuer
	projectLookup ProjectDirectoryLookup
	chatOwnership ChatOwnershipCheck
	files         domain.ConfinedFileStore
	hiddenRoot    string
	now           func() time.Time
}

// NewService creates a Jobs Service backed by jobs and runs, enqueuing new
// Runs via enqueuer, resolving Project-context working directories via
// projectLookup (may be nil — see ProjectDirectoryLookup's doc comment),
// and verifying Chat-context ownership via chatOwnership (may be nil — see
// ChatOwnershipCheck's doc comment; nil behaves differently from a nil
// projectLookup, deliberately). files performs this Service's confined
// filesystem I/O (FR-R5): Service depends only on this domain-defined
// interface, never on "os" or internal/infrastructure/fsguard directly,
// per AGENTS.md's Clean Architecture dependency rule.
//
// hiddenRoot is the storage root under which each Job's HiddenDirectory is
// created, following the same per-owner layout as
// storage.FileJobRepository's record path:
// <hiddenRoot>/users/<ownerUserID>/jobs/<jobID>/. A Job has no OutputDirectory
// of its own to nest a hidden directory inside (unlike Project, which nests
// its hidden directory inside OutputDirectory) — a Job's context may be a
// Chat, which exposes no working directory at all — so Service needs an
// explicit storage root rather than deriving one from the Job itself.
func NewService(jobs domain.JobRepository, runs domain.RunRepository, enqueuer RunEnqueuer, projectLookup ProjectDirectoryLookup, chatOwnership ChatOwnershipCheck, files domain.ConfinedFileStore, hiddenRoot string) *Service {
	return &Service{
		jobs:          jobs,
		runs:          runs,
		enqueuer:      enqueuer,
		projectLookup: projectLookup,
		chatOwnership: chatOwnership,
		files:         files,
		hiddenRoot:    hiddenRoot,
		now:           time.Now,
	}
}

// CreateJob creates a new Job (FR-024), persists description as
// JOB-DESCRIPTION.md in its HiddenDirectory (FR-025), resolves
// WorkingDirectory when contextType is ContextTypeProject (FR-026), creates
// a queued Run for it, and enqueues that Run onto the shared worker pool
// (FR-027).
//
// A non-empty contextID that does not resolve to a Project/Chat owned by
// ownerUserID is rejected (domain.ErrNotFound) before anything is
// persisted (FR-002, auto-review fix pass — closes spec.md line 157's
// acceptance criterion) — see verifyContextOwnership's doc comment for the
// full rationale, including why "not owned" and "stale/deleted" are
// rejected identically.
func (s *Service) CreateJob(ctx context.Context, ownerUserID, title, description string, contextType domain.ContextType, contextID string) (*domain.Job, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", domain.ErrInvalidInput)
	}

	// Verified before anything is persisted, so a rejected context never
	// leaves behind an orphaned hidden directory (FR-002).
	workingDirectory, err := s.verifyContextOwnership(ctx, ownerUserID, contextType, contextID)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	hiddenDir := filepath.Join(s.hiddenRoot, "users", ownerUserID, "jobs", id)
	if err := s.files.EnsureDir(hiddenDir); err != nil {
		return nil, fmt.Errorf("failed to create job hidden directory: %w", err)
	}
	if err := s.writeJobDescription(hiddenDir, description); err != nil {
		return nil, err
	}

	now := s.now()
	job := &domain.Job{
		ID:               id,
		OwnerUserID:      ownerUserID,
		Title:            title,
		Description:      description,
		HiddenDirectory:  hiddenDir,
		ContextType:      contextType,
		ContextID:        contextID,
		WorkingDirectory: workingDirectory,
		Status:           domain.JobStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.jobs.SaveJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save job: %w", err)
	}

	run := &domain.Run{
		ID:          uuid.NewString(),
		OwnerUserID: ownerUserID,
		SourceType:  domain.SourceTypeJob,
		SourceID:    job.ID,
		Status:      domain.RunStatusQueued,
		CreatedAt:   now,
	}
	if err := s.runs.SaveRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	if err := s.enqueuer.Enqueue(ctx, EnqueueRequest{RunID: run.ID, OwnerUserID: ownerUserID, SourceID: job.ID}); err != nil {
		return nil, fmt.Errorf("failed to enqueue job run: %w", err)
	}

	return job, nil
}

// writeJobDescription writes description to JOB-DESCRIPTION.md inside
// hiddenDir (FR-025), via s.files (domain.ConfinedFileStore), which
// resolves the path through fsguard.ResolveWithin per package fsguard's
// mandate that every Job/Chore/Project filesystem operation use it.
func (s *Service) writeJobDescription(hiddenDir, description string) error {
	if err := s.files.WriteFile(hiddenDir, jobDescriptionFileName, []byte(description)); err != nil {
		return fmt.Errorf("failed to write job description: %w", err)
	}
	return nil
}

// verifyContextOwnership resolves and verifies a non-empty contextID
// against contextType (FR-002), rejecting (domain.ErrNotFound) one that
// does not resolve to a Project/Chat owned by ownerUserID. Returns the
// resolved WorkingDirectory for ContextTypeProject (always empty for
// ContextTypeChat, which has none, and for an empty contextID — no
// context to verify).
//
// A foreign-owned and a genuinely nonexistent/stale contextID are rejected
// identically: FileProjectRepository.GetProject and the Chat-ownership
// check's underlying domain.ConversationRepository.GetConversation both
// deliberately fold "belongs to someone else" into the same
// domain.ErrNotFound a caller sees for "never existed" — an anti-IDOR
// design choice (see FileProjectRepository's doc comment), not an
// oversight. CreateJob cannot distinguish the two cases without
// re-introducing an existence oracle, so it does not try.
//
// This intentionally supersedes this feature's original "stale reference
// tolerance" behavior for ContextTypeProject (formerly Edge Case #2): a
// contextID that fails to resolve against a *non-nil* lookup/checker now
// rejects CreateJob outright, rather than silently proceeding with an
// unresolved WorkingDirectory. This does not reopen Edge Case #2's actual
// concern — a Project deleted *after* Job creation — since CreateJob-time
// validation was never able to see a future deletion anyway; that case
// remains handled at run time by
// internal/infrastructure/scheduler/stub_executor.go's checkProjectExists.
// See implementation-notes.md (task A.1, Research Question 1) for the full
// rationale, including why a nil ChatOwnershipCheck behaves differently
// from a nil ProjectDirectoryLookup.
func (s *Service) verifyContextOwnership(ctx context.Context, ownerUserID string, contextType domain.ContextType, contextID string) (string, error) {
	if contextID == "" {
		return "", nil
	}

	switch contextType {
	case domain.ContextTypeProject:
		if s.projectLookup == nil {
			// No lookup wired at all: a deliberate test-only convenience
			// (production always wires one) — verification is skipped
			// entirely, exactly as before FR-002.
			return "", nil
		}
		dir, err := s.projectLookup.OutputDirectoryFor(ctx, ownerUserID, contextID)
		if err != nil {
			return "", fmt.Errorf("%w: --project %q is not accessible to this user", domain.ErrNotFound, contextID)
		}
		return dir, nil
	case domain.ContextTypeChat:
		if s.chatOwnership == nil {
			// Unlike a nil ProjectDirectoryLookup, a nil ChatOwnershipCheck
			// fails closed rather than skipping verification — see
			// ChatOwnershipCheck's doc comment.
			return "", fmt.Errorf("%w: --chat %q could not be verified (no chat ownership check configured)", domain.ErrNotFound, contextID)
		}
		if err := s.chatOwnership.VerifyChatOwnership(ctx, ownerUserID, contextID); err != nil {
			return "", fmt.Errorf("%w: --chat %q is not accessible to this user", domain.ErrNotFound, contextID)
		}
		return "", nil
	default:
		// Unreachable via any caller in this codebase today: every path
		// that supplies a non-empty contextID also supplies a matching
		// ContextTypeProject/ContextTypeChat (see job_commands.go's
		// parseContextFlag and jobs_handler.go's handleJobCreate). Rejects
		// rather than silently skipping verification anyway, for the same
		// fail-closed reason ChatOwnershipCheck's nil case does: an
		// unrecognized contextType paired with a real contextID cannot be
		// ownership-verified, so it must not be trusted implicitly.
		return "", fmt.Errorf("%w: unrecognized context type %q for %q", domain.ErrNotFound, contextType, contextID)
	}
}

// ListJobs returns ownerUserID's Jobs.
func (s *Service) ListJobs(ctx context.Context, ownerUserID string) ([]*domain.Job, error) {
	return s.jobs.ListJobs(ctx, ownerUserID)
}

// GetJob retrieves a Job by ID, scoped to its owner. Cross-owner access
// resolves as domain.ErrNotFound (enforced by the repository layer; see
// domain.JobRepository's doc comment).
func (s *Service) GetJob(ctx context.Context, ownerUserID, jobID string) (*domain.Job, error) {
	return s.jobs.GetJob(ctx, ownerUserID, jobID)
}

// DeleteJob deletes a Job (spec.md Edge Case #3): if its most recent status
// is JobStatusRunning or JobStatusQueued, the record is soft-marked
// PendingDeletion instead of removed, so the in-flight/queued run is not
// killed mid-write; otherwise the record is deleted outright.
//
// TODO(scheduler-integration): a background sweep that hard-deletes a
// PendingDeletion Job once its run reaches a terminal state is out of
// scope for this pass — see spec.md Edge Case #3's second half. No new
// runs should be enqueued for a Job with PendingDeletion set, but enforcing
// that at enqueue time also belongs to that future sweep/worker
// integration, not this Service.
//
// FR-R14 decision: domain.Job previously had an IsQueueable() guard
// (PendingDeletion + not-already-running/queued) encoding exactly this
// invariant, but it had no caller — CreateJob (the only place that
// enqueues) always builds a brand-new Job, which is queueable by
// construction. There is no re-run/re-enqueue path anywhere in this
// codebase today for IsQueueable to guard, so it was removed as dead code
// rather than wired in speculatively. Reintroduce an equivalent check at
// whatever point a re-run/retry flow is added, guarding that flow's
// enqueue call specifically.
func (s *Service) DeleteJob(ctx context.Context, ownerUserID, jobID string) error {
	job, err := s.jobs.GetJob(ctx, ownerUserID, jobID)
	if err != nil {
		return err
	}

	if job.Status == domain.JobStatusRunning || job.Status == domain.JobStatusQueued {
		job.PendingDeletion = true
		job.UpdatedAt = s.now()
		if err := s.jobs.SaveJob(ctx, job); err != nil {
			return fmt.Errorf("failed to soft-delete job: %w", err)
		}
		return nil
	}

	return s.jobs.DeleteJob(ctx, ownerUserID, jobID)
}

// CleanupPendingDeletion hard-deletes every PendingDeletion Job owned by
// ownerUserID whose Run has reached a terminal state (FR-R9), returning the
// count deleted. A PendingDeletion Job whose Run is still active is left
// alone — a later sweep pass will find it eligible once the run finishes.
// No new runs are enqueued for a PendingDeletion Job in the first place
// (enforced by CreateJob's callers never re-enqueuing a Job past deletion),
// so this only ever needs to check the Job's most recent Run, not defend
// against new ones appearing mid-sweep.
func (s *Service) CleanupPendingDeletion(ctx context.Context, ownerUserID string) (int, error) {
	list, err := s.jobs.ListJobs(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, j := range list {
		if !j.PendingDeletion {
			continue
		}
		active, err := s.hasActiveRun(ctx, ownerUserID, j.ID)
		if err != nil {
			continue // Best-effort sweep; one failure must not abort the rest.
		}
		if active {
			continue
		}
		// Hard-delete directly via the repository, not s.DeleteJob: that
		// method's soft/hard-delete branch reads Job.Status, which this
		// sweep's own hasActiveRun check (querying Runs directly) has
		// already made redundant — and, unlike Chore, Job.Status is not
		// currently kept in sync with its Run's actual lifecycle (no
		// caller ever invokes JobRepository.UpdateStatus in this codebase
		// today), so re-deriving activeness from Status here would be
		// unreliable.
		if err := s.jobs.DeleteJob(ctx, ownerUserID, j.ID); err != nil {
			continue // Best-effort sweep; one failure must not abort the rest.
		}
		deleted++
	}
	return deleted, nil
}

// hasActiveRun reports whether jobID has any Run in a non-terminal state
// (Queued or Running), mirroring chores.Service's identical-shaped check
// (not shared across packages — each usecase package depends only on
// domain, per AGENTS.md's Clean Architecture rule, and this is a handful
// of lines, not worth a new shared package for).
func (s *Service) hasActiveRun(ctx context.Context, ownerUserID, jobID string) (bool, error) {
	sourceType := domain.SourceTypeJob
	runs, err := s.runs.ListRuns(ctx, ownerUserID, domain.RunFilter{SourceType: &sourceType, SourceID: &jobID})
	if err != nil {
		return false, fmt.Errorf("failed to check job's active runs: %w", err)
	}
	for _, r := range runs {
		if !r.Status.IsTerminal() {
			return true, nil
		}
	}
	return false, nil
}
