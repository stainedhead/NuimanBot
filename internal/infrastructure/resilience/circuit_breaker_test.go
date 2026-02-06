package resilience_test

import (
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/infrastructure/resilience"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	if !cb.IsClosed() {
		t.Error("Expected circuit breaker to be initially closed")
	}

	if cb.IsOpen() {
		t.Error("Expected circuit breaker not to be open initially")
	}
}

func TestCircuitBreaker_SuccessfulCall(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !cb.IsClosed() {
		t.Error("Expected circuit to remain closed after success")
	}
}

func TestCircuitBreaker_FailureIncrementsCount(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	testErr := errors.New("test error")

	// First failure
	err := cb.Call(func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	// Should still be closed after 1 failure (threshold is 3)
	if !cb.IsClosed() {
		t.Error("Expected circuit to remain closed after 1 failure")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 100*time.Millisecond)

	testErr := errors.New("test error")

	// Trigger 3 failures to open the circuit
	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return testErr
		})
	}

	// Circuit should now be open
	if !cb.IsOpen() {
		t.Error("Expected circuit to be open after threshold failures")
	}

	// Next call should fail immediately without executing function
	executed := false
	err := cb.Call(func() error {
		executed = true
		return nil
	})

	if executed {
		t.Error("Function should not execute when circuit is open")
	}

	if err != resilience.ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := resilience.NewCircuitBreaker(2, 200*time.Millisecond)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return testErr
		})
	}

	if !cb.IsOpen() {
		t.Error("Expected circuit to be open")
	}

	// Wait for reset timeout
	time.Sleep(250 * time.Millisecond)

	// Circuit should transition to half-open
	if !cb.IsHalfOpen() {
		t.Error("Expected circuit to be half-open after timeout")
	}

	// Successful call should close the circuit
	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !cb.IsClosed() {
		t.Error("Expected circuit to close after successful call in half-open state")
	}
}

func TestCircuitBreaker_FailureInHalfOpenReopens(t *testing.T) {
	cb := resilience.NewCircuitBreaker(2, 100*time.Millisecond)

	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return testErr
		})
	}

	// Wait for half-open
	time.Sleep(150 * time.Millisecond)

	if !cb.IsHalfOpen() {
		t.Error("Expected circuit to be half-open")
	}

	// Failure in half-open should reopen the circuit
	cb.Call(func() error {
		return testErr
	})

	if !cb.IsOpen() {
		t.Error("Expected circuit to reopen after failure in half-open state")
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	// Success
	cb.Call(func() error { return nil })

	// Failures
	testErr := errors.New("test error")
	cb.Call(func() error { return testErr })
	cb.Call(func() error { return testErr })

	stats := cb.Stats()

	if stats.SuccessCount != 1 {
		t.Errorf("Expected 1 success, got %d", stats.SuccessCount)
	}

	if stats.FailureCount != 2 {
		t.Errorf("Expected 2 failures, got %d", stats.FailureCount)
	}

	if stats.State != "closed" {
		t.Errorf("Expected state 'closed', got '%s'", stats.State)
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := resilience.NewCircuitBreaker(10, 5*time.Second)

	// Run concurrent operations
	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func(id int) {
			if id%2 == 0 {
				cb.Call(func() error { return nil })
			} else {
				cb.Call(func() error { return errors.New("error") })
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	stats := cb.Stats()

	if stats.SuccessCount != 10 {
		t.Errorf("Expected 10 successes, got %d", stats.SuccessCount)
	}

	if stats.FailureCount != 10 {
		t.Errorf("Expected 10 failures, got %d", stats.FailureCount)
	}
}

func TestCircuitBreaker_ResetAfterSuccess(t *testing.T) {
	cb := resilience.NewCircuitBreaker(3, 5*time.Second)

	testErr := errors.New("test error")

	// 2 failures (below threshold)
	cb.Call(func() error { return testErr })
	cb.Call(func() error { return testErr })

	// Success should reset failure count
	cb.Call(func() error { return nil })

	stats := cb.Stats()

	// After success, consecutive failures should have been reset
	if stats.ConsecutiveFailures != 0 {
		t.Errorf("Expected consecutive failures to reset to 0, got %d", stats.ConsecutiveFailures)
	}
}
