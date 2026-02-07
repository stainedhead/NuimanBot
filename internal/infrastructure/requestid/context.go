package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	// requestIDKey is the context key for request IDs.
	requestIDKey contextKey = "request_id"
)

// Generate creates a new unique request ID.
func Generate() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random fails
		return "req_fallback"
	}
	return hex.EncodeToString(b)
}

// WithRequestID adds a request ID to the context.
// If an ID already exists, it returns the context unchanged.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = Generate()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// FromContext retrieves the request ID from the context.
// Returns an empty string if no request ID is present.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// MustFromContext retrieves the request ID from the context.
// If no request ID exists, it generates and adds one to the context.
func MustFromContext(ctx context.Context) (newCtx context.Context, id string) {
	id = FromContext(ctx)
	if id != "" {
		return ctx, id
	}

	id = Generate()
	return WithRequestID(ctx, id), id
}

// LogAttrs returns slog attributes for the request ID.
// This can be used with slog.With() to add request ID to all log entries.
func LogAttrs(ctx context.Context) []any {
	if id := FromContext(ctx); id != "" {
		return []any{slog.String("request_id", id)}
	}
	return nil
}

// Logger returns a logger with the request ID from context.
func Logger(ctx context.Context) *slog.Logger {
	if attrs := LogAttrs(ctx); attrs != nil {
		return slog.With(attrs...)
	}
	return slog.Default()
}
