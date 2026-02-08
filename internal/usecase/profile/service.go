package profile

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"nuimanbot/internal/domain"
)

// Service provides user profile management operations.
// It handles CRUD operations for user profiles and enforces business rules.
type Service struct {
	repo        domain.UserProfileRepository
	securitySvc domain.SecurityService
}

// NewService creates a new user profile management service.
func NewService(repo domain.UserProfileRepository, securitySvc domain.SecurityService) *Service {
	return &Service{
		repo:        repo,
		securitySvc: securitySvc,
	}
}

// CreateProfile creates a new user profile.
// Validates the profile and checks for uniqueness constraints before creating.
func (s *Service) CreateProfile(ctx context.Context, profile *domain.UserProfile) error {
	// Validate profile
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}

	// Check if profile already exists
	existing, _ := s.repo.GetProfileByUserID(ctx, profile.UserID)
	if existing != nil {
		return fmt.Errorf("profile already exists for user ID %s", profile.UserID)
	}

	// Check for duplicate email
	if profile.PrimaryEmail != "" {
		existingByEmail, _ := s.repo.GetProfileByEmail(ctx, profile.PrimaryEmail)
		if existingByEmail != nil {
			return fmt.Errorf("profile already exists with email %s", profile.PrimaryEmail)
		}
	}

	// Check for duplicate platform IDs
	if err := s.checkDuplicatePlatformIDs(ctx, profile); err != nil {
		return err
	}

	// Generate API key if not already set
	if profile.APIKey == "" {
		apiKey, err := s.securitySvc.GenerateAPIKey(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate API key: %w", err)
		}
		profile.APIKey = apiKey
	}

	// Set timestamps
	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	profile.LastVerified = now

	// Save profile
	if err := s.repo.SaveProfile(ctx, profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	// Audit profile creation
	s.auditSuccess(ctx, "profile_created", profile.UserID, map[string]any{
		"user_id":        profile.UserID,
		"email":          profile.PrimaryEmail,
		"user_type":      string(profile.UserType),
		"data_directory": profile.DataDirectory,
	})

	return nil
}

// GetProfile retrieves a user profile by user ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	return s.repo.GetProfileByUserID(ctx, userID)
}

// GetProfileByEmail retrieves a user profile by email address.
func (s *Service) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	return s.repo.GetProfileByEmail(ctx, email)
}

// GetProfileByPlatformID retrieves a user profile by platform-specific ID.
func (s *Service) GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error) {
	return s.repo.GetProfileByPlatformID(ctx, platform, platformID)
}

// GetByAPIKey retrieves a user profile by API key.
// Used for REST API authentication.
func (s *Service) GetByAPIKey(ctx context.Context, apiKey string) (*domain.UserProfile, error) {
	return s.repo.GetProfileByAPIKey(ctx, apiKey)
}

// UpdateProfile updates a user profile with partial updates.
// Only the fields present in the updates map will be modified.
func (s *Service) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	// Get existing profile
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	// Apply updates using reflection
	if err := s.applyFieldUpdates(profile, updates); err != nil {
		return err
	}

	// Validate updated profile
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("updated profile validation failed: %w", err)
	}

	// Check for uniqueness constraints if relevant fields were updated
	if err := s.checkUniquenessConstraints(ctx, userID, profile, updates); err != nil {
		return err
	}

	// Update timestamp
	profile.UpdatedAt = time.Now()

	// Save updated profile
	if err := s.repo.SaveProfile(ctx, profile); err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	// Audit profile update
	s.auditSuccess(ctx, "profile_updated", userID, map[string]any{
		"user_id": userID,
		"fields":  extractFieldNames(updates),
	})

	return nil
}

// DeleteProfile deletes a user profile.
func (s *Service) DeleteProfile(ctx context.Context, userID string) error {
	// Verify profile exists
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	// Delete profile
	if err := s.repo.DeleteProfile(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// Audit profile deletion
	s.auditSuccess(ctx, "profile_deleted", userID, map[string]any{
		"user_id":   userID,
		"email":     profile.PrimaryEmail,
		"user_type": string(profile.UserType),
	})

	return nil
}

// ListProfiles returns all user profiles with pagination support.
func (s *Service) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	return s.repo.ListProfiles(ctx, offset, limit)
}

