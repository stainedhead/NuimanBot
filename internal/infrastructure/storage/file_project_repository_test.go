package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

func newTestProjectRepo(t *testing.T) *FileProjectRepository {
	t.Helper()
	return NewFileProjectRepository(t.TempDir())
}

func TestFileProjectRepository_SaveAndGet(t *testing.T) {
	repo := newTestProjectRepo(t)
	ctx := context.Background()

	p := &domain.Project{
		ID:              "proj-1",
		OwnerUserID:     "user-a",
		Name:            "Widget",
		OutputDirectory: "/tmp/widget",
		HiddenDirectory: "/tmp/.widget",
		Retention:       domain.NeverExpire(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	got, err := repo.GetProject(ctx, "user-a", "proj-1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Widget" || got.OutputDirectory != "/tmp/widget" {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
}

func TestFileProjectRepository_GetNotFound(t *testing.T) {
	repo := newTestProjectRepo(t)
	_, err := repo.GetProject(context.Background(), "user-a", "does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileProjectRepository_CrossOwnerIsolation(t *testing.T) {
	// Edge Case #10 (IDOR): a project owned by user-a must be invisible to
	// user-b, even when user-b guesses the exact project ID, and must
	// resolve as ErrNotFound (not a distinct "forbidden" error) so
	// existence is never disclosed.
	repo := newTestProjectRepo(t)
	ctx := context.Background()

	p := &domain.Project{ID: "shared-id", OwnerUserID: "user-a", Name: "Secret"}
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	_, err := repo.GetProject(ctx, "user-b", "shared-id")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner access, got %v", err)
	}

	if err := repo.DeleteProject(ctx, "user-b", "shared-id"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-owner delete, got %v", err)
	}

	// The project must still exist for its real owner.
	if _, err := repo.GetProject(ctx, "user-a", "shared-id"); err != nil {
		t.Fatalf("expected project to still exist for owner: %v", err)
	}
}

func TestFileProjectRepository_ListProjects(t *testing.T) {
	repo := newTestProjectRepo(t)
	ctx := context.Background()

	empty, err := repo.ListProjects(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListProjects (empty): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected empty (non-nil) slice, got %v", empty)
	}

	for i, id := range []string{"p1", "p2"} {
		p := &domain.Project{ID: id, OwnerUserID: "user-a", Name: id, CreatedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := repo.SaveProject(ctx, p); err != nil {
			t.Fatalf("SaveProject: %v", err)
		}
	}
	// A different user's project must never appear in user-a's list.
	if err := repo.SaveProject(ctx, &domain.Project{ID: "p3", OwnerUserID: "user-b", Name: "not-mine"}); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	got, err := repo.ListProjects(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 projects for user-a, got %d", len(got))
	}
}

func TestFileProjectRepository_DeleteProject(t *testing.T) {
	repo := newTestProjectRepo(t)
	ctx := context.Background()

	p := &domain.Project{ID: "p1", OwnerUserID: "user-a", Name: "Widget"}
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if err := repo.DeleteProject(ctx, "user-a", "p1"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := repo.GetProject(ctx, "user-a", "p1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestFileProjectRepository_DeleteNotFound(t *testing.T) {
	repo := newTestProjectRepo(t)
	err := repo.DeleteProject(context.Background(), "user-a", "does-not-exist")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
