package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// setupIntegrationTestDB creates a temporary test database with migrations applied
func setupIntegrationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "integration_test.db")

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

// TestIntegration_CellsAndScenes tests cells and scenes working together
func TestIntegration_CellsAndScenes(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cellRepo := NewSQLiteMemoryCellRepository(db)
	sceneRepo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	t.Run("create_scene_and_cells", func(t *testing.T) {
		// Create a scene
		scene := &memoryv2.MemoryScene{
			Scene:      "user-onboarding",
			Summary:    "User completed onboarding flow with preferences set",
			TokenCount: 250,
			UpdatedAt:  time.Now(),
		}

		err := sceneRepo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Failed to create scene: %v", err)
		}

		// Create cells for this scene
		cells := []*memoryv2.MemoryCell{
			{
				ID:             uuid.New().String(),
				ConversationID: "conv-123",
				Scene:          "user-onboarding",
				CellType:       memoryv2.CellTypeFact,
				Salience:       0.9,
				Content:        "User prefers dark mode",
				Source:         `["msg-1"]`,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			{
				ID:             uuid.New().String(),
				ConversationID: "conv-123",
				Scene:          "user-onboarding",
				CellType:       memoryv2.CellTypePreference,
				Salience:       0.85,
				Content:        "User wants notifications enabled",
				Source:         `["msg-2"]`,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			{
				ID:             uuid.New().String(),
				ConversationID: "conv-123",
				Scene:          "user-onboarding",
				CellType:       memoryv2.CellTypeDecision,
				Salience:       0.8,
				Content:        "User decided to skip tutorial",
				Source:         `["msg-3"]`,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		}

		for _, cell := range cells {
			err := cellRepo.Create(ctx, cell)
			if err != nil {
				t.Fatalf("Failed to create cell: %v", err)
			}
		}

		// Verify scene exists
		retrievedScene, err := sceneRepo.Get(ctx, "user-onboarding")
		if err != nil {
			t.Fatalf("Failed to get scene: %v", err)
		}
		if retrievedScene.Summary != scene.Summary {
			t.Errorf("Scene summary mismatch")
		}

		// Verify cells can be retrieved by scene
		sceneCells, err := cellRepo.GetByScene(ctx, "user-onboarding", 10)
		if err != nil {
			t.Fatalf("Failed to get cells by scene: %v", err)
		}
		if len(sceneCells) != 3 {
			t.Errorf("Expected 3 cells for scene, got %d", len(sceneCells))
		}

		// Verify cells are ordered by salience DESC
		if len(sceneCells) >= 2 && sceneCells[0].Salience < sceneCells[1].Salience {
			t.Error("Cells not ordered by salience DESC")
		}
	})

	t.Run("delete_scene_preserves_cells", func(t *testing.T) {
		// Create another scene with cells
		scene := &memoryv2.MemoryScene{
			Scene:      "bug-fixing",
			Summary:    "User reported and fixed authentication bug",
			TokenCount: 180,
			UpdatedAt:  time.Now(),
		}

		err := sceneRepo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Failed to create scene: %v", err)
		}

		cellID := uuid.New().String()
		cell := &memoryv2.MemoryCell{
			ID:             cellID,
			ConversationID: "conv-456",
			Scene:          "bug-fixing",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.75,
			Content:        "Authentication bug was in JWT validation",
			Source:         `["msg-10"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err = cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}

		// Delete the scene
		err = sceneRepo.Delete(ctx, "bug-fixing")
		if err != nil {
			t.Fatalf("Failed to delete scene: %v", err)
		}

		// Verify cell still exists (no CASCADE DELETE)
		retrievedCell, err := cellRepo.Get(ctx, cellID)
		if err != nil {
			t.Fatalf("Expected cell to still exist after scene deletion: %v", err)
		}
		if retrievedCell.Scene != "bug-fixing" {
			t.Error("Cell scene name should be preserved even after scene deletion")
		}
	})
}

// TestIntegration_FullTextSearch tests FTS across multiple cells and scenes
func TestIntegration_FullTextSearch(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cellRepo := NewSQLiteMemoryCellRepository(db)
	sceneRepo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	// Create multiple scenes
	scenes := []*memoryv2.MemoryScene{
		{
			Scene:      "authentication",
			Summary:    "User configured OAuth and JWT authentication",
			TokenCount: 200,
			UpdatedAt:  time.Now(),
		},
		{
			Scene:      "database-setup",
			Summary:    "User set up PostgreSQL database with migrations",
			TokenCount: 150,
			UpdatedAt:  time.Now(),
		},
		{
			Scene:      "api-development",
			Summary:    "User implemented REST API endpoints",
			TokenCount: 300,
			UpdatedAt:  time.Now(),
		},
	}

	for _, scene := range scenes {
		err := sceneRepo.Upsert(ctx, scene)
		if err != nil {
			t.Fatalf("Failed to create scene: %v", err)
		}
	}

	// Create cells across different scenes
	cells := []*memoryv2.MemoryCell{
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-789",
			Scene:          "authentication",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.9,
			Content:        "User decided to use OAuth2 for authentication",
			Source:         `["msg-20"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-789",
			Scene:          "authentication",
			CellType:       memoryv2.CellTypeDecision,
			Salience:       0.85,
			Content:        "JWT tokens will expire after 24 hours",
			Source:         `["msg-21"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-789",
			Scene:          "database-setup",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        "PostgreSQL database running on port 5432",
			Source:         `["msg-22"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-789",
			Scene:          "api-development",
			CellType:       memoryv2.CellTypeTask,
			Salience:       0.75,
			Content:        "Need to add authentication middleware to API endpoints",
			Source:         `["msg-23"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, cell := range cells {
		err := cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("search_finds_authentication_cells", func(t *testing.T) {
		results, err := cellRepo.SearchFTS(ctx, "authentication", 10)
		if err != nil {
			t.Fatalf("Failed to search: %v", err)
		}

		// Should find cells mentioning authentication (3 cells: OAuth, JWT, middleware)
		if len(results) < 2 {
			t.Errorf("Expected at least 2 cells with 'authentication', got %d", len(results))
		}

		// Verify results contain authentication-related content
		found := false
		for _, cell := range results {
			if cell.Content == "User decided to use OAuth2 for authentication" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find OAuth authentication cell in search results")
		}
	})

	t.Run("search_across_multiple_scenes", func(t *testing.T) {
		results, err := cellRepo.SearchFTS(ctx, "database", 10)
		if err != nil {
			t.Fatalf("Failed to search: %v", err)
		}

		if len(results) < 1 {
			t.Error("Expected at least 1 cell with 'database'")
		}
	})

	t.Run("list_all_scenes", func(t *testing.T) {
		allScenes, err := sceneRepo.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list scenes: %v", err)
		}

		if len(allScenes) < 3 {
			t.Errorf("Expected at least 3 scenes, got %d", len(allScenes))
		}

		// Verify scenes are ordered by updated_at DESC
		for i := 1; i < len(allScenes); i++ {
			if allScenes[i-1].UpdatedAt.Before(allScenes[i].UpdatedAt) {
				t.Error("Scenes not ordered by updated_at DESC")
			}
		}
	})
}

// TestIntegration_SalienceRanking tests salience-based retrieval
func TestIntegration_SalienceRanking(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cellRepo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create cells with varying salience
	cells := []*memoryv2.MemoryCell{
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-999",
			Scene:          "project-planning",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.95, // High salience
			Content:        "Critical: Project deadline is next Friday",
			Source:         `["msg-30"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-999",
			Scene:          "project-planning",
			CellType:       memoryv2.CellTypeDecision,
			Salience:       0.85, // Medium salience
			Content:        "Team decided to use agile methodology",
			Source:         `["msg-31"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-999",
			Scene:          "project-planning",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.65, // Lower salience
			Content:        "Meeting room is booked for Tuesday",
			Source:         `["msg-32"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, cell := range cells {
		err := cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("get_high_salience_cells", func(t *testing.T) {
		// Get cells with salience >= 0.8
		highSalienceCells, err := cellRepo.GetHighSalience(ctx, "conv-999", 0.8, 10)
		if err != nil {
			t.Fatalf("Failed to get high salience cells: %v", err)
		}

		if len(highSalienceCells) != 2 {
			t.Errorf("Expected 2 cells with salience >= 0.8, got %d", len(highSalienceCells))
		}

		// Verify cells are ordered by salience DESC
		for i := 1; i < len(highSalienceCells); i++ {
			if highSalienceCells[i-1].Salience < highSalienceCells[i].Salience {
				t.Error("High salience cells not ordered by salience DESC")
			}
		}

		// Verify highest salience cell is first
		if len(highSalienceCells) > 0 && highSalienceCells[0].Salience != 0.95 {
			t.Errorf("Expected highest salience cell (0.95) first, got %f", highSalienceCells[0].Salience)
		}
	})

	t.Run("scene_cells_ordered_by_salience", func(t *testing.T) {
		sceneCells, err := cellRepo.GetByScene(ctx, "project-planning", 10)
		if err != nil {
			t.Fatalf("Failed to get cells by scene: %v", err)
		}

		if len(sceneCells) != 3 {
			t.Errorf("Expected 3 cells for scene, got %d", len(sceneCells))
		}

		// Verify ordering by salience DESC
		for i := 1; i < len(sceneCells); i++ {
			if sceneCells[i-1].Salience < sceneCells[i].Salience {
				t.Error("Scene cells not ordered by salience DESC")
			}
		}
	})
}

// TestIntegration_ExpirationManagement tests cell expiration
func TestIntegration_ExpirationManagement(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cellRepo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	now := time.Now()
	pastCreated := now.Add(-48 * time.Hour)
	pastExpired := now.Add(-1 * time.Hour)
	futureExpires := now.Add(24 * time.Hour)

	// Create cells with different expiration states
	cells := []*memoryv2.MemoryCell{
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-exp",
			Scene:          "temporary-notes",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.7,
			Content:        "This cell has expired",
			Source:         `["msg-40"]`,
			CreatedAt:      pastCreated,
			UpdatedAt:      pastCreated,
			ExpiresAt:      &pastExpired,
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-exp",
			Scene:          "temporary-notes",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.75,
			Content:        "This cell will expire in the future",
			Source:         `["msg-41"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
			ExpiresAt:      &futureExpires,
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-exp",
			Scene:          "permanent-notes",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        "This cell never expires",
			Source:         `["msg-42"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
			ExpiresAt:      nil,
		},
	}

	for _, cell := range cells {
		err := cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("delete_expired_cells", func(t *testing.T) {
		// Before deletion - should have 3 cells
		filter := memoryv2.MemoryCellFilter{ConversationID: "conv-exp"}
		beforeCells, err := cellRepo.List(ctx, filter)
		if err != nil {
			t.Fatalf("Failed to list cells: %v", err)
		}
		if len(beforeCells) != 3 {
			t.Errorf("Expected 3 cells before expiration cleanup, got %d", len(beforeCells))
		}

		// Delete expired cells
		deletedCount, err := cellRepo.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("Failed to delete expired cells: %v", err)
		}

		if deletedCount != 1 {
			t.Errorf("Expected 1 expired cell deleted, got %d", deletedCount)
		}

		// After deletion - should have 2 cells (future expiry + never expires)
		afterCells, err := cellRepo.List(ctx, filter)
		if err != nil {
			t.Fatalf("Failed to list cells: %v", err)
		}
		if len(afterCells) != 2 {
			t.Errorf("Expected 2 cells after expiration cleanup, got %d", len(afterCells))
		}

		// Verify the expired cell was deleted
		for _, cell := range afterCells {
			if cell.Content == "This cell has expired" {
				t.Error("Expired cell should have been deleted")
			}
		}
	})
}
