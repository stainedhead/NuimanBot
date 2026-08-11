package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// fakeEnqueuer is a test double for RunEnqueuer.
type fakeEnqueuer struct {
	requests []EnqueueRequest
	err      error
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, req EnqueueRequest) error {
	if f.err != nil {
		return f.err
	}
	f.requests = append(f.requests, req)
	return nil
}

// errJobRepo wraps a real domain.JobRepository, optionally forcing SaveJob
// to fail — used to exercise CreateJob/DeleteJob's error-propagation paths.
type errJobRepo struct {
	domain.JobRepository
	saveErr error
}

func (r *errJobRepo) SaveJob(ctx context.Context, j *domain.Job) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.JobRepository.SaveJob(ctx, j)
}

// errRunRepo wraps a real domain.RunRepository, optionally forcing SaveRun
// to fail.
type errRunRepo struct {
	domain.RunRepository
	saveErr error
}

func (r *errRunRepo) SaveRun(ctx context.Context, run *domain.Run) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	return r.RunRepository.SaveRun(ctx, run)
}

// fakeProjectLookup is a test double for ProjectDirectoryLookup, scoped by
// ownerUserID exactly like the real adapter
// (cmd/nuimanbot/extended_context.go's projectDirectoryLookupAdapter, backed
// by domain.ProjectRepository.GetProject): a project ID only resolves for
// the owner it is registered under — a caller-supplied ownerUserID that
// doesn't match (or an unregistered projectID) returns domain.ErrNotFound,
// exactly as FileProjectRepository.GetProject does for both a foreign-owned
// and a genuinely nonexistent project (see that type's doc comment: cross-
// owner access is deliberately indistinguishable from "never existed" — an
// anti-IDOR design choice, not an oversight). This owner-scoping (added for
// FR-002) is required to actually exercise the cross-owner-rejection tests
// below; the previous single-level map couldn't represent "exists, but for
// a different owner" at all.
type fakeProjectLookup struct {
	dirs map[string]map[string]string // ownerUserID -> projectID -> outputDirectory
	err  error
}

