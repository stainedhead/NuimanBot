package errortracking

import (
	"context"
	"log/slog"
	"sync"
)

// Config defines error tracking configuration.
type Config struct {
	Enabled     bool
	ServiceName string
	Environment string
	DSN         string // Sentry DSN or other error tracking service endpoint
	SampleRate  float64
}

// Severity levels for error tracking.
const (
	SeverityDebug   = "debug"
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
	SeverityFatal   = "fatal"
)

// Context keys for error tracking metadata.
type (
	userContextKey        struct{}
	tagsContextKey        struct{}
	extrasContextKey      struct{}
	breadcrumbsContextKey struct{}
	fingerprintContextKey struct{}
)

// UserContext represents user information for error tracking.
type UserContext struct {
	ID    string
	Email string
}

// Breadcrumb represents a trail of events leading to an error.
type Breadcrumb struct {
	Type    string
	Message string
	Data    map[string]any
}

var (
	globalConfig Config
	initialized  bool
	mu           sync.RWMutex
)

// Initialize sets up the error tracking system.
// For MVP, this logs errors with structured data.
// In production, this would initialize Sentry or similar service.
func Initialize(config Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalConfig = config
	initialized = true

	if config.Enabled {
		slog.Info("Error tracking initialized",
			"service", config.ServiceName,
			"environment", config.Environment,
		)
	} else {
		slog.Info("Error tracking disabled")
	}

	return nil
}

// Shutdown cleanly shuts down error tracking.
func Shutdown() error {
	mu.Lock()
	defer mu.Unlock()

	if initialized && globalConfig.Enabled {
		slog.Info("Error tracking shutdown")
	}

	initialized = false
	return nil
}

// CaptureError captures an error with context.
func CaptureError(ctx context.Context, err error) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	mu.RUnlock()

	if !enabled || err == nil {
		return
	}

	// Extract context metadata
	user := getUserContext(ctx)
	tags := getTags(ctx)
	extras := getExtras(ctx)
	breadcrumbs := getBreadcrumbs(ctx)
	fingerprint := getFingerprint(ctx)

	// Build structured log
	logAttrs := []any{
		"error", err.Error(),
		"service", globalConfig.ServiceName,
		"environment", globalConfig.Environment,
	}

	if user != nil {
		logAttrs = append(logAttrs, "user_id", user.ID, "user_email", user.Email)
	}

	if len(tags) > 0 {
		logAttrs = append(logAttrs, "tags", tags)
	}

	if len(extras) > 0 {
		logAttrs = append(logAttrs, "extras", extras)
	}

	if len(breadcrumbs) > 0 {
		logAttrs = append(logAttrs, "breadcrumb_count", len(breadcrumbs))
	}

	if fingerprint != "" {
		logAttrs = append(logAttrs, "fingerprint", fingerprint)
	}

	slog.Error("Error captured", logAttrs...)
}

// CaptureMessage captures a message with severity level.
func CaptureMessage(ctx context.Context, severity, message string) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	mu.RUnlock()

	if !enabled {
		return
	}

	// Extract context metadata
	user := getUserContext(ctx)
	tags := getTags(ctx)
	extras := getExtras(ctx)

	// Build structured log
	logAttrs := []any{
		"message", message,
		"severity", severity,
		"service", globalConfig.ServiceName,
		"environment", globalConfig.Environment,
	}

	if user != nil {
		logAttrs = append(logAttrs, "user_id", user.ID)
	}

	if len(tags) > 0 {
		logAttrs = append(logAttrs, "tags", tags)
	}

	if len(extras) > 0 {
		logAttrs = append(logAttrs, "extras", extras)
	}

	// Log at appropriate level
	switch severity {
	case SeverityFatal, SeverityError:
		slog.Error("Message captured", logAttrs...)
	case SeverityWarning:
		slog.Warn("Message captured", logAttrs...)
	default:
		slog.Info("Message captured", logAttrs...)
	}
}

// WithUser adds user context.
func WithUser(ctx context.Context, id, email string) context.Context {
	return context.WithValue(ctx, userContextKey{}, &UserContext{
		ID:    id,
		Email: email,
	})
}

// WithTag adds a tag to the context.
func WithTag(ctx context.Context, key, value string) context.Context {
	tags := getTags(ctx)
	if tags == nil {
		tags = make(map[string]string)
	}
	tags[key] = value
	return context.WithValue(ctx, tagsContextKey{}, tags)
}

// WithExtra adds extra data to the context.
func WithExtra(ctx context.Context, key string, value any) context.Context {
	extras := getExtras(ctx)
	if extras == nil {
		extras = make(map[string]any)
	}
	extras[key] = value
	return context.WithValue(ctx, extrasContextKey{}, extras)
}

// AddBreadcrumb adds a breadcrumb to the context.
func AddBreadcrumb(ctx context.Context, typ, message string, data map[string]any) context.Context {
	breadcrumbs := getBreadcrumbs(ctx)
	breadcrumbs = append(breadcrumbs, Breadcrumb{
		Type:    typ,
		Message: message,
		Data:    data,
	})
	return context.WithValue(ctx, breadcrumbsContextKey{}, breadcrumbs)
}

// WithFingerprint sets a custom fingerprint for error grouping.
func WithFingerprint(ctx context.Context, fingerprint string) context.Context {
	return context.WithValue(ctx, fingerprintContextKey{}, fingerprint)
}

// Helper functions to extract context metadata

func getUserContext(ctx context.Context) *UserContext {
	if ctx == nil {
		return nil
	}
	user, _ := ctx.Value(userContextKey{}).(*UserContext)
	return user
}

func getTags(ctx context.Context) map[string]string {
	if ctx == nil {
		return nil
	}
	tags, _ := ctx.Value(tagsContextKey{}).(map[string]string)
	return tags
}

func getExtras(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}
	extras, _ := ctx.Value(extrasContextKey{}).(map[string]any)
	return extras
}

func getBreadcrumbs(ctx context.Context) []Breadcrumb {
	if ctx == nil {
		return nil
	}
	breadcrumbs, _ := ctx.Value(breadcrumbsContextKey{}).([]Breadcrumb)
	return breadcrumbs
}

func getFingerprint(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	fingerprint, _ := ctx.Value(fingerprintContextKey{}).(string)
	return fingerprint
}
