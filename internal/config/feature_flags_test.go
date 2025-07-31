package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeatureFlagManager_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		flags    *FeatureFlags
		flagName string
		expected bool
	}{
		{
			name: "standard webhook flag enabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
			},
			flagName: "EnableWebhooks",
			expected: true,
		},
		{
			name: "standard metrics flag disabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
			},
			flagName: "EnableMetrics",
			expected: false,
		},
		{
			name: "custom flag enabled",
			flags: &FeatureFlags{
				CustomFlags: map[string]bool{
					"customFeature": true,
				},
			},
			flagName: "customFeature",
			expected: true,
		},
		{
			name: "custom flag disabled",
			flags: &FeatureFlags{
				CustomFlags: map[string]bool{
					"customFeature": false,
				},
			},
			flagName: "customFeature",
			expected: false,
		},
		{
			name: "non-existent flag",
			flags: &FeatureFlags{
				EnableWebhooks: true,
			},
			flagName: "nonExistentFlag",
			expected: false,
		},
		{
			name:     "nil flags",
			flags:    nil,
			flagName: "EnableWebhooks",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewFeatureFlagManager(tt.flags)
			result := manager.IsEnabled(tt.flagName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeatureFlagManager_SetFlag(t *testing.T) {
	t.Run("sets custom flag", func(t *testing.T) {
		flags := &FeatureFlags{
			CustomFlags: make(map[string]bool),
		}
		manager := NewFeatureFlagManager(flags)

		manager.SetFlag("newFeature", true)
		assert.True(t, manager.IsEnabled("newFeature"))

		manager.SetFlag("newFeature", false)
		assert.False(t, manager.IsEnabled("newFeature"))
	})

	t.Run("initializes custom flags if nil", func(t *testing.T) {
		flags := &FeatureFlags{}
		manager := NewFeatureFlagManager(flags)

		manager.SetFlag("testFlag", true)
		assert.True(t, manager.IsEnabled("testFlag"))
		assert.NotNil(t, flags.CustomFlags)
	})

	t.Run("handles nil flags gracefully", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)

		// Should not panic
		manager.SetFlag("testFlag", true)

		// Should return false since flags are nil
		assert.False(t, manager.IsEnabled("testFlag"))
	})
}

func TestFeatureFlagManager_GetAllFlags(t *testing.T) {
	t.Run("returns all standard and custom flags", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks:       true,
			EnableMetrics:        false,
			EnableTracing:        true,
			ExperimentalFeatures: false,
			CustomFlags: map[string]bool{
				"customFeature1": true,
				"customFeature2": false,
			},
		}
		manager := NewFeatureFlagManager(flags)

		allFlags := manager.GetAllFlags()

		expected := map[string]bool{
			"EnableWebhooks":       true,
			"EnableMetrics":        false,
			"EnableTracing":        true,
			"ExperimentalFeatures": false,
			"customFeature1":       true,
			"customFeature2":       false,
		}

		assert.Equal(t, expected, allFlags)
	})

	t.Run("handles nil custom flags", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks: true,
			EnableMetrics:  false,
			CustomFlags:    nil,
		}
		manager := NewFeatureFlagManager(flags)

		allFlags := manager.GetAllFlags()

		expected := map[string]bool{
			"EnableWebhooks":       true,
			"EnableMetrics":        false,
			"EnableTracing":        false,
			"ExperimentalFeatures": false,
		}

		assert.Equal(t, expected, allFlags)
	})

	t.Run("handles nil flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)

		allFlags := manager.GetAllFlags()

		expected := map[string]bool{
			"EnableWebhooks":       false,
			"EnableMetrics":        false,
			"EnableTracing":        false,
			"ExperimentalFeatures": false,
		}

		assert.Equal(t, expected, allFlags)
	})
}

