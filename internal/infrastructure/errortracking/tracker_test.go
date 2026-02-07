package errortracking

import (
	"context"
	"errors"
	"testing"
)

// Test: Initialize error tracker
func TestInitialize(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
		DSN:         "", // Empty DSN for testing
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// Test: Capture error
func TestCaptureError(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	// Capture an error
	testErr := errors.New("test error")
	CaptureError(context.Background(), testErr)
}

// Test: Capture error with context
func TestCaptureErrorWithContext(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	// Create context with metadata
	ctx := context.Background()
	ctx = WithUser(ctx, "user-123", "alice@example.com")
	ctx = WithTag(ctx, "component", "chat")
	ctx = WithExtra(ctx, "request_id", "req-456")

	// Capture error
	testErr := errors.New("error with context")
	CaptureError(ctx, testErr)
}

// Test: Capture message
func TestCaptureMessage(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	// Capture a message
	CaptureMessage(context.Background(), "info", "Test message")
}

// Test: Set severity levels
func TestSeverityLevels(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	ctx := context.Background()

	// Test different severity levels
	CaptureMessage(ctx, SeverityDebug, "Debug message")
	CaptureMessage(ctx, SeverityInfo, "Info message")
	CaptureMessage(ctx, SeverityWarning, "Warning message")
	CaptureMessage(ctx, SeverityError, "Error message")
	CaptureMessage(ctx, SeverityFatal, "Fatal message")
}

// Test: Disabled tracking
func TestDisabledTracking(t *testing.T) {
	config := Config{
		Enabled:     false,
		ServiceName: "nuimanbot-test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	// Should be noop
	CaptureError(context.Background(), errors.New("test error"))
	CaptureMessage(context.Background(), "info", "test message")
}

// Test: Breadcrumbs
func TestBreadcrumbs(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	ctx := context.Background()

	// Add breadcrumbs
	ctx = AddBreadcrumb(ctx, "navigation", "User clicked button", map[string]any{
		"button": "submit",
	})
	ctx = AddBreadcrumb(ctx, "api", "API call started", map[string]any{
		"endpoint": "/api/chat",
	})

	// Capture error with breadcrumbs
	CaptureError(ctx, errors.New("error with breadcrumbs"))
}

// Test: Fingerprinting
func TestFingerprinting(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "nuimanbot-test",
		Environment: "test",
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer Shutdown()

	ctx := context.Background()
	ctx = WithFingerprint(ctx, "custom-group-key")

	// Capture error with custom fingerprint
	CaptureError(ctx, errors.New("error with fingerprint"))
}
