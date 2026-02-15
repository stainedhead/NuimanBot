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

// setupTestDB creates a temporary test database with migrations applied
func setupTestDB(t *testing.T) *sql.DB {
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

// createTestCell creates a valid test memory cell with a UUID
func createTestCell() *memoryv2.MemoryCell {
	now := time.Now()
	return &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-test-123",
		Scene:          "test-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "Test memory cell content",
		Source:         `["msg-1", "msg-2"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      nil,
	}
}

func TestSQLiteMemoryCellRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	t.Run("create_valid_cell", func(t *testing.T) {
		cell := createTestCell()

		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify cell was created
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_cells WHERE id = ?", cell.ID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 cell, got %d", count)
		}
	})

	t.Run("create_duplicate_returns_error", func(t *testing.T) {
		cell := createTestCell()

		// Create first time
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("First create failed: %v", err)
		}

		// Try to create duplicate
		err = repo.Create(ctx, cell)
		if err == nil {
			t.Error("Expected error for duplicate ID, got nil")
		}
		if err != memoryv2.ErrAlreadyExists {
			t.Errorf("Expected ErrAlreadyExists, got: %v", err)
		}
	})
}

func TestSQLiteMemoryCellRepository_Get(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	t.Run("get_existing_cell", func(t *testing.T) {
		cell := createTestCell()
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}

		retrieved, err := repo.Get(ctx, cell.ID)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if retrieved.ID != cell.ID {
			t.Errorf("Expected ID %s, got %s", cell.ID, retrieved.ID)
		}
		if retrieved.Content != cell.Content {
			t.Errorf("Expected content %s, got %s", cell.Content, retrieved.Content)
		}
		if retrieved.Salience != cell.Salience {
			t.Errorf("Expected salience %f, got %f", cell.Salience, retrieved.Salience)
		}
	})

	t.Run("get_nonexistent_cell_returns_error", func(t *testing.T) {
		_, err := repo.Get(ctx, "nonexistent-id")
		if err == nil {
			t.Error("Expected error for nonexistent cell, got nil")
		}
		if err != memoryv2.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got: %v", err)
		}
	})
}

func TestSQLiteMemoryCellRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	t.Run("delete_existing_cell", func(t *testing.T) {
		cell := createTestCell()
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}

		err = repo.Delete(ctx, cell.ID)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify cell was deleted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_cells WHERE id = ?", cell.ID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 cells after delete, got %d", count)
		}
	})

	t.Run("delete_nonexistent_cell_returns_error", func(t *testing.T) {
		err := repo.Delete(ctx, "nonexistent-id")
		if err == nil {
			t.Error("Expected error for nonexistent cell, got nil")
		}
		if err != memoryv2.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got: %v", err)
		}
	})
}

func TestSQLiteMemoryCellRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create test cells
	cells := []*memoryv2.MemoryCell{
		createTestCell(),
		createTestCell(),
		createTestCell(),
	}
	cells[1].ConversationID = "conv-other"
	cells[2].Scene = "other-scene"

	for _, cell := range cells {
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("list_by_conversation_id", func(t *testing.T) {
		filter := memoryv2.MemoryCellFilter{
			ConversationID: "conv-test-123",
		}

		results, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells, got %d", len(results))
		}
	})

	t.Run("list_by_scene", func(t *testing.T) {
		filter := memoryv2.MemoryCellFilter{
			Scene: "test-scene",
		}

		results, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells, got %d", len(results))
		}
	})

	t.Run("list_with_limit", func(t *testing.T) {
		filter := memoryv2.MemoryCellFilter{
			Limit: 2,
		}

		results, err := repo.List(ctx, filter)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells (limit), got %d", len(results))
		}
	})
}

func TestSQLiteMemoryCellRepository_SearchFTS(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create test cells with different content
	cells := []*memoryv2.MemoryCell{
		createTestCell(),
		createTestCell(),
		createTestCell(),
	}
	cells[0].Content = "User decided to use authentication with OAuth"
	cells[1].Content = "System configuration for database connection"
	cells[2].Content = "Authentication flow requires token validation"

	for _, cell := range cells {
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("search_finds_matching_cells", func(t *testing.T) {
		results, err := repo.SearchFTS(ctx, "authentication", 10)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells matching 'authentication', got %d", len(results))
		}
	})

	t.Run("search_respects_limit", func(t *testing.T) {
		results, err := repo.SearchFTS(ctx, "authentication", 1)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 cell (limit), got %d", len(results))
		}
	})

	t.Run("search_no_matches_returns_empty", func(t *testing.T) {
		results, err := repo.SearchFTS(ctx, "nonexistent", 10)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 cells, got %d", len(results))
		}
	})
}

func TestSQLiteMemoryCellRepository_GetByScene(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create cells with different scenes and salience
	cells := []*memoryv2.MemoryCell{
		createTestCell(),
		createTestCell(),
		createTestCell(),
	}
	cells[0].Scene = "project-setup"
	cells[0].Salience = 0.9
	cells[1].Scene = "project-setup"
	cells[1].Salience = 0.7
	cells[2].Scene = "other-scene"
	cells[2].Salience = 0.95

	for _, cell := range cells {
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("get_by_scene_orders_by_salience", func(t *testing.T) {
		results, err := repo.GetByScene(ctx, "project-setup", 10)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells, got %d", len(results))
		}

		// Should be ordered by salience DESC
		if len(results) >= 2 && results[0].Salience < results[1].Salience {
			t.Error("Expected results ordered by salience DESC")
		}
	})
}

func TestSQLiteMemoryCellRepository_GetHighSalience(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create cells with different salience
	cells := []*memoryv2.MemoryCell{
		createTestCell(),
		createTestCell(),
		createTestCell(),
	}
	cells[0].Salience = 0.9
	cells[1].Salience = 0.6
	cells[2].Salience = 0.8

	for _, cell := range cells {
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("get_high_salience_filters_correctly", func(t *testing.T) {
		results, err := repo.GetHighSalience(ctx, "conv-test-123", 0.7, 10)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 cells with salience >= 0.7, got %d", len(results))
		}

		// Verify all results have salience >= threshold
		for _, cell := range results {
			if cell.Salience < 0.7 {
				t.Errorf("Expected salience >= 0.7, got %f", cell.Salience)
			}
		}
	})
}

func TestSQLiteMemoryCellRepository_DeleteExpired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteMemoryCellRepository(db)
	ctx := context.Background()

	// Create cells with different expiration
	now := time.Now()
	pastCreated := now.Add(-48 * time.Hour)  // Created 48 hours ago
	pastExpired := now.Add(-24 * time.Hour)  // Expired 24 hours ago
	futureExpires := now.Add(24 * time.Hour) // Expires in 24 hours

	// Create cells with custom timestamps to test expiration
	cells := []*memoryv2.MemoryCell{
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-test-123",
			Scene:          "test-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        "Expired cell",
			Source:         `["msg-1"]`,
			CreatedAt:      pastCreated,
			UpdatedAt:      pastCreated,
			ExpiresAt:      &pastExpired, // Expired
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-test-123",
			Scene:          "test-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        "Future expiry cell",
			Source:         `["msg-2"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
			ExpiresAt:      &futureExpires, // Not expired
		},
		{
			ID:             uuid.New().String(),
			ConversationID: "conv-test-123",
			Scene:          "test-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        "No expiry cell",
			Source:         `["msg-3"]`,
			CreatedAt:      now,
			UpdatedAt:      now,
			ExpiresAt:      nil, // No expiration
		},
	}

	for _, cell := range cells {
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell: %v", err)
		}
	}

	t.Run("delete_expired_removes_only_expired", func(t *testing.T) {
		count, err := repo.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 expired cell deleted, got %d", count)
		}

		// Verify only expired cell was deleted
		var remaining int
		err = db.QueryRow("SELECT COUNT(*) FROM memory_cells").Scan(&remaining)
		if err != nil {
			t.Fatalf("Failed to query database: %v", err)
		}
		if remaining != 2 {
			t.Errorf("Expected 2 cells remaining, got %d", remaining)
		}
	})
}
