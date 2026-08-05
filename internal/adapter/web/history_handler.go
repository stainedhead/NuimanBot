package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// historyDateFormat is the query-param date format for since/until filters
// (?since=2026-01-01&until=2026-01-31).
const historyDateFormat = "2006-01-02"

// HistoryService is the interface the web admin's History environment
// (FR-040–FR-044) depends on. Production wiring composes
// internal/usecase/history.Service. ownerUserID = session Username (see
// chats_handler.go's ChatsService doc comment); cross-owner access -> 404.
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
	// ReadLog returns runID's captured processing log content, scoped to
	// its owner (FR-R17). Returns ("", nil) if the run has no log yet.
	ReadLog(ctx context.Context, ownerUserID, runID string) (string, error)
	// ReadResults returns runID's RESULTS.md content, scoped to its owner
	// (FR-R17). Returns ("", nil) if the run hasn't produced results yet.
	ReadResults(ctx context.Context, ownerUserID, runID string) (string, error)
}

// SetHistoryService sets the History environment's service.
func (s *Server) SetHistoryService(svc HistoryService) {
	s.historyService = svc
}

// HistoryPageData is the template data for the History list/filter page.
type HistoryPageData struct {
	*BaseData
	Runs   []*domain.Run
	Filter historyFilterForm
}

// historyFilterForm mirrors the parsed query params back into the template
// so the filter form can preserve the user's current selection.
type historyFilterForm struct {
	SourceType string
	Status     string
	Since      string
	Until      string
}

// RunDetailPageData is the template data for a single Run's detail page.
type RunDetailPageData struct {
	*BaseData
	Run            *domain.Run
	LogContent     string
	ResultsContent string
}

// handleHistory lists/filters the current user's Job/Chore runs
// (GET /admin/history?source_type=&status=&since=&until=), FR-040/FR-041.
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

	form, filter := parseRunFilter(r)

	runs, err := s.historyService.ListRuns(r.Context(), user.Username, filter)
	if err != nil {
		slog.Error("Failed to list history runs", "error", err)
		s.Error500(w, r, err)
		return
	}

	base := s.baseDataFor(user, "History", "history")
	s.withUnviewedRunCount(r.Context(), base, user)

	data := &HistoryPageData{
		BaseData: base,
		Runs:     runs,
		Filter:   form,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "history.html", data); err != nil {
		slog.Error("Failed to render history template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleHistorySubroutes dispatches /admin/history/{runID} — per-run detail
// (FR-040), marking the run viewed as a side effect (FR-044).
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

	id, action := runIDAndActionFromPath(r.URL.Path)
	if id == "" || action != "" {
		http.NotFound(w, r)
		return
	}

	s.handleHistoryDetail(w, r, user, id)
}

// handleHistoryDetail renders a single Run's status/timing/log/results
// (FR-040), reading log/results content directly from disk, and marks the
// run viewed (FR-044) as a side effect of viewing it. A run that doesn't
// exist or belongs to a different user renders 404 (Edge Case #10).
func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	run, err := s.historyService.GetRun(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get run", "error", err)
		s.Error500(w, r, err)
		return
	}

	if err := s.historyService.MarkViewed(r.Context(), user.Username, id); err != nil {
		slog.Error("Failed to mark run viewed", "error", err, "run_id", id)
	}

	base := s.baseDataFor(user, "Run Detail", "history")
	s.withUnviewedRunCount(r.Context(), base, user)

	logContent, err := s.historyService.ReadLog(r.Context(), user.Username, id)
	if err != nil {
		slog.Error("Failed to read run log", "error", err, "run_id", id)
	}
	resultsContent, err := s.historyService.ReadResults(r.Context(), user.Username, id)
	if err != nil {
		slog.Error("Failed to read run results", "error", err, "run_id", id)
	}

	data := &RunDetailPageData{
		BaseData:       base,
		Run:            run,
		LogContent:     logContent,
		ResultsContent: resultsContent,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "run_detail.html", data); err != nil {
		slog.Error("Failed to render run detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// withUnviewedRunCount populates base.UnviewedRunCount from the History
// notification badge (FR-044), logging and leaving it at zero on error
// rather than failing the page render.
func (s *Server) withUnviewedRunCount(ctx context.Context, base *BaseData, user *User) {
	count, err := s.historyService.UnviewedCount(ctx, user.Username)
	if err != nil {
		slog.Error("Failed to get unviewed run count", "error", err)
		return
	}
	base.UnviewedRunCount = count
}

// parseRunFilter parses /admin/history's query params into a
// domain.RunFilter (FR-041) plus the form-echo struct used to preserve the
// user's current filter selection in the template. Unrecognized or
// unparsable values are treated as "no filter" for that dimension rather
// than erroring, so a malformed query string degrades to an unfiltered list
// instead of a broken page.
func parseRunFilter(r *http.Request) (historyFilterForm, domain.RunFilter) {
	q := r.URL.Query()
	form := historyFilterForm{
		SourceType: strings.TrimSpace(q.Get("source_type")),
		Status:     strings.TrimSpace(q.Get("status")),
		Since:      strings.TrimSpace(q.Get("since")),
		Until:      strings.TrimSpace(q.Get("until")),
	}

	var filter domain.RunFilter
	if form.SourceType != "" {
		st := domain.SourceType(form.SourceType)
		filter.SourceType = &st
	}
	if form.Status != "" {
		status := domain.RunStatus(form.Status)
		filter.Status = &status
	}
	if since, err := time.Parse(historyDateFormat, form.Since); err == nil {
		filter.Since = &since
	}
	if until, err := time.Parse(historyDateFormat, form.Until); err == nil {
		// Inclusive of the full "until" day.
		until = until.Add(24*time.Hour - time.Nanosecond)
		filter.Until = &until
	}
	return form, filter
}

// runIDAndActionFromPath parses "/admin/history/{id}" into (id, ""). Any
// further path segment is returned as action so the caller can 404 on it —
// History has no per-run subroutes beyond the detail page itself (FR-042's
// per-run chat interface is out of scope for this environment).
func runIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "history", "{id}"] or ["admin", "history", "{id}", "{action}", ...]
	if len(parts) < 3 || parts[2] == "" {
		return "", ""
	}
	if len(parts) == 3 {
		return parts[2], ""
	}
	return parts[2], strings.Join(parts[3:], "/")
}
