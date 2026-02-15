package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// SQLiteMemorySceneRepository implements memoryv2.MemorySceneRepository using SQLite
type SQLiteMemorySceneRepository struct {
	db *sql.DB
}

// NewSQLiteMemorySceneRepository creates a new SQLite scene repository instance
func NewSQLiteMemorySceneRepository(db *sql.DB) *SQLiteMemorySceneRepository {
	return &SQLiteMemorySceneRepository{db: db}
}

// Upsert creates a scene if it doesn't exist, or updates it if it does
func (r *SQLiteMemorySceneRepository) Upsert(ctx context.Context, scene *memoryv2.MemoryScene) error {
	if err := scene.Validate(); err != nil {
		return err
	}

	query := `
		INSERT INTO memory_scenes (scene, summary, token_count, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(scene) DO UPDATE SET
			summary = excluded.summary,
			token_count = excluded.token_count,
			updated_at = excluded.updated_at
	`

	_, err := r.db.ExecContext(ctx, query,
		scene.Scene,
		scene.Summary,
		scene.TokenCount,
		scene.UpdatedAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to upsert memory scene: %w", err)
	}

	return nil
}

// Get retrieves a scene by name
func (r *SQLiteMemorySceneRepository) Get(ctx context.Context, scene string) (*memoryv2.MemoryScene, error) {
	query := `
		SELECT scene, summary, token_count, updated_at
		FROM memory_scenes
		WHERE scene = ?
	`

	var result memoryv2.MemoryScene
	var updatedAtStr string

	err := r.db.QueryRowContext(ctx, query, scene).Scan(
		&result.Scene,
		&result.Summary,
		&result.TokenCount,
		&updatedAtStr,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, memoryv2.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get memory scene: %w", err)
	}

	// Parse timestamp
	result.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
	}

	return &result, nil
}

// List retrieves all scenes
func (r *SQLiteMemorySceneRepository) List(ctx context.Context) ([]*memoryv2.MemoryScene, error) {
	query := `
		SELECT scene, summary, token_count, updated_at
		FROM memory_scenes
		ORDER BY updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list memory scenes: %w", err)
	}
	defer rows.Close()

	scenes := []*memoryv2.MemoryScene{}

	for rows.Next() {
		var scene memoryv2.MemoryScene
		var updatedAtStr string

		err := rows.Scan(
			&scene.Scene,
			&scene.Summary,
			&scene.TokenCount,
			&updatedAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse timestamp
		scene.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
		}

		scenes = append(scenes, &scene)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return scenes, nil
}

// Delete removes a scene by name
func (r *SQLiteMemorySceneRepository) Delete(ctx context.Context, scene string) error {
	query := `DELETE FROM memory_scenes WHERE scene = ?`

	result, err := r.db.ExecContext(ctx, query, scene)
	if err != nil {
		return fmt.Errorf("failed to delete memory scene: %w", err)
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
