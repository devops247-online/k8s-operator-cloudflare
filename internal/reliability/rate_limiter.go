package reliability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// RateLimiterType defines the type of rate limiting algorithm
type RateLimiterType string

const (
	// TokenBucket uses token bucket algorithm
	TokenBucket RateLimiterType = "token_bucket"
	// SlidingWindow uses sliding window algorithm
	SlidingWindow RateLimiterType = "sliding_window"
	// FixedWindow uses fixed window algorithm
	FixedWindow RateLimiterType = "fixed_window"
)

// RateLimiterConfig holds configuration for the rate limiter
type RateLimiterConfig struct {
	// Name is the rate limiter identifier
	Name string

	// Type is the rate limiting algorithm type
	Type RateLimiterType

	// RequestsPerSecond is the allowed rate (requests per second)
	RequestsPerSecond float64

	// BurstSize is the maximum burst size allowed (only for token bucket)
	BurstSize int

	// WindowSize is the time window size (for windowed algorithms)
	WindowSize time.Duration

	// BackoffStrategy defines how to handle rate limit exceeded
	BackoffStrategy BackoffStrategy

	// OnRateLimited is called when rate limit is exceeded
	OnRateLimited func(limiterName string, delay time.Duration)
}

// BackoffStrategy defines how to handle rate limit exceeded
type BackoffStrategy string

const (
	// Block waits until rate limit allows the request
	Block BackoffStrategy = "block"
	// Reject immediately rejects the request
	Reject BackoffStrategy = "reject"
	// Adaptive dynamically adjusts based on current load
	Adaptive BackoffStrategy = "adaptive"
)

// RateLimiter interface defines rate limiting operations
type RateLimiter interface {
	// Allow checks if a request is allowed and returns immediately
	Allow() bool

	// Wait blocks until the request can be processed
	Wait(ctx context.Context) error

	// Reserve reserves a slot and returns the delay needed
	Reserve() time.Duration

	// SetRate updates the rate limit dynamically
	SetRate(requestsPerSecond float64, burstSize int)

	// Stats returns current rate limiter statistics
	Stats() RateLimiterStats

	// Reset resets the rate limiter state
	Reset()
}

// RateLimiterStats holds rate limiter statistics
type RateLimiterStats struct {
	RequestsAllowed   uint64
	RequestsRejected  uint64
	RequestsWaiting   uint64
	CurrentTokens     float64
	RequestsPerSecond float64
	BurstSize         int
	LastRefill        time.Time
}

// TokenBucketLimiter implements token bucket rate limiting
type TokenBucketLimiter struct {
	config RateLimiterConfig
	logger *zap.Logger

	// Token bucket state
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	maxTokens  float64
	refillRate float64

	// Statistics
	allowed  uint64
	rejected uint64
	waiting  uint64

	// Prometheus metrics
	requestsTotal   *prometheus.CounterVec
	waitingDuration *prometheus.HistogramVec
	tokensGauge     *prometheus.GaugeVec
	rateGauge       *prometheus.GaugeVec
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimiterConfig, logger *zap.Logger) RateLimiter {
	return NewRateLimiterWithRegistry(config, logger, prometheus.DefaultRegisterer)
}

// NewRateLimiterWithRegistry creates a new rate limiter with a custom metrics registry
func NewRateLimiterWithRegistry(config RateLimiterConfig, logger *zap.Logger, //nolint:lll
	registerer prometheus.Registerer) RateLimiter {
	// Set default values
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = 10.0
	}
	if config.BurstSize <= 0 {
		config.BurstSize = int(config.RequestsPerSecond)
	}
	if config.WindowSize == 0 {
		config.WindowSize = 1 * time.Second
	}
	if config.BackoffStrategy == "" {
		config.BackoffStrategy = Block
	}
	if config.Type == "" {
		config.Type = TokenBucket
	}

	switch config.Type {
	case TokenBucket:
		return newTokenBucketLimiter(config, logger, registerer)
	case SlidingWindow:
		return newSlidingWindowLimiter(config, logger, registerer)
	case FixedWindow:
		return newFixedWindowLimiter(config, logger, registerer)
	default:
		logger.Warn("Unknown rate limiter type, using token bucket",
			zap.String("type", string(config.Type)))
		config.Type = TokenBucket
		return newTokenBucketLimiter(config, logger, registerer)
	}
}

