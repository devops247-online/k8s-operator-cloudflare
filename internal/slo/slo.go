package slo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Manager manages SLO monitoring and alerting
type Manager struct {
	config     *Config
	calculator *Calculator
	logger     *zap.Logger

	// State management
	mu              sync.RWMutex
	lastCalculation map[string]*SLIValues
	errorBudgets    map[string]*ErrorBudgetState

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}
}

// ErrorBudgetState tracks the state of an error budget
type ErrorBudgetState struct {
	SLOName          string
	CurrentRemaining float64
	ConsumedPercent  float64
	LastUpdated      time.Time
	PolicyTriggered  []string
	BurnRate         float64
	TimeToExhaustion time.Duration
}

// NewManager creates a new SLO manager
func NewManager(config *Config, logger *zap.Logger) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid SLO config: %w", err)
	}

	return NewManagerWithRegistry(config, logger, prometheus.DefaultRegisterer)
}

// NewManagerWithRegistry creates a new SLO manager with a custom metrics registry
func NewManagerWithRegistry(config *Config, logger *zap.Logger, registerer prometheus.Registerer) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid SLO config: %w", err)
	}

	return &Manager{
		config:          config,
		calculator:      NewCalculatorWithRegistry(config, registerer),
		logger:          logger,
		lastCalculation: make(map[string]*SLIValues),
		errorBudgets:    make(map[string]*ErrorBudgetState),
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}, nil
}

// Start begins SLO monitoring
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("SLO monitoring is disabled")
		return nil
	}

	m.logger.Info("Starting SLO monitoring")

	// Start monitoring goroutine
	go m.run(ctx)

	return nil
}

// Stop stops SLO monitoring
func (m *Manager) Stop() {
	m.logger.Info("Stopping SLO monitoring")
	close(m.stopCh)

	// Only wait for doneCh if SLO monitoring was actually started
	if m.config.Enabled {
		<-m.doneCh
	}
}

// run is the main monitoring loop
func (m *Manager) run(ctx context.Context) {
	defer close(m.doneCh)

	// Create tickers for each window
	tickers := make(map[string]*time.Ticker)
	for _, window := range m.config.Windows {
		// Calculate SLIs more frequently than the window duration
		// Short-term windows: every 30 seconds
		// Long-term windows: every 5 minutes
		interval := window.Duration / 10
		if window.IsShortTerm {
			interval = 30 * time.Second
		} else if interval < 5*time.Minute {
			interval = 5 * time.Minute
		}

		ticker := time.NewTicker(interval)
		tickers[window.Name] = ticker

		// Initial calculation
		go m.calculateForWindow(ctx, window)
	}

	// Clean up tickers on exit
	defer func() {
		for _, ticker := range tickers {
			ticker.Stop()
		}
	}()

	// Main monitoring loop
	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Check each ticker
			for _, window := range m.config.Windows {
				select {
				case <-tickers[window.Name].C:
					go m.calculateForWindow(ctx, window)
				default:
					// Non-blocking
				}
			}

			// Small sleep to prevent busy loop
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// calculateForWindow calculates SLIs for a specific time window
func (m *Manager) calculateForWindow(ctx context.Context, window TimeWindow) {
	logger := m.logger.With(
		zap.String("window", window.Name),
		zap.Duration("duration", window.Duration),
	)

	logger.Debug("Calculating SLIs for window")

	// Calculate SLI values
	values, err := m.calculator.Calculate(ctx, window)
	if err != nil {
		logger.Error("Failed to calculate SLIs", zap.Error(err))
		return
	}

	// Store calculation
	m.mu.Lock()
	m.lastCalculation[window.Name] = values
	m.mu.Unlock()

	// Update error budgets
	m.updateErrorBudgets(values)

	// Check multi-window burn rates for alerting
	if window.IsShortTerm {
		m.checkBurnRates(ctx)
	}

	logger.Debug("SLI calculation completed",
		zap.Float64("availability", values.Availability),
		zap.Float64("success_rate", values.SuccessRate),
		zap.Duration("latency_p95", values.LatencyP95),
		zap.Float64("throughput", values.Throughput),
	)
}

// updateErrorBudgets updates error budget states based on SLI values
func (m *Manager) updateErrorBudgets(values *SLIValues) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update availability error budget
	m.updateSingleErrorBudget("availability", values.Availability, m.config.Targets.Availability/100, values.Window)

	// Update success rate error budget
	m.updateSingleErrorBudget("success_rate", values.SuccessRate, m.config.Targets.SuccessRate/100, values.Window)

	// Update latency error budget (simplified)
	latencyConformance := 1.0
	if values.LatencyP95 > m.config.Targets.LatencyP95 {
		latencyConformance = 0.95
	}
	m.updateSingleErrorBudget("latency_p95", latencyConformance, 0.95, values.Window)
}

