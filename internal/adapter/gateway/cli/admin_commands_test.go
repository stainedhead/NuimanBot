package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/user"
)

// Use the same mock from user service tests
type MockUserRepository struct {
	users map[string]*domain.User
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[string]*domain.User),
	}
}

func (m *MockUserRepository) SaveUser(ctx context.Context, u *domain.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (m *MockUserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	for _, u := range m.users {
		if u.PlatformIDs != nil {
			if uid, ok := u.PlatformIDs[platform]; ok && uid == platformUID {
				return u, nil
			}
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	users := make([]*domain.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *MockUserRepository) Delete(ctx context.Context, userID string) error {
	if _, ok := m.users[userID]; !ok {
		return domain.ErrUserNotFound
	}
	delete(m.users, userID)
	return nil
}

type MockSecurityService struct{}

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
	return nil
}

func setupAdminHandler() (*cli.AdminCommandHandler, *MockUserRepository) {
	repo := NewMockUserRepository()
	security := &MockSecurityService{}
	userService := user.NewService(repo, security)
	handler := cli.NewAdminCommandHandler(userService)
	return handler, repo
}

func TestIsAdminCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"/admin user list", true},
		{"/admin help", true},
		{"admin user list", false},
		{"hello", false},
		{"/help", false},
	}

	for _, tt := range tests {
		result := cli.IsAdminCommand(tt.input)
		if result != tt.expected {
			t.Errorf("IsAdminCommand(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestHandleAdminCommand_NonAdmin(t *testing.T) {
	handler, _ := setupAdminHandler()
	ctx := context.Background()

	regularUser := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	_, err := handler.HandleAdminCommand(ctx, regularUser, "/admin user list")
	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("Expected ErrInsufficientPermissions, got: %v", err)
	}
}

func TestHandleAdminCommand_Help(t *testing.T) {
	handler, _ := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	result, err := handler.HandleAdminCommand(ctx, admin, "/admin help")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "Admin Commands") {
		t.Error("Help text should contain 'Admin Commands'")
	}
}

func TestHandleAdminCommand_CreateUser(t *testing.T) {
	handler, repo := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "created successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify user was created
	user, err := repo.GetUserByPlatformID(ctx, domain.PlatformCLI, "alice")
	if err != nil {
		t.Errorf("User should have been created: %v", err)
	}
	if user.Role != domain.RoleUser {
		t.Errorf("Expected role user, got %s", user.Role)
	}
}

func TestHandleAdminCommand_ListUsers(t *testing.T) {
	handler, _ := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	// Create some users first
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli bob admin")

	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user list")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "alice") || !strings.Contains(result, "bob") {
		t.Errorf("List should contain both users, got: %s", result)
	}
	if !strings.Contains(result, "Found 2 user(s)") {
		t.Errorf("Should show 2 users, got: %s", result)
	}
}

func TestHandleAdminCommand_GetUser(t *testing.T) {
	handler, repo := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	// Create a user first
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")

	// Get the user ID
	user, _ := repo.GetUserByPlatformID(ctx, domain.PlatformCLI, "alice")

	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user get "+user.ID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "User Details") {
		t.Errorf("Expected user details, got: %s", result)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("Should contain username alice, got: %s", result)
	}
}

func TestHandleAdminCommand_UpdateUserRole(t *testing.T) {
	handler, repo := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	// Create users (need two admins so we can demote one)
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli bob admin")

	// Get alice's ID
	alice, _ := repo.GetUserByPlatformID(ctx, domain.PlatformCLI, "alice")

	// Update role
	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user update "+alice.ID+" --role admin")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "role updated") {
		t.Errorf("Expected role updated message, got: %s", result)
	}

	// Verify role was updated
	updated, _ := repo.GetUserByID(ctx, alice.ID)
	if updated.Role != domain.RoleAdmin {
		t.Errorf("Expected role admin, got %s", updated.Role)
	}
}

func TestHandleAdminCommand_UpdateUserSkills(t *testing.T) {
	handler, repo := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	// Create a user
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")
	alice, _ := repo.GetUserByPlatformID(ctx, domain.PlatformCLI, "alice")

	// Update skills
	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user update "+alice.ID+" --skills calculator,datetime")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "skills updated") {
		t.Errorf("Expected skills updated message, got: %s", result)
	}

	// Verify skills were updated
	updated, _ := repo.GetUserByID(ctx, alice.ID)
	if len(updated.AllowedSkills) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(updated.AllowedSkills))
	}
}

func TestHandleAdminCommand_DeleteUser(t *testing.T) {
	handler, repo := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	// Create users (need an admin so deletion doesn't fail)
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice user")
	handler.HandleAdminCommand(ctx, admin, "/admin user create cli bob admin")

	alice, _ := repo.GetUserByPlatformID(ctx, domain.PlatformCLI, "alice")

	// Delete user
	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user delete "+alice.ID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(result, "deleted successfully") {
		t.Errorf("Expected deletion message, got: %s", result)
	}

	// Verify user was deleted
	_, err = repo.GetUserByID(ctx, alice.ID)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Error("User should have been deleted")
	}
}

func TestHandleAdminCommand_InvalidRole(t *testing.T) {
	handler, _ := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	result, err := handler.HandleAdminCommand(ctx, admin, "/admin user create cli alice superuser")
	if err != nil {
		t.Errorf("Should return error message, not error: %v", err)
	}
	if !strings.Contains(result, "Invalid role") {
		t.Errorf("Expected invalid role message, got: %s", result)
	}
}

func TestHandleAdminCommand_MissingArguments(t *testing.T) {
	handler, _ := setupAdminHandler()
	ctx := context.Background()

	admin := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	tests := []struct {
		command string
		want    string
	}{
		{"/admin user create", "Usage:"},
		{"/admin user create cli", "Usage:"},
		{"/admin user get", "Usage:"},
		{"/admin user update", "Usage:"},
		{"/admin user delete", "Usage:"},
	}

	for _, tt := range tests {
		result, err := handler.HandleAdminCommand(ctx, admin, tt.command)
		if err != nil {
			t.Errorf("Command %q returned error: %v", tt.command, err)
		}
		if !strings.Contains(result, tt.want) {
			t.Errorf("Command %q should contain %q, got: %s", tt.command, tt.want, result)
		}
	}
}
