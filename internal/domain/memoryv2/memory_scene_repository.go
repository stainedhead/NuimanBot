package memoryv2

import "context"

// MemorySceneRepository defines operations for scene persistence.
type MemorySceneRepository interface {
	// Upsert creates a scene if it doesn't exist, or updates it if it does.
	Upsert(ctx context.Context, scene *MemoryScene) error

	// Get retrieves a scene by name.
	// Returns ErrNotFound if the scene doesn't exist.
	Get(ctx context.Context, scene string) (*MemoryScene, error)

	// List retrieves all scenes.
	// Returns empty slice (never nil) if no scenes exist.
	List(ctx context.Context) ([]*MemoryScene, error)

	// Delete removes a scene by name.
	// Returns ErrNotFound if the scene doesn't exist.
	Delete(ctx context.Context, scene string) error
}
