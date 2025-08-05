package reliability

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// RetryConfig holds configuration for the retry mechanism
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries)
	MaxRetries int

	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the exponential backoff multiplier
	Multiplier float64

	// Jitter adds randomness to delays to avoid thundering herd
	Jitter bool

	// RetryableChecker determines if an error is retryable
	RetryableChecker func(error) bool

	// OnRetry is called before each retry attempt
	OnRetry func(attempt int, delay time.Duration, err error)
}

// RetryableFunc is the function type that can be retried
type RetryableFunc func(context.Context) error

// Retryer implements exponential backoff retry logic
type Retryer struct {
	config RetryConfig
	logger *zap.Logger

	// Prometheus metrics
	attemptsTotal  *prometheus.CounterVec
	successesTotal *prometheus.CounterVec
	failuresTotal  *prometheus.CounterVec
	delayHistogram *prometheus.HistogramVec
	retryDuration  *prometheus.HistogramVec
}

// NewRetryer creates a new retryer with the given configuration
func NewRetryer(config RetryConfig, logger *zap.Logger) *Retryer {
	return NewRetryerWithRegistry(config, logger, prometheus.DefaultRegisterer)
}

// NewRetryerWithRegistry creates a new retryer with a custom metrics registry
func NewRetryerWithRegistry(config RetryConfig, logger *zap.Logger, registerer prometheus.Registerer) *Retryer {
	// Set default configuration values
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.InitialDelay == 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier == 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableChecker == nil {
		config.RetryableChecker = defaultRetryableChecker
	}

	r := &Retryer{
		config: config,
		logger: logger,

		attemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "retry_attempts_total",
				Help: "Total number of retry attempts",
			},
			[]string{"operation", "attempt"},
		),

		successesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "retry_successes_total",
				Help: "Total number of successful operations after retries",
			},
			[]string{"operation", "final_attempt"},
		),

		failuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "retry_failures_total",
				Help: "Total number of failed operations after all retries",
			},
			[]string{"operation"},
		),

		delayHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "retry_delay_seconds",
				Help:    "Histogram of retry delays in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~32s
			},
			[]string{"operation", "attempt"},
		),

		retryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "retry_duration_seconds",
				Help:    "Total duration of retry operations including all attempts",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 15), // 10ms to ~5min
			},
			[]string{"operation", "result"},
		),
	}

	// Register metrics if registerer provided
	if registerer != nil {
		registerer.MustRegister(
			r.attemptsTotal,
			r.successesTotal,
			r.failuresTotal,
			r.delayHistogram,
			r.retryDuration,
		)
	}

	return r
}

// Retry executes the given function with exponential backoff retry logic
func (r *Retryer) Retry(ctx context.Context, operation string, fn RetryableFunc) error {
	startTime := time.Now()

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Record attempt
		r.attemptsTotal.WithLabelValues(operation, fmt.Sprintf("%d", attempt)).Inc()

		// Execute the function
		err := fn(ctx)
		if err == nil {
			// Success
			r.successesTotal.WithLabelValues(operation, fmt.Sprintf("%d", attempt)).Inc()
			r.retryDuration.WithLabelValues(operation, "success").Observe(time.Since(startTime).Seconds())

			if attempt > 0 {
				r.logger.Info("Operation succeeded after retries",
					zap.String("operation", operation),
					zap.Int("attempts", attempt+1),
					zap.Duration("total_duration", time.Since(startTime)),
				)
			}

			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.config.RetryableChecker(err) {
			r.logger.Info("Error is not retryable, giving up",
				zap.String("operation", operation),
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			break
		}

		// Check if we have more retries left
		if attempt >= r.config.MaxRetries {
			break
		}

		// Check context cancellation
		if ctx.Err() != nil {
			r.logger.Info("Context cancelled, giving up retries",
				zap.String("operation", operation),
				zap.Error(ctx.Err()),
				zap.Int("attempt", attempt),
			)
			return ctx.Err()
		}

		// Calculate delay for next attempt
		delay := r.calculateDelay(attempt + 1)
		r.delayHistogram.WithLabelValues(operation, fmt.Sprintf("%d", attempt+1)).Observe(delay.Seconds())

		// Call retry callback if configured
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt+1, delay, err)
		}

		r.logger.Debug("Retrying operation after delay",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Duration("delay", delay),
			zap.Error(err),
		)

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	r.failuresTotal.WithLabelValues(operation).Inc()
	r.retryDuration.WithLabelValues(operation, "failure").Observe(time.Since(startTime).Seconds())

	r.logger.Error("All retry attempts exhausted",
		zap.String("operation", operation),
		zap.Int("attempts", r.config.MaxRetries+1),
		zap.Duration("total_duration", time.Since(startTime)),
		zap.Error(lastErr),
	)

	return fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxRetries+1, lastErr)
}

