package resilience_test

import (
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/infrastructure/resilience"
)

func TestRetry_Success(t *testing.T) {
	policy := resilience.NewRetryPolicy(3, 10*time.Millisecond)

	attempts := 0
	err := policy.Retry(func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessAfterFailures(t *testing.T) {
	policy := resilience.NewRetryPolicy(3, 10*time.Millisecond)

	attempts := 0
	err := policy.Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	policy := resilience.NewRetryPolicy(3, 10*time.Millisecond)

	testErr := errors.New("persistent error")
	attempts := 0

	err := policy.Retry(func() error {
		attempts++
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	policy := resilience.NewRetryPolicy(3, 50*time.Millisecond)

	attempts := 0
	start := time.Now()

	policy.Retry(func() error {
		attempts++
		return errors.New("error")
	})

	duration := time.Since(start)

	// Expected delays: 50ms, 100ms = 150ms total (roughly)
	// Allow some margin for test execution
	if duration < 100*time.Millisecond {
		t.Errorf("Expected exponential backoff delays, got %v", duration)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NoRetryOnSuccess(t *testing.T) {
	policy := resilience.NewRetryPolicy(5, 10*time.Millisecond)

	attempts := 0
	start := time.Now()

	err := policy.Retry(func() error {
		attempts++
		return nil
	})

	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}

	// Should complete immediately without delays
	if duration > 50*time.Millisecond {
		t.Errorf("Expected immediate success, took %v", duration)
	}
}

func TestRetry_WithMaxDelay(t *testing.T) {
	policy := resilience.NewRetryPolicyWithMax(5, 10*time.Millisecond, 50*time.Millisecond)

	attempts := 0
	start := time.Now()

	policy.Retry(func() error {
		attempts++
		return errors.New("error")
	})

	duration := time.Since(start)

	// With max delay of 50ms:
	// Attempt 1: 0ms
	// Attempt 2: 10ms delay
	// Attempt 3: 20ms delay
	// Attempt 4: 40ms delay (would be 40ms)
	// Attempt 5: 50ms delay (capped at max 50ms)
	// Total: ~120ms

	if duration < 100*time.Millisecond || duration > 200*time.Millisecond {
		t.Errorf("Expected delays with max cap, got %v", duration)
	}

	if attempts != 5 {
		t.Errorf("Expected 5 attempts, got %d", attempts)
	}
}

func TestRetry_Stats(t *testing.T) {
	policy := resilience.NewRetryPolicy(3, 10*time.Millisecond)

	// Successful retry
	policy.Retry(func() error {
		return nil
	})

	// Failed retry (2 failures, then success)
	attempts := 0
	policy.Retry(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("error")
		}
		return nil
	})

	stats := policy.Stats()

	if stats.TotalRetries != 1 {
		t.Errorf("Expected 1 total retry (second call), got %d", stats.TotalRetries)
	}

	if stats.TotalAttempts < 4 { // 1 + 3 attempts
		t.Errorf("Expected at least 4 total attempts, got %d", stats.TotalAttempts)
	}
}
