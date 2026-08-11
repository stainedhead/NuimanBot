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

// TestFileJobRepository_GetJob_RejectsPathTraversal is a P7.1 adversarial
// test: jobID ultimately derives from a URL path segment
// (adapter/web/jobs_handler.go's jobIDAndActionFromPath), so a crafted
// value must never escape the calling user's own jobs directory. Before
// recordPath routed through fsguard.ResolveWithin, some of these reached
// arbitrary files elsewhere on disk.
func TestFileJobRepository_GetJob_RejectsPathTraversal(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()

	malicious := []string{
		"../../../etc/passwd",
		"..",
		"../bob/jobs/some-job",
		"job/../../escape",
	}
	for _, id := range malicious {
		if _, err := repo.GetJob(ctx, "alice", id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("GetJob(%q): expected ErrNotFound, got %v", id, err)
		}
		if err := repo.DeleteJob(ctx, "alice", id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("DeleteJob(%q): expected ErrNotFound, got %v", id, err)
		}
	}
}

// TestFileJobRepository_GetJob_CannotReadAnotherUsersRecordViaTraversal
// plants a real Job for "bob" and confirms alice cannot read it by crafting
// a jobID that traverses out of her own directory and back into bob's —
// the concrete cross-owner exploitation this confinement prevents, beyond
// the generic escape-to-arbitrary-file case above.
func TestFileJobRepository_GetJob_CannotReadAnotherUsersRecordViaTraversal(t *testing.T) {
	repo := newTestJobRepo(t)
	ctx := context.Background()

	bobJob := &domain.Job{ID: "legit-job", OwnerUserID: "bob", Title: "Bob's private job", Status: domain.JobStatusQueued}
	if err := repo.SaveJob(ctx, bobJob); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	craftedID := "../../bob/jobs/legit-job"
	if _, err := repo.GetJob(ctx, "alice", craftedID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner traversal via jobID %q, got err=%v (should never disclose bob's job)", craftedID, err)
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
