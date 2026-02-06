package resilience

import (
	"sync/atomic"
	"time"
)

// RetryStats provides retry policy statistics.
type RetryStats struct {
	TotalRetries  uint64
	TotalAttempts uint64
}

// RetryPolicy implements exponential backoff retry logic.
type RetryPolicy struct {
	maxAttempts   int
	initialDelay  time.Duration
	maxDelay      time.Duration
	totalRetries  atomic.Uint64
	totalAttempts atomic.Uint64
}

// NewRetryPolicy creates a new retry policy with exponential backoff.
//
// maxAttempts: Maximum number of attempts (including the initial attempt).
// initialDelay: Initial delay between retries (doubles with each retry).
func NewRetryPolicy(maxAttempts int, initialDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		maxAttempts:  maxAttempts,
		initialDelay: initialDelay,
		maxDelay:     0, // No max delay
	}
}

// NewRetryPolicyWithMax creates a retry policy with a maximum delay cap.
//
// maxAttempts: Maximum number of attempts.
// initialDelay: Initial delay between retries.
// maxDelay: Maximum delay between retries (caps exponential growth).
func NewRetryPolicyWithMax(maxAttempts int, initialDelay, maxDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		maxAttempts:  maxAttempts,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
	}
}

// Retry executes the given function with exponential backoff retry logic.
//
// The function will be retried up to maxAttempts times. The delay between
// retries starts at initialDelay and doubles with each attempt.
// If maxDelay is set, delays are capped at that value.
//
// Returns the error from the last attempt if all retries fail.
func (r *RetryPolicy) Retry(fn func() error) error {
	var err error
	delay := r.initialDelay

	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		r.totalAttempts.Add(1)

		err = fn()
		if err == nil {
			return nil
		}

		// Don't sleep after the last attempt
		if attempt < r.maxAttempts-1 {
			if attempt > 0 {
				r.totalRetries.Add(1)
			}

			// Sleep with exponential backoff
			time.Sleep(delay)

			// Double the delay for next attempt
			delay *= 2

			// Cap at max delay if set
			if r.maxDelay > 0 && delay > r.maxDelay {
				delay = r.maxDelay
			}
		}
	}

	return err
}

// Stats returns current retry policy statistics.
func (r *RetryPolicy) Stats() RetryStats {
	return RetryStats{
		TotalRetries:  r.totalRetries.Load(),
		TotalAttempts: r.totalAttempts.Load(),
	}
}
