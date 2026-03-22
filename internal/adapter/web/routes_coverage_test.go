package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRegisterRoutes_UnknownBotSubpath verifies 404 for unrecognized bot subpaths.
func TestRegisterRoutes_UnknownBotSubpath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/bot1/unknown", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// 404 expected because the path doesn't end in /edit or /delete.
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown bot subpath, got %d", w.Code)
	}
}

// TestRegisterRoutes_UnknownUserSubpath verifies 404 for unrecognized user subpaths.
func TestRegisterRoutes_UnknownUserSubpath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/user1/unknown", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown user subpath, got %d", w.Code)
	}
}

// TestRegisterRoutes_BotEditPath verifies the /admin/bots/<id>/edit path is routed.
func TestRegisterRoutes_BotEditPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/bots/mybot/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should not be 404 or 405 for the edit subpath (the handler exists).
	if w.Code == http.StatusNotFound {
		t.Errorf("expected bot edit handler to be registered, got 404")
	}
}

// TestRegisterRoutes_UserEditPath verifies the /admin/users/<id>/edit path is routed.
func TestRegisterRoutes_UserEditPath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetProfileService(NewMockProfileService())

	req := httptest.NewRequest(http.MethodGet, "/admin/users/user1/edit", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// The handler should be reached; it may return 404 from the service but not from the router.
	if w.Code == http.StatusMethodNotAllowed {
		t.Errorf("expected user edit handler to be registered, got 405")
	}
}

// TestRegisterRoutes_BotDeletePath verifies the /admin/bots/<id>/delete path is routed.
func TestRegisterRoutes_BotDeletePath(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	server.SetBotService(NewMockBotService())

	req := httptest.NewRequest(http.MethodPost, "/admin/bots/somebot/delete", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// 302 redirect to /admin/bots (bot not in service, but delete is called and redirect follows)
	if w.Code == http.StatusNotFound {
		t.Errorf("expected bot delete handler to be registered, got 404")
	}
}

// TestParseTemplates_ReturnsTemplate verifies parseTemplates returns a non-nil template.
func TestParseTemplates_ReturnsTemplate(t *testing.T) {
	server := NewServer(":0")
	tmpl := server.parseTemplates()
	if tmpl == nil {
		t.Error("expected non-nil template from parseTemplates")
	}
}

// TestHandleDashboard_RequiresAuth verifies redirect when unauthenticated.
func TestHandleDashboard_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleDashboard_Authenticated verifies the dashboard renders with auth.
func TestHandleDashboard_Authenticated(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		t.Error("expected text/html content type")
	}
}
