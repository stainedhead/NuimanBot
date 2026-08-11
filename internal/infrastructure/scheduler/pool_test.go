package scheduler

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// recordingExecutor records every RunRequest it executes and completes
// immediately, unless a gate channel is set — in which case Execute blocks
// until the gate is closed, letting tests control exactly when work
// "finishes" to assert concurrency behavior deterministically.
type recordingExecutor struct {
	mu       sync.Mutex
	executed []RunRequest
	started  chan RunRequest // optionally notified when Execute begins
	gate     <-chan struct{} // optionally block until closed
}

func (e *recordingExecutor) Execute(_ context.Context, req RunRequest) {
	if e.started != nil {
		e.started <- req
	}
	if e.gate != nil {
		<-e.gate
	}
	e.mu.Lock()
	e.executed = append(e.executed, req)
	e.mu.Unlock()
}

func (e *recordingExecutor) executedCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.executed)
}

func newTestPool(t *testing.T, exec Executor, concurrency int) *WorkerPool {
	t.Helper()
	q := NewQueue(filepath.Join(t.TempDir(), "queue.json"))
	if err := q.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := NewWorkerPool(q, exec, concurrency)
	p.pollInterval = 5 * time.Millisecond
	return p
}

func TestWorkerPool_ExecutesQueuedWork(t *testing.T) {
	exec := &recordingExecutor{}
	p := newTestPool(t, exec, 2)

	for _, id := range []string{"r1", "r2", "r3"} {
		if err := p.Enqueue(RunRequest{RunID: id, SourceID: id}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	deadline := time.After(2 * time.Second)
	for exec.executedCount() < 3 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for all work to execute, got %d/3", exec.executedCount())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestWorkerPool_RespectsConcurrencyLimit(t *testing.T) {
	started := make(chan RunRequest, 10)
	gate := make(chan struct{})
	exec := &recordingExecutor{started: started, gate: gate}
	p := newTestPool(t, exec, 2)

	for _, id := range []string{"r1", "r2", "r3", "r4"} {
		if err := p.Enqueue(RunRequest{RunID: id, SourceID: id}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer func() {
		close(gate)
		p.Stop()
	}()

	// Exactly 2 should start (concurrency limit); the other 2 stay queued.
	<-started
	<-started

	select {
	case <-started:
		t.Fatal("expected only 2 concurrent executions, got a 3rd starting before any completed")
	case <-time.After(100 * time.Millisecond):
		// Expected: no 3rd start yet.
	}

	if got := p.ActiveCount(); got != 2 {
		t.Fatalf("expected ActiveCount() == 2, got %d", got)
	}
}

func TestWorkerPool_SetConcurrency_DoesNotPreemptInFlight(t *testing.T) {
	started := make(chan RunRequest, 10)
	gate := make(chan struct{})
	exec := &recordingExecutor{started: started, gate: gate}
	p := newTestPool(t, exec, 2)

	for _, id := range []string{"r1", "r2"} {
		if err := p.Enqueue(RunRequest{RunID: id, SourceID: id}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer func() {
		close(gate)
		p.Stop()
	}()

	<-started
	<-started
	if got := p.ActiveCount(); got != 2 {
		t.Fatalf("expected 2 active before reducing concurrency, got %d", got)
	}

	// Edge Case #4: reducing N below the current running count must not
	// pre-empt the in-flight runs.
	p.SetConcurrency(1)
	time.Sleep(50 * time.Millisecond)
	if got := p.ActiveCount(); got != 2 {
		t.Fatalf("expected in-flight runs to be unaffected by SetConcurrency, ActiveCount() = %d", got)
	}
}

func TestWorkerPool_IsSourceRunning(t *testing.T) {
	started := make(chan RunRequest, 10)
	gate := make(chan struct{})
	exec := &recordingExecutor{started: started, gate: gate}
	p := newTestPool(t, exec, 1)

	if err := p.Enqueue(RunRequest{RunID: "r1", SourceID: "chore-1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	<-started
	if !p.IsSourceRunning("chore-1") {
		t.Fatal("expected IsSourceRunning to report true while executing")
	}
	close(gate)
	p.Stop()

	if p.IsSourceRunning("chore-1") {
		t.Fatal("expected IsSourceRunning to report false after completion")
	}
}

func TestWorkerPool_Concurrency(t *testing.T) {
	exec := &recordingExecutor{}
	p := newTestPool(t, exec, 3)
	if got := p.Concurrency(); got != 3 {
		t.Fatalf("expected initial concurrency 3, got %d", got)
	}
	p.SetConcurrency(5)
	if got := p.Concurrency(); got != 5 {
		t.Fatalf("expected concurrency 5 after SetConcurrency, got %d", got)
	}
}

func TestWorkerPool_StartIsIdempotent(t *testing.T) {
	exec := &recordingExecutor{}
	p := newTestPool(t, exec, 1)
	if err := p.Enqueue(RunRequest{RunID: "r1", SourceID: "r1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Start(ctx) // second call must be a no-op, not a second dispatch loop
	defer p.Stop()

	deadline := time.After(1 * time.Second)
	for exec.executedCount() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for work to execute")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
