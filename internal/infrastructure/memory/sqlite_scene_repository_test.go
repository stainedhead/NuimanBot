package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"

	_ "modernc.org/sqlite"
)

// setupSceneTestDB creates a temporary test database with migrations applied
func setupSceneTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Apply migration
	migrationSQL, err := os.ReadFile("migrations/001_memory_tables.sql")
	if err != nil {
		t.Fatalf("Failed to read migration: %v", err)
	}

	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		t.Fatalf("Failed to apply migration: %v", err)
	}

	return db
}

// createTestScene creates a valid test memory scene
func createTestScene(sceneName string) *memoryv2.MemoryScene {
	return &memoryv2.MemoryScene{
		Scene:      sceneName,
		Summary:    "Test scene summary for " + sceneName,
		TokenCount: 100,
		UpdatedAt:  time.Now(),
	}
}

func TestSQLiteMemorySceneRepository_Upsert(t *testing.T) {
	db := setupSceneTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	t.Run("upsert_creates_new_scene", func(t *testing.T) {
		scene := createTestScene("project-setup")

		err := repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify scene was created
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_scenes WHERE scene = ?", scene.Scene).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 scene, got %d", count)
		}
	})

	t.Run("upsert_updates_existing_scene", func(t *testing.T) {
		scene := createTestScene("authentication")

		// Create first time
		err := repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("First upsert failed: %v", err)
		}

		// Update with new summary
		scene.Summary = "Updated summary for authentication"
		scene.TokenCount = 150
		scene.UpdatedAt = time.Now()

		err = repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Second upsert failed: %v", err)
		}

		// Verify scene was updated
		var summary string
		var tokenCount int
		err = db.QueryRow("SELECT summary, token_count FROM memory_scenes WHERE scene = ?", scene.Scene).Scan(&summary, &tokenCount)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if summary != scene.Summary {
			t.Errorf("Expected summary %s, got %s", scene.Summary, summary)
		}
		if tokenCount != scene.TokenCount {
			t.Errorf("Expected token count %d, got %d", scene.TokenCount, tokenCount)
		}

		// Verify only one row exists (update, not insert)
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_scenes WHERE scene = ?", scene.Scene).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 scene after upsert, got %d", count)
		}
	})

	t.Run("upsert_validates_scene", func(t *testing.T) {
		scene := &memoryv2.MemoryScene{
			Scene:      "", // Invalid empty scene
			Summary:    "Test",
			TokenCount: 100,
			UpdatedAt:  time.Now(),
		}

		err := repo.Upsert(ctx, scene)
		if err == nil {
			t.Error("Expected validation error for empty scene name, got nil")
		}
	})
}

func TestSQLiteMemorySceneRepository_Get(t *testing.T) {
	db := setupSceneTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	t.Run("get_existing_scene", func(t *testing.T) {
		scene := createTestScene("database-config")
		err := repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Failed to create scene: %v", err)
		}

		retrieved, err := repo.Get(ctx, scene.Scene)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if retrieved.Scene != scene.Scene {
			t.Errorf("Expected scene %s, got %s", scene.Scene, retrieved.Scene)
		}
		if retrieved.Summary != scene.Summary {
			t.Errorf("Expected summary %s, got %s", scene.Summary, retrieved.Summary)
		}
		if retrieved.TokenCount != scene.TokenCount {
			t.Errorf("Expected token count %d, got %d", scene.TokenCount, retrieved.TokenCount)
		}
	})

	t.Run("get_nonexistent_scene_returns_error", func(t *testing.T) {
		_, err := repo.Get(ctx, "nonexistent-scene")
		if err == nil {
			t.Error("Expected error for nonexistent scene, got nil")
		}
		if err != memoryv2.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got: %v", err)
		}
	})
}

func TestSQLiteMemorySceneRepository_List(t *testing.T) {
	db := setupSceneTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	t.Run("list_returns_all_scenes", func(t *testing.T) {
		// Create test scenes
		scenes := []*memoryv2.MemoryScene{
			createTestScene("scene-1"),
			createTestScene("scene-2"),
			createTestScene("scene-3"),
		}

		for _, scene := range scenes {
			err := repo.Upsert(ctx, scene)
			if err != nil {
				t.Fatalf("Failed to create scene: %v", err)
			}
		}

		results, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 scenes, got %d", len(results))
		}

		// Verify scenes are returned
		sceneNames := make(map[string]bool)
		for _, s := range results {
			sceneNames[s.Scene] = true
		}
		for _, expected := range scenes {
			if !sceneNames[expected.Scene] {
				t.Errorf("Expected scene %s not found in results", expected.Scene)
			}
		}
	})

	t.Run("list_returns_empty_when_no_scenes", func(t *testing.T) {
		// Clear database
		_, err := db.Exec("DELETE FROM memory_scenes")
		if err != nil {
			t.Fatalf("Failed to clear database: %v", err)
		}

		results, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if results == nil {
			t.Error("Expected empty slice, got nil")
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 scenes, got %d", len(results))
		}
	})
}

func TestSQLiteMemorySceneRepository_Delete(t *testing.T) {
	db := setupSceneTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	t.Run("delete_existing_scene", func(t *testing.T) {
		scene := createTestScene("to-delete")
		err := repo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Failed to create scene: %v", err)
		}

		err = repo.Delete(ctx, scene.Scene)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify scene was deleted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_scenes WHERE scene = ?", scene.Scene).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 scenes after delete, got %d", count)
		}
	})

	t.Run("delete_nonexistent_scene_returns_error", func(t *testing.T) {
		err := repo.Delete(ctx, "nonexistent-scene")
		if err == nil {
			t.Error("Expected error for nonexistent scene, got nil")
		}
		if err != memoryv2.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got: %v", err)
		}
	})
}
