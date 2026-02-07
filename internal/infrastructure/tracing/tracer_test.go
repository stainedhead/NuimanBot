package tracing

import (
	"context"
	"testing"
)

// Test: Initialize tracer
func TestInitialize(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
		Endpoint:    "", // Empty endpoint for testing (noop exporter)
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Cleanup
	err = Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// Test: Start and end span
func TestStartSpan(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown(context.Background())

	// Start a span
	ctx, span := StartSpan(context.Background(), "test-operation")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}

	// Add attributes
	AddAttribute(ctx, "key1", "value1")
	AddAttribute(ctx, "key2", 42)

	// End span
	EndSpan(ctx)
}

// Test: Nested spans
func TestNestedSpans(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown(context.Background())

	// Parent span
	ctx1, span1 := StartSpan(context.Background(), "parent-operation")
	if span1 == nil {
		t.Fatal("Expected non-nil parent span")
	}

	// Child span
	ctx2, span2 := StartSpan(ctx1, "child-operation")
	if span2 == nil {
		t.Fatal("Expected non-nil child span")
	}

	// End spans in reverse order
	EndSpan(ctx2)
	EndSpan(ctx1)
}

// Test: Tracing disabled
func TestTracingDisabled(t *testing.T) {
	config := Config{
		Enabled:     false,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown(context.Background())

	// Should still work, but be a noop
	ctx, span := StartSpan(context.Background(), "test-operation")
	if span == nil {
		t.Fatal("Expected non-nil span (noop)")
	}

	AddAttribute(ctx, "key", "value")
	EndSpan(ctx)
}

// Test: Record error
func TestRecordError(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown(context.Background())

	ctx, span := StartSpan(context.Background(), "error-operation")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}

	// Record an error
	RecordError(ctx, "something went wrong")

	EndSpan(ctx)
}

// Test: Extract and inject trace context
func TestTraceContext(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown(context.Background())

	// Start a span
	ctx, span := StartSpan(context.Background(), "test-operation")
	if span == nil {
		t.Fatal("Expected non-nil span")
	}

	// Extract trace context
	traceID := GetTraceID(ctx)
	if traceID == "" {
		t.Error("Expected non-empty trace ID")
	}

	spanID := GetSpanID(ctx)
	if spanID == "" {
		t.Error("Expected non-empty span ID")
	}

	EndSpan(ctx)
}
