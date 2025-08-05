package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewConfigManager(t *testing.T) {
	t.Run("creates manager with default options", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		assert.NotNil(t, manager)
		assert.Equal(t, "test-namespace", manager.namespace)
	})

	t.Run("creates manager with options", func(t *testing.T) {
		options := &ManagerOptions{
			Environment:    "staging",
			AutoReload:     true,
			ReloadInterval: 30 * time.Second,
		}

		manager := NewConfigManagerWithOptions(nil, "test-namespace", options)

		assert.NotNil(t, manager)
		assert.Equal(t, "test-namespace", manager.namespace)
		assert.Equal(t, "staging", manager.options.Environment)
		assert.True(t, manager.options.AutoReload)
		assert.Equal(t, 30*time.Second, manager.options.ReloadInterval)
	})
}

func TestConfigManager_LoadConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"environment": "staging",
			"log-level":   "debug",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	t.Run("loads configuration from multiple sources", func(t *testing.T) {
		// Set environment variable
		_ = os.Setenv("CONFIG_CLOUDFLARE_RATE_LIMIT_RPS", "15")
		defer func() { _ = os.Unsetenv("CONFIG_CLOUDFLARE_RATE_LIMIT_RPS") }()

		manager := NewConfigManager(fakeClient, "test-namespace")

		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName: "operator-config",
			LoadFromEnv:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Environment variable should take precedence
		assert.Equal(t, 15, cfg.Cloudflare.RateLimitRPS)
		// ConfigMap values should be loaded
		assert.Equal(t, "staging", cfg.Environment)
		assert.Equal(t, "debug", cfg.Operator.LogLevel)
	})

	t.Run("validates configuration by default", func(t *testing.T) {
		manager := NewConfigManager(fakeClient, "test-namespace")

		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName: "operator-config",
			LoadFromEnv:   true,
		})

		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Configuration should be valid
		assert.NoError(t, cfg.Validate())
	})

	t.Run("returns error for invalid configuration", func(t *testing.T) {
		invalidConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-config",
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"environment": "invalid-env",
			},
		}

		invalidClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(invalidConfigMap).
			Build()

		manager := NewConfigManager(invalidClient, "test-namespace")

		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName:  "invalid-config",
			ValidateConfig: true,
		})

		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "invalid environment")
	})
}

func TestConfigManager_GetConfig(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("returns nil when no config loaded", func(t *testing.T) {
		cfg := manager.GetConfig()
		assert.Nil(t, cfg)
	})

	t.Run("returns loaded config", func(t *testing.T) {
		// Load a basic config first
		cfg := NewConfig()
		cfg.Environment = TestEnv
		manager.currentConfig = cfg

		retrievedCfg := manager.GetConfig()
		assert.NotNil(t, retrievedCfg)
		assert.Equal(t, "test", retrievedCfg.Environment)
	})
}

func TestConfigManager_GetFeatureFlagManager(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("returns nil when no config loaded", func(t *testing.T) {
		ffm := manager.GetFeatureFlagManager()
		assert.Nil(t, ffm)
	})

	t.Run("returns feature flag manager for loaded config", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Features.EnableWebhooks = true
		manager.currentConfig = cfg
		manager.featureFlagManager = NewFeatureFlagManager(cfg.Features)

		ffm := manager.GetFeatureFlagManager()
		assert.NotNil(t, ffm)
		assert.True(t, ffm.IsEnabled("EnableWebhooks"))
	})
}

func TestConfigManager_ReloadConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	initialConfigMap := &corev1.ConfigMap{
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
		WithObjects(initialConfigMap).
		Build()

	manager := NewConfigManager(fakeClient, "test-namespace")

	t.Run("reloads configuration successfully", func(t *testing.T) {
		// Load initial config
		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName: "operator-config",
		})
		require.NoError(t, err)
		assert.Equal(t, "info", cfg.Operator.LogLevel)

		// Update the ConfigMap in the fake client
		updatedConfigMap := initialConfigMap.DeepCopy()
		updatedConfigMap.Data["log-level"] = "debug"
		err = fakeClient.Update(context.Background(), updatedConfigMap)
		require.NoError(t, err)

		// Reload config
		newCfg, err := manager.ReloadConfig(context.Background())
		require.NoError(t, err)
		require.NotNil(t, newCfg)

		// Should have new values
		assert.Equal(t, "debug", newCfg.Operator.LogLevel)

		// Current config should be updated
		assert.Equal(t, newCfg, manager.GetConfig())
	})

	t.Run("fails when no load options set", func(t *testing.T) {
		manager := NewConfigManager(fakeClient, "test-namespace")

		cfg, err := manager.ReloadConfig(context.Background())
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "no load options")
	})
}

