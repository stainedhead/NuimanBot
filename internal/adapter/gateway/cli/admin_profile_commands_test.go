package cli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
)

// MockProfileService implements the profile service interface for testing
type MockProfileService struct {
	CreateProfileFunc     func(ctx context.Context, profile *domain.UserProfile) error
	GetProfileFunc        func(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetProfileByEmailFunc func(ctx context.Context, email string) (*domain.UserProfile, error)
	UpdateProfileFunc     func(ctx context.Context, userID string, updates map[string]interface{}) error
	DeleteProfileFunc     func(ctx context.Context, userID string) error
	ListProfilesFunc      func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
	profiles              map[string]*domain.UserProfile
}

func NewMockProfileService() *MockProfileService {
	return &MockProfileService{
		profiles: make(map[string]*domain.UserProfile),
	}
}

func (m *MockProfileService) CreateProfile(ctx context.Context, profile *domain.UserProfile) error {
	if m.CreateProfileFunc != nil {
		return m.CreateProfileFunc(ctx, profile)
	}
	m.profiles[profile.UserID] = profile
	return nil
}

func (m *MockProfileService) GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	if m.GetProfileFunc != nil {
		return m.GetProfileFunc(ctx, userID)
	}
	profile, ok := m.profiles[userID]
	if !ok {
		return nil, errors.New("profile not found")
	}
	return profile, nil
}

func (m *MockProfileService) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	if m.GetProfileByEmailFunc != nil {
		return m.GetProfileByEmailFunc(ctx, email)
	}
	for _, p := range m.profiles {
		if p.PrimaryEmail == email {
			return p, nil
		}
	}
	return nil, errors.New("profile not found")
}

func (m *MockProfileService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	if m.UpdateProfileFunc != nil {
		return m.UpdateProfileFunc(ctx, userID, updates)
	}
	profile, ok := m.profiles[userID]
	if !ok {
		return errors.New("profile not found")
	}
	// Apply simple updates for testing
	if moniker, ok := updates["moniker"]; ok {
		profile.Moniker = moniker.(string)
	}
	if firstName, ok := updates["firstName"]; ok {
		profile.FirstName = firstName.(string)
	}
	return nil
}

func (m *MockProfileService) DeleteProfile(ctx context.Context, userID string) error {
	if m.DeleteProfileFunc != nil {
		return m.DeleteProfileFunc(ctx, userID)
	}
	if _, ok := m.profiles[userID]; !ok {
		return errors.New("profile not found")
	}
	delete(m.profiles, userID)
	return nil
}

func (m *MockProfileService) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	if m.ListProfilesFunc != nil {
		return m.ListProfilesFunc(ctx, offset, limit)
	}
	profiles := make([]*domain.UserProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// Test profile command - not admin (should fail)
func TestHandleProfileCommand_NotAdmin(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	regularUser := &domain.User{
		ID:       "user-123",
		Username: "alice",
		Role:     domain.RoleUser,
	}

	ctx := context.Background()
	_, err := handler.HandleProfileCommand(ctx, regularUser, "/admin profile list")

	if err != domain.ErrInsufficientPermissions {
		t.Errorf("Expected ErrInsufficientPermissions, got: %v", err)
	}
}

// Test profile create command
func TestHandleProfileCommand_Create(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	input := "/admin profile create user-123 alice@example.com --moniker alice --first-name Alice --last-name Smith"
	result, err := handler.HandleProfileCommand(ctx, adminUser, input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "Profile created successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify profile was created
	profile, err := mockService.GetProfile(ctx, "user-123")
	if err != nil {
		t.Errorf("Profile should be created")
	}
	if profile.Moniker != "alice" {
		t.Errorf("Expected moniker 'alice', got '%s'", profile.Moniker)
	}
}

// Test profile list command
func TestHandleProfileCommand_List(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	// Create test profiles
	p1 := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p1.Moniker = "alice"
	p2 := domain.NewUserProfile("user-456", "bob@example.com", domain.UserTypeIndividual)
	p2.Moniker = "bob"
	mockService.CreateProfile(context.Background(), p1)
	mockService.CreateProfile(context.Background(), p2)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile list")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "alice") || !strings.Contains(result, "bob") {
		t.Errorf("Expected both profiles in list, got: %s", result)
	}
}

// Test profile view command
func TestHandleProfileCommand_View(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	// Create test profile
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p.Moniker = "alice"
	p.FirstName = "Alice"
	p.LastName = "Smith"
	mockService.CreateProfile(context.Background(), p)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile view user-123")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "alice") || !strings.Contains(result, "Alice") {
		t.Errorf("Expected profile details, got: %s", result)
	}
}

// Test profile update command
func TestHandleProfileCommand_Update(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	// Create test profile
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	p.Moniker = "alice"
	mockService.CreateProfile(context.Background(), p)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile update user-123 --moniker alice-updated")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "updated successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify update
	updated, _ := mockService.GetProfile(ctx, "user-123")
	if updated.Moniker != "alice-updated" {
		t.Errorf("Expected moniker 'alice-updated', got '%s'", updated.Moniker)
	}
}

// Test profile delete command
func TestHandleProfileCommand_Delete(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	// Create test profile
	p := domain.NewUserProfile("user-123", "alice@example.com", domain.UserTypeIndividual)
	mockService.CreateProfile(context.Background(), p)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile delete user-123")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "deleted successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify deletion
	_, err = mockService.GetProfile(ctx, "user-123")
	if err == nil {
		t.Error("Profile should be deleted")
	}
}

// Test profile command - invalid subcommand
func TestHandleProfileCommand_InvalidSubcommand(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile invalid")

	if err != nil {
		t.Fatalf("Expected no error for invalid command, got: %v", err)
	}

	if !strings.Contains(result, "Unknown") || !strings.Contains(result, "invalid") {
		t.Errorf("Expected unknown command message, got: %s", result)
	}
}

// Test profile command - help
func TestHandleProfileCommand_Help(t *testing.T) {
	mockService := NewMockProfileService()
	handler := cli.NewAdminProfileCommandHandler(mockService)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile help")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(result, "create") || !strings.Contains(result, "list") {
		t.Errorf("Expected help text, got: %s", result)
	}
}

// Test IsProfileCommand
func TestIsProfileCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"/admin profile list", true},
		{"/admin profile create", true},
		{"/admin user list", false},
		{"/help", false},
		{"regular message", false},
	}

	for _, tt := range tests {
		result := cli.IsProfileCommand(tt.input)
		if result != tt.expected {
			t.Errorf("IsProfileCommand(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// Test error handling - service error
func TestHandleProfileCommand_ServiceError(t *testing.T) {
	mockService := NewMockProfileService()
	mockService.ListProfilesFunc = func(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
		return nil, errors.New("database error")
	}
	handler := cli.NewAdminProfileCommandHandler(mockService)

	adminUser := &domain.User{
		ID:       "admin-123",
		Username: "admin",
		Role:     domain.RoleAdmin,
	}

	ctx := context.Background()
	_, err := handler.HandleProfileCommand(ctx, adminUser, "/admin profile list")

	if err == nil {
		t.Error("Expected error from service failure")
	}
}
