package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// MockHistoryService is a test double for HistoryService.
type MockHistoryService struct {
	runs map[string]*domain.Run // runID -> run

	listErr       error
	markViewedErr error
	unviewedCount int
	unviewedErr   error

	lastFilter          domain.RunFilter
	markViewedCalledFor string
}

func NewMockHistoryService() *MockHistoryService {
	return &MockHistoryService{runs: make(map[string]*domain.Run)}
}

func (m *MockHistoryService) ListRuns(_ context.Context, ownerUserID string, filter domain.RunFilter) ([]*domain.Run, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	m.lastFilter = filter
	out := make([]*domain.Run, 0)
	for _, r := range m.runs {
		if r.OwnerUserID != ownerUserID {
			continue
		}
		if filter.SourceType != nil && r.SourceType != *filter.SourceType {
			continue
		}
		if filter.Status != nil && r.Status != *filter.Status {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *MockHistoryService) GetRun(_ context.Context, ownerUserID, runID string) (*domain.Run, error) {
	r, ok := m.runs[runID]
	if !ok || r.OwnerUserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return r, nil
}

func (m *MockHistoryService) MarkViewed(_ context.Context, ownerUserID, runID string) error {
	if m.markViewedErr != nil {
		return m.markViewedErr
	}
	r, ok := m.runs[runID]
	if !ok || r.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	m.markViewedCalledFor = runID
	now := time.Now()
	r.NotifiedAt = &now
	return nil
}

func (m *MockHistoryService) UnviewedCount(_ context.Context, ownerUserID string) (int, error) {
	if m.unviewedErr != nil {
		return 0, m.unviewedErr
	}
	return m.unviewedCount, nil
}

func newHistoryTestServer(t *testing.T) (*Server, *MockHistoryService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockHistoryService()
	server.SetHistoryService(mock)
	return server, mock
}

func TestHandleHistory_RequiresAuth(t *testing.T) {
	server, _ := newHistoryTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/history", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleHistory_ServiceNotConfigured(t *testing.T) {
	server := NewServer(":0")
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when HistoryService is unconfigured, got %d", w.Code)
	}
}

func TestHandleHistory_ListEmpty(t *testing.T) {
	server, _ := newHistoryTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleHistory_FiltersParsedFromQueryParams(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/history?source_type=job&status=failed", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if mock.lastFilter.SourceType == nil || *mock.lastFilter.SourceType != domain.SourceTypeJob {
		t.Fatalf("expected source_type=job to reach the service filter, got %+v", mock.lastFilter)
	}
	if mock.lastFilter.Status == nil || *mock.lastFilter.Status != domain.RunStatusFailed {
		t.Fatalf("expected status=failed to reach the service filter, got %+v", mock.lastFilter)
	}
}

func TestHandleHistory_DateRangeParsedFromQueryParams(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/history?since=2026-01-01&until=2026-01-31", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if mock.lastFilter.Since == nil {
		t.Fatal("expected since to be parsed into the filter")
	}
	if mock.lastFilter.Until == nil {
		t.Fatal("expected until to be parsed into the filter")
	}
	if !mock.lastFilter.Since.Before(*mock.lastFilter.Until) {
		t.Fatalf("expected since to be before until, got since=%v until=%v", mock.lastFilter.Since, mock.lastFilter.Until)
	}
}

func TestHandleHistory_BadgeCountSetOnBaseData(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.unviewedCount = 3
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/history", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "3") {
		t.Fatalf("expected unviewed count badge (3) to appear in rendered page, body: %s", w.Body.String())
	}
}

func TestHandleHistoryDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "bob", Status: domain.RunStatusCompleted}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access, got %d", w.Code)
	}
}

func TestHandleHistoryDetail_OwnerSuccess(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", Status: domain.RunStatusCompleted}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "run-1") {
		t.Fatal("expected run ID to appear in rendered detail page")
	}
}

func TestHandleHistoryDetail_MarksViewedAsSideEffect(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusCompleted}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if mock.markViewedCalledFor != "run-1" {
		t.Fatal("expected viewing the detail page to call MarkViewed (FR-044)")
	}
}

func TestHandleHistoryDetail_LogAndResultsContentRendered(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run-1.log")
	resultsPath := filepath.Join(dir, "RESULTS.md")
	if err := os.WriteFile(logPath, []byte("log line one"), 0644); err != nil {
		t.Fatalf("writing test log file: %v", err)
	}
	if err := os.WriteFile(resultsPath, []byte("the results"), 0644); err != nil {
		t.Fatalf("writing test results file: %v", err)
	}
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusCompleted, LogPath: logPath, ResultsPath: resultsPath}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "log line one") {
		t.Fatal("expected log content to appear in rendered detail page")
	}
	if !strings.Contains(w.Body.String(), "the results") {
		t.Fatal("expected results content to appear in rendered detail page")
	}
}

func TestHandleHistoryDetail_MissingLogAndResultsHandledGracefully(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusQueued, LogPath: "", ResultsPath: ""}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even with no log/results paths, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleHistoryDetail_SkipReasonAndErrorRendered(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	skipReason := "skipped — previous run still active"
	runErr := "provider timeout"
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice", Status: domain.RunStatusSkipped, SkipReason: &skipReason, Error: &runErr}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), skipReason) {
		t.Fatal("expected SkipReason to appear in rendered detail page")
	}
	if !strings.Contains(w.Body.String(), runErr) {
		t.Fatal("expected Error to appear in rendered detail page")
	}
}

func TestHandleHistorySubroutes_UnknownActionNotFound(t *testing.T) {
	server, mock := newHistoryTestServer(t)
	mock.runs["run-1"] = &domain.Run{ID: "run-1", OwnerUserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1/bogus", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subroute action, got %d", w.Code)
	}
}

func TestHandleHistorySubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newHistoryTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty run ID, got %d", w.Code)
	}
}

func TestHandleHistorySubroutes_RequiresAuth(t *testing.T) {
	server, _ := newHistoryTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleHistorySubroutes_ServiceNotConfigured(t *testing.T) {
	server := NewServer(":0")
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/history/run-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when HistoryService is unconfigured, got %d", w.Code)
	}
}

// Ensure MockHistoryService satisfies the interface at compile time.
var _ HistoryService = (*MockHistoryService)(nil)
