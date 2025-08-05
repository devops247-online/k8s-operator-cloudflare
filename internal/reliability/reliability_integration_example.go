package reliability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// ReliabilityComponents demonstrates how to use all reliability patterns together
// ReliabilityComponents combines all reliability patterns //nolint:revive // Descriptive name
type ReliabilityComponents struct { //nolint:revive // Descriptive name
	CircuitBreaker *CircuitBreaker
	Retryer        *Retryer
	RateLimiter    RateLimiter
	logger         *zap.Logger
}

// NewReliabilityComponents creates a complete reliability setup for Cloudflare API
func NewReliabilityComponents(logger *zap.Logger) *ReliabilityComponents {
	return NewReliabilityComponentsWithRegistry(logger, prometheus.DefaultRegisterer)
}

// NewReliabilityComponentsWithRegistry creates a complete reliability setup with custom registry
func NewReliabilityComponentsWithRegistry(logger *zap.Logger, registerer prometheus.Registerer) *ReliabilityComponents {
	// Circuit breaker for API protection
	circuitBreaker := NewCircuitBreakerWithRegistry(CircuitBreakerConfig{
		Name:        "cloudflare-api",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.Requests >= 5 && counts.FailureRate() >= 0.6
		},
	}, logger, registerer)

	// Retry logic with exponential backoff
	retryer := NewRetryerWithRegistry(CloudflareRetryConfig(), logger, registerer)

	// Rate limiter for Cloudflare API limits
	rateLimiter := NewRateLimiterWithRegistry(CloudflareRateLimiterConfig(), logger, registerer)

	return &ReliabilityComponents{
		CircuitBreaker: circuitBreaker,
		Retryer:        retryer,
		RateLimiter:    rateLimiter,
		logger:         logger,
	}
}

// ExecuteWithReliability executes a function with all reliability patterns applied
func (rc *ReliabilityComponents) ExecuteWithReliability(ctx context.Context, operation string, //nolint:lll
	fn func(context.Context) error) error {
	return rc.Retryer.Retry(ctx, operation, func(_ context.Context) error {
		// First check rate limit
		if err := rc.RateLimiter.Wait(ctx); err != nil {
			return err
		}

		// Then execute through circuit breaker
		return rc.CircuitBreaker.Execute(ctx, fn)
	})
}

// Example usage:
//
// func main() {
//     logger := zap.NewProduction()
//     components := NewReliabilityComponents(logger)
//
//     err := components.ExecuteWithReliability(context.Background(), "cloudflare_dns_update", //nolint:lll
//        func(_ context.Context) error {
//         // Your Cloudflare API call here
//         return cloudflareClient.UpdateDNSRecord(ctx, record)
//     })
//
//     if err != nil {
//         logger.Error("Failed to update DNS record", zap.Error(err))
//     }
// }