// newTokenBucketLimiter creates a new token bucket rate limiter
func newTokenBucketLimiter(config RateLimiterConfig, logger *zap.Logger, //nolint:lll
	registerer prometheus.Registerer) *TokenBucketLimiter {
	limiter := &TokenBucketLimiter{
		config:     config,
		logger:     logger,
		tokens:     float64(config.BurstSize),
		lastRefill: time.Now(),
		maxTokens:  float64(config.BurstSize),
		refillRate: config.RequestsPerSecond,

		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limiter_requests_total",
				Help: "Total number of requests processed by rate limiter",
			},
			[]string{"name", "result"},
		),

		waitingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rate_limiter_waiting_duration_seconds",
				Help:    "Time spent waiting for rate limiter",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
			},
			[]string{"name"},
		),

		tokensGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rate_limiter_tokens_available",
				Help: "Current number of available tokens",
			},
			[]string{"name"},
		),

		rateGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rate_limiter_rate_per_second",
				Help: "Current rate limit in requests per second",
			},
			[]string{"name"},
		),
	}

	// Register metrics
	if registerer != nil {
		registerer.MustRegister(
			limiter.requestsTotal,
			limiter.waitingDuration,
			limiter.tokensGauge,
			limiter.rateGauge,
		)
	}

	// Set initial metric values
	limiter.tokensGauge.WithLabelValues(config.Name).Set(limiter.tokens)
	limiter.rateGauge.WithLabelValues(config.Name).Set(config.RequestsPerSecond)

	return limiter
}

// Allow checks if a request is allowed and returns immediately
func (tb *TokenBucketLimiter) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillTokens()

	if tb.tokens >= 1.0 {
		tb.tokens--
		tb.allowed++
		tb.requestsTotal.WithLabelValues(tb.config.Name, "allowed").Inc()
		tb.tokensGauge.WithLabelValues(tb.config.Name).Set(tb.tokens)
		return true
	}

	tb.rejected++
	tb.requestsTotal.WithLabelValues(tb.config.Name, "rejected").Inc()
	return false
}

// Wait blocks until the request can be processed
func (tb *TokenBucketLimiter) Wait(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		tb.waitingDuration.WithLabelValues(tb.config.Name).Observe(duration.Seconds())
	}()

	for {
		// Check if we can proceed
		if tb.Allow() {
			return nil
		}

		// Calculate delay needed
		delay := tb.Reserve()
		if delay == 0 {
			continue
		}

		// Handle different backoff strategies
		switch tb.config.BackoffStrategy {
		case Reject:
			return ErrRateLimited
		case Block:
			// Wait for the calculated delay
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				// Continue the loop to try again
			}
		case Adaptive:
			// Adaptive strategy - reduce delay based on system load
			adaptiveDelay := tb.calculateAdaptiveDelay(delay)
			timer := time.NewTimer(adaptiveDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				// Continue the loop to try again
			}
		}

		// Call rate limited callback if configured
		if tb.config.OnRateLimited != nil {
			tb.config.OnRateLimited(tb.config.Name, delay)
		}
	}
}

// Reserve reserves a slot and returns the delay needed
func (tb *TokenBucketLimiter) Reserve() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillTokens()

	if tb.tokens >= 1.0 {
		return 0 // No delay needed
	}

	// Calculate delay needed to get one token
	tokensNeeded := 1.0 - tb.tokens
	delay := time.Duration(tokensNeeded / tb.refillRate * float64(time.Second))

	return delay
}

// SetRate updates the rate limit dynamically
func (tb *TokenBucketLimiter) SetRate(requestsPerSecond float64, burstSize int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillRate = requestsPerSecond
	tb.maxTokens = float64(burstSize)

	// Adjust current tokens if needed
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}

	tb.config.RequestsPerSecond = requestsPerSecond
	tb.config.BurstSize = burstSize

	// Update metrics
	tb.tokensGauge.WithLabelValues(tb.config.Name).Set(tb.tokens)
	tb.rateGauge.WithLabelValues(tb.config.Name).Set(requestsPerSecond)

	tb.logger.Info("Rate limit updated",
		zap.String("name", tb.config.Name),
		zap.Float64("requests_per_second", requestsPerSecond),
		zap.Int("burst_size", burstSize),
	)
}

