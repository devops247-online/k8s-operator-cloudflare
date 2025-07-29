package health

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewGracefulShutdownManager(t *testing.T) {
	t.Run("creates manager with correct timeout", func(t *testing.T) {
		timeout := 30 * time.Second
		gsm := NewGracefulShutdownManager(timeout)

		assert.NotNil(t, gsm)
		assert.Equal(t, timeout, gsm.shutdownTimeout)
		assert.NotNil(t, gsm.logger)
		assert.Empty(t, gsm.shutdownCallbacks)
	})

	t.Run("creates manager with different timeouts", func(t *testing.T) {
		testCases := []time.Duration{
			1 * time.Second,
			10 * time.Second,
			1 * time.Minute,
		}

		for _, timeout := range testCases {
			gsm := NewGracefulShutdownManager(timeout)
			assert.Equal(t, timeout, gsm.shutdownTimeout)
		}
	})
}

func TestAddShutdownCallback(t *testing.T) {
	t.Run("adds single callback", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		callback := func(ctx context.Context) error {
			return nil
		}

		gsm.AddShutdownCallback(callback)

		gsm.mu.RLock()
		defer gsm.mu.RUnlock()
		assert.Equal(t, 1, len(gsm.shutdownCallbacks))
	})

	t.Run("adds multiple callbacks", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		callbacks := []func(context.Context) error{
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
			func(ctx context.Context) error { return nil },
		}

		for _, callback := range callbacks {
			gsm.AddShutdownCallback(callback)
		}

		gsm.mu.RLock()
		defer gsm.mu.RUnlock()
		assert.Equal(t, len(callbacks), len(gsm.shutdownCallbacks))
	})

	t.Run("thread safety", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		var wg sync.WaitGroup
		numGoroutines := 10
		numCallbacks := 5

		// Add callbacks concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numCallbacks; j++ {
					gsm.AddShutdownCallback(func(ctx context.Context) error {
						return nil
					})
				}
			}()
		}

		wg.Wait()

		gsm.mu.RLock()
		defer gsm.mu.RUnlock()
		assert.Equal(t, numGoroutines*numCallbacks, len(gsm.shutdownCallbacks))
	})
}

func TestExecuteGracefulShutdown(t *testing.T) {
	t.Run("executes callbacks in reverse order", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		executionOrder := make([]int, 0)
		mu := sync.Mutex{}

		// Add callbacks that record execution order
		for i := 0; i < 3; i++ {
			index := i
			gsm.AddShutdownCallback(func(ctx context.Context) error {
				mu.Lock()
				defer mu.Unlock()
				executionOrder = append(executionOrder, index)
				return nil
			})
		}

		gsm.executeGracefulShutdown()

		// Should execute in reverse order: 2, 1, 0
		expected := []int{2, 1, 0}
		assert.Equal(t, expected, executionOrder)
	})

	t.Run("continues execution even if callback fails", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		executed := make([]bool, 3)
		mu := sync.Mutex{}

		// First callback succeeds
		gsm.AddShutdownCallback(func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			executed[0] = true
			return nil
		})

		// Second callback fails
		gsm.AddShutdownCallback(func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			executed[1] = true
			return errors.New("callback error")
		})

		// Third callback succeeds
		gsm.AddShutdownCallback(func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			executed[2] = true
			return nil
		})

		gsm.executeGracefulShutdown()

		// All callbacks should have been executed despite the error
		for i, wasExecuted := range executed {
			assert.True(t, wasExecuted, "Callback %d should have been executed", i)
		}
	})

	t.Run("respects context timeout", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(100 * time.Millisecond)

		timeoutOccurred := false
		gsm.AddShutdownCallback(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				timeoutOccurred = true
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return nil
			}
		})

		start := time.Now()
		gsm.executeGracefulShutdown()
		elapsed := time.Since(start)

		// Should complete within timeout period
		assert.True(t, elapsed < 150*time.Millisecond, "Should complete within timeout")
		assert.True(t, timeoutOccurred, "Context timeout should have occurred")
	})

	t.Run("handles empty callback list", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		// Should not panic with empty callback list
		assert.NotPanics(t, func() {
			gsm.executeGracefulShutdown()
		})
	})
}

