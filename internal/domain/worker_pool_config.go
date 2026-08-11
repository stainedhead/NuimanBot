package domain

import "fmt"

// WorkerPoolConfig configures the shared Job/Chore execution worker pool
// (FR-004, FR-039): N concurrent workers, system-wide, admin-only.
type WorkerPoolConfig struct {
	MaxConcurrentWorkers int
}

// MaxWorkerPoolSize is the upper bound Validate enforces on
// MaxConcurrentWorkers (FR-R15). This is admin-only (FR-004), so the
// practical severity of an unbounded value is bounded — but a fat-fingered
// extra digit with no ceiling could momentarily burst-dispatch an unbounded
// number of concurrent Runs (each potentially an LLM call) against a single
// process with no other resource limit in front of it. 256 is a generous
// cap, well beyond any realistic single-machine deployment's concurrency,
// chosen to catch input mistakes rather than to tune production throughput.
const MaxWorkerPoolSize = 256

// Validate reports whether the configured worker count is usable. A pool of
// zero or negative workers can never execute a Job/Chore, silently stalling
// every run forever; a pool above MaxWorkerPoolSize is almost certainly an
// input mistake, not an intentional deployment choice — both are rejected
// explicitly rather than left to misbehave at runtime.
func (c WorkerPoolConfig) Validate() error {
	if c.MaxConcurrentWorkers <= 0 {
		return fmt.Errorf("%w: worker pool size must be positive, got %d", ErrInvalidInput, c.MaxConcurrentWorkers)
	}
	if c.MaxConcurrentWorkers > MaxWorkerPoolSize {
		return fmt.Errorf("%w: worker pool size must not exceed %d, got %d", ErrInvalidInput, MaxWorkerPoolSize, c.MaxConcurrentWorkers)
	}
	return nil
}
