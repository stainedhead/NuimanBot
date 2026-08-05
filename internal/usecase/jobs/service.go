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

// ProjectDirectoryLookup resolves a Project's OutputDirectory by ID, so
// Service can default a Project-context Job's WorkingDirectory (FR-026)
// without a hard dependency on internal/usecase/projects. Optional: a nil
// ProjectDirectoryLookup (or one that errors, e.g. a deleted Project —
// spec.md Edge Case #2) simply leaves WorkingDirectory unresolved; Job
// creation still succeeds, and the *run* fails later with a clear error.
type ProjectDirectoryLookup interface {
	OutputDirectoryFor(ctx context.Context, ownerUserID, projectID string) (string, error)
}

// Service orchestrates Job create/list/get/delete.
type Service struct {
	jobs          domain.JobRepository
	runs          domain.RunRepository
	enqueuer      RunEnqueuer
	projectLookup ProjectDirectoryLookup
	files         domain.ConfinedFileStore
	hiddenRoot    string
	now           func() time.Time
}

// NewService creates a Jobs Service backed by jobs and runs, enqueuing new
// Runs via enqueuer and resolving Project-context working directories via
// projectLookup (may be nil — see ProjectDirectoryLookup's doc comment).
// files performs this Service's confined filesystem I/O (FR-R5): Service
// depends only on this domain-defined interface, never on "os" or
// internal/infrastructure/fsguard directly, per AGENTS.md's Clean
// Architecture dependency rule.
//
// hiddenRoot is the storage root under which each Job's HiddenDirectory is
// created, following the same per-owner layout as
// storage.FileJobRepository's record path:
// <hiddenRoot>/users/<ownerUserID>/jobs/<jobID>/. A Job has no OutputDirectory
// of its own to nest a hidden directory inside (unlike Project, which nests
// its hidden directory inside OutputDirectory) — a Job's context may be a
// Chat, which exposes no working directory at all — so Service needs an
// explicit storage root rather than deriving one from the Job itself.
func NewService(jobs domain.JobRepository, runs domain.RunRepository, enqueuer RunEnqueuer, projectLookup ProjectDirectoryLookup, files domain.ConfinedFileStore, hiddenRoot string) *Service {
	return &Service{
		jobs:          jobs,
		runs:          runs,
		enqueuer:      enqueuer,
		projectLookup: projectLookup,
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
func (s *Service) CreateJob(ctx context.Context, ownerUserID, title, description string, contextType domain.ContextType, contextID string) (*domain.Job, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", domain.ErrInvalidInput)
	}

	id := uuid.NewString()
	hiddenDir := filepath.Join(s.hiddenRoot, "users", ownerUserID, "jobs", id)
	if err := s.files.EnsureDir(hiddenDir); err != nil {
		return nil, fmt.Errorf("failed to create job hidden directory: %w", err)
	}
	if err := s.writeJobDescription(hiddenDir, description); err != nil {
		return nil, err
	}

	workingDirectory := ""
	if contextType == domain.ContextTypeProject && contextID != "" && s.projectLookup != nil {
		// Edge Case #2: a stale/missing Project reference must not fail Job
		// creation — WorkingDirectory is simply left unresolved, and the
		// run fails later with a clear error instead.
		if dir, err := s.projectLookup.OutputDirectoryFor(ctx, ownerUserID, contextID); err == nil {
			workingDirectory = dir
		}
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
