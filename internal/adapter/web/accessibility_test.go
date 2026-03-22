package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLoginPageAccessibility tests accessibility features of login page
func TestLoginPageAccessibility(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	w := httptest.NewRecorder()
	server.handleLogin(w, req)

	body := w.Body.String()

	// Check for proper HTML structure
	requiredElements := []string{
		`<label for="username"`,
		`<label for="password"`,
		`type="text"`,
		`type="password"`,
		`type="submit"`,
	}

	for _, element := range requiredElements {
		if !strings.Contains(body, element) {
			t.Errorf("Login page missing accessibility element: %s", element)
		}
	}
}

// TestDashboardResponsiveClasses tests responsive design classes
func TestDashboardResponsiveClasses(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()
	server.handleDashboard(w, req)

	body := w.Body.String()

	// Check for Tailwind responsive classes
	responsiveClasses := []string{
		"sm:", // Small screens
		"lg:", // Large screens
		"container",
		"mx-auto",
		"px-4",
	}

	for _, class := range responsiveClasses {
		if !strings.Contains(body, class) {
			t.Errorf("Dashboard missing responsive class: %s", class)
		}
	}
}

// TestUsersPageResponsive tests users page responsive design
func TestUsersPageResponsive(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	server.SetProfileService(NewMockProfileService())

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()
	server.handleUsers(w, req)

	body := w.Body.String()

	// Check for viewport meta tag
	if !strings.Contains(body, "viewport") {
		t.Error("Users page should have viewport meta tag")
	}

	// Check for responsive container
	if !strings.Contains(body, "container") {
		t.Error("Users page should have responsive container")
	}
}

// TestKeyboardNavigation tests keyboard navigation support
func TestKeyboardNavigation(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	server.SetProfileService(NewMockProfileService())

	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()
	server.handleUsers(w, req)

	body := w.Body.String()

	// Check for keyboard-accessible elements
	// Links should be present
	if !strings.Contains(body, "<a href") {
		t.Error("Page should have navigable links")
	}

	// Page title should be present
	if !strings.Contains(body, "Users") {
		t.Error("Page should have proper heading")
	}
}

// TestStylesCSS tests that styles.css is served
func TestStylesCSS(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected styles.css to be served, got status %d", w.Code)
	}

	body := w.Body.String()

	// Check for custom CSS classes
	expectedStyles := []string{
		"form-group",
		"btn",
		"badge",
		"stat-card",
	}

	for _, style := range expectedStyles {
		if !strings.Contains(body, style) {
			t.Errorf("styles.css missing style: %s", style)
		}
	}
}