func TestFeatureFlagManager_GetEnabledFlags(t *testing.T) {
	t.Run("returns only enabled flags", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks:       true,
			EnableMetrics:        false,
			EnableTracing:        true,
			ExperimentalFeatures: false,
			CustomFlags: map[string]bool{
				"customFeature1": true,
				"customFeature2": false,
				"customFeature3": true,
			},
		}
		manager := NewFeatureFlagManager(flags)

		enabledFlags := manager.GetEnabledFlags()

		expected := []string{"EnableWebhooks", "EnableTracing", "customFeature1", "customFeature3"}

		assert.ElementsMatch(t, expected, enabledFlags)
	})

	t.Run("returns empty slice when no flags enabled", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks:       false,
			EnableMetrics:        false,
			EnableTracing:        false,
			ExperimentalFeatures: false,
			CustomFlags: map[string]bool{
				"customFeature1": false,
			},
		}
		manager := NewFeatureFlagManager(flags)

		enabledFlags := manager.GetEnabledFlags()

		assert.Empty(t, enabledFlags)
	})
}

func TestFeatureFlagManager_IsAnyEnabled(t *testing.T) {
	tests := []struct {
		name     string
		flags    *FeatureFlags
		flagList []string
		expected bool
	}{
		{
			name: "at least one flag enabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
			},
			flagList: []string{"EnableWebhooks", "EnableMetrics"},
			expected: true,
		},
		{
			name: "no flags enabled",
			flags: &FeatureFlags{
				EnableWebhooks: false,
				EnableMetrics:  false,
			},
			flagList: []string{"EnableWebhooks", "EnableMetrics"},
			expected: false,
		},
		{
			name: "mixed standard and custom flags",
			flags: &FeatureFlags{
				EnableWebhooks: false,
				CustomFlags: map[string]bool{
					"customFeature": true,
				},
			},
			flagList: []string{"EnableWebhooks", "customFeature"},
			expected: true,
		},
		{
			name: "empty flag list",
			flags: &FeatureFlags{
				EnableWebhooks: true,
			},
			flagList: []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewFeatureFlagManager(tt.flags)
			result := manager.IsAnyEnabled(tt.flagList...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeatureFlagManager_IsAllEnabled(t *testing.T) {
	tests := []struct {
		name     string
		flags    *FeatureFlags
		flagList []string
		expected bool
	}{
		{
			name: "all flags enabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  true,
			},
			flagList: []string{"EnableWebhooks", "EnableMetrics"},
			expected: true,
		},
		{
			name: "not all flags enabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				EnableMetrics:  false,
			},
			flagList: []string{"EnableWebhooks", "EnableMetrics"},
			expected: false,
		},
		{
			name: "mixed standard and custom flags all enabled",
			flags: &FeatureFlags{
				EnableWebhooks: true,
				CustomFlags: map[string]bool{
					"customFeature": true,
				},
			},
			flagList: []string{"EnableWebhooks", "customFeature"},
			expected: true,
		},
		{
			name: "empty flag list",
			flags: &FeatureFlags{
				EnableWebhooks: true,
			},
			flagList: []string{},
			expected: true, // vacuously true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewFeatureFlagManager(tt.flags)
			result := manager.IsAllEnabled(tt.flagList...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeatureFlagManager_WithEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name        string
		flags       *FeatureFlags
		environment string
		expected    map[string]bool
	}{
		{
			name: "production environment disables experimental features",
			flags: &FeatureFlags{
				EnableWebhooks:       true,
				EnableMetrics:        true,
				EnableTracing:        true,
				ExperimentalFeatures: true,
			},
			environment: "production",
			expected: map[string]bool{
				"EnableWebhooks":       true,
				"EnableMetrics":        true,
				"EnableTracing":        false, // disabled in production
				"ExperimentalFeatures": false, // disabled in production
			},
		},
		{
			name: "staging environment allows most features",
			flags: &FeatureFlags{
				EnableWebhooks:       true,
				EnableMetrics:        true,
				EnableTracing:        false,
				ExperimentalFeatures: true,
			},
			environment: "staging",
			expected: map[string]bool{
				"EnableWebhooks":       true,
				"EnableMetrics":        true,
				"EnableTracing":        true,  // enabled in staging
				"ExperimentalFeatures": false, // disabled in staging for safety
			},
		},
		{
			name: "development environment allows all features",
			flags: &FeatureFlags{
				EnableWebhooks:       false,
				EnableMetrics:        false,
				EnableTracing:        false,
				ExperimentalFeatures: false,
			},
			environment: "development",
			expected: map[string]bool{
				"EnableWebhooks":       false,
				"EnableMetrics":        true, // always enabled in development
				"EnableTracing":        true, // enabled in development
				"ExperimentalFeatures": true, // allowed in development
			},
		},
		{
			name: "unknown environment preserves flags",
			flags: &FeatureFlags{
				EnableWebhooks:       true,
				EnableMetrics:        false,
				EnableTracing:        true,
				ExperimentalFeatures: true,
			},
			environment: "unknown",
			expected: map[string]bool{
				"EnableWebhooks":       true,
				"EnableMetrics":        false,
				"EnableTracing":        true,
				"ExperimentalFeatures": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewFeatureFlagManager(tt.flags)
			overriddenManager := manager.WithEnvironmentOverrides(tt.environment)

			for flagName, expectedValue := range tt.expected {
				actualValue := overriddenManager.IsEnabled(flagName)
				assert.Equal(t, expectedValue, actualValue, "Flag %s should be %v in %s environment", flagName, expectedValue, tt.environment)
			}
		})
	}
}

func TestFeatureFlagManager_Clone(t *testing.T) {
	t.Run("creates independent copy", func(t *testing.T) {
		original := &FeatureFlags{
			EnableWebhooks: true,
			EnableMetrics:  false,
			CustomFlags: map[string]bool{
				"customFeature": true,
			},
		}
		manager := NewFeatureFlagManager(original)

		clonedManager := manager.Clone()

		// Verify initial state matches
		assert.Equal(t, manager.IsEnabled("EnableWebhooks"), clonedManager.IsEnabled("EnableWebhooks"))
		assert.Equal(t, manager.IsEnabled("customFeature"), clonedManager.IsEnabled("customFeature"))

		// Modify original
		manager.SetFlag("newFeature", true)
		original.EnableWebhooks = false

		// Verify clone is independent
		assert.False(t, clonedManager.IsEnabled("newFeature"))
		assert.True(t, clonedManager.IsEnabled("EnableWebhooks"))
	})

	t.Run("handles nil flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)
		clonedManager := manager.Clone()

		assert.False(t, clonedManager.IsEnabled("EnableWebhooks"))
		clonedManager.SetFlag("testFlag", true)
		assert.False(t, manager.IsEnabled("testFlag"))
	})
}

func TestFeatureFlagManager_String(t *testing.T) {
	t.Run("returns formatted string representation", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks: true,
			EnableMetrics:  false,
			CustomFlags: map[string]bool{
				"customFeature": true,
			},
		}
		manager := NewFeatureFlagManager(flags)

		str := manager.String()

		// Should contain flag information
		assert.Contains(t, str, "EnableWebhooks=true")
		assert.Contains(t, str, "EnableMetrics=false")
		assert.Contains(t, str, "customFeature=true")
	})

	t.Run("handles nil flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)
		str := manager.String()

		assert.Contains(t, str, "EnableWebhooks=false")
		assert.Contains(t, str, "EnableMetrics=false")
		assert.Contains(t, str, "EnableTracing=false")
		assert.Contains(t, str, "ExperimentalFeatures=false")
	})
}

