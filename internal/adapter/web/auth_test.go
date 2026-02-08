package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestLoginPageRendered tests that the login page is rendered
func TestLoginPageRendered(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}
}

// TestLoginWithValidCredentials tests successful login
func TestLoginWithValidCredentials(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth // Use server's auth service

	// Add a test user
	auth.AddUser("admin", "password123", "admin")

	// Generate CSRF token
	csrfToken := auth.GenerateCSRFToken()

	// Create login form data
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "password123")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	// Should redirect after successful login
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	// Should set session cookie
	cookies := w.Result().Cookies()
	foundSession := false
	for _, cookie := range cookies {
		if cookie.Name == "session_id" {
			foundSession = true
			if cookie.Value == "" {
				t.Error("session cookie value should not be empty")
			}
		}
	}
	if !foundSession {
		t.Error("session cookie not set")
	}
}

// TestLoginWithInvalidCredentials tests failed login
func TestLoginWithInvalidCredentials(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth // Use server's auth service

	// Add a test user
	auth.AddUser("admin", "password123", "admin")

	// Generate CSRF token
	csrfToken := auth.GenerateCSRFToken()

	// Try with wrong password
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "wrongpassword")
	form.Add("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.handleLogin(w, req)

	// Should return 401 or show login form again
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusOK {
		t.Errorf("expected unauthorized or OK status, got %d", w.Code)
	}
}

// TestLogout tests logout functionality
func TestLogout(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth // Use server's auth service

	// Create a session
	sessionID := auth.CreateSession("admin", "admin")

	// Create logout request with session cookie
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleLogout(w, req)

	// Should redirect to login
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	// Session should be destroyed
	if auth.ValidateSession(sessionID) {
		t.Error("session should be destroyed after logout")
	}
}

// TestSessionValidation tests session validation middleware
func TestSessionValidation(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth // Use server's auth service

	// Create a valid session
	sessionID := auth.CreateSession("admin", "admin")

	// Create protected handler
	protected := server.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("protected content"))
	}))

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
	}{
		{
			name:           "valid session",
			sessionID:      sessionID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no session",
			sessionID:      "",
			expectedStatus: http.StatusFound, // Redirect to login
		},
		{
			name:           "invalid session",
			sessionID:      "invalid",
			expectedStatus: http.StatusFound, // Redirect to login
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
			if tt.sessionID != "" {
				req.AddCookie(&http.Cookie{
					Name:  "session_id",
					Value: tt.sessionID,
				})
			}
			w := httptest.NewRecorder()

			protected.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestSessionExpiry tests that sessions expire after timeout
func TestSessionExpiry(t *testing.T) {
	auth := NewAuthService()
	auth.sessionTimeout = 100 * time.Millisecond // Short timeout for testing

	// Create session
	sessionID := auth.CreateSession("admin", "admin")

	// Session should be valid initially
	if !auth.ValidateSession(sessionID) {
		t.Error("session should be valid initially")
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Session should be expired
	if auth.ValidateSession(sessionID) {
		t.Error("session should be expired")
	}
}

// TestCSRFProtection tests CSRF token generation and validation
func TestCSRFProtection(t *testing.T) {
	auth := NewAuthService()

	// Generate CSRF token
	token := auth.GenerateCSRFToken()
	if token == "" {
		t.Error("CSRF token should not be empty")
	}

	// Valid token should pass validation
	if !auth.ValidateCSRFToken(token) {
		t.Error("valid CSRF token should pass validation")
	}

	// Invalid token should fail
	if auth.ValidateCSRFToken("invalid") {
		t.Error("invalid CSRF token should fail validation")
	}

	// Token can only be used once
	auth.ValidateCSRFToken(token) // Consume token
	if auth.ValidateCSRFToken(token) {
		t.Error("CSRF token should only be valid once")
	}
}
