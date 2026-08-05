package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"nuimanbot/internal/domain"
)

// ProjectsService is the interface the web admin's Projects environment
// (FR-017–FR-023) depends on. Production wiring composes
// internal/usecase/projects.Service. ownerUserID is the current session's
// Username (see ChatsService's doc comment for why — session.ID is a
// per-session token, not a stable user identifier).
type ProjectsService interface {
	// CreateProject creates a Project with the given output directory
	// (FR-017). The service is responsible for creating OutputDirectory
	// and a HiddenDirectory (FR-018) on disk.
	CreateProject(ctx context.Context, ownerUserID, name, outputDirectory string) (*domain.Project, error)
	// ListProjects returns ownerUserID's Projects.
	ListProjects(ctx context.Context, ownerUserID string) ([]*domain.Project, error)
	// GetProject retrieves a Project by ID, scoped to its owner.
	GetProject(ctx context.Context, ownerUserID, projectID string) (*domain.Project, error)
	// DeleteProject deletes a Project by ID, scoped to its owner. Per
	// spec.md Edge Case #2, deleting a Project must NOT delete any Job/
	// Chore that references it — those simply fail their next run with a
	// clear Error once the Project is gone.
	DeleteProject(ctx context.Context, ownerUserID, projectID string) error
	// AddAgentsFile creates an empty/starter AGENTS.md in the Project's
	// OutputDirectory if one doesn't already exist (FR-021's "subdued,
	// secondary control"). Must use internal/infrastructure/fsguard to
	// resolve the path — never join OutputDirectory + "AGENTS.md" directly.
	AddAgentsFile(ctx context.Context, ownerUserID, projectID string) error
}

// SetProjectsService sets the Projects environment's service.
func (s *Server) SetProjectsService(svc ProjectsService) {
	s.projectsService = svc
}

// ProjectsPageData is the template data for the Projects list/create page.
type ProjectsPageData struct {
	*BaseData
	Projects  []*domain.Project
	CSRFToken string
}

// ProjectDetailPageData is the template data for a single Project's detail page.
type ProjectDetailPageData struct {
	*BaseData
	Project          *domain.Project
	AgentsFileExists bool
	CSRFToken        string
}

// handleProjects lists the current user's Projects (GET) and creates a new
// one (POST).
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.projectsService == nil {
		http.Error(w, "Projects service not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		s.handleProjectCreate(w, r, user)
		return
	}

	projectsList, err := s.projectsService.ListProjects(r.Context(), user.Username)
	if err != nil {
		slog.Error("Failed to list projects", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &ProjectsPageData{
		BaseData:  s.baseDataFor(user, "Projects", "projects"),
		Projects:  projectsList,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "projects.html", data); err != nil {
		slog.Error("Failed to render projects template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleProjectCreate creates a new Project from a form-posted name and
// output directory, and redirects to its detail page (FR-017).
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request, user *User) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	name := sanitizedFormValue(r, "name")
	outputDirectory := sanitizedFormValue(r, "output_directory")
	project, err := s.projectsService.CreateProject(r.Context(), user.Username, name, outputDirectory)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("Failed to create project", "error", err)
		s.Error500(w, r, err)
		return
	}

	http.Redirect(w, r, "/admin/projects/"+project.ID, http.StatusFound)
}

// handleProjectSubroutes dispatches /admin/projects/{id}[/delete|/add-agents-file].
func (s *Server) handleProjectSubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.projectsService == nil {
		http.Error(w, "Projects service not configured", http.StatusInternalServerError)
		return
	}

	id, action := projectIDAndActionFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleProjectDetail(w, r, user, id)
	case "delete":
		s.handleProjectDelete(w, r, user, id)
	case "add-agents-file":
		s.handleProjectAddAgentsFile(w, r, user, id)
	default:
		http.NotFound(w, r)
	}
}

// handleProjectDetail renders a single Project's detail page (FR-019/
// FR-021/FR-022). A project that doesn't exist or belongs to a different
// user renders 404 — never a distinct "forbidden" response — per spec.md
// Edge Case #10.
func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	project, err := s.projectsService.GetProject(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get project", "error", err)
		s.Error500(w, r, err)
		return
	}

	agentsFileExists := false
	if _, statErr := os.Stat(project.AgentsFilePath()); statErr == nil {
		agentsFileExists = true
	}

	data := &ProjectDetailPageData{
		BaseData:         s.baseDataFor(user, project.Name, "projects"),
		Project:          project,
		AgentsFileExists: agentsFileExists,
		CSRFToken:        s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "project_detail.html", data); err != nil {
		slog.Error("Failed to render project detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleProjectDelete deletes a Project (FR-023's manual-delete counterpart).
func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.projectsService.DeleteProject(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to delete project", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/projects", http.StatusFound)
}

// handleProjectAddAgentsFile adds a starter AGENTS.md to a Project that
// doesn't yet have one (FR-021's subdued, secondary control).
func (s *Server) handleProjectAddAgentsFile(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.projectsService.AddAgentsFile(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to add AGENTS.md", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/projects/"+id, http.StatusFound)
}

// projectIDAndActionFromPath parses "/admin/projects/{id}" or
// "/admin/projects/{id}/{action}" into (id, action). action is "" for the
// bare detail path.
func projectIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "projects", "{id}"] or ["admin", "projects", "{id}", "{action}"]
	if len(parts) < 3 || parts[2] == "" {
		return "", ""
	}
	if len(parts) == 3 {
		return parts[2], ""
	}
	if len(parts) == 4 {
		return parts[2], parts[3]
	}
	return "", ""
}
