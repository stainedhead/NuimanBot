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

// MockProjectsService is a test double for ProjectsService.
type MockProjectsService struct {
	projects map[string]*domain.Project // projectID -> project

	createErr         error
	agentsFileWritten map[string]bool
}

func NewMockProjectsService() *MockProjectsService {
	return &MockProjectsService{
		projects:          make(map[string]*domain.Project),
		agentsFileWritten: make(map[string]bool),
	}
}

func (m *MockProjectsService) CreateProject(_ context.Context, ownerUserID, name, outputDirectory string) (*domain.Project, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	now := time.Now()
	p := &domain.Project{
		ID:              "project-" + ownerUserID + "-" + name,
		OwnerUserID:     ownerUserID,
		Name:            name,
		OutputDirectory: outputDirectory,
		HiddenDirectory: outputDirectory + "/.nuimanbot",
		Retention:       domain.NeverExpire(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	m.projects[p.ID] = p
	return p, nil
}

func (m *MockProjectsService) ListProjects(_ context.Context, ownerUserID string) ([]*domain.Project, error) {
	out := make([]*domain.Project, 0)
	for _, p := range m.projects {
		if p.OwnerUserID == ownerUserID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MockProjectsService) GetProject(_ context.Context, ownerUserID, projectID string) (*domain.Project, error) {
	p, ok := m.projects[projectID]
	if !ok || p.OwnerUserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return p, nil
}

func (m *MockProjectsService) DeleteProject(_ context.Context, ownerUserID, projectID string) error {
	p, ok := m.projects[projectID]
	if !ok || p.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	delete(m.projects, projectID)
	return nil
}

func (m *MockProjectsService) AddAgentsFile(_ context.Context, ownerUserID, projectID string) error {
	p, ok := m.projects[projectID]
	if !ok || p.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	m.agentsFileWritten[p.ID] = true
	return nil
}

// newProjectsTestServer builds a server wired with a mock ProjectsService.
func newProjectsTestServer(t *testing.T) (*Server, *MockProjectsService) {
	t.Helper()
	server := NewServer(":0")
	mock := NewMockProjectsService()
	server.SetProjectsService(mock)
	return server, mock
}

func TestHandleProjects_RequiresAuth(t *testing.T) {
	server, _ := newProjectsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestHandleProjects_ListEmpty(t *testing.T) {
	server, _ := newProjectsTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	req.AddCookie(sessionCookieFor(server, "alice", "user"))
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleProjects_CreateAndRedirect(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()

	form := url.Values{}
	form.Set("name", "My Project")
	form.Set("output_directory", "/tmp/my-project")
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d, body: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/admin/projects/") {
		t.Fatalf("expected redirect to a project detail page, got %q", loc)
	}
	if len(mock.projects) != 1 {
		t.Fatalf("expected 1 project created, got %d", len(mock.projects))
	}
}

func TestHandleProjects_CreateInvalidCSRFRejected(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")

	form := url.Values{}
	form.Set("name", "My Project")
	form.Set("output_directory", "/tmp/my-project")
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/projects", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if len(mock.projects) != 0 {
		t.Fatal("expected no project to be created with an invalid CSRF token")
	}
}

func TestHandleProjectDetail_CrossOwnerReturns404(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "bob", Name: "Bob's secret", OutputDirectory: t.TempDir()}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/projects/project-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-owner access (not 403 — existence must not be disclosed), got %d", w.Code)
	}
}

func TestHandleProjectDetail_OwnerSuccess(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice", Name: "Trip planning project", OutputDirectory: t.TempDir()}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/projects/project-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Trip planning project") {
		t.Fatal("expected project name to appear in rendered page")
	}
}

func TestHandleProjectDetail_ShowsAgentsFileAbsentByDefault(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	outDir := t.TempDir()
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice", Name: "No AGENTS.md yet", OutputDirectory: outDir}

	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/projects/project-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "add-agents-file") {
		t.Fatal("expected the Add AGENTS.md control to be present when the file is absent")
	}
}

func TestHandleProjectDelete_CrossOwnerReturns404(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "bob"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if _, ok := mock.projects["project-1"]; !ok {
		t.Fatal("expected bob's project to remain undeleted")
	}
}

func TestHandleProjectDelete_OwnerSuccess(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if _, ok := mock.projects["project-1"]; ok {
		t.Fatal("expected project to be deleted")
	}
}

func TestHandleProjectDelete_InvalidCSRFRejected(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	form := url.Values{}
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if _, ok := mock.projects["project-1"]; !ok {
		t.Fatal("expected project to remain undeleted with an invalid CSRF token")
	}
}

func TestHandleProjectAddAgentsFile_CrossOwnerReturns404(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "bob"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/add-agents-file", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if mock.agentsFileWritten["project-1"] {
		t.Fatal("expected AGENTS.md not to be added for cross-owner request")
	}
}

func TestHandleProjectAddAgentsFile_OwnerSuccess(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	csrfToken := server.auth.GenerateCSRFToken()
	form := url.Values{}
	form.Set("csrf_token", csrfToken)

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/add-agents-file", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	if !mock.agentsFileWritten["project-1"] {
		t.Fatal("expected AGENTS.md to have been added")
	}
}

func TestHandleProjectAddAgentsFile_InvalidCSRFRejected(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice"}

	cookie := sessionCookieFor(server, "alice", "user")
	form := url.Values{}
	form.Set("csrf_token", "bogus-token")

	req := httptest.NewRequest(http.MethodPost, "/admin/projects/project-1/add-agents-file", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid CSRF token, got %d", w.Code)
	}
	if mock.agentsFileWritten["project-1"] {
		t.Fatal("expected AGENTS.md not to be added with an invalid CSRF token")
	}
}

func TestHandleProjectSubroutes_UnknownActionNotFound(t *testing.T) {
	server, mock := newProjectsTestServer(t)
	mock.projects["project-1"] = &domain.Project{ID: "project-1", OwnerUserID: "alice"}
	cookie := sessionCookieFor(server, "alice", "user")

	req := httptest.NewRequest(http.MethodGet, "/admin/projects/project-1/bogus", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown subroute action, got %d", w.Code)
	}
}

func TestHandleProjectSubroutes_EmptyIDNotFound(t *testing.T) {
	server, _ := newProjectsTestServer(t)
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/projects/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for empty project ID, got %d", w.Code)
	}
}

func TestProjectsService_NotConfigured(t *testing.T) {
	server := NewServer(":0") // no SetProjectsService call
	cookie := sessionCookieFor(server, "alice", "user")
	req := httptest.NewRequest(http.MethodGet, "/admin/projects", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when ProjectsService is unconfigured, got %d", w.Code)
	}
}

func TestProjectIDAndActionFromPath(t *testing.T) {
	cases := []struct {
		path       string
		wantID     string
		wantAction string
	}{
		{"/admin/projects/abc", "abc", ""},
		{"/admin/projects/abc/delete", "abc", "delete"},
		{"/admin/projects/abc/add-agents-file", "abc", "add-agents-file"},
		{"/admin/projects/", "", ""},
		{"/admin/projects/abc/too/many", "", ""},
	}
	for _, tc := range cases {
		id, action := projectIDAndActionFromPath(tc.path)
		if id != tc.wantID || action != tc.wantAction {
			t.Errorf("path %q: expected (%q, %q), got (%q, %q)", tc.path, tc.wantID, tc.wantAction, id, action)
		}
	}
}

// Ensure MockProjectsService satisfies the interface at compile time.
var _ ProjectsService = (*MockProjectsService)(nil)
