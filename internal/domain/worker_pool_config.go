package domain

import "fmt"

// WorkerPoolConfig configures the shared Job/Chore execution worker pool
// (FR-004, FR-039): N concurrent workers, system-wide, admin-only.
type WorkerPoolConfig struct {
	MaxConcurrentWorkers int
}

// Validate reports whether the configured worker count is usable. A pool of
// zero or negative workers can never execute a Job/Chore, silently stalling
// every run forever — reject that explicitly rather than letting it
// misbehave at runtime.
func (c WorkerPoolConfig) Validate() error {
	if c.MaxConcurrentWorkers <= 0 {
		return fmt.Errorf("%w: worker pool size must be positive, got %d", ErrInvalidInput, c.MaxConcurrentWorkers)
	}
	return nil
}