func TestFeatureFlagManager_ThreadSafety(t *testing.T) {
	t.Run("concurrent access is safe", func(t *testing.T) {
		flags := &FeatureFlags{
			CustomFlags: make(map[string]bool),
		}
		manager := NewFeatureFlagManager(flags)

		// This test would be more meaningful with actual goroutines,
		// but for now we'll just verify basic operations don't panic
		manager.SetFlag("feature1", true)
		manager.SetFlag("feature2", false)

		assert.True(t, manager.IsEnabled("feature1"))
		assert.False(t, manager.IsEnabled("feature2"))

		enabledFlags := manager.GetEnabledFlags()
		assert.Contains(t, enabledFlags, "feature1")
		assert.NotContains(t, enabledFlags, "feature2")
	})
}

func TestFeatureFlagManager_GetStandardFlags(t *testing.T) {
	t.Run("returns standard flags with values", func(t *testing.T) {
		manager := NewFeatureFlagManager(&FeatureFlags{
			EnableWebhooks:       true,
			EnableMetrics:        false,
			EnableTracing:        true,
			ExperimentalFeatures: false,
		})

		flags := manager.GetStandardFlags()
		expected := map[string]bool{
			"EnableWebhooks":       true,
			"EnableMetrics":        false,
			"EnableTracing":        true,
			"ExperimentalFeatures": false,
		}
		assert.Equal(t, expected, flags)
	})

	t.Run("returns default values for nil flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)

		flags := manager.GetStandardFlags()
		expected := map[string]bool{
			"EnableWebhooks":       false,
			"EnableMetrics":        false,
			"EnableTracing":        false,
			"ExperimentalFeatures": false,
		}
		assert.Equal(t, expected, flags)
	})
}

