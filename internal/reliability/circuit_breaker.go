// Package reliability provides circuit breaker, retry mechanism, and rate limiting
// patterns for building resilient applications with Cloudflare DNS operator.
package reliability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	// StateClosed - Circuit is closed, requests flow normally
	StateClosed CircuitBreakerState = iota
	// StateOpen - Circuit is open, requests are rejected
	StateOpen
	// StateHalfOpen - Circuit is half-open, limited requests are allowed to test recovery
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// Name is the circuit breaker identifier
	Name string

	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for the circuit breaker
	// to clear the internal counts
	Interval time.Duration

	// Timeout is the period of the open state after which the state becomes half-open
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
	// If ReadyToTrip returns true, the circuit breaker will be placed into the open state.
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes
	OnStateChange func(name string, from CircuitBreakerState, to CircuitBreakerState)

	// IsSuccessful determines whether a given error should be counted as a success or failure
	IsSuccessful func(err error) bool
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// Reset resets all counts to zero
func (c *Counts) Reset() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// OnRequest increments the number of requests
func (c *Counts) OnRequest() {
	c.Requests++
}

// OnSuccess increments the number of successes
func (c *Counts) OnSuccess() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

// OnFailure increments the number of failures
func (c *Counts) OnFailure() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

// SuccessRate returns the success rate as a float between 0 and 1
func (c *Counts) SuccessRate() float64 {
	if c.Requests == 0 {
		return 0
	}
	return float64(c.TotalSuccesses) / float64(c.Requests)
}

// FailureRate returns the failure rate as a float between 0 and 1
func (c *Counts) FailureRate() float64 {
	if c.Requests == 0 {
		return 0
	}
	return float64(c.TotalFailures) / float64(c.Requests)
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig
	logger *zap.Logger

	// Synchronization
	mu sync.RWMutex

	// State management
	state      CircuitBreakerState
	generation uint64
	counts     Counts
	expiry     time.Time

	// Metrics
	requestsTotal     *prometheus.CounterVec
	successesTotal    *prometheus.CounterVec
	failuresTotal     *prometheus.CounterVec
	stateChangesTotal *prometheus.CounterVec
	stateDuration     *prometheus.GaugeVec
	currentStateGauge *prometheus.GaugeVec
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig, logger *zap.Logger) *CircuitBreaker {
	return NewCircuitBreakerWithRegistry(config, logger, prometheus.DefaultRegisterer)
}

// NewCircuitBreakerWithRegistry creates a new circuit breaker with a custom metrics registry
func NewCircuitBreakerWithRegistry(config CircuitBreakerConfig, logger *zap.Logger, //nolint:lll
	registerer prometheus.Registerer) *CircuitBreaker {
	// Set default configuration values
	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	if config.Interval == 0 {
		config.Interval = 60 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = defaultReadyToTrip
	}
	if config.IsSuccessful == nil {
		config.IsSuccessful = defaultIsSuccessful
	}

	cb := &CircuitBreaker{
		config:     config,
		logger:     logger,
		state:      StateClosed,
		generation: 0,
		expiry:     time.Now().Add(config.Interval),

		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_requests_total",
				Help: "Total number of requests handled by circuit breaker",
			},
			[]string{"name", "state"},
		),

		successesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_successes_total",
				Help: "Total number of successful requests",
			},
			[]string{"name"},
		),

		failuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_failures_total",
				Help: "Total number of failed requests",
			},
			[]string{"name"},
		),

		stateChangesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "circuit_breaker_state_changes_total",
				Help: "Total number of state changes",
			},
			[]string{"name", "from_state", "to_state"},
		),

		stateDuration: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_state_duration_seconds",
				Help: "Duration spent in each state",
			},
			[]string{"name", "state"},
		),

		currentStateGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_current_state",
				Help: "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
			},
			[]string{"name"},
		),
	}

	// Register metrics if registerer provided
	if registerer != nil {
		registerer.MustRegister(
			cb.requestsTotal,
			cb.successesTotal,
			cb.failuresTotal,
			cb.stateChangesTotal,
			cb.stateDuration,
			cb.currentStateGauge,
		)
	}

	// Set initial state metric
	cb.currentStateGauge.WithLabelValues(config.Name).Set(float64(StateClosed))

	return cb
}