// Stats returns current rate limiter statistics
func (tb *TokenBucketLimiter) Stats() RateLimiterStats {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refillTokens()

	return RateLimiterStats{
		RequestsAllowed:   tb.allowed,
		RequestsRejected:  tb.rejected,
		RequestsWaiting:   tb.waiting,
		CurrentTokens:     tb.tokens,
		RequestsPerSecond: tb.refillRate,
		BurstSize:         int(tb.maxTokens),
		LastRefill:        tb.lastRefill,
	}
}

// Reset resets the rate limiter state
func (tb *TokenBucketLimiter) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens = tb.maxTokens
	tb.lastRefill = time.Now()
	tb.allowed = 0
	tb.rejected = 0
	tb.waiting = 0

	tb.tokensGauge.WithLabelValues(tb.config.Name).Set(tb.tokens)
}

// refillTokens refills the token bucket based on elapsed time
func (tb *TokenBucketLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tb.lastRefill = now

	// Calculate tokens to add
	tokensToAdd := tb.refillRate * elapsed.Seconds()
	tb.tokens = minVal(tb.tokens+tokensToAdd, tb.maxTokens)
}

// calculateAdaptiveDelay calculates adaptive delay based on current system state
func (tb *TokenBucketLimiter) calculateAdaptiveDelay(baseDelay time.Duration) time.Duration {
	// Simple adaptive strategy: reduce delay if we have been waiting
	waitingRatio := float64(tb.waiting) / (float64(tb.allowed+tb.rejected) + 1)

	// Reduce delay if too many requests are waiting
	if waitingRatio > 0.5 {
		return time.Duration(float64(baseDelay) * 0.5)
	}

	return baseDelay
}

// Errors
var (
	ErrRateLimited = fmt.Errorf("rate limit exceeded")
)

// Convenience functions for common rate limiting patterns

// CloudflareRateLimiterConfig returns a configuration optimized for Cloudflare API
func CloudflareRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Name:              "cloudflare-api",
		Type:              TokenBucket,
		RequestsPerSecond: 1200.0 / 300.0, // 1200 requests per 5 minutes = 4 RPS
		BurstSize:         10,
		BackoffStrategy:   Block,
	}
}

// DefaultRateLimiterConfig returns a default rate limiter configuration
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Name:              "default",
		Type:              TokenBucket,
		RequestsPerSecond: 10.0,
		BurstSize:         10,
		BackoffStrategy:   Block,
	}
}

// AggressiveRateLimiterConfig returns a configuration for high-throughput scenarios
func AggressiveRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Name:              "aggressive",
		Type:              TokenBucket,
		RequestsPerSecond: 100.0,
		BurstSize:         50,
		BackoffStrategy:   Reject,
	}
}

// ConservativeRateLimiterConfig returns a conservative rate limiter configuration
func ConservativeRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Name:              "conservative",
		Type:              TokenBucket,
		RequestsPerSecond: 1.0,
		BurstSize:         5,
		BackoffStrategy:   Block,
	}
}

// Helper function
func minVal(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Placeholder implementations for other rate limiter types
// These would be implemented similarly to TokenBucketLimiter

// SlidingWindowLimiter implements sliding window rate limiting
type SlidingWindowLimiter struct {
	config RateLimiterConfig //nolint:unused // Will be used in future implementation
	logger *zap.Logger       //nolint:unused // Will be used in future implementation
	// Implementation would go here
}

func newSlidingWindowLimiter(config RateLimiterConfig, logger *zap.Logger, //nolint:lll
	registerer prometheus.Registerer) RateLimiter {
	// For now, fall back to token bucket
	logger.Warn("Sliding window limiter not yet implemented, using token bucket")
	config.Type = TokenBucket
	return newTokenBucketLimiter(config, logger, registerer)
}

// FixedWindowLimiter implements fixed window rate limiting
type FixedWindowLimiter struct {
	config RateLimiterConfig //nolint:unused // Will be used in future implementation
	logger *zap.Logger       //nolint:unused // Will be used in future implementation
	// Implementation would go here
}

func newFixedWindowLimiter(config RateLimiterConfig, logger *zap.Logger, registerer prometheus.Registerer) RateLimiter {
	// For now, fall back to token bucket
	logger.Warn("Fixed window limiter not yet implemented, using token bucket")
	config.Type = TokenBucket
	return newTokenBucketLimiter(config, logger, registerer)
}
