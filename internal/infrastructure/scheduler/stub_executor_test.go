package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// newTestStubExecutor builds a StubExecutor with fresh, empty file-backed
// Job/Chore/Project repositories for tests that don't care about FR-R12's
// deleted-Project check.
func newTestStubExecutor(runsRepo domain.RunRepository, baseDir string) *StubExecutor {
	tmp := baseDir + "-repos"
	return NewStubExecutor(
		runsRepo,
		storage.NewFileJobRepository(tmp),
		storage.NewFileChoreRepository(tmp),
		storage.NewFileProjectRepository(tmp),
		baseDir,
	)
}

func TestStubExecutor_CompletesRun(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	baseDir := t.TempDir()
	ctx := context.Background()

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusQueued, CreatedAt: time.Now()}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	exec := newTestStubExecutor(runsRepo, baseDir)
	exec.Execute(ctx, RunRequest{RunID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1"})

	got, err := runsRepo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusCompleted {
		t.Fatalf("expected Completed, got %v (error=%v)", got.Status, got.Error)
	}
	if got.StartedAt == nil || got.EndedAt == nil {
		t.Fatal("expected StartedAt and EndedAt to be set")
	}
	if got.ResultsPath == "" {
		t.Fatal("expected ResultsPath to be set")
	}
	if _, err := os.Stat(got.ResultsPath); err != nil {
		t.Fatalf("expected RESULTS.md to exist on disk: %v", err)
	}
	content, err := os.ReadFile(got.ResultsPath)
	if err != nil {
		t.Fatalf("reading results file: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty results content")
	}
	// ResultsPath must be confined under baseDir (fsguard discipline).
	absBase, _ := filepath.Abs(baseDir)
	absResults, _ := filepath.Abs(got.ResultsPath)
	if !filepathHasPrefix(absResults, absBase) {
		t.Fatalf("expected results path %q to be confined under %q", absResults, absBase)
	}
}

func TestStubExecutor_AppendsLifecycleLog(t *testing.T) {
	runsBase := t.TempDir()
	runsRepo := storage.NewFileRunRepository(runsBase)
	baseDir := t.TempDir()
	ctx := context.Background()

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1"}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	exec := newTestStubExecutor(runsRepo, baseDir)
	exec.Execute(ctx, RunRequest{RunID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1"})

	// FileRunRepository's log layout: <basePath>/users/<owner>/runs/<runID>.log
	logPath := filepath.Join(runsBase, "users", "user-a", "runs", "r1.log")
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if len(logData) == 0 {
		t.Fatal("expected non-empty log content")
	}
}

func TestStubExecutor_FailsGracefullyOnGetRunError(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	baseDir := t.TempDir()
	ctx := context.Background()

	exec := newTestStubExecutor(runsRepo, baseDir)
	// No run was ever saved — Execute must not panic, just log and return.
	exec.Execute(ctx, RunRequest{RunID: "does-not-exist", OwnerUserID: "user-a", SourceID: "job-1"})
}

func TestStubExecutor_FailsOnCancelledContext(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	baseDir := t.TempDir()
	ctx := context.Background()

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1"}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	exec := newTestStubExecutor(runsRepo, baseDir)
	exec.Execute(cancelledCtx, RunRequest{RunID: "r1", OwnerUserID: "user-a", SourceID: "job-1"})

	got, err := runsRepo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusFailed {
		t.Fatalf("expected Failed for a cancelled context, got %v", got.Status)
	}
	if got.Error == nil {
		t.Fatal("expected Error to be populated")
	}
}

// FR-R12/Edge Case #2: a Job run against a Project that has since been
// deleted must fail cleanly rather than complete as if nothing were wrong.
func TestStubExecutor_FailsWhenJobsProjectDeleted(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	reposDir := t.TempDir()
	jobsRepo := storage.NewFileJobRepository(reposDir)
	choresRepo := storage.NewFileChoreRepository(reposDir)
	projectsRepo := storage.NewFileProjectRepository(reposDir)
	baseDir := t.TempDir()
	ctx := context.Background()

	project := &domain.Project{ID: "proj-1", OwnerUserID: "user-a", Name: "Test Project", OutputDirectory: t.TempDir()}
	if err := projectsRepo.SaveProject(ctx, project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	job := &domain.Job{ID: "job-1", OwnerUserID: "user-a", Title: "Test Job", ContextType: domain.ContextTypeProject, ContextID: project.ID}
	if err := jobsRepo.SaveJob(ctx, job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: job.ID, Status: domain.RunStatusQueued, CreatedAt: time.Now()}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	if err := projectsRepo.DeleteProject(ctx, "user-a", project.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	exec := NewStubExecutor(runsRepo, jobsRepo, choresRepo, projectsRepo, baseDir)
	exec.Execute(ctx, RunRequest{RunID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: job.ID})

	got, err := runsRepo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusFailed {
		t.Fatalf("expected Failed, got %v", got.Status)
	}
	if got.Error == nil || *got.Error != "referenced Project no longer exists" {
		t.Fatalf(`expected Error = "referenced Project no longer exists", got %v`, got.Error)
	}
}

// A Job/Chore with no Project context (ContextType == ContextTypeChat, or
// unset) must not be affected by the deleted-Project check.
func TestStubExecutor_NoProjectContextCompletesNormally(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	reposDir := t.TempDir()
	jobsRepo := storage.NewFileJobRepository(reposDir)
	choresRepo := storage.NewFileChoreRepository(reposDir)
	projectsRepo := storage.NewFileProjectRepository(reposDir)
	baseDir := t.TempDir()
	ctx := context.Background()

	job := &domain.Job{ID: "job-1", OwnerUserID: "user-a", Title: "Chat Job", ContextType: domain.ContextTypeChat, ContextID: "chat-1"}
	if err := jobsRepo.SaveJob(ctx, job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: job.ID, Status: domain.RunStatusQueued, CreatedAt: time.Now()}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	exec := NewStubExecutor(runsRepo, jobsRepo, choresRepo, projectsRepo, baseDir)
	exec.Execute(ctx, RunRequest{RunID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: job.ID})

	got, err := runsRepo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusCompleted {
		t.Fatalf("expected Completed, got %v (error=%v)", got.Status, got.Error)
	}
}

// filepathHasPrefix reports whether path is dir itself or a descendant of it.
func filepathHasPrefix(path, dir string) bool {
	if path == dir {
		return true
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
