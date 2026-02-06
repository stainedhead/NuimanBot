package notes

import (
	"context"

	"nuimanbot/internal/domain"
)

// Repository defines the interface for note persistence.
type Repository interface {
	// Create creates a new note.
	Create(ctx context.Context, note *domain.Note) error

	// GetByID retrieves a note by ID.
	GetByID(ctx context.Context, noteID string) (*domain.Note, error)

	// List retrieves all notes for a user.
	List(ctx context.Context, userID string) ([]*domain.Note, error)

	// Update updates an existing note.
	Update(ctx context.Context, note *domain.Note) error

	// Delete deletes a note by ID.
	Delete(ctx context.Context, noteID string) error
}
