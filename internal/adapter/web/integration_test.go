//go:build integration
// +build integration

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestE2ELoginToDashboard tests complete login to dashboard flow
func TestE2ELoginToDashboard(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Add admin user
	if err := auth.AddUser("admin", "testpass", "admin"); err != nil {
		t.Fatalf("Failed to add user: %v", err)
	}

	// Step 1: Get login page
	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	w := httptest.NewRecorder()
	server.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for login page, got %d", w.Code)
	}

	// Step 2: Submit login form
	csrfToken := auth.GenerateCSRFToken()
	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "testpass")
	form.Add("csrf_token", csrfToken)

	req = httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	server.handleLogin(w, req)

	// Should redirect to dashboard
	if w.Code != http.StatusFound {
		t.Fatalf("Expected redirect after login, got %d", w.Code)
	}

	// Extract session cookie
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "session_id" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("No session cookie set after login")
	}

	// Step 3: Access dashboard with session
	req = httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	server.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for dashboard, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Dashboard") {
		t.Error("Expected dashboard content")
	}

	// Step 4: Logout
	req = httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	server.handleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("Expected redirect after logout, got %d", w.Code)
	}

	// Step 5: Verify session is destroyed
	if auth.ValidateSession(sessionCookie.Value) {
		t.Error("Session should be destroyed after logout")
	}
}

// TestE2EUserManagement tests complete user creation flow
func TestE2EUserManagement(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	profileService := NewMockProfileService()
	server.SetProfileService(profileService)

	// Setup admin session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")
	sessionCookie := &http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	}

	// Step 1: Access users list
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()
	server.handleUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for users list, got %d", w.Code)
	}

	// Step 2: Get create user form
	req = httptest.NewRequest(http.MethodGet, "/admin/users/create", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	server.handleUserCreate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for user create form, got %d", w.Code)
	}

	// Step 3: Submit create user form
	form := url.Values{}
	form.Add("userID", "testuser")
	form.Add("firstName", "Test")
	form.Add("lastName", "User")
	form.Add("primaryEmail", "test@example.com")
	form.Add("role", "user")

	req = httptest.NewRequest(http.MethodPost, "/admin/users/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	server.handleUserCreate(w, req)

	// Should redirect back to users list
	if w.Code != http.StatusFound {
		t.Fatalf("Expected redirect after user creation, got %d", w.Code)
	}

	// Step 4: Verify user was created
	profile, err := profileService.GetProfile(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("User should be created: %v", err)
	}
	if profile.FirstName != "Test" {
		t.Errorf("Expected first name 'Test', got '%s'", profile.FirstName)
	}

	// Step 5: Delete user
	req = httptest.NewRequest(http.MethodPost, "/admin/users/testuser/delete", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	server.handleUserDelete(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("Expected redirect after user deletion, got %d", w.Code)
	}

	// Verify user was deleted
	_, err = profileService.GetProfile(context.Background(), "testuser")
	if err == nil {
		t.Error("User should be deleted")
	}
}

// TestE2EBotManagement tests complete bot management flow
func TestE2EBotManagement(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	botService := NewMockBotService()
	server.SetBotService(botService)

	// Setup admin session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")
	sessionCookie := &http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	}

	// Step 1: Access bots list
	req := httptest.NewRequest(http.MethodGet, "/admin/bots", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()
	server.handleBots(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for bots list, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Bots") {
		t.Error("Expected bots page content")
	}
}

// TestE2EConfigPages tests accessing configuration pages
func TestE2EConfigPages(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Setup admin session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")
	sessionCookie := &http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	}

	testCases := []struct {
		name string
		path string
	}{
		{"LLM Config", "/admin/llm"},
		{"Server Config", "/admin/config"},
		{"Activity Logs", "/admin/logs"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.AddCookie(sessionCookie)
			w := httptest.NewRecorder()

			server.httpServer.Handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected OK for %s, got %d", tc.name, w.Code)
			}

			body := w.Body.String()
			if !strings.Contains(body, "Configuration") && !strings.Contains(body, "Logs") {
				t.Errorf("Expected relevant content for %s", tc.name)
			}
		})
	}
}
