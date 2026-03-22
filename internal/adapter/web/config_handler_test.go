package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper to create an authenticated server + sessionID.
func newAuthenticatedServer(t *testing.T) (*Server, string) {
	t.Helper()
	server := NewServer(":0")
	auth := server.auth
	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")
	return server, sessionID
}

// TestHandleLLMConfig_RequiresAuth verifies redirect when unauthenticated.
func TestHandleLLMConfig_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/llm", nil)
	w := httptest.NewRecorder()

	server.handleLLMConfig(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleLLMConfig_Authenticated verifies the LLM config page renders with auth.
func TestHandleLLMConfig_Authenticated(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/llm", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleLLMConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "LLM") {
		t.Error("expected body to contain 'LLM'")
	}
}

// TestHandleServerConfig_RequiresAuth verifies redirect when unauthenticated.
func TestHandleServerConfig_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/config", nil)
	w := httptest.NewRecorder()

	server.handleServerConfig(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleServerConfig_Authenticated verifies the server config page renders with auth.
func TestHandleServerConfig_Authenticated(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/config", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleServerConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Server Configuration") {
		t.Error("expected body to contain 'Server Configuration'")
	}
}

// TestHandleLogs_RequiresAuth verifies redirect when unauthenticated.
func TestHandleLogs_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/logs", nil)
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleLogs_Authenticated verifies the logs page renders with auth.
func TestHandleLogs_Authenticated(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/logs", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Logs") {
		t.Error("expected body to contain 'Logs'")
	}
}

// TestHandleAdminIndex_RequiresAuth verifies redirect when unauthenticated.
func TestHandleAdminIndex_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	w := httptest.NewRecorder()

	server.handleAdminIndex(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/admin/login" {
		t.Errorf("expected redirect to /admin/login, got %q", location)
	}
}

// TestHandleAdminIndex_Authenticated verifies redirect to dashboard with auth.
func TestHandleAdminIndex_Authenticated(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleAdminIndex(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/admin/dashboard" {
		t.Errorf("expected redirect to /admin/dashboard, got %q", location)
	}
}