func (f *fakeProjectLookup) OutputDirectoryFor(_ context.Context, ownerUserID, projectID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	byOwner, ok := f.dirs[ownerUserID]
	if !ok {
		return "", domain.ErrNotFound
	}
	dir, ok := byOwner[projectID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return dir, nil
}

// fakeChatOwnership is a test double for ChatOwnershipCheck (FR-002), scoped
// by ownerUserID exactly like the real adapter
// (cmd/nuimanbot/extended_context.go's chatOwnershipCheckAdapter, backed by
// domain.ConversationRepository.GetConversation plus an owner comparison,
// mirroring chats.Service.GetChat's own ownership check).
type fakeChatOwnership struct {
	owned map[string]map[string]bool // ownerUserID -> chatID -> owned
	err   error
}

func (f *fakeChatOwnership) VerifyChatOwnership(_ context.Context, ownerUserID, chatID string) error {
	if f.err != nil {
		return f.err
	}
	if byOwner, ok := f.owned[ownerUserID]; ok && byOwner[chatID] {
		return nil
	}
	return domain.ErrNotFound
}

func newTestService(t *testing.T) (*Service, *fakeEnqueuer) {
	t.Helper()
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	svc := NewService(jobRepo, runRepo, enqueuer, nil, nil, storage.NewFileConfinedFileStore(), tmp)
	return svc, enqueuer
}

func TestCreateJob_Success(t *testing.T) {
	svc, enqueuer := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "Clean the inbox", "Archive anything older than 30 days.", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Title != "Clean the inbox" {
		t.Fatalf("expected title preserved, got %q", job.Title)
	}
	if job.Status != domain.JobStatusQueued {
		t.Fatalf("expected JobStatusQueued, got %v", job.Status)
	}
	if job.HiddenDirectory == "" {
		t.Fatal("expected a non-empty HiddenDirectory")
	}
	if job.WorkingDirectory != "" {
		t.Fatalf("expected empty WorkingDirectory for ContextTypeChat, got %q", job.WorkingDirectory)
	}

	// FR-025: Description persisted as JOB-DESCRIPTION.md in HiddenDirectory.
	descPath := filepath.Join(job.HiddenDirectory, "JOB-DESCRIPTION.md")
	data, err := os.ReadFile(descPath)
	if err != nil {
		t.Fatalf("expected JOB-DESCRIPTION.md to exist: %v", err)
	}
	if string(data) != "Archive anything older than 30 days." {
		t.Fatalf("unexpected JOB-DESCRIPTION.md content: %q", data)
	}

	// FR-027: enqueued onto the shared worker pool.
	if len(enqueuer.requests) != 1 {
		t.Fatalf("expected 1 enqueued request, got %d", len(enqueuer.requests))
	}
	if enqueuer.requests[0].OwnerUserID != "user-a" || enqueuer.requests[0].SourceID != job.ID {
		t.Fatalf("unexpected enqueue request: %+v", enqueuer.requests[0])
	}

	// FR-028: a Run record must exist for the job.
	runs, err := svc.runs.ListRuns(ctx, "user-a", domain.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run created, got %d", len(runs))
	}
	if runs[0].SourceType != domain.SourceTypeJob || runs[0].SourceID != job.ID {
		t.Fatalf("unexpected run: %+v", runs[0])
	}
	if runs[0].Status != domain.RunStatusQueued {
		t.Fatalf("expected RunStatusQueued, got %v", runs[0].Status)
	}
}

func TestCreateJob_RequiresOwnerUserID(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateJob(context.Background(), "", "Title", "Description", domain.ContextTypeChat, "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateJob_RequiresTitle(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.CreateJob(context.Background(), "user-a", "   ", "Description", domain.ContextTypeChat, "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateJob_ProjectContextResolvesWorkingDirectory(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	lookup := &fakeProjectLookup{dirs: map[string]map[string]string{"user-a": {"proj-1": "/output/proj-1"}}}
	svc := NewService(jobRepo, runRepo, enqueuer, lookup, nil, storage.NewFileConfinedFileStore(), tmp)

	job, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "Refactor the widget module.", domain.ContextTypeProject, "proj-1")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.WorkingDirectory != "/output/proj-1" {
		t.Fatalf("expected WorkingDirectory resolved from project, got %q", job.WorkingDirectory)
	}
	if job.ContextType != domain.ContextTypeProject || job.ContextID != "proj-1" {
		t.Fatalf("expected context preserved, got %v/%v", job.ContextType, job.ContextID)
	}
}

// TestCreateJob_UnresolvedProjectReferenceIsRejected supersedes the former
// "TestCreateJob_StaleProjectReferenceStillCreatesJob" (spec.md's original
// Edge Case #2). FR-002 (auto-review fix pass) established that a
// non-nil ProjectDirectoryLookup returning an error is *indistinguishable*
// from a foreign-owned project ID: FileProjectRepository.GetProject folds
// both cases into the same domain.ErrNotFound by design (see its doc
// comment — an anti-IDOR/existence-oracle defense, not an oversight), so
// CreateJob cannot tell "stale, formerly mine" from "belongs to someone
// else" and must not try. It now rejects both alike.
//
// This does not regress Edge Case #2's actual promise: that promise was
// about a Project deleted *after* a Job referencing it was created, which
// CreateJob-time validation cannot observe anyway (the deletion happens
// later). That case remains handled at run time by
// internal/infrastructure/scheduler/stub_executor.go's checkProjectExists
// (errProjectDeleted) — unaffected by this change. Only creation-time
// tolerance for an already-unresolvable reference is removed, and that
// tolerance was never actually distinguishable from the cross-owner bug
// FR-002 closes.
func TestCreateJob_UnresolvedProjectReferenceIsRejected(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	lookup := &fakeProjectLookup{dirs: map[string]map[string]string{}} // proj-1 not registered for anyone
	svc := NewService(jobRepo, runRepo, enqueuer, lookup, nil, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "Refactor the widget module.", domain.ContextTypeProject, "proj-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound rejecting an unresolvable project reference, got: %v", err)
	}

	list, err := svc.ListJobs(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no Job to be persisted for a rejected create, got %d", len(list))
	}
}

// TestCreateJob_ForeignProjectContextRejected is FR-002's mandatory explicit
// acceptance test (spec.md line 157), mirroring TestGetJob_CrossOwnerIsolation's
// pattern: a --project <id> belonging to a *different* user must be
// rejected, never silently attached to the caller's new Job.
func TestCreateJob_ForeignProjectContextRejected(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	// proj-1 exists, but only under "user-b", not "user-a".
	lookup := &fakeProjectLookup{dirs: map[string]map[string]string{"user-b": {"proj-1": "/output/proj-1"}}}
	svc := NewService(jobRepo, runRepo, enqueuer, lookup, nil, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "desc", domain.ContextTypeProject, "proj-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a foreign-owned project context, got: %v", err)
	}

	list, err := svc.ListJobs(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no Job to be persisted for a rejected create, got %d", len(list))
	}
}

