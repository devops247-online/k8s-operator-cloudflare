package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid production config",
			config:  NewConfig(),
			wantErr: false,
		},
		{
			name:    "valid staging config",
			config:  GetEnvironmentDefaults("staging"),
			wantErr: false,
		},
		{
			name:    "valid development config",
			config:  GetEnvironmentDefaults("development"),
			wantErr: false,
		},
		{
			name: "invalid environment",
			config: &Config{
				Environment: "invalid",
				Operator: OperatorConfig{
					LogLevel:          "info",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 10,
				},
				Performance: PerformanceConfig{
					MaxConcurrentReconciles:     5,
					ResyncPeriod:                10 * time.Minute,
					LeaderElectionLeaseDuration: 15 * time.Second,
				},
			},
			wantErr: true,
			errMsg:  "invalid environment",
		},
		{
			name: "invalid operator config",
			config: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "invalid",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 10,
				},
				Performance: PerformanceConfig{
					MaxConcurrentReconciles:     5,
					ResyncPeriod:                10 * time.Minute,
					LeaderElectionLeaseDuration: 15 * time.Second,
				},
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "invalid cloudflare config",
			config: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "info",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   0,
					RateLimitRPS: 10,
				},
				Performance: PerformanceConfig{
					MaxConcurrentReconciles:     5,
					ResyncPeriod:                10 * time.Minute,
					LeaderElectionLeaseDuration: 15 * time.Second,
				},
			},
			wantErr: true,
			errMsg:  "API timeout must be positive",
		},
		{
			name: "invalid performance config",
			config: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "info",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 10,
				},
				Performance: PerformanceConfig{
					MaxConcurrentReconciles:     0,
					ResyncPeriod:                10 * time.Minute,
					LeaderElectionLeaseDuration: 15 * time.Second,
				},
			},
			wantErr: true,
			errMsg:  "max concurrent reconciles must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironment(t *testing.T) {
	tests := []struct {
		env     string
		wantErr bool
	}{
		{"production", false},
		{"staging", false},
		{"development", false},
		{"test", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			err := ValidateEnvironment(tt.env)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"fatal", false},
		{"panic", false},
		{"trace", false},
		{"invalid", true},
		{"", true},
		{"DEBUG", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			err := ValidateLogLevel(tt.level)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		min      time.Duration
		max      time.Duration
		wantErr  bool
	}{
		{
			name:     "valid duration",
			duration: 5 * time.Minute,
			min:      1 * time.Second,
			max:      10 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "duration too small",
			duration: 500 * time.Millisecond,
			min:      1 * time.Second,
			max:      10 * time.Minute,
			wantErr:  true,
		},
		{
			name:     "duration too large",
			duration: 15 * time.Minute,
			min:      1 * time.Second,
			max:      10 * time.Minute,
			wantErr:  true,
		},
		{
			name:     "zero duration invalid",
			duration: 0,
			min:      1 * time.Second,
			max:      10 * time.Minute,
			wantErr:  true,
		},
		{
			name:     "negative duration invalid",
			duration: -1 * time.Second,
			min:      1 * time.Second,
			max:      10 * time.Minute,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDuration(tt.duration, tt.min, tt.max, "test duration")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"positive value", 5, false},
		{"zero value", 0, true},
		{"negative value", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveInt(tt.value, "test value")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePortRange(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{8080, false},
		{1, false},
		{65535, false},
		{0, true},
		{-1, true},
		{65536, true},
		{100000, true},
	}

	for _, tt := range tests {
		t.Run("port_"+string(rune(tt.port)), func(t *testing.T) {
			err := ValidatePortRange(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFeatureFlags_Validate(t *testing.T) {
	tests := []struct {
		name    string
		flags   *FeatureFlags
		wantErr bool
	}{
		{
			name: "valid feature flags",
			flags: &FeatureFlags{
				EnableWebhooks:       true,
				EnableMetrics:        true,
				EnableTracing:        false,
				ExperimentalFeatures: false,
				CustomFlags: map[string]bool{
					"testFlag": true,
				},
			},
			wantErr: false,
		},
		{
			name:    "nil feature flags",
			flags:   nil,
			wantErr: false,
		},
		{
			name: "feature flags with nil custom flags",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags:    nil,
			},
			wantErr: false,
		},
		{
			name: "feature flags with empty custom flags",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags:    map[string]bool{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.flags.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationHelper_CustomValidation(t *testing.T) {
	t.Run("validates complex configuration scenarios", func(t *testing.T) {
		config := &Config{
			Environment: "production",
			Operator: OperatorConfig{
				LogLevel:               "info",
				ReconcileInterval:      5 * time.Minute,
				MetricsBindAddress:     ":8080",
				HealthProbeBindAddress: ":8081",
				LeaderElection:         true,
			},
			Cloudflare: CloudflareConfig{
				APITimeout:    30 * time.Second,
				RateLimitRPS:  10,
				RetryAttempts: 3,
				RetryDelay:    1 * time.Second,
			},
			Features: &FeatureFlags{
				EnableWebhooks:       true,
				EnableMetrics:        true,
				EnableTracing:        false,
				ExperimentalFeatures: false,
			},
			Performance: PerformanceConfig{
				MaxConcurrentReconciles:     5,
				ReconcileTimeout:            5 * time.Minute,
				RequeueInterval:             5 * time.Minute,
				RequeueIntervalOnError:      1 * time.Minute,
				ResyncPeriod:                10 * time.Minute,
				LeaderElectionLeaseDuration: 15 * time.Second,
				LeaderElectionRenewDeadline: 10 * time.Second,
				LeaderElectionRetryPeriod:   2 * time.Second,
			},
		}

		err := config.Validate()
		require.NoError(t, err)
	})

	t.Run("detects conflicting feature flags in development", func(t *testing.T) {
		config := GetEnvironmentDefaults("development")
		config.Features.ExperimentalFeatures = true
		config.Features.EnableTracing = true

		// This should still be valid as experimental features should work in development
		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestValidationEdgeCases(t *testing.T) {
	t.Run("validates bind addresses format", func(t *testing.T) {
		tests := []struct {
			address string
			valid   bool
		}{
			{":8080", true},
			{"127.0.0.1:8080", true},
			{"0.0.0.0:8080", true},
			{"localhost:8080", true},
			{"8080", false},
			{":0", true}, // OS chooses port
			{":", false},
			{"", false},
			{"invalid:port", false},
		}

		for _, tt := range tests {
			t.Run(tt.address, func(t *testing.T) {
				err := ValidateBindAddress(tt.address)
				if tt.valid {
					assert.NoError(t, err, "address %s should be valid", tt.address)
				} else {
					assert.Error(t, err, "address %s should be invalid", tt.address)
				}
			})
		}
	})

	t.Run("validates duration bounds for different environments", func(t *testing.T) {
		// Production should have stricter limits
		prodConfig := GetEnvironmentDefaults("production")
		prodConfig.Operator.ReconcileInterval = 10 * time.Second // too frequent for production
		err := prodConfig.Validate()
		assert.Error(t, err)

		// Development can have more frequent reconciliation
		devConfig := GetEnvironmentDefaults("development")
		devConfig.Operator.ReconcileInterval = 10 * time.Second
		err = devConfig.Validate()
		assert.NoError(t, err)
	})
}
