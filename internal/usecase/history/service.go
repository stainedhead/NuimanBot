// Package history orchestrates the web admin's History environment (FR-040–
// FR-044): list/filter a user's own Job and Chore runs, retrieve a single
// run's detail, clear the notification badge on view, and sweep expired
// runs under an independently configurable retention policy.
package history

import (
	"context"
	"time"

	"nuimanbot/internal/domain"
)

// Service orchestrates History list/filter/detail/notification/retention,
// backed by domain.RunRepository.
type Service struct {
	runs domain.RunRepository
}

// NewService creates a History Service backed by runs.
func NewService(runs domain.RunRepository) *Service {
	return &Service{runs: runs}
}

// ListRuns returns ownerUserID's Job/Chore runs matching filter (FR-040/
// FR-041), most recent first.
func (s *Service) ListRuns(ctx context.Context, ownerUserID string, filter domain.RunFilter) ([]*domain.Run, error) {
	return s.runs.ListRuns(ctx, ownerUserID, filter)
}

// GetRun retrieves a single Run by ID, scoped to its owner.
func (s *Service) GetRun(ctx context.Context, ownerUserID, runID string) (*domain.Run, error) {
	return s.runs.GetRun(ctx, ownerUserID, runID)
}

// MarkViewed clears the notification badge for a single run (FR-044),
// enforcing ownership.
func (s *Service) MarkViewed(ctx context.Context, ownerUserID, runID string) error {
	if _, err := s.runs.GetRun(ctx, ownerUserID, runID); err != nil {
		return err
	}
	return s.runs.MarkNotified(ctx, ownerUserID, runID)
}

// UnviewedCount returns the current notification badge count (FR-044).
func (s *Service) UnviewedCount(ctx context.Context, ownerUserID string) (int, error) {
	return s.runs.CountUnnotified(ctx, ownerUserID)
}

// ReadLog returns runID's captured processing log content, scoped to its
// owner (FR-R17). Returns ("", nil) if the run has no log yet.
func (s *Service) ReadLog(ctx context.Context, ownerUserID, runID string) (string, error) {
	return s.runs.ReadLog(ctx, ownerUserID, runID)
}

// ReadResults returns runID's RESULTS.md content, scoped to its owner
// (FR-R17). Returns ("", nil) if the run hasn't produced results yet.
func (s *Service) ReadResults(ctx context.Context, ownerUserID, runID string) (string, error) {
	return s.runs.ReadResults(ctx, ownerUserID, runID)
}

// SweepExpired deletes every terminal Run owned by ownerUserID that is
// expired under policy (FR-043), measured from each Run's CreatedAt — Runs
// are immutable historical records once terminal, so CreatedAt (unlike
// Conversation's UpdatedAt) is the correct retention anchor. Non-terminal
// runs are never swept. Returns the count deleted. A "Never" policy deletes
// nothing.
//
// Per spec.md Edge Case #7: before deleting a run the user hasn't viewed
// (Run.IsUnviewed()), MarkNotified is called first, so the notification
// badge count never ends up referencing a run that has since been deleted.
func (s *Service) SweepExpired(ctx context.Context, ownerUserID string, policy domain.RetentionPolicy, now time.Time) (int, error) {
	if policy.IsNever() {
		return 0, nil
	}
	runs, err := s.runs.ListRuns(ctx, ownerUserID, domain.RunFilter{})
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, run := range runs {
		if !run.Status.IsTerminal() || !policy.IsExpired(run.CreatedAt, now) {
			continue
		}
		if run.IsUnviewed() {
			if err := s.runs.MarkNotified(ctx, ownerUserID, run.ID); err != nil {
				continue // Best-effort sweep; one failure must not abort the rest.
			}
		}
		if err := s.runs.DeleteRun(ctx, ownerUserID, run.ID); err != nil {
			continue // Best-effort sweep; one failure must not abort the rest.
		}
		deleted++
	}
	return deleted, nil
}
