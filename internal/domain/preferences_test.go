package domain

import (
	"testing"
)

func TestDefaultUserPreferences(t *testing.T) {
	prefs := DefaultUserPreferences()

	if prefs.PreferredProvider != LLMProviderAnthropic {
		t.Errorf("PreferredProvider = %v, want %v", prefs.PreferredProvider, LLMProviderAnthropic)
	}
	if prefs.PreferredModel == "" {
		t.Error("PreferredModel should not be empty")
	}
	if prefs.Temperature == nil {
		t.Error("Temperature should not be nil")
	}
	if prefs.MaxTokens == nil {
		t.Error("MaxTokens should not be nil")
	}
	if prefs.ResponseFormat == "" {
		t.Error("ResponseFormat should not be empty")
	}
}

func TestUserPreferences_GetTemperature(t *testing.T) {
	t.Run("with value set", func(t *testing.T) {
		temp := 0.5
		prefs := UserPreferences{Temperature: &temp}
		if got := prefs.GetTemperature(); got != 0.5 {
			t.Errorf("GetTemperature() = %v, want 0.5", got)
		}
	})

	t.Run("nil returns default", func(t *testing.T) {
		prefs := UserPreferences{}
		if got := prefs.GetTemperature(); got != 0.7 {
			t.Errorf("GetTemperature() = %v, want 0.7 (default)", got)
		}
	})
}

func TestUserPreferences_GetMaxTokens(t *testing.T) {
	t.Run("with value set", func(t *testing.T) {
		mt := 2048
		prefs := UserPreferences{MaxTokens: &mt}
		if got := prefs.GetMaxTokens(); got != 2048 {
			t.Errorf("GetMaxTokens() = %v, want 2048", got)
		}
	})

	t.Run("nil returns default", func(t *testing.T) {
		prefs := UserPreferences{}
		if got := prefs.GetMaxTokens(); got != 1024 {
			t.Errorf("GetMaxTokens() = %v, want 1024 (default)", got)
		}
	})
}

func TestUserPreferences_GetResponseFormat(t *testing.T) {
	t.Run("with value set", func(t *testing.T) {
		prefs := UserPreferences{ResponseFormat: "json"}
		if got := prefs.GetResponseFormat(); got != "json" {
			t.Errorf("GetResponseFormat() = %v, want json", got)
		}
	})

	t.Run("empty returns default", func(t *testing.T) {
		prefs := UserPreferences{}
		if got := prefs.GetResponseFormat(); got != "markdown" {
			t.Errorf("GetResponseFormat() = %v, want markdown (default)", got)
		}
	})
}
