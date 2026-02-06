package datetime_test

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/skills/datetime"
)

func TestDateTime_Name(t *testing.T) {
	dt := datetime.NewDateTime()
	if dt.Name() != "datetime" {
		t.Errorf("Expected name 'datetime', got '%s'", dt.Name())
	}
}

func TestDateTime_Description(t *testing.T) {
	dt := datetime.NewDateTime()
	desc := dt.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestDateTime_InputSchema(t *testing.T) {
	dt := datetime.NewDateTime()
	schema := dt.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestDateTime_Execute_Now(t *testing.T) {
	dt := datetime.NewDateTime()
	ctx := context.Background()

	params := map[string]any{
		"operation": "now",
	}

	result, err := dt.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected non-empty output for 'now' operation")
	}
}

func TestDateTime_Execute_Format(t *testing.T) {
	dt := datetime.NewDateTime()
	ctx := context.Background()

	params := map[string]any{
		"operation": "format",
		"format":    "2006-01-02",
	}

	result, err := dt.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	// Check that output matches the format pattern (YYYY-MM-DD)
	_, parseErr := time.Parse("2006-01-02", result.Output)
	if parseErr != nil {
		t.Errorf("Output '%s' doesn't match expected format '2006-01-02'", result.Output)
	}
}

func TestDateTime_Execute_Unix(t *testing.T) {
	dt := datetime.NewDateTime()
	ctx := context.Background()

	params := map[string]any{
		"operation": "unix",
	}

	result, err := dt.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected non-empty output for 'unix' operation")
	}
}

func TestDateTime_Execute_InvalidOperation(t *testing.T) {
	dt := datetime.NewDateTime()
	ctx := context.Background()

	params := map[string]any{
		"operation": "invalid",
	}

	result, err := dt.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for invalid operation")
	}
}

func TestDateTime_Execute_MissingOperation(t *testing.T) {
	dt := datetime.NewDateTime()
	ctx := context.Background()

	params := map[string]any{}

	result, err := dt.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for missing operation parameter")
	}
}

func TestDateTime_RequiredPermissions(t *testing.T) {
	dt := datetime.NewDateTime()
	perms := dt.RequiredPermissions()

	// DateTime should not require any special permissions
	if len(perms) != 0 {
		t.Errorf("Expected no required permissions, got %d", len(perms))
	}
}

func TestDateTime_Config(t *testing.T) {
	dt := datetime.NewDateTime()
	config := dt.Config()

	// Check that config is returned (basic smoke test)
	_ = config
}
