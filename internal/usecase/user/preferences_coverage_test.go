package user

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// TestGetPreferences_ErrorPath covers the error path in GetPreferences
func TestGetPreferences_ErrorPath(t *testing.T) {
	t.Parallel()

	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}

	prefsRepo := &mockPreferencesRepository{
		getFunc: func(ctx context.Context, userID string) (domain.UserPreferences, error) {
			return domain.UserPreferences{}, errors.New("database error")
		},
	}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	_, err := service.GetPreferences(context.Background(), "user1")
	if err == nil {
		t.Error("Expected error when preferences repository returns non-ErrNotFound error")
	}
}

// TestGetPreferences_NoRepository covers the nil prefsRepo path
func TestGetPreferences_NoRepository(t *testing.T) {
	t.Parallel()

	userRepo := &mockExtendedUserRepository{}
	service := NewService(userRepo, &mockSecurityService{})
	// Do NOT set preferences repository

	prefs, err := service.GetPreferences(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetPreferences without repo should return defaults: %v", err)
	}

	// Should return defaults
	if prefs.GetTemperature() != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", prefs.GetTemperature())
	}
}

// TestUpdatePreferences_UserNotFound covers the user not found path
func TestUpdatePreferences_UserNotFound(t *testing.T) {
	t.Parallel()

	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return nil, domain.ErrUserNotFound
		},
	}

	prefsRepo := &mockPreferencesRepository{}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	err := service.UpdatePreferences(context.Background(), "missing-user", domain.DefaultUserPreferences())
	if err == nil {
		t.Error("Expected error when user not found in UpdatePreferences")
	}
}

// TestUpdatePreferences_NoRepository covers the nil prefsRepo path in UpdatePreferences
func TestUpdatePreferences_NoRepository(t *testing.T) {
	t.Parallel()

	userRepo := &mockExtendedUserRepository{}
	service := NewService(userRepo, &mockSecurityService{})
	// Do NOT set preferences repository

	err := service.UpdatePreferences(context.Background(), "user1", domain.DefaultUserPreferences())
	if err == nil {
		t.Error("Expected error when preferences repo not configured")
	}
}

// TestUpdatePreferences_SaveError covers the save error path
func TestUpdatePreferences_SaveError(t *testing.T) {
	t.Parallel()

	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}

	prefsRepo := &mockPreferencesRepository{
		saveFunc: func(ctx context.Context, userID string, prefs domain.UserPreferences) error {
			return errors.New("save error")
		},
	}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	err := service.UpdatePreferences(context.Background(), "user1", domain.DefaultUserPreferences())
	if err == nil {
		t.Error("Expected error when preferences save fails")
	}
}