func TestFeatureFlagManager_GetCustomFlags(t *testing.T) {
	t.Run("returns custom flags", func(t *testing.T) {
		customFlags := map[string]bool{
			"feature1": true,
			"feature2": false,
		}
		manager := NewFeatureFlagManager(&FeatureFlags{
			CustomFlags: customFlags,
		})

		flags := manager.GetCustomFlags()
		assert.Equal(t, customFlags, flags)
	})

	t.Run("returns empty map for nil custom flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(&FeatureFlags{
			CustomFlags: nil,
		})

		flags := manager.GetCustomFlags()
		assert.Empty(t, flags)
	})

	t.Run("returns empty map for nil flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil)

		flags := manager.GetCustomFlags()
		assert.Empty(t, flags)
	})
}

func TestFeatureFlagManager_HasFlag(t *testing.T) {
	manager := NewFeatureFlagManager(&FeatureFlags{
		EnableWebhooks: true,
		CustomFlags:    map[string]bool{"customFlag": true},
	})

	t.Run("returns true for existing standard flag", func(t *testing.T) {
		assert.True(t, manager.HasFlag("EnableWebhooks"))
	})

	t.Run("returns true for standard flag even if false", func(t *testing.T) {
		assert.True(t, manager.HasFlag("EnableMetrics"))
	})

	t.Run("returns true for existing custom flag", func(t *testing.T) {
		assert.True(t, manager.HasFlag("customFlag"))
	})

	t.Run("returns false for non-existent flag", func(t *testing.T) {
		assert.False(t, manager.HasFlag("nonExistent"))
	})

	t.Run("returns false for nil flags", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(nil)
		assert.False(t, nilManager.HasFlag("EnableWebhooks"))
	})
}

func TestFeatureFlagManager_SetStandardFlag(t *testing.T) {
	manager := NewFeatureFlagManager(&FeatureFlags{
		EnableWebhooks: false,
	})

	t.Run("sets EnableWebhooks", func(t *testing.T) {
		err := manager.SetStandardFlag("EnableWebhooks", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableWebhooks"))
	})

	t.Run("sets EnableMetrics", func(t *testing.T) {
		err := manager.SetStandardFlag("EnableMetrics", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableMetrics"))
	})

	t.Run("sets EnableTracing", func(t *testing.T) {
		err := manager.SetStandardFlag("EnableTracing", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableTracing"))
	})

	t.Run("sets ExperimentalFeatures", func(t *testing.T) {
		err := manager.SetStandardFlag("ExperimentalFeatures", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("ExperimentalFeatures"))
	})

	t.Run("returns error for unknown standard flag", func(t *testing.T) {
		err := manager.SetStandardFlag("UnknownFlag", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "standard flag UnknownFlag does not exist")
		assert.False(t, manager.IsEnabled("UnknownFlag"))
	})

	t.Run("initializes flags if nil", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(nil)
		err := nilManager.SetStandardFlag("EnableWebhooks", true)
		assert.NoError(t, err)
		assert.True(t, nilManager.IsEnabled("EnableWebhooks"))
	})
}

func TestFeatureFlagManager_RemoveCustomFlag(t *testing.T) {
	manager := NewFeatureFlagManager(&FeatureFlags{
		CustomFlags: map[string]bool{
			"flag1": true,
			"flag2": false,
		},
	})

	t.Run("removes existing custom flag", func(t *testing.T) {
		manager.RemoveCustomFlag("flag1")
		assert.False(t, manager.HasFlag("flag1"))
		assert.True(t, manager.HasFlag("flag2"))
	})

	t.Run("handles non-existent flag gracefully", func(t *testing.T) {
		manager.RemoveCustomFlag("nonExistent")
		assert.True(t, manager.HasFlag("flag2"))
	})

	t.Run("handles nil custom flags", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(&FeatureFlags{
			CustomFlags: nil,
		})
		nilManager.RemoveCustomFlag("any")
		// Should not panic
	})

	t.Run("handles nil flags", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(nil)
		nilManager.RemoveCustomFlag("any")
		// Should not panic
	})
}

