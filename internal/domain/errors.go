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

// ErrSkillNotFound is returned when a requested skill is not found.
var ErrSkillNotFound = errors.New("skill not found")

// ErrInsufficientPermissions is returned when a user lacks permission to execute an action.
var ErrInsufficientPermissions = errors.New("insufficient permissions")

// ErrCannotDeleteLastAdmin is returned when attempting to delete the last admin user.
var ErrCannotDeleteLastAdmin = errors.New("cannot delete last admin user")

// Other potential errors could be added here as needed, e.g.:
// ErrLLMProviderNotConfigured
// ErrCredentialRotationFailed
