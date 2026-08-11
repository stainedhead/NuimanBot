package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// TestReconcileInterruptedRuns_CrashSimulation is the FR-R2 acceptance
// test: persist a Run at Running (simulating a worker mid-execution when
// the process died), construct a fresh Queue/RunRepository against the
// same storage directory with no live in-memory state carried over
// (simulating a real restart), and assert the Run is no longer stuck at
// Running afterward.
func TestReconcileInterruptedRuns_CrashSimulation(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()

	runRepo := storage.NewFileRunRepository(storagePath)
	startedAt := time.Now().Add(-time.Hour)
	stuckRun := &domain.Run{
		ID:          "stuck-run",
		OwnerUserID: "alice",
		SourceType:  domain.SourceTypeJob,
		SourceID:    "job-1",
		Status:      domain.RunStatusRunning,
		StartedAt:   &startedAt,
		CreatedAt:   startedAt,
	}
	if err := runRepo.SaveRun(ctx, stuckRun); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	// Pre-fix sanity check: nothing has reconciled it yet.
	got, err := runRepo.GetRun(ctx, "alice", "stuck-run")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusRunning {
		t.Fatalf("precondition failed: expected Running, got %v", got.Status)
	}

	// Simulate the restart: fresh Queue/RunRepository instances against the
	// same storage directory, no live in-memory state carried over.
	queuePath := filepath.Join(storagePath, "scheduler", "queue.json")
	freshQueue := NewQueue(queuePath)
	if err := freshQueue.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	freshRunRepo := storage.NewFileRunRepository(storagePath)

	n, err := ReconcileInterruptedRuns(ctx, freshRunRepo, freshQueue, time.Now)
	if err != nil {
		t.Fatalf("ReconcileInterruptedRuns: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 run reconciled, got %d", n)
	}

	got, err = freshRunRepo.GetRun(ctx, "alice", "stuck-run")
	if err != nil {
		t.Fatalf("GetRun after reconcile: %v", err)
	}
	if got.Status != domain.RunStatusFailed {
		t.Fatalf("expected Failed after reconciliation, got %v", got.Status)
	}
	if got.Error == nil || *got.Error != "run interrupted by server restart" {
		t.Fatalf("expected a restart-interruption Error message, got %v", got.Error)
	}
	if got.EndedAt == nil {
		t.Fatal("expected EndedAt to be set")
	}
}

// TestReconcileInterruptedRuns_QueuedWithNoMatchingQueueEntryIsReconciled
// covers the second non-terminal case FR-R2 targets: a Run left at Queued
// whose queue.json entry is gone (Dequeue removes the queue entry before
// execution begins — see queue.go's doc comment — so a crash between
// Dequeue and the worker actually starting can strand a Run at Queued with
// nothing left to ever pick it up).
func TestReconcileInterruptedRuns_QueuedWithNoMatchingQueueEntryIsReconciled(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()

	runRepo := storage.NewFileRunRepository(storagePath)
	run := &domain.Run{
		ID: "orphaned-queued-run", OwnerUserID: "alice",
		SourceType: domain.SourceTypeJob, SourceID: "job-1",
		Status: domain.RunStatusQueued, CreatedAt: time.Now(),
	}
	if err := runRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	queuePath := filepath.Join(storagePath, "scheduler", "queue.json")
	queue := NewQueue(queuePath)
	if err := queue.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Deliberately do not enqueue a matching RunRequest — the queue entry
	// is gone, as if Dequeue already removed it before the crash.

	n, err := ReconcileInterruptedRuns(ctx, runRepo, queue, time.Now)
	if err != nil {
		t.Fatalf("ReconcileInterruptedRuns: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 run reconciled, got %d", n)
	}
	got, err := runRepo.GetRun(ctx, "alice", "orphaned-queued-run")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusFailed {
		t.Fatalf("expected Failed, got %v", got.Status)
	}
}

// TestReconcileInterruptedRuns_QueuedWithMatchingQueueEntryIsLeftAlone
// proves a genuinely-still-queued Run (its queue.json entry is intact) is
// NOT touched — it's not interrupted, just legitimately waiting for a
// worker, and reconciliation must not fail it out from under a queue that
// will dispatch it normally once the pool starts.
func TestReconcileInterruptedRuns_QueuedWithMatchingQueueEntryIsLeftAlone(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()

	runRepo := storage.NewFileRunRepository(storagePath)
	run := &domain.Run{
		ID: "still-queued-run", OwnerUserID: "alice",
		SourceType: domain.SourceTypeJob, SourceID: "job-1",
		Status: domain.RunStatusQueued, CreatedAt: time.Now(),
	}
	if err := runRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	queuePath := filepath.Join(storagePath, "scheduler", "queue.json")
	queue := NewQueue(queuePath)
	if err := queue.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := queue.Enqueue(RunRequest{RunID: "still-queued-run", OwnerUserID: "alice", SourceType: domain.SourceTypeJob, SourceID: "job-1", EnqueuedAt: time.Now()}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	n, err := ReconcileInterruptedRuns(ctx, runRepo, queue, time.Now)
	if err != nil {
		t.Fatalf("ReconcileInterruptedRuns: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 runs reconciled, got %d", n)
	}
	got, err := runRepo.GetRun(ctx, "alice", "still-queued-run")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != domain.RunStatusQueued {
		t.Fatalf("expected still-queued run to remain Queued, got %v", got.Status)
	}
}

func TestReconcileInterruptedRuns_TerminalRunsUntouched(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()

	runRepo := storage.NewFileRunRepository(storagePath)
	completed := &domain.Run{ID: "done", OwnerUserID: "alice", Status: domain.RunStatusCompleted, CreatedAt: time.Now()}
	if err := runRepo.SaveRun(ctx, completed); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	queue := NewQueue(filepath.Join(storagePath, "scheduler", "queue.json"))
	if err := queue.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n, err := ReconcileInterruptedRuns(ctx, runRepo, queue, time.Now)
	if err != nil {
		t.Fatalf("ReconcileInterruptedRuns: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 runs reconciled, got %d", n)
	}
}

func TestReconcileInterruptedRuns_NoRunsIsANoOp(t *testing.T) {
	storagePath := t.TempDir()
	runRepo := storage.NewFileRunRepository(storagePath)
	queue := NewQueue(filepath.Join(storagePath, "scheduler", "queue.json"))
	if err := queue.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n, err := ReconcileInterruptedRuns(context.Background(), runRepo, queue, time.Now)
	if err != nil {
		t.Fatalf("ReconcileInterruptedRuns: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}
