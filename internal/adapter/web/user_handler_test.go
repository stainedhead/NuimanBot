package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

// MockProfileService is a mock implementation of ProfileService for testing
type MockProfileService struct {
	profiles map[string]*domain.UserProfile
}

func NewMockProfileService() *MockProfileService {
	return &MockProfileService{
		profiles: make(map[string]*domain.UserProfile),
	}
}

func (m *MockProfileService) CreateProfile(ctx context.Context, profile *domain.UserProfile) error {
	m.profiles[profile.UserID] = profile
	return nil
}

func (m *MockProfileService) GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	profile, exists := m.profiles[userID]
	if !exists {
		return nil, domain.ErrUserNotFound
	}
	return profile, nil
}

func (m *MockProfileService) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	for _, profile := range m.profiles {
		if profile.PrimaryEmail == email {
			return profile, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockProfileService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	profile, exists := m.profiles[userID]
	if !exists {
		return domain.ErrUserNotFound
	}
	// Apply updates (simplified for testing)
	if firstName, ok := updates["firstName"].(string); ok {
		profile.FirstName = firstName
	}
	if lastName, ok := updates["lastName"].(string); ok {
		profile.LastName = lastName
	}
	return nil
}

func (m *MockProfileService) DeleteProfile(ctx context.Context, userID string) error {
	if _, exists := m.profiles[userID]; !exists {
		return domain.ErrUserNotFound
	}
	delete(m.profiles, userID)
	return nil
}

func (m *MockProfileService) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	profiles := make([]*domain.UserProfile, 0, len(m.profiles))
	for _, profile := range m.profiles {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (m *MockProfileService) GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error) {
	for _, profile := range m.profiles {
		switch platform {
		case domain.PlatformSlack:
			if profile.PlatformIDs.Slack == platformID {
				return profile, nil
			}
		case domain.PlatformTelegram:
			if profile.PlatformIDs.Telegram == platformID {
				return profile, nil
			}
		}
	}
	return nil, domain.ErrUserNotFound
}

// TestUsersPageRequiresAuth tests that users page requires authentication
func TestUsersPageRequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()

	server.handleUsers(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect to login, got status %d", w.Code)
	}
}

// TestUsersPageWithAuth tests that authenticated users can access users page
func TestUsersPageWithAuth(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	// Create test user and session
	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	// Add some test profiles
	if err := profileService.CreateProfile(context.Background(), &domain.UserProfile{
		UserID:       "user1",
		FirstName:    "John",
		LastName:     "Doe",
		PrimaryEmail: "john@example.com",
		Role:         domain.RoleUser,
	}); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Users") {
		t.Error("expected users page to contain 'Users'")
	}
}

// TestUserCreateForm tests displaying the user creation form
func TestUserCreateForm(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	server.SetProfileService(NewMockProfileService())

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/create", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleUserCreate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	body := w.Body.String()
	expectedStrings := []string{"Create User", "User ID", "Email"}
	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("expected form to contain '%s'", expected)
		}
	}
}

// TestUserCreateSubmit tests creating a new user
func TestUserCreateSubmit(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	// Create form data
	form := url.Values{}
	form.Add("userID", "newuser")
	form.Add("firstName", "Jane")
	form.Add("lastName", "Smith")
	form.Add("primaryEmail", "jane@example.com")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleUserCreate(w, req)

	// Should redirect after successful creation
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	// Verify user was created
	profile, err := profileService.GetProfile(context.Background(), "newuser")
	if err != nil {
		t.Errorf("expected user to be created, got error: %v", err)
	}
	if profile.FirstName != "Jane" {
		t.Errorf("expected first name 'Jane', got '%s'", profile.FirstName)
	}
}

// TestUserDelete tests deleting a user
func TestUserDelete(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	// Create a test user
	if err := profileService.CreateProfile(context.Background(), &domain.UserProfile{
		UserID:       "deleteuser",
		FirstName:    "Delete",
		LastName:     "Me",
		PrimaryEmail: "delete@example.com",
	}); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/users/deleteuser/delete", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleUserDelete(w, req)

	// Should redirect after deletion
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	// Verify user was deleted
	_, err := profileService.GetProfile(context.Background(), "deleteuser")
	if err != domain.ErrUserNotFound {
		t.Error("expected user to be deleted")
	}
}
