package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nuimanbot/internal/domain/memoryv2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryCellIndex represents the index structure for memory cells
type MemoryCellIndex struct {
	Version     string                     `json:"version"`
	LastUpdated string                     `json:"lastUpdated"`
	ByScene     map[string][]string        `json:"byScene"`     // scene -> []cellID
	ByType      map[string][]string        `json:"byType"`      // cellType -> []cellID
	BySalience  map[string]float64         `json:"bySalience"`  // cellID -> salience
	ByConvID    map[string][]string        `json:"byConvID"`    // convID -> []cellID
	SearchIndex map[string]map[string]bool `json:"searchIndex"` // word -> {cellID: true}
}

// FileMemoryCellRepository implements MemoryCellRepository using file storage
type FileMemoryCellRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileMemoryCellRepository creates a new file-based memory cell repository
func NewFileMemoryCellRepository(basePath string) *FileMemoryCellRepository {
	return &FileMemoryCellRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

// getCellsDir returns the path to the memory cells directory
func (r *FileMemoryCellRepository) getCellsDir() string {
	return filepath.Join(r.basePath, "memory", "cells")
}

// getCellFile returns the path to a cell's JSON file
func (r *FileMemoryCellRepository) getCellFile(cellID string) string {
	return filepath.Join(r.getCellsDir(), cellID+".json")
}

// getIndexFile returns the path to the memory index file
func (r *FileMemoryCellRepository) getIndexFile() string {
	return filepath.Join(r.basePath, "memory", "index.json")
}

// loadIndex loads the memory cell index
func (r *FileMemoryCellRepository) loadIndex() (*MemoryCellIndex, error) {
	indexPath := r.getIndexFile()

	// Check if file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// Return empty index
		return &MemoryCellIndex{
			Version:     "1.0",
			ByScene:     make(map[string][]string),
			ByType:      make(map[string][]string),
			BySalience:  make(map[string]float64),
			ByConvID:    make(map[string][]string),
			SearchIndex: make(map[string]map[string]bool),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	// Parse JSON
	var index MemoryCellIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index file: %w", err)
	}

	// Initialize maps if nil
	if index.ByScene == nil {
		index.ByScene = make(map[string][]string)
	}
	if index.ByType == nil {
		index.ByType = make(map[string][]string)
	}
	if index.BySalience == nil {
		index.BySalience = make(map[string]float64)
	}
	if index.ByConvID == nil {
		index.ByConvID = make(map[string][]string)
	}
	if index.SearchIndex == nil {
		index.SearchIndex = make(map[string]map[string]bool)
	}

	return &index, nil
}

// saveIndex saves the memory cell index
func (r *FileMemoryCellRepository) saveIndex(index *MemoryCellIndex) error {
	indexPath := r.getIndexFile()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Update timestamp
	index.LastUpdated = time.Now().Format(time.RFC3339)

	// Marshal to JSON
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write atomically
	if err := r.writer.Write(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// tokenize extracts searchable words from content
func tokenize(content string) []string {
	// Simple tokenization: lowercase, split on spaces, remove short words
	words := strings.Fields(strings.ToLower(content))
	var tokens []string
	for _, word := range words {
		// Remove punctuation and keep words with 3+ chars
		clean := strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(clean) >= 3 {
			tokens = append(tokens, clean)
		}
	}
	return tokens
}

// addToIndex adds a cell to all indexes
func (r *FileMemoryCellRepository) addToIndex(index *MemoryCellIndex, cell *memoryv2.MemoryCell) {
	// By scene
	index.ByScene[cell.Scene] = appendUnique(index.ByScene[cell.Scene], cell.ID)

	// By type
	cellTypeStr := cell.CellType.String()
	index.ByType[cellTypeStr] = appendUnique(index.ByType[cellTypeStr], cell.ID)

	// By salience
	index.BySalience[cell.ID] = cell.Salience

	// By conversation ID
	index.ByConvID[cell.ConversationID] = appendUnique(index.ByConvID[cell.ConversationID], cell.ID)

	// Search index
	tokens := tokenize(cell.Content)
	for _, token := range tokens {
		if index.SearchIndex[token] == nil {
			index.SearchIndex[token] = make(map[string]bool)
		}
		index.SearchIndex[token][cell.ID] = true
	}
}

// removeFromIndex removes a cell from all indexes
func (r *FileMemoryCellRepository) removeFromIndex(index *MemoryCellIndex, cell *memoryv2.MemoryCell) {
	// By scene
	index.ByScene[cell.Scene] = removeString(index.ByScene[cell.Scene], cell.ID)

	// By type
	cellTypeStr := cell.CellType.String()
	index.ByType[cellTypeStr] = removeString(index.ByType[cellTypeStr], cell.ID)

	// By salience
	delete(index.BySalience, cell.ID)

	// By conversation ID
	index.ByConvID[cell.ConversationID] = removeString(index.ByConvID[cell.ConversationID], cell.ID)

	// Search index
	tokens := tokenize(cell.Content)
	for _, token := range tokens {
		if index.SearchIndex[token] != nil {
			delete(index.SearchIndex[token], cell.ID)
		}
	}
}

// appendUnique appends s to slice if not already present
func appendUnique(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// removeString removes s from slice
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// Create inserts a new memory cell
func (r *FileMemoryCellRepository) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate cell
	if err := cell.Validate(); err != nil {
		return fmt.Errorf("cell validation failed: %w", err)
	}

	// Ensure cells directory exists
	cellsDir := r.getCellsDir()
	if err := os.MkdirAll(cellsDir, 0755); err != nil {
		return fmt.Errorf("failed to create cells directory: %w", err)
	}

	// Write cell to file
	cellPath := r.getCellFile(cell.ID)
	data, err := json.MarshalIndent(cell, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cell: %w", err)
	}

	if err := r.writer.Write(cellPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cell file: %w", err)
	}

	// Update index
	index, err := r.loadIndex()
	if err != nil {
		return err
	}

	r.addToIndex(index, cell)

	return r.saveIndex(index)
}

// Get retrieves a cell by ID
func (r *FileMemoryCellRepository) Get(ctx context.Context, id string) (*memoryv2.MemoryCell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cellPath := r.getCellFile(id)

	// Check if file exists
	if _, err := os.Stat(cellPath); os.IsNotExist(err) {
		return nil, errors.New("cell not found")
	}

	// Read file
	data, err := os.ReadFile(cellPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cell file: %w", err)
	}

	// Parse JSON
	var cell memoryv2.MemoryCell
	if err := json.Unmarshal(data, &cell); err != nil {
		return nil, fmt.Errorf("failed to parse cell file: %w", err)
	}

	return &cell, nil
}

// List retrieves cells matching the filter
func (r *FileMemoryCellRepository) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}

	// Get cell IDs matching filter
	var cellIDs []string
	if filter.ConversationID != "" {
		cellIDs = index.ByConvID[filter.ConversationID]
	} else {
		// Get all cells
		for scene, ids := range index.ByScene {
			_ = scene
			cellIDs = append(cellIDs, ids...)
		}
	}

	// Load cells
	cells := make([]*memoryv2.MemoryCell, 0, len(cellIDs))
	for _, id := range cellIDs {
		cell, err := r.Get(ctx, id)
		if err != nil {
			continue // Skip cells that can't be loaded
		}
		cells = append(cells, cell)
	}

	return cells, nil
}

// Delete removes a cell by ID
func (r *FileMemoryCellRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load cell first (to update index)
	cellPath := r.getCellFile(id)
	data, err := os.ReadFile(cellPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("cell not found")
		}
		return fmt.Errorf("failed to read cell file: %w", err)
	}

	var cell memoryv2.MemoryCell
	if err := json.Unmarshal(data, &cell); err != nil {
		return fmt.Errorf("failed to parse cell file: %w", err)
	}

	// Delete file
	if err := os.Remove(cellPath); err != nil {
		return fmt.Errorf("failed to delete cell file: %w", err)
	}

	// Update index
	index, err := r.loadIndex()
	if err != nil {
		return err
	}

	r.removeFromIndex(index, &cell)

	return r.saveIndex(index)
}

