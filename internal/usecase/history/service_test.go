package history

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

func newTestService(t *testing.T) (*Service, domain.RunRepository) {
	t.Helper()
	repo := storage.NewFileRunRepository(t.TempDir())
	return NewService(repo), repo
}

func mustSave(t *testing.T, repo domain.RunRepository, run *domain.Run) {
	t.Helper()
	if err := repo.SaveRun(context.Background(), run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
}

func TestListRuns_FilterPassthrough(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	now := time.Now()

	mustSave(t, repo, &domain.Run{ID: "job-run", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour)})
	mustSave(t, repo, &domain.Run{ID: "chore-run", OwnerUserID: "alice", SourceType: domain.SourceTypeChore, SourceID: "chore-1", Status: domain.RunStatusFailed, CreatedAt: now})

	jobType := domain.SourceTypeJob
	got, err := svc.ListRuns(ctx, "alice", domain.RunFilter{SourceType: &jobType})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(got) != 1 || got[0].ID != "job-run" {
		t.Fatalf("expected filter to reach the repository, got %+v", got)
	}

	since := now.Add(-1 * time.Hour)
	got, err = svc.ListRuns(ctx, "alice", domain.RunFilter{Since: &since})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(got) != 1 || got[0].ID != "chore-run" {
		t.Fatalf("expected date-range filter to reach the repository, got %+v", got)
	}
}

func TestListRuns_CrossOwnerIsolation(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "bob", Status: domain.RunStatusCompleted, CreatedAt: time.Now()})

	got, err := svc.ListRuns(ctx, "alice", domain.RunFilter{})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no runs visible to a different owner, got %+v", got)
	}
}

func TestGetRun_CrossOwnerIsolation(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "bob"})

	if _, err := svc.GetRun(ctx, "alice", "r1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner GetRun, got %v", err)
	}
}

func TestGetRun_Success(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "alice", SourceID: "job-1"})

	got, err := svc.GetRun(ctx, "alice", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.SourceID != "job-1" {
		t.Fatalf("unexpected run: %+v", got)
	}
}

func TestMarkViewed_CrossOwnerIsolation(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "bob", Status: domain.RunStatusCompleted})

	if err := svc.MarkViewed(ctx, "alice", "r1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner MarkViewed, got %v", err)
	}
	got, err := repo.GetRun(ctx, "bob", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.NotifiedAt != nil {
		t.Fatal("expected bob's run to remain unviewed after alice's failed MarkViewed attempt")
	}
}

func TestMarkViewed_Success(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "alice", Status: domain.RunStatusCompleted})

	if err := svc.MarkViewed(ctx, "alice", "r1"); err != nil {
		t.Fatalf("MarkViewed: %v", err)
	}
	got, err := repo.GetRun(ctx, "alice", "r1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.NotifiedAt == nil {
		t.Fatal("expected NotifiedAt to be set after MarkViewed")
	}
}

func TestUnviewedCount_CrossOwnerIsolation(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "bob", Status: domain.RunStatusCompleted})
	mustSave(t, repo, &domain.Run{ID: "r2", OwnerUserID: "alice", Status: domain.RunStatusCompleted})

	count, err := svc.UnviewedCount(ctx, "alice")
	if err != nil {
		t.Fatalf("UnviewedCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected alice's unviewed count to exclude bob's run, got %d", count)
	}
}

func TestSweepExpired_NeverPolicyDeletesNothing(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	old := time.Now().Add(-1000 * time.Hour)
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: old})

	deleted, err := svc.SweepExpired(ctx, "alice", domain.NeverExpire(), time.Now())
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deletions under Never policy, got %d", deleted)
	}
	if _, err := repo.GetRun(ctx, "alice", "r1"); err != nil {
		t.Fatalf("expected run to survive, GetRun err: %v", err)
	}
}

func TestSweepExpired_NotYetExpiredSurvives(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(24 * time.Hour)
	mustSave(t, repo, &domain.Run{ID: "r1", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-1 * time.Hour)})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deletions for a not-yet-expired run, got %d", deleted)
	}
	if _, err := repo.GetRun(ctx, "alice", "r1"); err != nil {
		t.Fatalf("expected run to survive, GetRun err: %v", err)
	}
}

