package web

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestMemoryIDFromPath(t *testing.T) {
	cases := []struct {
		path   string
		wantID string
	}{
		{"/admin/memories/abc", "abc"},
		{"/admin/memories/", ""},
		{"/admin/memories/abc/extra", ""},
	}
	for _, tc := range cases {
		if got := memoryIDFromPath(tc.path); got != tc.wantID {
			t.Errorf("path %q: expected %q, got %q", tc.path, tc.wantID, got)
		}
	}
}

// Ensure MockMemoriesService satisfies the interface at compile time.
var _ MemoriesService = (*MockMemoriesService)(nil)
