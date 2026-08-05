package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
)

// ChoresService is the interface the web admin's Chores environment
// (FR-031–FR-038) depends on. Production wiring composes
// internal/usecase/chores.Service. See JobsService's doc comment re: not
// importing internal/infrastructure/scheduler from this adapter/web file.
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

// ChoresPageData is the template data for the Chores list/create page.
type ChoresPageData struct {
	*BaseData
	Chores    []*domain.Chore
	Presets   []domain.SchedulePreset
	CSRFToken string
}

// ChoreDetailPageData is the template data for a single Chore's detail page.
type ChoreDetailPageData struct {
	*BaseData
	Chore     *domain.Chore
	CSRFToken string
}

// handleChores lists the current user's Chores (GET) and creates a new one
// (POST /admin/chores).
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

	if r.Method == http.MethodPost {
		s.handleChoreCreate(w, r, user)
		return
	}

	choresList, err := s.choresService.ListChores(r.Context(), user.Username)
	if err != nil {
		slog.Error("Failed to list chores", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &ChoresPageData{
		BaseData:  s.baseDataFor(user, "Chores", "chores"),
		Chores:    choresList,
		Presets:   domain.KnownPresets(),
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "chores.html", data); err != nil {
		slog.Error("Failed to render chores template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleChoreCreate creates a new Chore from the form-posted Title,
// Description, optional WorkingDirectory, and schedule (a preset dropdown
// selection or, when "custom" is selected, the advanced raw cron
// expression field — FR-034). The web UI's own create form always sets a
// user-set schedule (userConfirmed=true, FR-033) — an agent-proposed,
// pending-confirmation schedule is only ever created via the (not-yet-built)
// chat interface, not this form.
func (s *Server) handleChoreCreate(w http.ResponseWriter, r *http.Request, user *User) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	title := sanitizedFormValue(r, "title")
	description := sanitizedFormValue(r, "description")
	workingDirectory := sanitizedFormValue(r, "working_directory")
	presetValue := sanitizedFormValue(r, "preset")
	rawCron := sanitizedFormValue(r, "cron_expression")

	schedule, err := resolveScheduleFromForm(presetValue, rawCron)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chore, err := s.choresService.CreateChore(r.Context(), user.Username, title, description, workingDirectory, schedule, true)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("Failed to create chore", "error", err)
		s.Error500(w, r, err)
		return
	}

	http.Redirect(w, r, "/admin/chores/"+chore.ID, http.StatusFound)
}

// resolveScheduleFromForm resolves a Chore create form's schedule fields
// into a domain.Schedule: a non-empty preset takes precedence; otherwise
// rawCron (the advanced raw cron expression field, FR-034) is used.
func resolveScheduleFromForm(presetValue, rawCron string) (domain.Schedule, error) {
	if presetValue != "" {
		schedule, err := domain.NewScheduleFromPreset(domain.SchedulePreset(presetValue))
		if err != nil {
			return domain.Schedule{}, err
		}
		return schedule, nil
	}
	schedule, err := domain.NewScheduleFromCron(rawCron)
	if err != nil {
		return domain.Schedule{}, err
	}
	return schedule, nil
}

// handleChoreSubroutes dispatches /admin/chores/{id}[/delete|/confirm].
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

	id, action := choreIDAndActionFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleChoreDetail(w, r, user, id)
	case "delete":
		s.handleChoreDelete(w, r, user, id)
	case "confirm":
		s.handleChoreConfirm(w, r, user, id)
	default:
		http.NotFound(w, r)
	}
}

// handleChoreDetail renders a single Chore's detail page (FR-031–FR-034).
// A chore that doesn't exist or belongs to a different user renders 404 —
// never a distinct "forbidden" response — per spec.md Edge Case #10.
func (s *Server) handleChoreDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	chore, err := s.choresService.GetChore(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get chore", "error", err)
		s.Error500(w, r, err)
		return
	}

	data := &ChoreDetailPageData{
		BaseData:  s.baseDataFor(user, chore.Title, "chores"),
		Chore:     chore,
		CSRFToken: s.auth.GenerateCSRFToken(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "chore_detail.html", data); err != nil {
		slog.Error("Failed to render chore detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleChoreDelete deletes a Chore (FR-031), enforcing ownership.
func (s *Server) handleChoreDelete(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.choresService.DeleteChore(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to delete chore", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/chores", http.StatusFound)
}

// handleChoreConfirm confirms an agent-proposed schedule (FR-033),
// enforcing ownership.
func (s *Server) handleChoreConfirm(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	if err := s.choresService.ConfirmSchedule(r.Context(), user.Username, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to confirm chore schedule", "error", err)
		s.Error500(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/chores/"+id, http.StatusFound)
}

// choreIDAndActionFromPath parses "/admin/chores/{id}" or
// "/admin/chores/{id}/{action}" into (id, action). action is "" for the
// bare detail path.
func choreIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "chores", "{id}"] or ["admin", "chores", "{id}", "{action}"]
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
