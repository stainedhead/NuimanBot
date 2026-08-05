package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEnvironmentScaffold_RoutesRegisteredAndAuthGated is a coarse smoke
// test for the Projects/Jobs/Chores/History/Memories placeholder routes
// added ahead of their full implementation (see each <env>_handler.go's
// "STATUS: scaffold only" doc comment). It only asserts routes exist and
// are auth-gated — NOT full behavior, which lands with each environment's
// real implementation.
func TestEnvironmentScaffold_RoutesRegisteredAndAuthGated(t *testing.T) {
	server := NewServer(":0")

	paths := []string{
		"/admin/projects", "/admin/projects/x",
		"/admin/jobs", "/admin/jobs/x",
		"/admin/chores", "/admin/chores/x",
		"/admin/history", "/admin/history/x",
		"/admin/memories", "/admin/memories/x",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("path %q: expected 401 for unauthenticated request, got %d", path, w.Code)
		}
	}
}

func TestEnvironmentScaffold_AuthenticatedButUnconfiguredReturns500(t *testing.T) {
	server := NewServer(":0")
	cookie := sessionCookieFor(server, "alice", "user")

	paths := []string{"/admin/projects", "/admin/jobs", "/admin/chores", "/admin/history", "/admin/memories"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("path %q: expected 500 (service not configured), got %d", path, w.Code)
		}
	}
}
