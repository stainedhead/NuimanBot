package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// SQLiteMemoryCellRepository implements memoryv2.MemoryCellRepository using SQLite
type SQLiteMemoryCellRepository struct {
	db *sql.DB
}

// NewSQLiteMemoryCellRepository creates a new SQLite repository instance
func NewSQLiteMemoryCellRepository(db *sql.DB) *SQLiteMemoryCellRepository {
	return &SQLiteMemoryCellRepository{db: db}
}

// Create inserts a new memory cell
func (r *SQLiteMemoryCellRepository) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	if err := cell.Validate(); err != nil {
		return err
	}

	query := `
		INSERT INTO memory_cells (
			id, conversation_id, scene, cell_type, salience, content, source,
			created_at, updated_at, expires_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var expiresAt interface{}
	if cell.ExpiresAt != nil {
		expiresAt = cell.ExpiresAt.Format(time.RFC3339)
	}

	_, err := r.db.ExecContext(ctx, query,
		cell.ID,
		cell.ConversationID,
		cell.Scene,
		cell.CellType.String(),
		cell.Salience,
		cell.Content,
		cell.Source,
		cell.CreatedAt.Format(time.RFC3339),
		cell.UpdatedAt.Format(time.RFC3339),
		expiresAt,
	)

	if err != nil {
		// Check for unique constraint violation
		if isUniqueConstraintError(err) {
			return memoryv2.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create memory cell: %w", err)
	}

	return nil
}

// Get retrieves a memory cell by ID
func (r *SQLiteMemoryCellRepository) Get(ctx context.Context, id string) (*memoryv2.MemoryCell, error) {
	query := `
		SELECT id, conversation_id, scene, cell_type, salience, content, source,
		       created_at, updated_at, expires_at
		FROM memory_cells
		WHERE id = ?
	`

	cell := &memoryv2.MemoryCell{}
	var cellTypeStr string
	var createdAtStr, updatedAtStr string
	var expiresAtStr sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&cell.ID,
		&cell.ConversationID,
		&cell.Scene,
		&cellTypeStr,
		&cell.Salience,
		&cell.Content,
		&cell.Source,
		&createdAtStr,
		&updatedAtStr,
		&expiresAtStr,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, memoryv2.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get memory cell: %w", err)
	}

	// Parse cell type
	cellType, err := memoryv2.ParseCellType(cellTypeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cell type in database: %w", err)
	}
	cell.CellType = cellType

	// Parse timestamps
	cell.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	cell.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
	}

	if expiresAtStr.Valid {
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("invalid expires_at timestamp: %w", err)
		}
		cell.ExpiresAt = &expiresAt
	}

	return cell, nil
}

// Delete removes a memory cell
func (r *SQLiteMemoryCellRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM memory_cells WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory cell: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return memoryv2.ErrNotFound
	}

	return nil
}

// List retrieves memory cells matching filter
func (r *SQLiteMemoryCellRepository) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	query := `
		SELECT id, conversation_id, scene, cell_type, salience, content, source,
		       created_at, updated_at, expires_at
		FROM memory_cells
		WHERE 1=1
	`
	args := []interface{}{}

	// Apply filters
	if filter.ConversationID != "" {
		query += " AND conversation_id = ?"
		args = append(args, filter.ConversationID)
	}

	if filter.Scene != "" {
		query += " AND scene = ?"
		args = append(args, filter.Scene)
	}

	if filter.CellType != nil {
		query += " AND cell_type = ?"
		args = append(args, filter.CellType.String())
	}

	// Order by created_at DESC by default
	query += " ORDER BY created_at DESC"

	// Apply limit
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list memory cells: %w", err)
	}
	defer rows.Close()

	return r.scanCells(rows)
}

// SearchFTS performs full-text search on memory cells.
// A limit of 0 or less means no limit.
func (r *SQLiteMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	sqlQuery := `
		SELECT mc.id, mc.conversation_id, mc.scene, mc.cell_type, mc.salience,
		       mc.content, mc.source, mc.created_at, mc.updated_at, mc.expires_at
		FROM memory_cells mc
		JOIN memory_cells_fts fts ON mc.rowid = fts.rowid
		WHERE memory_cells_fts MATCH ?
		ORDER BY rank
	`

	args := []interface{}{query}
	if limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search memory cells: %w", err)
	}
	defer rows.Close()

	return r.scanCells(rows)
}

// GetByScene retrieves cells for a specific scene, ordered by salience.
// A limit of 0 or less means no limit.
func (r *SQLiteMemoryCellRepository) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	query := `
		SELECT id, conversation_id, scene, cell_type, salience, content, source,
		       created_at, updated_at, expires_at
		FROM memory_cells
		WHERE scene = ?
		ORDER BY salience DESC
	`

	args := []interface{}{scene}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get cells by scene: %w", err)
	}
	defer rows.Close()

	return r.scanCells(rows)
}

// GetHighSalience retrieves cells with salience above threshold.
// A limit of 0 or less means no limit.
func (r *SQLiteMemoryCellRepository) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	query := `
		SELECT id, conversation_id, scene, cell_type, salience, content, source,
		       created_at, updated_at, expires_at
		FROM memory_cells
		WHERE conversation_id = ? AND salience >= ?
		ORDER BY salience DESC
	`

	args := []interface{}{conversationID, threshold}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get high salience cells: %w", err)
	}
	defer rows.Close()

	return r.scanCells(rows)
}

// DeleteExpired removes cells past their expiration time
func (r *SQLiteMemoryCellRepository) DeleteExpired(ctx context.Context) (int, error) {
	query := `
		DELETE FROM memory_cells
		WHERE expires_at IS NOT NULL AND expires_at < ?
	`

	result, err := r.db.ExecContext(ctx, query, time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired cells: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rows), nil
}

// scanCells scans multiple rows into memory cells
func (r *SQLiteMemoryCellRepository) scanCells(rows *sql.Rows) ([]*memoryv2.MemoryCell, error) {
	cells := []*memoryv2.MemoryCell{}

	for rows.Next() {
		cell := &memoryv2.MemoryCell{}
		var cellTypeStr string
		var createdAtStr, updatedAtStr string
		var expiresAtStr sql.NullString

		err := rows.Scan(
			&cell.ID,
			&cell.ConversationID,
			&cell.Scene,
			&cellTypeStr,
			&cell.Salience,
			&cell.Content,
			&cell.Source,
			&createdAtStr,
			&updatedAtStr,
			&expiresAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse cell type
		cellType, err := memoryv2.ParseCellType(cellTypeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid cell type in database: %w", err)
		}
		cell.CellType = cellType

		// Parse timestamps
		cell.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
		}

		cell.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
		}

		if expiresAtStr.Valid {
			expiresAt, err := time.Parse(time.RFC3339, expiresAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("invalid expires_at timestamp: %w", err)
			}
			cell.ExpiresAt = &expiresAt
		}

		cells = append(cells, cell)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return cells, nil
}

// isUniqueConstraintError checks if error is a unique constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite error message contains "UNIQUE constraint failed"
	return contains(err.Error(), "UNIQUE") || contains(err.Error(), "unique")
}

// contains checks if string contains substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
