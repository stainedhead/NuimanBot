package domain

import "context"

// NotesRepository defines operations for notes persistence.
type NotesRepository interface {
	// Create inserts a new note.
	// Returns ErrAlreadyExists if a note with the same ID exists.
	Create(ctx context.Context, note *Note) error

	// GetByID retrieves a note by ID.
	// Returns ErrNotFound if the note doesn't exist.
	GetByID(ctx context.Context, noteID string) (*Note, error)

	// List retrieves notes for a user.
	// Returns empty slice (never nil) if no notes exist.
	List(ctx context.Context, userID string) ([]*Note, error)

	// Update updates an existing note.
	// Returns ErrNotFound if the note doesn't exist.
	Update(ctx context.Context, note *Note) error

	// Delete removes a note by ID.
	// Returns ErrNotFound if the note doesn't exist.
	Delete(ctx context.Context, noteID string) error
}
