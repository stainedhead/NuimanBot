package web

import (
	"context"
	"net/http"

	"nuimanbot/internal/domain/memoryv2"
)

// MemoriesService is the interface the web admin's Memories environment
// (FR-045–FR-047) depends on. Production wiring composes
// internal/usecase/memories.Service, wrapping the existing
// memoryv2.MemoryCellRepository/MemorySceneRepository read-only (FR-046:
// users cannot create/edit/delete memory entries via the UI — only the
// agent, via chat, per FR-047).
//
// ownerUserID scoping note: memoryv2.MemoryCellFilter.ConversationID is the
// existing repository's scoping key (see internal/usecase/memoryv2's
// curator/recall services); the Memories usecase Service is responsible for
// mapping ownerUserID to the right filter value(s) for that user's visible
// cells — verify against how ConversationID is populated for a given user
// across gateways before assuming a 1:1 mapping (a user may have multiple
// conversation IDs across platforms).
//
// STATUS: scaffold only — flesh out following chats_handler.go's exact
// pattern (read-only: no create/edit/delete routes should exist here at
// all, per FR-046).
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

// handleMemories lists/searches the current user's memory cells
// (GET /admin/memories only — no POST/create route exists, FR-046).
// PLACEHOLDER: replace with the full chats_handler.go-style implementation.
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
	http.Error(w, "Memories environment not yet implemented", http.StatusNotImplemented)
}

// handleMemorySubroutes dispatches /admin/memories/{cellID} — read-only
// detail plus a chat interface for discussing/requesting edits (FR-047;
// the agent is the sole writer — this page never exposes an edit form).
// PLACEHOLDER: replace with the full implementation.
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
	http.Error(w, "Memories environment not yet implemented", http.StatusNotImplemented)
}