// SearchFTS performs full-text search on cell content
func (r *FileMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}

	// Tokenize query
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return []*memoryv2.MemoryCell{}, nil
	}

	// Find cells matching all query tokens
	cellScores := make(map[string]int)
	for _, token := range queryTokens {
		if cellIDs, found := index.SearchIndex[token]; found {
			for cellID := range cellIDs {
				cellScores[cellID]++
			}
		}
	}

	// Sort by score descending
	type scored struct {
		id    string
		score int
	}
	var scoredCells []scored
	for id, score := range cellScores {
		scoredCells = append(scoredCells, scored{id, score})
	}
	sort.Slice(scoredCells, func(i, j int) bool {
		return scoredCells[i].score > scoredCells[j].score
	})

	// Load cells up to limit
	cells := make([]*memoryv2.MemoryCell, 0, limit)
	for i, sc := range scoredCells {
		if i >= limit {
			break
		}
		cell, err := r.Get(ctx, sc.id)
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}

	return cells, nil
}

// GetByScene retrieves cells for a specific scene
func (r *FileMemoryCellRepository) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}

	cellIDs := index.ByScene[scene]

	// Load cells
	cells := make([]*memoryv2.MemoryCell, 0, len(cellIDs))
	for _, id := range cellIDs {
		cell, err := r.Get(ctx, id)
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}

	// Sort by salience descending
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].Salience > cells[j].Salience
	})

	// Apply limit
	if len(cells) > limit {
		cells = cells[:limit]
	}

	return cells, nil
}

// GetHighSalience retrieves cells above a salience threshold
func (r *FileMemoryCellRepository) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex()
	if err != nil {
		return nil, err
	}

	// Get cells for conversation
	cellIDs := index.ByConvID[conversationID]

	// Filter and load cells above threshold
	cells := make([]*memoryv2.MemoryCell, 0, len(cellIDs))
	for _, id := range cellIDs {
		salience, found := index.BySalience[id]
		if !found || salience < threshold {
			continue
		}

		cell, err := r.Get(ctx, id)
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}

	// Sort by salience descending
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].Salience > cells[j].Salience
	})

	// Apply limit
	if len(cells) > limit {
		cells = cells[:limit]
	}

	return cells, nil
}

// DeleteExpired removes cells past their expiration time
func (r *FileMemoryCellRepository) DeleteExpired(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Get all cells
	cellsDir := r.getCellsDir()
	entries, err := os.ReadDir(cellsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read cells directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read cell
		cellPath := filepath.Join(cellsDir, entry.Name())
		data, err := os.ReadFile(cellPath)
		if err != nil {
			continue
		}

		var cell memoryv2.MemoryCell
		if err := json.Unmarshal(data, &cell); err != nil {
			continue
		}

		// Check if expired
		if cell.ExpiresAt != nil && cell.ExpiresAt.Before(now) {
			// Delete cell
			if err := os.Remove(cellPath); err != nil {
				continue
			}

			// Update index
			index, err := r.loadIndex()
			if err != nil {
				continue
			}

			r.removeFromIndex(index, &cell)

			if err := r.saveIndex(index); err != nil {
				continue
			}

			count++
		}
	}

	return count, nil
}
