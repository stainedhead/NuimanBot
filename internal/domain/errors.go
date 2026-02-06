package domain

import "errors"

// ErrNotFound is returned when a requested entity is not found.
var ErrNotFound = errors.New("not found")

// ErrUnauthorized is returned when an action is not authorized.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when access to a resource is forbidden.
var ErrForbidden = errors.New("forbidden")

// ErrInvalidInput is returned when provided input is invalid.
var ErrInvalidInput = errors.New("invalid input")

// ErrConflict is returned when a resource already exists or a state conflict occurs.
var ErrConflict = errors.New("conflict")

// ErrInternal is a generic error for unexpected internal issues.
var ErrInternal = errors.New("internal error")

// ErrUserNotFound is returned when a user is not found.
var ErrUserNotFound = errors.New("user not found")

// Other potential errors could be added here as needed, e.g.:
// ErrSkillNotFound
// ErrLLMProviderNotConfigured
// ErrCredentialRotationFailed
