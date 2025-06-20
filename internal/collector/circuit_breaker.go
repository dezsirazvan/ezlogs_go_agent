package collector

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	StateOpen     CircuitBreakerState = 0
	StateHalfOpen CircuitBreakerState = 1
	StateClosed   CircuitBreakerState = 2
)

// CircuitBreaker provides circuit breaker functionality for the collector
type CircuitBreaker struct {
	mu sync.RWMutex

	// Configuration
	failureThreshold int
	timeout          time.Duration
	successThreshold int

	// State
	state CircuitBreakerState

	// Counters
	failureCount int
	successCount int

	// Timing
	lastFailureTime time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, timeout time.Duration, successThreshold int) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		successThreshold: successThreshold,
		state:            StateClosed,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.canExecute() {
		return ErrCircuitBreakerOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// canExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		timeSinceFailure := time.Since(cb.lastFailureTime)
		if timeSinceFailure >= cb.timeout {
			logrus.WithFields(logrus.Fields{
				"time_since_failure": timeSinceFailure,
				"timeout":            cb.timeout,
				"current_state":      cb.state,
			}).Debug("Circuit breaker timeout reached, transitioning to half-open")

			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.transitionToHalfOpen()
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		logrus.WithFields(logrus.Fields{
			"time_since_failure": timeSinceFailure,
			"timeout":            cb.timeout,
		}).Debug("Circuit breaker still in timeout period")
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an execution
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.transitionToOpen()
		}
	case StateHalfOpen:
		cb.transitionToOpen()
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.transitionToClosed()
		}
	}
}

// transitionToOpen transitions the circuit breaker to open state
func (cb *CircuitBreaker) transitionToOpen() {
	if cb.state != StateOpen {
		logrus.WithFields(logrus.Fields{
			"old_state":         cb.state,
			"failure_count":     cb.failureCount,
			"failure_threshold": cb.failureThreshold,
		}).Warn("Circuit breaker opened")

		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		// Reset counters when transitioning to open
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	if cb.state != StateHalfOpen {
		logrus.WithField("old_state", cb.state).Info("Circuit breaker half-open")

		cb.state = StateHalfOpen
		cb.lastStateChange = time.Now()
		// Reset counters when transitioning to half-open
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// transitionToClosed transitions the circuit breaker to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	if cb.state != StateClosed {
		logrus.WithField("old_state", cb.state).Info("Circuit breaker closed")

		cb.state = StateClosed
		cb.lastStateChange = time.Now()
		// Reset counters when transitioning to closed
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state,
		"failure_count":     cb.failureCount,
		"success_count":     cb.successCount,
		"last_failure_time": cb.lastFailureTime,
		"last_state_change": cb.lastStateChange,
		"failure_threshold": cb.failureThreshold,
		"success_threshold": cb.successThreshold,
		"timeout":           cb.timeout,
	}
}
