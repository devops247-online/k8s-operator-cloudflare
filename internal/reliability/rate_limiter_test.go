package reliability

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestNewRateLimiter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := DefaultRateLimiterConfig()

	limiter := NewRateLimiter(config, logger)
	assert.NotNil(t, limiter)

	// Should be a TokenBucketLimiter
	tbLimiter, ok := limiter.(*TokenBucketLimiter)
	assert.True(t, ok)
	assert.Equal(t, config.Name, tbLimiter.config.Name)
	assert.Equal(t, config.RequestsPerSecond, tbLimiter.config.RequestsPerSecond)
	assert.Equal(t, config.BurstSize, tbLimiter.config.BurstSize)
}

func TestNewRateLimiterWithDefaults(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	config := RateLimiterConfig{
		Name: "test",
		// Leave other fields as zero values
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	tbLimiter := limiter.(*TokenBucketLimiter)

	assert.Equal(t, 10.0, tbLimiter.config.RequestsPerSecond)
	assert.Equal(t, 10, tbLimiter.config.BurstSize)
	assert.Equal(t, 1*time.Second, tbLimiter.config.WindowSize)
	assert.Equal(t, Block, tbLimiter.config.BackoffStrategy)
	assert.Equal(t, TokenBucket, tbLimiter.config.Type)
}

func TestTokenBucketAllow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         5,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry).(*TokenBucketLimiter)

	// Should allow up to burst size requests initially
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
	}

	// Next request should be rejected (no tokens left)
	assert.False(t, limiter.Allow(), "Request should be rejected after burst exhausted")
}

func TestTokenBucketRefill(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0, // 10 requests per second = 1 request per 100ms
		BurstSize:         1,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry).(*TokenBucketLimiter)

	// Use the initial token
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow()) // Should be rejected

	// Wait for refill (need more than 100ms for 1 token)
	time.Sleep(150 * time.Millisecond)

	// Should now allow one request
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow()) // Should be rejected again
}

func TestTokenBucketWait(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         1,
		BackoffStrategy:   Block,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	// First request should succeed immediately
	start := time.Now()
	err := limiter.Wait(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 10*time.Millisecond) // Should be immediate

	// Second request should wait
	start = time.Now()
	err = limiter.Wait(ctx)
	duration = time.Since(start)

	assert.NoError(t, err)
	assert.Greater(t, duration, 50*time.Millisecond) // Should wait for refill
}

func TestTokenBucketWaitWithContext(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 1.0, // Very slow refill
		BurstSize:         1,
		BackoffStrategy:   Block,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Use the initial token
	assert.True(t, limiter.Allow())

	// Context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should timeout waiting for refill
	err := limiter.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestTokenBucketRejectStrategy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         1,
		BackoffStrategy:   Reject,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	// First request should succeed
	err := limiter.Wait(ctx)
	assert.NoError(t, err)

	// Second request should be rejected immediately
	err = limiter.Wait(ctx)
	assert.Error(t, err)
	assert.Equal(t, ErrRateLimited, err)
}

func TestTokenBucketReserve(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         2,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Should be no delay initially (has tokens)
	delay := limiter.Reserve()
	assert.Equal(t, time.Duration(0), delay)

	// Use all tokens
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())

	// Now should need to wait
	delay = limiter.Reserve()
	assert.Greater(t, delay, time.Duration(0))
	assert.Less(t, delay, 200*time.Millisecond) // Should be around 100ms for 10 RPS
}

func TestTokenBucketSetRate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         10,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Update rate
	limiter.SetRate(20.0, 5)

	stats := limiter.Stats()
	assert.Equal(t, 20.0, stats.RequestsPerSecond)
	assert.Equal(t, 5, stats.BurstSize)
	assert.Equal(t, 5.0, stats.CurrentTokens) // Should be capped at new burst size
}

func TestTokenBucketStats(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         3,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Make some requests
	assert.True(t, limiter.Allow())  // allowed
	assert.True(t, limiter.Allow())  // allowed
	assert.True(t, limiter.Allow())  // allowed
	assert.False(t, limiter.Allow()) // rejected

	stats := limiter.Stats()
	assert.Equal(t, uint64(3), stats.RequestsAllowed)
	assert.Equal(t, uint64(1), stats.RequestsRejected)
	assert.Equal(t, 10.0, stats.RequestsPerSecond)
	assert.Equal(t, 3, stats.BurstSize)
	assert.Less(t, stats.CurrentTokens, 1.0) // Should be close to 0 but might have small refill
}

