package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests to improve types.go coverage
func TestConfig_Merge_AdditionalCases(t *testing.T) {
	t.Run("merge with nil other config", func(t *testing.T) {
		base := NewConfig()
		base.Environment = "production"

		result := base.Merge(nil)
		assert.Equal(t, base.Environment, result.Environment)
		assert.NotSame(t, base, result) // should be a copy
	})

	t.Run("merge performance config fields", func(t *testing.T) {
		base := NewConfig()
		override := &Config{
			Performance: PerformanceConfig{
				ReconcileTimeout:            10 * time.Minute,
				RequeueInterval:             10 * time.Minute,
				RequeueIntervalOnError:      2 * time.Minute,
				LeaderElectionRenewDeadline: 12 * time.Second,
				LeaderElectionRetryPeriod:   3 * time.Second,
			},
		}

		result := base.Merge(override)
		assert.Equal(t, 10*time.Minute, result.Performance.ReconcileTimeout)
		assert.Equal(t, 10*time.Minute, result.Performance.RequeueInterval)
		assert.Equal(t, 2*time.Minute, result.Performance.RequeueIntervalOnError)
		assert.Equal(t, 12*time.Second, result.Performance.LeaderElectionRenewDeadline)
		assert.Equal(t, 3*time.Second, result.Performance.LeaderElectionRetryPeriod)
	})

	t.Run("merge feature flags - nil to non-nil", func(t *testing.T) {
		base := &Config{
			Environment:   "test",
			Features:      nil,
			ConfigSources: make(map[string]ConfigSource),
		}
		override := &Config{
			Features: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
				CustomFlags:    map[string]bool{"test": true},
			},
		}

		result := base.Merge(override)
		require.NotNil(t, result.Features)
		assert.True(t, result.Features.EnableWebhooks)
		assert.False(t, result.Features.EnableMetrics)
		assert.True(t, result.Features.CustomFlags["test"])
	})

	t.Run("merge operator config - leader election boolean", func(t *testing.T) {
		base := NewConfig()
		base.Operator.LeaderElection = true

		override := &Config{
			Operator: OperatorConfig{
				// LeaderElection remains false (default value), so should not override
			},
		}

		result := base.Merge(override)
		assert.True(t, result.Operator.LeaderElection) // should keep base value
	})
}

func TestConfig_Copy_AdditionalCases(t *testing.T) {
	t.Run("copy with nil config", func(t *testing.T) {
		var config *Config = nil
		result := config.Copy()
		assert.Nil(t, result)
	})

	t.Run("copy with nil features", func(t *testing.T) {
		config := &Config{
			Environment:   "test",
			Features:      nil,
			ConfigSources: map[string]ConfigSource{"test": ConfigSourceEnv},
		}

		result := config.Copy()
		assert.Equal(t, "test", result.Environment)
		assert.Nil(t, result.Features)
		assert.Equal(t, ConfigSourceEnv, result.ConfigSources["test"])
	})

	t.Run("copy with nil custom flags", func(t *testing.T) {
		config := &Config{
			Environment: "test",
			Features: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags:    nil,
			},
			ConfigSources: make(map[string]ConfigSource),
		}

		result := config.Copy()
		require.NotNil(t, result.Features)
		assert.True(t, result.Features.EnableWebhooks)
		assert.NotNil(t, result.Features.CustomFlags) // should be initialized
		assert.Empty(t, result.Features.CustomFlags)
	})
}

func TestGetEnvironmentDefaults_AdditionalCases(t *testing.T) {
	t.Run("unknown environment returns production defaults", func(t *testing.T) {
		config := GetEnvironmentDefaults("unknown-env")
		assert.Equal(t, "unknown-env", config.Environment)
		// Should have production-like defaults
		assert.Equal(t, "info", config.Operator.LogLevel)
		assert.Equal(t, 30*time.Second, config.Cloudflare.APITimeout)
		assert.Equal(t, 10, config.Cloudflare.RateLimitRPS)
	})

	t.Run("staging environment has correct defaults", func(t *testing.T) {
		config := GetEnvironmentDefaults("staging")
		assert.Equal(t, "staging", config.Environment)
		assert.Equal(t, "debug", config.Operator.LogLevel)
		assert.Equal(t, 1*time.Minute, config.Operator.ReconcileInterval)
		assert.Equal(t, 15*time.Second, config.Cloudflare.APITimeout)
		assert.Equal(t, 5, config.Cloudflare.RateLimitRPS)
	})

	t.Run("development environment has correct defaults", func(t *testing.T) {
		config := GetEnvironmentDefaults("development")
		assert.Equal(t, "development", config.Environment)
		assert.Equal(t, "debug", config.Operator.LogLevel)
		assert.Equal(t, 30*time.Second, config.Operator.ReconcileInterval)
		assert.Equal(t, 10*time.Second, config.Cloudflare.APITimeout)
		assert.Equal(t, 2, config.Cloudflare.RateLimitRPS)
		assert.True(t, config.Features.ExperimentalFeatures)
	})
}

