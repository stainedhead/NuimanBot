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

// errorProfileService is a ProfileService that always returns errors.
type errorProfileService struct{}

func (e *errorProfileService) CreateProfile(_ context.Context, _ *domain.UserProfile) error {
	return domain.ErrUserNotFound
}
func (e *errorProfileService) GetProfile(_ context.Context, _ string) (*domain.UserProfile, error) {
	return nil, domain.ErrUserNotFound
}
func (e *errorProfileService) UpdateProfile(_ context.Context, _ string, _ map[string]interface{}) error {
	return domain.ErrUserNotFound
}
func (e *errorProfileService) DeleteProfile(_ context.Context, _ string) error {
	return domain.ErrUserNotFound
}
func (e *errorProfileService) ListProfiles(_ context.Context, _, _ int) ([]*domain.UserProfile, error) {
	return nil, domain.ErrUserNotFound
}

// TestHandleUserEdit_RequiresAuth verifies redirect when unauthenticated.
func TestHandleUserEdit_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/user1/edit", nil)
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleUserEdit_InvalidPath verifies 400 for too-short paths.
func TestHandleUserEdit_InvalidPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/x/y", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestHandleUserEdit_GET_NotFound verifies 404 when profile not found.
func TestHandleUserEdit_GET_NotFound(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(&errorProfileService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/users/unknown/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when user not found, got %d", w.Code)
	}
}

// TestHandleUserEdit_GET_Success verifies the edit form is shown for a known user.
func TestHandleUserEdit_GET_Success(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	if err := profileService.CreateProfile(context.Background(), &domain.UserProfile{
		UserID:       "edituser",
		FirstName:    "Edit",
		LastName:     "Me",
		PrimaryEmail: "edit@example.com",
		Role:         domain.RoleUser,
	}); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/users/edituser/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestHandleUserEdit_POST_Success verifies a successful user update.
func TestHandleUserEdit_POST_Success(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	if err := profileService.CreateProfile(context.Background(), &domain.UserProfile{
		UserID:       "updateuser",
		FirstName:    "Old",
		LastName:     "Name",
		PrimaryEmail: "old@example.com",
		Role:         domain.RoleUser,
	}); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	form := url.Values{}
	form.Set("firstName", "New")
	form.Set("lastName", "Name")
	form.Set("primaryEmail", "new@example.com")
	form.Set("role", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/updateuser/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleUserEdit_POST_ServiceError verifies 500 when UpdateProfile fails.
func TestHandleUserEdit_POST_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(&errorProfileService{})

	form := url.Values{}
	form.Set("firstName", "Bad")
	form.Set("lastName", "Update")
	form.Set("primaryEmail", "bad@example.com")
	form.Set("role", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/failuser/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleUserEdit_MethodNotAllowed verifies 405 for DELETE.
func TestHandleUserEdit_MethodNotAllowed(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/user1/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserEdit(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleUserCreate_NonAdminForbidden verifies 403 for non-admin.
func TestHandleUserCreate_NonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regular", "pass", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("regular", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/users/create", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserCreate(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestHandleUserCreate_POST_ServiceError verifies 500 when CreateProfile fails.
func TestHandleUserCreate_POST_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(&errorProfileService{})

	form := url.Values{}
	form.Set("userID", "failuser")
	form.Set("firstName", "Fail")
	form.Set("lastName", "User")
	form.Set("primaryEmail", "fail@example.com")
	form.Set("role", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserCreate(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleUserCreate_MethodNotAllowed verifies 405 for PATCH.
func TestHandleUserCreate_MethodNotAllowed(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodPatch, "/admin/users/create", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserCreate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleUserDelete_RequiresAuth verifies redirect when unauthenticated.
func TestHandleUserDelete_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/user1/delete", nil)
	w := httptest.NewRecorder()

	server.handleUserDelete(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleUserDelete_NonAdminForbidden verifies 403 for non-admin.
func TestHandleUserDelete_NonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regular", "pass", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("regular", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/user1/delete", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserDelete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestHandleUserDelete_InvalidPath verifies 400 for short paths.
func TestHandleUserDelete_InvalidPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodPost, "/x/y", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserDelete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestHandleUserDelete_ServiceError verifies 500 when DeleteProfile fails.
func TestHandleUserDelete_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(&errorProfileService{})

	req := httptest.NewRequest(http.MethodPost, "/admin/users/failuser/delete", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUserDelete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestHandleUsers_ServiceError verifies the page still renders when ListProfiles fails.
func TestHandleUsers_ServiceError(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(&errorProfileService{})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleUsers(w, req)

	// Still renders the page with empty user list.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 despite service error, got %d", w.Code)
	}
}