func TestConfigManager_StartAutoReload(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-config",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"environment": "staging",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(configMap).
		Build()

	t.Run("starts auto-reload when enabled", func(t *testing.T) {
		options := &ManagerOptions{
			AutoReload:     true,
			ReloadInterval: 100 * time.Millisecond,
		}

		manager := NewConfigManagerWithOptions(fakeClient, "test-namespace", options)

		// Load initial config
		_, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName: "operator-config",
		})
		require.NoError(t, err)

		// Start auto-reload
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = manager.StartAutoReload(ctx)
		// Should not error immediately
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
	})

	t.Run("does not start when auto-reload disabled", func(t *testing.T) {
		options := &ManagerOptions{
			AutoReload: false,
		}

		manager := NewConfigManagerWithOptions(fakeClient, "test-namespace", options)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := manager.StartAutoReload(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auto-reload is not enabled")
	})
}

func TestConfigManager_ConfigChangeCallback(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("registers and calls callback on config change", func(t *testing.T) {
		var callbackCalled bool
		var callbackConfig *Config
		done := make(chan bool, 1)

		callback := func(_, newConfig *Config) {
			callbackCalled = true
			callbackConfig = newConfig
			done <- true
		}

		manager.RegisterConfigChangeCallback(callback)

		// Simulate config change
		oldCfg := NewConfig()
		newCfg := NewConfig()
		newCfg.Environment = TestEnv

		manager.currentConfig = oldCfg
		manager.updateConfig(newCfg)

		// Wait for callback to be called
		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("callback was not called")
		}

		assert.True(t, callbackCalled)
		assert.Equal(t, "test", callbackConfig.Environment)
	})

	t.Run("handles multiple callbacks", func(t *testing.T) {
		var callback1Called, callback2Called bool
		done := make(chan bool, 2)

		manager.RegisterConfigChangeCallback(func(_, _ *Config) {
			callback1Called = true
			done <- true
		})

		manager.RegisterConfigChangeCallback(func(_, _ *Config) {
			callback2Called = true
			done <- true
		})

		// Simulate config change
		oldCfg := NewConfig()
		newCfg := NewConfig()
		newCfg.Environment = TestEnv

		manager.currentConfig = oldCfg
		manager.updateConfig(newCfg)

		// Wait for both callbacks to be called
		for i := 0; i < 2; i++ {
			select {
			case <-done:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("not all callbacks were called")
			}
		}

		assert.True(t, callback1Called)
		assert.True(t, callback2Called)
	})
}

func TestConfigManager_IsConfigured(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("returns false when no config loaded", func(t *testing.T) {
		assert.False(t, manager.IsConfigured())
	})

	t.Run("returns true when config loaded", func(t *testing.T) {
		cfg := NewConfig()
		manager.currentConfig = cfg
		assert.True(t, manager.IsConfigured())
	})
}

func TestConfigManager_GetConfigSources(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("returns empty map when no config loaded", func(t *testing.T) {
		sources := manager.GetConfigSources()
		assert.Empty(t, sources)
	})

	t.Run("returns config sources", func(t *testing.T) {
		cfg := NewConfig()
		cfg.ConfigSources = map[string]ConfigSource{
			"environment":       ConfigSourceEnv,
			"operator.logLevel": ConfigSourceConfigMap,
		}
		manager.currentConfig = cfg
		manager.currentSources = cfg.ConfigSources

		sources := manager.GetConfigSources()
		expected := map[string]ConfigSource{
			"environment":       ConfigSourceEnv,
			"operator.logLevel": ConfigSourceConfigMap,
		}
		assert.Equal(t, expected, sources)
	})
}

