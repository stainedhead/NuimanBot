package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	. "nuimanbot/internal/usecase/user"
)

// MockUserRepository implements domain.UserRepository for testing
type MockUserRepository struct {
	SaveUserFunc            func(ctx context.Context, user *domain.User) error
	GetUserByIDFunc         func(ctx context.Context, id string) (*domain.User, error)
	GetUserByPlatformIDFunc func(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error)
	ListAllFunc             func(ctx context.Context) ([]*domain.User, error)
	DeleteFunc              func(ctx context.Context, userID string) error
	users                   map[string]*domain.User // In-memory store for tests
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[string]*domain.User),
	}
}

func (m *MockUserRepository) SaveUser(ctx context.Context, user *domain.User) error {
	if m.SaveUserFunc != nil {
		return m.SaveUserFunc(ctx, user)
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	user, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

func (m *MockUserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	if m.GetUserByPlatformIDFunc != nil {
		return m.GetUserByPlatformIDFunc(ctx, platform, platformUID)
	}
	for _, user := range m.users {
		if user.PlatformIDs != nil {
			if uid, ok := user.PlatformIDs[platform]; ok && uid == platformUID {
				return user, nil
			}
		}
	}
	return nil, domain.ErrUserNotFound
}

// ListAll returns all users (for testing)
func (m *MockUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	if m.ListAllFunc != nil {
		return m.ListAllFunc(ctx)
	}
	users := make([]*domain.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

// Delete removes a user (for testing)
func (m *MockUserRepository) Delete(ctx context.Context, userID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID)
	}
	if _, ok := m.users[userID]; !ok {
		return domain.ErrUserNotFound
	}
	delete(m.users, userID)
	return nil
}

// MockSecurityService implements domain.SecurityService for testing
type MockSecurityService struct {
	AuditFunc func(ctx context.Context, event *domain.AuditEvent) error
}

func (m *MockSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (m *MockSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (m *MockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	return input, nil
}

func (m *MockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.AuditFunc != nil {
		return m.AuditFunc(ctx, event)
	}
	return nil
}

func TestCreateUser(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	user, err := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

	if err != nil {
		t.Errorf("CreateUser failed: %v", err)
	}
	if user == nil {
		t.Fatal("CreateUser returned nil user")
	}
	if user.ID == "" {
		t.Error("User ID should not be empty")
	}
	if user.Role != domain.RoleUser {
		t.Errorf("Expected role %s, got %s", domain.RoleUser, user.Role)
	}
	if user.PlatformIDs[domain.PlatformCLI] != "alice" {
		t.Error("Platform ID not set correctly")
	}
}

func TestCreateUser_DuplicatePlatformUID(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create first user
	_, err := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)
	if err != nil {
		t.Fatalf("First CreateUser failed: %v", err)
	}

	// Try to create duplicate
	_, err = svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("Expected ErrConflict for duplicate, got: %v", err)
	}
}

func TestGetUser(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create user first
	created, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

	// Get user
	user, err := svc.GetUser(ctx, created.ID)
	if err != nil {
		t.Errorf("GetUser failed: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("Expected user ID %s, got %s", created.ID, user.ID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()
	_, err := svc.GetUser(ctx, "nonexistent")

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("Expected ErrUserNotFound, got: %v", err)
	}
}

func TestListUsers(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create multiple users
	svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)
	svc.CreateUser(ctx, domain.PlatformCLI, "bob", domain.RoleAdmin)

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Errorf("ListUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestUpdateUserRole(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create user
	user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

	// Update role
	err := svc.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	if err != nil {
		t.Errorf("UpdateUserRole failed: %v", err)
	}

	// Verify update
	updated, _ := svc.GetUser(ctx, user.ID)
	if updated.Role != domain.RoleAdmin {
		t.Errorf("Expected role %s, got %s", domain.RoleAdmin, updated.Role)
	}
}

func TestUpdateUserRole_CannotDemoteLastAdmin(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create single admin
	admin, _ := svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin)

	// Try to demote last admin
	err := svc.UpdateUserRole(ctx, admin.ID, domain.RoleUser)
	if err == nil {
		t.Error("Should not allow demoting last admin")
	}
}

func TestUpdateAllowedSkills(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create user
	user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)

	// Update allowed skills
	skills := []string{"calculator", "datetime"}
	err := svc.UpdateAllowedSkills(ctx, user.ID, skills)
	if err != nil {
		t.Errorf("UpdateAllowedSkills failed: %v", err)
	}

	// Verify update
	updated, _ := svc.GetUser(ctx, user.ID)
	if len(updated.AllowedTools) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(updated.AllowedTools))
	}
	if updated.AllowedTools[0] != "calculator" {
		t.Errorf("Expected 'calculator', got '%s'", updated.AllowedTools[0])
	}
}

func TestDeleteUser(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create users
	user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)
	svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin)

	// Delete user
	err := svc.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Errorf("DeleteUser failed: %v", err)
	}

	// Verify deletion
	_, err = svc.GetUser(ctx, user.ID)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Error("User should be deleted")
	}
}

func TestDeleteUser_CannotDeleteLastAdmin(t *testing.T) {
	mockRepo := NewMockUserRepository()
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Create single admin
	admin, _ := svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin)

	// Try to delete last admin
	err := svc.DeleteUser(ctx, admin.ID)
	if err == nil {
		t.Error("Should not allow deleting last admin")
	}
}

func TestAuditLogging(t *testing.T) {
	mockRepo := NewMockUserRepository()
	auditEvents := make(chan domain.AuditEvent, 10)
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(mockRepo, mockSecurity)

	ctx := context.Background()

	// Operations that should be audited
	user, _ := svc.CreateUser(ctx, domain.PlatformCLI, "alice", domain.RoleUser)
	_, _ = svc.CreateUser(ctx, domain.PlatformCLI, "admin", domain.RoleAdmin) // Ensure there's an admin
	svc.UpdateUserRole(ctx, user.ID, domain.RoleAdmin)
	svc.DeleteUser(ctx, user.ID)

	// Verify audit events were logged
	timeout := time.After(100 * time.Millisecond)
	eventCount := 0
	for {
		select {
		case event := <-auditEvents:
			eventCount++
			if event.Action == "" {
				t.Error("Audit event should have an action")
			}
		case <-timeout:
			if eventCount < 4 {
				t.Errorf("Expected at least 4 audit events (2 creates + 1 update + 1 delete), got %d", eventCount)
			}
			return
		}
	}
}
