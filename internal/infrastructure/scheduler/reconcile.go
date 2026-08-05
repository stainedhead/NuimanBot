package scheduler

import (
	"context"
	"log/slog"
	"time"

	"nuimanbot/internal/domain"
)

// interruptedRunError is the Error message recorded on a Run reconciled at
// startup (FR-R2), distinguishing a restart-interrupted run from an
// ordinary execution failure in History (Observability NFR).
const interruptedRunError = "run interrupted by server restart"

// ReconcileInterruptedRuns scans every Run currently in a non-terminal
// state (Running, or Queued with no matching entry in queue) and
// transitions each to Failed with a clear restart-related Error (FR-R2,
// Reliability NFR): a server restart must not silently strand a Run at
// Running or Queued forever with no error, no retry, and no way for the
// user to know it isn't still progressing. Returns the number of Runs
// reconciled.
//
// Re-enqueuing an interrupted run instead of failing it was considered
// (the acceptance criteria allows either) but rejected: no live Executor
// exists yet that can determine whether a partially-executed run is safe
// to retry from scratch without duplicating side effects (see pool.go's
// Executor doc comment on the deferred real agent-invocation wiring), so
// failing with a clear, visible error is the only choice that can't
// silently duplicate or lose work. Revisit once a real Executor's
// idempotency guarantees are known.
//
// Must be called after queue has been Load()ed and before
// WorkerPool.Start, so the queue snapshot reflects on-disk state and no
// worker has had a chance to pick up new work yet.
func ReconcileInterruptedRuns(ctx context.Context, runs domain.RunRepository, queue *Queue, now func() time.Time) (int, error) {
	stuck, err := runs.ListAllNonTerminal(ctx)
	if err != nil {
		return 0, err
	}

	stillQueued := make(map[string]bool)
	for _, req := range queue.Snapshot() {
		stillQueued[req.RunID] = true
	}

	reconciled := 0
	for _, run := range stuck {
		if run.Status == domain.RunStatusQueued && stillQueued[run.ID] {
			continue // Genuinely still queued, not interrupted.
		}

		errMsg := interruptedRunError
		run.Status = domain.RunStatusFailed
		run.Error = &errMsg
		ended := now()
		run.EndedAt = &ended
		if run.StartedAt == nil {
			run.StartedAt = &ended
		}

		if err := runs.SaveRun(ctx, run); err != nil {
			slog.Error("reconcile: failed to mark interrupted run failed", "runID", run.ID, "error", err)
			continue
		}
		reconciled++
	}

	return reconciled, nil
}
