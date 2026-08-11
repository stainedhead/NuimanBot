package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNotificationBadge_AppearsOnNonHistoryPages is FR-R19's required test:
// UnviewedCount was previously only wired into History's own two handlers
// (handleHistory/handleHistoryDetail), so an unviewed completed Run's badge
// was invisible everywhere else (FR-044 promises it on every authenticated
// page). This exercises Chats, Jobs, and the Dashboard — three pages that
// never called withUnviewedRunCount before this fix — plus History itself,
// to prove the fix doesn't regress the page the badge originally worked on.
func TestNotificationBadge_AppearsOnNonHistoryPages(t *testing.T) {
	const badgeCount = 3

	pages := []struct {
		name string
		path string
		role string
		set  func(s *Server, h *MockHistoryService)
	}{
		{
			name: "Chats",
			path: "/admin/chats",
			role: "user",
			set: func(s *Server, h *MockHistoryService) {
				s.SetChatsService(NewMockChatsService())
				s.SetHistoryService(h)
			},
		},
		{
			name: "Jobs",
			path: "/admin/jobs",
			role: "user",
			set: func(s *Server, h *MockHistoryService) {
				s.SetJobsService(NewMockJobsService())
				s.SetHistoryService(h)
			},
		},
		{
			name: "Dashboard",
			path: "/admin/dashboard",
			role: "admin",
			set: func(s *Server, h *MockHistoryService) {
				s.SetHistoryService(h)
			},
		},
		{
			name: "History",
			path: "/admin/history",
			role: "user",
			set: func(s *Server, h *MockHistoryService) {
				s.SetHistoryService(h)
			},
		},
	}

	for _, p := range pages {
		t.Run(p.name, func(t *testing.T) {
			server := NewServer(":0")
			mockHistory := NewMockHistoryService()
			mockHistory.unviewedCount = badgeCount
			p.set(server, mockHistory)

			req := httptest.NewRequest(http.MethodGet, p.path, nil)
			req.AddCookie(sessionCookieFor(server, "alice", p.role))
			w := httptest.NewRecorder()
			server.httpServer.Handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
			}
			// The badge markup lives in nav.html's History link and renders
			// the raw count whenever BaseData.UnviewedRunCount is non-zero.
			if !strings.Contains(w.Body.String(), ">3<") {
				t.Fatalf("expected the unviewed-run badge (count 3) to render on the %s page's nav sidebar, body: %s", p.name, w.Body.String())
			}
		})
	}
}

// TestNotificationBadge_AbsentWhenHistoryServiceUnconfigured proves the
// centralized population is defensive: a page rendered before
// SetHistoryService is ever called (or in a deployment/test that never
// wires History) must not panic and must simply omit the badge.
func TestNotificationBadge_AbsentWhenHistoryServiceUnconfigured(t *testing.T) {
	server := NewServer(":0")
	server.SetChatsService(NewMockChatsService()) // HistoryService deliberately NOT set

	req := httptest.NewRequest(http.MethodGet, "/admin/chats", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (no panic) even with HistoryService unset, got %d, body: %s", w.Code, w.Body.String())
	}
}
