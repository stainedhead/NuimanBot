package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"

	"github.com/google/uuid"
)

func newTestUserRepo(t *testing.T) *storage.FileUserRepository {
	t.Helper()
	tempDir := t.TempDir()
	return storage.NewFileUserRepository(filepath.Join(tempDir, "users.json"))
}

func TestFileUserRepository_SaveAndGetByID(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	u := &domain.User{
		ID:          uuid.New().String(),
		Username:    "pubkey123",
		Role:        domain.RoleGuest,
		PlatformIDs: map[domain.Platform]string{domain.PlatformBuzz: "pubkey123"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}

	got, err := repo.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if got.ID != u.ID || got.Role != domain.RoleGuest {
		t.Errorf("GetUserByID() = %+v, want ID=%q Role=%q", got, u.ID, domain.RoleGuest)
	}
}

func TestFileUserRepository_GetUserByID_NotFound(t *testing.T) {
	repo := newTestUserRepo(t)
	_, err := repo.GetUserByID(context.Background(), "nonexistent")
	if err != domain.ErrUserNotFound {
		t.Errorf("GetUserByID() error = %v, want domain.ErrUserNotFound", err)
	}
}

func TestFileUserRepository_GetUserByPlatformID(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	u := &domain.User{
		ID:          uuid.New().String(),
		Role:        domain.RoleGuest,
		PlatformIDs: map[domain.Platform]string{domain.PlatformBuzz: "pubkey-abc"},
	}
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}

	got, err := repo.GetUserByPlatformID(ctx, domain.PlatformBuzz, "pubkey-abc")
	if err != nil {
		t.Fatalf("GetUserByPlatformID() error = %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("GetUserByPlatformID() ID = %q, want %q", got.ID, u.ID)
	}
}

func TestFileUserRepository_GetUserByPlatformID_NotFound(t *testing.T) {
	repo := newTestUserRepo(t)
	_, err := repo.GetUserByPlatformID(context.Background(), domain.PlatformBuzz, "no-such-pubkey")
	if err != domain.ErrUserNotFound {
		t.Errorf("GetUserByPlatformID() error = %v, want domain.ErrUserNotFound", err)
	}
}

func TestFileUserRepository_ListAll(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		u := &domain.User{ID: uuid.New().String(), Role: domain.RoleGuest}
		if err := repo.SaveUser(ctx, u); err != nil {
			t.Fatalf("SaveUser() error = %v", err)
		}
	}

	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListAll() returned %d users, want 3", len(all))
	}
}

func TestFileUserRepository_Delete(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	u := &domain.User{ID: uuid.New().String(), Role: domain.RoleGuest}
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := repo.GetUserByID(ctx, u.ID); err != domain.ErrUserNotFound {
		t.Errorf("GetUserByID() after Delete() error = %v, want domain.ErrUserNotFound", err)
	}
}

func TestFileUserRepository_Delete_NotFound(t *testing.T) {
	repo := newTestUserRepo(t)
	if err := repo.Delete(context.Background(), "nonexistent"); err != domain.ErrUserNotFound {
		t.Errorf("Delete() error = %v, want domain.ErrUserNotFound", err)
	}
}

func TestFileUserRepository_SaveUser_UpdatesExisting(t *testing.T) {
	repo := newTestUserRepo(t)
	ctx := context.Background()

	u := &domain.User{ID: uuid.New().String(), Role: domain.RoleGuest}
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}

	u.Role = domain.RoleAdmin
	if err := repo.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() (update) error = %v", err)
	}

	all, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAll() returned %d users, want 1 (update, not duplicate)", len(all))
	}
	if all[0].Role != domain.RoleAdmin {
		t.Errorf("Role = %q, want %q (update should persist)", all[0].Role, domain.RoleAdmin)
	}
}

func TestFileUserRepository_PersistsAcrossInstances(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "users.json")
	ctx := context.Background()

	repo1 := storage.NewFileUserRepository(filePath)
	u := &domain.User{ID: uuid.New().String(), Role: domain.RoleGuest, PlatformIDs: map[domain.Platform]string{domain.PlatformBuzz: "pk"}}
	if err := repo1.SaveUser(ctx, u); err != nil {
		t.Fatalf("SaveUser() error = %v", err)
	}

	repo2 := storage.NewFileUserRepository(filePath)
	got, err := repo2.GetUserByPlatformID(ctx, domain.PlatformBuzz, "pk")
	if err != nil {
		t.Fatalf("GetUserByPlatformID() on fresh instance error = %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got ID %q, want %q", got.ID, u.ID)
	}
}
