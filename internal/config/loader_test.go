package config

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfigLoader_LoadFromFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "valid JSON config",
			content: `{
				"environment": "test",
				"operator": {
					"logLevel": "debug",
					"reconcileInterval": 60000000000
				},
				"cloudflare": {
					"apiTimeout": 30000000000,
					"rateLimitRPS": 5
				}
			}`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test", cfg.Environment)
				assert.Equal(t, "debug", cfg.Operator.LogLevel)
				assert.Equal(t, 1*time.Minute, cfg.Operator.ReconcileInterval)
				assert.Equal(t, 5, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name: "valid YAML config",
			content: `
environment: staging
operator:
  logLevel: info
  reconcileInterval: 300000000000
cloudflare:
  apiTimeout: 15000000000
  rateLimitRPS: 8
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "staging", cfg.Environment)
				assert.Equal(t, "info", cfg.Operator.LogLevel)
				assert.Equal(t, 5*time.Minute, cfg.Operator.ReconcileInterval)
				assert.Equal(t, 8, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name:    "invalid JSON",
			content: `{"invalid": json}`,
			wantErr: true,
		},
		{
			name:    "empty file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with appropriate extension
			var tmpFile *os.File
			var err error
			if strings.Contains(tt.name, "YAML") {
				tmpFile, err = os.CreateTemp("", "config-*.yaml")
			} else {
				tmpFile, err = os.CreateTemp("", "config-*.json")
			}
			require.NoError(t, err)
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			_ = tmpFile.Close()

			loader := NewConfigLoader(nil, "test-namespace")
			cfg, err := loader.LoadFromFile(tmpFile.Name())

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.check != nil {
					tt.check(t, cfg)
				}
			}
		})
	}
}

func TestConfigLoader_LoadFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		check   func(*testing.T, *Config)
	}{
		{
			name: "basic environment variables",
			envVars: map[string]string{
				"CONFIG_ENVIRONMENT":                 "staging",
				"CONFIG_OPERATOR_LOG_LEVEL":          "debug",
				"CONFIG_OPERATOR_RECONCILE_INTERVAL": "2m",
				"CONFIG_CLOUDFLARE_API_TIMEOUT":      "45s",
				"CONFIG_CLOUDFLARE_RATE_LIMIT_RPS":   "15",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "staging", cfg.Environment)
				assert.Equal(t, "debug", cfg.Operator.LogLevel)
				assert.Equal(t, 2*time.Minute, cfg.Operator.ReconcileInterval)
				assert.Equal(t, 45*time.Second, cfg.Cloudflare.APITimeout)
				assert.Equal(t, 15, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name: "feature flags from environment",
			envVars: map[string]string{
				"CONFIG_FEATURES_ENABLE_WEBHOOKS":       "true",
				"CONFIG_FEATURES_ENABLE_METRICS":        "false",
				"CONFIG_FEATURES_ENABLE_TRACING":        "true",
				"CONFIG_FEATURES_EXPERIMENTAL_FEATURES": "true",
			},
			check: func(t *testing.T, cfg *Config) {
				require.NotNil(t, cfg.Features)
				assert.True(t, cfg.Features.EnableWebhooks)
				assert.False(t, cfg.Features.EnableMetrics)
				assert.True(t, cfg.Features.EnableTracing)
				assert.True(t, cfg.Features.ExperimentalFeatures)
			},
		},
		{
			name: "performance settings from environment",
			envVars: map[string]string{
				"CONFIG_PERFORMANCE_MAX_CONCURRENT_RECONCILES":      "10",
				"CONFIG_PERFORMANCE_RESYNC_PERIOD":                  "15m",
				"CONFIG_PERFORMANCE_LEADER_ELECTION_LEASE_DURATION": "30s",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 10, cfg.Performance.MaxConcurrentReconciles)
				assert.Equal(t, 15*time.Minute, cfg.Performance.ResyncPeriod)
				assert.Equal(t, 30*time.Second, cfg.Performance.LeaderElectionLeaseDuration)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
				defer func() { _ = os.Unsetenv(key) }()
			}

			loader := NewConfigLoader(nil, "test-namespace")
			cfg, err := loader.LoadFromEnv()

			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConfigLoader_LoadFromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name      string
		configMap *corev1.ConfigMap
		wantErr   bool
		check     func(*testing.T, *Config)
	}{
		{
			name: "valid ConfigMap with JSON",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"config.json": `{
						"environment": "production",
						"operator": {
							"logLevel": "info",
							"reconcileInterval": 300000000000
						}
					}`,
				},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "production", cfg.Environment)
				assert.Equal(t, "info", cfg.Operator.LogLevel)
				assert.Equal(t, 5*time.Minute, cfg.Operator.ReconcileInterval)
			},
		},
		{
			name: "valid ConfigMap with YAML",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"config.yaml": `
environment: development
operator:
  logLevel: debug
  reconcileInterval: 60000000000
`,
				},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "development", cfg.Environment)
				assert.Equal(t, "debug", cfg.Operator.LogLevel)
				assert.Equal(t, 1*time.Minute, cfg.Operator.ReconcileInterval)
			},
		},
		{
			name: "ConfigMap with individual keys",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"environment":    "staging",
					"log-level":      "warn",
					"api-timeout":    "20s",
					"rate-limit-rps": "12",
				},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "staging", cfg.Environment)
				assert.Equal(t, "warn", cfg.Operator.LogLevel)
				assert.Equal(t, 20*time.Second, cfg.Cloudflare.APITimeout)
				assert.Equal(t, 12, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name:      "missing ConfigMap",
			configMap: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fakeClient client.Client
			if tt.configMap != nil {
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(tt.configMap).
					Build()
			} else {
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					Build()
			}

			loader := NewConfigLoader(fakeClient, "test-namespace")
			cfg, err := loader.LoadFromConfigMap(context.Background(), "operator-config")

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.check != nil {
					tt.check(t, cfg)
				}
			}
		})
	}
}

func TestConfigLoader_LoadFromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "valid Secret with JSON config",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-secret-config",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"config.json": []byte(`{
						"environment": "production",
						"cloudflare": {
							"apiTimeout": 60000000000,
							"rateLimitRPS": 20
						}
					}`),
				},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "production", cfg.Environment)
				assert.Equal(t, 1*time.Minute, cfg.Cloudflare.APITimeout)
				assert.Equal(t, 20, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name: "Secret with individual sensitive keys",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-secret-config",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"api-timeout":    []byte("45s"),
					"rate-limit-rps": []byte("25"),
				},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 45*time.Second, cfg.Cloudflare.APITimeout)
				assert.Equal(t, 25, cfg.Cloudflare.RateLimitRPS)
			},
		},
		{
			name:    "missing Secret",
			secret:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fakeClient client.Client
			if tt.secret != nil { // pragma: allowlist secret
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(tt.secret).
					Build()
			} else {
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					Build()
			}

			loader := NewConfigLoader(fakeClient, "test-namespace")
			cfg, err := loader.LoadFromSecret(context.Background(), "operator-secret-config")

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.check != nil {
					tt.check(t, cfg)
				}
			}
		})
	}
}

func TestConfigLoader_LoadWithPriority(t *testing.T) {
	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "config-*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fileContent := `{
		"environment": "file-env",
		"operator": {
			"logLevel": "file-level",
			"reconcileInterval": 180000000000
		}
	}`
	_, err = tmpFile.WriteString(fileContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set environment variables
	envVars := map[string]string{
		"CONFIG_ENVIRONMENT":               "test",
		"CONFIG_OPERATOR_LOG_LEVEL":        "debug",
		"CONFIG_CLOUDFLARE_RATE_LIMIT_RPS": "99",
	}
	for key, value := range envVars {
		_ = os.Setenv(key, value)
		defer func() { _ = os.Unsetenv(key) }()
	}

	// Create ConfigMap
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"environment": "configmap-env",
			"api-timeout": "25s",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	loader := NewConfigLoader(fakeClient, "test-namespace")

	t.Run("loads with proper priority: env > configmap > file > defaults", func(t *testing.T) {
		cfg, sources, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			FilePath:      tmpFile.Name(),
			ConfigMapName: "operator-config",
			LoadFromEnv:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotEmpty(t, sources)

		// Environment variables should override everything
		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "debug", cfg.Operator.LogLevel)
		assert.Equal(t, 99, cfg.Cloudflare.RateLimitRPS)

		// ConfigMap should override file and defaults for non-env fields
		assert.Equal(t, 25*time.Second, cfg.Cloudflare.APITimeout)

		// File should override defaults for fields not in env or configmap
		assert.Equal(t, 3*time.Minute, cfg.Operator.ReconcileInterval)

		// Check config sources tracking
		assert.Equal(t, ConfigSourceEnv, sources["environment"])
		assert.Equal(t, ConfigSourceEnv, sources["operator.logLevel"])
		assert.Equal(t, ConfigSourceConfigMap, sources["cloudflare.apiTimeout"])
	})

	t.Run("validates merged configuration", func(t *testing.T) {
		cfg, _, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			FilePath:       tmpFile.Name(),
			ConfigMapName:  "operator-config",
			LoadFromEnv:    true,
			ValidateConfig: true,
		})

		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Configuration should be valid
		err = cfg.Validate()
		assert.NoError(t, err)
	})
}

func TestConfigLoader_WatchConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"environment": "staging",
			"log-level":   "info",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	loader := NewConfigLoader(fakeClient, "test-namespace")

	t.Run("watches for config changes", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		configChan, errorChan := loader.WatchConfig(ctx, WatchOptions{
			ConfigMapName: "operator-config",
			Interval:      1 * time.Second,
		})

		// Should receive initial config
		select {
		case cfg := <-configChan:
			assert.Equal(t, "staging", cfg.Environment)
			assert.Equal(t, "info", cfg.Operator.LogLevel)
		case err := <-errorChan:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatal("timeout waiting for initial config")
		}

		// Note: In a real test environment, we would update the ConfigMap
		// and verify that changes are detected, but with fake client this is complex
	})
}

func TestLoadOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options LoadOptions
		wantErr bool
	}{
		{
			name: "valid options",
			options: LoadOptions{
				FilePath:      "/path/to/config.json",
				ConfigMapName: "config",
				LoadFromEnv:   true,
			},
			wantErr: false,
		},
		{
			name: "valid options with minimal settings",
			options: LoadOptions{
				LoadFromEnv: true,
			},
			wantErr: false,
		},
		{
			name: "invalid - no sources specified",
			options: LoadOptions{
				LoadFromEnv: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWatchOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options WatchOptions
		wantErr bool
	}{
		{
			name: "valid watch options",
			options: WatchOptions{
				ConfigMapName: "config",
				Interval:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid - no resources to watch",
			options: WatchOptions{
				Interval: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid - too short interval",
			options: WatchOptions{
				ConfigMapName: "config",
				Interval:      500 * time.Millisecond,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigLoader_EdgeCases(t *testing.T) {
	t.Run("handles file permissions error", func(t *testing.T) {
		// Create a file we can't read
		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		_ = tmpFile.Close()
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		// Remove read permissions
		err = os.Chmod(tmpFile.Name(), 0000)
		require.NoError(t, err)

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromFile(tmpFile.Name())

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("handles directory instead of file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "config-dir")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromFile(tmpDir)

		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("handles malformed environment variable values", func(t *testing.T) {
		_ = os.Setenv("CONFIG_OPERATOR_RECONCILE_INTERVAL", "invalid-duration")
		defer func() { _ = os.Unsetenv("CONFIG_OPERATOR_RECONCILE_INTERVAL") }()

		_ = os.Setenv("CONFIG_CLOUDFLARE_RATE_LIMIT_RPS", "not-a-number")
		defer func() { _ = os.Unsetenv("CONFIG_CLOUDFLARE_RATE_LIMIT_RPS") }()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromEnv()

		// Should not error, but should ignore invalid values
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		// Should ignore invalid env vars (they will be zero values)
		assert.Equal(t, time.Duration(0), cfg.Operator.ReconcileInterval)
		assert.Equal(t, 0, cfg.Cloudflare.RateLimitRPS)
	})
}

// Test WatchConfig error cases
func TestConfigLoader_WatchConfig_ErrorCases(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("watch config with empty options", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		// Empty options should return error via error channel
		watchOptions := WatchOptions{}
		_, errorChan := loader.WatchConfig(context.Background(), watchOptions)

		// Should receive error from error channel
		select {
		case err := <-errorChan:
			assert.Error(t, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected error from error channel")
		}
	})

	t.Run("watch config with nonexistent configmap", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		watchOptions := WatchOptions{
			ConfigMapName: "nonexistent-config",
			Interval:      1 * time.Second,
		}

		_, errorChan := loader.WatchConfig(context.Background(), watchOptions)

		// Should receive error due to nonexistent ConfigMap
		select {
		case err := <-errorChan:
			assert.Error(t, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected error from error channel")
		}
	})
}

// Test error cases in LoadFromFile
func TestConfigLoader_LoadFromFile_ErrorCases(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load from nonexistent file", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		_, err := loader.LoadFromFile("/nonexistent/file.yaml")
		assert.Error(t, err)
	})

	t.Run("load from invalid yaml file", func(t *testing.T) {
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		// Create temporary invalid YAML file
		tmpFile, err := os.CreateTemp("", "invalid_*.yaml")
		assert.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		_, err = tmpFile.WriteString("invalid: yaml: content: [}")
		assert.NoError(t, err)
		_ = tmpFile.Close()

		_, err = loader.LoadFromFile(tmpFile.Name())
		assert.Error(t, err)
	})
}

// Test error cases in LoadFromConfigMap
func TestConfigLoader_LoadFromConfigMap_ErrorCases(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load from nonexistent configmap", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		_, err := loader.LoadFromConfigMap(context.Background(), "nonexistent-config")
		assert.Error(t, err)
	})

	t.Run("load from configmap with nil client", func(t *testing.T) {
		loader := NewConfigLoader(nil, "test-namespace")

		_, err := loader.LoadFromConfigMap(context.Background(), "test-config")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubernetes client is required")
	})
}

// Test error cases in LoadFromSecret
func TestConfigLoader_LoadFromSecret_ErrorCases(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load from nonexistent secret", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		loader := NewConfigLoader(fakeClient, "test-namespace")

		_, err := loader.LoadFromSecret(context.Background(), "nonexistent-secret")
		assert.Error(t, err)
	})

	t.Run("load from secret with nil client", func(t *testing.T) {
		loader := NewConfigLoader(nil, "test-namespace")

		_, err := loader.LoadFromSecret(context.Background(), "test-secret")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubernetes client is required")
	})
}

// Test additional coverage for LoadFromFile with extensions and error paths
func TestConfigLoader_LoadFromFile_Coverage(t *testing.T) {
	t.Run("load from file with no extension tries JSON then YAML", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "config")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		validYAML := `
environment: test
operator:
  logLevel: info
`
		_, err = tmpFile.WriteString(validYAML)
		require.NoError(t, err)
		_ = tmpFile.Close()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromFile(tmpFile.Name())

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "info", cfg.Operator.LogLevel)
	})

	t.Run("load from file with no extension fails both JSON and YAML", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "config")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		invalidContent := "invalid content not json or yaml"
		_, err = tmpFile.WriteString(invalidContent)
		require.NoError(t, err)
		_ = tmpFile.Close()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromFile(tmpFile.Name())

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	t.Run("load from empty file path", func(t *testing.T) {
		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromFile("")

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "file path cannot be empty")
	})
}

// Test additional coverage for LoadFromEnv with more environment variables
func TestConfigLoader_LoadFromEnv_Coverage(t *testing.T) {
	t.Run("load all environment variables", func(t *testing.T) {
		envVars := map[string]string{
			"CONFIG_ENVIRONMENT":                                "test",
			"CONFIG_OPERATOR_LOG_LEVEL":                         "debug",
			"CONFIG_OPERATOR_RECONCILE_INTERVAL":                "2m",
			"CONFIG_OPERATOR_METRICS_BIND_ADDRESS":              ":8080",
			"CONFIG_OPERATOR_HEALTH_PROBE_BIND_ADDRESS":         ":8081",
			"CONFIG_OPERATOR_LEADER_ELECTION":                   "true",
			"CONFIG_CLOUDFLARE_API_TIMEOUT":                     "45s",
			"CONFIG_CLOUDFLARE_RATE_LIMIT_RPS":                  "15",
			"CONFIG_CLOUDFLARE_RETRY_ATTEMPTS":                  "5",
			"CONFIG_CLOUDFLARE_RETRY_DELAY":                     "2s",
			"CONFIG_FEATURES_ENABLE_WEBHOOKS":                   "true",
			"CONFIG_FEATURES_ENABLE_METRICS":                    "false",
			"CONFIG_FEATURES_ENABLE_TRACING":                    "true",
			"CONFIG_FEATURES_EXPERIMENTAL_FEATURES":             "true",
			"CONFIG_PERFORMANCE_MAX_CONCURRENT_RECONCILES":      "10",
			"CONFIG_PERFORMANCE_RESYNC_PERIOD":                  "15m",
			"CONFIG_PERFORMANCE_LEADER_ELECTION_LEASE_DURATION": "30s",
			"CONFIG_PERFORMANCE_LEADER_ELECTION_RENEW_DEADLINE": "20s",
			"CONFIG_PERFORMANCE_LEADER_ELECTION_RETRY_PERIOD":   "5s",
		}

		for key, value := range envVars {
			_ = os.Setenv(key, value)
			defer func() { _ = os.Unsetenv(key) }()
		}

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, err := loader.LoadFromEnv()

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "debug", cfg.Operator.LogLevel)
		assert.Equal(t, 2*time.Minute, cfg.Operator.ReconcileInterval)
		assert.Equal(t, ":8080", cfg.Operator.MetricsBindAddress)
		assert.Equal(t, ":8081", cfg.Operator.HealthProbeBindAddress)
		assert.True(t, cfg.Operator.LeaderElection)
		assert.Equal(t, 45*time.Second, cfg.Cloudflare.APITimeout)
		assert.Equal(t, 15, cfg.Cloudflare.RateLimitRPS)
		assert.Equal(t, 5, cfg.Cloudflare.RetryAttempts)
		assert.Equal(t, 2*time.Second, cfg.Cloudflare.RetryDelay)
		assert.True(t, cfg.Features.EnableWebhooks)
		assert.False(t, cfg.Features.EnableMetrics)
		assert.True(t, cfg.Features.EnableTracing)
		assert.True(t, cfg.Features.ExperimentalFeatures)
		assert.Equal(t, 10, cfg.Performance.MaxConcurrentReconciles)
		assert.Equal(t, 15*time.Minute, cfg.Performance.ResyncPeriod)
		assert.Equal(t, 30*time.Second, cfg.Performance.LeaderElectionLeaseDuration)
		assert.Equal(t, 20*time.Second, cfg.Performance.LeaderElectionRenewDeadline)
		assert.Equal(t, 5*time.Second, cfg.Performance.LeaderElectionRetryPeriod)
		// Note: ReconcileTimeout, RequeueInterval, RequeueIntervalOnError
		// are not supported via environment variables in LoadFromEnv
	})
}

// Test additional coverage for LoadFromSecret
func TestConfigLoader_LoadFromSecret_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load from secret with YAML config", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-secret-config",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"config.yaml": []byte(`
environment: production
operator:
  logLevel: warn
  reconcileInterval: 300000000000
cloudflare:
  apiTimeout: 60000000000
  rateLimitRPS: 20
`),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(secret).
			Build()

		loader := NewConfigLoader(fakeClient, "test-namespace")
		cfg, err := loader.LoadFromSecret(context.Background(), "operator-secret-config")

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "production", cfg.Environment)
		assert.Equal(t, "warn", cfg.Operator.LogLevel)
		assert.Equal(t, 5*time.Minute, cfg.Operator.ReconcileInterval)
		assert.Equal(t, 1*time.Minute, cfg.Cloudflare.APITimeout)
		assert.Equal(t, 20, cfg.Cloudflare.RateLimitRPS)
	})

	t.Run("load from secret with both JSON and individual keys", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-secret-config",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"config.json":    []byte(`{"environment": "staging"}`),
				"api-timeout":    []byte("30s"),
				"rate-limit-rps": []byte("15"),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(secret).
			Build()

		loader := NewConfigLoader(fakeClient, "test-namespace")
		cfg, err := loader.LoadFromSecret(context.Background(), "operator-secret-config")

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		// JSON config should take precedence
		assert.Equal(t, "staging", cfg.Environment)
	})
}

// Test additional coverage for LoadFromConfigMap
func TestConfigLoader_LoadFromConfigMap_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load from configmap with invalid JSON falls back to individual keys", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-config",
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"config.json": "invalid json {",
				"environment": "staging",
				"log-level":   "debug",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(configMap).
			Build()

		loader := NewConfigLoader(fakeClient, "test-namespace")
		cfg, err := loader.LoadFromConfigMap(context.Background(), "operator-config")

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		// Should fall back to individual keys
		assert.Equal(t, "staging", cfg.Environment)
		assert.Equal(t, "debug", cfg.Operator.LogLevel)
	})

	t.Run("load from configmap with invalid YAML falls back to individual keys", func(t *testing.T) {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-config",
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"config.yaml": "invalid yaml [",
				"environment": "development",
				"api-timeout": "25s",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(configMap).
			Build()

		loader := NewConfigLoader(fakeClient, "test-namespace")
		cfg, err := loader.LoadFromConfigMap(context.Background(), "operator-config")

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		// Should fall back to individual keys
		assert.Equal(t, "development", cfg.Environment)
		assert.Equal(t, 25*time.Second, cfg.Cloudflare.APITimeout)
	})
}

// Test additional coverage for LoadWithPriority
func TestConfigLoader_LoadWithPriority_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	t.Run("load with priority and validation failure", func(t *testing.T) {
		// Create invalid config in environment
		_ = os.Setenv("CONFIG_ENVIRONMENT", "invalid-environment")
		defer func() { _ = os.Unsetenv("CONFIG_ENVIRONMENT") }()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, sources, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			LoadFromEnv:    true,
			ValidateConfig: true,
		})

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Nil(t, sources)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("load with priority file only", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "config-*.json")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		content := `{"environment": "test", "operator": {"logLevel": "info"}}`
		_, err = tmpFile.WriteString(content)
		require.NoError(t, err)
		_ = tmpFile.Close()

		loader := NewConfigLoader(nil, "test-namespace")
		cfg, sources, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			FilePath: tmpFile.Name(),
		})

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.NotNil(t, sources)
		assert.Equal(t, "test", cfg.Environment)
		assert.Equal(t, "info", cfg.Operator.LogLevel)
	})

	t.Run("load with priority secret only", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"environment": []byte("secret-env"),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(secret).
			Build()

		loader := NewConfigLoader(fakeClient, "test-namespace")
		cfg, sources, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			SecretName: "test-secret",
		})

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.NotNil(t, sources)
		assert.Equal(t, "secret-env", cfg.Environment)
	})

	t.Run("load with priority no sources should error", func(t *testing.T) {
		loader := NewConfigLoader(nil, "test-namespace")
		cfg, sources, err := loader.LoadWithPriority(context.Background(), LoadOptions{
			// No sources specified
		})

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Nil(t, sources)
	})
}
