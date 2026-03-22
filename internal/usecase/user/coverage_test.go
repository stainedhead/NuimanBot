package user_test

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
	. "nuimanbot/internal/usecase/user" //nolint:revive // dot import used intentionally to simplify test access
)

// TestGetUserByPlatformUID covers the 0.0% function
func TestGetUserByPlatformUID(t *testing.T) {
	t.Parallel()

	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)
	ctx := context.Background()

	// Create a user first
	created, err := svc.CreateUser(ctx, domain.PlatformCLI, "test-user", domain.RoleUser)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	t.Run("existing_user_found", func(t *testing.T) {
		user, err := svc.GetUserByPlatformUID(ctx, domain.PlatformCLI, "test-user")
		if err != nil {
			t.Fatalf("GetUserByPlatformUID failed: %v", err)
		}
		if user.ID != created.ID {
			t.Errorf("Expected user ID %s, got %s", created.ID, user.ID)
		}
	})

	t.Run("non_existent_user_returns_error", func(t *testing.T) {
		_, err := svc.GetUserByPlatformUID(ctx, domain.PlatformCLI, "non-existent")
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("Expected ErrUserNotFound, got: %v", err)
		}
	})
}

// TestUpdateAllowedSkills_ErrorPaths covers error paths in UpdateAllowedSkills
func TestUpdateAllowedSkills_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("user_not_found", func(t *testing.T) {
		mockRepo := NewMockUserRepository()
		mockSecurity := &MockSecurityService{}
		svc := NewService(mockRepo, mockSecurity)
		ctx := context.Background()

		err := svc.UpdateAllowedSkills(ctx, "nonexistent-user", []string{"skill1"})
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("Expected ErrUserNotFound, got: %v", err)
		}
	})

	t.Run("save_error_propagated", func(t *testing.T) {
		mockRepo := NewMockUserRepository()
		mockSecurity := &MockSecurityService{}
		svc := NewService(mockRepo, mockSecurity)
		ctx := context.Background()

		user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

		saveErr := errors.New("save error")
		mockRepo.SaveUserFunc = func(ctx context.Context, u *domain.User) error {
			return saveErr
		}

		err := svc.UpdateAllowedSkills(ctx, user.ID, []string{"skill1"})
		if err == nil {
			t.Error("Expected error when save fails")
		}
	})
}

// TestDeleteUser_ErrorPaths covers error paths in DeleteUser
func TestDeleteUser_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("user_not_found", func(t *testing.T) {
		mockRepo := NewMockUserRepository()
		mockSecurity := &MockSecurityService{}
		svc := NewService(mockRepo, mockSecurity)
		ctx := context.Background()

		err := svc.DeleteUser(ctx, "nonexistent-user")
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("Expected ErrUserNotFound, got: %v", err)
		}
	})

	t.Run("delete_error_propagated", func(t *testing.T) {
		mockRepo := NewMockUserRepository()
		mockSecurity := &MockSecurityService{}
		svc := NewService(mockRepo, mockSecurity)
		ctx := context.Background()

		// Create two users so we can delete a non-admin
		svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin) //nolint:errcheck
		user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

		deleteErr := errors.New("delete error")
		mockRepo.DeleteFunc = func(ctx context.Context, userID string) error {
			return deleteErr
		}

		err := svc.DeleteUser(ctx, user.ID)
		if err == nil {
			t.Error("Expected error when delete fails")
		}
	})
}

// TestUpdateUserRole_SaveError covers the save error path
func TestUpdateUserRole_SaveError(t *testing.T) {
	t.Parallel()

	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)
	ctx := context.Background()

	user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

	saveErr := errors.New("save error")
	mockRepo.SaveUserFunc = func(ctx context.Context, u *domain.User) error {
		// Allow initial save (CreateUser), but fail subsequent ones
		if u.ID == user.ID && u.Role == domain.RoleAdmin {
			return saveErr
		}
		mockRepo.users[u.ID] = u
		return nil
	}

	err := svc.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	if err == nil {
		t.Error("Expected error when save fails during UpdateUserRole")
	}
}

// TestCheckNotLastAdmin_ListError covers the list error path in checkNotLastAdmin
func TestCheckNotLastAdmin_ListError(t *testing.T) {
	t.Parallel()

	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)
	ctx := context.Background()

	// Create an admin
	admin, _ := svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin)

	listErr := errors.New("list error")
	mockRepo.ListAllFunc = func(ctx context.Context) ([]*domain.User, error) {
		return nil, listErr
	}

	// Attempting to demote admin should trigger checkNotLastAdmin which calls ListAll
	err := svc.UpdateUserRole(ctx, admin.ID, domain.RoleUser)
	if err == nil {
		t.Error("Expected error when ListAll fails in checkNotLastAdmin")
	}
}
