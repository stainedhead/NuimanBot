package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// MemoriesService is the interface the web admin's Memories environment
// (FR-045–FR-047) depends on. Production wiring composes
// internal/usecase/memories.Service, wrapping the existing
// memoryv2.MemoryCellRepository read-only (FR-046: users cannot create/
// edit/delete memory entries via the UI — only the agent, via chat, per
// FR-047, which is not built in this pass).
//
// ownerUserID scoping note: see internal/usecase/memories.Service's package
// doc comment for the documented ownerUserID -> ConversationID mapping
// assumption this interface's implementation relies on.
type MemoriesService interface {
	// ListCells returns memory cells visible to ownerUserID matching filter
	// (FR-045's browse/search view).
	ListCells(ctx context.Context, ownerUserID string, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error)
	// GetCell retrieves a single memory cell by ID, scoped to ownerUserID's
	// visibility.
	GetCell(ctx context.Context, ownerUserID, cellID string) (*memoryv2.MemoryCell, error)
	// AskAboutCell answers a single-turn question grounded in one memory
	// cell's own content (FR-047/FR-R4's per-item chat template). Ownership
	// is enforced the same way GetCell enforces it.
	AskAboutCell(ctx context.Context, ownerUserID, cellID, question string) (string, error)
}

// SetMemoriesService sets the Memories environment's service.
func (s *Server) SetMemoriesService(svc MemoriesService) {
	s.memoriesService = svc
}

// MemoriesPageData is the template data for the Memories list/search page.
// It carries no CSRF token: the environment is read-only (FR-046) and its
// only form is a GET filter form, which needs none.
type MemoriesPageData struct {
	*BaseData
	Cells       []*memoryv2.MemoryCell
	Scene       string
	MinSalience string
}

// MemoryDetailPageData is the template data for a single memory cell's
// detail page. The page is otherwise read-only (FR-046) except for the
// per-item chat form (FR-047/FR-R4): Question/Answer/AskError carry the
// result of the most recent single-turn question, rendered inline on the
// same response — there is no persisted per-item chat history, only the
// last exchange.
type MemoryDetailPageData struct {
	*BaseData
	Cell      *memoryv2.MemoryCell
	CSRFToken string
	Question  string
	Answer    string
	AskError  string
}

// handleMemories lists/searches the current user's memory cells (FR-045).
// GET only — there is no POST/create route for this environment (FR-046).
func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.memoriesService == nil {
		http.Error(w, "Memories service not configured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	scene := query.Get("scene")
	minSalienceStr := query.Get("min_salience")

	filter := memoryv2.MemoryCellFilter{Scene: scene}
	if minSalienceStr != "" {
		if v, err := strconv.ParseFloat(minSalienceStr, 64); err == nil {
			filter.MinSalience = &v
		}
	}

	cells, err := s.memoriesService.ListCells(r.Context(), user.Username, filter)
	if err != nil {
		slog.Error("Failed to list memory cells", "error", err)
		s.Error500(w, r, err)
		return
	}
	sortCellsByCreatedAtDesc(cells)

	data := &MemoriesPageData{
		BaseData:    s.baseDataFor(r.Context(), user, "Memories", "memories"),
		Cells:       cells,
		Scene:       scene,
		MinSalience: minSalienceStr,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "memories.html", data); err != nil {
		slog.Error("Failed to render memories template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleMemorySubroutes dispatches /admin/memories/{cellID}[/ask]. The bare
// detail path is GET-only (read-only environment, FR-046); /ask is the
// per-item chat template's POST-only affordance (FR-047/FR-R4). Any other
// method or subroute returns 405/404 respectively.
func (s *Server) handleMemorySubroutes(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.memoriesService == nil {
		http.Error(w, "Memories service not configured", http.StatusInternalServerError)
		return
	}

	id, action := memoryIDAndActionFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		s.handleMemoryDetail(w, r, user, id)
	case "ask":
		s.handleMemoryAsk(w, r, user, id)
	default:
		http.NotFound(w, r)
	}
}

// handleMemoryDetail renders a single memory cell's read-only detail page,
// including the per-item chat form (FR-047/FR-R4).
func (s *Server) handleMemoryDetail(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cell, err := s.memoriesService.GetCell(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, memoryv2.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get memory cell", "error", err)
		s.Error500(w, r, err)
		return
	}

	s.renderMemoryDetail(w, r, user, cell, "", "", "")
}

// handleMemoryAsk answers a single-turn question grounded in one memory
// cell's own content (FR-047/FR-R4 — Memory is the reference implementation
// of the per-item chat template the other three environments, Job/Chore/
// Run, are meant to follow in a fast-follow pass). POST-only; the answer is
// rendered inline on the same detail page, not persisted — there is no
// per-item conversation history in this minimal template.
func (s *Server) handleMemoryAsk(w http.ResponseWriter, r *http.Request, user *User, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	question := sanitizedFormValue(r, "question")

	cell, err := s.memoriesService.GetCell(r.Context(), user.Username, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, memoryv2.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to get memory cell", "error", err)
		s.Error500(w, r, err)
		return
	}

	answer, askErr := s.memoriesService.AskAboutCell(r.Context(), user.Username, id, question)
	if askErr != nil {
		if errors.Is(askErr, domain.ErrNotFound) || errors.Is(askErr, memoryv2.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("Failed to answer memory chat question", "error", askErr)
		s.renderMemoryDetail(w, r, user, cell, question, "", "Sorry, couldn't get an answer right now — please try again.")
		return
	}

	s.renderMemoryDetail(w, r, user, cell, question, answer, "")
}

// renderMemoryDetail is the single render path shared by the bare GET
// detail page and the POST /ask handler, so both stay in sync.
func (s *Server) renderMemoryDetail(w http.ResponseWriter, r *http.Request, user *User, cell *memoryv2.MemoryCell, question, answer, askError string) {
	data := &MemoryDetailPageData{
		BaseData:  s.baseDataFor(r.Context(), user, "Memory: "+cell.Scene, "memories"),
		Cell:      cell,
		CSRFToken: s.auth.GenerateCSRFToken(),
		Question:  question,
		Answer:    answer,
		AskError:  askError,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "memory_detail.html", data); err != nil {
		slog.Error("Failed to render memory detail template", "error", err)
		s.Error500(w, r, err)
	}
}

// sortCellsByCreatedAtDesc sorts in place, most-recently-created first —
// display ordering only, not a business rule.
func sortCellsByCreatedAtDesc(cells []*memoryv2.MemoryCell) {
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].CreatedAt.After(cells[j].CreatedAt)
	})
}

// memoryIDFromPath parses "/admin/memories/{id}" into id. Any other shape
// (empty, trailing extra segments) returns "" so the caller 404s.
func memoryIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "memories", "{id}"]
	if len(parts) != 3 || parts[2] == "" {
		return ""
	}
	return parts[2]
}

// memoryIDAndActionFromPath parses "/admin/memories/{id}" or
// "/admin/memories/{id}/{action}" into (id, action), matching
// chatIDAndActionFromPath's convention. action is "" for the bare detail
// path. Any deeper path returns ("", "") so the caller 404s.
func memoryIDAndActionFromPath(path string) (id string, action string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "memories", "{id}"] or ["admin", "memories", "{id}", "{action}"]
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
