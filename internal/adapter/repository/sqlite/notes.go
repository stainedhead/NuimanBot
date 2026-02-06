package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// NotesRepository implements notes.Repository using SQLite.
type NotesRepository struct {
	db *sql.DB
}

// NewNotesRepository creates a new SQLite notes repository.
func NewNotesRepository(db *sql.DB) *NotesRepository {
	return &NotesRepository{db: db}
}

// Create creates a new note.
func (r *NotesRepository) Create(ctx context.Context, note *domain.Note) error {
	if err := note.Validate(); err != nil {
		return err
	}

	// Convert tags to comma-separated string
	tagsStr := strings.Join(note.Tags, ",")

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notes (id, user_id, title, content, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, note.ID, note.UserID, note.Title, note.Content, tagsStr, note.CreatedAt, note.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}

	return nil
}

// GetByID retrieves a note by ID.
func (r *NotesRepository) GetByID(ctx context.Context, noteID string) (*domain.Note, error) {
	var note domain.Note
	var tagsStr string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, content, tags, created_at, updated_at
		FROM notes
		WHERE id = ?
	`, noteID).Scan(&note.ID, &note.UserID, &note.Title, &note.Content, &tagsStr, &note.CreatedAt, &note.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}

	// Parse tags
	if tagsStr != "" {
		note.Tags = strings.Split(tagsStr, ",")
	}

	return &note, nil
}

// List retrieves all notes for a user.
func (r *NotesRepository) List(ctx context.Context, userID string) ([]*domain.Note, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, content, tags, created_at, updated_at
		FROM notes
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	defer rows.Close()

	var notes []*domain.Note
	for rows.Next() {
		var note domain.Note
		var tagsStr string

		err := rows.Scan(&note.ID, &note.UserID, &note.Title, &note.Content, &tagsStr, &note.CreatedAt, &note.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}

		// Parse tags
		if tagsStr != "" {
			note.Tags = strings.Split(tagsStr, ",")
		}

		notes = append(notes, &note)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating notes: %w", err)
	}

	return notes, nil
}

// Update updates an existing note.
func (r *NotesRepository) Update(ctx context.Context, note *domain.Note) error {
	if err := note.Validate(); err != nil {
		return err
	}

	// Convert tags to comma-separated string
	tagsStr := strings.Join(note.Tags, ",")

	// Update the updated_at timestamp
	note.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, `
		UPDATE notes
		SET title = ?, content = ?, tags = ?, updated_at = ?
		WHERE id = ?
	`, note.Title, note.Content, tagsStr, note.UpdatedAt, note.ID)

	if err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Delete deletes a note by ID.
func (r *NotesRepository) Delete(ctx context.Context, noteID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM notes WHERE id = ?
	`, noteID)

	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}
