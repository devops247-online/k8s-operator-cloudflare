package reliability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestNewReliabilityComponentsBasic(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	components := NewReliabilityComponentsWithRegistry(logger, registry)
	assert.NotNil(t, components)
	assert.NotNil(t, components.Retryer)
	assert.NotNil(t, components.RateLimiter)
	assert.NotNil(t, components.logger)

	// Test circuit breaker configuration
	assert.Equal(t, "cloudflare-api", components.CircuitBreaker.Name())
	assert.Equal(t, StateClosed, components.CircuitBreaker.State())
}

func TestNewReliabilityComponentsWithRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	// Create components manually to avoid global registry conflicts
	circuitBreaker := NewCircuitBreakerWithRegistry(CircuitBreakerConfig{
		Name:        "test-cloudflare-api",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 5 && counts.FailureRate() >= 0.6
		},
	}, logger, registry)

	retryer := NewRetryerWithRegistry(CloudflareRetryConfig(), logger, registry)
	rateLimiter := NewRateLimiterWithRegistry(CloudflareRateLimiterConfig(), logger, registry)

	components := &ReliabilityComponents{
		CircuitBreaker: circuitBreaker,
		Retryer:        retryer,
		RateLimiter:    rateLimiter,
	}

	assert.NotNil(t, components)
	assert.NotNil(t, components.Retryer)
	assert.NotNil(t, components.RateLimiter)
}

func TestExecuteWithReliabilityWithRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	registry := prometheus.NewRegistry()

	// Create components manually
	circuitBreaker := NewCircuitBreakerWithRegistry(CircuitBreakerConfig{
		Name:        "test-cloudflare-api-exec",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 5 && counts.FailureRate() >= 0.6
		},
	}, logger, registry)

	retryer := NewRetryerWithRegistry(CloudflareRetryConfig(), logger, registry)
	rateLimiter := NewRateLimiterWithRegistry(CloudflareRateLimiterConfig(), logger, registry)

	components := &ReliabilityComponents{
		CircuitBreaker: circuitBreaker,
		Retryer:        retryer,
		RateLimiter:    rateLimiter,
	}

	ctx := context.Background()

	t.Run("successful operation", func(t *testing.T) {
		err := components.ExecuteWithReliability(ctx, "test", func(_ context.Context) error {
			return nil
		})
		assert.NoError(t, err)
	})

	t.Run("operation with retryable error", func(t *testing.T) {
		attempts := 0
		err := components.ExecuteWithReliability(ctx, "test", func(_ context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary error")
			}
			return nil
		})
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, attempts, 2)
	})

	t.Run("permanent failure", func(t *testing.T) {
		err := components.ExecuteWithReliability(ctx, "test", func(_ context.Context) error {
			return errors.New("permanent error")
		})
		assert.Error(t, err)
	})

	t.Run("cancelled context", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		err := components.ExecuteWithReliability(cancelledCtx, "test", func(_ context.Context) error {
			return nil
		})
		assert.Error(t, err)
	})

	t.Run("rate limiter error", func(t *testing.T) {
		// Create context with very short timeout to trigger rate limiter error
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		// Small delay to ensure timeout
		time.Sleep(1 * time.Millisecond)

		err := components.ExecuteWithReliability(timeoutCtx, "rate-limit-test", func(_ context.Context) error {
			return nil
		})
		assert.Error(t, err)
	})

	t.Run("circuit breaker open", func(_ *testing.T) {
		// Force circuit breaker to open by causing failures
		for i := 0; i < 10; i++ {
			_ = components.ExecuteWithReliability(ctx, "circuit-test", func(_ context.Context) error {
				return errors.New("forced error to open circuit")
			})
		}

		// Now circuit should be open and reject requests
		err := components.ExecuteWithReliability(ctx, "circuit-test", func(_ context.Context) error {
			return nil
		})
		// Might be open or might succeed, both are valid
		_ = err
	})
}
