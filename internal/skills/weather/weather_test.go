package weather_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/skills/weather"
)

func TestWeatherSkill_Metadata(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	if skill.Name() != "weather" {
		t.Errorf("Expected name 'weather', got '%s'", skill.Name())
	}

	if skill.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := skill.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}

	// Check required fields in schema
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Schema should have properties field")
	}

	if _, ok := props["operation"]; !ok {
		t.Error("Schema should have operation property")
	}

	if _, ok := props["location"]; !ok {
		t.Error("Schema should have location property")
	}
}

func TestWeatherSkill_Execute_MissingOperation(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"location": "London",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing operation")
	}
}

func TestWeatherSkill_Execute_MissingLocation(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "current",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing location")
	}
}

func TestWeatherSkill_Execute_InvalidOperation(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "invalid_operation",
		"location":  "London",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for invalid operation")
	}
}

func TestWeatherSkill_Execute_InvalidUnits(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "London",
		"units":     "invalid_units",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for invalid units")
	}
}

func TestWeatherSkill_Execute_DefaultUnits(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	// This will fail with API error because we don't have a valid API key,
	// but it tests that default units are handled (no validation error)
	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "London",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Should get an API error, not a parameter validation error
	if result.Error != "" && (result.Error == "missing operation parameter" || result.Error == "missing location parameter") {
		t.Error("Should not get parameter validation error for optional units")
	}
}

func TestWeatherSkill_RequiredPermissions(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	perms := skill.RequiredPermissions()
	if len(perms) != 1 {
		t.Errorf("Expected 1 permission, got %d", len(perms))
	}
	if len(perms) > 0 && perms[0] != domain.PermissionNetwork {
		t.Errorf("Expected PermissionNetwork, got %v", perms[0])
	}
}

func TestWeatherSkill_Config(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	config := skill.Config()
	if !config.Enabled {
		t.Error("Expected skill to be enabled by default")
	}
}

func TestWeatherSkill_Execute_EmptyLocation(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for empty location")
	}
}

func TestWeatherSkill_Execute_Forecast_MissingLocation(t *testing.T) {
	skill := weather.NewWeather("test-api-key", 10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"operation": "forecast",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing location")
	}
}
