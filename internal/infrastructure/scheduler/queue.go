package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// RunRequest is a unit of work waiting in the FIFO queue (FR-027, FR-039):
// a Run to execute, referencing the Job or Chore that produced it.
type RunRequest struct {
	RunID       string            `json:"runID"`
	OwnerUserID string            `json:"ownerUserID"`
	SourceType  domain.SourceType `json:"sourceType"`
	SourceID    string            `json:"sourceID"`
	EnqueuedAt  time.Time         `json:"enqueuedAt"`
}

// Queue is a durable, in-process FIFO queue of RunRequests. Reliability NFR:
// a server restart must not lose queued Jobs — every mutation is persisted
// via AtomicFileWriter (temp-file + rename) immediately, and Load restores
// in-memory state from that file on startup.
//
// A sync.Mutex (not storage.FileLock) guards in-memory access: this queue
// is owned and mutated exclusively by this process's WorkerPool, so there
// is no cross-process writer to coordinate with — FileLock's flock-based
// exclusion (used elsewhere for users.json/bots.json, which can be written
// by concurrent request-handling goroutines across... still one process,
// but historically guarded that way) buys nothing extra here beyond what
// AtomicFileWriter's atomic rename already guarantees: a persisted read
// during a restart either sees the whole prior state or the whole new
// state, never a torn write.
type Queue struct {
	mu     sync.Mutex
	path   string
	writer *storage.AtomicFileWriter
	items  []RunRequest
}

// NewQueue creates a Queue that persists to persistPath. Call Load once at
// startup to restore any state left over from a prior run.
func NewQueue(persistPath string) *Queue {
	return &Queue{
		path:   persistPath,
		writer: storage.NewAtomicFileWriter(),
		items:  make([]RunRequest, 0),
	}
}

// Load restores the queue's contents from disk, replacing any in-memory
// state. A missing persist file is treated as an empty queue (first run).
func (q *Queue) Load() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := os.ReadFile(q.path)
	if err != nil {
		if os.IsNotExist(err) {
			q.items = make([]RunRequest, 0)
			return nil
		}
		return fmt.Errorf("failed to read queue file: %w", err)
	}

	var items []RunRequest
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("failed to parse queue file: %w", err)
	}
	if items == nil {
		items = make([]RunRequest, 0)
	}
	q.items = items
	return nil
}

// persistLocked writes the current in-memory state to disk. Callers must
// hold q.mu.
func (q *Queue) persistLocked() error {
	data, err := json.MarshalIndent(q.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}
	if err := q.writer.Write(q.path, data, 0644); err != nil {
		return fmt.Errorf("failed to persist queue: %w", err)
	}
	return nil
}

// Enqueue appends req to the back of the queue and persists the new state
// before returning, so a crash immediately after Enqueue returns cannot
// lose the request.
func (q *Queue) Enqueue(req RunRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, req)
	if err := q.persistLocked(); err != nil {
		// Roll back the in-memory append so state stays consistent with
		// what's on disk if the caller retries.
		q.items = q.items[:len(q.items)-1]
		return err
	}
	return nil
}

// Dequeue removes and returns the request at the front of the queue (FIFO).
// Returns false if the queue is empty.
func (q *Queue) Dequeue() (RunRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return RunRequest{}, false
	}
	req := q.items[0]
	remaining := q.items[1:]
	q.items = remaining
	if err := q.persistLocked(); err != nil {
		// Persistence failed — restore the item to the front so it isn't
		// silently lost from the in-memory queue; the caller sees the
		// error via a subsequent Len()/Load() mismatch on next restart if
		// the process dies before this is resolved, but within-process we
		// keep it in memory rather than dropping it.
		q.items = append([]RunRequest{req}, remaining...)
		return RunRequest{}, false
	}
	return req, true
}

// Len returns the current number of queued requests.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Snapshot returns a copy of the currently queued requests, front-to-back,
// for status reporting (e.g. a Job's queue position).
func (q *Queue) Snapshot() []RunRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]RunRequest, len(q.items))
	copy(out, q.items)
	return out
}
