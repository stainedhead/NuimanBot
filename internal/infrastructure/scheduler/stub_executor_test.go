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

func TestStubExecutor_CompletesRun(t *testing.T) {
	runsRepo := storage.NewFileRunRepository(t.TempDir())
	baseDir := t.TempDir()
	ctx := context.Background()

	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusQueued, CreatedAt: time.Now()}
	if err := runsRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	exec := NewStubExecutor(runsRepo, baseDir)
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

	exec := NewStubExecutor(runsRepo, baseDir)
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

	exec := NewStubExecutor(runsRepo, baseDir)
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

	exec := NewStubExecutor(runsRepo, baseDir)
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