func TestCreateJob_NilProjectLookupStillCreatesJob(t *testing.T) {
	// No ProjectDirectoryLookup configured at all (e.g. wiring omitted it):
	// Project-context Jobs must still be creatable, just without a resolved
	// WorkingDirectory. Production always wires a non-nil lookup (see
	// cmd/nuimanbot/extended_context.go) — this nil-tolerance is a
	// deliberate test-only convenience, unlike ChatOwnershipCheck's nil
	// behavior (see TestCreateJob_NilChatOwnershipRejectsNonEmptyChatContext),
	// which was defined fresh for FR-002 with no such pre-existing contract
	// to preserve.
	svc, _ := newTestService(t) // constructed with a nil ProjectDirectoryLookup
	job, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "desc", domain.ContextTypeProject, "proj-1")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.WorkingDirectory != "" {
		t.Fatalf("expected empty WorkingDirectory, got %q", job.WorkingDirectory)
	}
}

// TestCreateJob_ChatContextOwnedSucceeds verifies the happy path for
// FR-002's new ContextTypeChat ownership check: a --chat <id> the caller
// does own resolves normally (no WorkingDirectory — Chat has none).
func TestCreateJob_ChatContextOwnedSucceeds(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	chatOwnership := &fakeChatOwnership{owned: map[string]map[string]bool{"user-a": {"chat-1": true}}}
	svc := NewService(jobRepo, runRepo, enqueuer, nil, chatOwnership, storage.NewFileConfinedFileStore(), tmp)

	job, err := svc.CreateJob(context.Background(), "user-a", "Summarize", "desc", domain.ContextTypeChat, "chat-1")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.ContextType != domain.ContextTypeChat || job.ContextID != "chat-1" {
		t.Fatalf("expected context preserved, got %v/%v", job.ContextType, job.ContextID)
	}
	if job.WorkingDirectory != "" {
		t.Fatalf("expected empty WorkingDirectory for a Chat context, got %q", job.WorkingDirectory)
	}
}

// TestCreateJob_ForeignChatContextRejected is FR-002's mandatory explicit
// acceptance test for --chat, mirroring TestCreateJob_ForeignProjectContextRejected:
// previously ContextTypeChat had NO ownership check at all (worse than
// Project's best-effort one) — this proves it now gets the same rejection
// treatment.
func TestCreateJob_ForeignChatContextRejected(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	// chat-1 exists, but only under "user-b", not "user-a".
	chatOwnership := &fakeChatOwnership{owned: map[string]map[string]bool{"user-b": {"chat-1": true}}}
	svc := NewService(jobRepo, runRepo, enqueuer, nil, chatOwnership, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Summarize", "desc", domain.ContextTypeChat, "chat-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a foreign-owned chat context, got: %v", err)
	}

	list, err := svc.ListJobs(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no Job to be persisted for a rejected create, got %d", len(list))
	}
}

// TestCreateJob_NilChatOwnershipRejectsNonEmptyChatContext documents a
// deliberate asymmetry with ProjectDirectoryLookup's nil-tolerance
// (Research Question 1, see implementation-notes.md task A.1): unlike
// Project, ChatOwnershipCheck has no pre-existing "nil is fine" contract or
// test to preserve — it is introduced fresh by FR-002. A nil checker with a
// non-empty --chat contextID therefore fails closed (rejects) rather than
// silently skipping verification, so a future caller who forgets to wire a
// ChatOwnershipCheck cannot silently reopen the exact hole this fix closes.
// Production always wires one (see cmd/nuimanbot/extended_context.go).
func TestCreateJob_NilChatOwnershipRejectsNonEmptyChatContext(t *testing.T) {
	svc, _ := newTestService(t) // constructed with a nil ChatOwnershipCheck
	if _, err := svc.CreateJob(context.Background(), "user-a", "Summarize", "desc", domain.ContextTypeChat, "chat-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no ChatOwnershipCheck is wired and a chat context is requested, got: %v", err)
	}
}

