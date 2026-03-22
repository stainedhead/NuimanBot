package calculator_test

import (
	"context"
	"testing"

	"nuimanbot/internal/tools/calculator"
)

func TestCalculator_Execute_MissingOperation(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"a": float64(5),
		"b": float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing operation")
	}
}

func TestCalculator_Execute_MissingA(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "add",
		"b":         float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing 'a' param")
	}
}

func TestCalculator_Execute_FloatResult(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	// 10 / 3 = 3.3333...
	params := map[string]any{
		"operation": "divide",
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
	// Result should not be an integer representation
	if result.Output == "3" {
		t.Error("10/3 should not return integer 3")
	}
}

func TestCalculator_Execute_NegativeNumbers(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		a, b      float64
		want      string
	}{
		{"add negative", "add", -5, 3, "-2"},
		{"subtract to negative", "subtract", 3, 10, "-7"},
		{"multiply negative", "multiply", -4, 5, "-20"},
		{"divide negative", "divide", -10, 2, "-5"},
	}

	calc := calculator.NewCalculator()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]any{
				"operation": tt.operation,
				"a":         tt.a,
				"b":         tt.b,
			}

			result, err := calc.Execute(ctx, params)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.Error != "" {
				t.Errorf("Expected no error, got: %s", result.Error)
			}
			if result.Output != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, result.Output)
			}
		})
	}
}

func TestCalculator_Execute_LargeNumbers(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "multiply",
		"a":         float64(1000000),
		"b":         float64(1000000),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error for large numbers, got: %s", result.Error)
	}
	if result.Output != "1000000000000" {
		t.Errorf("Expected 1000000000000, got %s", result.Output)
	}
}

func TestCalculator_Execute_ZeroOperands(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	// 0 + 0 = 0
	params := map[string]any{
		"operation": "add",
		"a":         float64(0),
		"b":         float64(0),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
	if result.Output != "0" {
		t.Errorf("Expected 0, got %s", result.Output)
	}
}

func TestCalculator_Execute_Metadata(t *testing.T) {
	calc := calculator.NewCalculator()
	ctx := context.Background()

	params := map[string]any{
		"operation": "add",
		"a":         float64(2),
		"b":         float64(3),
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Metadata == nil {
		t.Error("Expected metadata to be non-nil")
	}
	if result.Metadata["operation"] != "add" {
		t.Errorf("Expected metadata operation 'add', got %v", result.Metadata["operation"])
	}
}
