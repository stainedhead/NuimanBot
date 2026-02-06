package calculator

import (
	"context"
	"fmt"
	"strconv"

	"nuimanbot/internal/domain"
)

// Calculator implements the domain.Skill interface for basic arithmetic operations.
type Calculator struct {
	config domain.SkillConfig
}

// NewCalculator creates a new Calculator skill instance.
func NewCalculator() *Calculator {
	return &Calculator{
		config: domain.SkillConfig{
			Enabled: true,
		},
	}
}

// Name returns the skill name.
func (c *Calculator) Name() string {
	return "calculator"
}

// Description returns a description of the calculator skill.
func (c *Calculator) Description() string {
	return "Performs basic arithmetic operations: add, subtract, multiply, divide"
}

// InputSchema returns the JSON schema for the calculator's input parameters.
func (c *Calculator) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "The arithmetic operation to perform",
				"enum":        []string{"add", "subtract", "multiply", "divide"},
			},
			"a": map[string]any{
				"type":        "number",
				"description": "The first operand",
			},
			"b": map[string]any{
				"type":        "number",
				"description": "The second operand",
			},
		},
		"required": []string{"operation", "a", "b"},
	}
}

// Execute performs the calculator operation.
func (c *Calculator) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	// Extract parameters
	operation, ok := params["operation"].(string)
	if !ok {
		return &domain.SkillResult{
			Error: "missing or invalid 'operation' parameter",
		}, nil
	}

	a, ok := params["a"].(float64)
	if !ok {
		return &domain.SkillResult{
			Error: "missing or invalid 'a' parameter",
		}, nil
	}

	b, ok := params["b"].(float64)
	if !ok {
		return &domain.SkillResult{
			Error: "missing or invalid 'b' parameter",
		}, nil
	}

	// Perform the operation
	var result float64

	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return &domain.SkillResult{
				Error: "division by zero",
			}, nil
		}
		result = a / b
	default:
		return &domain.SkillResult{
			Error: fmt.Sprintf("unsupported operation: %s", operation),
		}, nil
	}

	return &domain.SkillResult{
		Output:   formatResult(result),
		Metadata: map[string]any{"operation": operation, "a": a, "b": b},
		Error:    "",
	}, nil
}

// RequiredPermissions returns the permissions required to execute this skill.
func (c *Calculator) RequiredPermissions() []domain.Permission {
	// Calculator doesn't require any special permissions
	return []domain.Permission{}
}

// Config returns the skill's configuration.
func (c *Calculator) Config() domain.SkillConfig {
	return c.config
}

// formatResult formats the result as a string, removing unnecessary decimal places.
func formatResult(f float64) string {
	// If the result is a whole number, format without decimals
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
