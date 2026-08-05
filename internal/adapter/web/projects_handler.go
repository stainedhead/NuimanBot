package web

import (
	"context"
	"net/http"

	"nuimanbot/internal/domain"
)

// ProjectsService is the interface the web admin's Projects environment
// (FR-017–FR-023) depends on. Production wiring composes
// internal/usecase/projects.Service. ownerUserID is the current session's
// Username (see ChatsService's doc comment for why — session.ID is a
// per-session token, not a stable user identifier).
//
// STATUS: scaffold only — CreateProject/ListProjects/GetProject/
// DeleteProject/AddAgentsFile are the interface this environment's usecase
// Service must satisfy (see data-dictionary.md's Project entity and
// FR-017/018/019/021/023). handleProjects/handleProjectSubroutes below are
// placeholders; flesh them out following chats_handler.go's exact pattern
// (list+create on the bare path, {id}/{id}/action on the trailing-slash
// path, CSRF via s.validCSRF, cross-owner access -> domain.ErrNotFound ->
// http.NotFound, never 403).
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

// handleProjects lists/creates Projects (GET/POST /admin/projects).
// PLACEHOLDER: replace with the full chats_handler.go-style implementation.
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
	http.Error(w, "Projects environment not yet implemented", http.StatusNotImplemented)
}

// handleProjectSubroutes dispatches /admin/projects/{id}[/action].
// PLACEHOLDER: replace with the full chats_handler.go-style implementation
// (detail, delete, add-agents-file per FR-021).
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
	http.Error(w, "Projects environment not yet implemented", http.StatusNotImplemented)
}
