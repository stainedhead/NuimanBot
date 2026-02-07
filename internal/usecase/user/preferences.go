package user

import (
	"context"
	"fmt"

	"nuimanbot/internal/domain"
)

// SetPreferencesRepository sets the preferences repository for the service.
func (s *Service) SetPreferencesRepository(repo domain.PreferencesRepository) {
	s.prefsRepo = repo
}

// GetPreferences retrieves user preferences.
func (s *Service) GetPreferences(ctx context.Context, userID string) (domain.UserPreferences, error) {
	if s.prefsRepo == nil {
		return domain.DefaultUserPreferences(), nil
	}

	prefs, err := s.prefsRepo.Get(ctx, userID)
	if err != nil {
		// If not found, return defaults
		if err == domain.ErrNotFound {
			return domain.DefaultUserPreferences(), nil
		}
		return domain.UserPreferences{}, fmt.Errorf("failed to get preferences: %w", err)
	}

	return prefs, nil
}

// UpdatePreferences updates user preferences.
func (s *Service) UpdatePreferences(ctx context.Context, userID string, prefs domain.UserPreferences) error {
	if s.prefsRepo == nil {
		return fmt.Errorf("preferences repository not configured")
	}

	// Verify user exists
	if _, err := s.userRepo.GetUserByID(ctx, userID); err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	// Save preferences
	if err := s.prefsRepo.Save(ctx, userID, prefs); err != nil {
		return fmt.Errorf("failed to save preferences: %w", err)
	}

	return nil
}

// ResetPreferences resets user preferences to defaults.
func (s *Service) ResetPreferences(ctx context.Context, userID string) error {
	return s.UpdatePreferences(ctx, userID, domain.DefaultUserPreferences())
}
