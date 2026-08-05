package scheduler

import (
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

func TestQueue_EnqueueDequeue_FIFOOrder(t *testing.T) {
	q := NewQueue(filepath.Join(t.TempDir(), "queue.json"))
	if err := q.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	r1 := RunRequest{RunID: "r1", SourceType: domain.SourceTypeJob, SourceID: "job-1"}
	r2 := RunRequest{RunID: "r2", SourceType: domain.SourceTypeJob, SourceID: "job-2"}
	r3 := RunRequest{RunID: "r3", SourceType: domain.SourceTypeJob, SourceID: "job-3"}

	for _, r := range []RunRequest{r1, r2, r3} {
		if err := q.Enqueue(r); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	if q.Len() != 3 {
		t.Fatalf("expected length 3, got %d", q.Len())
	}

	got, ok := q.Dequeue()
	if !ok || got.RunID != "r1" {
		t.Fatalf("expected r1 first (FIFO), got %+v ok=%v", got, ok)
	}
	got, ok = q.Dequeue()
	if !ok || got.RunID != "r2" {
		t.Fatalf("expected r2 second (FIFO), got %+v ok=%v", got, ok)
	}
}

func TestQueue_DequeueEmpty(t *testing.T) {
	q := NewQueue(filepath.Join(t.TempDir(), "queue.json"))
	_, ok := q.Dequeue()
	if ok {
		t.Fatal("expected Dequeue on empty queue to return ok=false")
	}
}

func TestQueue_Load_MissingFileIsEmptyQueue(t *testing.T) {
	q := NewQueue(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err := q.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if q.Len() != 0 {
		t.Fatalf("expected empty queue, got length %d", q.Len())
	}
}

func TestQueue_RestartDurability(t *testing.T) {
	// Reliability NFR: a server restart must not lose queued Jobs.
	path := filepath.Join(t.TempDir(), "queue.json")

	q1 := NewQueue(path)
	if err := q1.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := q1.Enqueue(RunRequest{RunID: "r1", EnqueuedAt: time.Now()}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := q1.Enqueue(RunRequest{RunID: "r2", EnqueuedAt: time.Now()}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Simulate a restart: a brand new Queue instance loading the same path.
	q2 := NewQueue(path)
	if err := q2.Load(); err != nil {
		t.Fatalf("Load after restart: %v", err)
	}
	if q2.Len() != 2 {
		t.Fatalf("expected 2 queued requests to survive restart, got %d", q2.Len())
	}
	got, ok := q2.Dequeue()
	if !ok || got.RunID != "r1" {
		t.Fatalf("expected FIFO order preserved after restart, got %+v", got)
	}
}

func TestQueue_RestartDurability_AfterPartialDequeue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")

	q1 := NewQueue(path)
	if err := q1.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, id := range []string{"r1", "r2", "r3"} {
		if err := q1.Enqueue(RunRequest{RunID: id}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}
	if _, ok := q1.Dequeue(); !ok {
		t.Fatal("expected successful dequeue")
	}

	q2 := NewQueue(path)
	if err := q2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if q2.Len() != 2 {
		t.Fatalf("expected 2 remaining after restart, got %d", q2.Len())
	}
	got, _ := q2.Dequeue()
	if got.RunID != "r2" {
		t.Fatalf("expected r2 (the item after the dequeued one) to be next, got %q", got.RunID)
	}
}

func TestQueue_Snapshot_DoesNotMutate(t *testing.T) {
	q := NewQueue(filepath.Join(t.TempDir(), "queue.json"))
	if err := q.Enqueue(RunRequest{RunID: "r1"}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	snap := q.Snapshot()
	if len(snap) != 1 || snap[0].RunID != "r1" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	snap[0].RunID = "mutated"
	if q.Snapshot()[0].RunID != "r1" {
		t.Fatal("expected Snapshot to return a copy, not a reference to internal state")
	}
}