func TestSweepExpired_ExactBoundaryAndPastBoundaryDeleted(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(24 * time.Hour)

	mustSave(t, repo, &domain.Run{ID: "at-boundary", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-24 * time.Hour)})
	mustSave(t, repo, &domain.Run{ID: "past-boundary", OwnerUserID: "alice", Status: domain.RunStatusFailed, CreatedAt: now.Add(-48 * time.Hour)})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deletions (at-boundary and past-boundary), got %d", deleted)
	}
	if _, err := repo.GetRun(ctx, "alice", "at-boundary"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected at-boundary run to be deleted, got err: %v", err)
	}
	if _, err := repo.GetRun(ctx, "alice", "past-boundary"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected past-boundary run to be deleted, got err: %v", err)
	}
}

func TestSweepExpired_NonTerminalRunsNeverDeleted(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(1 * time.Hour)
	mustSave(t, repo, &domain.Run{ID: "still-running", OwnerUserID: "alice", Status: domain.RunStatusRunning, CreatedAt: now.Add(-1000 * time.Hour)})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected non-terminal runs to never be swept, got %d deletions", deleted)
	}
	if _, err := repo.GetRun(ctx, "alice", "still-running"); err != nil {
		t.Fatalf("expected in-flight run to survive, GetRun err: %v", err)
	}
}

func TestSweepExpired_CrossOwnerIsolation(t *testing.T) {
	svc, repo := newTestService(t)
	ctx := context.Background()
	old := time.Now().Add(-1000 * time.Hour)
	mustSave(t, repo, &domain.Run{ID: "bobs-run", OwnerUserID: "bob", Status: domain.RunStatusCompleted, CreatedAt: old})

	deleted, err := svc.SweepExpired(ctx, "alice", domain.NewRetentionPolicy(time.Hour), time.Now())
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected sweeping alice to not touch bob's runs, got %d deletions", deleted)
	}
	if _, err := repo.GetRun(ctx, "bob", "bobs-run"); err != nil {
		t.Fatalf("expected bob's run to survive alice's sweep, GetRun err: %v", err)
	}
}

// markNotifiedThenDeleteSpy wraps a domain.RunRepository and records the
// order in which MarkNotified/DeleteRun are invoked for a given run, so
// Edge Case #7 (mark-before-delete for unviewed runs) can be verified
// directly rather than inferred.
type markNotifiedThenDeleteSpy struct {
	domain.RunRepository
	calls []string
}

func (s *markNotifiedThenDeleteSpy) MarkNotified(ctx context.Context, ownerUserID, runID string) error {
	s.calls = append(s.calls, "MarkNotified:"+runID)
	return s.RunRepository.MarkNotified(ctx, ownerUserID, runID)
}

func (s *markNotifiedThenDeleteSpy) DeleteRun(ctx context.Context, ownerUserID, runID string) error {
	s.calls = append(s.calls, "DeleteRun:"+runID)
	return s.RunRepository.DeleteRun(ctx, ownerUserID, runID)
}

func TestSweepExpired_UnviewedRun_MarksNotifiedBeforeDelete(t *testing.T) {
	base := storage.NewFileRunRepository(t.TempDir())
	spy := &markNotifiedThenDeleteSpy{RunRepository: base}
	svc := NewService(spy)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(time.Hour)

	// Unviewed (NotifiedAt nil) terminal run, expired.
	mustSave(t, base, &domain.Run{ID: "unviewed-expired", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour)})
	// Already-viewed terminal run, expired — MarkNotified must NOT be called again for this one.
	viewedAt := now.Add(-2 * time.Hour)
	mustSave(t, base, &domain.Run{ID: "viewed-expired", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour), NotifiedAt: &viewedAt})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected both expired runs deleted, got %d", deleted)
	}

	wantOrder := []string{"MarkNotified:unviewed-expired", "DeleteRun:unviewed-expired", "DeleteRun:viewed-expired"}
	if len(spy.calls) != len(wantOrder) {
		t.Fatalf("expected calls %v, got %v", wantOrder, spy.calls)
	}
	for i, want := range wantOrder {
		if spy.calls[i] != want {
			t.Fatalf("expected call %d to be %q, got %q (full sequence: %v)", i, want, spy.calls[i], spy.calls)
		}
	}

	// The badge count must never reference a deleted run.
	count, err := svc.UnviewedCount(ctx, "alice")
	if err != nil {
		t.Fatalf("UnviewedCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected badge count to be 0 after sweeping the only unviewed run, got %d", count)
	}
}

