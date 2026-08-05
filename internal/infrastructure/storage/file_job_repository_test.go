package storage

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

func newTestJobRepo(t *testing.T) *FileJobRepository {
	t.Helper()
	return NewFileJobRepository(t.TempDir())
}

func TestFileJobRepository_SaveAndGet(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()

	j := &domain.Job{ID: "job-1", OwnerUserID: "user-a", Title: "Do the thing", Status: domain.JobStatusQueued}
	if err := repo.SaveJob(ctx, j); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	got, err := repo.GetJob(ctx, "user-a", "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Title != "Do the thing" || got.Status != domain.JobStatusQueued {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
}

func TestFileJobRepository_CrossOwnerIsolation(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()
	if err := repo.SaveJob(ctx, &domain.Job{ID: "shared", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	if _, err := repo.GetJob(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner get, got %v", err)
	}
	if err := repo.DeleteJob(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}
	if err := repo.UpdateStatus(ctx, "user-b", "shared", domain.JobStatusFailed); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner status update, got %v", err)
	}
}

func TestFileJobRepository_ListJobs(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()

	empty, err := repo.ListJobs(ctx, "user-a")
	if err != nil || len(empty) != 0 {
		t.Fatalf("expected empty slice, got %v, err %v", empty, err)
	}

	for _, id := range []string{"j1", "j2", "j3"} {
		if err := repo.SaveJob(ctx, &domain.Job{ID: id, OwnerUserID: "user-a"}); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
	}
	if err := repo.SaveJob(ctx, &domain.Job{ID: "j-other", OwnerUserID: "user-b"}); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	got, err := repo.ListJobs(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(got))
	}
}

func TestFileJobRepository_UpdateStatus(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()
	if err := repo.SaveJob(ctx, &domain.Job{ID: "j1", OwnerUserID: "user-a", Status: domain.JobStatusQueued}); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	if err := repo.UpdateStatus(ctx, "user-a", "j1", domain.JobStatusRunning); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := repo.GetJob(ctx, "user-a", "j1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Status != domain.JobStatusRunning {
		t.Fatalf("expected status Running, got %v", got.Status)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set by UpdateStatus")
	}
}

func TestFileJobRepository_DeleteJob(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()
	if err := repo.SaveJob(ctx, &domain.Job{ID: "j1", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	if err := repo.DeleteJob(ctx, "user-a", "j1"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, err := repo.GetJob(ctx, "user-a", "j1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFileJobRepository_NotFoundCases(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()
	if _, err := repo.GetJob(ctx, "user-a", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := repo.DeleteJob(ctx, "user-a", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := repo.UpdateStatus(ctx, "user-a", "missing", domain.JobStatusFailed); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
