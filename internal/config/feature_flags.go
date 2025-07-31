package config

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// FeatureFlagManager provides thread-safe access to feature flags
type FeatureFlagManager struct {
	flags *FeatureFlags
	mutex sync.RWMutex
}

// NewFeatureFlagManager creates a new feature flag manager
func NewFeatureFlagManager(flags *FeatureFlags) *FeatureFlagManager {
	return &FeatureFlagManager{
		flags: flags,
	}
}

// IsEnabled checks if a specific feature flag is enabled
func (m *FeatureFlagManager) IsEnabled(flagName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.flags == nil {
		return false
	}

	// Check standard flags first
	switch flagName {
	case "EnableWebhooks":
		return m.flags.EnableWebhooks
	case "EnableMetrics":
		return m.flags.EnableMetrics
	case "EnableTracing":
		return m.flags.EnableTracing
	case "ExperimentalFeatures":
		return m.flags.ExperimentalFeatures
	}

	// Check custom flags
	if m.flags.CustomFlags != nil {
		if value, exists := m.flags.CustomFlags[flagName]; exists {
			return value
		}
	}

	return false
}

// SetFlag sets a custom feature flag
func (m *FeatureFlagManager) SetFlag(flagName string, enabled bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.flags == nil {
		return // Cannot set flags on nil FeatureFlags
	}

	// Initialize custom flags if nil
	if m.flags.CustomFlags == nil {
		m.flags.CustomFlags = make(map[string]bool)
	}

	m.flags.CustomFlags[flagName] = enabled
}

// GetAllFlags returns a map of all flags (standard and custom) with their values
func (m *FeatureFlagManager) GetAllFlags() map[string]bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]bool)

	// Add standard flags
	if m.flags != nil {
		result["EnableWebhooks"] = m.flags.EnableWebhooks
		result["EnableMetrics"] = m.flags.EnableMetrics
		result["EnableTracing"] = m.flags.EnableTracing
		result["ExperimentalFeatures"] = m.flags.ExperimentalFeatures

		// Add custom flags
		if m.flags.CustomFlags != nil {
			for k, v := range m.flags.CustomFlags {
				result[k] = v
			}
		}
	} else {
		// Default values when flags is nil
		result["EnableWebhooks"] = false
		result["EnableMetrics"] = false
		result["EnableTracing"] = false
		result["ExperimentalFeatures"] = false
	}

	return result
}

// GetEnabledFlags returns a slice of flag names that are currently enabled
func (m *FeatureFlagManager) GetEnabledFlags() []string {
	allFlags := m.GetAllFlags()
	var enabled []string

	for flagName, isEnabled := range allFlags {
		if isEnabled {
			enabled = append(enabled, flagName)
		}
	}

	// Sort for consistent ordering
	sort.Strings(enabled)
	return enabled
}

// IsAnyEnabled checks if any of the specified flags are enabled
func (m *FeatureFlagManager) IsAnyEnabled(flagNames ...string) bool {
	for _, flagName := range flagNames {
		if m.IsEnabled(flagName) {
			return true
		}
	}
	return false
}

// IsAllEnabled checks if all of the specified flags are enabled
func (m *FeatureFlagManager) IsAllEnabled(flagNames ...string) bool {
	if len(flagNames) == 0 {
		return true // vacuously true
	}

	for _, flagName := range flagNames {
		if !m.IsEnabled(flagName) {
			return false
		}
	}
	return true
}

// WithEnvironmentOverrides returns a new FeatureFlagManager with environment-specific overrides applied
func (m *FeatureFlagManager) WithEnvironmentOverrides(environment string) *FeatureFlagManager {
	m.mutex.RLock()
	clonedFlags := m.cloneFlags()
	m.mutex.RUnlock()

	// Apply environment-specific overrides
	switch environment {
	case ProductionEnv:
		// Production should be conservative
		clonedFlags.EnableTracing = false        // Disable tracing in production for performance
		clonedFlags.ExperimentalFeatures = false // Never allow experimental features in production

	case StagingEnv:
		// Staging allows more features for testing
		clonedFlags.EnableTracing = true         // Enable tracing in staging for debugging
		clonedFlags.ExperimentalFeatures = false // Still disable experimental features for safety

	case DevelopmentEnv:
		// Development environment allows all features
		clonedFlags.EnableMetrics = true        // Always enable metrics in development
		clonedFlags.EnableTracing = true        // Enable tracing for debugging
		clonedFlags.ExperimentalFeatures = true // Allow experimental features in development

	case TestEnv:
		// Test environment typically mirrors production but may need specific flags
		clonedFlags.EnableTracing = false
		clonedFlags.ExperimentalFeatures = false
	}

	return NewFeatureFlagManager(clonedFlags)
}

