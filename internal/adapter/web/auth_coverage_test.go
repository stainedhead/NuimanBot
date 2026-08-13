package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestSetAuthService verifies SetAuthService assigns the auth service on the server.
func TestSetAuthService(t *testing.T) {
	server := NewServer(":0")
	auth := NewAuthService()
	server.SetAuthService(auth)
	if server.auth != auth {
		t.Error("SetAuthService did not assign the auth service")
	}
}

// TestUpdatePassword_Success verifies UpdatePassword updates the hash.
func TestUpdatePassword_Success(t *testing.T) {
	auth := NewAuthService()
	if err := auth.AddUser("alice", "oldpass", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if err := auth.UpdatePassword("alice", "newpass"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	// Old password should no longer work.
	if auth.ValidateCredentials("alice", "oldpass") {
		t.Error("old password should be invalid after UpdatePassword")
	}
	// New password should work.
	if !auth.ValidateCredentials("alice", "newpass") {
		t.Error("new password should be valid after UpdatePassword")
	}
}

// TestUpdatePassword_UnknownUser verifies UpdatePassword returns an error for unknown users.
func TestUpdatePassword_UnknownUser(t *testing.T) {
	auth := NewAuthService()
	err := auth.UpdatePassword("nobody", "pass")
	if err == nil {
		t.Error("expected error when updating unknown user, got nil")
	}
}

// TestClearForcePasswordChange verifies the flag is cleared on the session.
func TestClearForcePasswordChange(t *testing.T) {
	auth := NewAuthService()
	sessionID := auth.createSessionWithFlags("user", "admin", true)

	// Flag should be set initially.
	s := auth.GetSession(sessionID)
	if s == nil || !s.ForcePasswordChange {
		t.Fatal("expected ForcePasswordChange=true initially")
	}

	auth.ClearForcePasswordChange(sessionID)

	s2 := auth.GetSession(sessionID)
	if s2 == nil || s2.ForcePasswordChange {
		t.Error("expected ForcePasswordChange=false after ClearForcePasswordChange")
	}
}

// TestClearForcePasswordChange_UnknownSession verifies no panic for missing session.
func TestClearForcePasswordChange_UnknownSession(t *testing.T) {
	auth := NewAuthService()
	// Should not panic.
	auth.ClearForcePasswordChange("nonexistent")
}

// TestSetSecureCookies verifies setSecureCookies / isSecureCookies.
func TestSetSecureCookies(t *testing.T) {
	auth := NewAuthService()
	if auth.isSecureCookies() {
		t.Error("expected false by default")
	}
	auth.setSecureCookies(true)
	if !auth.isSecureCookies() {
		t.Error("expected true after setSecureCookies(true)")
	}
	auth.setSecureCookies(false)
	if auth.isSecureCookies() {
		t.Error("expected false after setSecureCookies(false)")
	}
}

// TestHandleChangePassword_GET verifies the GET path renders the form.
func TestHandleChangePassword_GET(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/change-password", nil)
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Change") {
		t.Error("expected form body to contain 'Change'")
	}
}

// TestHandleChangePassword_POST_EmptyPassword verifies 400 when new_password is empty.
func TestHandleChangePassword_POST_EmptyPassword(t *testing.T) {
	server := NewServer(":0")

	form := url.Values{}
	form.Set("new_password", "")
	req := httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty password, got %d", w.Code)
	}
}

