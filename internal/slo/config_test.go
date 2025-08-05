package slo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test basic configuration
	assert.True(t, config.Enabled)
	assert.Equal(t, 99.9, config.Targets.Availability)
	assert.Equal(t, 99.5, config.Targets.SuccessRate)
	assert.Equal(t, 30*time.Second, config.Targets.LatencyP95)
	assert.Equal(t, 60*time.Second, config.Targets.LatencyP99)
	assert.Equal(t, 10.0, config.Targets.ThroughputMin)

	// Test error budget configuration
	assert.Equal(t, 30, config.ErrorBudget.WindowDays)
	assert.Equal(t, 0.5, config.ErrorBudget.AlertThresholds.Warning)
	assert.Equal(t, 0.9, config.ErrorBudget.AlertThresholds.Critical)
	assert.Len(t, config.ErrorBudget.Policies, 2)

	// Test time windows
	assert.Len(t, config.Windows, 6)
	assert.Equal(t, "5m", config.Windows[0].Name)
	assert.Equal(t, 5*time.Minute, config.Windows[0].Duration)
	assert.True(t, config.Windows[0].IsShortTerm)

	// Test alerting rules
	assert.True(t, config.AlertingRules.PageAlerts.Enabled)
	assert.Equal(t, 5*time.Minute, config.AlertingRules.PageAlerts.ShortWindow)
	assert.Equal(t, 1*time.Hour, config.AlertingRules.PageAlerts.LongWindow)
	assert.Equal(t, 14.4, config.AlertingRules.PageAlerts.BurnRateThreshold)
	assert.Equal(t, "critical", config.AlertingRules.PageAlerts.Severity)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "default config is valid",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "disabled config is valid",
			config: &Config{
				Enabled: false,
			},
			wantErr: false,
		},
		// Add more validation test cases as validation is implemented
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSLOTargets(t *testing.T) {
	targets := SLOTargets{
		Availability:  99.95,
		SuccessRate:   99.9,
		LatencyP95:    15 * time.Second,
		LatencyP99:    30 * time.Second,
		ThroughputMin: 25.0,
	}

	assert.Equal(t, 99.95, targets.Availability)
	assert.Equal(t, 99.9, targets.SuccessRate)
	assert.Equal(t, 15*time.Second, targets.LatencyP95)
	assert.Equal(t, 30*time.Second, targets.LatencyP99)
	assert.Equal(t, 25.0, targets.ThroughputMin)
}

func TestErrorBudgetConfig(t *testing.T) {
	config := ErrorBudgetConfig{
		WindowDays: 7,
		AlertThresholds: ErrorBudgetThresholds{
			Warning:  0.6,
			Critical: 0.95,
		},
		Policies: []ErrorBudgetPolicy{
			{
				Threshold:   0.75,
				Action:      "alert_team",
				Description: "Alert team when 75% consumed",
			},
		},
	}

	assert.Equal(t, 7, config.WindowDays)
	assert.Equal(t, 0.6, config.AlertThresholds.Warning)
	assert.Equal(t, 0.95, config.AlertThresholds.Critical)
	assert.Len(t, config.Policies, 1)
	assert.Equal(t, 0.75, config.Policies[0].Threshold)
	assert.Equal(t, "alert_team", config.Policies[0].Action)
}

func TestTimeWindow(t *testing.T) {
	window := TimeWindow{
		Name:        "test_window",
		Duration:    10 * time.Minute,
		IsShortTerm: true,
	}

	assert.Equal(t, "test_window", window.Name)
	assert.Equal(t, 10*time.Minute, window.Duration)
	assert.True(t, window.IsShortTerm)
}

func TestAlertingConfig(t *testing.T) {
	config := AlertingConfig{
		PageAlerts: BurnRateAlert{
			Enabled:           true,
			ShortWindow:       2 * time.Minute,
			LongWindow:        30 * time.Minute,
			BurnRateThreshold: 20.0,
			Severity:          "page",
		},
		TicketAlerts: BurnRateAlert{
			Enabled:           true,
			ShortWindow:       15 * time.Minute,
			LongWindow:        2 * time.Hour,
			BurnRateThreshold: 5.0,
			Severity:          "ticket",
		},
	}

	// Test page alerts
	assert.True(t, config.PageAlerts.Enabled)
	assert.Equal(t, 2*time.Minute, config.PageAlerts.ShortWindow)
	assert.Equal(t, 30*time.Minute, config.PageAlerts.LongWindow)
	assert.Equal(t, 20.0, config.PageAlerts.BurnRateThreshold)
	assert.Equal(t, "page", config.PageAlerts.Severity)

	// Test ticket alerts
	assert.True(t, config.TicketAlerts.Enabled)
	assert.Equal(t, 15*time.Minute, config.TicketAlerts.ShortWindow)
	assert.Equal(t, 2*time.Hour, config.TicketAlerts.LongWindow)
	assert.Equal(t, 5.0, config.TicketAlerts.BurnRateThreshold)
	assert.Equal(t, "ticket", config.TicketAlerts.Severity)
}