// Clone creates a deep copy of the FeatureFlagManager
func (m *FeatureFlagManager) Clone() *FeatureFlagManager {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	clonedFlags := m.cloneFlags()
	return NewFeatureFlagManager(clonedFlags)
}

// cloneFlags creates a deep copy of FeatureFlags (internal method, caller must hold lock)
func (m *FeatureFlagManager) cloneFlags() *FeatureFlags {
	if m.flags == nil {
		return &FeatureFlags{
			CustomFlags: make(map[string]bool),
		}
	}

	cloned := &FeatureFlags{
		EnableWebhooks:       m.flags.EnableWebhooks,
		EnableMetrics:        m.flags.EnableMetrics,
		EnableTracing:        m.flags.EnableTracing,
		ExperimentalFeatures: m.flags.ExperimentalFeatures,
		CustomFlags:          make(map[string]bool),
	}

	// Deep copy custom flags
	if m.flags.CustomFlags != nil {
		for k, v := range m.flags.CustomFlags {
			cloned.CustomFlags[k] = v
		}
	}

	return cloned
}

// String returns a string representation of all feature flags
func (m *FeatureFlagManager) String() string {
	allFlags := m.GetAllFlags()

	var parts []string

	// Add standard flags in consistent order
	standardFlags := []string{"EnableWebhooks", "EnableMetrics", "EnableTracing", "ExperimentalFeatures"}
	for _, flag := range standardFlags {
		if value, exists := allFlags[flag]; exists {
			parts = append(parts, fmt.Sprintf("%s=%t", flag, value))
			delete(allFlags, flag) // Remove from map so we don't include it again
		}
	}

	// Add custom flags in sorted order
	var customFlags []string
	for flag := range allFlags {
		customFlags = append(customFlags, flag)
	}
	sort.Strings(customFlags)

	for _, flag := range customFlags {
		parts = append(parts, fmt.Sprintf("%s=%t", flag, allFlags[flag]))
	}

	return "FeatureFlags{" + strings.Join(parts, ", ") + "}"
}

// GetStandardFlags returns the values of all standard (non-custom) feature flags
func (m *FeatureFlagManager) GetStandardFlags() map[string]bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]bool)

	if m.flags != nil {
		result["EnableWebhooks"] = m.flags.EnableWebhooks
		result["EnableMetrics"] = m.flags.EnableMetrics
		result["EnableTracing"] = m.flags.EnableTracing
		result["ExperimentalFeatures"] = m.flags.ExperimentalFeatures
	} else {
		result["EnableWebhooks"] = false
		result["EnableMetrics"] = false
		result["EnableTracing"] = false
		result["ExperimentalFeatures"] = false
	}

	return result
}

// GetCustomFlags returns a copy of all custom feature flags
func (m *FeatureFlagManager) GetCustomFlags() map[string]bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]bool)

	if m.flags != nil && m.flags.CustomFlags != nil {
		for k, v := range m.flags.CustomFlags {
			result[k] = v
		}
	}

	return result
}

// HasFlag checks if a flag exists (regardless of its value)
func (m *FeatureFlagManager) HasFlag(flagName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.flags == nil {
		return false
	}

	// Check if it's a standard flag
	standardFlags := []string{"EnableWebhooks", "EnableMetrics", "EnableTracing", "ExperimentalFeatures"}
	for _, flag := range standardFlags {
		if flag == flagName {
			return true
		}
	}

	// Check if it's a custom flag
	if m.flags.CustomFlags != nil {
		_, exists := m.flags.CustomFlags[flagName]
		return exists
	}

	return false
}

// SetStandardFlag sets a standard feature flag using reflection
func (m *FeatureFlagManager) SetStandardFlag(flagName string, enabled bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.flags == nil {
		m.flags = &FeatureFlags{
			CustomFlags: make(map[string]bool),
		}
	}

	// Use reflection to set the standard flag
	flagsValue := reflect.ValueOf(m.flags).Elem()
	fieldValue := flagsValue.FieldByName(flagName)

	if !fieldValue.IsValid() {
		return fmt.Errorf("standard flag %s does not exist", flagName)
	}

	if !fieldValue.CanSet() {
		return fmt.Errorf("cannot set standard flag %s", flagName)
	}

	if fieldValue.Kind() != reflect.Bool {
		return fmt.Errorf("standard flag %s is not a boolean", flagName)
	}

	fieldValue.SetBool(enabled)
	return nil
}

// RemoveCustomFlag removes a custom feature flag
func (m *FeatureFlagManager) RemoveCustomFlag(flagName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.flags != nil && m.flags.CustomFlags != nil {
		delete(m.flags.CustomFlags, flagName)
	}
}

// ClearCustomFlags removes all custom feature flags
func (m *FeatureFlagManager) ClearCustomFlags() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.flags != nil {
		m.flags.CustomFlags = make(map[string]bool)
	}
}