func TestWaitForShutdownSignal(t *testing.T) {
	// Note: Testing signal handling is complex and often requires integration tests
	// We'll test the setup and basic structure here

	t.Run("signal channel setup", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		// This is a structural test - we can't easily test the actual signal handling
		// without sending real signals to the process
		assert.NotNil(t, gsm)
		assert.NotNil(t, gsm.logger)
	})
}

func TestDefaultShutdownCallbacks(t *testing.T) {
	t.Run("returns expected number of callbacks", func(t *testing.T) {
		callbacks := DefaultShutdownCallbacks()

		assert.NotNil(t, callbacks)
		assert.Equal(t, 3, len(callbacks), "Should return 3 default callbacks")
	})

	t.Run("all callbacks are executable", func(t *testing.T) {
		callbacks := DefaultShutdownCallbacks()
		ctx := context.Background()

		for i, callback := range callbacks {
			assert.NotPanics(t, func() {
				err := callback(ctx)
				assert.NoError(t, err, "Default callback %d should not return error", i)
			}, "Default callback %d should not panic", i)
		}
	})

	t.Run("callbacks complete within reasonable time", func(t *testing.T) {
		callbacks := DefaultShutdownCallbacks()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for i, callback := range callbacks {
			start := time.Now()
			err := callback(ctx)
			elapsed := time.Since(start)

			assert.NoError(t, err, "Default callback %d should not return error", i)
			assert.True(t, elapsed < 5*time.Second, "Default callback %d should complete quickly", i)
		}
	})

	t.Run("callbacks handle context cancellation", func(t *testing.T) {
		callbacks := DefaultShutdownCallbacks()
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context immediately
		cancel()

		for i, callback := range callbacks {
			// Should still execute without panicking even with cancelled context
			assert.NotPanics(t, func() {
				_ = callback(ctx)
			}, "Default callback %d should handle cancelled context", i)
		}
	})
}

func TestGracefulShutdownIntegration(t *testing.T) {
	t.Run("full shutdown flow", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(5 * time.Second)

		// Add default callbacks
		for _, callback := range DefaultShutdownCallbacks() {
			gsm.AddShutdownCallback(callback)
		}

		// Add custom callback
		customExecuted := false
		gsm.AddShutdownCallback(func(ctx context.Context) error {
			customExecuted = true
			return nil
		})

		// Execute shutdown
		gsm.executeGracefulShutdown()

		assert.True(t, customExecuted, "Custom callback should have been executed")
	})

	t.Run("concurrent access during shutdown", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(5 * time.Second)

		var wg sync.WaitGroup

		// Add callbacks concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				gsm.AddShutdownCallback(func(ctx context.Context) error {
					time.Sleep(10 * time.Millisecond)
					return nil
				})
				time.Sleep(5 * time.Millisecond)
			}
		}()

		// Execute shutdown concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(25 * time.Millisecond) // Let some callbacks be added
			gsm.executeGracefulShutdown()
		}()

		wg.Wait()

		// Should not panic or deadlock
		assert.True(t, true, "Concurrent access should complete without issues")
	})
}

// Test helper functions
func TestShutdownManagerHelpers(t *testing.T) {
	t.Run("manager state after operations", func(t *testing.T) {
		gsm := NewGracefulShutdownManager(10 * time.Second)

		// Initial state
		gsm.mu.RLock()
		initialCount := len(gsm.shutdownCallbacks)
		gsm.mu.RUnlock()
		assert.Equal(t, 0, initialCount)

		// After adding callbacks
		gsm.AddShutdownCallback(func(ctx context.Context) error { return nil })
		gsm.AddShutdownCallback(func(ctx context.Context) error { return nil })

		gsm.mu.RLock()
		afterAddCount := len(gsm.shutdownCallbacks)
		gsm.mu.RUnlock()
		assert.Equal(t, 2, afterAddCount)

		// After shutdown (callbacks should still be there)
		gsm.executeGracefulShutdown()

		gsm.mu.RLock()
		afterShutdownCount := len(gsm.shutdownCallbacks)
		gsm.mu.RUnlock()
		assert.Equal(t, 2, afterShutdownCount, "Callbacks should remain after shutdown")
	})
}
