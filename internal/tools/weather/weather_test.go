package weather_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/tools/weather"
)

func TestWeatherSkill_Metadata(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	if tool.Name() != "weather" {
		t.Errorf("Expected name 'weather', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := tool.InputSchema()
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
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	// This will fail with API error because we don't have a valid API key,
	// but it tests that default units are handled (no validation error)
	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	perms := tool.RequiredPermissions()
	if len(perms) != 1 {
		t.Errorf("Expected 1 permission, got %d", len(perms))
	}
	if len(perms) > 0 && perms[0] != domain.PermissionNetwork {
		t.Errorf("Expected PermissionNetwork, got %v", perms[0])
	}
}

func TestWeatherSkill_Config(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	config := tool.Config()
	if !config.Enabled {
		t.Error("Expected tool to be enabled by default")
	}
}

func TestWeatherSkill_Execute_EmptyLocation(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
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
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "forecast",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing location")
	}
}

func TestWeatherSkill_Execute_Current_MetricUnits(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "London",
		"units":     "metric",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Will fail with API error due to invalid key, but tests parameter handling
	// The error should be from the API, not from parameter validation
	if result.Error != "" && result.Error == "missing location parameter" {
		t.Error("Should not get parameter validation error")
	}
}

func TestWeatherSkill_Execute_Current_ImperialUnits(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "New York",
		"units":     "imperial",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Will fail with API error, but tests imperial units parameter
	if result.Error != "" && result.Error == "missing location parameter" {
		t.Error("Should not get parameter validation error")
	}
}

func TestWeatherSkill_Execute_Current_StandardUnits(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "current",
		"location":  "Tokyo",
		"units":     "standard",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Will fail with API error, but tests standard units parameter
	if result.Error != "" && result.Error == "missing location parameter" {
		t.Error("Should not get parameter validation error")
	}
}

func TestWeatherSkill_Execute_Forecast_WithUnits(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "forecast",
		"location":  "Paris",
		"units":     "metric",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Will fail with API error, but tests forecast with units
	if result.Error != "" && result.Error == "missing location parameter" {
		t.Error("Should not get parameter validation error")
	}
}

func TestWeatherSkill_Execute_Forecast_InvalidUnits(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "forecast",
		"location":  "Berlin",
		"units":     "celsius",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for invalid units")
	}
}

func TestWeatherSkill_Execute_Forecast_EmptyLocation(t *testing.T) {
	tool := weather.NewWeather("test-api-key", 10)

	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "forecast",
		"location":  "",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for empty location")
	}
}
