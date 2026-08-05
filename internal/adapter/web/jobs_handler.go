package web

import (
	"context"
	"net/http"

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
//
// STATUS: scaffold only — flesh out following chats_handler.go's exact
// pattern (ownerUserID = session Username, cross-owner -> 404, CSRF via
// s.validCSRF).
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

// handleJobs lists/creates Jobs (GET/POST /admin/jobs).
// PLACEHOLDER: replace with the full chats_handler.go-style implementation.
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
	http.Error(w, "Jobs environment not yet implemented", http.StatusNotImplemented)
}

// handleJobSubroutes dispatches /admin/jobs/{id}[/action] — detail, delete,
// and a per-Job chat interface (FR-029; may reuse the ChatsService pattern
// with a Job-scoped Chat, or a dedicated thread — decide and document in
// implementation-notes.md).
// PLACEHOLDER: replace with the full implementation.
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
	http.Error(w, "Jobs environment not yet implemented", http.StatusNotImplemented)
}
