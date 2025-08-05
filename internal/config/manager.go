package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *Config) //nolint:revive,lll // ConfigChangeCallback is clear

// ManagerOptions contains options for the configuration manager
type ManagerOptions struct {
	// Environment specifies the default environment if not loaded from config
	Environment string

	// AutoReload enables automatic configuration reloading
	AutoReload bool

	// ReloadInterval specifies how often to check for configuration changes
	ReloadInterval time.Duration

	// ValidateOnLoad validates configuration after loading
	ValidateOnLoad bool
}

// ConfigManager manages configuration loading, reloading, and change notifications
type ConfigManager struct { //nolint:revive // ConfigManager is clear and consistent
	client    client.Client
	namespace string
	options   *ManagerOptions

	// Current state
	currentConfig  *Config
	currentSources map[string]ConfigSource
	loadedAt       time.Time
	loadOptions    *LoadOptions

	// Thread safety
	mutex sync.RWMutex

	// Change notifications
	callbacks []ConfigChangeCallback

	// Auto-reload control
	reloadCancel context.CancelFunc

	// Internal components
	loader             *ConfigLoader
	featureFlagManager *FeatureFlagManager
}

// NewConfigManager creates a new configuration manager with default options
func NewConfigManager(kubeClient client.Client, namespace string) *ConfigManager {
	return NewConfigManagerWithOptions(kubeClient, namespace, &ManagerOptions{
		Environment:    ProductionEnv,
		AutoReload:     false,
		ReloadInterval: 5 * time.Minute,
		ValidateOnLoad: true,
	})
}

// NewConfigManagerWithOptions creates a new configuration manager with custom options
func NewConfigManagerWithOptions(kubeClient client.Client, namespace string, options *ManagerOptions) *ConfigManager {
	if options == nil {
		options = &ManagerOptions{
			Environment:    ProductionEnv,
			AutoReload:     false,
			ReloadInterval: 5 * time.Minute,
			ValidateOnLoad: true,
		}
	}

	// Set defaults for missing values
	if options.ReloadInterval == 0 {
		options.ReloadInterval = 5 * time.Minute
	}

	return &ConfigManager{
		client:    kubeClient,
		namespace: namespace,
		options:   options,
		loader:    NewConfigLoader(kubeClient, namespace),
		callbacks: make([]ConfigChangeCallback, 0),
	}
}

// LoadConfig loads configuration from the specified sources
func (m *ConfigManager) LoadConfig(ctx context.Context, options LoadOptions) (*Config, error) {
	// Set default validation if not specified
	if !options.ValidateConfig {
		options.ValidateConfig = m.options.ValidateOnLoad
	}

	// Load configuration using the loader
	config, sources, err := m.loader.LoadWithPriority(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply environment-specific overrides if manager has a default environment
	if config.Environment == "" && m.options.Environment != "" {
		config.Environment = m.options.Environment
	}

	// Store the load options for future reloads
	m.mutex.Lock()
	oldConfig := m.currentConfig
	m.currentConfig = config
	m.currentSources = sources
	m.loadedAt = time.Now()
	m.loadOptions = &options
	m.featureFlagManager = NewFeatureFlagManager(config.Features)
	m.mutex.Unlock()

	// Notify callbacks of config change
	if oldConfig != nil {
		m.notifyCallbacks(oldConfig, config)
	}

	return config, nil
}

// ReloadConfig reloads the configuration using the same options as the last load
func (m *ConfigManager) ReloadConfig(ctx context.Context) (*Config, error) {
	m.mutex.RLock()
	loadOptions := m.loadOptions
	m.mutex.RUnlock()

	if loadOptions == nil {
		return nil, fmt.Errorf("no load options available for reload - LoadConfig must be called first")
	}

	return m.LoadConfig(ctx, *loadOptions)
}

// GetConfig returns the current configuration (thread-safe)
func (m *ConfigManager) GetConfig() *Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.currentConfig == nil {
		return nil
	}

	return m.currentConfig.Copy()
}

// GetFeatureFlagManager returns the feature flag manager for the current configuration
func (m *ConfigManager) GetFeatureFlagManager() *FeatureFlagManager {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.featureFlagManager == nil {
		return nil
	}

	return m.featureFlagManager.Clone()
}

// RegisterConfigChangeCallback registers a callback to be called when configuration changes
func (m *ConfigManager) RegisterConfigChangeCallback(callback ConfigChangeCallback) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.callbacks = append(m.callbacks, callback)
}

// StartAutoReload starts automatic configuration reloading (if enabled)
func (m *ConfigManager) StartAutoReload(ctx context.Context) error {
	if !m.options.AutoReload {
		return fmt.Errorf("auto-reload is not enabled")
	}

	m.mutex.Lock()
	if m.reloadCancel != nil {
		m.reloadCancel() // Cancel any existing reload
	}

	reloadCtx, cancel := context.WithCancel(ctx)
	m.reloadCancel = cancel
	m.mutex.Unlock()

	go m.autoReloadLoop(reloadCtx)

	return nil
}

