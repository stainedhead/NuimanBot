package scheduler

import (
	"context"
	"sync"
	"time"
)

// Executor performs the actual work for a queued RunRequest: invoking the
// agent against the source Job/Chore's description in its working
// directory, and recording status/log/results/timing against the Run via
// domain.RunRepository as it progresses. Execute must not panic and must
// itself handle/record any failure (including provider/LLM failures,
// spec.md Edge Case #13) rather than letting WorkerPool's dispatch loop
// see an error — the pool's only job is concurrency and FIFO ordering, not
// interpreting execution outcomes.
//
// A concrete Executor wiring the agent's LLM/tool-loop orchestration
// (internal/usecase/chat and friends) into this interface is deliberately
// out of scope for this pass — see implementation-notes.md's "Deferred"
// section. WorkerPool is fully testable against a fake Executor.
type Executor interface {
	Execute(ctx context.Context, req RunRequest)
}

// WorkerPool consumes RunRequests from a Queue with up to N concurrent
// workers (FR-039), where N is adjustable at runtime (FR-004) without
// pre-empting in-flight work (spec.md Edge Case #4: reducing N below the
// current running count simply stops pulling new work until concurrency
// drops at or below the new N).
type WorkerPool struct {
	queue    *Queue
	executor Executor

	mu          sync.Mutex
	concurrency int
	running     map[string]bool // SourceID -> true, for FR-035 skip-if-still-running
	activeCount int

	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	started      bool
}

// defaultPollInterval is how often an idle dispatcher checks the queue for
// new work when it isn't already blocked waiting for a worker slot.
const defaultPollInterval = 50 * time.Millisecond

// NewWorkerPool creates a WorkerPool. concurrency must be positive — see
// domain.WorkerPoolConfig.Validate, which callers should apply to the
// configured value before constructing a pool.
func NewWorkerPool(queue *Queue, executor Executor, concurrency int) *WorkerPool {
	if concurrency < 1 {
		concurrency = 1
	}
	return &WorkerPool{
		queue:        queue,
		executor:     executor,
		concurrency:  concurrency,
		running:      make(map[string]bool),
		pollInterval: defaultPollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Enqueue adds req to the underlying Queue. Safe to call concurrently with
// Start/Stop.
func (p *WorkerPool) Enqueue(req RunRequest) error {
	return p.queue.Enqueue(req)
}

// Start begins dispatching queued work to workers, until ctx is cancelled
// or Stop is called. Start is idempotent-safe to call once; calling it
// again on an already-started pool is a no-op.
func (p *WorkerPool) Start(ctx context.Context) {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.started = true
	p.mu.Unlock()

	p.wg.Add(1)
	go p.dispatchLoop(ctx)
}

// Stop signals the dispatch loop to exit and waits for it (and any
// in-flight Execute calls it started) to return. In-flight work is allowed
// to finish; Stop does not cancel it directly — callers wanting a hard
// cutoff should cancel the ctx passed to Start instead.
func (p *WorkerPool) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

// SetConcurrency changes the maximum number of concurrent workers. Per
// spec.md Edge Case #4, this never pre-empts in-flight runs — a reduction
// simply makes the dispatch loop stop pulling new work until the active
// count drops to or below the new value.
func (p *WorkerPool) SetConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.concurrency = n
}

// ActiveCount returns the number of runs currently executing.
func (p *WorkerPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeCount
}

// IsSourceRunning reports whether a run for the given Job/Chore SourceID is
// currently executing — the primitive the Chore scheduler uses to
// implement FR-035 (skip-if-still-running).
func (p *WorkerPool) IsSourceRunning(sourceID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running[sourceID]
}

// hasCapacityLocked reports whether another worker may be started. Callers
// must hold p.mu.
func (p *WorkerPool) hasCapacityLocked() bool {
	return p.activeCount < p.concurrency
}

// dispatchLoop pulls work from the queue whenever a worker slot is free and
// hands it to a new goroutine running Executor.Execute.
func (p *WorkerPool) dispatchLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.tryDispatch(ctx)
		}
	}
}

// tryDispatch starts as many workers as current capacity and queue depth
// allow, in FIFO order.
func (p *WorkerPool) tryDispatch(ctx context.Context) {
	for {
		p.mu.Lock()
		if !p.hasCapacityLocked() {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		req, ok := p.queue.Dequeue()
		if !ok {
			return
		}

		p.mu.Lock()
		p.activeCount++
		p.running[req.SourceID] = true
		p.mu.Unlock()

		p.wg.Add(1)
		go p.runOne(ctx, req)
	}
}

// runOne executes a single RunRequest and updates pool bookkeeping when done.
func (p *WorkerPool) runOne(ctx context.Context, req RunRequest) {
	defer p.wg.Done()
	defer func() {
		p.mu.Lock()
		p.activeCount--
		delete(p.running, req.SourceID)
		p.mu.Unlock()
	}()

	p.executor.Execute(ctx, req)
}
