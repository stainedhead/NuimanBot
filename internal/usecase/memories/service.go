// Package memories orchestrates the web admin's Memories environment
// (FR-045–FR-046): a read-only browse/search view over the agent-maintained
// memoryv2 memory-cell store. There is no create/update/delete here — the
// agent is the store's sole writer (FR-047, built elsewhere, outside this
// web UI pass).
//
// ownerUserID -> ConversationID mapping (ASSUMPTION, needs reviewer
// confirmation): memoryv2.MemoryCellFilter/MemoryCell.ConversationID is the
// existing repository's only scoping key. Tracing its production callers
// (internal/usecase/memoryv2's curator/recall services) shows it is
// populated from whatever ConversationID string the calling gateway passes
// in — which, depending on gateway, may be a per-conversation/session ID
// rather than a stable per-user identifier. No existing 1:1 mapping from a
// web-admin ownerUserID (session Username) to a set of ConversationIDs was
// found in this codebase. Pending a real mapping, this Service takes the
// pragmatic, explicit choice of treating ownerUserID itself as the
// ConversationID filter value (conversationIDFor below) — i.e. memory cells
// are only visible here if some caller populated ConversationID with the
// user's username verbatim. This under-shows a user's memories if their
// cells were actually filed under a session/conversation UUID instead. A
// human reviewer should confirm or replace this mapping before relying on
// Memories as a complete view of a user's stored knowledge.
package memories

import (
	"context"
	"fmt"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
)

// Service provides read-only, per-user-scoped access to the memoryv2
// memory-cell store (FR-045). It never calls Create/Update/Delete on the
// underlying repository (FR-046).
type Service struct {
	cells memoryv2.MemoryCellRepository
}

// NewService creates a memories Service backed by cells.
func NewService(cells memoryv2.MemoryCellRepository) *Service {
	return &Service{cells: cells}
}

// conversationIDFor maps ownerUserID to the ConversationID scoping value
// used to filter memoryv2 cells. See the package doc comment for the
// assumption this encodes.
func conversationIDFor(ownerUserID string) string {
	return ownerUserID
}

// ListCells returns memory cells visible to ownerUserID matching filter
// (FR-045's browse/search view). filter.ConversationID is always overridden
// from ownerUserID's mapping, regardless of what the caller passed — this
// is the isolation enforcement point that prevents a caller from viewing
// another user's cells by manipulating the filter.
func (s *Service) ListCells(ctx context.Context, ownerUserID string, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	filter.ConversationID = conversationIDFor(ownerUserID)
	return s.cells.List(ctx, filter)
}

// GetCell retrieves a single memory cell by ID, scoped to ownerUserID's
// visibility. A cell that doesn't exist resolves as memoryv2.ErrNotFound (the
// repository's own not-found error); a cell that exists but belongs to a
// different owner resolves as domain.ErrNotFound (FR-010/Edge Case #10 —
// existence is never disclosed across owners).
func (s *Service) GetCell(ctx context.Context, ownerUserID, cellID string) (*memoryv2.MemoryCell, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	cell, err := s.cells.Get(ctx, cellID)
	if err != nil {
		return nil, err
	}
	if cell.ConversationID != conversationIDFor(ownerUserID) {
		return nil, domain.ErrNotFound
	}
	return cell, nil
}
