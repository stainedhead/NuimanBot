package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
)

// JobsService is the interface the web admin's Jobs environment
// (FR-024–FR-030) depends on. Production wiring composes
// internal/usecase/jobs.Service, which in turn enqueues onto the shared
// worker pool (internal/infrastructure/scheduler.WorkerPool) via a small
// Enqueuer-shaped interface of its own (do not import the scheduler
// package from this adapter/web file — usecase orchestrates infrastructure,
// per AGENTS.md's Clean Architecture layering; the web layer only talks to
// the usecase Service).
type JobsService interface {
	// CreateJob creates a Job (FR-024), persists Description as
	// JOB-DESCRIPTION.md in its HiddenDirectory (FR-025), and enqueues it
	// onto the worker pool in FIFO order (FR-027).
	CreateJob(ctx context.Context, ownerUserID, title, description string, contextType domain.ContextType, contextID string) (*domain.Job, error)
	// ListJobs returns ownerUserID's Jobs.
	ListJobs(ctx context.Context, ownerUserID string) ([]*domain.Job, error)
	// GetJob retrieves a Job by ID, scoped to its owner.
	GetJob(ctx context.Context, ownerUserID, jobID string) (*domain.Job, error)
	// DeleteJob soft-deletes a Job (spec.md Edge Case #3): if a run is
	// in-flight, the Job is marked PendingDeletion and removed once that
	// run reaches a terminal state, not immediately.
	DeleteJob(ctx context.Context, ownerUserID, jobID string) error
}

// SetJobsService sets the Jobs environment's service.
func (s *Server) SetJobsService(svc JobsService) {
	s.jobsService = svc
}

// JobsPageData is the template data for the Jobs list/create page.
type JobsPageData struct {
	*BaseData
	Jobs      []*domain.Job
	CSRFToken string
}

// JobDetailPageData is the template data for a single Job's detail page.
type JobDetailPageData struct {
	*BaseData
	Job       *domain.Job
	CSRFToken string
}

// handleJobs lists the current user's Jobs (GET) and creates a new one (POST).
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.jobsService == nil {
		http.Error(w, "Jobs service not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		s.handleJobCreate(w, r, user)
		return
	}

	jobsList, err := s.jobsService.ListJobs(r.Context(), user.Username)
	if err != nil {
		slog.Error("Failed to list jobs", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &JobsPageData{
		BaseData:  s.baseDataFor(r.Context(), user, "Jobs", "jobs"),
		Jobs:      jobsList,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "jobs.html", data); err != nil {
		slog.Error("Failed to render jobs template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleJobCreate creates a new Job from form fields (FR-024): Title,
// Description, and a context selector — "context" is "project" or anything
// else (treated as no context/ContextTypeChat), with "project_id" supplying
// ContextID when "context" is "project" (FR-026).
func (s *Server) handleJobCreate(w http.ResponseWriter, r *http.Request, user *User) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	title := sanitizedFormValue(r, "title")
	description := sanitizedFormValue(r, "description")
	contextType := domain.ContextTypeChat
	contextID := ""
	if sanitizedFormValue(r, "context") == "project" {
		contextType = domain.ContextTypeProject
		contextID = sanitizedFormValue(r, "project_id")
	}

	job, err := s.jobsService.CreateJob(r.Context(), user.Username, title, description, contextType, contextID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("Failed to create job", "error", err)
		s.Error500(w, r, err)
		return
	}

	http.Redirect(w, r, "/admin/jobs/"+job.ID, http.StatusFound)
}

// handleJobSubroutes dispatches /admin/jobs/{id}[/delete].
func (s *Server) handleJobSubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.jobsService == nil {
		http.Error(w, "Jobs service not configured", http.StatusInternalServerError)
		return
	}

	id, action := jobIDAndActionFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleJobDetail(w, r, user, id)
	case "delete":
		s.handleJobDelete(w, r, user, id)
	default:
		http.NotFound(w, r)
	}
}

// handleJobDetail renders a single Job (FR-024–FR-030's detail surface). A
// Job that doesn't exist or belongs to a different user renders 404 — never
// a distinct "forbidden" response — per spec.md Edge Case #10. The Job's
// own chat interface (FR-029) is out of scope for this pass; the template
// notes it is not yet available.
func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	job, err := s.jobsService.GetJob(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get job", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &JobDetailPageData{
		BaseData:  s.baseDataFor(r.Context(), user, job.Title, "jobs"),
		Job:       job,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "job_detail.html", data); err != nil {
		slog.Error("Failed to render job detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleJobDelete deletes a Job the caller owns (FR-024–FR-030; soft-delete
// semantics for an in-flight run are implemented at the usecase layer, see
// jobs.Service.DeleteJob).
func (s *Server) handleJobDelete(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.jobsService.DeleteJob(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to delete job", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/jobs", http.StatusFound)
}

// jobIDAndActionFromPath parses "/admin/jobs/{id}" or
// "/admin/jobs/{id}/{action}" into (id, action). action is "" for the bare
// detail path.
func jobIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "jobs", "{id}"] or ["admin", "jobs", "{id}", "{action}"]
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
