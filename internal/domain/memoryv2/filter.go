package memoryv2

// MemoryCellFilter defines criteria for filtering memory cells.
type MemoryCellFilter struct {
	// ConversationID filters by conversation ID (optional).
	ConversationID string

	// Scene filters by scene name (optional).
	Scene string

	// CellType filters by cell type (optional, nil means all types).
	CellType *CellType

	// MinSalience filters cells with salience >= this value (optional).
	MinSalience *float64

	// IncludeExpired includes expired cells if true (default: false).
	IncludeExpired bool

	// Limit is the maximum number of results to return (0 = no limit).
	Limit int

	// Offset is the number of results to skip (for pagination).
	Offset int
}
