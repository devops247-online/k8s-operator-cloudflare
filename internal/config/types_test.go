package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_NewConfig(t *testing.T) {
	t.Run("creates config with defaults", func(t *testing.T) {
		cfg := NewConfig()

		assert.NotNil(t, cfg)
		assert.Equal(t, "production", cfg.Environment)
		assert.Equal(t, "info", cfg.Operator.LogLevel)
		assert.Equal(t, 5*time.Minute, cfg.Operator.ReconcileInterval)
		assert.Equal(t, 30*time.Second, cfg.Cloudflare.APITimeout)
		assert.Equal(t, 10, cfg.Cloudflare.RateLimitRPS)
		assert.NotNil(t, cfg.Features)
		assert.True(t, cfg.Features.EnableMetrics)
		assert.True(t, cfg.Features.EnableWebhooks)
		assert.False(t, cfg.Features.EnableTracing)
		assert.False(t, cfg.Features.ExperimentalFeatures)
	})
}

func TestConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     *Config
		override *Config
		expected *Config
	}{
		{
			name: "merge with empty override",
			base: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel: "info",
				},
				ConfigSources: make(map[string]ConfigSource),
			},
			override: &Config{},
			expected: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel: "info",
				},
				ConfigSources: make(map[string]ConfigSource),
			},
		},
		{
			name: "override environment",
			base: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel: "info",
				},
				ConfigSources: make(map[string]ConfigSource),
			},
			override: &Config{
				Environment: "staging",
			},
			expected: &Config{
				Environment: "staging",
				Operator: OperatorConfig{
					LogLevel: "info",
				},
				ConfigSources: make(map[string]ConfigSource),
			},
		},
		{
			name: "override nested values",
			base: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "info",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 10,
				},
				ConfigSources: make(map[string]ConfigSource),
			},
			override: &Config{
				Operator: OperatorConfig{
					LogLevel: "debug",
				},
				Cloudflare: CloudflareConfig{
					RateLimitRPS: 5,
				},
			},
			expected: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "debug",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 5,
				},
				ConfigSources: make(map[string]ConfigSource),
			},
		},
		{
			name: "merge feature flags",
			base: &Config{
				Features: &FeatureFlags{
					EnableWebhooks: true,
					EnableMetrics:  true,
					CustomFlags:    make(map[string]bool),
				},
				ConfigSources: make(map[string]ConfigSource),
			},
			override: &Config{
				Features: &FeatureFlags{
					EnableTracing:        true,
					ExperimentalFeatures: true,
					CustomFlags:          make(map[string]bool),
				},
			},
			expected: &Config{
				Features: &FeatureFlags{
					EnableWebhooks:       true,
					EnableMetrics:        true,
					EnableTracing:        true,
					ExperimentalFeatures: true,
					CustomFlags:          make(map[string]bool),
				},
				ConfigSources: make(map[string]ConfigSource),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.base.Merge(tt.override)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_Copy(t *testing.T) {
	t.Run("creates deep copy", func(t *testing.T) {
		original := &Config{
			Environment: "production",
			Operator: OperatorConfig{
				LogLevel:          "info",
				ReconcileInterval: 5 * time.Minute,
			},
			Cloudflare: CloudflareConfig{
				APITimeout:   30 * time.Second,
				RateLimitRPS: 10,
			},
			Features: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  true,
				CustomFlags:    make(map[string]bool),
			},
			Performance: PerformanceConfig{
				MaxConcurrentReconciles:     5,
				ResyncPeriod:                10 * time.Minute,
				LeaderElectionLeaseDuration: 15 * time.Second,
			},
			ConfigSources: make(map[string]ConfigSource),
		}

		copied := original.Copy()

		// Verify values are copied
		assert.Equal(t, original, copied)

		// Verify it's a deep copy by modifying the copy
		copied.Environment = "staging"
		copied.Operator.LogLevel = "debug"
		copied.Features.EnableWebhooks = false

		// Original should remain unchanged
		assert.Equal(t, "production", original.Environment)
		assert.Equal(t, "info", original.Operator.LogLevel)
		assert.True(t, original.Features.EnableWebhooks)
	})
}

func TestOperatorConfig_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		config  OperatorConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: OperatorConfig{
				LogLevel:          "info",
				ReconcileInterval: 5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: OperatorConfig{
				LogLevel:          "invalid",
				ReconcileInterval: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "zero reconcile interval",
			config: OperatorConfig{
				LogLevel:          "info",
				ReconcileInterval: 0,
			},
			wantErr: true,
		},
		{
			name: "negative reconcile interval",
			config: OperatorConfig{
				LogLevel:          "info",
				ReconcileInterval: -1 * time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.IsValid()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCloudflareConfig_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		config  CloudflareConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: CloudflareConfig{
				APITimeout:   30 * time.Second,
				RateLimitRPS: 10,
			},
			wantErr: false,
		},
		{
			name: "zero timeout",
			config: CloudflareConfig{
				APITimeout:   0,
				RateLimitRPS: 10,
			},
			wantErr: true,
		},
		{
			name: "zero rate limit",
			config: CloudflareConfig{
				APITimeout:   30 * time.Second,
				RateLimitRPS: 0,
			},
			wantErr: true,
		},
		{
			name: "negative rate limit",
			config: CloudflareConfig{
				APITimeout:   30 * time.Second,
				RateLimitRPS: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.IsValid()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPerformanceConfig_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		config  PerformanceConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: PerformanceConfig{
				MaxConcurrentReconciles:     5,
				ReconcileTimeout:            5 * time.Minute,
				RequeueInterval:             5 * time.Minute,
				RequeueIntervalOnError:      1 * time.Minute,
				ResyncPeriod:                10 * time.Minute,
				LeaderElectionLeaseDuration: 15 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero concurrent reconciles",
			config: PerformanceConfig{
				MaxConcurrentReconciles:     0,
				ResyncPeriod:                10 * time.Minute,
				LeaderElectionLeaseDuration: 15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero resync period",
			config: PerformanceConfig{
				MaxConcurrentReconciles:     5,
				ResyncPeriod:                0,
				LeaderElectionLeaseDuration: 15 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero lease duration",
			config: PerformanceConfig{
				MaxConcurrentReconciles:     5,
				ResyncPeriod:                10 * time.Minute,
				LeaderElectionLeaseDuration: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.IsValid()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigSource_String(t *testing.T) {
	tests := []struct {
		source   ConfigSource
		expected string
	}{
		{ConfigSourceDefault, "default"},
		{ConfigSourceEnv, "env"},
		{ConfigSourceFile, "file"},
		{ConfigSourceConfigMap, "configmap"},
		{ConfigSourceSecret, "secret"},
		{ConfigSource(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.source.String())
		})
	}
}

func TestConfig_GetEnvironmentDefaults(t *testing.T) {
	tests := []struct {
		env      string
		expected *Config
	}{
		{
			env: "production",
			expected: &Config{
				Environment: "production",
				Operator: OperatorConfig{
					LogLevel:          "info",
					ReconcileInterval: 5 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   30 * time.Second,
					RateLimitRPS: 10,
				},
			},
		},
		{
			env: "staging",
			expected: &Config{
				Environment: "staging",
				Operator: OperatorConfig{
					LogLevel:          "debug",
					ReconcileInterval: 1 * time.Minute,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   15 * time.Second,
					RateLimitRPS: 5,
				},
			},
		},
		{
			env: "development",
			expected: &Config{
				Environment: "development",
				Operator: OperatorConfig{
					LogLevel:          "debug",
					ReconcileInterval: 30 * time.Second,
				},
				Cloudflare: CloudflareConfig{
					APITimeout:   10 * time.Second,
					RateLimitRPS: 2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			result := GetEnvironmentDefaults(tt.env)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected.Environment, result.Environment)
			assert.Equal(t, tt.expected.Operator.LogLevel, result.Operator.LogLevel)
			assert.Equal(t, tt.expected.Operator.ReconcileInterval, result.Operator.ReconcileInterval)
			assert.Equal(t, tt.expected.Cloudflare.APITimeout, result.Cloudflare.APITimeout)
			assert.Equal(t, tt.expected.Cloudflare.RateLimitRPS, result.Cloudflare.RateLimitRPS)
		})
	}
}

func TestConfig_ToJSON(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		cfg := &Config{
			Environment: "test",
			Operator: OperatorConfig{
				LogLevel:          "debug",
				ReconcileInterval: 1 * time.Minute,
			},
			Features: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
			},
		}

		jsonData, err := cfg.ToJSON()
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), `"environment": "test"`)
		assert.Contains(t, string(jsonData), `"logLevel": "debug"`)
		assert.Contains(t, string(jsonData), `"enableWebhooks": true`)
		assert.Contains(t, string(jsonData), `"enableMetrics": false`)
	})
}

func TestConfig_FromJSON(t *testing.T) {
	t.Run("unmarshals from JSON", func(t *testing.T) {
		jsonData := []byte(`{
			"environment": "test",
			"operator": {
				"logLevel": "debug",
				"reconcileInterval": 120000000000
			},
			"features": {
				"enableWebhooks": true,
				"enableMetrics": false
			}
		}`)

		cfg := &Config{}
		err := cfg.FromJSON(jsonData)
		require.NoError(t, err)

		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "debug", cfg.Operator.LogLevel)
		assert.Equal(t, 2*time.Minute, cfg.Operator.ReconcileInterval)
		assert.True(t, cfg.Features.EnableWebhooks)
		assert.False(t, cfg.Features.EnableMetrics)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.FromJSON([]byte("invalid json"))
		assert.Error(t, err)
	})
}