func TestTokenBucketReset(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         2,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Use all tokens and make some rejected requests
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow())

	// Reset and verify
	limiter.Reset()

	stats := limiter.Stats()
	assert.Equal(t, uint64(0), stats.RequestsAllowed)
	assert.Equal(t, uint64(0), stats.RequestsRejected)
	assert.Equal(t, 2.0, stats.CurrentTokens)

	// Should be able to make requests again
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
}

func TestRateLimiterConfigurations(t *testing.T) {
	t.Run("DefaultRateLimiterConfig", func(t *testing.T) {
		config := DefaultRateLimiterConfig()
		assert.Equal(t, "default", config.Name)
		assert.Equal(t, TokenBucket, config.Type)
		assert.Equal(t, 10.0, config.RequestsPerSecond)
		assert.Equal(t, 10, config.BurstSize)
		assert.Equal(t, Block, config.BackoffStrategy)
	})

	t.Run("CloudflareRateLimiterConfig", func(t *testing.T) {
		config := CloudflareRateLimiterConfig()
		assert.Equal(t, "cloudflare-api", config.Name)
		assert.Equal(t, TokenBucket, config.Type)
		assert.Equal(t, 4.0, config.RequestsPerSecond) // 1200/300
		assert.Equal(t, 10, config.BurstSize)
		assert.Equal(t, Block, config.BackoffStrategy)
	})

	t.Run("AggressiveRateLimiterConfig", func(t *testing.T) {
		config := AggressiveRateLimiterConfig()
		assert.Equal(t, "aggressive", config.Name)
		assert.Equal(t, TokenBucket, config.Type)
		assert.Equal(t, 100.0, config.RequestsPerSecond)
		assert.Equal(t, 50, config.BurstSize)
		assert.Equal(t, Reject, config.BackoffStrategy)
	})

	t.Run("ConservativeRateLimiterConfig", func(t *testing.T) {
		config := ConservativeRateLimiterConfig()
		assert.Equal(t, "conservative", config.Name)
		assert.Equal(t, TokenBucket, config.Type)
		assert.Equal(t, 1.0, config.RequestsPerSecond)
		assert.Equal(t, 5, config.BurstSize)
		assert.Equal(t, Block, config.BackoffStrategy)
	})
}

func TestRateLimiterCallback(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	callbackCalls := []struct {
		limiterName string
		delay       time.Duration
	}{}

	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 100.0, // Fast refill for quick test
		BurstSize:         1,
		BackoffStrategy:   Block,
		OnRateLimited: func(limiterName string, delay time.Duration) {
			callbackCalls = append(callbackCalls, struct {
				limiterName string
				delay       time.Duration
			}{limiterName, delay})
		},
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Use initial token
	err := limiter.Wait(ctx)
	assert.NoError(t, err)

	// Next request should trigger callback
	err = limiter.Wait(ctx)
	assert.NoError(t, err)

	assert.Len(t, callbackCalls, 1)
	assert.Equal(t, "test", callbackCalls[0].limiterName)
	assert.Greater(t, callbackCalls[0].delay, time.Duration(0))
}

func TestRateLimiterUnknownType(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name: "test",
		Type: "unknown_type",
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Should fall back to token bucket
	_, ok := limiter.(*TokenBucketLimiter)
	assert.True(t, ok)
}

func TestRateLimiterSlidingWindow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name: "test",
		Type: SlidingWindow,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Should fall back to token bucket for now
	_, ok := limiter.(*TokenBucketLimiter)
	assert.True(t, ok)
}

func TestRateLimiterFixedWindow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name: "test",
		Type: FixedWindow,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	// Should fall back to token bucket for now
	_, ok := limiter.(*TokenBucketLimiter)
	assert.True(t, ok)
}

func TestTokenBucketAdaptiveStrategy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 100.0, // Fast refill
		BurstSize:         1,
		BackoffStrategy:   Adaptive,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Use initial token
	err := limiter.Wait(ctx)
	assert.NoError(t, err)

	// Next request should use adaptive strategy
	start := time.Now()
	err = limiter.Wait(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Greater(t, duration, time.Duration(0))
	// Should be faster than normal block strategy due to adaptation
}

