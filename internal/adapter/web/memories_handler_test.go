package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// MockMemoriesService is a test double for MemoriesService. Cell ownership
// is modeled by ConversationID, matching the Memories usecase Service's
// documented ownerUserID -> ConversationID mapping.
type MockMemoriesService struct {
	cells map[string]*memoryv2.MemoryCell // cellID -> cell

	listErr error
	getErr  error
	askErr  error
	askedQ  string // last question passed to AskAboutCell, for assertions
}

func NewMockMemoriesService() *MockMemoriesService {
	return &MockMemoriesService{cells: make(map[string]*memoryv2.MemoryCell)}
}

func (m *MockMemoriesService) ListCells(_ context.Context, ownerUserID string, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := []*memoryv2.MemoryCell{}
	for _, c := range m.cells {
		if c.ConversationID != ownerUserID {
			continue
		}
		if filter.Scene != "" && c.Scene != filter.Scene {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (m *MockMemoriesService) GetCell(_ context.Context, ownerUserID, cellID string) (*memoryv2.MemoryCell, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.cells[cellID]
	if !ok || c.ConversationID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (m *MockMemoriesService) AskAboutCell(_ context.Context, ownerUserID, cellID, question string) (string, error) {
	m.askedQ = question
	if m.askErr != nil {
		return "", m.askErr
	}
	c, ok := m.cells[cellID]
	if !ok || c.ConversationID != ownerUserID {
		return "", domain.ErrNotFound
	}
	return "mock answer about " + c.Scene, nil
}

func newMemoriesTestServer(t *testing.T) (*Server, *MockMemoriesService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockMemoriesService()
	server.SetMemoriesService(mock)
	return server, mock
}

func newTestMemoryCell(id, ownerUserID, scene, content string) *memoryv2.MemoryCell {
	now := time.Now()
	return &memoryv2.MemoryCell{
		ID:             id,
		ConversationID: ownerUserID,
		Scene:          scene,
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        content,
		Source:         `["msg-1"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestHandleMemories_RequiresAuth(t *testing.T) {
	server, _ := newMemoriesTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/memories", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleMemories_ListEmpty(t *testing.T) {
	server, _ := newMemoriesTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/memories", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleMemories_ListScopedToOwner(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "Alice likes window seats")
	mock.cells["bob-1"] = newTestMemoryCell("bob-1", "bob", "trip-planning", "Bob's secret plan")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Alice likes window seats") {
		t.Fatal("expected alice's cell content to appear in rendered page")
	}
	if strings.Contains(w.Body.String(), "Bob's secret plan") {
		t.Fatal("expected bob's cell content to NOT appear in alice's rendered page")
	}
}

func TestHandleMemories_ListWithSceneFilter(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "trip content")
	mock.cells["alice-2"] = newTestMemoryCell("alice-2", "alice", "work-notes", "work content")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories?scene=work-notes", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "work content") {
		t.Fatal("expected work-notes cell to appear")
	}
	if strings.Contains(w.Body.String(), "trip content") {
		t.Fatal("expected trip-planning cell to be filtered out")
	}
}

func TestHandleMemories_MethodNotAllowed(t *testing.T) {
	server, _ := newMemoriesTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/memories", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 (no POST/create route exists for Memories, FR-046), got %d", w.Code)
	}
}

func TestHandleMemories_NoCreateEditDeleteControlsRendered(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["alice-1"] = newTestMemoryCell("alice-1", "alice", "trip-planning", "trip content")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	body := strings.ToLower(w.Body.String())
	if strings.Contains(body, "method=\"post\"") {
		t.Fatal("expected no POST forms on the read-only Memories list page (FR-046)")
	}
	// Check for actual delete/edit controls (links/buttons), not incidental
	// prose mentioning the words — the filter form itself is a legitimate
	// GET control and must not trip this check.
	for _, control := range []string{">delete<", ">edit<", "/delete\"", "/edit\""} {
		if strings.Contains(body, control) {
			t.Fatalf("expected no delete/edit controls on the read-only Memories list page (FR-046), found %q", control)
		}
	}
}

func TestHandleMemoryDetail_OwnerSuccess(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "Alice likes window seats")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories/cell-1", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Alice likes window seats") {
		t.Fatal("expected cell content to appear in rendered detail page")
	}
}

func TestHandleMemoryDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "bob", "trip-planning", "Bob's secret plan")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories/cell-1", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access (not 403 — existence must not be disclosed), got %d", w.Code)
	}
}

func TestHandleMemoryDetail_NotFound(t *testing.T) {
	server, _ := newMemoriesTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/memories/does-not-exist", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleMemorySubroutes_MethodNotAllowed(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "content")

	req := httptest.NewRequest(http.MethodPost, "/admin/memories/cell-1", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 (no edit/delete route exists for Memories, FR-046), got %d", w.Code)
	}
}

func TestHandleMemorySubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newMemoriesTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/memories/", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty cell ID, got %d", w.Code)
	}
}

func TestHandleMemorySubroutes_ExtraPathSegmentNotFound(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "content")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories/cell-1/bogus", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unsupported subroute (no actions exist for Memories), got %d", w.Code)
	}
}

func TestMemoriesService_NotConfigured(t *testing.T) {
	server := NewServer(":0") // no SetMemoriesService call
	req := httptest.NewRequest(http.MethodGet, "/admin/memories", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when MemoriesService is unconfigured, got %d", w.Code)
	}
}

// TestMemoryIDAndActionFromPath supersedes the removed memoryIDFromPath and
// its identically-named test — memoryIDAndActionFromPath (added for the
// FR-R4 /ask route) does everything the old parser did plus action
// parsing, leaving the old one an orphaned, never-called-in-production
// function once handleMemorySubroutes switched over to it.
func TestMemoryIDAndActionFromPath(t *testing.T) {
	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/admin/memories/abc", "abc", ""},
		{"/admin/memories/", "", ""},
		{"/admin/memories/abc/ask", "abc", "ask"},
		{"/admin/memories/abc/extra/segments", "", ""},
	}
	for _, tc := range cases {
		gotID, gotAction := memoryIDAndActionFromPath(tc.path)
		if gotID != tc.wantID || gotAction != tc.wantAction {
			t.Errorf("path %q: expected (%q, %q), got (%q, %q)", tc.path, tc.wantID, tc.wantAction, gotID, gotAction)
		}
	}
}

func TestHandleMemoryAsk_OwnerSuccess_RendersAnswer(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "Alice likes window seats")
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("question", "What seat does Alice prefer?")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/memories/cell-1/ask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if mock.askedQ != "What seat does Alice prefer?" {
		t.Fatalf("expected the question to reach AskAboutCell, got %q", mock.askedQ)
	}
	if !strings.Contains(w.Body.String(), "mock answer about trip-planning") {
		t.Fatalf("expected the answer to be rendered, body: %s", w.Body.String())
	}
}

func TestHandleMemoryAsk_RequiresCSRF(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "content")
	cookie := sessionCookieFor(server, "alice", "user")

	form := url.Values{}
	form.Set("question", "anything")
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/memories/cell-1/ask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
}

func TestHandleMemoryAsk_CrossOwnerReturns404(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "bob", "trip-planning", "Bob's secret")
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("question", "anything")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/memories/cell-1/ask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access, got %d", w.Code)
	}
}

func TestHandleMemoryAsk_MethodNotAllowed(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "content")

	req := httptest.NewRequest(http.MethodGet, "/admin/memories/cell-1/ask", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 (ask is POST-only), got %d", w.Code)
	}
}

func TestHandleMemoryAsk_ServiceErrorRendersInline(t *testing.T) {
	server, mock := newMemoriesTestServer(t)
	mock.cells["cell-1"] = newTestMemoryCell("cell-1", "alice", "trip-planning", "content")
	mock.askErr = context.DeadlineExceeded
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("question", "anything")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/memories/cell-1/ask", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	// A failed LLM call is not a server bug — render the detail page again
	// with an inline error, not a 500.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with an inline error message, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(strings.ToLower(w.Body.String()), "get an answer") {
		t.Fatalf("expected an inline error message, body: %s", w.Body.String())
	}
}

// Ensure MockMemoriesService satisfies the interface at compile time.
var _ MemoriesService = (*MockMemoriesService)(nil)