func TestCreateJob_SaveJobErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := &errJobRepo{JobRepository: storage.NewFileJobRepository(tmp), saveErr: errors.New("disk full")}
	runRepo := storage.NewFileRunRepository(tmp)
	svc := NewService(jobRepo, runRepo, &fakeEnqueuer{}, nil, nil, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Title", "desc", domain.ContextTypeChat, ""); err == nil {
		t.Fatal("expected CreateJob to propagate the SaveJob error")
	}
}

func TestCreateJob_SaveRunErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := &errRunRepo{RunRepository: storage.NewFileRunRepository(tmp), saveErr: errors.New("disk full")}
	svc := NewService(jobRepo, runRepo, &fakeEnqueuer{}, nil, nil, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Title", "desc", domain.ContextTypeChat, ""); err == nil {
		t.Fatal("expected CreateJob to propagate the SaveRun error")
	}
}

func TestCreateJob_EnqueueErrorPropagates(t *testing.T) {
	svc, enqueuer := newTestService(t)
	enqueuer.err = errors.New("queue unavailable")

	if _, err := svc.CreateJob(context.Background(), "user-a", "Title", "desc", domain.ContextTypeChat, ""); err == nil {
		t.Fatal("expected CreateJob to propagate the Enqueue error")
	}
}

func TestWriteJobDescription_WriteFileErrorPropagates(t *testing.T) {
	svc, _ := newTestService(t)
	tmp := t.TempDir()
	blockingFile := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := svc.writeJobDescription(blockingFile, "desc"); err == nil {
		t.Fatal("expected an error writing JOB-DESCRIPTION.md under a non-directory path")
	}
}

func TestListJobs_Empty(t *testing.T) {
	svc, _ := newTestService(t)
	got, err := svc.ListJobs(context.Background(), "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", got)
	}
}

func TestListJobs_OwnerScoped(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.CreateJob(ctx, "user-a", "Job A", "desc", domain.ContextTypeChat, ""); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if _, err := svc.CreateJob(ctx, "user-b", "Job B", "desc", domain.ContextTypeChat, ""); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	got, err := svc.ListJobs(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Job A" {
		t.Fatalf("expected only user-a's job, got %+v", got)
	}
}

func TestGetJob_CrossOwnerIsolation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "secret job", "desc", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if _, err := svc.GetJob(ctx, "user-b", job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner access, got %v", err)
	}
	if got, err := svc.GetJob(ctx, "user-a", job.ID); err != nil || got.ID != job.ID {
		t.Fatalf("expected owner to retrieve their own job, got %v, err %v", got, err)
	}
}

func TestDeleteJob_CrossOwnerIsolation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "secret job", "desc", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if err := svc.DeleteJob(ctx, "user-b", job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}
	if _, err := svc.GetJob(ctx, "user-a", job.ID); err != nil {
		t.Fatalf("expected job to still exist: %v", err)
	}
}

