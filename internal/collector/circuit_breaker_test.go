package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 10*time.Second, 2)

	assert.Equal(t, StateClosed, cb.GetState())

	stats := cb.GetStats()
	assert.Equal(t, StateClosed, stats["state"])
	assert.Equal(t, 0, stats["failure_count"])
	assert.Equal(t, 0, stats["success_count"])
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 10*time.Second, 2)

	// Should allow execution initially
	err := cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, StateClosed, cb.GetState())

	// Second failure
	err = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, StateClosed, cb.GetState())

	// Third failure should open the circuit
	err = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())

	// Next execution should be rejected
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 100*time.Millisecond, 2)

	// Trigger open state
	err := cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Trigger state transition
	cb.Execute(context.Background(), func() error { return nil })

	// Should transition to half-open
	assert.Equal(t, StateHalfOpen, cb.GetState())
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(1, 100*time.Millisecond, 2)

	// Trigger open state
	err := cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Trigger state transition
	cb.Execute(context.Background(), func() error { return nil })

	// Should be half-open now
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// First success
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())

	// Second success should close the circuit
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 100*time.Millisecond, 2)

	// Trigger open state
	err := cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Trigger state transition
	cb.Execute(context.Background(), func() error { return nil })

	// Should be half-open now
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Failure should open the circuit again
	err = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(10, 100*time.Millisecond, 5)

	// Run multiple goroutines that trigger failures
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			cb.Execute(context.Background(), func() error {
				return errors.New("concurrent error")
			})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Circuit should be open (enough failures)
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(2, 10*time.Second, 1)

	// First failure
	cb.Execute(context.Background(), func() error {
		return errors.New("error 1")
	})
	assert.Equal(t, StateClosed, cb.GetState())

	// Success
	cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.Equal(t, StateClosed, cb.GetState())

	// Second failure - should open the circuit
	cb.Execute(context.Background(), func() error {
		return errors.New("error 2")
	})
	assert.Equal(t, StateOpen, cb.GetState())

	stats := cb.GetStats()
	assert.Equal(t, StateOpen, stats["state"])
	assert.Equal(t, 0, stats["failure_count"])
	assert.Equal(t, 0, stats["success_count"]) // Reset on failure
	assert.Equal(t, 2, stats["failure_threshold"])
	assert.Equal(t, 1, stats["success_threshold"])
	assert.Equal(t, 10*time.Second, stats["timeout"])
}

func TestSimple(t *testing.T) {
	cb := NewCircuitBreaker(1, 1*time.Second, 1)

	// Simple test - should be closed initially
	state := cb.GetState()
	t.Logf("Initial state = %d", state)
	assert.Equal(t, StateClosed, state)

	// Test stats
	stats := cb.GetStats()
	t.Logf("Stats state = %v", stats["state"])
	assert.Equal(t, StateClosed, stats["state"])
}
