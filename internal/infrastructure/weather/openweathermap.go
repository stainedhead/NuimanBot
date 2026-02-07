package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL = "https://api.openweathermap.org/data/2.5"
)

// Client represents an OpenWeatherMap API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// CurrentWeather represents current weather data for a location.
type CurrentWeather struct {
	Location    string
	Temperature float64
	FeelsLike   float64
	Humidity    int
	Pressure    int
	Description string
	WindSpeed   float64
	Units       string
}

// ForecastData represents forecast weather data for a location.
type ForecastData struct {
	Location  string
	Forecasts []ForecastEntry
	Units     string
}

// ForecastEntry represents a single forecast entry.
type ForecastEntry struct {
	Timestamp   int64
	Temperature float64
	Description string
}

// NewClient creates a new OpenWeatherMap client with default base URL.
func NewClient(apiKey string, timeoutSeconds int) *Client {
	return NewClientWithBaseURL(apiKey, timeoutSeconds, defaultBaseURL)
}

// NewClientWithBaseURL creates a new OpenWeatherMap client with custom base URL.
func NewClientWithBaseURL(apiKey string, timeoutSeconds int, baseURL string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		baseURL: baseURL,
	}
}

// validateInputs validates location and units parameters.
func (c *Client) validateInputs(location, units string) error {
	if location == "" {
		return fmt.Errorf("location cannot be empty")
	}
	if units != "metric" && units != "imperial" && units != "standard" {
		return fmt.Errorf("invalid units: %s (must be metric, imperial, or standard)", units)
	}
	return nil
}

// makeRequest performs an HTTP GET request to the OpenWeatherMap API.
func (c *Client) makeRequest(ctx context.Context, endpoint, location, units string) (map[string]interface{}, error) {
	// Build request URL
	reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	params := url.Values{}
	params.Set("q", location)
	params.Set("units", units)
	params.Set("appid", c.apiKey)

	fullURL := fmt.Sprintf("%s?%s", reqURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errorResp) //nolint:errcheck // Best effort error message extraction
		return nil, fmt.Errorf("API error (status %d): %v", resp.StatusCode, errorResp["message"])
	}

	// Parse response
	var apiResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return apiResp, nil
}

// GetCurrentWeather fetches current weather for a location.
func (c *Client) GetCurrentWeather(ctx context.Context, location, units string) (*CurrentWeather, error) {
	// Validate inputs
	if err := c.validateInputs(location, units); err != nil {
		return nil, err
	}

	// Make API request
	apiResp, err := c.makeRequest(ctx, "/weather", location, units)
	if err != nil {
		return nil, err
	}

	// Extract weather data
	return c.parseCurrentWeather(apiResp, units)
}

// GetForecast fetches 5-day forecast for a location.
func (c *Client) GetForecast(ctx context.Context, location, units string) (*ForecastData, error) {
	// Validate inputs
	if err := c.validateInputs(location, units); err != nil {
		return nil, err
	}

	// Make API request
	apiResp, err := c.makeRequest(ctx, "/forecast", location, units)
	if err != nil {
		return nil, err
	}

	// Extract forecast data
	return c.parseForecast(apiResp, units)
}

// parseCurrentWeather extracts current weather data from API response.
func (c *Client) parseCurrentWeather(apiResp map[string]interface{}, units string) (*CurrentWeather, error) {
	main, ok := apiResp["main"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing 'main' field")
	}

	weather, ok := apiResp["weather"].([]interface{})
	if !ok || len(weather) == 0 {
		return nil, fmt.Errorf("invalid response format: missing 'weather' field")
	}

	weatherData, ok := weather[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: invalid 'weather' data")
	}

	// Extract wind speed (optional)
	windSpeed := 0.0
	if wind, ok := apiResp["wind"].(map[string]interface{}); ok {
		windSpeed, _ = wind["speed"].(float64)
	}

	name, _ := apiResp["name"].(string)

	return &CurrentWeather{
		Location:    name,
		Temperature: main["temp"].(float64),
		FeelsLike:   main["feels_like"].(float64),
		Humidity:    int(main["humidity"].(float64)),
		Pressure:    int(main["pressure"].(float64)),
		Description: weatherData["description"].(string),
		WindSpeed:   windSpeed,
		Units:       units,
	}, nil
}

// parseForecast extracts forecast data from API response.
func (c *Client) parseForecast(apiResp map[string]interface{}, units string) (*ForecastData, error) {
	// Extract city name
	city, ok := apiResp["city"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing 'city' field")
	}
	cityName, _ := city["name"].(string)

	// Extract forecast list
	list, ok := apiResp["list"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing 'list' field")
	}

	// Convert to ForecastEntry slice
	forecasts := make([]ForecastEntry, 0, len(list))
	for _, item := range list {
		entry := c.parseForecastEntry(item)
		if entry != nil {
			forecasts = append(forecasts, *entry)
		}
	}

	return &ForecastData{
		Location:  cityName,
		Forecasts: forecasts,
		Units:     units,
	}, nil
}

// parseForecastEntry extracts a single forecast entry from API response.
func (c *Client) parseForecastEntry(item interface{}) *ForecastEntry {
	entry, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	main, ok := entry["main"].(map[string]interface{})
	if !ok {
		return nil
	}

	weather, ok := entry["weather"].([]interface{})
	if !ok || len(weather) == 0 {
		return nil
	}

	weatherData, ok := weather[0].(map[string]interface{})
	if !ok {
		return nil
	}

	dt, _ := entry["dt"].(float64)
	temp, _ := main["temp"].(float64)
	desc, _ := weatherData["description"].(string)

	return &ForecastEntry{
		Timestamp:   int64(dt),
		Temperature: temp,
		Description: desc,
	}
}
