package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

func newTestChoreRepo(t *testing.T) *FileChoreRepository {
	t.Helper()
	return NewFileChoreRepository(t.TempDir())
}

func TestFileChoreRepository_SaveAndGet(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	c := &domain.Chore{ID: "c1", OwnerUserID: "user-a", Title: "Nightly backup"}
	if err := repo.SaveChore(ctx, c); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}
	got, err := repo.GetChore(ctx, "user-a", "c1")
	if err != nil {
		t.Fatalf("GetChore: %v", err)
	}
	if got.Title != "Nightly backup" {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
}

func TestFileChoreRepository_CrossOwnerIsolation(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	if err := repo.SaveChore(ctx, &domain.Chore{ID: "shared", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}
	if _, err := repo.GetChore(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner get, got %v", err)
	}
	if err := repo.DeleteChore(ctx, "user-b", "shared"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}
	if err := repo.UpdateNextFireTime(ctx, "user-b", "shared", time.Now()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner update, got %v", err)
	}
}

// TestFileChoreRepository_GetChore_RejectsPathTraversal is a P7.1
// adversarial test mirroring TestFileJobRepository_GetJob_RejectsPathTraversal:
// choreID derives from a URL path segment, so a crafted value must never
// escape the calling user's own chores directory.
func TestFileChoreRepository_GetChore_RejectsPathTraversal(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()

	malicious := []string{
		"../../../etc/passwd",
		"..",
		"../bob/chores/some-chore",
		"chore/../../escape",
	}
	for _, id := range malicious {
		if _, err := repo.GetChore(ctx, "alice", id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("GetChore(%q): expected ErrNotFound, got %v", id, err)
		}
		if err := repo.DeleteChore(ctx, "alice", id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("DeleteChore(%q): expected ErrNotFound, got %v", id, err)
		}
	}
}

// TestFileChoreRepository_GetChore_CannotReadAnotherUsersRecordViaTraversal
// plants a real Chore for "bob" and confirms alice cannot read it via a
// crafted choreID that traverses out of her own directory and back into
// bob's.
func TestFileChoreRepository_GetChore_CannotReadAnotherUsersRecordViaTraversal(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()

	bobChore := &domain.Chore{ID: "legit-chore", OwnerUserID: "bob", Title: "Bob's private chore"}
	if err := repo.SaveChore(ctx, bobChore); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}

	craftedID := "../../bob/chores/legit-chore"
	if _, err := repo.GetChore(ctx, "alice", craftedID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner traversal via choreID %q, got err=%v (should never disclose bob's chore)", craftedID, err)
	}
}

func TestFileChoreRepository_ListChores(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	for _, id := range []string{"c1", "c2"} {
		if err := repo.SaveChore(ctx, &domain.Chore{ID: id, OwnerUserID: "user-a"}); err != nil {
			t.Fatalf("SaveChore: %v", err)
		}
	}
	got, err := repo.ListChores(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListChores: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 chores, got %d", len(got))
	}
}

func TestFileChoreRepository_UpdateNextFireTime(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	if err := repo.SaveChore(ctx, &domain.Chore{ID: "c1", OwnerUserID: "user-a"}); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}
	next := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	if err := repo.UpdateNextFireTime(ctx, "user-a", "c1", next); err != nil {
		t.Fatalf("UpdateNextFireTime: %v", err)
	}
	got, err := repo.GetChore(ctx, "user-a", "c1")
	if err != nil {
		t.Fatalf("GetChore: %v", err)
	}
	if !got.NextFireTime.Equal(next) {
		t.Fatalf("expected NextFireTime %v, got %v", next, got.NextFireTime)
	}
}

func TestFileChoreRepository_ListAllDue(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	now := time.Now()

	due := &domain.Chore{ID: "due", OwnerUserID: "user-a", ScheduleConfirmed: true, NextFireTime: now.Add(-time.Minute)}
	notYetDue := &domain.Chore{ID: "not-due", OwnerUserID: "user-a", ScheduleConfirmed: true, NextFireTime: now.Add(time.Hour)}
	unconfirmed := &domain.Chore{ID: "unconfirmed", OwnerUserID: "user-a", ScheduleConfirmed: false, NextFireTime: now.Add(-time.Hour)}
	otherUserDue := &domain.Chore{ID: "other-due", OwnerUserID: "user-b", ScheduleConfirmed: true, NextFireTime: now.Add(-time.Minute)}
	pendingDeletion := &domain.Chore{ID: "pending-del", OwnerUserID: "user-a", ScheduleConfirmed: true, NextFireTime: now.Add(-time.Minute), PendingDeletion: true}

	for _, c := range []*domain.Chore{due, notYetDue, unconfirmed, otherUserDue, pendingDeletion} {
		if err := repo.SaveChore(ctx, c); err != nil {
			t.Fatalf("SaveChore: %v", err)
		}
	}

	got, err := repo.ListAllDue(ctx, now)
	if err != nil {
		t.Fatalf("ListAllDue: %v", err)
	}

	ids := make(map[string]bool)
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["due"] || !ids["other-due"] {
		t.Fatalf("expected both cross-user due chores in result, got %v", ids)
	}
	if ids["not-due"] || ids["unconfirmed"] || ids["pending-del"] {
		t.Fatalf("expected not-due/unconfirmed/pending-deletion chores excluded, got %v", ids)
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 due chores, got %d", len(got))
	}
}

func TestFileChoreRepository_NotFoundCases(t *testing.T) {
	repo := newTestChoreRepo(t)
	ctx := context.Background()
	if _, err := repo.GetChore(ctx, "user-a", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := repo.DeleteChore(ctx, "user-a", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
