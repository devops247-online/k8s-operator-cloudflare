package reliability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestNewCircuitBreakerWithRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name: "test-cb",
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)

	assert.Equal(t, "test-cb", cb.Name())
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(1), cb.config.MaxRequests)   // Default value
	assert.Equal(t, 60*time.Second, cb.config.Interval) // Default value
	assert.Equal(t, 60*time.Second, cb.config.Timeout)  // Default value
}

func TestNewCircuitBreaker(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := CircuitBreakerConfig{
		Name:        "test-cb-default",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
	}

	cb := NewCircuitBreaker(config, logger)

	assert.Equal(t, "test-cb-default", cb.Name())
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, uint32(5), cb.config.MaxRequests)
	assert.Equal(t, 30*time.Second, cb.config.Interval)
	assert.Equal(t, 10*time.Second, cb.config.Timeout)
}

func TestCircuitBreakerClosed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Execute successful requests
	for i := 0; i < 5; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}

	// Circuit should remain closed
	assert.Equal(t, StateClosed, cb.State())

	counts := cb.Counts()
	assert.Equal(t, uint32(5), counts.Requests)
	assert.Equal(t, uint32(5), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
}

func TestCircuitBreakerOpens(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 3 && counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Execute failed requests to trip the circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	// Circuit should be open now
	assert.Equal(t, StateOpen, cb.State())

	// Subsequent requests should be rejected
	err := cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 2,
		Interval:    10 * time.Second,
		Timeout:     100 * time.Millisecond, // Short timeout for testing
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 2 && counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Trip the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout to move to half-open
	time.Sleep(150 * time.Millisecond)

	// First request in half-open state should be allowed
	err := cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Circuit should still be half-open until MaxRequests successful
	assert.Equal(t, StateHalfOpen, cb.State())

	// Second successful request should close the circuit
	err = cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 2 && counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Trip the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Fail in half-open state should reopen the circuit
	err := cb.Execute(ctx, func(_ context.Context) error {
		return testErr
	})
	assert.Equal(t, testErr, err)

	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreakerMaxRequestsInHalfOpen(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 2,
		Interval:    10 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 2 && counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Trip the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	assert.Equal(t, StateOpen, cb.State())

	// Wait for half-open
	time.Sleep(150 * time.Millisecond)

	// First request should be allowed in half-open
	err := cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Second request should be allowed (MaxRequests = 2)
	err = cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Circuit should now be closed after MaxRequests successful requests
	assert.Equal(t, StateClosed, cb.State())

	// Additional requests should now work since circuit is closed
	err = cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestCircuitBreakerPanicRecovery(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 1 && counts.ConsecutiveFailures >= 1
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// First verify circuit is in closed state
	assert.Equal(t, StateClosed, cb.State())

	// Test panic recovery
	assert.Panics(t, func() {
		_ = cb.Execute(ctx, func(_ context.Context) error {
			panic("test panic")
		})
	})

	// Circuit should trip due to panic being treated as failure
	assert.Equal(t, StateOpen, cb.State())

	// Test that subsequent requests are rejected (proving the circuit opened due to panic)
	err := cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.Equal(t, ErrCircuitBreakerOpen, err)
}

func TestCircuitBreakerReset(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 2 && counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Trip the circuit
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
		assert.Equal(t, testErr, err)
	}

	assert.Equal(t, StateOpen, cb.State())

	// Reset the circuit
	cb.Reset()

	assert.Equal(t, StateClosed, cb.State())

	counts := cb.Counts()
	assert.Equal(t, uint32(0), counts.Requests)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	assert.Equal(t, uint32(0), counts.TotalSuccesses)
}

func TestCircuitBreakerStateCallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	var stateChanges []string

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 2,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 2 && counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(_ string, from, to CircuitBreakerState) {
			stateChanges = append(stateChanges, fmt.Sprintf("%s->%s", from.String(), to.String()))
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	testErr := errors.New("test error")

	// Trip the circuit (closed -> open)
	for i := 0; i < 2; i++ {
		_ = cb.Execute(ctx, func(_ context.Context) error {
			return testErr
		})
	}

	// Wait for half-open (open -> half-open)
	time.Sleep(150 * time.Millisecond)
	cb.State() // Trigger state check

	// Close circuit (half-open -> closed)
	for i := 0; i < int(config.MaxRequests); i++ {
		_ = cb.Execute(ctx, func(_ context.Context) error {
			return nil
		})
	}

	expected := []string{
		"closed->open",
		"open->half-open",
		"half-open->closed",
	}

	assert.Equal(t, expected, stateChanges)
}

func TestCircuitBreakerCustomIsSuccessful(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	// Consider context.Canceled as success (for graceful shutdowns)
	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 3 && counts.ConsecutiveFailures >= 3
		},
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, context.Canceled)
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Execute with context.Canceled - should be treated as success
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return context.Canceled
		})
		assert.Equal(t, context.Canceled, err)
	}

	// Circuit should remain closed
	assert.Equal(t, StateClosed, cb.State())

	counts := cb.Counts()
	assert.Equal(t, uint32(3), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
}