// StopAutoReload stops automatic configuration reloading
func (m *ConfigManager) StopAutoReload() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.reloadCancel != nil {
		m.reloadCancel()
		m.reloadCancel = nil
	}
}

// IsConfigured returns true if configuration has been loaded
func (m *ConfigManager) IsConfigured() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.currentConfig != nil
}

// GetConfigSources returns a copy of the configuration sources map
func (m *ConfigManager) GetConfigSources() map[string]ConfigSource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.currentSources == nil {
		return make(map[string]ConfigSource)
	}

	result := make(map[string]ConfigSource)
	for k, v := range m.currentSources {
		result[k] = v
	}

	return result
}

// GetLoadedAt returns the time when the configuration was last loaded
func (m *ConfigManager) GetLoadedAt() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.loadedAt
}

// ValidateConfiguration validates the current configuration
func (m *ConfigManager) ValidateConfiguration() error {
	m.mutex.RLock()
	config := m.currentConfig
	m.mutex.RUnlock()

	if config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	return config.Validate()
}

// GetNamespace returns the namespace used by this manager
func (m *ConfigManager) GetNamespace() string {
	return m.namespace
}

// GetOptions returns a copy of the manager options
func (m *ConfigManager) GetOptions() ManagerOptions {
	return *m.options
}

// autoReloadLoop runs the automatic reload loop
func (m *ConfigManager) autoReloadLoop(ctx context.Context) {
	ticker := time.NewTicker(m.options.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Attempt to reload configuration
			_, err := m.ReloadConfig(ctx)
			if err != nil {
				// In a real implementation, we might want to log this error
				// For now, we'll continue trying
				continue
			}

		case <-ctx.Done():
			return
		}
	}
}

// notifyCallbacks notifies all registered callbacks of a configuration change
func (m *ConfigManager) notifyCallbacks(oldConfig, newConfig *Config) {
	// Make copies to avoid race conditions
	oldCopy := oldConfig.Copy()
	newCopy := newConfig.Copy()

	// Call callbacks in separate goroutines to avoid blocking
	for _, callback := range m.callbacks {
		go func(cb ConfigChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					// Handle panics in callbacks gracefully
					// Log the panic and continue with other callbacks
					_ = r // Acknowledge recovery value
				}
			}()
			cb(oldCopy, newCopy)
		}(callback)
	}
}

// updateConfig updates the current configuration (internal method)
func (m *ConfigManager) updateConfig(newConfig *Config) {
	m.mutex.Lock()
	oldConfig := m.currentConfig
	m.currentConfig = newConfig
	m.loadedAt = time.Now()
	if newConfig != nil && newConfig.Features != nil {
		m.featureFlagManager = NewFeatureFlagManager(newConfig.Features)
	}
	m.mutex.Unlock()

	if oldConfig != nil {
		m.notifyCallbacks(oldConfig, newConfig)
	}
}

// Validate validates the manager options
func (opts *ManagerOptions) Validate() error {
	if opts.Environment != "" {
		if err := ValidateEnvironment(opts.Environment); err != nil {
			return fmt.Errorf("invalid environment in options: %w", err)
		}
	}

	if opts.AutoReload && opts.ReloadInterval < 1*time.Second {
		return fmt.Errorf("reload interval must be at least 1 second when auto-reload is enabled, got: %v",
			opts.ReloadInterval)
	}

	return nil
}

// String returns a string representation of the manager options
func (opts *ManagerOptions) String() string {
	return fmt.Sprintf("ManagerOptions{Environment: %s, AutoReload: %v, ReloadInterval: %v, ValidateOnLoad: %v}",
		opts.Environment, opts.AutoReload, opts.ReloadInterval, opts.ValidateOnLoad)
}

// String returns a string representation of the configuration manager
func (m *ConfigManager) String() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := "not configured"
	if m.currentConfig != nil {
		status = fmt.Sprintf("configured (loaded at %s)", m.loadedAt.Format(time.RFC3339))
	}

	autoReload := "disabled"
	if m.options.AutoReload {
		autoReload = fmt.Sprintf("enabled (interval: %v)", m.options.ReloadInterval)
	}

	return fmt.Sprintf("ConfigManager{namespace: %s, status: %s, auto-reload: %s, callbacks: %d}",
		m.namespace, status, autoReload, len(m.callbacks))
}

// Close gracefully shuts down the configuration manager
func (m *ConfigManager) Close() error {
	m.StopAutoReload()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear callbacks
	m.callbacks = nil

	// Clear current state
	m.currentConfig = nil
	m.currentSources = nil
	m.featureFlagManager = nil
	m.loadOptions = nil

	return nil
}
