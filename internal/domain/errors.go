package domain

import (
	"errors"
	"fmt"
)

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

// ErrToolNotFound is returned when a requested tool is not found.
var ErrToolNotFound = errors.New("tool not found")

// ErrInsufficientPermissions is returned when a user lacks permission to execute an action.
var ErrInsufficientPermissions = errors.New("insufficient permissions")

// ErrCannotDeleteLastAdmin is returned when attempting to delete the last admin user.
var ErrCannotDeleteLastAdmin = errors.New("cannot delete last admin user")

// ErrRateLimitExceeded is returned when a rate limit is exceeded.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// ErrLLMUnavailable is returned when an LLM provider is unavailable.
var ErrLLMUnavailable = errors.New("LLM provider unavailable")

// Other potential errors could be added here as needed, e.g.:
// ErrLLMProviderNotConfigured
// ErrCredentialRotationFailed

// ErrorCategory represents the type of error that occurred.
type ErrorCategory string

const (
	// ErrorCategoryUser represents errors caused by user input or actions.
	ErrorCategoryUser ErrorCategory = "user_error"

	// ErrorCategorySystem represents internal system errors.
	ErrorCategorySystem ErrorCategory = "system_error"

	// ErrorCategoryExternal represents errors from external services.
	ErrorCategoryExternal ErrorCategory = "external_error"

	// ErrorCategoryAuth represents authentication/authorization errors.
	ErrorCategoryAuth ErrorCategory = "auth_error"
)

// CategorizedError is a structured error with category, code, and user-facing message.
type CategorizedError struct {
	Category    ErrorCategory // The category of error
	Code        string        // Machine-readable error code (e.g., "INPUT_TOO_LONG")
	Message     string        // Technical error message for logs
	UserMessage string        // User-friendly error message
	Cause       error         // Underlying error, if any
}

// Error implements the error interface.
func (e *CategorizedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Category, e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is and errors.As support.
func (e *CategorizedError) Unwrap() error {
	return e.Cause
}

// IsUserError checks if the error is a user error.
func IsUserError(err error) bool {
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category == ErrorCategoryUser
	}
	return false
}

// IsSystemError checks if the error is a system error.
func IsSystemError(err error) bool {
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category == ErrorCategorySystem
	}
	return false
}

// GetErrorCategory extracts the error category from an error.
// Returns empty string and false if the error is not categorized.
func GetErrorCategory(err error) (ErrorCategory, bool) {
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category, true
	}
	return "", false
}

// GetUserMessage returns a user-friendly error message.
// Falls back to the technical message or a generic message.
func GetUserMessage(err error) string {
	if err == nil {
		return ""
	}

	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		if catErr.UserMessage != "" {
			return catErr.UserMessage
		}
		return catErr.Message
	}

	return "An error occurred. Please try again."
}

// NewUserError creates a new user error.
func NewUserError(code, message, userMessage string) *CategorizedError {
	return &CategorizedError{
		Category:    ErrorCategoryUser,
		Code:        code,
		Message:     message,
		UserMessage: userMessage,
	}
}

// NewSystemError creates a new system error with optional cause.
func NewSystemError(code, message string, cause error) *CategorizedError {
	return &CategorizedError{
		Category:    ErrorCategorySystem,
		Code:        code,
		Message:     message,
		UserMessage: "An internal error occurred. Please try again later.",
		Cause:       cause,
	}
}

// NewExternalError creates a new external service error with optional cause.
func NewExternalError(code, message string, cause error) *CategorizedError {
	return &CategorizedError{
		Category:    ErrorCategoryExternal,
		Code:        code,
		Message:     message,
		UserMessage: "An external service is currently unavailable. Please try again later.",
		Cause:       cause,
	}
}

// NewAuthError creates a new authentication/authorization error.
func NewAuthError(code, message, userMessage string) *CategorizedError {
	return &CategorizedError{
		Category:    ErrorCategoryAuth,
		Code:        code,
		Message:     message,
		UserMessage: userMessage,
	}
}
