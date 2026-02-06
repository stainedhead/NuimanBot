package domain

import (
	"time"
)

// Note represents a user note.
type Note struct {
	ID        string
	UserID    string
	Title     string
	Content   string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks if the note has valid data.
func (n *Note) Validate() error {
	if n.UserID == "" {
		return ErrInvalidInput
	}
	if n.Title == "" {
		return ErrInvalidInput
	}
	if n.Content == "" {
		return ErrInvalidInput
	}
	if len(n.Content) > 100000 {
		return ErrInvalidInput
	}
	return nil
}
