package storage

import (
	"context"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileUserProfileRepository_SaveProfile(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	profile := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Alice"
	profile.LastName = "Anderson"
	profile.Moniker = "alice_admin"

	ctx := context.Background()
	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(usersJSONPath); os.IsNotExist(err) {
		t.Fatal("users.json was not created")
	}

	// Verify can retrieve profile
	retrieved, err := repo.GetProfileByUserID(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetProfileByUserID failed: %v", err)
	}

	if retrieved.FirstName != "Alice" {
		t.Errorf("expected FirstName Alice, got %s", retrieved.FirstName)
	}
}

func TestFileUserProfileRepository_GetProfileByEmail(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	profile := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)

	ctx := context.Background()
	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Retrieve by email
	retrieved, err := repo.GetProfileByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("GetProfileByEmail failed: %v", err)
	}

	if retrieved.UserID != "user-123" {
		t.Errorf("expected UserID user-123, got %s", retrieved.UserID)
	}
}

func TestFileUserProfileRepository_GetProfileByPlatformID(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	profile := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	profile.PlatformIDs = domain.PlatformIdentifiers{
		Slack: "U01ABC123",
	}

	ctx := context.Background()
	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Retrieve by Slack ID
	retrieved, err := repo.GetProfileByPlatformID(ctx, domain.PlatformSlack, "U01ABC123")
	if err != nil {
		t.Fatalf("GetProfileByPlatformID failed: %v", err)
	}

	if retrieved.UserID != "user-123" {
		t.Errorf("expected UserID user-123, got %s", retrieved.UserID)
	}
}

func TestFileUserProfileRepository_ListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	ctx := context.Background()

	// Create multiple profiles
	for i := 0; i < 5; i++ {
		profile := domain.NewUserProfile(
			string(rune('0'+i)),
			string(rune('a'+i))+"@example.com",
			domain.UserTypeIndividual,
		)
		err := repo.SaveProfile(ctx, profile)
		if err != nil {
			t.Fatalf("SaveProfile failed: %v", err)
		}
	}

	// List all profiles
	profiles, err := repo.ListProfiles(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}

	if len(profiles) != 5 {
		t.Errorf("expected 5 profiles, got %d", len(profiles))
	}
}

func TestFileUserProfileRepository_UpdateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	profile := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Alice"

	ctx := context.Background()
	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Update profile
	profile.FirstName = "Alicia"
	profile.UpdatedAt = time.Now()
	err = repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile (update) failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetProfileByUserID(ctx, "user-123")
	if err != nil {
		t.Fatalf("GetProfileByUserID failed: %v", err)
	}

	if retrieved.FirstName != "Alicia" {
		t.Errorf("expected FirstName Alicia, got %s", retrieved.FirstName)
	}
}

func TestFileUserProfileRepository_DeleteProfile(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	profile := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)

	ctx := context.Background()
	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Delete profile
	err = repo.DeleteProfile(ctx, "user-123")
	if err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Verify deletion
	_, err = repo.GetProfileByUserID(ctx, "user-123")
	if err == nil {
		t.Error("expected error when getting deleted profile")
	}
}
