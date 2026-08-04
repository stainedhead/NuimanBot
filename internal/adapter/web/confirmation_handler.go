package web

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// PendingConfirmation is a simplified view of a pending side-effecting-action
// confirmation (specs/260802-improve-nuimanbot-security, Part C) used for web
// admin UI display (task P5.8). Mirrors security.ConfirmationRequest without
// exposing that usecase-layer type directly to templates, matching this
// package's existing BotConfig/UserProfile convention.
type PendingConfirmation struct {
	ID             string
	UserID         string
	ConversationID string
	ToolName       string
	Action         string
	Summary        string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// ConfirmationService is the interface the web admin UI's confirmation pages
// (P5.8) depend on for listing pending side-effecting-action confirmations
// and resolving them (approve/deny). Production wiring composes
// security.ConfirmationStore.ListPending/Get with
// chat.Service.ResolveConfirmation (see cmd/nuimanbot/main.go).
type ConfirmationService interface {
	// ListPendingConfirmations returns every currently pending confirmation
	// across all users, oldest first.
	ListPendingConfirmations(ctx context.Context) ([]PendingConfirmation, error)
	// GetConfirmation retrieves a single pending confirmation by ID. Used to
	// check ownership (UserID) before letting a non-admin caller resolve it.
	GetConfirmation(ctx context.Context, id string) (PendingConfirmation, error)
	// ResolveConfirmation approves (approved=true) or denies (approved=false)
	// the confirmation identified by confirmationID.
	ResolveConfirmation(ctx context.Context, confirmationID string, approved bool) error
}

// SetConfirmationService sets the confirmation service for the server (optional).
func (s *Server) SetConfirmationService(svc ConfirmationService) {
	s.confirmationService = svc
}

// handleConfirmations lists pending confirmations (Part C, P5.8): an admin
// user sees every pending confirmation across all users; a non-admin user
// sees only their own (matched by PendingConfirmation.UserID against the
// current session's username).
func (s *Server) handleConfirmations(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	var pending []PendingConfirmation
	if s.confirmationService != nil {
		all, err := s.confirmationService.ListPendingConfirmations(r.Context())
		if err != nil {
			slog.Error("Failed to list pending confirmations", "error", err)
		} else {
			pending = visibleConfirmationsFor(user, all)
		}
	}

	data := struct {
		Title         string
		Confirmations []PendingConfirmation
		IsAdmin       bool
	}{
		Title:         "Pending Confirmations",
		Confirmations: pending,
		IsAdmin:       user.Role == "admin",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "confirmations.html", data); err != nil {
		slog.Error("Failed to render confirmations template", "error", err)
		s.Error500(w, r, err)
	}
}

// visibleConfirmationsFor returns the subset of all confirmations a given
// user is allowed to see: every entry for an admin, or only entries whose
// UserID matches the user's own username otherwise.
func visibleConfirmationsFor(user *User, all []PendingConfirmation) []PendingConfirmation {
	if user.Role == "admin" {
		return all
	}
	owned := make([]PendingConfirmation, 0, len(all))
	for _, c := range all {
		if c.UserID == user.Username {
			owned = append(owned, c)
		}
	}
	return owned
}

// handleConfirmationApprove approves the confirmation identified in the URL
// path (/admin/confirmations/{id}/approve).
func (s *Server) handleConfirmationApprove(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmationResolve(w, r, true)
}

// handleConfirmationDeny denies the confirmation identified in the URL path
// (/admin/confirmations/{id}/deny).
func (s *Server) handleConfirmationDeny(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmationResolve(w, r, false)
}

// handleConfirmationResolve approves or denies the confirmation identified in
// the URL path, enforcing that a non-admin user may only resolve their OWN
// pending confirmations (Part C, P5.8): ownership is checked against the
// confirmation's stored UserID before ResolveConfirmation is ever called, so
// a mismatched-user request is rejected without side effects.
func (s *Server) handleConfirmationResolve(w http.ResponseWriter, r *http.Request, approved bool) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := confirmationIDFromPath(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if s.confirmationService == nil {
		http.Error(w, "Confirmation service not configured", http.StatusInternalServerError)
		return
	}

	confirmation, err := s.confirmationService.GetConfirmation(r.Context(), id)
	if err != nil {
		http.Error(w, "Confirmation not found", http.StatusNotFound)
		return
	}

	if user.Role != "admin" && confirmation.UserID != user.Username {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := s.confirmationService.ResolveConfirmation(r.Context(), id, approved); err != nil {
		slog.Error("Failed to resolve confirmation", "error", err, "confirmation_id", id, "approved", approved)
		http.Error(w, "Failed to resolve confirmation", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/confirmations", http.StatusFound)
}

// confirmationIDFromPath extracts the confirmation ID from a path of the
// shape /admin/confirmations/{id}/approve or /admin/confirmations/{id}/deny.
// Returns "" if path does not have exactly this shape.
func confirmationIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "confirmations", "{id}", "approve"|"deny"]
	if len(parts) != 4 || parts[2] == "" {
		return ""
	}
	return parts[2]
}
