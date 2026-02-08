package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDashboardPageRequiresAuth tests that dashboard requires authentication
func TestDashboardPageRequiresAuth(t *testing.T) {
	server := NewServer(":0")

	// Request without session should redirect to login
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect to login, got status %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/admin/login" {
		t.Errorf("expected redirect to /admin/login, got %s", location)
	}
}

// TestDashboardPageWithAuth tests that authenticated users can access dashboard
func TestDashboardPageWithAuth(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Create a test user and session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")

	// Request with valid session should show dashboard
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	// Check that dashboard content is present
	body := w.Body.String()
	expectedStrings := []string{
		"Dashboard",
		"Server Status",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("expected dashboard to contain '%s', but it doesn't", expected)
		}
	}
}

// TestDashboardServerStats tests that server stats are displayed
func TestDashboardServerStats(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Create a test user and session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")

	// Request dashboard
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	body := w.Body.String()

	// Dashboard should show version info
	if !strings.Contains(body, "Version") || !strings.Contains(body, "1.0.0") {
		t.Error("expected dashboard to show version information")
	}
}

// TestDashboardReloadConfig tests the reload config functionality
func TestDashboardReloadConfig(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	// Create a test user and session
	auth.AddUser("admin", "password", "admin")
	sessionID := auth.CreateSession("admin", "admin")

	// Request config reload
	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/reload", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
	})
	w := httptest.NewRecorder()

	server.handleReloadConfig(w, req)

	// Should return success (or redirect back to dashboard)
	if w.Code != http.StatusOK && w.Code != http.StatusFound {
		t.Errorf("expected status OK or redirect, got %d", w.Code)
	}
}
