package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
)

// RunEnqueuer is the subset of WorkerPool a ChoreScheduler needs: enqueue a
// new RunRequest and check whether a given source is currently executing
// (FR-035's skip-if-still-running check).
type RunEnqueuer interface {
	Enqueue(req RunRequest) error
	IsSourceRunning(sourceID string) bool
}

// RunRecorder is the subset of persistence a ChoreScheduler needs beyond
// the ChoreRepository itself: creating the Run record for a firing (or
// skipped) Chore.
type RunRecorder interface {
	SaveRun(ctx context.Context, r *domain.Run) error
}

// ChoreScheduler polls domain.ChoreRepository.ListAllDue on a fixed tick and,
// for each due Chore, either enqueues a new Run (advancing NextFireTime) or
// records a skipped Run when the Chore's previous run is still active
// (FR-035). This is the infrastructure-layer driver behind FR-032/FR-035;
// it depends only on domain repository interfaces plus RunEnqueuer, so it
// has no dependency on the adapter/web layer.
type ChoreScheduler struct {
	chores   domain.ChoreRepository
	runs     RunRecorder
	pool     RunEnqueuer
	interval time.Duration
	now      func() time.Time
}

// NewChoreScheduler creates a ChoreScheduler polling every interval.
func NewChoreScheduler(chores domain.ChoreRepository, runs RunRecorder, pool RunEnqueuer, interval time.Duration) *ChoreScheduler {
	return &ChoreScheduler{
		chores:   chores,
		runs:     runs,
		pool:     pool,
		interval: interval,
		now:      time.Now,
	}
}

// Run polls until ctx is cancelled. Intended to be launched in its own
// goroutine by the caller (cmd/nuimanbot's DI wiring).
func (s *ChoreScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick evaluates all due Chores once. Exported indirectly via Run, but kept
// callable directly in tests for determinism (no need to wait on a ticker).
func (s *ChoreScheduler) tick(ctx context.Context) {
	now := s.now()
	due, err := s.chores.ListAllDue(ctx, now)
	if err != nil {
		slog.Error("chore scheduler: failed to list due chores", "error", err)
		return
	}

	for _, chore := range due {
		s.fireOne(ctx, chore, now)
	}
}

// fireOne handles a single due Chore: skip-and-log if its previous run is
// still active (FR-035), otherwise enqueue a new Run. Either way,
// NextFireTime is advanced so a scheduler outage doesn't cause the Chore to
// fire repeatedly for the same missed window once it catches up.
func (s *ChoreScheduler) fireOne(ctx context.Context, chore *domain.Chore, now time.Time) {
	next, err := NextFireTime(chore.Schedule.CronExpression, now)
	if err != nil {
		slog.Error("chore scheduler: invalid cron expression, chore will not be rescheduled",
			"choreID", chore.ID, "error", err)
		return
	}

	runID := uuid.NewString()

	if s.pool.IsSourceRunning(chore.ID) {
		reason := "skipped — previous run still active"
		run := &domain.Run{
			ID:          runID,
			OwnerUserID: chore.OwnerUserID,
			SourceType:  domain.SourceTypeChore,
			SourceID:    chore.ID,
			Status:      domain.RunStatusSkipped,
			SkipReason:  &reason,
			CreatedAt:   now,
		}
		if err := s.runs.SaveRun(ctx, run); err != nil {
			slog.Error("chore scheduler: failed to record skipped run", "choreID", chore.ID, "error", err)
		}
	} else {
		run := &domain.Run{
			ID:          runID,
			OwnerUserID: chore.OwnerUserID,
			SourceType:  domain.SourceTypeChore,
			SourceID:    chore.ID,
			Status:      domain.RunStatusQueued,
			CreatedAt:   now,
		}
		if err := s.runs.SaveRun(ctx, run); err != nil {
			slog.Error("chore scheduler: failed to create run record", "choreID", chore.ID, "error", err)
			return
		}
		if err := s.pool.Enqueue(RunRequest{
			RunID:       runID,
			OwnerUserID: chore.OwnerUserID,
			SourceType:  domain.SourceTypeChore,
			SourceID:    chore.ID,
			EnqueuedAt:  now,
		}); err != nil {
			slog.Error("chore scheduler: failed to enqueue run", "choreID", chore.ID, "error", err)
			return
		}
	}

	if err := s.chores.UpdateNextFireTime(ctx, chore.OwnerUserID, chore.ID, next); err != nil {
		slog.Error("chore scheduler: failed to advance NextFireTime", "choreID", chore.ID, "error", err)
	}
}
