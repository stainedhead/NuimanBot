package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nuimanbot/internal/domain/memoryv2"
	"os"
	"path/filepath"
	"sync"
)

// FileMemorySceneRepository implements MemorySceneRepository using file storage
type FileMemorySceneRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileMemorySceneRepository creates a new file-based memory scene repository
func NewFileMemorySceneRepository(basePath string) *FileMemorySceneRepository {
	return &FileMemorySceneRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

// getScenesDir returns the path to the memory scenes directory
func (r *FileMemorySceneRepository) getScenesDir() string {
	return filepath.Join(r.basePath, "memory", "scenes")
}

// getSceneFile returns the path to a scene's JSON file
func (r *FileMemorySceneRepository) getSceneFile(scene string) string {
	return filepath.Join(r.getScenesDir(), scene+".json")
}

// Upsert creates a scene if it doesn't exist, or updates it if it does
func (r *FileMemorySceneRepository) Upsert(ctx context.Context, scene *memoryv2.MemoryScene) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate scene
	if err := scene.Validate(); err != nil {
		return fmt.Errorf("scene validation failed: %w", err)
	}

	// Ensure scenes directory exists
	scenesDir := r.getScenesDir()
	if err := os.MkdirAll(scenesDir, 0755); err != nil {
		return fmt.Errorf("failed to create scenes directory: %w", err)
	}

	// Write scene to file
	scenePath := r.getSceneFile(scene.Scene)
	data, err := json.MarshalIndent(scene, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scene: %w", err)
	}

	if err := r.writer.Write(scenePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write scene file: %w", err)
	}

	return nil
}

// Get retrieves a scene by name
func (r *FileMemorySceneRepository) Get(ctx context.Context, scene string) (*memoryv2.MemoryScene, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scenePath := r.getSceneFile(scene)

	// Check if file exists
	if _, err := os.Stat(scenePath); os.IsNotExist(err) {
		return nil, errors.New("scene not found")
	}

	// Read file
	data, err := os.ReadFile(scenePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read scene file: %w", err)
	}

	// Parse JSON
	var memScene memoryv2.MemoryScene
	if err := json.Unmarshal(data, &memScene); err != nil {
		return nil, fmt.Errorf("failed to parse scene file: %w", err)
	}

	return &memScene, nil
}

// List retrieves all scenes
func (r *FileMemorySceneRepository) List(ctx context.Context) ([]*memoryv2.MemoryScene, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scenesDir := r.getScenesDir()

	// Check if directory exists
	if _, err := os.Stat(scenesDir); os.IsNotExist(err) {
		// Return empty slice
		return []*memoryv2.MemoryScene{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(scenesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenes directory: %w", err)
	}

	// Load all scenes
	scenes := make([]*memoryv2.MemoryScene, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read scene file
		scenePath := filepath.Join(scenesDir, entry.Name())
		data, err := os.ReadFile(scenePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var scene memoryv2.MemoryScene
		if err := json.Unmarshal(data, &scene); err != nil {
			continue // Skip files that can't be parsed
		}

		scenes = append(scenes, &scene)
	}

	return scenes, nil
}

// Delete removes a scene by name
func (r *FileMemorySceneRepository) Delete(ctx context.Context, scene string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	scenePath := r.getSceneFile(scene)

	// Check if file exists
	if _, err := os.Stat(scenePath); os.IsNotExist(err) {
		return errors.New("scene not found")
	}

	// Delete file
	if err := os.Remove(scenePath); err != nil {
		return fmt.Errorf("failed to delete scene file: %w", err)
	}

	return nil
}
