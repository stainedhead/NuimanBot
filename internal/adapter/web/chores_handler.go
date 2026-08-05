package web

import (
	"context"
	"net/http"

	"nuimanbot/internal/domain"
)

// ChoresService is the interface the web admin's Chores environment
// (FR-031–FR-038) depends on. Production wiring composes
// internal/usecase/chores.Service. See JobsService's doc comment re: not
// importing internal/infrastructure/scheduler from this adapter/web file.
//
// STATUS: scaffold only — flesh out following chats_handler.go's exact
// pattern (ownerUserID = session Username, cross-owner -> 404, CSRF via
// s.validCSRF).
type ChoresService interface {
	// CreateChore creates a Chore (FR-031) with the given schedule
	// (FR-032/FR-034 — resolved from either a preset or a raw cron
	// expression before this call, see domain.NewScheduleFromPreset/
	// NewScheduleFromCron). userConfirmed distinguishes a user-set schedule
	// (fires immediately) from an agent-proposed one pending confirmation
	// (FR-033) — the resulting Chore's ScheduleConfirmed is set from this.
	CreateChore(ctx context.Context, ownerUserID, title, description, workingDirectory string, schedule domain.Schedule, userConfirmed bool) (*domain.Chore, error)
	// ListChores returns ownerUserID's Chores.
	ListChores(ctx context.Context, ownerUserID string) ([]*domain.Chore, error)
	// GetChore retrieves a Chore by ID, scoped to its owner.
	GetChore(ctx context.Context, ownerUserID, choreID string) (*domain.Chore, error)
	// DeleteChore soft-deletes a Chore (spec.md Edge Case #3), mirroring
	// JobsService.DeleteJob's in-flight-run handling.
	DeleteChore(ctx context.Context, ownerUserID, choreID string) error
	// ConfirmSchedule confirms an agent-proposed schedule (FR-033),
	// allowing the Chore to begin firing. Discarding a pending schedule is
	// simply DeleteChore.
	ConfirmSchedule(ctx context.Context, ownerUserID, choreID string) error
}

// SetChoresService sets the Chores environment's service.
func (s *Server) SetChoresService(svc ChoresService) {
	s.choresService = svc
}

// handleChores lists/creates Chores (GET/POST /admin/chores). The create
// form must offer both common presets and an advanced raw cron field
// (FR-034); validate raw cron via internal/infrastructure/scheduler.
// ValidateCronExpression at the usecase layer, not here.
// PLACEHOLDER: replace with the full chats_handler.go-style implementation.
func (s *Server) handleChores(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.choresService == nil {
		http.Error(w, "Chores service not configured", http.StatusInternalServerError)
		return
	}
	http.Error(w, "Chores environment not yet implemented", http.StatusNotImplemented)
}

// handleChoreSubroutes dispatches /admin/chores/{id}[/action] — detail,
// delete, /confirm (FR-033), and a per-Chore chat interface (FR-037).
// PLACEHOLDER: replace with the full implementation.
func (s *Server) handleChoreSubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.choresService == nil {
		http.Error(w, "Chores service not configured", http.StatusInternalServerError)
		return
	}
	http.Error(w, "Chores environment not yet implemented", http.StatusNotImplemented)
}
