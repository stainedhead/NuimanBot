package storage

import (
	"context"
	"nuimanbot/internal/domain/memoryv2"
	"path/filepath"
	"testing"
	"time"
)

func TestFileMemorySceneRepository_Upsert(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	scene := &memoryv2.MemoryScene{
		Scene:      "project-setup",
		Summary:    "User is setting up a new project with Go and PostgreSQL",
		TokenCount: 50,
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	err := repo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Verify can retrieve scene
	retrieved, err := repo.Get(ctx, "project-setup")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Summary != scene.Summary {
		t.Errorf("expected summary %s, got %s", scene.Summary, retrieved.Summary)
	}

	// Update scene
	scene.Summary = "Updated: User is setting up a new project with Go, PostgreSQL, and Redis"
	scene.TokenCount = 60
	scene.UpdatedAt = time.Now()

	err = repo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Upsert (update) failed: %v", err)
	}

	// Verify update
	retrieved, err = repo.Get(ctx, "project-setup")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Summary != scene.Summary {
		t.Errorf("expected updated summary, got %s", retrieved.Summary)
	}
	if retrieved.TokenCount != 60 {
		t.Errorf("expected token count 60, got %d", retrieved.TokenCount)
	}
}

func TestFileMemorySceneRepository_Get(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	scene := &memoryv2.MemoryScene{
		Scene:      "user-preferences",
		Summary:    "User prefers dark mode and vim keybindings",
		TokenCount: 30,
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	err := repo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Get scene
	retrieved, err := repo.Get(ctx, "user-preferences")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Scene != "user-preferences" {
		t.Errorf("expected scene user-preferences, got %s", retrieved.Scene)
	}
}

func TestFileMemorySceneRepository_GetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	ctx := context.Background()
	_, err := repo.Get(ctx, "nonexistent-scene")
	if err == nil {
		t.Error("expected error for nonexistent scene")
	}
}

func TestFileMemorySceneRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	ctx := context.Background()

	// Create multiple scenes
	scenes := []*memoryv2.MemoryScene{
		{
			Scene:      "project-setup",
			Summary:    "Summary A",
			TokenCount: 10,
			UpdatedAt:  time.Now(),
		},
		{
			Scene:      "user-preferences",
			Summary:    "Summary B",
			TokenCount: 20,
			UpdatedAt:  time.Now(),
		},
		{
			Scene:      "task-planning",
			Summary:    "Summary C",
			TokenCount: 30,
			UpdatedAt:  time.Now(),
		},
	}

	for _, scene := range scenes {
		err := repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
	}

	// List all scenes
	retrieved, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("expected 3 scenes, got %d", len(retrieved))
	}
}

func TestFileMemorySceneRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	scene := &memoryv2.MemoryScene{
		Scene:      "temporary-scene",
		Summary:    "This scene will be deleted",
		TokenCount: 15,
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	err := repo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Verify scene exists
	_, err = repo.Get(ctx, "temporary-scene")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Delete scene
	err = repo.Delete(ctx, "temporary-scene")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify scene is gone
	_, err = repo.Get(ctx, "temporary-scene")
	if err == nil {
		t.Error("expected error when getting deleted scene")
	}
}

func TestFileMemorySceneRepository_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemorySceneRepository(basePath)

	ctx := context.Background()
	err := repo.Delete(ctx, "nonexistent-scene")
	if err == nil {
		t.Error("expected error when deleting nonexistent scene")
	}
}