func TestDeleteJob_HardDeletesWhenIdle(t *testing.T) {
	// A freshly-created job is Queued, so first drive it to a terminal state
	// via UpdateStatus (simulating a completed run) before testing the idle
	// hard-delete path.
	svc, _ := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "job", "desc", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := svc.jobs.UpdateStatus(ctx, "user-a", job.ID, domain.JobStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	if err := svc.DeleteJob(ctx, "user-a", job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, err := svc.GetJob(ctx, "user-a", job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after hard delete, got %v", err)
	}
}

func TestDeleteJob_SoftDeletesWhenRunningOrQueued(t *testing.T) {
	// Edge Case #3: deleting a Job with an active (queued/running) run
	// soft-marks it PendingDeletion rather than removing the record.
	svc, _ := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "job", "desc", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Status != domain.JobStatusQueued {
		t.Fatalf("expected freshly-created job to be Queued, got %v", job.Status)
	}

	if err := svc.DeleteJob(ctx, "user-a", job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	got, err := svc.GetJob(ctx, "user-a", job.ID)
	if err != nil {
		t.Fatalf("expected job record to still exist after soft delete, got err: %v", err)
	}
	if !got.PendingDeletion {
		t.Fatal("expected PendingDeletion to be set")
	}
	if got.Status != domain.JobStatusQueued {
		t.Fatalf("expected status unchanged by soft delete, got %v", got.Status)
	}

	// Also verify the Running case.
	if err := svc.jobs.UpdateStatus(ctx, "user-a", job.ID, domain.JobStatusRunning); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if err := svc.DeleteJob(ctx, "user-a", job.ID); err != nil {
		t.Fatalf("DeleteJob (running): %v", err)
	}
	got, err = svc.GetJob(ctx, "user-a", job.ID)
	if err != nil {
		t.Fatalf("expected job record to still exist while running, got err: %v", err)
	}
	if !got.PendingDeletion {
		t.Fatal("expected PendingDeletion to remain set while running")
	}
}

func TestDeleteJob_SoftDeleteSaveErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	realJobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	svc := NewService(realJobRepo, runRepo, enqueuer, nil, nil, storage.NewFileConfinedFileStore(), tmp)

	job, err := svc.CreateJob(context.Background(), "user-a", "job", "desc", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	svc.jobs = &errJobRepo{JobRepository: realJobRepo, saveErr: errors.New("disk full")}
	if err := svc.DeleteJob(context.Background(), "user-a", job.ID); err == nil {
		t.Fatal("expected DeleteJob to propagate the soft-delete SaveJob error")
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	if err := svc.DeleteJob(context.Background(), "user-a", "missing-id"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCleanupPendingDeletion_HardDeletesOnceRunIsTerminal(t *testing.T) {
	// FR-R9: a PendingDeletion Job whose Run has reached a terminal state
	// is hard-deleted by the sweep; one whose Run is still active is left
	// alone until a later sweep pass finds it eligible.
	svc, _ := newTestService(t)
	ctx := context.Background()

	stillActive, err := svc.CreateJob(ctx, "user-a", "still running", "d", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := svc.DeleteJob(ctx, "user-a", stillActive.ID); err != nil {
		t.Fatalf("DeleteJob (soft): %v", err)
	}

	doneRunning, err := svc.CreateJob(ctx, "user-a", "finished", "d", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := svc.DeleteJob(ctx, "user-a", doneRunning.ID); err != nil {
		t.Fatalf("DeleteJob (soft): %v", err)
	}
	// Advance the run for doneRunning to a terminal state.
	runs, err := svc.runs.ListRuns(ctx, "user-a", domain.RunFilter{SourceID: &doneRunning.ID})
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected exactly 1 run for doneRunning, got %v (err %v)", runs, err)
	}
	runs[0].Status = domain.RunStatusCompleted
	if err := svc.runs.SaveRun(ctx, runs[0]); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	n, err := svc.CleanupPendingDeletion(ctx, "user-a")
	if err != nil {
		t.Fatalf("CleanupPendingDeletion: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 job cleaned up, got %d", n)
	}

	if _, err := svc.GetJob(ctx, "user-a", doneRunning.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected doneRunning job to be hard-deleted, got %v", err)
	}
	if _, err := svc.GetJob(ctx, "user-a", stillActive.ID); err != nil {
		t.Fatalf("expected stillActive job to remain (its run is still active), got %v", err)
	}
}

func TestCleanupPendingDeletion_IgnoresNonPendingJobs(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, "user-a", "idle", "d", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	_ = job

	n, err := svc.CleanupPendingDeletion(ctx, "user-a")
	if err != nil {
		t.Fatalf("CleanupPendingDeletion: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 cleaned up (job is not PendingDeletion), got %d", n)
	}
}

func TestCleanupPendingDeletion_ListJobsErrorPropagates(t *testing.T) {
	svc, _ := newTestService(t)
	svc.jobs = &errListJobsRepo{JobRepository: svc.jobs, listErr: errors.New("disk error")}

	if _, err := svc.CleanupPendingDeletion(context.Background(), "user-a"); err == nil {
		t.Fatal("expected CleanupPendingDeletion to propagate the ListJobs error")
	}
}

// errListJobsRepo wraps a real domain.JobRepository, optionally forcing
// ListJobs to fail.
type errListJobsRepo struct {
	domain.JobRepository
	listErr error
}

func (r *errListJobsRepo) ListJobs(ctx context.Context, ownerUserID string) ([]*domain.Job, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.JobRepository.ListJobs(ctx, ownerUserID)
}
