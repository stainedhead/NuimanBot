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
// read-only detail page.
type MemoryDetailPageData struct {
	*BaseData
	Cell *memoryv2.MemoryCell
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
		BaseData:    s.baseDataFor(user, "Memories", "memories"),
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

// handleMemorySubroutes dispatches /admin/memories/{cellID} — read-only
// detail only. GET only: there is nothing to POST to in this environment
// (FR-046), so any other method returns 405.
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

	id := memoryIDFromPath(r.URL.Path)
	if id == "" {
		http.NotFound(w, r)
		return
	}
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

	data := &MemoryDetailPageData{
		BaseData: s.baseDataFor(user, "Memory: "+cell.Scene, "memories"),
		Cell:     cell,
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
// (empty, trailing extra segments) returns "" so the caller 404s — this
// environment has no subroutes/actions beyond the bare detail page.
func memoryIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["admin", "memories", "{id}"]
	if len(parts) != 3 || parts[2] == "" {
		return ""
	}
	return parts[2]
}