// failingRepo wraps a domain.RunRepository and forces MarkNotified and/or
// DeleteRun to fail for a specific run ID, so SweepExpired's best-effort
// continue-on-error paths can be exercised directly.
type failingRepo struct {
	domain.RunRepository
	failMarkNotifiedFor string
	failDeleteFor       string
}

func (f *failingRepo) MarkNotified(ctx context.Context, ownerUserID, runID string) error {
	if runID == f.failMarkNotifiedFor {
		return errors.New("simulated MarkNotified failure")
	}
	return f.RunRepository.MarkNotified(ctx, ownerUserID, runID)
}

func (f *failingRepo) DeleteRun(ctx context.Context, ownerUserID, runID string) error {
	if runID == f.failDeleteFor {
		return errors.New("simulated DeleteRun failure")
	}
	return f.RunRepository.DeleteRun(ctx, ownerUserID, runID)
}

func TestSweepExpired_MarkNotifiedFailure_SkipsRunButContinuesSweep(t *testing.T) {
	base := storage.NewFileRunRepository(t.TempDir())
	repo := &failingRepo{RunRepository: base, failMarkNotifiedFor: "unviewed-fails"}
	svc := NewService(repo)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(time.Hour)

	mustSave(t, base, &domain.Run{ID: "unviewed-fails", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour)})
	mustSave(t, base, &domain.Run{ID: "other-expired", OwnerUserID: "alice", Status: domain.RunStatusFailed, CreatedAt: now.Add(-2 * time.Hour)})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected only the unaffected run to be deleted, got %d", deleted)
	}
	if _, err := base.GetRun(ctx, "alice", "unviewed-fails"); err != nil {
		t.Fatalf("expected run whose MarkNotified failed to survive (not deleted), GetRun err: %v", err)
	}
	if _, err := base.GetRun(ctx, "alice", "other-expired"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected other-expired to be deleted, got %v", err)
	}
}

func TestSweepExpired_DeleteFailure_ContinuesSweepingRemainingRuns(t *testing.T) {
	base := storage.NewFileRunRepository(t.TempDir())
	repo := &failingRepo{RunRepository: base, failDeleteFor: "delete-fails"}
	svc := NewService(repo)
	ctx := context.Background()
	now := time.Now()
	policy := domain.NewRetentionPolicy(time.Hour)

	mustSave(t, base, &domain.Run{ID: "delete-fails", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: now.Add(-2 * time.Hour)})
	mustSave(t, base, &domain.Run{ID: "deletes-fine", OwnerUserID: "alice", Status: domain.RunStatusFailed, CreatedAt: now.Add(-2 * time.Hour)})

	deleted, err := svc.SweepExpired(ctx, "alice", policy, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected only the unaffected run to be deleted, got %d", deleted)
	}
	if _, err := base.GetRun(ctx, "alice", "delete-fails"); err != nil {
		t.Fatalf("expected run whose DeleteRun failed to survive, GetRun err: %v", err)
	}
	if _, err := base.GetRun(ctx, "alice", "deletes-fine"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected deletes-fine to be deleted, got %v", err)
	}
}

func TestSweepExpired_ListRunsErrorPropagates(t *testing.T) {
	svc := NewService(&failingListRepo{err: errors.New("boom")})
	if _, err := svc.SweepExpired(context.Background(), "alice", domain.NewRetentionPolicy(time.Hour), time.Now()); err == nil {
		t.Fatal("expected SweepExpired to propagate the repository error")
	}
}

func TestListRuns_RepositoryError(t *testing.T) {
	svc := NewService(&failingListRepo{err: errors.New("boom")})
	if _, err := svc.ListRuns(context.Background(), "alice", domain.RunFilter{}); err == nil {
		t.Fatal("expected ListRuns to propagate the repository error")
	}
}

type failingListRepo struct {
	domain.RunRepository
	err error
}

func (f *failingListRepo) ListRuns(context.Context, string, domain.RunFilter) ([]*domain.Run, error) {
	return nil, f.err
}
