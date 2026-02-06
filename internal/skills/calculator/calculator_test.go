package calculator_test

import (
	"context"
	"testing"

	"nuimanbot/internal/skills/calculator"
)

func TestCalculator_Name(t *testing.T) {
	calc := calculator.NewCalculator()
	if calc.Name() != "calculator" {
		t.Errorf("Expected name 'calculator', got '%s'", calc.Name())
	}
}

func TestCalculator_Description(t *testing.T) {
	calc := calculator.NewCalculator()
	desc := calc.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestCalculator_InputSchema(t *testing.T) {
	calc := calculator.NewCalculator()
	schema := calc.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestCalculator_Execute_Add(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "add",
		"a":         float64(5),
		"b":         float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output != "8" {
		t.Errorf("Expected output '8', got '%s'", result.Output)
	}
}

func TestCalculator_Execute_Subtract(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "subtract",
		"a":         float64(10),
		"b":         float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output != "7" {
		t.Errorf("Expected output '7', got '%s'", result.Output)
	}
}

func TestCalculator_Execute_Multiply(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "multiply",
		"a":         float64(4),
		"b":         float64(5),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output != "20" {
		t.Errorf("Expected output '20', got '%s'", result.Output)
	}
}

func TestCalculator_Execute_Divide(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "divide",
		"a":         float64(20),
		"b":         float64(4),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output != "5" {
		t.Errorf("Expected output '5', got '%s'", result.Output)
	}
}

func TestCalculator_Execute_DivideByZero(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "divide",
		"a":         float64(10),
		"b":         float64(0),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error message for divide by zero")
	}

	if result.Error != "division by zero" {
		t.Errorf("Expected 'division by zero' error, got: %s", result.Error)
	}
}

func TestCalculator_Execute_InvalidOperation(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "invalid",
		"a":         float64(5),
		"b":         float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for invalid operation")
	}
}

func TestCalculator_Execute_MissingParams(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "add",
		"a":         float64(5),
		// Missing "b"
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Error == "" {
		t.Error("Expected error for missing parameters")
	}
}

func TestCalculator_RequiredPermissions(t *testing.T) {
	calc := calculator.NewCalculator()
	perms := calc.RequiredPermissions()

	// Calculator should not require any special permissions
	if len(perms) != 0 {
		t.Errorf("Expected no required permissions, got %d", len(perms))
	}
}

func TestCalculator_Config(t *testing.T) {
	calc := calculator.NewCalculator()
	config := calc.Config()

	// Check that config is returned (basic smoke test)
	_ = config
}