// updateSingleErrorBudget updates a single error budget state
func (m *Manager) updateSingleErrorBudget(sloName string, actual, target float64, window string) {
	key := fmt.Sprintf("%s_%s", sloName, window)

	state, exists := m.errorBudgets[key]
	if !exists {
		state = &ErrorBudgetState{
			SLOName: sloName,
		}
		m.errorBudgets[key] = state
	}

	// Calculate remaining budget
	errorBudgetTotal := 1.0 - target
	errorBudgetConsumed := 0.0
	if actual < target {
		errorBudgetConsumed = target - actual
	}

	state.CurrentRemaining = 1.0
	if errorBudgetTotal > 0 {
		state.CurrentRemaining = (errorBudgetTotal - errorBudgetConsumed) / errorBudgetTotal
	}
	state.ConsumedPercent = 1.0 - state.CurrentRemaining
	state.LastUpdated = time.Now()

	// Calculate burn rate (simplified - would use historical data in production)
	if state.BurnRate == 0 {
		state.BurnRate = state.ConsumedPercent // Initial estimate
	} else {
		// Exponential moving average
		state.BurnRate = 0.9*state.BurnRate + 0.1*state.ConsumedPercent
	}

	// Estimate time to exhaustion
	if state.BurnRate > 0 && state.CurrentRemaining > 0 {
		hoursRemaining := state.CurrentRemaining / (state.BurnRate / 24) // Assuming daily burn rate
		state.TimeToExhaustion = time.Duration(hoursRemaining) * time.Hour
	}

	// Check policies
	m.checkErrorBudgetPolicies(state)
}

// checkErrorBudgetPolicies checks if any policies should be triggered
func (m *Manager) checkErrorBudgetPolicies(state *ErrorBudgetState) {
	state.PolicyTriggered = []string{}

	for _, policy := range m.config.ErrorBudget.Policies {
		if state.ConsumedPercent >= policy.Threshold {
			state.PolicyTriggered = append(state.PolicyTriggered, policy.Action)

			m.logger.Warn("Error budget policy triggered",
				zap.String("slo", state.SLOName),
				zap.Float64("consumed", state.ConsumedPercent),
				zap.Float64("threshold", policy.Threshold),
				zap.String("action", policy.Action),
				zap.String("description", policy.Description),
			)
		}
	}
}

// checkBurnRates checks multi-window burn rates for alerting
func (m *Manager) checkBurnRates(ctx context.Context) {
	// Check page alerts (fast burn)
	if m.config.AlertingRules.PageAlerts.Enabled {
		m.checkBurnRateAlert(ctx, m.config.AlertingRules.PageAlerts, "page")
	}

	// Check ticket alerts (slow burn)
	if m.config.AlertingRules.TicketAlerts.Enabled {
		m.checkBurnRateAlert(ctx, m.config.AlertingRules.TicketAlerts, "ticket")
	}
}

// checkBurnRateAlert checks a specific burn rate alert
func (m *Manager) checkBurnRateAlert(ctx context.Context, alert BurnRateAlert, alertType string) {
	// Find the windows
	var shortWindow, longWindow *TimeWindow
	for i := range m.config.Windows {
		if m.config.Windows[i].Duration == alert.ShortWindow {
			shortWindow = &m.config.Windows[i]
		}
		if m.config.Windows[i].Duration == alert.LongWindow {
			longWindow = &m.config.Windows[i]
		}
	}

	if shortWindow == nil || longWindow == nil {
		m.logger.Error("Burn rate windows not found",
			zap.String("alert_type", alertType),
			zap.Duration("short_window", alert.ShortWindow),
			zap.Duration("long_window", alert.LongWindow),
		)
		return
	}

	// Calculate burn rate
	burnRate, err := m.calculator.CalculateMultiWindowBurnRate(ctx, *shortWindow, *longWindow)
	if err != nil {
		m.logger.Error("Failed to calculate burn rate",
			zap.String("alert_type", alertType),
			zap.Error(err),
		)
		return
	}

	// Check threshold
	if burnRate >= alert.BurnRateThreshold {
		m.logger.Error("Burn rate threshold exceeded",
			zap.String("alert_type", alertType),
			zap.Float64("burn_rate", burnRate),
			zap.Float64("threshold", alert.BurnRateThreshold),
			zap.String("severity", alert.Severity),
		)

		// In a real implementation, this would trigger alerts via AlertManager
	}
}

// GetSLIValues returns the last calculated SLI values for a window
func (m *Manager) GetSLIValues(window string) (*SLIValues, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	values, exists := m.lastCalculation[window]
	return values, exists
}

// GetErrorBudgetState returns the error budget state for an SLO
func (m *Manager) GetErrorBudgetState(sloName, window string) (*ErrorBudgetState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s_%s", sloName, window)
	state, exists := m.errorBudgets[key]
	return state, exists
}

// GetAllErrorBudgets returns all error budget states
func (m *Manager) GetAllErrorBudgets() map[string]*ErrorBudgetState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*ErrorBudgetState)
	for k, v := range m.errorBudgets {
		result[k] = v
	}
	return result
}
