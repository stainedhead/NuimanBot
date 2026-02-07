package user

import (
	"context"
	"fmt"
	"time"

	"nuimanbot/internal/domain"

	"github.com/google/uuid"
)

// ExtendedUserRepository adds methods needed for user management
// that aren't yet in the base UserRepository interface.
// TODO: Move these to domain.UserRepository when ready for production.
type ExtendedUserRepository interface {
	domain.UserRepository
	ListAll(ctx context.Context) ([]*domain.User, error)
	Delete(ctx context.Context, userID string) error
}

// Service provides user management operations.
// It handles CRUD operations for users and enforces business rules.
type Service struct {
	userRepo    ExtendedUserRepository
	securitySvc domain.SecurityService
	prefsRepo   domain.PreferencesRepository // Optional preferences storage
}

// NewService creates a new user management service.
func NewService(userRepo ExtendedUserRepository, securitySvc domain.SecurityService) *Service {
	return &Service{
		userRepo:    userRepo,
		securitySvc: securitySvc,
	}
}

// CreateUser creates a new user in the system.
// Returns the created user or an error if the user already exists.
func (s *Service) CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error) {
	// Check if user already exists with this platform+platformUID
	existing, err := s.userRepo.GetUserByPlatformID(ctx, platform, platformUID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("user already exists with platform %s and UID %s: %w", platform, platformUID, domain.ErrConflict)
	}

	// Create new user
	user := &domain.User{
		ID:       uuid.New().String(),
		Username: platformUID, // Default username to platformUID
		Role:     role,
		PlatformIDs: map[domain.Platform]string{
			platform: platformUID,
		},
		AllowedSkills: []string{}, // Empty = all skills for role
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Save user
	if err := s.userRepo.SaveUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	// Audit user creation
	s.auditSuccess(ctx, "user_created", user.ID, map[string]any{
		"user_id":      user.ID,
		"platform":     string(platform),
		"platform_uid": platformUID,
		"role":         string(role),
	})

	return user, nil
}

// GetUser retrieves a user by their ID.
// Returns ErrUserNotFound if the user doesn't exist.
func (s *Service) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

// GetUserByPlatformUID retrieves a user by their platform and platform UID.
// Returns ErrUserNotFound if the user doesn't exist.
func (s *Service) GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	return s.userRepo.GetUserByPlatformID(ctx, platform, platformUID)
}

// ListUsers retrieves all users in the system.
// Requires admin permissions (enforced by caller).
func (s *Service) ListUsers(ctx context.Context) ([]*domain.User, error) {
	return s.userRepo.ListAll(ctx)
}

// UpdateUserRole updates a user's role.
// Enforces business rules (e.g., cannot demote last admin).
func (s *Service) UpdateUserRole(ctx context.Context, userID string, role domain.Role) error {
	// Get the user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// If demoting from admin, check if this is the last admin
	if user.Role == domain.RoleAdmin && role != domain.RoleAdmin {
		if err := s.checkNotLastAdmin(ctx, userID); err != nil {
			return err
		}
	}

	// Update role
	user.Role = role
	user.UpdatedAt = time.Now()

	// Save user
	if err := s.userRepo.SaveUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	// Audit role update
	s.auditSuccess(ctx, "user_role_updated", userID, map[string]any{
		"user_id":  userID,
		"new_role": string(role),
	})

	return nil
}

// UpdateAllowedSkills updates a user's allowed skills whitelist.
func (s *Service) UpdateAllowedSkills(ctx context.Context, userID string, skills []string) error {
	// Get the user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update allowed skills
	user.AllowedSkills = skills
	user.UpdatedAt = time.Now()

	// Save user
	if err := s.userRepo.SaveUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update allowed skills: %w", err)
	}

	// Audit skills update
	s.auditSuccess(ctx, "user_skills_updated", userID, map[string]any{
		"user_id": userID,
		"skills":  skills,
	})

	return nil
}

// DeleteUser deletes a user from the system.
// Enforces business rules (e.g., cannot delete last admin).
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	// Get the user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// If deleting an admin, check if this is the last admin
	if user.Role == domain.RoleAdmin {
		if err := s.checkNotLastAdmin(ctx, userID); err != nil {
			return err
		}
	}

	// Delete user
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Audit user deletion
	s.auditSuccess(ctx, "user_deleted", userID, map[string]any{
		"user_id": userID,
		"role":    string(user.Role),
	})

	return nil
}

// checkNotLastAdmin verifies that the given user is not the last admin.
// Returns ErrCannotDeleteLastAdmin if they are the last admin.
func (s *Service) checkNotLastAdmin(ctx context.Context, userID string) error {
	// Get all users
	users, err := s.userRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to check admin count: %w", err)
	}

	// Count admins (excluding the user being modified/deleted)
	adminCount := 0
	for _, u := range users {
		if u.Role == domain.RoleAdmin && u.ID != userID {
			adminCount++
		}
	}

	// If no other admins exist, cannot proceed
	if adminCount == 0 {
		return domain.ErrCannotDeleteLastAdmin
	}

	return nil
}

// auditSuccess logs a successful operation to the audit log.
// This helper reduces code duplication across user management operations.
func (s *Service) auditSuccess(ctx context.Context, action, resource string, details map[string]any) {
	s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    action,
		Resource:  resource,
		Outcome:   "success",
		Details:   details,
	})
}
