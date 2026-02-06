package weather

import (
	"context"
	"fmt"
	"time"

	"nuimanbot/internal/domain"
	weatherClient "nuimanbot/internal/infrastructure/weather"
)

// Weather implements the domain.Skill interface for weather information.
type Weather struct {
	client *weatherClient.Client
	config domain.SkillConfig
}

// NewWeather creates a new Weather skill.
func NewWeather(apiKey string, timeoutSeconds int) *Weather {
	return &Weather{
		client: weatherClient.NewClient(apiKey, timeoutSeconds),
		config: domain.SkillConfig{
			Enabled: true,
		},
	}
}

// Name returns the skill name.
func (w *Weather) Name() string {
	return "weather"
}

// Description returns the skill description.
func (w *Weather) Description() string {
	return "Get current weather or forecast for any location using OpenWeatherMap API"
}

// InputSchema returns the JSON schema for the skill's input parameters.
func (w *Weather) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "Operation to perform: 'current' for current weather, 'forecast' for 5-day forecast",
				"enum":        []string{"current", "forecast"},
			},
			"location": map[string]any{
				"type":        "string",
				"description": "City name or location (e.g., 'London', 'New York, US', 'Tokyo, JP')",
			},
			"units": map[string]any{
				"type":        "string",
				"description": "Temperature units: 'metric' (Celsius), 'imperial' (Fahrenheit), or 'standard' (Kelvin)",
				"enum":        []string{"metric", "imperial", "standard"},
				"default":     "metric",
			},
		},
		"required": []string{"operation", "location"},
	}
}

// Execute performs the weather skill operation.
func (w *Weather) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	// Extract and validate operation
	operation, ok := params["operation"].(string)
	if !ok || operation == "" {
		return &domain.SkillResult{
			Error: "missing operation parameter",
		}, nil
	}

	// Extract and validate location
	location, ok := params["location"].(string)
	if !ok || location == "" {
		return &domain.SkillResult{
			Error: "missing location parameter",
		}, nil
	}

	// Extract units (optional, default to metric)
	units := "metric"
	if u, ok := params["units"].(string); ok && u != "" {
		units = u
	}

	// Validate units
	if units != "metric" && units != "imperial" && units != "standard" {
		return &domain.SkillResult{
			Error: fmt.Sprintf("invalid units: %s (must be metric, imperial, or standard)", units),
		}, nil
	}

	// Execute operation
	switch operation {
	case "current":
		return w.getCurrentWeather(ctx, location, units)
	case "forecast":
		return w.getForecast(ctx, location, units)
	default:
		return &domain.SkillResult{
			Error: fmt.Sprintf("invalid operation: %s (must be 'current' or 'forecast')", operation),
		}, nil
	}
}

// getCurrentWeather fetches current weather for a location.
func (w *Weather) getCurrentWeather(ctx context.Context, location, units string) (*domain.SkillResult, error) {
	current, err := w.client.GetCurrentWeather(ctx, location, units)
	if err != nil {
		return &domain.SkillResult{
			Error: fmt.Sprintf("failed to get current weather: %v", err),
		}, nil
	}

	// Format response
	unitsSymbol := w.getUnitsSymbol(units)
	output := fmt.Sprintf("Current weather in %s:\n", current.Location)
	output += fmt.Sprintf("Temperature: %.1f%s (feels like %.1f%s)\n", current.Temperature, unitsSymbol, current.FeelsLike, unitsSymbol)
	output += fmt.Sprintf("Conditions: %s\n", current.Description)
	output += fmt.Sprintf("Humidity: %d%%\n", current.Humidity)
	output += fmt.Sprintf("Pressure: %d hPa\n", current.Pressure)
	output += fmt.Sprintf("Wind Speed: %.1f m/s", current.WindSpeed)

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"location":    current.Location,
			"temperature": current.Temperature,
			"feels_like":  current.FeelsLike,
			"humidity":    current.Humidity,
			"pressure":    current.Pressure,
			"description": current.Description,
			"wind_speed":  current.WindSpeed,
			"units":       units,
		},
	}, nil
}

// getForecast fetches 5-day forecast for a location.
func (w *Weather) getForecast(ctx context.Context, location, units string) (*domain.SkillResult, error) {
	forecast, err := w.client.GetForecast(ctx, location, units)
	if err != nil {
		return &domain.SkillResult{
			Error: fmt.Sprintf("failed to get forecast: %v", err),
		}, nil
	}

	// Format response (show first 8 entries = ~24 hours)
	unitsSymbol := w.getUnitsSymbol(units)
	output := fmt.Sprintf("Weather forecast for %s:\n\n", forecast.Location)

	maxEntries := 8
	if len(forecast.Forecasts) < maxEntries {
		maxEntries = len(forecast.Forecasts)
	}

	for i := 0; i < maxEntries; i++ {
		entry := forecast.Forecasts[i]
		timestamp := time.Unix(entry.Timestamp, 0)
		output += fmt.Sprintf("%s: %.1f%s - %s\n",
			timestamp.Format("Mon 15:04"),
			entry.Temperature,
			unitsSymbol,
			entry.Description,
		)
	}

	// Convert forecasts for metadata field
	forecastsData := make([]map[string]any, len(forecast.Forecasts))
	for i, entry := range forecast.Forecasts {
		forecastsData[i] = map[string]any{
			"timestamp":   entry.Timestamp,
			"temperature": entry.Temperature,
			"description": entry.Description,
		}
	}

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"location":  forecast.Location,
			"forecasts": forecastsData,
			"units":     units,
		},
	}, nil
}

// getUnitsSymbol returns the temperature symbol for the given units.
func (w *Weather) getUnitsSymbol(units string) string {
	switch units {
	case "metric":
		return "°C"
	case "imperial":
		return "°F"
	case "standard":
		return "K"
	default:
		return ""
	}
}

// RequiredPermissions returns the permissions required for this skill.
func (w *Weather) RequiredPermissions() []domain.Permission {
	// Weather skill requires network permission to call external API
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the skill's configuration.
func (w *Weather) Config() domain.SkillConfig {
	return w.config
}