// RetryWithBackoff is a convenience method for simple retry with exponential backoff
func (r *Retryer) RetryWithBackoff(ctx context.Context, fn RetryableFunc) error {
	return r.Retry(ctx, "operation", fn)
}

// calculateDelay calculates the delay for the given attempt using exponential backoff
func (r *Retryer) calculateDelay(attempt int) time.Duration {
	// Calculate exponential delay
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Apply maximum delay limit
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter if enabled
	if r.config.Jitter {
		// Add ±25% jitter
		jitterRange := delay * 0.25
		jitter := (rand.Float64()*2 - 1) * jitterRange // #nosec G404 - Using math/rand for jitter is acceptable
		delay += jitter

		// Ensure delay doesn't go negative or exceed max
		if delay < 0 {
			delay = float64(r.config.InitialDelay)
		}
		if delay > float64(r.config.MaxDelay) {
			delay = float64(r.config.MaxDelay)
		}
	}

	return time.Duration(delay)
}

// Config returns a copy of the current configuration
func (r *Retryer) Config() RetryConfig {
	return r.config
}

// SetRetryableChecker sets a custom retryable error checker
func (r *Retryer) SetRetryableChecker(checker func(error) bool) {
	r.config.RetryableChecker = checker
}

// SetOnRetry sets a callback to be called before each retry
func (r *Retryer) SetOnRetry(callback func(attempt int, delay time.Duration, err error)) {
	r.config.OnRetry = callback
}

// Predefined retry configurations for common use cases

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// AggressiveRetryConfig returns a configuration for aggressive retrying
func AggressiveRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   5,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   1.5,
		Jitter:       true,
	}
}

// ConservativeRetryConfig returns a configuration for conservative retrying
func ConservativeRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   2,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
		Jitter:       true,
	}
}

// CloudflareRetryConfig returns a configuration optimized for Cloudflare API
func CloudflareRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       3,
		InitialDelay:     200 * time.Millisecond,
		MaxDelay:         30 * time.Second,
		Multiplier:       2.0,
		Jitter:           true,
		RetryableChecker: cloudflareRetryable,
	}
}

// Default retryable error checker
func defaultRetryableChecker(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// By default, all other errors are considered retryable
	return true
}

// Cloudflare-specific retryable error checker
func cloudflareRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errStr := err.Error()

	// Network errors are usually retryable
	if contains(errStr, "connection refused") ||
		contains(errStr, "timeout") ||
		contains(errStr, "temporary failure") ||
		contains(errStr, "network is unreachable") {
		return true
	}

	// HTTP 5xx errors are retryable
	if contains(errStr, "500") || contains(errStr, "502") ||
		contains(errStr, "503") || contains(errStr, "504") {
		return true
	}

	// Cloudflare rate limiting (429) is retryable
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return true
	}

	// Authentication errors (401, 403) are not retryable
	if contains(errStr, "401") || contains(errStr, "403") ||
		contains(errStr, "unauthorized") || contains(errStr, "forbidden") {
		return false
	}

	// Client errors (4xx except 429) are generally not retryable
	if contains(errStr, "400") || contains(errStr, "404") || contains(errStr, "422") {
		return false
	}

	// Default to retryable for unknown errors
	return true
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				indexInString(s, substr) != -1))
}

// Simple string search
func indexInString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Convenience functions for common retry patterns

// RetryOnError retries a function with default configuration
func RetryOnError(ctx context.Context, fn RetryableFunc) error {
	retryer := NewRetryerWithRegistry(DefaultRetryConfig(), zap.NewNop(), nil)
	return retryer.RetryWithBackoff(ctx, fn)
}

// RetryCloudflareOperation retries a Cloudflare API operation
func RetryCloudflareOperation(ctx context.Context, logger *zap.Logger, operation string, fn RetryableFunc) error {
	retryer := NewRetryerWithRegistry(CloudflareRetryConfig(), logger, nil)
	return retryer.Retry(ctx, operation, fn)
}

// RetryWithConfig retries a function with custom configuration
func RetryWithConfig(ctx context.Context, config RetryConfig, logger *zap.Logger, //nolint:lll
	operation string, fn RetryableFunc) error {
	retryer := NewRetryerWithRegistry(config, logger, nil)
	return retryer.Retry(ctx, operation, fn)
}
