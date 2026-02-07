package user

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

// mockExtendedUserRepository is a mock implementation of ExtendedUserRepository.
type mockExtendedUserRepository struct {
	getUserByIDFunc         func(ctx context.Context, id string) (*domain.User, error)
	getUserByPlatformIDFunc func(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error)
	saveUserFunc            func(ctx context.Context, user *domain.User) error
	listAllFunc             func(ctx context.Context) ([]*domain.User, error)
	deleteFunc              func(ctx context.Context, userID string) error
}

func (m *mockExtendedUserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, id)
	}
	return &domain.User{ID: id}, nil
}

func (m *mockExtendedUserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	if m.getUserByPlatformIDFunc != nil {
		return m.getUserByPlatformIDFunc(ctx, platform, platformUID)
	}
	return nil, domain.ErrUserNotFound
}

func (m *mockExtendedUserRepository) SaveUser(ctx context.Context, user *domain.User) error {
	if m.saveUserFunc != nil {
		return m.saveUserFunc(ctx, user)
	}
	return nil
}

func (m *mockExtendedUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	if m.listAllFunc != nil {
		return m.listAllFunc(ctx)
	}
	return nil, nil
}

func (m *mockExtendedUserRepository) Delete(ctx context.Context, userID string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, userID)
	}
	return nil
}

// mockSecurityService is a mock implementation of SecurityService.
type mockSecurityService struct{}

func (m *mockSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (m *mockSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (m *mockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	return input, nil
}

func (m *mockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	return nil
}

// mockPreferencesRepository is a mock implementation of PreferencesRepository.
type mockPreferencesRepository struct {
	getFunc    func(ctx context.Context, userID string) (domain.UserPreferences, error)
	saveFunc   func(ctx context.Context, userID string, prefs domain.UserPreferences) error
	deleteFunc func(ctx context.Context, userID string) error
}

func (m *mockPreferencesRepository) Get(ctx context.Context, userID string) (domain.UserPreferences, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, userID)
	}
	return domain.UserPreferences{}, domain.ErrNotFound
}

func (m *mockPreferencesRepository) Save(ctx context.Context, userID string, prefs domain.UserPreferences) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, userID, prefs)
	}
	return nil
}

func (m *mockPreferencesRepository) Delete(ctx context.Context, userID string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, userID)
	}
	return nil
}

func TestGetPreferences_DefaultWhenNoPreferences(t *testing.T) {
	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}

	prefsRepo := &mockPreferencesRepository{
		getFunc: func(ctx context.Context, userID string) (domain.UserPreferences, error) {
			return domain.UserPreferences{}, domain.ErrNotFound
		},
	}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	prefs, err := service.GetPreferences(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetPreferences failed: %v", err)
	}

	// Should return defaults
	if prefs.GetTemperature() != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", prefs.GetTemperature())
	}

	if prefs.GetMaxTokens() != 1024 {
		t.Errorf("Expected default max tokens 1024, got %d", prefs.GetMaxTokens())
	}
}

func TestUpdatePreferences(t *testing.T) {
	var savedPrefs domain.UserPreferences
	var savedUserID string

	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}

	prefsRepo := &mockPreferencesRepository{
		saveFunc: func(ctx context.Context, userID string, prefs domain.UserPreferences) error {
			savedUserID = userID
			savedPrefs = prefs
			return nil
		},
	}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	// Update preferences
	temp := 0.5
	maxTokens := 2048
	prefs := domain.UserPreferences{
		PreferredProvider: domain.LLMProviderOpenAI,
		PreferredModel:    "gpt-4",
		Temperature:       &temp,
		MaxTokens:         &maxTokens,
		ResponseFormat:    "json",
	}

	err := service.UpdatePreferences(context.Background(), "user1", prefs)
	if err != nil {
		t.Fatalf("UpdatePreferences failed: %v", err)
	}

	// Verify preferences were saved
	if savedUserID != "user1" {
		t.Errorf("Expected user ID 'user1', got '%s'", savedUserID)
	}

	if savedPrefs.PreferredProvider != domain.LLMProviderOpenAI {
		t.Error("Preferences not saved correctly")
	}

	if savedPrefs.GetTemperature() != 0.5 {
		t.Errorf("Expected temperature 0.5, got %f", savedPrefs.GetTemperature())
	}
}

func TestResetPreferences(t *testing.T) {
	var savedPrefs domain.UserPreferences

	userRepo := &mockExtendedUserRepository{
		getUserByIDFunc: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}

	prefsRepo := &mockPreferencesRepository{
		saveFunc: func(ctx context.Context, userID string, prefs domain.UserPreferences) error {
			savedPrefs = prefs
			return nil
		},
		getFunc: func(ctx context.Context, userID string) (domain.UserPreferences, error) {
			return savedPrefs, nil
		},
	}

	service := NewService(userRepo, &mockSecurityService{})
	service.SetPreferencesRepository(prefsRepo)

	err := service.ResetPreferences(context.Background(), "user1")
	if err != nil {
		t.Fatalf("ResetPreferences failed: %v", err)
	}

	// Verify preferences were reset to defaults
	if savedPrefs.PreferredProvider != domain.LLMProviderAnthropic {
		t.Error("Expected default provider (Anthropic) after reset")
	}

	if savedPrefs.GetTemperature() != 0.7 {
		t.Errorf("Expected default temperature 0.7, got %f", savedPrefs.GetTemperature())
	}
}
