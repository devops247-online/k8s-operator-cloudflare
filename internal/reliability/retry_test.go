package reliability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestNewRetryer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultRetryConfig()

	retryer := NewRetryer(config, logger)

	assert.NotNil(t, retryer)
	assert.Equal(t, config.MaxRetries, retryer.config.MaxRetries)
	assert.Equal(t, config.InitialDelay, retryer.config.InitialDelay)
	assert.Equal(t, config.MaxDelay, retryer.config.MaxDelay)
	assert.Equal(t, config.Multiplier, retryer.config.Multiplier)
	assert.Equal(t, config.Jitter, retryer.config.Jitter)
	assert.NotNil(t, retryer.config.RetryableChecker)
}

func TestNewRetryerWithDefaults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := RetryConfig{
		MaxRetries: -1, // Should be corrected to 0
		// Other fields left as zero values
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)

	assert.Equal(t, 0, retryer.config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, retryer.config.InitialDelay)
	assert.Equal(t, 30*time.Second, retryer.config.MaxDelay)
	assert.Equal(t, 2.0, retryer.config.Multiplier)
	assert.NotNil(t, retryer.config.RetryableChecker)
}

func TestRetrySuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       false, // Disable jitter for predictable testing
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return nil // Success on first attempt
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestRetrySuccessAfterFailures(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			return errors.New("temporary error")
		}
		return nil // Success on third attempt
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestRetryFailureAfterMaxRetries(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	testErr := errors.New("persistent error")

	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return testErr
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, testErr))
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount)) // 1 + 2 retries
	assert.Contains(t, err.Error(), "operation failed after 3 attempts")
}

func TestRetryNonRetryableError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		RetryableChecker: func(err error) bool {
			return !strings.Contains(err.Error(), "non-retryable")
		},
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	testErr := errors.New("non-retryable error")

	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return testErr
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, testErr))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount)) // Only one attempt
}

func TestRetryContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	callCount := int32(0)
	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return errors.New("always fails")
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	// Should have made at least one attempt, possibly more depending on timing
	assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(1))
}

func TestRetryContextCancelledInFunction(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx, cancel := context.WithCancel(context.Background())

	callCount := int32(0)
	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count == 2 {
			cancel() // Cancel context on second attempt
		}
		return errors.New("error")
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestCalculateDelay(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, nil)

	tests := []struct {
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{1, 100 * time.Millisecond, 100 * time.Millisecond},
		{2, 200 * time.Millisecond, 200 * time.Millisecond},
		{3, 400 * time.Millisecond, 400 * time.Millisecond},
		{4, 800 * time.Millisecond, 800 * time.Millisecond},
		{10, 10 * time.Second, 10 * time.Second}, // Should be capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := retryer.calculateDelay(tt.attempt)
			assert.GreaterOrEqual(t, delay, tt.expectedMin)
			assert.LessOrEqual(t, delay, tt.expectedMax)
		})
	}
}

func TestCalculateDelayWithJitter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	retryer := NewRetryerWithRegistry(config, logger, nil)

	// Test that jitter produces different values
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = retryer.calculateDelay(2) // Second attempt
	}

	// Check that we got some variation (not all delays are identical)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}
	assert.False(t, allSame, "Jitter should produce different delay values")

	// All delays should be positive and within reasonable bounds
	expectedBase := 200 * time.Millisecond
	expectedMin := time.Duration(float64(expectedBase) * 0.75) // Base - 25% jitter
	expectedMax := time.Duration(float64(expectedBase) * 1.25) // Base + 25% jitter

	for i, delay := range delays {
		assert.Greater(t, delay, time.Duration(0), "Delay %d should be positive", i)
		assert.GreaterOrEqual(t, delay, expectedMin, "Delay %d should be >= min expected", i)
		assert.LessOrEqual(t, delay, expectedMax, "Delay %d should be <= max expected", i)
	}
}

func TestRetryOnRetryCallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	callbackCalls := []struct {
		attempt int
		delay   time.Duration
		err     error
	}{}

	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
		OnRetry: func(attempt int, delay time.Duration, err error) {
			callbackCalls = append(callbackCalls, struct {
				attempt int
				delay   time.Duration
				err     error
			}{attempt, delay, err})
		},
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	testErr := errors.New("test error")

	err := retryer.Retry(ctx, "test_operation", func(_ context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			return testErr
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	assert.Len(t, callbackCalls, 2) // Should be called before each retry

	// Check callback parameters
	assert.Equal(t, 1, callbackCalls[0].attempt)
	assert.Equal(t, 10*time.Millisecond, callbackCalls[0].delay)
	assert.Equal(t, testErr, callbackCalls[0].err)

	assert.Equal(t, 2, callbackCalls[1].attempt)
	assert.Equal(t, 20*time.Millisecond, callbackCalls[1].delay)
	assert.Equal(t, testErr, callbackCalls[1].err)
}

func TestRetryWithBackoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := DefaultRetryConfig()
	config.InitialDelay = 1 * time.Millisecond
	config.MaxDelay = 10 * time.Millisecond

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	callCount := int32(0)
	err := retryer.RetryWithBackoff(ctx, func(_ context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestRetryConfigurations(t *testing.T) {
	t.Run("DefaultRetryConfig", func(t *testing.T) {
		config := DefaultRetryConfig()
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 100*time.Millisecond, config.InitialDelay)
		assert.Equal(t, 30*time.Second, config.MaxDelay)
		assert.Equal(t, 2.0, config.Multiplier)
		assert.Equal(t, true, config.Jitter)
	})

	t.Run("AggressiveRetryConfig", func(t *testing.T) {
		config := AggressiveRetryConfig()
		assert.Equal(t, 5, config.MaxRetries)
		assert.Equal(t, 50*time.Millisecond, config.InitialDelay)
		assert.Equal(t, 10*time.Second, config.MaxDelay)
		assert.Equal(t, 1.5, config.Multiplier)
		assert.Equal(t, true, config.Jitter)
	})

	t.Run("ConservativeRetryConfig", func(t *testing.T) {
		config := ConservativeRetryConfig()
		assert.Equal(t, 2, config.MaxRetries)
		assert.Equal(t, 500*time.Millisecond, config.InitialDelay)
		assert.Equal(t, 60*time.Second, config.MaxDelay)
		assert.Equal(t, 3.0, config.Multiplier)
		assert.Equal(t, true, config.Jitter)
	})

	t.Run("CloudflareRetryConfig", func(t *testing.T) {
		config := CloudflareRetryConfig()
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 200*time.Millisecond, config.InitialDelay)
		assert.Equal(t, 30*time.Second, config.MaxDelay)
		assert.Equal(t, 2.0, config.Multiplier)
		assert.Equal(t, true, config.Jitter)
		assert.NotNil(t, config.RetryableChecker)
	})
}

func TestDefaultRetryableChecker(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context cancelled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"generic error", errors.New("some error"), true},
		{"network error", errors.New("connection refused"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultRetryableChecker(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloudflareRetryableChecker(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context cancelled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"connection refused", errors.New("connection refused"), true},
		{"timeout", errors.New("timeout occurred"), true},
		{"500 error", errors.New("HTTP 500 internal server error"), true},
		{"502 error", errors.New("HTTP 502 bad gateway"), true},
		{"503 error", errors.New("HTTP 503 service unavailable"), true},
		{"504 error", errors.New("HTTP 504 gateway timeout"), true},
		{"429 rate limit", errors.New("HTTP 429 too many requests"), true},
		{"rate limit text", errors.New("rate limit exceeded"), true},
		{"401 unauthorized", errors.New("HTTP 401 unauthorized"), false},
		{"403 forbidden", errors.New("HTTP 403 forbidden"), false},
		{"400 bad request", errors.New("HTTP 400 bad request"), false},
		{"404 not found", errors.New("HTTP 404 not found"), false},
		{"422 unprocessable", errors.New("HTTP 422 unprocessable entity"), false},
		{"unknown error", errors.New("some unknown error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cloudflareRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryerMethods(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := DefaultRetryConfig()
	retryer := NewRetryerWithRegistry(config, logger, registry)

	t.Run("Config", func(t *testing.T) {
		retrievedConfig := retryer.Config()
		assert.Equal(t, config.MaxRetries, retrievedConfig.MaxRetries)
		assert.Equal(t, config.InitialDelay, retrievedConfig.InitialDelay)
		assert.Equal(t, config.MaxDelay, retrievedConfig.MaxDelay)
		assert.Equal(t, config.Multiplier, retrievedConfig.Multiplier)
		assert.Equal(t, config.Jitter, retrievedConfig.Jitter)
	})

	t.Run("SetRetryableChecker", func(t *testing.T) {
		// Create a separate registry and retryer for this test
		testRegistry := prometheus.NewRegistry()
		testConfig := RetryConfig{
			MaxRetries:   1,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
			Jitter:       false,
		}
		testRetryer := NewRetryerWithRegistry(testConfig, logger, testRegistry)

		// Before setting custom checker - should retry by default
		ctx := context.Background()
		callCount1 := int32(0)

		err1 := testRetryer.Retry(ctx, "test_default", func(_ context.Context) error {
			atomic.AddInt32(&callCount1, 1)
			return errors.New("some error")
		})

		assert.Error(t, err1)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount1)) // Should retry once (1 + 1 retry)

		// Set custom checker that never retries
		testRetryer.SetRetryableChecker(func(_ error) bool {
			return false // Never retry
		})

		// After setting custom checker - should not retry
		callCount2 := int32(0)
		err2 := testRetryer.Retry(ctx, "test_custom", func(_ context.Context) error {
			atomic.AddInt32(&callCount2, 1)
			return errors.New("another error")
		})

		assert.Error(t, err2)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount2)) // Should not retry
	})

	t.Run("SetOnRetry", func(t *testing.T) {
		// Create a separate registry and retryer for this test
		testRegistry := prometheus.NewRegistry()
		testRetryer := NewRetryerWithRegistry(DefaultRetryConfig(), logger, testRegistry)

		callbackCalled := int32(0)

		testRetryer.SetOnRetry(func(_ int, _ time.Duration, _ error) {
			atomic.AddInt32(&callbackCalled, 1)
		})

		ctx := context.Background()
		_ = testRetryer.Retry(ctx, "test", func(_ context.Context) error {
			return errors.New("retryable error")
		})

		assert.Greater(t, atomic.LoadInt32(&callbackCalled), int32(0))
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("RetryOnError", func(t *testing.T) {
		ctx := context.Background()
		callCount := int32(0)

		err := RetryOnError(ctx, func(_ context.Context) error {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				return errors.New("temporary error")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})

	t.Run("RetryCloudflareOperation", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		ctx := context.Background()
		callCount := int32(0)

		err := RetryCloudflareOperation(ctx, logger, "cloudflare_api", func(_ context.Context) error {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				return errors.New("HTTP 503 service unavailable")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})

	t.Run("RetryWithConfig", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		ctx := context.Background()
		config := RetryConfig{
			MaxRetries:   1,
			InitialDelay: 1 * time.Millisecond,
		}
		callCount := int32(0)

		err := RetryWithConfig(ctx, config, logger, "custom_operation", func(_ context.Context) error {
			count := atomic.AddInt32(&callCount, 1)
			if count < 2 {
				return errors.New("temporary error")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
	})
}

func TestStringHelpers(t *testing.T) {
	t.Run("contains", func(t *testing.T) {
		assert.True(t, contains("hello world", "hello"))
		assert.True(t, contains("hello world", "world"))
		assert.True(t, contains("hello world", "lo wo"))
		assert.False(t, contains("hello world", "goodbye"))
		assert.False(t, contains("short", "very long string"))
		assert.True(t, contains("test", "test"))
	})

	t.Run("indexInString", func(t *testing.T) {
		assert.Equal(t, 0, indexInString("hello world", "hello"))
		assert.Equal(t, 6, indexInString("hello world", "world"))
		assert.Equal(t, 3, indexInString("hello world", "lo wo"))
		assert.Equal(t, -1, indexInString("hello world", "goodbye"))
		assert.Equal(t, -1, indexInString("short", "very long string"))
		assert.Equal(t, 0, indexInString("test", "test"))
	})
}

// Concurrent access tests
func TestRetryConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Run concurrent retry operations
	done := make(chan bool)
	errors := make(chan error, 50)

	for i := 0; i < 50; i++ {
		go func(id int) {
			defer func() { done <- true }()

			callCount := int32(0)
			err := retryer.Retry(ctx, fmt.Sprintf("operation_%d", id), func(_ context.Context) error {
				count := atomic.AddInt32(&callCount, 1)
				if count < 2 {
					return fmt.Errorf("temporary error from goroutine %d", id)
				}
				return nil
			})

			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 50; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Concurrent retry error: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	close(errors)
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}
}

// Benchmark tests
func BenchmarkRetrySuccess(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Millisecond,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryer.Retry(ctx, "bench_operation", func(_ context.Context) error {
			return nil // Always succeed
		})
	}
}

func BenchmarkRetryWithFailures(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()
	config := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		Jitter:       false,
	}

	retryer := NewRetryerWithRegistry(config, logger, registry)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callCount := 0
		_ = retryer.Retry(ctx, "bench_operation", func(_ context.Context) error {
			callCount++
			if callCount < 2 {
				return errors.New("temporary error")
			}
			return nil
		})
	}
}

// Edge case tests
func TestRetryEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("zero retries", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RetryConfig{
			MaxRetries:   0,
			InitialDelay: 10 * time.Millisecond,
		}

		retryer := NewRetryerWithRegistry(config, logger, registry)
		ctx := context.Background()

		callCount := int32(0)
		err := retryer.Retry(ctx, "test", func(_ context.Context) error {
			atomic.AddInt32(&callCount, 1)
			return errors.New("always fails")
		})

		assert.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&callCount)) // Only one attempt
	})

	t.Run("very large delays", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RetryConfig{
			MaxRetries:   1,
			InitialDelay: 1 * time.Hour,
			MaxDelay:     1 * time.Second, // Should cap the delay
			Multiplier:   2.0,
			Jitter:       false,
		}

		retryer := NewRetryerWithRegistry(config, logger, registry)

		delay := retryer.calculateDelay(1)
		assert.Equal(t, 1*time.Second, delay) // Should be capped
	})

	t.Run("negative jitter edge case", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RetryConfig{
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
			Jitter:       true,
		}

		retryer := NewRetryerWithRegistry(config, logger, registry)

		// Run many times to try to trigger edge cases with jitter
		for i := 0; i < 100; i++ {
			delay := retryer.calculateDelay(1)
			assert.Greater(t, delay, time.Duration(0))
			assert.LessOrEqual(t, delay, config.MaxDelay)
		}
	})
}
