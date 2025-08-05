package slo

import (
	"time"
)

// Config defines the SLO configuration
type Config struct {
	// Enabled indicates if SLO monitoring is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Targets defines the SLO targets
	Targets SLOTargets `json:"targets" yaml:"targets"`

	// ErrorBudget defines error budget configuration
	ErrorBudget ErrorBudgetConfig `json:"errorBudget" yaml:"errorBudget"`

	// Windows defines the time windows for SLI calculation
	Windows []TimeWindow `json:"windows" yaml:"windows"`

	// AlertingRules defines the alerting configuration
	AlertingRules AlertingConfig `json:"alertingRules" yaml:"alertingRules"`
}

// SLOTargets defines the target objectives for each SLI //nolint:revive // Descriptive name
type SLOTargets struct { //nolint:revive // Descriptive name
	// Availability target percentage (e.g., 99.9)
	Availability float64 `json:"availability" yaml:"availability"`

	// SuccessRate target percentage (e.g., 99.5)
	SuccessRate float64 `json:"successRate" yaml:"successRate"`

	// LatencyP95 target duration for 95th percentile
	LatencyP95 time.Duration `json:"latencyP95" yaml:"latencyP95"`

	// LatencyP99 target duration for 99th percentile
	LatencyP99 time.Duration `json:"latencyP99" yaml:"latencyP99"`

	// ThroughputMin minimum operations per minute
	ThroughputMin float64 `json:"throughputMin" yaml:"throughputMin"`
}

// ErrorBudgetConfig defines error budget configuration
type ErrorBudgetConfig struct {
	// WindowDays defines the rolling window for error budget calculation
	WindowDays int `json:"windowDays" yaml:"windowDays"`

	// AlertThresholds defines when to alert on error budget consumption
	AlertThresholds ErrorBudgetThresholds `json:"alertThresholds" yaml:"alertThresholds"`

	// Policies defines what actions to take at different consumption levels
	Policies []ErrorBudgetPolicy `json:"policies" yaml:"policies"`
}

// ErrorBudgetThresholds defines alert thresholds for error budget
type ErrorBudgetThresholds struct {
	// Warning threshold as a fraction of error budget consumed (0.0-1.0)
	Warning float64 `json:"warning" yaml:"warning"`

	// Critical threshold as a fraction of error budget consumed (0.0-1.0)
	Critical float64 `json:"critical" yaml:"critical"`
}

// ErrorBudgetPolicy defines actions based on error budget consumption
type ErrorBudgetPolicy struct {
	// Threshold is the consumption threshold that triggers this policy
	Threshold float64 `json:"threshold" yaml:"threshold"`

	// Action to take when threshold is exceeded
	Action string `json:"action" yaml:"action"`

	// Description of the policy
	Description string `json:"description" yaml:"description"`
}

// TimeWindow defines a time window for SLI calculation
type TimeWindow struct {
	// Name of the window (e.g., "5m", "30m", "1h")
	Name string `json:"name" yaml:"name"`

	// Duration of the window
	Duration time.Duration `json:"duration" yaml:"duration"`

	// IsShortTerm indicates if this is a short-term window for fast burn alerts
	IsShortTerm bool `json:"isShortTerm" yaml:"isShortTerm"`
}

// AlertingConfig defines multi-window multi-burn-rate alerting
type AlertingConfig struct {
	// PageAlerts for immediate attention (fast burn)
	PageAlerts BurnRateAlert `json:"pageAlerts" yaml:"pageAlerts"`

	// TicketAlerts for slower burns
	TicketAlerts BurnRateAlert `json:"ticketAlerts" yaml:"ticketAlerts"`
}

// BurnRateAlert defines a burn rate alert configuration
type BurnRateAlert struct {
	// Enabled indicates if this alert type is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ShortWindow for recent burn rate
	ShortWindow time.Duration `json:"shortWindow" yaml:"shortWindow"`

	// LongWindow for sustained burn rate
	LongWindow time.Duration `json:"longWindow" yaml:"longWindow"`

	// BurnRateThreshold multiplier for error budget burn rate
	BurnRateThreshold float64 `json:"burnRateThreshold" yaml:"burnRateThreshold"`

	// Severity of the alert
	Severity string `json:"severity" yaml:"severity"`
}

// DefaultConfig returns a default SLO configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Targets: SLOTargets{
			Availability:  99.9,
			SuccessRate:   99.5,
			LatencyP95:    30 * time.Second,
			LatencyP99:    60 * time.Second,
			ThroughputMin: 10.0,
		},
		ErrorBudget: ErrorBudgetConfig{
			WindowDays: 30,
			AlertThresholds: ErrorBudgetThresholds{
				Warning:  0.5,
				Critical: 0.9,
			},
			Policies: []ErrorBudgetPolicy{
				{
					Threshold:   0.5,
					Action:      "notify",
					Description: "Notify team when 50% of error budget is consumed",
				},
				{
					Threshold:   0.9,
					Action:      "freeze_deployments",
					Description: "Freeze non-critical deployments when 90% of error budget is consumed",
				},
			},
		},
		Windows: []TimeWindow{
			{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true},
			{Name: "30m", Duration: 30 * time.Minute, IsShortTerm: true},
			{Name: "1h", Duration: 1 * time.Hour, IsShortTerm: false},
			{Name: "6h", Duration: 6 * time.Hour, IsShortTerm: false},
			{Name: "1d", Duration: 24 * time.Hour, IsShortTerm: false},
			{Name: "30d", Duration: 30 * 24 * time.Hour, IsShortTerm: false},
		},
		AlertingRules: AlertingConfig{
			PageAlerts: BurnRateAlert{
				Enabled:           true,
				ShortWindow:       5 * time.Minute,
				LongWindow:        1 * time.Hour,
				BurnRateThreshold: 14.4, // Consumes monthly budget in ~6 hours
				Severity:          "critical",
			},
			TicketAlerts: BurnRateAlert{
				Enabled:           true,
				ShortWindow:       30 * time.Minute,
				LongWindow:        6 * time.Hour,
				BurnRateThreshold: 6.0, // Consumes monthly budget in ~3 days
				Severity:          "warning",
			},
		},
	}
}

// Validate validates the SLO configuration
func (c *Config) Validate() error {
	// TODO: Implement validation
	return nil
}
