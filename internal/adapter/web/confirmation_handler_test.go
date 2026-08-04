package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockConfirmationService is a test double for ConfirmationService (P5.8).
type MockConfirmationService struct {
	pending map[string]PendingConfirmation

	listErr    error
	getErr     error
	resolveErr error

	resolveCalls []struct {
		ID       string
		Approved bool
	}
}

func NewMockConfirmationService() *MockConfirmationService {
	return &MockConfirmationService{pending: make(map[string]PendingConfirmation)}
}

func (m *MockConfirmationService) Add(c PendingConfirmation) {
	m.pending[c.ID] = c
}

func (m *MockConfirmationService) ListPendingConfirmations(ctx context.Context) ([]PendingConfirmation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]PendingConfirmation, 0, len(m.pending))
	for _, c := range m.pending {
		out = append(out, c)
	}
	return out, nil
}

func (m *MockConfirmationService) GetConfirmation(ctx context.Context, id string) (PendingConfirmation, error) {
	if m.getErr != nil {
		return PendingConfirmation{}, m.getErr
	}
	c, ok := m.pending[id]
	if !ok {
		return PendingConfirmation{}, errors.New("confirmation not found")
	}
	return c, nil
}

func (m *MockConfirmationService) ResolveConfirmation(ctx context.Context, confirmationID string, approved bool) error {
	if m.resolveErr != nil {
		return m.resolveErr
	}
	m.resolveCalls = append(m.resolveCalls, struct {
		ID       string
		Approved bool
	}{confirmationID, approved})
	delete(m.pending, confirmationID)
	return nil
}

// TestConfirmationsPage_RequiresAuth verifies unauthenticated requests are
// redirected to login rather than shown the confirmations list.
func TestConfirmationsPage_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	w := httptest.NewRecorder()

	server.handleConfirmations(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect to login, got status %d", w.Code)
	}
}

// TestConfirmationsPage_RegisteredRoute_RequiresAuth verifies the route is
// actually wired into the mux (not just the bare handler), and that the
// mux-level auth-gating middleware also redirects unauthenticated requests.
func TestConfirmationsPage_RegisteredRoute_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 from requireRole middleware for an unauthenticated request, got %d", w.Code)
	}
}

// TestConfirmationsPage_UserRoleCanAccess verifies a plain "user"-role
// session (not just admin) can reach the confirmations page, since P5.8
// requires it be scoped to "the logged-in user (or all, if admin)" rather
// than admin-only.
func TestConfirmationsPage_UserRoleCanAccess(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	svc := NewMockConfirmationService()
	server.SetConfirmationService(svc)

	if err := auth.AddUser("alice", "password", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected a RoleUser session to reach the confirmations page, got %d", w.Code)
	}
}

// TestConfirmationsPage_NonAdminSeesOnlyOwnConfirmations verifies a non-admin
// user's confirmations list is filtered to their own UserID, per P5.8's
// requirement that a user only sees (and can act on) their own pending
// confirmations.
func TestConfirmationsPage_NonAdminSeesOnlyOwnConfirmations(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "alice", ToolName: "github", Summary: "Alice own action"})
	svc.Add(PendingConfirmation{ID: "c2", UserID: "bob", ToolName: "executor", Summary: "Bob action"})
	server.SetConfirmationService(svc)

	if err := auth.AddUser("alice", "password", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleConfirmations(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Alice own action") {
		t.Error("expected alice's own confirmation to be visible")
	}
	if strings.Contains(body, "Bob action") {
		t.Error("expected bob's confirmation to be hidden from alice (non-admin)")
	}
}

// TestConfirmationsPage_AdminSeesAllConfirmations verifies an admin user's
// list is not filtered by ownership.
func TestConfirmationsPage_AdminSeesAllConfirmations(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "alice", ToolName: "github", Summary: "Alice own action"})
	svc.Add(PendingConfirmation{ID: "c2", UserID: "bob", ToolName: "executor", Summary: "Bob action"})
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleConfirmations(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Alice own action") || !strings.Contains(body, "Bob action") {
		t.Error("expected admin to see both users' confirmations")
	}
}

// TestConfirmationApprove_OwnerCanApprove verifies the owning user can
// approve their own pending confirmation, and that the call reaches
// ConfirmationService.ResolveConfirmation with approved=true.
func TestConfirmationApprove_OwnerCanApprove(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "alice", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	if err := auth.AddUser("alice", "password", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/c1/approve", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect after approve, got %d", w.Code)
	}
	if len(svc.resolveCalls) != 1 {
		t.Fatalf("expected exactly 1 ResolveConfirmation call, got %d", len(svc.resolveCalls))
	}
	if svc.resolveCalls[0].ID != "c1" || !svc.resolveCalls[0].Approved {
		t.Errorf("expected ResolveConfirmation(c1, true), got ResolveConfirmation(%s, %v)",
			svc.resolveCalls[0].ID, svc.resolveCalls[0].Approved)
	}
}

