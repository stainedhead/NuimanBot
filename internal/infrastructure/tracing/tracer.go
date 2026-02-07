package tracing

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// Config defines tracing configuration.
type Config struct {
	Enabled     bool
	ServiceName string
	Environment string
	Endpoint    string // OTLP endpoint (e.g., "localhost:4317")
	SampleRate  float64
}

// Span represents a trace span.
type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	Attributes map[string]any
	mu         sync.RWMutex
}

type spanContextKey struct{}

var (
	globalConfig Config
	initialized  bool
	mu           sync.RWMutex
)

// Initialize sets up the tracing system.
// For MVP, this is a simplified in-memory implementation.
// In production, this would initialize OpenTelemetry with OTLP exporter.
func Initialize(config Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalConfig = config
	initialized = true

	if config.Enabled {
		slog.Info("Tracing initialized",
			"service", config.ServiceName,
			"environment", config.Environment,
			"endpoint", config.Endpoint,
		)
	} else {
		slog.Info("Tracing disabled")
	}

	return nil
}

// Shutdown cleanly shuts down the tracing system.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized && globalConfig.Enabled {
		slog.Info("Tracing shutdown")
	}

	initialized = false
	return nil
}

// StartSpan starts a new trace span.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	mu.RUnlock()

	span := &Span{
		Name:       name,
		Attributes: make(map[string]any),
	}

	// Get parent span if exists
	if parent := getSpan(ctx); parent != nil {
		span.TraceID = parent.TraceID
		span.ParentID = parent.SpanID
	} else {
		// New trace
		span.TraceID = uuid.New().String()
	}

	span.SpanID = uuid.New().String()

	if enabled {
		slog.Debug("Span started",
			"trace_id", span.TraceID,
			"span_id", span.SpanID,
			"parent_id", span.ParentID,
			"name", name,
		)
	}

	// Store span in context
	return context.WithValue(ctx, spanContextKey{}, span), span
}

// EndSpan ends the current span.
func EndSpan(ctx context.Context) {
	span := getSpan(ctx)
	if span == nil {
		return
	}

	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	mu.RUnlock()

	if enabled {
		slog.Debug("Span ended",
			"trace_id", span.TraceID,
			"span_id", span.SpanID,
			"name", span.Name,
		)
	}
}

// AddAttribute adds an attribute to the current span.
func AddAttribute(ctx context.Context, key string, value any) {
	span := getSpan(ctx)
	if span == nil {
		return
	}

	span.mu.Lock()
	defer span.mu.Unlock()

	span.Attributes[key] = value
}

// RecordError records an error in the current span.
func RecordError(ctx context.Context, message string) {
	span := getSpan(ctx)
	if span == nil {
		return
	}

	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	mu.RUnlock()

	if enabled {
		slog.Warn("Span error",
			"trace_id", span.TraceID,
			"span_id", span.SpanID,
			"name", span.Name,
			"error", message,
		)
	}

	AddAttribute(ctx, "error", true)
	AddAttribute(ctx, "error.message", message)
}

// GetTraceID returns the trace ID from the context.
func GetTraceID(ctx context.Context) string {
	span := getSpan(ctx)
	if span == nil {
		return ""
	}
	return span.TraceID
}

// GetSpanID returns the span ID from the context.
func GetSpanID(ctx context.Context) string {
	span := getSpan(ctx)
	if span == nil {
		return ""
	}
	return span.SpanID
}

// getSpan retrieves the span from context.
func getSpan(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}

	span, ok := ctx.Value(spanContextKey{}).(*Span)
	if !ok {
		return nil
	}

	return span
}
