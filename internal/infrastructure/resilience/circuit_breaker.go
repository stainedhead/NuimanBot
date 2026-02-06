package resilience

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// State represents the circuit breaker state.
type State string

const (
	// StateClosed allows all requests through.
	StateClosed State = "closed"
	// StateOpen rejects all requests.
	StateOpen State = "open"
	// StateHalfOpen allows a single test request through.
	StateHalfOpen State = "half_open"
)

// Stats provides circuit breaker statistics.
type Stats struct {
	State               State
	SuccessCount        uint64
	FailureCount        uint64
	ConsecutiveFailures uint32
	LastFailureTime     time.Time
	LastStateChange     time.Time
}

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures.
type CircuitBreaker struct {
	maxFailures     uint32
	resetTimeout    time.Duration
	state           State
	failures        uint32
	successCount    uint64
	failureCount    uint64
	lastFailureTime time.Time
	lastStateChange time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker.
//
// maxFailures: Number of consecutive failures before opening the circuit.
// resetTimeout: Duration after which to attempt recovery (transition to half-open).
func NewCircuitBreaker(maxFailures uint32, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:     maxFailures,
		resetTimeout:    resetTimeout,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Call executes the given function if the circuit is closed or half-open.
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		cb.state = StateHalfOpen
		cb.lastStateChange = time.Now()
	}

	// Reject if circuit is open
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onSuccess is called after a successful operation.
func (cb *CircuitBreaker) onSuccess() {
	cb.successCount++
	cb.failures = 0

	// Close circuit if we're in half-open state
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.lastStateChange = time.Now()
	}
}

// onFailure is called after a failed operation.
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.failures++
	cb.lastFailureTime = time.Now()

	// Open circuit if threshold exceeded
	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}

	// Reopen circuit if failure occurs in half-open state
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
	}
}

// IsClosed returns true if the circuit is closed.
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateClosed
}

// IsOpen returns true if the circuit is open.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == StateOpen
}

// IsHalfOpen returns true if the circuit is half-open.
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if we should transition to half-open
	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		cb.state = StateHalfOpen
		cb.lastStateChange = time.Now()
	}

	return cb.state == StateHalfOpen
}

// Stats returns current circuit breaker statistics.
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return Stats{
		State:               cb.state,
		SuccessCount:        cb.successCount,
		FailureCount:        cb.failureCount,
		ConsecutiveFailures: cb.failures,
		LastFailureTime:     cb.lastFailureTime,
		LastStateChange:     cb.lastStateChange,
	}
}