func TestConfig_FeatureFlags_EdgeCases(t *testing.T) {
	t.Run("merge feature flags preserves original when no custom flags in override", func(t *testing.T) {
		base := &Config{
			Features: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags:    map[string]bool{"original": true},
			},
			ConfigSources: make(map[string]ConfigSource),
		}
		override := &Config{
			Features: &FeatureFlags{
				EnableMetrics: true,
				// No CustomFlags
			},
		}

		result := base.Merge(override)
		require.NotNil(t, result.Features)
		assert.True(t, result.Features.EnableWebhooks)
		assert.True(t, result.Features.EnableMetrics)
		assert.True(t, result.Features.CustomFlags["original"])
	})

	t.Run("merge feature flags with empty custom flags in override", func(t *testing.T) {
		base := &Config{
			Features: &FeatureFlags{
				CustomFlags: map[string]bool{"base": true},
			},
			ConfigSources: make(map[string]ConfigSource),
		}
		override := &Config{
			Features: &FeatureFlags{
				CustomFlags: map[string]bool{}, // empty but not nil
			},
		}

		result := base.Merge(override)
		require.NotNil(t, result.Features)
		// Base custom flags should be preserved
		assert.True(t, result.Features.CustomFlags["base"])
	})
}

// Test edge cases for Merge function
func TestConfig_Merge_EdgeCases(t *testing.T) {
	t.Run("merge with nil config", func(t *testing.T) {
		base := NewConfig()
		result := base.Merge(nil)

		// Should return a copy of base
		assert.NotSame(t, base, result)
		assert.Equal(t, base.Environment, result.Environment)
	})

	t.Run("merge with nil features in base", func(t *testing.T) {
		base := &Config{
			Environment: "production",
			Features:    nil,
		}

		override := &Config{
			Features: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags:    map[string]bool{"test": true},
			},
		}

		result := base.Merge(override)
		assert.NotNil(t, result.Features)
		assert.True(t, result.Features.EnableWebhooks)
		assert.True(t, result.Features.CustomFlags["test"])
	})

	t.Run("merge performance config zero values", func(t *testing.T) {
		base := NewConfig()
		override := &Config{
			Performance: PerformanceConfig{
				MaxConcurrentReconciles: 0, // Zero value, should be ignored
				ReconcileTimeout:        10 * time.Minute,
			},
		}

		result := base.Merge(override)

		// Zero value should be ignored
		assert.Equal(t, 5, result.Performance.MaxConcurrentReconciles)       // Original value
		assert.Equal(t, 10*time.Minute, result.Performance.ReconcileTimeout) // Merged value
	})
}

// Test edge cases for IsValid functions
func TestConfig_IsValid_EdgeCases(t *testing.T) {
	t.Run("operator config with empty metrics address", func(t *testing.T) {
		config := &OperatorConfig{
			LogLevel:               "info",
			ReconcileInterval:      5 * time.Minute,
			MetricsBindAddress:     "", // Empty should be valid
			HealthProbeBindAddress: ":8081",
		}

		err := config.IsValid()
		assert.NoError(t, err)
	})

	t.Run("cloudflare config with zero retry attempts", func(t *testing.T) {
		config := &CloudflareConfig{
			APITimeout:    30 * time.Second,
			RateLimitRPS:  10,
			RetryAttempts: 0, // Zero should be valid
			RetryDelay:    1 * time.Second,
		}

		err := config.IsValid()
		assert.NoError(t, err)
	})

	t.Run("performance config with zero renew deadline", func(t *testing.T) {
		config := &PerformanceConfig{
			MaxConcurrentReconciles:     5,
			ReconcileTimeout:            5 * time.Minute,
			RequeueInterval:             5 * time.Minute,
			RequeueIntervalOnError:      1 * time.Minute,
			ResyncPeriod:                10 * time.Minute,
			LeaderElectionLeaseDuration: 15 * time.Second,
			LeaderElectionRenewDeadline: 0, // Zero should be valid (optional)
			LeaderElectionRetryPeriod:   2 * time.Second,
		}

		err := config.IsValid()
		assert.NoError(t, err)
	})
}

func TestConfigSource_String_AllValues(t *testing.T) {
	tests := []struct {
		source   ConfigSource
		expected string
	}{
		{ConfigSourceDefault, "default"},
		{ConfigSourceEnv, "env"},
		{ConfigSourceFile, "file"},
		{ConfigSourceConfigMap, "configmap"},
		{ConfigSourceSecret, "secret"},
		{ConfigSource(99), "unknown"}, // invalid value
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			assert.Equal(t, test.expected, test.source.String())
		})
	}
}

func TestOperatorConfig_IsValid_EdgeCases(t *testing.T) {
	t.Run("empty log level fails", func(t *testing.T) {
		config := OperatorConfig{
			LogLevel:          "", // empty
			ReconcileInterval: 1 * time.Minute,
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "log level cannot be empty")
	})

	t.Run("very short reconcile interval fails", func(t *testing.T) {
		config := OperatorConfig{
			LogLevel:          "info",
			ReconcileInterval: 500 * time.Millisecond, // too short
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reconcile interval")
	})

	t.Run("very long reconcile interval fails", func(t *testing.T) {
		config := OperatorConfig{
			LogLevel:          "info",
			ReconcileInterval: 2 * time.Hour, // too long
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reconcile interval")
	})
}

func TestCloudflareConfig_IsValid_EdgeCases(t *testing.T) {
	t.Run("very short API timeout fails", func(t *testing.T) {
		config := CloudflareConfig{
			APITimeout:   500 * time.Millisecond, // too short
			RateLimitRPS: 10,
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API timeout")
	})

	t.Run("very long API timeout fails", func(t *testing.T) {
		config := CloudflareConfig{
			APITimeout:   10 * time.Minute, // too long
			RateLimitRPS: 10,
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API timeout")
	})

	t.Run("very long retry delay fails", func(t *testing.T) {
		config := CloudflareConfig{
			APITimeout:    30 * time.Second,
			RateLimitRPS:  10,
			RetryAttempts: 3,
			RetryDelay:    45 * time.Second, // too long
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retry delay")
	})
}