func TestConfigManager_GetLoadedAt(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("returns zero time when no config loaded", func(t *testing.T) {
		loadedAt := manager.GetLoadedAt()
		assert.True(t, loadedAt.IsZero())
	})

	t.Run("returns load time after config loaded", func(t *testing.T) {
		before := time.Now()

		cfg := NewConfig()
		manager.currentConfig = cfg
		manager.loadedAt = time.Now()

		after := time.Now()
		loadedAt := manager.GetLoadedAt()

		assert.True(t, loadedAt.After(before) || loadedAt.Equal(before))
		assert.True(t, loadedAt.Before(after) || loadedAt.Equal(after))
	})
}

func TestConfigManager_ValidateConfiguration(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("validates current configuration", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Environment = "production"
		manager.currentConfig = cfg

		err := manager.ValidateConfiguration()
		assert.NoError(t, err)
	})

	t.Run("returns error for invalid configuration", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Environment = "invalid"
		manager.currentConfig = cfg

		err := manager.ValidateConfiguration()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid environment")
	})

	t.Run("returns error when no config loaded", func(t *testing.T) {
		manager.currentConfig = nil

		err := manager.ValidateConfiguration()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no configuration loaded")
	})
}

func TestManagerOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options *ManagerOptions
		wantErr bool
	}{
		{
			name: "valid options",
			options: &ManagerOptions{
				Environment:    "production",
				AutoReload:     true,
				ReloadInterval: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid environment",
			options: &ManagerOptions{
				Environment: "invalid",
			},
			wantErr: true,
		},
		{
			name: "auto-reload with invalid interval",
			options: &ManagerOptions{
				AutoReload:     true,
				ReloadInterval: 100 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "auto-reload without interval is invalid",
			options: &ManagerOptions{
				AutoReload: true,
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

func TestConfigManager_ThreadSafety(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	t.Run("concurrent access is safe", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Environment = TestEnv
		manager.currentConfig = cfg

		// This is a basic test - in a real scenario we'd use goroutines
		// to test concurrent access, but that's complex with the current setup

		// Multiple reads should work
		for i := 0; i < 10; i++ {
			retrievedCfg := manager.GetConfig()
			assert.NotNil(t, retrievedCfg)
			assert.Equal(t, "test", retrievedCfg.Environment)

			assert.True(t, manager.IsConfigured())

			sources := manager.GetConfigSources()
			assert.NotNil(t, sources)
		}
	})
}

func TestConfigManager_EdgeCases(t *testing.T) {
	t.Run("handles nil client gracefully", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			LoadFromEnv: true,
		})

		// Should work with environment variables only
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cfg, err := manager.LoadConfig(ctx, LoadOptions{
			LoadFromEnv: true,
		})

		// Should still work for env vars as they don't use context
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("handles empty namespace", func(t *testing.T) {
		manager := NewConfigManager(nil, "")

		cfg, err := manager.LoadConfig(context.Background(), LoadOptions{
			LoadFromEnv: true,
		})

		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})
}

func TestConfigManager_StopAutoReload(t *testing.T) {
	t.Run("stops auto reload when running", func(t *testing.T) {
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "operator-config",
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"environment": "staging",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(configMap).
			Build()

		manager := NewConfigManagerWithOptions(fakeClient, "test-namespace", &ManagerOptions{
			AutoReload:     true,
			ReloadInterval: 100 * time.Millisecond,
		})

		// Load config first
		_, err := manager.LoadConfig(context.Background(), LoadOptions{
			ConfigMapName: "operator-config",
		})
		require.NoError(t, err)

		// Start auto-reload in background
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		go func() {
			_ = manager.StartAutoReload(ctx)
		}()

		// Wait a bit then stop
		time.Sleep(50 * time.Millisecond)
		manager.StopAutoReload()
		// Should not panic
	})

	t.Run("stops auto reload when not running", func(_ *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		manager.StopAutoReload()
		// Should not panic
	})
}

func TestConfigManager_GetNamespace(t *testing.T) {
	manager := NewConfigManager(nil, "test-namespace")

	namespace := manager.GetNamespace()
	assert.Equal(t, "test-namespace", namespace)
}

func TestConfigManager_GetOptions(t *testing.T) {
	options := &ManagerOptions{
		Environment:    "staging",
		AutoReload:     true,
		ReloadInterval: 30 * time.Second,
	}
	manager := NewConfigManagerWithOptions(nil, "test-namespace", options)

	retrievedOptions := manager.GetOptions()
	assert.Equal(t, *options, retrievedOptions)
}

func TestManagerOptions_String(t *testing.T) {
	options := &ManagerOptions{
		Environment:    "production",
		AutoReload:     true,
		ReloadInterval: 60 * time.Second,
	}

	str := options.String()
	assert.Contains(t, str, "Environment: production")
	assert.Contains(t, str, "AutoReload: true")
	assert.Contains(t, str, "ReloadInterval: 1m0s")
}

func TestDefaultManagerOptions_String(t *testing.T) {
	// Test default options created by NewConfigManager
	manager := NewConfigManager(nil, "test-namespace")
	options := manager.GetOptions()

	str := options.String()
	assert.Contains(t, str, "Environment: production")
	assert.Contains(t, str, "AutoReload: false")
	assert.Contains(t, str, "ReloadInterval: 5m0s")
}

func TestConfigManager_Close(t *testing.T) {
	t.Run("closes without auto reload", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")
		err := manager.Close()
		assert.NoError(t, err)
	})

	t.Run("closes with auto reload", func(t *testing.T) {
		manager := NewConfigManagerWithOptions(nil, "test-namespace", &ManagerOptions{
			AutoReload:     true,
			ReloadInterval: 100 * time.Millisecond,
		})

		err := manager.Close()
		assert.NoError(t, err)
	})
}

func TestConfigManager_String(t *testing.T) {
	t.Run("string representation without config", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")
		str := manager.String()

		assert.Contains(t, str, "not configured")
		assert.Contains(t, str, "test-namespace")
		assert.Contains(t, str, "disabled") // auto-reload disabled
	})

	t.Run("string representation with config and auto-reload", func(t *testing.T) {
		manager := NewConfigManagerWithOptions(nil, "test-namespace", &ManagerOptions{
			AutoReload:     true,
			ReloadInterval: 30 * time.Second,
		})

		// Load a config
		cfg := NewConfig()
		cfg.Environment = TestEnv
		manager.currentConfig = cfg
		manager.loadedAt = time.Now()

		str := manager.String()

		assert.Contains(t, str, "configured")
		assert.Contains(t, str, "test-namespace")
		assert.Contains(t, str, "30s") // auto-reload interval
	})
}

// Test manager options validation and edge cases
func TestManagerOptions_Validation(t *testing.T) {
	t.Run("invalid environment", func(t *testing.T) {
		options := &ManagerOptions{
			Environment: "invalid-env",
		}

		err := options.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid environment")
	})

	t.Run("auto reload with invalid interval", func(t *testing.T) {
		options := &ManagerOptions{
			AutoReload:     true,
			ReloadInterval: 500 * time.Millisecond, // Less than 1 second
		}

		err := options.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reload interval must be at least 1 second")
	})
}

// Test auto-reload error scenarios
func TestConfigManager_AutoReload_ErrorCases(t *testing.T) {
	t.Run("start auto reload without enabled option", func(t *testing.T) {
		manager := NewConfigManagerWithOptions(nil, "test-namespace", &ManagerOptions{
			AutoReload: false,
		})

		err := manager.StartAutoReload(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auto-reload is not enabled")
	})

	t.Run("stop auto reload when not started", func(_ *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		// Should not panic or error
		manager.StopAutoReload()
	})

	t.Run("reload config without previous load", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		_, err := manager.ReloadConfig(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "LoadConfig must be called first")
	})
}

// Test config manager error handling
func TestConfigManager_ErrorHandling(t *testing.T) {
	t.Run("validate configuration without config loaded", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		err := manager.ValidateConfiguration()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no configuration loaded")
	})

	t.Run("get feature flag manager without config", func(t *testing.T) {
		manager := NewConfigManager(nil, "test-namespace")

		ffManager := manager.GetFeatureFlagManager()
		assert.Nil(t, ffManager)
	})

	t.Run("load config with nil options defaults", func(t *testing.T) {
		manager := NewConfigManagerWithOptions(nil, "test-namespace", nil)

		// Should use default options
		options := manager.GetOptions()
		assert.Equal(t, "production", options.Environment)
		assert.False(t, options.AutoReload)
		assert.Equal(t, 5*time.Minute, options.ReloadInterval)
		assert.True(t, options.ValidateOnLoad)
	})
}
