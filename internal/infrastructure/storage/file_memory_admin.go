package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cliadapter "nuimanbot/internal/adapter/cli"
)

// FileMemoryAdmin implements cliadapter.MemoryAdmin for file-based storage.
type FileMemoryAdmin struct {
	cellRepo  *FileMemoryCellRepository
	sceneRepo *FileMemorySceneRepository
	basePath  string
}

// NewFileMemoryAdmin creates a new FileMemoryAdmin.
func NewFileMemoryAdmin(
	cellRepo *FileMemoryCellRepository,
	sceneRepo *FileMemorySceneRepository,
	basePath string,
) *FileMemoryAdmin {
	return &FileMemoryAdmin{
		cellRepo:  cellRepo,
		sceneRepo: sceneRepo,
		basePath:  basePath,
	}
}

// Stats returns overall memory system statistics.
func (a *FileMemoryAdmin) Stats(ctx context.Context) (*cliadapter.MemoryStats, error) {
	// Count cells via index
	a.cellRepo.mu.RLock()
	index, err := a.cellRepo.loadIndex()
	a.cellRepo.mu.RUnlock()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	cellCount := len(index.BySalience)

	// Count scenes
	scenes, err := a.sceneRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenes: %w", err)
	}

	// Calculate memory directory size
	memDir := filepath.Join(a.basePath, "memory")
	dirSize, err := dirSizeBytes(memDir)
	if err != nil {
		dirSize = 0 // Non-fatal: report 0 if dir doesn't exist
	}

	return &cliadapter.MemoryStats{
		CellCount:   cellCount,
		SceneCount:  len(scenes),
		DBSizeBytes: dirSize,
	}, nil
}

// CountCellsByConversation counts memory cells for a specific conversation.
func (a *FileMemoryAdmin) CountCellsByConversation(ctx context.Context, conversationID string) (int, error) {
	a.cellRepo.mu.RLock()
	index, err := a.cellRepo.loadIndex()
	a.cellRepo.mu.RUnlock()
	if err != nil {
		return 0, fmt.Errorf("failed to load index: %w", err)
	}

	return len(index.ByConvID[conversationID]), nil
}

// DeleteCellsByConversation removes all memory cells for a conversation.
// Returns the number of deleted cells.
func (a *FileMemoryAdmin) DeleteCellsByConversation(ctx context.Context, conversationID string) (int, error) {
	a.cellRepo.mu.Lock()
	defer a.cellRepo.mu.Unlock()

	index, err := a.cellRepo.loadIndex()
	if err != nil {
		return 0, fmt.Errorf("failed to load index: %w", err)
	}

	cellIDs := index.ByConvID[conversationID]
	if len(cellIDs) == 0 {
		return 0, nil
	}

	count := 0
	for _, id := range cellIDs {
		cell, err := a.cellRepo.readCell(id)
		if err != nil {
			continue
		}

		cellPath := a.cellRepo.getCellFile(id)
		if err := os.Remove(cellPath); err != nil {
			continue
		}

		a.cellRepo.removeFromIndex(index, cell)
		count++
	}

	if err := a.cellRepo.saveIndex(index); err != nil {
		return count, fmt.Errorf("cells deleted but failed to save index: %w", err)
	}

	return count, nil
}

// RebuildFTSIndex rebuilds the search index from cell files on disk.
// This corrects any index corruption without losing cell data.
func (a *FileMemoryAdmin) RebuildFTSIndex(ctx context.Context) error {
	a.cellRepo.mu.Lock()
	defer a.cellRepo.mu.Unlock()

	cellsDir := a.cellRepo.getCellsDir()
	entries, err := os.ReadDir(cellsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to rebuild
		}
		return fmt.Errorf("failed to read cells directory: %w", err)
	}

	fresh := &MemoryCellIndex{
		Version:     "1.0",
		ByScene:     make(map[string][]string),
		ByType:      make(map[string][]string),
		BySalience:  make(map[string]float64),
		ByConvID:    make(map[string][]string),
		SearchIndex: make(map[string]map[string]bool),
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // strip .json
		cell, err := a.cellRepo.readCell(id)
		if err != nil {
			continue // Skip corrupt cells
		}

		a.cellRepo.addToIndex(fresh, cell)
	}

	return a.cellRepo.saveIndex(fresh)
}

// dirSizeBytes returns the total size of all files in a directory tree.
func dirSizeBytes(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable entries
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err
}
