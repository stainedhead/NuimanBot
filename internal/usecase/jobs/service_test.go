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

// fakeProjectLookup is a test double for ProjectDirectoryLookup.
type fakeProjectLookup struct {
	dirs map[string]string // projectID -> outputDirectory
	err  error
}

func (f *fakeProjectLookup) OutputDirectoryFor(_ context.Context, _, projectID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	dir, ok := f.dirs[projectID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return dir, nil
}

func newTestService(t *testing.T) (*Service, *fakeEnqueuer) {
	t.Helper()
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	svc := NewService(jobRepo, runRepo, enqueuer, nil, storage.NewFileConfinedFileStore(), tmp)
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
	lookup := &fakeProjectLookup{dirs: map[string]string{"proj-1": "/output/proj-1"}}
	svc := NewService(jobRepo, runRepo, enqueuer, lookup, storage.NewFileConfinedFileStore(), tmp)

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

func TestCreateJob_StaleProjectReferenceStillCreatesJob(t *testing.T) {
	// Edge Case #2: Job creation succeeds even if ProjectDirectoryLookup
	// fails to resolve the referenced Project (e.g. it was deleted). The
	// Job's WorkingDirectory is simply left unresolved; the *run* is what
	// fails later, not creation.
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := storage.NewFileRunRepository(tmp)
	enqueuer := &fakeEnqueuer{}
	lookup := &fakeProjectLookup{dirs: map[string]string{}} // proj-1 not found
	svc := NewService(jobRepo, runRepo, enqueuer, lookup, storage.NewFileConfinedFileStore(), tmp)

	job, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "Refactor the widget module.", domain.ContextTypeProject, "proj-1")
	if err != nil {
		t.Fatalf("expected CreateJob to succeed despite stale project reference, got err: %v", err)
	}
	if job.WorkingDirectory != "" {
		t.Fatalf("expected empty WorkingDirectory for unresolved project, got %q", job.WorkingDirectory)
	}
	if job.Status != domain.JobStatusQueued {
		t.Fatalf("expected job still queued, got %v", job.Status)
	}
}

func TestCreateJob_NilProjectLookupStillCreatesJob(t *testing.T) {
	// No ProjectDirectoryLookup configured at all (e.g. wiring omitted it):
	// Project-context Jobs must still be creatable, just without a resolved
	// WorkingDirectory.
	svc, _ := newTestService(t) // constructed with a nil ProjectDirectoryLookup
	job, err := svc.CreateJob(context.Background(), "user-a", "Refactor", "desc", domain.ContextTypeProject, "proj-1")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.WorkingDirectory != "" {
		t.Fatalf("expected empty WorkingDirectory, got %q", job.WorkingDirectory)
	}
}

func TestCreateJob_SaveJobErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := &errJobRepo{JobRepository: storage.NewFileJobRepository(tmp), saveErr: errors.New("disk full")}
	runRepo := storage.NewFileRunRepository(tmp)
	svc := NewService(jobRepo, runRepo, &fakeEnqueuer{}, nil, storage.NewFileConfinedFileStore(), tmp)

	if _, err := svc.CreateJob(context.Background(), "user-a", "Title", "desc", domain.ContextTypeChat, ""); err == nil {
		t.Fatal("expected CreateJob to propagate the SaveJob error")
	}
}

func TestCreateJob_SaveRunErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	jobRepo := storage.NewFileJobRepository(tmp)
	runRepo := &errRunRepo{RunRepository: storage.NewFileRunRepository(tmp), saveErr: errors.New("disk full")}
	svc := NewService(jobRepo, runRepo, &fakeEnqueuer{}, nil, storage.NewFileConfinedFileStore(), tmp)

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
	svc := NewService(realJobRepo, runRepo, enqueuer, nil, storage.NewFileConfinedFileStore(), tmp)

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
