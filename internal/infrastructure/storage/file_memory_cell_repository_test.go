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

func TestFileMemoryCellRepository_List_FilterByScene(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	makeCell := func(scene string, cellType memoryv2.CellType, salience float64) *memoryv2.MemoryCell {
		return &memoryv2.MemoryCell{
			ID: uuid.New().String(), ConversationID: "conv-1",
			Scene: scene, CellType: cellType, Salience: salience,
			Content: "Content for " + scene, Source: `[]`,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
	}

	// Create cells in two scenes with two types
	for _, c := range []*memoryv2.MemoryCell{
		makeCell("project-setup", memoryv2.CellTypeFact, 0.9),
		makeCell("project-setup", memoryv2.CellTypeDecision, 0.5),
		makeCell("user-prefs", memoryv2.CellTypeFact, 0.8),
	} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Filter by scene
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{Scene: "project-setup"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells for scene project-setup, got %d", len(cells))
	}
}

func TestFileMemoryCellRepository_List_FilterByCellType(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for i, ct := range []memoryv2.CellType{memoryv2.CellTypeFact, memoryv2.CellTypeDecision, memoryv2.CellTypeFact} {
		c := &memoryv2.MemoryCell{
			ID: uuid.New().String(), ConversationID: "conv-1",
			Scene: "test-scene", CellType: ct, Salience: float64(i+1) * 0.3,
			Content: "Content", Source: `[]`,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	factType := memoryv2.CellTypeFact
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{CellType: &factType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 fact cells, got %d", len(cells))
	}
	for _, c := range cells {
		if c.CellType != memoryv2.CellTypeFact {
			t.Errorf("unexpected cell type %v", c.CellType)
		}
	}
}

func TestFileMemoryCellRepository_List_FilterByMinSalience(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for _, sal := range []float64{0.9, 0.5, 0.2} {
		c := &memoryv2.MemoryCell{
			ID: uuid.New().String(), ConversationID: "conv-1",
			Scene: "test-scene", CellType: memoryv2.CellTypeFact, Salience: sal,
			Content: "Content", Source: `[]`,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	threshold := 0.5
	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{MinSalience: &threshold})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells with salience >= 0.5, got %d", len(cells))
	}
	for _, c := range cells {
		if c.Salience < threshold {
			t.Errorf("cell salience %f below threshold %f", c.Salience, threshold)
		}
	}
}

func TestFileMemoryCellRepository_List_LimitAndOffset(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		c := &memoryv2.MemoryCell{
			ID: uuid.New().String(), ConversationID: "conv-1",
			Scene: "test-scene", CellType: memoryv2.CellTypeFact, Salience: 0.5,
			Content: "Content", Source: `[]`,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	cells, err := repo.List(ctx, memoryv2.MemoryCellFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List with Limit: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells with limit=2, got %d", len(cells))
	}

	cells, err = repo.List(ctx, memoryv2.MemoryCellFilter{Offset: 3})
	if err != nil {
		t.Fatalf("List with Offset: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells with offset=3 (5 total), got %d", len(cells))
	}
}

func TestFileMemoryCellRepository_Update(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	cell := &memoryv2.MemoryCell{
		ID: uuid.New().String(), ConversationID: "conv-1",
		Scene: "test-scene", CellType: memoryv2.CellTypeFact, Salience: 0.5,
		Content: "Original content", Source: `[]`,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, cell); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update the cell
	updated := *cell
	updated.Content = "Updated content"
	updated.Salience = 0.9
	updated.UpdatedAt = time.Now()
	if err := repo.Update(ctx, &updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify the update persisted
	got, err := repo.Get(ctx, cell.ID)
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if got.Content != "Updated content" {
		t.Errorf("expected updated content, got %q", got.Content)
	}
	if got.Salience != 0.9 {
		t.Errorf("expected salience 0.9, got %f", got.Salience)
	}

	// Verify search index reflects new content
	results, err := repo.SearchFTS(ctx, "updated", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search to find updated content")
	}

	// Verify old content is no longer indexed
	oldResults, err := repo.SearchFTS(ctx, "original", 10)
	if err != nil {
		t.Fatalf("SearchFTS for old content: %v", err)
	}
	if len(oldResults) > 0 {
		t.Error("expected old content to be removed from search index")
	}
}

func TestFileMemoryCellRepository_Update_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	cell := &memoryv2.MemoryCell{
		ID: uuid.New().String(), ConversationID: "conv-1",
		Scene: "test-scene", CellType: memoryv2.CellTypeFact, Salience: 0.5,
		Content: "Content", Source: `[]`,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	err := repo.Update(ctx, cell)
	if err == nil {
		t.Error("expected error when updating non-existent cell")
	}
}

func TestFileMemoryCellRepository_GetHighSalience_Global(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewFileMemoryCellRepository(filepath.Join(tmpDir, "data"))
	ctx := context.Background()

	// Create cells across two conversations
	for i, convID := range []string{"conv-a", "conv-b"} {
		c := &memoryv2.MemoryCell{
			ID: uuid.New().String(), ConversationID: convID,
			Scene: "test-scene", CellType: memoryv2.CellTypeFact,
			Salience: float64(i+1) * 0.4, // 0.4 and 0.8
			Content:  "Content for " + convID, Source: `[]`,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Global search (empty conversationID) — should return both high-salience cells
	cells, err := repo.GetHighSalience(ctx, "", 0.3, 10)
	if err != nil {
		t.Fatalf("GetHighSalience global: %v", err)
	}
	if len(cells) != 2 {
		t.Errorf("expected 2 cells globally >= 0.3, got %d", len(cells))
	}

	// Scoped search — should return only conv-b's cell (salience 0.8)
	cells, err = repo.GetHighSalience(ctx, "conv-b", 0.5, 10)
	if err != nil {
		t.Fatalf("GetHighSalience scoped: %v", err)
	}
	if len(cells) != 1 {
		t.Errorf("expected 1 cell for conv-b >= 0.5, got %d", len(cells))
	}
}
