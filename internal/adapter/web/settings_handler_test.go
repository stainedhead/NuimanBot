package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// MockSettingsService is a test double for SettingsService.
type MockSettingsService struct {
	poolSize         int
	setPoolSizeErr   error
	setPoolSizeCalls []int
	skills           []string
	chatDays         int
	projectDays      int
	historyDays      int
}

func NewMockSettingsService() *MockSettingsService {
	return &MockSettingsService{poolSize: 3, skills: []string{"websearch", "notes"}, chatDays: 90, projectDays: 180, historyDays: 90}
}

func (m *MockSettingsService) WorkerPoolSize() int { return m.poolSize }

func (m *MockSettingsService) SetWorkerPoolSize(n int) error {
	m.setPoolSizeCalls = append(m.setPoolSizeCalls, n)
	if m.setPoolSizeErr != nil {
		return m.setPoolSizeErr
	}
	m.poolSize = n
	return nil
}

func (m *MockSettingsService) SkillNames() []string { return m.skills }

func (m *MockSettingsService) RetentionDefaults() (int, int, int) {
	return m.chatDays, m.projectDays, m.historyDays
}

func newSettingsTestServer(t *testing.T) (*Server, *MockSettingsService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockSettingsService()
	server.SetSettingsService(mock)
	return server, mock
}

func TestHandleSettings_RequiresAuth(t *testing.T) {
	server, _ := newSettingsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleSettings_NotConfigured(t *testing.T) {
	server := NewServer(":0")
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when unconfigured, got %d", w.Code)
	}
}

func TestHandleSettings_NonAdminViewOnly(t *testing.T) {
	server, _ := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `name="worker_pool_size"`) {
		t.Fatal("expected non-admin view to not include the editable system-wide form")
	}
}

func TestHandleSettings_NonAdminPostForbidden(t *testing.T) {
	server, mock := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("worker_pool_size", "10")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin POST, got %d", w.Code)
	}
	if len(mock.setPoolSizeCalls) != 0 {
		t.Fatal("expected no worker pool size change for a non-admin POST")
	}
}

func TestHandleSettings_AdminCanUpdateWorkerPoolSize(t *testing.T) {
	server, mock := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "admin", "admin")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("worker_pool_size", "10")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if len(mock.setPoolSizeCalls) != 1 || mock.setPoolSizeCalls[0] != 10 {
		t.Fatalf("expected SetWorkerPoolSize(10) to be called once, got %v", mock.setPoolSizeCalls)
	}
}

func TestHandleSettings_AdminInvalidWorkerPoolSizeRejected(t *testing.T) {
	server, mock := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "admin", "admin")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("worker_pool_size", "-5")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render with flash error), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "positive integer") {
		t.Fatal("expected validation error message in response")
	}
	if len(mock.setPoolSizeCalls) != 0 {
		t.Fatal("expected no SetWorkerPoolSize call for an invalid value")
	}
}

func TestHandleSettings_AdminInvalidCSRFRejected(t *testing.T) {
	server, mock := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "admin", "admin")
	form := url.Values{}
	form.Set("worker_pool_size", "10")
	form.Set("csrf_token", "bogus")

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF, got %d", w.Code)
	}
	if len(mock.setPoolSizeCalls) != 0 {
		t.Fatal("expected no change with an invalid CSRF token")
	}
}

func TestHandleSettings_AdminCanChangeNetworkMode(t *testing.T) {
	server, _ := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "admin", "admin")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("network_mode", "remote")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := server.NetworkAccessConfig().Mode; string(got) != "remote" {
		t.Fatalf("expected network mode to be updated to remote, got %v", got)
	}
}

// Ensure MockSettingsService satisfies the interface at compile time.
var _ SettingsService = (*MockSettingsService)(nil)
