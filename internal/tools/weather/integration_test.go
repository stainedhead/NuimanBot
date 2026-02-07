package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	weatherClient "nuimanbot/internal/infrastructure/weather"
)

func TestGetCurrentWeather_WithMockServer(t *testing.T) {
	// Create mock server that returns OpenWeatherMap-style response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/weather") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "London",
			"main": {
				"temp": 15.5,
				"feels_like": 13.2,
				"humidity": 75,
				"pressure": 1013
			},
			"weather": [
				{
					"description": "partly cloudy"
				}
			],
			"wind": {
				"speed": 5.2
			}
		}`))
	}))
	defer server.Close()

	// Create weather tool with mock server
	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{
		client: client,
	}

	result, err := w.getCurrentWeather(context.Background(), "London", "metric")
	if err != nil {
		t.Fatalf("getCurrentWeather() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected output to be non-empty")
	}

	if result.Metadata["location"] != "London" {
		t.Errorf("Expected location 'London', got %v", result.Metadata["location"])
	}

	if result.Metadata["temperature"] != 15.5 {
		t.Errorf("Expected temperature 15.5, got %v", result.Metadata["temperature"])
	}

	if result.Metadata["humidity"] != 75 {
		t.Errorf("Expected humidity 75, got %v", result.Metadata["humidity"])
	}
}

func TestGetCurrentWeather_ImperialUnits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify units parameter
		if r.URL.Query().Get("units") != "imperial" {
			t.Errorf("Expected units=imperial, got %s", r.URL.Query().Get("units"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"name": "New York",
			"main": {
				"temp": 68.0,
				"feels_like": 65.5,
				"humidity": 60,
				"pressure": 1015
			},
			"weather": [{"description": "clear sky"}],
			"wind": {"speed": 3.5}
		}`))
	}))
	defer server.Close()

	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{client: client}

	result, err := w.getCurrentWeather(context.Background(), "New York", "imperial")
	if err != nil {
		t.Fatalf("getCurrentWeather() returned error: %v", err)
	}

	if result.Metadata["units"] != "imperial" {
		t.Errorf("Expected units 'imperial', got %v", result.Metadata["units"])
	}
}

func TestGetForecast_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/forecast") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"city": {"name": "Tokyo"},
			"list": [
				{
					"dt": 1609459200,
					"main": {"temp": 12.5},
					"weather": [{"description": "cloudy"}]
				},
				{
					"dt": 1609470000,
					"main": {"temp": 13.2},
					"weather": [{"description": "partly cloudy"}]
				},
				{
					"dt": 1609480800,
					"main": {"temp": 14.0},
					"weather": [{"description": "clear"}]
				}
			]
		}`))
	}))
	defer server.Close()

	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{client: client}

	result, err := w.getForecast(context.Background(), "Tokyo", "metric")
	if err != nil {
		t.Fatalf("getForecast() returned error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}

	if result.Output == "" {
		t.Error("Expected output to be non-empty")
	}

	if result.Metadata["location"] != "Tokyo" {
		t.Errorf("Expected location 'Tokyo', got %v", result.Metadata["location"])
	}

	forecasts, ok := result.Metadata["forecasts"].([]map[string]any)
	if !ok {
		t.Fatal("Expected forecasts in metadata")
	}

	if len(forecasts) != 3 {
		t.Errorf("Expected 3 forecast entries, got %d", len(forecasts))
	}
}

func TestGetForecast_ManyEntries(t *testing.T) {
	// Test forecast with more than 8 entries (tests maxEntries logic)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"city": {"name": "Paris"},
			"list": [
				{"dt": 1609459200, "main": {"temp": 10.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609470000, "main": {"temp": 11.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609480800, "main": {"temp": 12.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609491600, "main": {"temp": 13.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609502400, "main": {"temp": 14.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609513200, "main": {"temp": 15.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609524000, "main": {"temp": 16.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609534800, "main": {"temp": 17.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609545600, "main": {"temp": 18.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609556400, "main": {"temp": 19.0}, "weather": [{"description": "rain"}]}
			]
		}`))
	}))
	defer server.Close()

	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{client: client}

	result, err := w.getForecast(context.Background(), "Paris", "metric")
	if err != nil {
		t.Fatalf("getForecast() returned error: %v", err)
	}

	// All 10 entries should be in metadata
	forecasts, ok := result.Metadata["forecasts"].([]map[string]any)
	if !ok {
		t.Fatal("Expected forecasts in metadata")
	}

	if len(forecasts) != 10 {
		t.Errorf("Expected all 10 forecast entries in metadata, got %d", len(forecasts))
	}
}

func TestGetForecast_FewEntries(t *testing.T) {
	// Test forecast with fewer than 8 entries
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"city": {"name": "Berlin"},
			"list": [
				{"dt": 1609459200, "main": {"temp": 5.0}, "weather": [{"description": "rain"}]},
				{"dt": 1609470000, "main": {"temp": 6.0}, "weather": [{"description": "drizzle"}]}
			]
		}`))
	}))
	defer server.Close()

	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{client: client}

	result, err := w.getForecast(context.Background(), "Berlin", "metric")
	if err != nil {
		t.Fatalf("getForecast() returned error: %v", err)
	}

	forecasts, ok := result.Metadata["forecasts"].([]map[string]any)
	if !ok {
		t.Fatal("Expected forecasts in metadata")
	}

	if len(forecasts) != 2 {
		t.Errorf("Expected 2 forecast entries, got %d", len(forecasts))
	}
}

func TestGetForecast_StandardUnits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify units parameter
		if r.URL.Query().Get("units") != "standard" {
			t.Errorf("Expected units=standard, got %s", r.URL.Query().Get("units"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"city": {"name": "Moscow"},
			"list": [
				{"dt": 1609459200, "main": {"temp": 273.15}, "weather": [{"description": "freezing"}]}
			]
		}`))
	}))
	defer server.Close()

	client := weatherClient.NewClientWithBaseURL("test-key", 10, server.URL)
	w := &Weather{client: client}

	result, err := w.getForecast(context.Background(), "Moscow", "standard")
	if err != nil {
		t.Fatalf("getForecast() returned error: %v", err)
	}

	if result.Metadata["units"] != "standard" {
		t.Errorf("Expected units 'standard', got %v", result.Metadata["units"])
	}
}