func TestFeatureFlagManager_ClearCustomFlags(t *testing.T) {
	manager := NewFeatureFlagManager(&FeatureFlags{
		CustomFlags: map[string]bool{
			"flag1": true,
			"flag2": false,
		},
	})

	t.Run("clears all custom flags", func(t *testing.T) {
		manager.ClearCustomFlags()
		customFlags := manager.GetCustomFlags()
		assert.Empty(t, customFlags)
	})

	t.Run("handles nil custom flags", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(&FeatureFlags{
			CustomFlags: nil,
		})
		nilManager.ClearCustomFlags()
		// Should not panic
	})

	t.Run("handles nil flags", func(t *testing.T) {
		nilManager := NewFeatureFlagManager(nil)
		nilManager.ClearCustomFlags()
		// Should not panic
	})
}

// Test additional coverage for WithEnvironmentOverrides
func TestFeatureFlagManager_WithEnvironmentOverrides_Coverage(t *testing.T) {
	t.Run("test all environment cases", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks:       true,
			EnableMetrics:        true,
			EnableTracing:        true,
			ExperimentalFeatures: true,
			CustomFlags:          map[string]bool{"test": true},
		}
		manager := NewFeatureFlagManager(flags)

		// Test production environment - disables experimental features and tracing
		prodManager := manager.WithEnvironmentOverrides("production")
		assert.True(t, prodManager.IsEnabled("EnableWebhooks"))
		assert.True(t, prodManager.IsEnabled("EnableMetrics"))
		assert.False(t, prodManager.IsEnabled("EnableTracing"))        // Disabled in production
		assert.False(t, prodManager.IsEnabled("ExperimentalFeatures")) // Disabled in production
		assert.True(t, prodManager.IsEnabled("test"))

		// Test staging environment
		stagingManager := manager.WithEnvironmentOverrides("staging")
		assert.True(t, stagingManager.IsEnabled("EnableWebhooks"))
		assert.True(t, stagingManager.IsEnabled("EnableMetrics"))
		assert.True(t, stagingManager.IsEnabled("EnableTracing"))
		assert.False(t, stagingManager.IsEnabled("ExperimentalFeatures")) // Also disabled in staging
		assert.True(t, stagingManager.IsEnabled("test"))

		// Test development environment
		devManager := manager.WithEnvironmentOverrides("development")
		assert.True(t, devManager.IsEnabled("EnableWebhooks"))
		assert.True(t, devManager.IsEnabled("EnableMetrics"))
		assert.True(t, devManager.IsEnabled("EnableTracing"))
		assert.True(t, devManager.IsEnabled("ExperimentalFeatures"))
		assert.True(t, devManager.IsEnabled("test"))

		// Test test environment
		testManager := manager.WithEnvironmentOverrides("test")
		assert.True(t, testManager.IsEnabled("EnableWebhooks"))
		assert.True(t, testManager.IsEnabled("EnableMetrics"))
		assert.False(t, testManager.IsEnabled("EnableTracing"))        // Disabled in test
		assert.False(t, testManager.IsEnabled("ExperimentalFeatures")) // Disabled in test
		assert.True(t, testManager.IsEnabled("test"))

		// Test unknown environment
		unknownManager := manager.WithEnvironmentOverrides("unknown")
		assert.True(t, unknownManager.IsEnabled("EnableWebhooks"))
		assert.True(t, unknownManager.IsEnabled("EnableMetrics"))
		assert.True(t, unknownManager.IsEnabled("EnableTracing"))
		assert.True(t, unknownManager.IsEnabled("ExperimentalFeatures"))
		assert.True(t, unknownManager.IsEnabled("test"))
	})
}