func TestBurnRateAlert(t *testing.T) {
	alert := BurnRateAlert{
		Enabled:           false,
		ShortWindow:       1 * time.Minute,
		LongWindow:        10 * time.Minute,
		BurnRateThreshold: 12.0,
		Severity:          "warning",
	}

	assert.False(t, alert.Enabled)
	assert.Equal(t, 1*time.Minute, alert.ShortWindow)
	assert.Equal(t, 10*time.Minute, alert.LongWindow)
	assert.Equal(t, 12.0, alert.BurnRateThreshold)
	assert.Equal(t, "warning", alert.Severity)
}

func TestErrorBudgetPolicy(t *testing.T) {
	policy := ErrorBudgetPolicy{
		Threshold:   0.8,
		Action:      "freeze_releases",
		Description: "Freeze releases when 80% of error budget consumed",
	}

	assert.Equal(t, 0.8, policy.Threshold)
	assert.Equal(t, "freeze_releases", policy.Action)
	assert.Equal(t, "Freeze releases when 80% of error budget consumed", policy.Description)
}

func TestErrorBudgetThresholds(t *testing.T) {
	thresholds := ErrorBudgetThresholds{
		Warning:  0.4,
		Critical: 0.85,
	}

	assert.Equal(t, 0.4, thresholds.Warning)
	assert.Equal(t, 0.85, thresholds.Critical)
}

// Benchmark tests for performance verification
func BenchmarkDefaultConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig()
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	config := DefaultConfig()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

// Table-driven tests for different configurations
func TestConfigurationScenarios(t *testing.T) {
	scenarios := []struct {
		name     string
		modifier func(*Config)
		validate func(*testing.T, *Config)
	}{
		{
			name: "high availability requirements",
			modifier: func(c *Config) {
				c.Targets.Availability = 99.99
				c.Targets.SuccessRate = 99.95
				c.ErrorBudget.AlertThresholds.Warning = 0.25
				c.ErrorBudget.AlertThresholds.Critical = 0.5
			},
			validate: func(t *testing.T, c *Config) {
				assert.Equal(t, 99.99, c.Targets.Availability)
				assert.Equal(t, 99.95, c.Targets.SuccessRate)
				assert.Equal(t, 0.25, c.ErrorBudget.AlertThresholds.Warning)
				assert.Equal(t, 0.5, c.ErrorBudget.AlertThresholds.Critical)
			},
		},
		{
			name: "relaxed requirements",
			modifier: func(c *Config) {
				c.Targets.Availability = 99.0
				c.Targets.SuccessRate = 95.0
				c.Targets.LatencyP95 = 60 * time.Second
				c.ErrorBudget.WindowDays = 7
			},
			validate: func(t *testing.T, c *Config) {
				assert.Equal(t, 99.0, c.Targets.Availability)
				assert.Equal(t, 95.0, c.Targets.SuccessRate)
				assert.Equal(t, 60*time.Second, c.Targets.LatencyP95)
				assert.Equal(t, 7, c.ErrorBudget.WindowDays)
			},
		},
		{
			name: "fast burn alerting",
			modifier: func(c *Config) {
				c.AlertingRules.PageAlerts.ShortWindow = 1 * time.Minute
				c.AlertingRules.PageAlerts.LongWindow = 15 * time.Minute
				c.AlertingRules.PageAlerts.BurnRateThreshold = 30.0
			},
			validate: func(t *testing.T, c *Config) {
				assert.Equal(t, 1*time.Minute, c.AlertingRules.PageAlerts.ShortWindow)
				assert.Equal(t, 15*time.Minute, c.AlertingRules.PageAlerts.LongWindow)
				assert.Equal(t, 30.0, c.AlertingRules.PageAlerts.BurnRateThreshold)
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			config := DefaultConfig()
			scenario.modifier(config)
			scenario.validate(t, config)

			// Ensure modified config is still valid
			err := config.Validate()
			assert.NoError(t, err)
		})
	}
}

// Edge case tests
func TestConfigEdgeCases(t *testing.T) {
	t.Run("zero values", func(t *testing.T) {
		config := &Config{}
		// Should not panic
		err := config.Validate()
		assert.NoError(t, err) // Since validation is not implemented yet
	})

	t.Run("negative values", func(t *testing.T) {
		config := DefaultConfig()
		config.Targets.Availability = -1.0
		config.Targets.SuccessRate = -1.0
		config.ErrorBudget.WindowDays = -1

		// Should handle gracefully when validation is implemented
		err := config.Validate()
		assert.NoError(t, err) // Since validation is not implemented yet
	})

	t.Run("extreme values", func(t *testing.T) {
		config := DefaultConfig()
		config.Targets.Availability = 100.0
		config.Targets.LatencyP95 = 24 * time.Hour
		config.ErrorBudget.WindowDays = 365

		err := config.Validate()
		assert.NoError(t, err)
	})
}

// Concurrent access tests
func TestConfigConcurrentAccess(t *testing.T) {
	config := DefaultConfig()

	// Test that concurrent reads don't cause issues
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				_ = config.Targets.Availability
				_ = config.ErrorBudget.WindowDays
				_ = len(config.Windows)
				err := config.Validate()
				require.NoError(t, err)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