// TestConfirmationDeny_OwnerCanDeny verifies the owning user can deny their
// own pending confirmation, and that the call reaches
// ConfirmationService.ResolveConfirmation with approved=false.
func TestConfirmationDeny_OwnerCanDeny(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "alice", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	if err := auth.AddUser("alice", "password", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/c1/deny", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect after deny, got %d", w.Code)
	}
	if len(svc.resolveCalls) != 1 {
		t.Fatalf("expected exactly 1 ResolveConfirmation call, got %d", len(svc.resolveCalls))
	}
	if svc.resolveCalls[0].ID != "c1" || svc.resolveCalls[0].Approved {
		t.Errorf("expected ResolveConfirmation(c1, false), got ResolveConfirmation(%s, %v)",
			svc.resolveCalls[0].ID, svc.resolveCalls[0].Approved)
	}
}

// TestConfirmationResolve_MismatchedUserRejected is the key security test for
// P5.8: a non-admin user must not be able to resolve a confirmation they
// don't own — the request must be rejected (403) and ResolveConfirmation must
// never be called.
func TestConfirmationResolve_MismatchedUserRejected(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "bob", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	// alice tries to approve bob's confirmation.
	if err := auth.AddUser("alice", "password", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("alice", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/c1/approve", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for a mismatched-user request, got %d", w.Code)
	}
	if len(svc.resolveCalls) != 0 {
		t.Errorf("expected ResolveConfirmation to never be called for a mismatched user, got %d calls", len(svc.resolveCalls))
	}
}

// TestConfirmationResolve_AdminCanResolveAnyUsersConfirmation verifies an
// admin is exempt from the ownership check.
func TestConfirmationResolve_AdminCanResolveAnyUsersConfirmation(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "bob", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/c1/approve", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (admin allowed to resolve any user's confirmation), got %d", w.Code)
	}
	if len(svc.resolveCalls) != 1 {
		t.Fatalf("expected exactly 1 ResolveConfirmation call, got %d", len(svc.resolveCalls))
	}
}

// TestConfirmationResolve_UnauthenticatedRedirectsToLogin verifies the
// approve/deny endpoints are also auth-gated, not just the list page.
func TestConfirmationResolve_UnauthenticatedRedirectsToLogin(t *testing.T) {
	server := NewServer(":0")
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "bob", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/c1/approve", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated approve request, got %d", w.Code)
	}
	if len(svc.resolveCalls) != 0 {
		t.Errorf("expected ResolveConfirmation to never be called when unauthenticated, got %d calls", len(svc.resolveCalls))
	}
}

// TestConfirmationResolve_UnknownIDReturnsNotFound verifies resolving an
// unknown confirmation ID returns 404 rather than panicking or silently
// succeeding.
func TestConfirmationResolve_UnknownIDReturnsNotFound(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	svc := NewMockConfirmationService()
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodPost, "/admin/confirmations/does-not-exist/approve", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for an unknown confirmation ID, got %d", w.Code)
	}
}

// TestConfirmationResolve_UnknownSubpathIs404 verifies an unrecognized
// /admin/confirmations/{id}/<something-else> subpath is a 404, matching the
// existing bots/users routing convention.
func TestConfirmationResolve_UnknownSubpathIs404(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	svc := NewMockConfirmationService()
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations/c1/unknown", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unrecognized confirmation subpath, got %d", w.Code)
	}
}

// TestConfirmationResolve_GetMethodNotAllowed verifies approve/deny only
// accept POST.
func TestConfirmationResolve_GetMethodNotAllowed(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)
	svc := NewMockConfirmationService()
	svc.Add(PendingConfirmation{ID: "c1", UserID: "admin", ToolName: "github", Summary: "Merge PR"})
	server.SetConfirmationService(svc)

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations/c1/approve", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for a GET to the approve endpoint, got %d", w.Code)
	}
}

// TestConfirmationIDFromPath covers the path-parsing helper directly.
func TestConfirmationIDFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/admin/confirmations/abc-123/approve", "abc-123"},
		{"/admin/confirmations/abc-123/deny", "abc-123"},
		{"/admin/confirmations//approve", ""},
		{"/admin/confirmations/abc-123", ""},
		{"/admin/confirmations/abc-123/approve/extra", ""},
	}
	for _, c := range cases {
		if got := confirmationIDFromPath(c.path); got != c.want {
			t.Errorf("confirmationIDFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestConfirmationsPage_NilServiceRendersEmpty verifies the page degrades
// gracefully (renders an empty list, not an error) when no
// ConfirmationService has been wired at all.
func TestConfirmationsPage_NilServiceRendersEmpty(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/confirmations", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleConfirmations(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even with no ConfirmationService configured, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "No pending confirmations") {
		t.Error("expected the empty-state message when no ConfirmationService is configured")
	}
}