// checkDuplicatePlatformIDs checks if any platform IDs in the profile are already in use.
func (s *Service) checkDuplicatePlatformIDs(ctx context.Context, profile *domain.UserProfile) error {
	return s.checkDuplicatePlatformIDsForUpdate(ctx, profile.UserID, profile)
}

// checkDuplicatePlatformIDsForUpdate checks platform ID uniqueness, excluding the specified user.
func (s *Service) checkDuplicatePlatformIDsForUpdate(ctx context.Context, excludeUserID string, profile *domain.UserProfile) error {
	if profile.PlatformIDs.Slack != "" {
		existing, _ := s.repo.GetProfileByPlatformID(ctx, domain.PlatformSlack, profile.PlatformIDs.Slack)
		if existing != nil && existing.UserID != excludeUserID {
			return fmt.Errorf("slack ID %s is already in use", profile.PlatformIDs.Slack)
		}
	}

	if profile.PlatformIDs.Telegram != "" {
		existing, _ := s.repo.GetProfileByPlatformID(ctx, domain.PlatformTelegram, profile.PlatformIDs.Telegram)
		if existing != nil && existing.UserID != excludeUserID {
			return fmt.Errorf("telegram ID %s is already in use", profile.PlatformIDs.Telegram)
		}
	}

	if profile.PlatformIDs.CLI != "" {
		existing, _ := s.repo.GetProfileByPlatformID(ctx, domain.PlatformCLI, profile.PlatformIDs.CLI)
		if existing != nil && existing.UserID != excludeUserID {
			return fmt.Errorf("CLI ID %s is already in use", profile.PlatformIDs.CLI)
		}
	}

	return nil
}

// auditSuccess logs a successful operation to the audit log.
func (s *Service) auditSuccess(ctx context.Context, action, resource string, details map[string]any) {
	_ = s.securitySvc.Audit(ctx, &domain.AuditEvent{ //nolint:errcheck // Best effort audit logging
		Timestamp: time.Now(),
		Action:    action,
		Resource:  resource,
		Outcome:   "success",
		Details:   details,
	})
}

// capitalizeFirst capitalizes the first letter of a string.
// Used for converting map keys to struct field names.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	// Check if first character is lowercase
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// applyFieldUpdates applies the updates map to the profile using reflection.
func (s *Service) applyFieldUpdates(profile *domain.UserProfile, updates map[string]interface{}) error {
	profileValue := reflect.ValueOf(profile).Elem()

	for key, value := range updates {
		fieldName := capitalizeFirst(key)
		field := profileValue.FieldByName(fieldName)

		if !field.IsValid() {
			return fmt.Errorf("invalid field: %s", key)
		}
		if !field.CanSet() {
			return fmt.Errorf("cannot set field: %s", key)
		}

		// Convert value to correct type and set
		valueToSet := reflect.ValueOf(value)
		if !valueToSet.Type().ConvertibleTo(field.Type()) {
			return fmt.Errorf("invalid type for field %s: expected %s, got %s",
				key, field.Type(), valueToSet.Type())
		}

		field.Set(valueToSet.Convert(field.Type()))
	}

	return nil
}

// checkUniquenessConstraints validates that updated fields don't violate uniqueness constraints.
func (s *Service) checkUniquenessConstraints(ctx context.Context, userID string, profile *domain.UserProfile, updates map[string]interface{}) error {
	// Check email uniqueness if email was updated
	if _, emailUpdated := updates["primaryEmail"]; emailUpdated {
		if err := s.checkEmailUniqueness(ctx, userID, profile.PrimaryEmail); err != nil {
			return err
		}
	}

	// Check platform ID uniqueness if any platform IDs were updated
	if _, platformUpdated := updates["platformIDs"]; platformUpdated {
		if err := s.checkDuplicatePlatformIDsForUpdate(ctx, userID, profile); err != nil {
			return err
		}
	}

	return nil
}

// checkEmailUniqueness verifies that an email is not already in use by another user.
func (s *Service) checkEmailUniqueness(ctx context.Context, userID, email string) error {
	if email == "" {
		return nil
	}

	existing, _ := s.repo.GetProfileByEmail(ctx, email)
	if existing != nil && existing.UserID != userID {
		return fmt.Errorf("email %s is already in use", email)
	}

	return nil
}

// extractFieldNames extracts field names from the updates map.
func extractFieldNames(updates map[string]interface{}) []string {
	fields := make([]string, 0, len(updates))
	for k := range updates {
		fields = append(fields, k)
	}
	return fields
}
