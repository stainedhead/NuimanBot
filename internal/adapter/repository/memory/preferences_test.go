package memory

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

func TestPreferencesRepository_SaveAndGet(t *testing.T) {
	repo := NewPreferencesRepository()
	ctx := context.Background()

	// Create preferences
	temp := 0.8
	maxTokens := 2048
	prefs := domain.UserPreferences{
		PreferredProvider: domain.LLMProviderOpenAI,
		PreferredModel:    "gpt-4",
		Temperature:       &temp,
		MaxTokens:         &maxTokens,
		ResponseFormat:    "json",
		StreamEnabled:     true,
	}

	// Save preferences
	err := repo.Save(ctx, "user1", prefs)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Retrieve preferences
	retrieved, err := repo.Get(ctx, "user1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Verify
	if retrieved.PreferredProvider != domain.LLMProviderOpenAI {
		t.Errorf("Expected provider OpenAI, got %s", retrieved.PreferredProvider)
	}

	if retrieved.GetTemperature() != 0.8 {
		t.Errorf("Expected temperature 0.8, got %f", retrieved.GetTemperature())
	}

	if retrieved.GetMaxTokens() != 2048 {
		t.Errorf("Expected max tokens 2048, got %d", retrieved.GetMaxTokens())
	}

	if !retrieved.StreamEnabled {
		t.Error("Expected streaming to be enabled")
	}
}

func TestPreferencesRepository_GetNotFound(t *testing.T) {
	repo := NewPreferencesRepository()
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if err != domain.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestPreferencesRepository_Delete(t *testing.T) {
	repo := NewPreferencesRepository()
	ctx := context.Background()

	// Save preferences
	prefs := domain.DefaultUserPreferences()
	err := repo.Save(ctx, "user1", prefs)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete preferences
	err = repo.Delete(ctx, "user1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = repo.Get(ctx, "user1")
	if err != domain.ErrNotFound {
		t.Errorf("Expected ErrNotFound after delete, got %v", err)
	}
}

func TestPreferencesRepository_UpdateOverwrites(t *testing.T) {
	repo := NewPreferencesRepository()
	ctx := context.Background()

	// Save initial preferences
	temp1 := 0.5
	prefs1 := domain.UserPreferences{
		Temperature: &temp1,
	}
	err := repo.Save(ctx, "user1", prefs1)
	if err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Update preferences
	temp2 := 0.9
	prefs2 := domain.UserPreferences{
		Temperature: &temp2,
	}
	err = repo.Save(ctx, "user1", prefs2)
	if err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.Get(ctx, "user1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.GetTemperature() != 0.9 {
		t.Errorf("Expected temperature 0.9 after update, got %f", retrieved.GetTemperature())
	}
}
