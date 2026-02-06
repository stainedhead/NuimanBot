package requestid_test

import (
	"context"
	"testing"

	"nuimanbot/internal/infrastructure/requestid"
)

func TestGenerate(t *testing.T) {
	id := requestid.Generate()

	if id == "" {
		t.Error("Expected non-empty request ID")
	}

	if len(id) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("Expected request ID length 32, got %d", len(id))
	}
}

func TestGenerate_Unique(t *testing.T) {
	id1 := requestid.Generate()
	id2 := requestid.Generate()

	if id1 == id2 {
		t.Error("Expected unique request IDs")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	id := "test-request-id"

	ctx = requestid.WithRequestID(ctx, id)

	retrieved := requestid.FromContext(ctx)
	if retrieved != id {
		t.Errorf("Expected request ID '%s', got '%s'", id, retrieved)
	}
}

func TestWithRequestID_GeneratesWhenEmpty(t *testing.T) {
	ctx := context.Background()

	ctx = requestid.WithRequestID(ctx, "")

	retrieved := requestid.FromContext(ctx)
	if retrieved == "" {
		t.Error("Expected generated request ID, got empty string")
	}
}

func TestFromContext_NoID(t *testing.T) {
	ctx := context.Background()

	id := requestid.FromContext(ctx)
	if id != "" {
		t.Errorf("Expected empty string, got '%s'", id)
	}
}

func TestFromContext_WithID(t *testing.T) {
	ctx := context.Background()
	expectedID := "my-request-id"

	ctx = requestid.WithRequestID(ctx, expectedID)
	id := requestid.FromContext(ctx)

	if id != expectedID {
		t.Errorf("Expected '%s', got '%s'", expectedID, id)
	}
}

func TestMustFromContext_ExistingID(t *testing.T) {
	ctx := context.Background()
	expectedID := "existing-id"

	ctx = requestid.WithRequestID(ctx, expectedID)
	newCtx, id := requestid.MustFromContext(ctx)

	if id != expectedID {
		t.Errorf("Expected '%s', got '%s'", expectedID, id)
	}

	// Context should be unchanged
	if requestid.FromContext(newCtx) != expectedID {
		t.Error("Context was modified when it shouldn't be")
	}
}

func TestMustFromContext_NoID(t *testing.T) {
	ctx := context.Background()

	newCtx, id := requestid.MustFromContext(ctx)

	if id == "" {
		t.Error("Expected generated request ID")
	}

	// New context should have the ID
	if requestid.FromContext(newCtx) != id {
		t.Error("New context doesn't have the generated ID")
	}
}

func TestLogAttrs_WithID(t *testing.T) {
	ctx := context.Background()
	expectedID := "log-test-id"

	ctx = requestid.WithRequestID(ctx, expectedID)
	attrs := requestid.LogAttrs(ctx)

	if attrs == nil {
		t.Fatal("Expected non-nil attributes")
	}

	if len(attrs) == 0 {
		t.Error("Expected non-empty attributes")
	}
}

func TestLogAttrs_NoID(t *testing.T) {
	ctx := context.Background()

	attrs := requestid.LogAttrs(ctx)

	if attrs != nil {
		t.Error("Expected nil attributes when no request ID")
	}
}

func TestLogger_WithID(t *testing.T) {
	ctx := context.Background()
	ctx = requestid.WithRequestID(ctx, "logger-test-id")

	logger := requestid.Logger(ctx)

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
}

func TestLogger_NoID(t *testing.T) {
	ctx := context.Background()

	logger := requestid.Logger(ctx)

	if logger == nil {
		t.Fatal("Expected non-nil logger (should return default)")
	}
}

func TestPropagation_ThroughMultipleLevels(t *testing.T) {
	ctx := context.Background()
	originalID := "propagation-test"

	// Level 1: Add request ID
	ctx = requestid.WithRequestID(ctx, originalID)

	// Level 2: Pass through function
	ctx = simulateProcessing(ctx)

	// Level 3: Verify it's still there
	retrievedID := requestid.FromContext(ctx)

	if retrievedID != originalID {
		t.Errorf("Request ID not propagated correctly. Expected '%s', got '%s'", originalID, retrievedID)
	}
}

// simulateProcessing simulates passing context through a function
func simulateProcessing(ctx context.Context) context.Context {
	// In real code, this would do actual work
	return ctx
}

func TestConcurrent_UniqueIDs(t *testing.T) {
	ids := make(chan string, 100)

	// Generate 100 IDs concurrently
	for i := 0; i < 100; i++ {
		go func() {
			ids <- requestid.Generate()
		}()
	}

	// Collect IDs
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := <-ids
		if seen[id] {
			t.Errorf("Duplicate request ID generated: %s", id)
		}
		seen[id] = true
	}
}
