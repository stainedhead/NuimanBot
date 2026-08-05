package storage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

func newTestRunRepo(t *testing.T) *FileRunRepository {
	t.Helper()
	return NewFileRunRepository(t.TempDir())
}

func TestFileRunRepository_SaveAndGet(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()
	run := &domain.Run{ID: "r1", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusQueued, CreatedAt: time.Now()}
	if err := repo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	got, err := repo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.SourceID != "job-1" || got.Status != domain.RunStatusQueued {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
}

func TestFileRunRepository_CrossOwnerIsolation(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()
	if err := repo.SaveRun(ctx, &domain.Run{ID: "shared", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if _, err := repo.GetRun(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := repo.AppendLog(ctx, "user-b", "shared", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner AppendLog, got %v", err)
	}
	if err := repo.MarkNotified(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner MarkNotified, got %v", err)
	}
	if err := repo.DeleteRun(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}
}

func TestFileRunRepository_ListRuns_FilterAndOrder(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()
	now := time.Now()

	jobRunOld := &domain.Run{ID: "r-old", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour)}
	jobRunNew := &domain.Run{ID: "r-new", OwnerUserID: "user-a", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-1 * time.Minute)}
	choreRun := &domain.Run{ID: "r-chore", OwnerUserID: "user-a", SourceType: domain.SourceTypeChore, SourceID: "chore-1", Status: domain.RunStatusFailed, CreatedAt: now}

	for _, run := range []*domain.Run{jobRunOld, jobRunNew, choreRun} {
		if err := repo.SaveRun(ctx, run); err != nil {
			t.Fatalf("SaveRun: %v", err)
		}
	}

	// No filter: all 3, newest first.
	all, err := repo.ListRuns(ctx, "user-a", domain.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(all))
	}
	if all[0].ID != "r-chore" || all[2].ID != "r-old" {
		t.Fatalf("expected newest-first ordering, got %v, %v, %v", all[0].ID, all[1].ID, all[2].ID)
	}

	// Filter by SourceType.
	jobType := domain.SourceTypeJob
	jobsOnly, err := repo.ListRuns(ctx, "user-a", domain.RunFilter{SourceType: &jobType})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(jobsOnly) != 2 {
		t.Fatalf("expected 2 job runs, got %d", len(jobsOnly))
	}

	// Filter by Status.
	failed := domain.RunStatusFailed
	failedOnly, err := repo.ListRuns(ctx, "user-a", domain.RunFilter{Status: &failed})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(failedOnly) != 1 || failedOnly[0].ID != "r-chore" {
		t.Fatalf("expected only r-chore, got %v", failedOnly)
	}

	// Filter by date range.
	since := now.Add(-30 * time.Minute)
	recentOnly, err := repo.ListRuns(ctx, "user-a", domain.RunFilter{Since: &since})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(recentOnly) != 2 {
		t.Fatalf("expected 2 recent runs, got %d", len(recentOnly))
	}
}

func TestFileRunRepository_ListRuns_Empty(t *testing.T) {
	repo := newTestRunRepo(t)
	got, err := repo.ListRuns(context.Background(), "user-a", domain.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", got)
	}
}

func TestFileRunRepository_AppendLog(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()
	if err := repo.SaveRun(ctx, &domain.Run{ID: "r1", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := repo.AppendLog(ctx, "user-a", "r1", "line one\n"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	if err := repo.AppendLog(ctx, "user-a", "r1", "line two\n"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	data, err := os.ReadFile(repo.logPath("user-a", "r1"))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if string(data) != "line one\nline two\n" {
		t.Fatalf("unexpected log content: %q", data)
	}
}

func TestFileRunRepository_AppendLog_NotFound(t *testing.T) {
	repo := newTestRunRepo(t)
	err := repo.AppendLog(context.Background(), "user-a", "missing", "x")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileRunRepository_MarkNotifiedAndCountUnnotified(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()

	r1 := &domain.Run{ID: "r1", OwnerUserID: "user-a", Status: domain.RunStatusCompleted}
	r2 := &domain.Run{ID: "r2", OwnerUserID: "user-a", Status: domain.RunStatusFailed}
	r3 := &domain.Run{ID: "r3", OwnerUserID: "user-a", Status: domain.RunStatusRunning} // not terminal
	for _, run := range []*domain.Run{r1, r2, r3} {
		if err := repo.SaveRun(ctx, run); err != nil {
			t.Fatalf("SaveRun: %v", err)
		}
	}

	count, err := repo.CountUnnotified(ctx, "user-a")
	if err != nil {
		t.Fatalf("CountUnnotified: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unnotified terminal runs, got %d", count)
	}

	if err := repo.MarkNotified(ctx, "user-a", "r1"); err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}

	count, err = repo.CountUnnotified(ctx, "user-a")
	if err != nil {
		t.Fatalf("CountUnnotified: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unnotified run after marking r1, got %d", count)
	}

	got, err := repo.GetRun(ctx, "user-a", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.NotifiedAt == nil {
		t.Fatal("expected NotifiedAt to be set")
	}
}

func TestFileRunRepository_DeleteRun_RemovesLogToo(t *testing.T) {
	repo := newTestRunRepo(t)
	ctx := context.Background()
	if err := repo.SaveRun(ctx, &domain.Run{ID: "r1", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := repo.AppendLog(ctx, "user-a", "r1", "hello\n"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	if err := repo.DeleteRun(ctx, "user-a", "r1"); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
	if _, err := repo.GetRun(ctx, "user-a", "r1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if _, err := os.Stat(repo.logPath("user-a", "r1")); !os.IsNotExist(err) {
		t.Fatalf("expected log file to be removed, stat err: %v", err)
	}
}

func TestFileRunRepository_DeleteRun_NotFound(t *testing.T) {
	repo := newTestRunRepo(t)
	err := repo.DeleteRun(context.Background(), "user-a", "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
