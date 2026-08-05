package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
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

// FR-R15: a worker_pool_size above domain.MaxWorkerPoolSize must be
// rejected the same way a non-positive value already is.
func TestHandleSettings_AdminWorkerPoolSizeAboveUpperBoundRejected(t *testing.T) {
	server, mock := newSettingsTestServer(t)
	cookie := sessionCookieFor(server, "admin", "admin")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("worker_pool_size", strconv.Itoa(domain.MaxWorkerPoolSize+1))
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render with flash error), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), strconv.Itoa(domain.MaxWorkerPoolSize)) {
		t.Fatalf("expected validation error message mentioning the upper bound, got body: %s", w.Body.String())
	}
	if len(mock.setPoolSizeCalls) != 0 {
		t.Fatal("expected no SetWorkerPoolSize call for a value above the upper bound")
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

// FR-R11: the Settings page must not present config-file-only network
// fields (bind address, allowlist) as if they were live-editable via the
// UI, and must clarify that Network access mode only affects the
// allowlist-check middleware's behavior — not the actual listener bind
// address, which is fixed at process start from config.yaml.
func TestHandleSettings_NetworkFieldsIndicateConfigFileOnly(t *testing.T) {
	server, _ := newSettingsTestServer(t)
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{
		Mode:        domain.AccessModeLocalhostOnly,
		BindAddress: "0.0.0.0:8443",
		Allowlist:   []string{"example.com"},
	})
	cookie := sessionCookieFor(server, "admin", "admin")
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()

	if !strings.Contains(body, "0.0.0.0:8443") {
		t.Fatal("expected the bind address value to be displayed on the Settings page")
	}
	if !strings.Contains(body, "config file only") && !strings.Contains(body, "config-file-only") {
		t.Fatal("expected the page to mark bind address/allowlist as config-file-only, not live-editable")
	}
	if !strings.Contains(body, "does not change the server's listening address") {
		t.Fatal("expected an explanatory note that Network access mode does not rebind the actual listener")
	}
}

// Ensure MockSettingsService satisfies the interface at compile time.
var _ SettingsService = (*MockSettingsService)(nil)
