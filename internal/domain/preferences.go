package domain

import "context"

// UserPreferences stores user-specific configuration preferences.
type UserPreferences struct {
	// LLM Preferences
	PreferredProvider LLMProvider `json:"preferred_provider,omitempty"` // anthropic, openai, ollama
	PreferredModel    string      `json:"preferred_model,omitempty"`    // e.g., claude-3-sonnet-20240229
	Temperature       *float64    `json:"temperature,omitempty"`        // 0.0-1.0, nil uses default
	MaxTokens         *int        `json:"max_tokens,omitempty"`         // nil uses default

	// Response Preferences
	ResponseFormat string `json:"response_format,omitempty"` // markdown, text, json
	StreamEnabled  bool   `json:"stream_enabled"`            // Enable streaming responses

	// Conversation Preferences
	ContextWindowSize *int `json:"context_window_size,omitempty"` // Max tokens for context, nil uses provider limit
}

// DefaultUserPreferences returns default preferences for a new user.
func DefaultUserPreferences() UserPreferences {
	temp := 0.7
	maxTokens := 1024

	return UserPreferences{
		PreferredProvider: LLMProviderAnthropic,
		PreferredModel:    "claude-3-sonnet-20240229",
		Temperature:       &temp,
		MaxTokens:         &maxTokens,
		ResponseFormat:    "markdown",
		StreamEnabled:     false,
	}
}

// GetTemperature returns the temperature value or default if not set.
func (p UserPreferences) GetTemperature() float64 {
	if p.Temperature != nil {
		return *p.Temperature
	}
	return 0.7 // default
}

// GetMaxTokens returns the max tokens value or default if not set.
func (p UserPreferences) GetMaxTokens() int {
	if p.MaxTokens != nil {
		return *p.MaxTokens
	}
	return 1024 // default
}

// GetResponseFormat returns the response format or default if not set.
func (p UserPreferences) GetResponseFormat() string {
	if p.ResponseFormat == "" {
		return "markdown"
	}
	return p.ResponseFormat
}

// PreferencesRepository defines the contract for user preferences persistence.
type PreferencesRepository interface {
	Get(ctx context.Context, userID string) (UserPreferences, error)
	Save(ctx context.Context, userID string, prefs UserPreferences) error
	Delete(ctx context.Context, userID string) error
}
