package web

import (
	"context"

	"nuimanbot/internal/domain"
)

// notifyingRunRepository decorates a domain.RunRepository, publishing a
// RunEvent to a Hub after each state-changing operation succeeds, so the
// web admin's Job/Chore/History UI can reflect Run progress live via
// WebSocket (P6.8) instead of polling. Read-only operations (GetRun,
// ListRuns, CountUnnotified) pass straight through.
type notifyingRunRepository struct {
	domain.RunRepository
	hub *Hub
}

// NewNotifyingRunRepository wraps repo so writes also publish to hub. hub
// may be nil (Hub.Publish is a documented no-op on a nil receiver), so this
// is safe to use unconditionally in wiring.
func NewNotifyingRunRepository(repo domain.RunRepository, hub *Hub) domain.RunRepository {
	return &notifyingRunRepository{RunRepository: repo, hub: hub}
}

// SaveRun persists r via the wrapped repository, then publishes a
// "run_status" event carrying r's new Status.
func (n *notifyingRunRepository) SaveRun(ctx context.Context, r *domain.Run) error {
	if err := n.RunRepository.SaveRun(ctx, r); err != nil {
		return err
	}
	n.hub.Publish(r.OwnerUserID, RunEvent{
		Type:       "run_status",
		RunID:      r.ID,
		SourceType: string(r.SourceType),
		SourceID:   r.SourceID,
		Status:     string(r.Status),
	})
	return nil
}

// AppendLog appends chunk via the wrapped repository, then publishes a
// "run_log" event carrying it.
func (n *notifyingRunRepository) AppendLog(ctx context.Context, ownerUserID, runID, chunk string) error {
	if err := n.RunRepository.AppendLog(ctx, ownerUserID, runID, chunk); err != nil {
		return err
	}
	n.hub.Publish(ownerUserID, RunEvent{
		Type:     "run_log",
		RunID:    runID,
		LogChunk: chunk,
	})
	return nil
}

// MarkNotified clears the badge via the wrapped repository, then publishes
// the resulting "notification_badge" count. A failure reading the refreshed
// count is logged (via Hub's own error handling — n/a here, CountUnnotified
// has no side effect to publish) and simply skips the push; MarkNotified's
// own success/failure is unaffected.
func (n *notifyingRunRepository) MarkNotified(ctx context.Context, ownerUserID, runID string) error {
	if err := n.RunRepository.MarkNotified(ctx, ownerUserID, runID); err != nil {
		return err
	}
	if count, err := n.CountUnnotified(ctx, ownerUserID); err == nil {
		n.hub.Publish(ownerUserID, RunEvent{Type: "notification_badge", UnnotifiedCount: &count})
	}
	return nil
}

var _ domain.RunRepository = (*notifyingRunRepository)(nil)
