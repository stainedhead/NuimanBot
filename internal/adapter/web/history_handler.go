package web

import (
	"context"
	"net/http"

	"nuimanbot/internal/domain"
)

// HistoryService is the interface the web admin's History environment
// (FR-040–FR-044) depends on. Production wiring composes
// internal/usecase/history.Service.
//
// STATUS: scaffold only — flesh out following chats_handler.go's exact
// pattern (ownerUserID = session Username, cross-owner -> 404).
type HistoryService interface {
	// ListRuns returns ownerUserID's Job/Chore runs matching filter
	// (FR-040/FR-041 — by source, date range, status), most recent first.
	ListRuns(ctx context.Context, ownerUserID string, filter domain.RunFilter) ([]*domain.Run, error)
	// GetRun retrieves a single Run by ID, scoped to its owner — the
	// grounding context for that run's chat interface (FR-042).
	GetRun(ctx context.Context, ownerUserID, runID string) (*domain.Run, error)
	// MarkViewed clears the notification badge for a single run (FR-044).
	MarkViewed(ctx context.Context, ownerUserID, runID string) error
	// UnviewedCount returns the current notification badge count (FR-044),
	// used to populate BaseData.UnviewedRunCount on every authenticated
	// page, not just /admin/history itself.
	UnviewedCount(ctx context.Context, ownerUserID string) (int, error)
}

// SetHistoryService sets the History environment's service.
func (s *Server) SetHistoryService(svc HistoryService) {
	s.historyService = svc
}

// handleHistory lists/filters the current user's Job/Chore runs
// (GET /admin/history).
// PLACEHOLDER: replace with the full chats_handler.go-style implementation.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.historyService == nil {
		http.Error(w, "History service not configured", http.StatusInternalServerError)
		return
	}
	http.Error(w, "History environment not yet implemented", http.StatusNotImplemented)
}

// handleHistorySubroutes dispatches /admin/history/{runID}[/action] —
// per-run detail (log/results, FR-040) and its chat interface (FR-042).
// PLACEHOLDER: replace with the full implementation.
func (s *Server) handleHistorySubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.historyService == nil {
		http.Error(w, "History service not configured", http.StatusInternalServerError)
		return
	}
	http.Error(w, "History environment not yet implemented", http.StatusNotImplemented)
}
