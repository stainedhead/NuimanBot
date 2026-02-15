package memoryv2

import "errors"

// Domain errors for memory v2.
var (
	// ErrNotFound indicates the requested entity was not found.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates an entity with the same ID already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput indicates the input failed validation.
	ErrInvalidInput = errors.New("invalid input")
)