// TestHandleChangePassword_POST_NoSession verifies redirect when no session cookie.
func TestHandleChangePassword_POST_NoSession(t *testing.T) {
	server := NewServer(":0")

	form := url.Values{}
	form.Set("new_password", "newstrongpassword")
	req := httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleChangePassword_POST_InvalidSession verifies redirect for invalid session.
func TestHandleChangePassword_POST_InvalidSession(t *testing.T) {
	server := NewServer(":0")

	form := url.Values{}
	form.Set("new_password", "newstrongpassword")
	req := httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-session"})
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleChangePassword_POST_Success verifies successful password change redirects to dashboard.
func TestHandleChangePassword_POST_Success(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "admin", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.createSessionWithFlags("admin", "admin", true)

	form := url.Values{}
	form.Set("new_password", "mynewsecurepassword")
	req := httptest.NewRequest(http.MethodPost, "/admin/change-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302) after successful change, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/admin/dashboard" {
		t.Errorf("expected redirect to /admin/dashboard, got %s", location)
	}
}

// TestHandleChangePassword_MethodNotAllowed verifies 405 for unsupported method.
func TestHandleChangePassword_MethodNotAllowed(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodPut, "/admin/change-password", nil)
	w := httptest.NewRecorder()

	server.handleChangePassword(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleLogin_MethodNotAllowed verifies 405 for unsupported methods.
func TestHandleLogin_MethodNotAllowed(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodPut, "/admin/login", nil)
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleLogin_CSRFValidationFails verifies 401 when CSRF token is invalid.
func TestHandleLogin_CSRFValidationFails(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "password123", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "password123")
	form.Set("csrf_token", "invalid-csrf-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:9999"
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid CSRF token, got %d", w.Code)
	}
}

// TestHandleLogout_MethodNotAllowed verifies 405 for DELETE method.
func TestHandleLogout_MethodNotAllowed(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodDelete, "/admin/logout", nil)
	w := httptest.NewRecorder()

	server.handleLogout(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestHandleLogout_WithoutCookie verifies logout works when no cookie is present.
func TestHandleLogout_WithoutCookie(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/logout", nil)
	w := httptest.NewRecorder()

	server.handleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestGetCurrentUser_NoAuth verifies nil is returned when auth is nil.
func TestGetCurrentUser_NoAuth(t *testing.T) {
	server := NewServer(":0")
	server.auth = nil

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "some-session"})
	user := server.getCurrentUser(req)
	if user != nil {
		t.Error("expected nil user when auth is nil")
	}
}

// TestGetCurrentUser_NoCookie verifies nil when no session cookie is present.
func TestGetCurrentUser_NoCookie(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := server.getCurrentUser(req)
	if user != nil {
		t.Error("expected nil user when no cookie is present")
	}
}

// TestGetCurrentUser_ValidSession verifies user is returned with valid session.
func TestGetCurrentUser_ValidSession(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("alice", "pass", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "admin")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})

	user := server.getCurrentUser(req)
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", user.Username)
	}
}

// TestIsDefaultCredentials verifies the detection logic.
func TestIsDefaultCredentials(t *testing.T) {
	auth := NewAuthService()

	tests := []struct {
		username string
		password string
		want     bool
	}{
		{"admin", "admin", true},
		{"admin", "notadmin", false},
		{"other", "admin", false},
		{"other", "other", false},
	}

	for _, tt := range tests {
		got := auth.isDefaultCredentials(tt.username, tt.password)
		if got != tt.want {
			t.Errorf("isDefaultCredentials(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
		}
	}
}

// TestGenerateRandomString verifies generateRandomString returns non-empty strings of expected encoded length.
func TestGenerateRandomString(t *testing.T) {
	s := generateRandomString(32)
	if s == "" {
		t.Error("generateRandomString should not return empty string")
	}
	// Second call should differ.
	s2 := generateRandomString(32)
	if s == s2 {
		t.Error("generateRandomString should produce unique values")
	}
}

// TestCreateSessionWithFlags verifies ForcePasswordChange is set correctly.
func TestCreateSessionWithFlags(t *testing.T) {
	auth := NewAuthService()

	tests := []struct {
		forceChange bool
	}{
		{true},
		{false},
	}

	for _, tt := range tests {
		sessionID := auth.createSessionWithFlags("user", "admin", tt.forceChange)
		session := auth.GetSession(sessionID)
		if session == nil {
			t.Fatal("expected session, got nil")
		}
		if session.ForcePasswordChange != tt.forceChange {
			t.Errorf("expected ForcePasswordChange=%v, got %v", tt.forceChange, session.ForcePasswordChange)
		}
	}
}

// TestCleanupExpiredSessions relocated verbatim to
// internal/usecase/auth/session_test.go (architecture.md AD-1's documented
// two-function exception: it reads the unexported sessions map directly,
// which now lives on auth.Service).