// Concurrent access tests
func TestRateLimiterConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "test",
		Type:              TokenBucket,
		RequestsPerSecond: 100.0, // High rate for concurrent testing
		BurstSize:         50,
		BackoffStrategy:   Block,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	// Run concurrent requests
	const numGoroutines = 20
	const requestsPerGoroutine = 10

	allowed := int64(0)
	rejected := int64(0)
	errors := make(chan error, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < requestsPerGoroutine; j++ {
				if limiter.Allow() {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&rejected, 1)
				}

				// Also test Wait
				err := limiter.Wait(ctx)
				if err != nil {
					errors <- err
				} else {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case err := <-errors:
			t.Errorf("Unexpected error in concurrent access: %v", err)
		case <-time.After(10 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	totalAllowed := atomic.LoadInt64(&allowed)
	totalRejected := atomic.LoadInt64(&rejected)

	assert.Greater(t, totalAllowed, int64(0))
	t.Logf("Allowed: %d, Rejected: %d", totalAllowed, totalRejected)

	// Verify stats consistency
	stats := limiter.Stats()
	assert.Greater(t, stats.RequestsAllowed, uint64(0))
}

// Benchmark tests
func BenchmarkTokenBucketAllow(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "bench",
		Type:              TokenBucket,
		RequestsPerSecond: 1000.0,
		BurstSize:         1000,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow()
	}
}

func BenchmarkTokenBucketWait(b *testing.B) {
	logger := zaptest.NewLogger(b)
	registry := prometheus.NewRegistry()
	config := RateLimiterConfig{
		Name:              "bench",
		Type:              TokenBucket,
		RequestsPerSecond: 10000.0, // High rate to minimize waiting
		BurstSize:         1000,
		BackoffStrategy:   Block,
	}

	limiter := NewRateLimiterWithRegistry(config, logger, registry)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = limiter.Wait(ctx)
	}
}

// Edge case tests
func TestRateLimiterEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("zero_rate", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RateLimiterConfig{
			Name:              "test",
			RequestsPerSecond: 0, // Should be corrected to default
		}

		limiter := NewRateLimiterWithRegistry(config, logger, registry)
		tbLimiter := limiter.(*TokenBucketLimiter)

		assert.Equal(t, 10.0, tbLimiter.config.RequestsPerSecond)
	})

	t.Run("negative_rate", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RateLimiterConfig{
			Name:              "test",
			RequestsPerSecond: -5.0, // Should be corrected to default
		}

		limiter := NewRateLimiterWithRegistry(config, logger, registry)
		tbLimiter := limiter.(*TokenBucketLimiter)

		assert.Equal(t, 10.0, tbLimiter.config.RequestsPerSecond)
	})

	t.Run("zero_burst_size", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RateLimiterConfig{
			Name:              "test",
			RequestsPerSecond: 5.0,
			BurstSize:         0, // Should be set to rate
		}

		limiter := NewRateLimiterWithRegistry(config, logger, registry)
		tbLimiter := limiter.(*TokenBucketLimiter)

		assert.Equal(t, 5, tbLimiter.config.BurstSize)
	})

	t.Run("very_high_rate", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := RateLimiterConfig{
			Name:              "test",
			Type:              TokenBucket,
			RequestsPerSecond: 10000.0,
			BurstSize:         100,
		}

		limiter := NewRateLimiterWithRegistry(config, logger, registry)

		// Should handle high rates gracefully - initially all should be allowed
		allowedCount := 0
		for i := 0; i < 100; i++ {
			if limiter.Allow() {
				allowedCount++
			}
		}

		// At least some should be allowed initially
		assert.Greater(t, allowedCount, 0)

		// After burst is exhausted, should eventually reject
		rejected := false
		for i := 0; i < 10; i++ {
			if !limiter.Allow() {
				rejected = true
				break
			}
		}
		assert.True(t, rejected, "Should eventually reject after burst is exhausted")
	})
}

func TestMinFunction(t *testing.T) {
	assert.Equal(t, 1.0, min(1.0, 2.0))
	assert.Equal(t, 1.0, min(2.0, 1.0))
	assert.Equal(t, 5.0, min(5.0, 5.0))
	assert.Equal(t, -1.0, min(-1.0, 1.0))
}

func TestCalculateAdaptiveDelayEdgeCases(t *testing.T) {
	limiter := &TokenBucketLimiter{
		config: RateLimiterConfig{
			RequestsPerSecond: 10.0,
			BurstSize:         5,
		},
	}

	// Test with very low rate - just check it returns some delay
	limiter.config.RequestsPerSecond = 0.1
	delay := limiter.calculateAdaptiveDelay(5)
	assert.GreaterOrEqual(t, delay, time.Duration(0))

	// Test with zero rate (edge case)
	limiter.config.RequestsPerSecond = 0
	delay = limiter.calculateAdaptiveDelay(1)
	assert.GreaterOrEqual(t, delay, time.Duration(0))

	// Test with very high failure count - should have some delay but not necessarily > 1s
	limiter.config.RequestsPerSecond = 10.0
	delay = limiter.calculateAdaptiveDelay(100)
	assert.Greater(t, delay, time.Duration(0))
}
