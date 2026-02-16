package storage

import (
	"context"
	"nuimanbot/internal/domain/memoryv2"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFileMemoryCellRepository_Create(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	cell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-123",
		Scene:          "project-setup",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "User prefers tabs over spaces",
		Source:         `["msg-1", "msg-2"]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, cell)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify can retrieve cell
	retrieved, err := repo.Get(ctx, cell.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Content != "User prefers tabs over spaces" {
		t.Errorf("expected content match, got %s", retrieved.Content)
	}
}

func TestFileMemoryCellRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	// Create multiple cells
	for i := 0; i < 3; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-123",
			Scene:          "project-setup",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			Content:        "Content " + string(rune('A'+i)),
			Source:         `[]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List cells
	filter := memoryv2.MemoryCellFilter{
		ConversationID: "conv-123",
	}
	cells, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(cells) != 3 {
		t.Errorf("expected 3 cells, got %d", len(cells))
	}
}

func TestFileMemoryCellRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	cell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-123",
		Scene:          "project-setup",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "Test content",
		Source:         `[]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, cell)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete cell
	err = repo.Delete(ctx, cell.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = repo.Get(ctx, cell.ID)
	if err == nil {
		t.Error("expected error when getting deleted cell")
	}
}

func TestFileMemoryCellRepository_GetByScene(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	// Create cells in different scenes
	scenes := []string{"project-setup", "user-preferences", "project-setup"}
	saliences := []float64{0.9, 0.5, 0.7}

	for i := 0; i < 3; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-123",
			Scene:          scenes[i],
			CellType:       memoryv2.CellTypeFact,
			Salience:       saliences[i],
			Content:        "Content " + string(rune('A'+i)),
			Source:         `[]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Get by scene
	cells, err := repo.GetByScene(ctx, "project-setup", 10)
	if err != nil {
		t.Fatalf("GetByScene failed: %v", err)
	}

	if len(cells) != 2 {
		t.Errorf("expected 2 cells, got %d", len(cells))
	}

	// Verify sorted by salience descending
	if cells[0].Salience < cells[1].Salience {
		t.Error("expected cells sorted by salience descending")
	}
}

func TestFileMemoryCellRepository_GetHighSalience(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	// Create cells with different saliences
	saliences := []float64{0.9, 0.5, 0.3, 0.7}

	for i := 0; i < 4; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-123",
			Scene:          "project-setup",
			CellType:       memoryv2.CellTypeFact,
			Salience:       saliences[i],
			Content:        "Content",
			Source:         `[]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Get high salience cells (>= 0.7)
	cells, err := repo.GetHighSalience(ctx, "conv-123", 0.7, 10)
	if err != nil {
		t.Fatalf("GetHighSalience failed: %v", err)
	}

	if len(cells) != 2 {
		t.Errorf("expected 2 cells with salience >= 0.7, got %d", len(cells))
	}

	// Verify all returned cells meet threshold
	for _, cell := range cells {
		if cell.Salience < 0.7 {
			t.Errorf("cell salience %f below threshold 0.7", cell.Salience)
		}
	}
}

func TestFileMemoryCellRepository_SearchFTS(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	// Create cells with different content
	contents := []string{
		"User prefers tabs over spaces for indentation",
		"User likes the color blue for UI themes",
		"User wants to use Python for data analysis",
	}

	for i, content := range contents {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "conv-" + string(rune('1'+i)),
			Scene:          "user-preferences",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        content,
			Source:         `[]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Search for "user prefers"
	cells, err := repo.SearchFTS(ctx, "user prefers", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}

	if len(cells) == 0 {
		t.Error("expected at least one result for 'user prefers'")
	}

	// Verify result contains expected content
	found := false
	for _, cell := range cells {
		if cell.Content == contents[0] {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find cell with content about tabs")
	}
}

func TestFileMemoryCellRepository_DeleteExpired(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	pastExpiry := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	// Create expired cell (CreatedAt before ExpiresAt, but ExpiresAt is in the past)
	expiredCell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-123",
		Scene:          "project-setup",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "Expired content",
		Source:         `[]`,
		CreatedAt:      pastTime,
		UpdatedAt:      pastTime,
		ExpiresAt:      &pastExpiry,
	}
	err := repo.Create(ctx, expiredCell)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create non-expired cell
	validCell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-123",
		Scene:          "project-setup",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "Valid content",
		Source:         `[]`,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      &futureTime,
	}
	err = repo.Create(ctx, validCell)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete expired
	count, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 expired cell, got %d", count)
	}

	// Verify expired cell is gone
	_, err = repo.Get(ctx, expiredCell.ID)
	if err == nil {
		t.Error("expected error when getting expired cell")
	}

	// Verify valid cell still exists
	_, err = repo.Get(ctx, validCell.ID)
	if err != nil {
		t.Errorf("valid cell should still exist: %v", err)
	}
}
