package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequirePasswordChange_NoCookie verifies the middleware passes through when no cookie exists.
func TestRequirePasswordChange_NoCookie(t *testing.T) {
	server := NewServer(":0")

	called := false
	handler := server.requirePasswordChange(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when no cookie present")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestRequirePasswordChange_InvalidSession verifies pass-through when session is invalid.
func TestRequirePasswordChange_InvalidSession(t *testing.T) {
	server := NewServer(":0")

	called := false
	handler := server.requirePasswordChange(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid-session-id"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called when session is invalid")
	}
}

// TestRequirePasswordChange_ChangePasswordPathAllowed verifies no redirect loop for change-password path.
func TestRequirePasswordChange_ChangePasswordPathAllowed(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("admin", "admin", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.createSessionWithFlags("admin", "admin", true)

	called := false
	handler := server.requirePasswordChange(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Accessing the change-password path itself must not redirect.
	req := httptest.NewRequest(http.MethodGet, "/admin/change-password", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("expected next handler to be called for /admin/change-password path")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for change-password path, got %d", w.Code)
	}
}
