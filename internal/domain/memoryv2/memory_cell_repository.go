package memoryv2

import "context"

// MemoryCellRepository defines operations for memory cell persistence.
type MemoryCellRepository interface {
	// Create inserts a new memory cell.
	// Returns ErrAlreadyExists if a cell with the same ID exists.
	Create(ctx context.Context, cell *MemoryCell) error

	// Get retrieves a cell by ID.
	// Returns ErrNotFound if the cell doesn't exist.
	Get(ctx context.Context, id string) (*MemoryCell, error)

	// List retrieves cells matching the filter.
	// Returns empty slice (never nil) if no matches.
	List(ctx context.Context, filter MemoryCellFilter) ([]*MemoryCell, error)

	// Update persists changes to an existing cell.
	// Returns ErrNotFound if the cell doesn't exist.
	Update(ctx context.Context, cell *MemoryCell) error

	// Delete removes a cell by ID.
	// Returns ErrNotFound if the cell doesn't exist.
	Delete(ctx context.Context, id string) error

	// SearchFTS performs full-text search on cell content.
	// Returns results ranked by relevance (BM25).
	SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error)

	// GetByScene retrieves cells for a specific scene.
	// Returns cells ordered by salience descending.
	GetByScene(ctx context.Context, scene string, limit int) ([]*MemoryCell, error)

	// GetHighSalience retrieves cells above a salience threshold.
	// Returns cells with salience >= threshold.
	GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*MemoryCell, error)

	// DeleteExpired removes cells past their expiration time.
	// Returns the count of deleted cells.
	DeleteExpired(ctx context.Context) (int, error)
}
