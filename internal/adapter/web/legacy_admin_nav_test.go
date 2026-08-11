package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLegacyAdminPagesIncludeNavSidebar verifies the four pre-existing admin
// pages (Dashboard, Bots, Users, Confirmations) render the same left-nav
// sidebar ({{template "nav" .}}) as the six newer environment pages
// (FR-R16). Before this fix, a user on one of these pages had no navigation
// path to the newer environments except by typing a URL directly.
func TestLegacyAdminPagesIncludeNavSidebar(t *testing.T) {
	server := NewServer(":0")
	server.SetProfileService(NewMockProfileService())
	server.SetBotService(NewMockBotService())
	server.SetConfirmationService(NewMockConfirmationService())

	auth := server.auth
	if err := auth.AddUser("admin", "password", "admin"); err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}
	sessionID := auth.CreateSession("admin", "admin")

	cases := []struct {
		name    string
		path    string
		handler http.HandlerFunc
	}{
		{"dashboard", "/admin/dashboard", server.handleDashboard},
		{"bots", "/admin/bots", server.handleBots},
		{"users", "/admin/users", server.handleUsers},
		{"confirmations", "/admin/confirmations", server.handleConfirmations},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			w := httptest.NewRecorder()

			tc.handler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status OK, got %d", w.Code)
			}

			body := w.Body.String()
			if !strings.Contains(body, `id="app-sidebar"`) {
				t.Errorf("%s page missing nav sidebar (expected {{template \"nav\" .}} to render #app-sidebar)", tc.name)
			}
		})
	}
}