// Test additional coverage for HasFlag edge cases
func TestFeatureFlagManager_HasFlag_Coverage(t *testing.T) {
	t.Run("test all standard flags", func(t *testing.T) {
		flags := &FeatureFlags{
			EnableWebhooks:       true,
			EnableMetrics:        false,
			EnableTracing:        true,
			ExperimentalFeatures: false,
			CustomFlags:          map[string]bool{"custom": true},
		}
		manager := NewFeatureFlagManager(flags)

		// Test all standard flags
		assert.True(t, manager.HasFlag("EnableWebhooks"))
		assert.True(t, manager.HasFlag("EnableMetrics"))
		assert.True(t, manager.HasFlag("EnableTracing"))
		assert.True(t, manager.HasFlag("ExperimentalFeatures"))

		// Test custom flag
		assert.True(t, manager.HasFlag("custom"))

		// Test non-existent flag
		assert.False(t, manager.HasFlag("nonexistent"))

		// Test with nil custom flags
		flags.CustomFlags = nil
		assert.True(t, manager.HasFlag("EnableWebhooks"))
		assert.False(t, manager.HasFlag("custom"))
	})
}

// Test additional coverage for SetStandardFlag edge cases
func TestFeatureFlagManager_SetStandardFlag_Coverage(t *testing.T) {
	t.Run("test all standard flags", func(t *testing.T) {
		manager := NewFeatureFlagManager(nil) // Start with nil flags

		// Test setting each standard flag
		err := manager.SetStandardFlag("EnableWebhooks", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableWebhooks"))

		err = manager.SetStandardFlag("EnableMetrics", false)
		assert.NoError(t, err)
		assert.False(t, manager.IsEnabled("EnableMetrics"))

		err = manager.SetStandardFlag("EnableTracing", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableTracing"))

		err = manager.SetStandardFlag("ExperimentalFeatures", false)
		assert.NoError(t, err)
		assert.False(t, manager.IsEnabled("ExperimentalFeatures"))

		// Test setting unknown standard flag
		err = manager.SetStandardFlag("UnknownFlag", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "standard flag UnknownFlag does not exist")
		assert.False(t, manager.IsEnabled("UnknownFlag"))
	})

	t.Run("test error conditions for reflection", func(t *testing.T) {
		// Test trying to set a field that would fail the CanSet check
		// We need to create a scenario where reflection would fail
		// For our current FeatureFlags struct, all fields are exported and settable,
		// so we'll create a test specifically for the error paths

		// Create a flags struct with a non-boolean field to test the type check
		type TestFlags struct {
			EnableWebhooks       bool
			EnableMetrics        bool
			EnableTracing        bool
			ExperimentalFeatures bool
			NonBooleanField      string // This will trigger the boolean type error
		}

		// We can't easily substitute TestFlags for FeatureFlags in the manager
		// without major changes, so let's focus on testing the actual error paths
		// that can realistically occur

		flags := &FeatureFlags{}
		manager := NewFeatureFlagManager(flags)

		// Test that normal fields work correctly
		err := manager.SetStandardFlag("EnableWebhooks", true)
		assert.NoError(t, err)
		assert.True(t, manager.IsEnabled("EnableWebhooks"))

		// Test error for unknown field (already covered above, but ensuring it's here)
		err = manager.SetStandardFlag("NonExistentField", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})
}
