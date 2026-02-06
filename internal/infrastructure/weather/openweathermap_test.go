package weather_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/infrastructure/weather"
)

func TestNewClient(t *testing.T) {
	client := weather.NewClient("test-api-key", 10)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
}

func TestGetCurrentWeather_Success(t *testing.T) {
	// Mock OpenWeatherMap API response
	mockResponse := map[string]interface{}{
		"name": "London",
		"main": map[string]interface{}{
			"temp":       15.5,
			"feels_like": 14.2,
			"humidity":   72,
			"pressure":   1013,
		},
		"weather": []interface{}{
			map[string]interface{}{
				"description": "light rain",
				"main":        "Rain",
			},
		},
		"wind": map[string]interface{}{
			"speed": 5.5,
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		q := r.URL.Query()
		if q.Get("q") != "London" {
			t.Errorf("Expected location 'London', got '%s'", q.Get("q"))
		}
		if q.Get("units") != "metric" {
			t.Errorf("Expected units 'metric', got '%s'", q.Get("units"))
		}
		if q.Get("appid") != "test-api-key" {
			t.Errorf("Expected appid 'test-api-key', got '%s'", q.Get("appid"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := weather.NewClientWithBaseURL("test-api-key", 10, server.URL)
	ctx := context.Background()

	result, err := client.GetCurrentWeather(ctx, "London", "metric")
	if err != nil {
		t.Fatalf("GetCurrentWeather() error = %v", err)
	}

	if result.Location != "London" {
		t.Errorf("Expected location 'London', got '%s'", result.Location)
	}
	if result.Temperature != 15.5 {
		t.Errorf("Expected temperature 15.5, got %f", result.Temperature)
	}
	if result.Humidity != 72 {
		t.Errorf("Expected humidity 72, got %d", result.Humidity)
	}
	if result.Description != "light rain" {
		t.Errorf("Expected description 'light rain', got '%s'", result.Description)
	}
}

func TestGetCurrentWeather_InvalidAPIKey(t *testing.T) {
	// Mock server returns 401 Unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cod":     401,
			"message": "Invalid API key",
		})
	}))
	defer server.Close()

	client := weather.NewClientWithBaseURL("invalid-key", 10, server.URL)
	ctx := context.Background()

	_, err := client.GetCurrentWeather(ctx, "London", "metric")
	if err == nil {
		t.Fatal("Expected error for invalid API key, got nil")
	}
}

func TestGetCurrentWeather_LocationNotFound(t *testing.T) {
	// Mock server returns 404 Not Found
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cod":     "404",
			"message": "city not found",
		})
	}))
	defer server.Close()

	client := weather.NewClientWithBaseURL("test-api-key", 10, server.URL)
	ctx := context.Background()

	_, err := client.GetCurrentWeather(ctx, "NonexistentCity", "metric")
	if err == nil {
		t.Fatal("Expected error for nonexistent city, got nil")
	}
}

func TestGetForecast_Success(t *testing.T) {
	// Mock OpenWeatherMap API forecast response
	mockResponse := map[string]interface{}{
		"city": map[string]interface{}{
			"name": "London",
		},
		"list": []interface{}{
			map[string]interface{}{
				"dt": 1609459200,
				"main": map[string]interface{}{
					"temp": 10.5,
				},
				"weather": []interface{}{
					map[string]interface{}{
						"description": "clear sky",
					},
				},
			},
			map[string]interface{}{
				"dt": 1609545600,
				"main": map[string]interface{}{
					"temp": 12.0,
				},
				"weather": []interface{}{
					map[string]interface{}{
						"description": "cloudy",
					},
				},
			},
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "London" {
			t.Errorf("Expected location 'London', got '%s'", q.Get("q"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := weather.NewClientWithBaseURL("test-api-key", 10, server.URL)
	ctx := context.Background()

	result, err := client.GetForecast(ctx, "London", "metric")
	if err != nil {
		t.Fatalf("GetForecast() error = %v", err)
	}

	if result.Location != "London" {
		t.Errorf("Expected location 'London', got '%s'", result.Location)
	}
	if len(result.Forecasts) != 2 {
		t.Errorf("Expected 2 forecasts, got %d", len(result.Forecasts))
	}
	if result.Forecasts[0].Temperature != 10.5 {
		t.Errorf("Expected temperature 10.5, got %f", result.Forecasts[0].Temperature)
	}
}

func TestGetCurrentWeather_EmptyLocation(t *testing.T) {
	client := weather.NewClient("test-api-key", 10)
	ctx := context.Background()

	_, err := client.GetCurrentWeather(ctx, "", "metric")
	if err == nil {
		t.Fatal("Expected error for empty location, got nil")
	}
}

func TestGetCurrentWeather_InvalidUnits(t *testing.T) {
	client := weather.NewClient("test-api-key", 10)
	ctx := context.Background()

	_, err := client.GetCurrentWeather(ctx, "London", "invalid")
	if err == nil {
		t.Fatal("Expected error for invalid units, got nil")
	}
}
