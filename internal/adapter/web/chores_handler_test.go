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
)

// MockChoresService is a test double for ChoresService.
type MockChoresService struct {
	chores map[string]*domain.Chore // choreID -> chore

	createErr  error
	getErr     error
	confirmErr error
}

func NewMockChoresService() *MockChoresService {
	return &MockChoresService{chores: make(map[string]*domain.Chore)}
}

func (m *MockChoresService) CreateChore(_ context.Context, ownerUserID, title, description, workingDirectory string, schedule domain.Schedule, userConfirmed bool) (*domain.Chore, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	now := time.Now()
	c := &domain.Chore{
		ID:                "chore-" + ownerUserID + "-" + title,
		OwnerUserID:       ownerUserID,
		Title:             title,
		Description:       description,
		WorkingDirectory:  workingDirectory,
		Schedule:          schedule,
		ScheduleConfirmed: userConfirmed,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if userConfirmed {
		c.NextFireTime = now.Add(24 * time.Hour)
	}
	m.chores[c.ID] = c
	return c, nil
}

func (m *MockChoresService) ListChores(_ context.Context, ownerUserID string) ([]*domain.Chore, error) {
	out := make([]*domain.Chore, 0)
	for _, c := range m.chores {
		if c.OwnerUserID == ownerUserID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *MockChoresService) GetChore(_ context.Context, ownerUserID, choreID string) (*domain.Chore, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (m *MockChoresService) DeleteChore(_ context.Context, ownerUserID, choreID string) error {
	c, ok := m.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	delete(m.chores, choreID)
	return nil
}

func (m *MockChoresService) ConfirmSchedule(_ context.Context, ownerUserID, choreID string) error {
	if m.confirmErr != nil {
		return m.confirmErr
	}
	c, ok := m.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	c.ScheduleConfirmed = true
	c.NextFireTime = time.Now().Add(24 * time.Hour)
	return nil
}

// newChoresTestServer wires a Server with a mock ChoresService.
func newChoresTestServer(t *testing.T) (*Server, *MockChoresService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockChoresService()
	server.SetChoresService(mock)
	return server, mock
}

func TestHandleChores_RequiresAuth(t *testing.T) {
	server, _ := newChoresTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/chores", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleChores_ListEmpty(t *testing.T) {
	server, _ := newChoresTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/chores", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleChores_CreateWithPresetAndRedirect(t *testing.T) {
	server, mock := newChoresTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("title", "Nightly backup")
	form.Set("description", "run the backup script")
	form.Set("preset", "daily")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/admin/chores/") {
		t.Fatalf("expected redirect to a chore detail page, got %q", loc)
	}
	if len(mock.chores) != 1 {
		t.Fatalf("expected 1 chore created, got %d", len(mock.chores))
	}
	for _, c := range mock.chores {
		if !c.ScheduleConfirmed {
			t.Fatal("expected the web UI's own create form to always set userConfirmed=true")
		}
		if c.Schedule.CronExpression != "0 0 * * *" {
			t.Fatalf("expected daily preset cron expression, got %q", c.Schedule.CronExpression)
		}
	}
}

func TestHandleChores_CreateWithRawCron(t *testing.T) {
	server, mock := newChoresTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("title", "Custom job")
	form.Set("description", "custom schedule")
	form.Set("preset", "")
	form.Set("cron_expression", "*/15 * * * *")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	for _, c := range mock.chores {
		if c.Schedule.CronExpression != "*/15 * * * *" {
			t.Fatalf("expected raw cron expression, got %q", c.Schedule.CronExpression)
		}
	}
}

func TestHandleChores_CreateEmptyRawCronRejected(t *testing.T) {
	server, mock := newChoresTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("title", "Custom job")
	form.Set("preset", "")
	form.Set("cron_expression", "")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty raw cron expression, got %d", w.Code)
	}
	if len(mock.chores) != 0 {
		t.Fatal("expected no chore to be created")
	}
}

func TestHandleChores_CreateInvalidCSRFRejected(t *testing.T) {
	server, mock := newChoresTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	form := url.Values{}
	form.Set("title", "Nightly backup")
	form.Set("preset", "daily")
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/chores", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if len(mock.chores) != 0 {
		t.Fatal("expected no chore to be created with an invalid CSRF token")
	}
}

func TestHandleChoreDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "bob", Title: "Bob's secret"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chores/chore-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access (not 403 — existence must not be disclosed), got %d", w.Code)
	}
}

func TestHandleChoreDetail_OwnerSuccess(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice", Title: "Nightly backup"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chores/chore-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Nightly backup") {
		t.Fatal("expected chore title to appear in rendered page")
	}
}

func TestHandleChoreDetail_UnconfirmedShowsPendingConfirmation(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice", Title: "Agent proposal", ScheduleConfirmed: false}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chores/chore-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Pending confirmation") {
		t.Fatal("expected 'Pending confirmation' to appear for an unconfirmed schedule")
	}
	if !strings.Contains(w.Body.String(), "Confirm Schedule") {
		t.Fatal("expected a Confirm Schedule control for an unconfirmed schedule")
	}
}

func TestHandleChoreDelete_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "bob"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores/chore-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if _, ok := mock.chores["chore-1"]; !ok {
		t.Fatal("expected bob's chore to remain undeleted")
	}
}

func TestHandleChoreDelete_OwnerSuccess(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores/chore-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if _, ok := mock.chores["chore-1"]; ok {
		t.Fatal("expected chore to be deleted")
	}
}

func TestHandleChoreConfirm_CrossOwnerReturns404(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "bob", ScheduleConfirmed: false}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores/chore-1/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if mock.chores["chore-1"].ScheduleConfirmed {
		t.Fatal("expected bob's chore to remain unconfirmed")
	}
}

func TestHandleChoreConfirm_OwnerSuccess(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice", ScheduleConfirmed: false}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/chores/chore-1/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if !mock.chores["chore-1"].ScheduleConfirmed {
		t.Fatal("expected chore to become confirmed")
	}
}

func TestHandleChoreConfirm_InvalidCSRFRejected(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice", ScheduleConfirmed: false}

	cookie := sessionCookieFor(server, "alice", "user")
	form := url.Values{}
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/chores/chore-1/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if mock.chores["chore-1"].ScheduleConfirmed {
		t.Fatal("expected chore to remain unconfirmed")
	}
}

func TestHandleChoreSubroutes_UnknownActionNotFound(t *testing.T) {
	server, mock := newChoresTestServer(t)
	mock.chores["chore-1"] = &domain.Chore{ID: "chore-1", OwnerUserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/chores/chore-1/bogus", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subroute action, got %d", w.Code)
	}
}

func TestHandleChoreSubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newChoresTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chores/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty chore ID, got %d", w.Code)
	}
}

func TestChoresService_NotConfigured(t *testing.T) {
	server := NewServer(":0") // no SetChoresService call
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/chores", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when ChoresService is unconfigured, got %d", w.Code)
	}
}

func TestChoreIDAndActionFromPath(t *testing.T) {
	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/admin/chores/abc", "abc", ""},
		{"/admin/chores/abc/confirm", "abc", "confirm"},
		{"/admin/chores/abc/delete", "abc", "delete"},
		{"/admin/chores/", "", ""},
		{"/admin/chores/abc/too/many", "", ""},
	}
	for _, tc := range cases {
		id, action := choreIDAndActionFromPath(tc.path)
		if id != tc.wantID || action != tc.wantAction {
			t.Errorf("path %q: expected (%q, %q), got (%q, %q)", tc.path, tc.wantID, tc.wantAction, id, action)
		}
	}
}

// Ensure MockChoresService satisfies the interface at compile time.
var _ ChoresService = (*MockChoresService)(nil)
