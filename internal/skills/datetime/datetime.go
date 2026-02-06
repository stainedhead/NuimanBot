package datetime

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"nuimanbot/internal/domain"
)

// DateTime implements the domain.Skill interface for date and time operations.
type DateTime struct {
	config domain.SkillConfig
}

// NewDateTime creates a new DateTime skill instance.
func NewDateTime() *DateTime {
	return &DateTime{
		config: domain.SkillConfig{
			Enabled: true,
		},
	}
}

// Name returns the skill name.
func (d *DateTime) Name() string {
	return "datetime"
}

// Description returns a description of the datetime skill.
func (d *DateTime) Description() string {
	return "Provides current date and time information with various formatting options"
}

// InputSchema returns the JSON schema for the datetime's input parameters.
func (d *DateTime) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "The datetime operation to perform",
				"enum":        []string{"now", "format", "unix"},
			},
			"format": map[string]any{
				"type":        "string",
				"description": "Optional: time format string (Go time layout format)",
			},
		},
		"required": []string{"operation"},
	}
}

// Execute performs the datetime operation.
func (d *DateTime) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	// Extract operation parameter
	operation, ok := params["operation"].(string)
	if !ok {
		return &domain.SkillResult{
			Error: "missing or invalid 'operation' parameter",
		}, nil
	}

	now := time.Now()

	switch operation {
	case "now":
		// Return current time in RFC3339 format
		return &domain.SkillResult{
			Output:   now.Format(time.RFC3339),
			Metadata: map[string]any{"operation": "now", "format": "RFC3339"},
			Error:    "",
		}, nil

	case "format":
		// Return formatted time using provided format string
		format, ok := params["format"].(string)
		if !ok || format == "" {
			return &domain.SkillResult{
				Error: "missing or invalid 'format' parameter for 'format' operation",
			}, nil
		}
		return &domain.SkillResult{
			Output:   now.Format(format),
			Metadata: map[string]any{"operation": "format", "format": format},
			Error:    "",
		}, nil

	case "unix":
		// Return Unix timestamp
		timestamp := now.Unix()
		return &domain.SkillResult{
			Output:   strconv.FormatInt(timestamp, 10),
			Metadata: map[string]any{"operation": "unix", "timestamp": timestamp},
			Error:    "",
		}, nil

	default:
		return &domain.SkillResult{
			Error: fmt.Sprintf("unsupported operation: %s", operation),
		}, nil
	}
}

// RequiredPermissions returns the permissions required to execute this skill.
func (d *DateTime) RequiredPermissions() []domain.Permission {
	// DateTime doesn't require any special permissions
	return []domain.Permission{}
}

// Config returns the skill's configuration.
func (d *DateTime) Config() domain.SkillConfig {
	return d.config
}
