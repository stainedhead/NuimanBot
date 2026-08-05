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

// MockJobsService is a test double for JobsService.
type MockJobsService struct {
	jobs map[string]*domain.Job // jobID -> job

	createErr error
	getErr    error
}

func NewMockJobsService() *MockJobsService {
	return &MockJobsService{jobs: make(map[string]*domain.Job)}
}

func (m *MockJobsService) CreateJob(_ context.Context, ownerUserID, title, description string, contextType domain.ContextType, contextID string) (*domain.Job, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if strings.TrimSpace(title) == "" {
		return nil, domain.ErrInvalidInput
	}
	now := time.Now()
	job := &domain.Job{
		ID:          "job-" + ownerUserID + "-" + title,
		OwnerUserID: ownerUserID,
		Title:       title,
		Description: description,
		ContextType: contextType,
		ContextID:   contextID,
		Status:      domain.JobStatusQueued,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.jobs[job.ID] = job
	return job, nil
}

func (m *MockJobsService) ListJobs(_ context.Context, ownerUserID string) ([]*domain.Job, error) {
	out := make([]*domain.Job, 0)
	for _, j := range m.jobs {
		if j.OwnerUserID == ownerUserID {
			out = append(out, j)
		}
	}
	return out, nil
}

func (m *MockJobsService) GetJob(_ context.Context, ownerUserID, jobID string) (*domain.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	j, ok := m.jobs[jobID]
	if !ok || j.OwnerUserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return j, nil
}

func (m *MockJobsService) DeleteJob(_ context.Context, ownerUserID, jobID string) error {
	j, ok := m.jobs[jobID]
	if !ok || j.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	delete(m.jobs, jobID)
	return nil
}

// newJobsTestServer builds a server wired to a mock JobsService.
func newJobsTestServer(t *testing.T) (*Server, *MockJobsService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockJobsService()
	server.SetJobsService(mock)
	return server, mock
}

func TestHandleJobs_RequiresAuth(t *testing.T) {
	server, _ := newJobsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleJobs_ListEmpty(t *testing.T) {
	server, _ := newJobsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleJobs_CreateAndRedirect(t *testing.T) {
	server, mock := newJobsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("title", "Clean the inbox")
	form.Set("description", "Archive anything older than 30 days.")
	form.Set("context", "none")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/admin/jobs/") {
		t.Fatalf("expected redirect to a job detail page, got %q", loc)
	}
	if len(mock.jobs) != 1 {
		t.Fatalf("expected 1 job created, got %d", len(mock.jobs))
	}
}

func TestHandleJobs_CreateWithProjectContext(t *testing.T) {
	server, mock := newJobsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("title", "Refactor widgets")
	form.Set("description", "Refactor the widget module.")
	form.Set("context", "project")
	form.Set("project_id", "proj-1")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	var created *domain.Job
	for _, j := range mock.jobs {
		created = j
	}
	if created == nil || created.ContextType != domain.ContextTypeProject || created.ContextID != "proj-1" {
		t.Fatalf("expected job with project context, got %+v", created)
	}
}

func TestHandleJobs_CreateInvalidCSRFRejected(t *testing.T) {
	server, mock := newJobsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	form := url.Values{}
	form.Set("title", "Clean the inbox")
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if len(mock.jobs) != 0 {
		t.Fatal("expected no job to be created with an invalid CSRF token")
	}
}

func TestHandleJobDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "bob", Title: "Bob's secret job"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/job-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access (not 403 — existence must not be disclosed), got %d", w.Code)
	}
}

func TestHandleJobDetail_OwnerSuccess(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "alice", Title: "Weekly report"}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/job-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Weekly report") {
		t.Fatal("expected job title to appear in rendered page")
	}
}

func TestHandleJobDelete_CrossOwnerReturns404(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "bob"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs/job-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if _, ok := mock.jobs["job-1"]; !ok {
		t.Fatal("expected bob's job to remain undeleted")
	}
}

func TestHandleJobDelete_OwnerSuccess(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs/job-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if _, ok := mock.jobs["job-1"]; ok {
		t.Fatal("expected job to be deleted")
	}
}

func TestHandleJobDeleteInvalidCSRFRejected(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	form := url.Values{}
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/jobs/job-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if _, ok := mock.jobs["job-1"]; !ok {
		t.Fatal("expected job to remain undeleted with an invalid CSRF token")
	}
}

func TestHandleJobSubroutes_UnknownActionNotFound(t *testing.T) {
	server, mock := newJobsTestServer(t)
	mock.jobs["job-1"] = &domain.Job{ID: "job-1", OwnerUserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/job-1/bogus", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subroute action, got %d", w.Code)
	}
}

func TestHandleJobSubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newJobsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty job ID, got %d", w.Code)
	}
}

func TestJobsService_NotConfigured(t *testing.T) {
	server := NewServer(":0") // no SetJobsService call
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when JobsService is unconfigured, got %d", w.Code)
	}
}

func TestJobIDAndActionFromPath(t *testing.T) {
	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/admin/jobs/abc", "abc", ""},
		{"/admin/jobs/abc/delete", "abc", "delete"},
		{"/admin/jobs/", "", ""},
		{"/admin/jobs/abc/too/many", "", ""},
	}
	for _, tc := range cases {
		id, action := jobIDAndActionFromPath(tc.path)
		if id != tc.wantID || action != tc.wantAction {
			t.Errorf("path %q: expected (%q, %q), got (%q, %q)", tc.path, tc.wantID, tc.wantAction, id, action)
		}
	}
}

// Ensure MockJobsService satisfies the interface at compile time.
var _ JobsService = (*MockJobsService)(nil)