// Execute runs the given function if the circuit breaker accepts the request
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		// Recover from panics and treat them as failures
		if r := recover(); r != nil {
			cb.afterRequest(generation, fmt.Errorf("panic recovered: %v", r))
			panic(r) // Re-panic after recording the failure
		}
	}()

	// Execute the function
	err = fn(ctx)
	cb.afterRequest(generation, err)

	return err
}

// beforeRequest is called before a request is executed
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		cb.requestsTotal.WithLabelValues(cb.config.Name, "open").Inc()
		return generation, ErrCircuitBreakerOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
		cb.requestsTotal.WithLabelValues(cb.config.Name, "half-open-rejected").Inc()
		return generation, ErrCircuitBreakerOpen
	}

	cb.counts.OnRequest()
	cb.requestsTotal.WithLabelValues(cb.config.Name, state.String()).Inc()

	return generation, nil
}

// afterRequest is called after a request is executed
func (cb *CircuitBreaker) afterRequest(before uint64, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	// If generation has changed, ignore this result (too late)
	if generation != before {
		return
	}

	if cb.config.IsSuccessful(err) {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess(state CircuitBreakerState, now time.Time) {
	cb.counts.OnSuccess()
	cb.successesTotal.WithLabelValues(cb.config.Name).Inc()

	if state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.config.MaxRequests {
		cb.setState(StateClosed, now)
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure(state CircuitBreakerState, now time.Time) {
	cb.counts.OnFailure()
	cb.failuresTotal.WithLabelValues(cb.config.Name).Inc()

	switch state {
	case StateClosed:
		if cb.config.ReadyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

// currentState returns the current state and generation
func (cb *CircuitBreaker) currentState(now time.Time) (CircuitBreakerState, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState changes the state of the circuit breaker
func (cb *CircuitBreaker) setState(state CircuitBreakerState, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state
	cb.toNewGeneration(now)

	// Update metrics
	cb.stateChangesTotal.WithLabelValues(cb.config.Name, prev.String(), state.String()).Inc()
	cb.currentStateGauge.WithLabelValues(cb.config.Name).Set(float64(state))

	// Call state change callback
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, prev, state)
	}

	// Log state change
	cb.logger.Info("Circuit breaker state changed",
		zap.String("name", cb.config.Name),
		zap.String("from", prev.String()),
		zap.String("to", state.String()),
		zap.Uint32("requests", cb.counts.Requests),
		zap.Uint32("failures", cb.counts.TotalFailures),
		zap.Float64("failure_rate", cb.counts.FailureRate()),
	)
}

// toNewGeneration resets counts and expiry for a new generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.Reset()

	var expiry time.Time
	switch cb.state {
	case StateClosed:
		if cb.config.Interval == 0 {
			expiry = time.Time{}
		} else {
			expiry = now.Add(cb.config.Interval)
		}
	case StateOpen:
		expiry = now.Add(cb.config.Timeout)
	default: // StateHalfOpen
		expiry = time.Time{}
	}
	cb.expiry = expiry
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state, _ := cb.currentState(time.Now())
	return state
}

// Counts returns a copy of the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.counts
}

// Name returns the circuit breaker name
func (cb *CircuitBreaker) Name() string {
	return cb.config.Name
}

// Reset resets the circuit breaker to the closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.toNewGeneration(time.Now())
	cb.setState(StateClosed, time.Now())
}

// Errors
var (
	ErrCircuitBreakerOpen = fmt.Errorf("circuit breaker is open")
)

// Default functions
func defaultReadyToTrip(counts Counts) bool {
	return counts.Requests >= 5 && counts.FailureRate() >= 0.6
}

func defaultIsSuccessful(err error) bool {
	return err == nil
}