func TestCircuitBreakerInterval(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 3,
		Interval:    100 * time.Millisecond, // Short interval for testing
		ReadyToTrip: func(_ Counts) bool {
			return false // Never trip
		},
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Execute some requests
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func(_ context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	}

	counts := cb.Counts()
	assert.Equal(t, uint32(3), counts.Requests)

	// Wait for interval to expire
	time.Sleep(150 * time.Millisecond)

	// Execute another request to trigger reset
	err := cb.Execute(ctx, func(_ context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Counts should be reset
	newCounts := cb.Counts()
	assert.Equal(t, uint32(1), newCounts.Requests)
}

func TestCounts(t *testing.T) {
	var counts Counts

	// Test initial state
	assert.Equal(t, uint32(0), counts.Requests)
	assert.Equal(t, float64(0), counts.SuccessRate())
	assert.Equal(t, float64(0), counts.FailureRate())

	// Test success
	counts.OnRequest()
	counts.OnSuccess()

	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalSuccesses)
	assert.Equal(t, uint32(1), counts.ConsecutiveSuccesses)
	assert.Equal(t, uint32(0), counts.ConsecutiveFailures)
	assert.Equal(t, float64(1), counts.SuccessRate())
	assert.Equal(t, float64(0), counts.FailureRate())

	// Test failure
	counts.OnRequest()
	counts.OnFailure()

	assert.Equal(t, uint32(2), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalFailures)
	assert.Equal(t, uint32(0), counts.ConsecutiveSuccesses)
	assert.Equal(t, uint32(1), counts.ConsecutiveFailures)
	assert.Equal(t, float64(0.5), counts.SuccessRate())
	assert.Equal(t, float64(0.5), counts.FailureRate())

	// Test reset
	counts.Reset()
	assert.Equal(t, uint32(0), counts.Requests)
	assert.Equal(t, uint32(0), counts.TotalSuccesses)
	assert.Equal(t, uint32(0), counts.TotalFailures)
}

func TestCircuitBreakerStateString(t *testing.T) {
	// Test all circuit breaker states String() method
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())

	// Test unknown state
	unknownState := CircuitBreakerState(99)
	assert.Equal(t, "unknown", unknownState.String())
}

// Concurrent access tests
func TestCircuitBreakerConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 10,
		Interval:    1 * time.Second,
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Run concurrent requests
	done := make(chan bool)
	errors := make(chan error, 50)

	for i := 0; i < 50; i++ {
		go func(id int) {
			defer func() { done <- true }()

			err := cb.Execute(ctx, func(_ context.Context) error {
				time.Sleep(1 * time.Millisecond) // Simulate work
				if id%10 == 0 {
					return fmt.Errorf("test error %d", id)
				}
				return nil
			})

			// Only report unexpected errors (not our test errors or circuit breaker errors)
			if err != nil && err != ErrCircuitBreakerOpen && !strings.Contains(fmt.Sprintf("%v", err), "test error") {
				errors <- err
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 50; i++ {
		<-done
	}

	// Check for unexpected errors
	close(errors)
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify counts make sense
	counts := cb.Counts()
	assert.GreaterOrEqual(t, counts.Requests, uint32(1))
}

// Benchmark tests
func BenchmarkCircuitBreakerExecute(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "bench-cb",
		MaxRequests: 10,
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(ctx, func(_ context.Context) error {
			return nil
		})
	}
}

func BenchmarkCircuitBreakerExecuteParallel(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()

	config := CircuitBreakerConfig{
		Name:        "bench-cb",
		MaxRequests: 10,
	}

	cb := NewCircuitBreakerWithRegistry(config, logger, registry)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cb.Execute(ctx, func(_ context.Context) error {
				return nil
			})
		}
	})
}
