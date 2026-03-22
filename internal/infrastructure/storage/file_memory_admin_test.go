package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"

	"github.com/google/uuid"
)

func newTestAdmin(t *testing.T) (*FileMemoryAdmin, *FileMemoryCellRepository, *FileMemorySceneRepository) {
	t.Helper()
	basePath := filepath.Join(t.TempDir(), "data")
	cellRepo := NewFileMemoryCellRepository(basePath)
	sceneRepo := NewFileMemorySceneRepository(basePath)
	admin := NewFileMemoryAdmin(cellRepo, sceneRepo, basePath)
	return admin, cellRepo, sceneRepo
}

func makeTestCell(convID, scene string, salience float64) *memoryv2.MemoryCell {
	return &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: convID,
		Scene:          scene,
		CellType:       memoryv2.CellTypeFact,
		Salience:       salience,
		Content:        "Test content for " + convID,
		Source:         `[]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func TestFileMemoryAdmin_Stats_Empty(t *testing.T) {
	admin, _, _ := newTestAdmin(t)
	ctx := context.Background()

	stats, err := admin.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.CellCount != 0 {
		t.Errorf("expected 0 cells, got %d", stats.CellCount)
	}
	if stats.SceneCount != 0 {
		t.Errorf("expected 0 scenes, got %d", stats.SceneCount)
	}
}

func TestFileMemoryAdmin_Stats_WithData(t *testing.T) {
	admin, cellRepo, sceneRepo := newTestAdmin(t)
	ctx := context.Background()

	// Create 3 cells in 2 scenes
	cells := []*memoryv2.MemoryCell{
		makeTestCell("conv-1", "project-setup", 0.8),
		makeTestCell("conv-1", "project-setup", 0.6),
		makeTestCell("conv-2", "user-prefs", 0.9),
	}
	for _, c := range cells {
		if err := cellRepo.Create(ctx, c); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Create 2 scenes
	for _, sceneName := range []string{"project-setup", "user-prefs"} {
		scene := &memoryv2.MemoryScene{
			Scene:      sceneName,
			Summary:    "Summary for " + sceneName,
			TokenCount: 10,
			UpdatedAt:  time.Now(),
		}
		if err := sceneRepo.Upsert(ctx, scene); err != nil {
			t.Fatalf("Upsert scene: %v", err)
		}
	}

	stats, err := admin.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.CellCount != 3 {
		t.Errorf("expected 3 cells, got %d", stats.CellCount)
	}
	if stats.SceneCount != 2 {
		t.Errorf("expected 2 scenes, got %d", stats.SceneCount)
	}
	if stats.DBSizeBytes <= 0 {
		t.Error("expected positive DBSizeBytes")
	}
}

func TestFileMemoryAdmin_CountCellsByConversation(t *testing.T) {
	admin, cellRepo, _ := newTestAdmin(t)
	ctx := context.Background()

	// Create cells for two conversations
	for i := 0; i < 3; i++ {
		if err := cellRepo.Create(ctx, makeTestCell("conv-a", "scene-x", 0.7)); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	if err := cellRepo.Create(ctx, makeTestCell("conv-b", "scene-x", 0.7)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err := admin.CountCellsByConversation(ctx, "conv-a")
	if err != nil {
		t.Fatalf("CountCellsByConversation: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 cells for conv-a, got %d", count)
	}

	count, err = admin.CountCellsByConversation(ctx, "conv-b")
	if err != nil {
		t.Fatalf("CountCellsByConversation: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 cell for conv-b, got %d", count)
	}

	count, err = admin.CountCellsByConversation(ctx, "conv-unknown")
	if err != nil {
		t.Fatalf("CountCellsByConversation: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 cells for unknown conv, got %d", count)
	}
}

func TestFileMemoryAdmin_DeleteCellsByConversation(t *testing.T) {
	admin, cellRepo, _ := newTestAdmin(t)
	ctx := context.Background()

	// Create cells for two conversations
	for i := 0; i < 2; i++ {
		if err := cellRepo.Create(ctx, makeTestCell("conv-a", "scene-x", 0.7)); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	keepCell := makeTestCell("conv-b", "scene-x", 0.7)
	if err := cellRepo.Create(ctx, keepCell); err != nil {
		t.Fatalf("Create: %v", err)
	}

	deleted, err := admin.DeleteCellsByConversation(ctx, "conv-a")
	if err != nil {
		t.Fatalf("DeleteCellsByConversation: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// conv-b cell must still exist
	_, err = cellRepo.Get(ctx, keepCell.ID)
	if err != nil {
		t.Errorf("conv-b cell should still exist: %v", err)
	}

	// conv-a count should now be 0
	count, _ := admin.CountCellsByConversation(ctx, "conv-a")
	if count != 0 {
		t.Errorf("expected 0 conv-a cells after delete, got %d", count)
	}
}

func TestFileMemoryAdmin_RebuildFTSIndex(t *testing.T) {
	admin, cellRepo, _ := newTestAdmin(t)
	ctx := context.Background()

	cell := makeTestCell("conv-1", "test-scene", 0.8)
	cell.Content = "golang programming language"
	if err := cellRepo.Create(ctx, cell); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Corrupt the index by resetting it
	cellRepo.mu.Lock()
	idx := &MemoryCellIndex{
		Version:     "1.0",
		ByScene:     make(map[string][]string),
		ByType:      make(map[string][]string),
		BySalience:  make(map[string]float64),
		ByConvID:    make(map[string][]string),
		SearchIndex: make(map[string]map[string]bool),
	}
	_ = cellRepo.saveIndex(idx)
	cellRepo.mu.Unlock()

	// Rebuild should restore the index
	if err := admin.RebuildFTSIndex(ctx); err != nil {
		t.Fatalf("RebuildFTSIndex: %v", err)
	}

	// Search should work again
	results, err := cellRepo.SearchFTS(ctx, "golang", 10)
	if err != nil {
		t.Fatalf("SearchFTS after rebuild: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search to find cell after index rebuild")
	}
}
