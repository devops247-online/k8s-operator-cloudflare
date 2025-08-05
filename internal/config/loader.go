package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ConfigLoader handles loading configuration from various sources
type ConfigLoader struct { //nolint:revive // ConfigLoader is clear and consistent
	client    client.Client
	namespace string
}

// LoadOptions specifies options for loading configuration
type LoadOptions struct {
	// FilePath specifies the path to a configuration file
	FilePath string

	// ConfigMapName specifies the name of a ConfigMap to load from
	ConfigMapName string

	// SecretName specifies the name of a Secret to load from
	SecretName string

	// LoadFromEnv indicates whether to load from environment variables
	LoadFromEnv bool

	// ValidateConfig indicates whether to validate the final configuration
	ValidateConfig bool
}

// WatchOptions specifies options for watching configuration changes
type WatchOptions struct {
	// ConfigMapName specifies the ConfigMap to watch
	ConfigMapName string

	// SecretName specifies the Secret to watch
	SecretName string

	// Interval specifies how often to check for changes
	Interval time.Duration
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(kubeClient client.Client, namespace string) *ConfigLoader {
	return &ConfigLoader{
		client:    kubeClient,
		namespace: namespace,
	}
}

// LoadFromFile loads configuration from a file (JSON or YAML)
func (cl *ConfigLoader) LoadFromFile(filePath string) (*Config, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Validate file path to prevent path traversal
	cleanPath := filepath.Clean(filePath)
	if cleanPath != filePath {
		return nil, fmt.Errorf("invalid file path: path traversal detected")
	}

	// #nosec G304 - file path is validated above
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("config file %s is empty", filePath)
	}

	config := &Config{}

	// Determine file type by extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", filePath, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", filePath, err)
		}
	default:
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, config); err != nil {
			if yamlErr := yaml.Unmarshal(data, config); yamlErr != nil {
				return nil, fmt.Errorf("failed to parse config file %s as JSON or YAML: JSON error: %v, YAML error: %v",
					filePath, err, yamlErr)
			}
		}
	}

	return config, nil
}

// LoadFromEnv loads configuration from environment variables
func (cl *ConfigLoader) LoadFromEnv() (*Config, error) {
	config := &Config{
		ConfigSources: make(map[string]ConfigSource),
	}

	// Load basic fields
	cl.loadBasicFields(config)

	// Load operator configuration
	cl.loadOperatorConfig(config)

	// Load Cloudflare configuration
	cl.loadCloudflareConfig(config)

	// Load feature flags
	cl.loadFeatureFlags(config)

	// Load performance configuration
	cl.loadPerformanceConfig(config)

	return config, nil
}

// loadBasicFields loads basic configuration fields from environment
func (cl *ConfigLoader) loadBasicFields(config *Config) {
	if env := os.Getenv("CONFIG_ENVIRONMENT"); env != "" {
		config.Environment = env
		config.ConfigSources["environment"] = ConfigSourceEnv
	}
}

// loadOperatorConfig loads operator configuration from environment
func (cl *ConfigLoader) loadOperatorConfig(config *Config) {
	if logLevel := os.Getenv("CONFIG_OPERATOR_LOG_LEVEL"); logLevel != "" {
		config.Operator.LogLevel = logLevel
		config.ConfigSources["operator.logLevel"] = ConfigSourceEnv
	}

	if reconcileInterval := os.Getenv("CONFIG_OPERATOR_RECONCILE_INTERVAL"); reconcileInterval != "" {
		if duration, err := time.ParseDuration(reconcileInterval); err == nil {
			config.Operator.ReconcileInterval = duration
			config.ConfigSources["operator.reconcileInterval"] = ConfigSourceEnv
		}
	}

	if metricsAddr := os.Getenv("CONFIG_OPERATOR_METRICS_BIND_ADDRESS"); metricsAddr != "" {
		config.Operator.MetricsBindAddress = metricsAddr
		config.ConfigSources["operator.metricsBindAddress"] = ConfigSourceEnv
	}

	if healthAddr := os.Getenv("CONFIG_OPERATOR_HEALTH_PROBE_BIND_ADDRESS"); healthAddr != "" {
		config.Operator.HealthProbeBindAddress = healthAddr
		config.ConfigSources["operator.healthProbeBindAddress"] = ConfigSourceEnv
	}

	if leaderElection := os.Getenv("CONFIG_OPERATOR_LEADER_ELECTION"); leaderElection != "" {
		if enabled, err := strconv.ParseBool(leaderElection); err == nil {
			config.Operator.LeaderElection = enabled
			config.ConfigSources["operator.leaderElection"] = ConfigSourceEnv
		}
	}
}

// loadCloudflareConfig loads Cloudflare configuration from environment
func (cl *ConfigLoader) loadCloudflareConfig(config *Config) {
	if apiTimeout := os.Getenv("CONFIG_CLOUDFLARE_API_TIMEOUT"); apiTimeout != "" {
		if duration, err := time.ParseDuration(apiTimeout); err == nil {
			config.Cloudflare.APITimeout = duration
			config.ConfigSources["cloudflare.apiTimeout"] = ConfigSourceEnv
		}
	}

	if rateLimitRPS := os.Getenv("CONFIG_CLOUDFLARE_RATE_LIMIT_RPS"); rateLimitRPS != "" {
		if rps, err := strconv.Atoi(rateLimitRPS); err == nil {
			config.Cloudflare.RateLimitRPS = rps
			config.ConfigSources["cloudflare.rateLimitRPS"] = ConfigSourceEnv
		}
	}

	if retryAttempts := os.Getenv("CONFIG_CLOUDFLARE_RETRY_ATTEMPTS"); retryAttempts != "" {
		if attempts, err := strconv.Atoi(retryAttempts); err == nil {
			config.Cloudflare.RetryAttempts = attempts
			config.ConfigSources["cloudflare.retryAttempts"] = ConfigSourceEnv
		}
	}

	if retryDelay := os.Getenv("CONFIG_CLOUDFLARE_RETRY_DELAY"); retryDelay != "" {
		if duration, err := time.ParseDuration(retryDelay); err == nil {
			config.Cloudflare.RetryDelay = duration
			config.ConfigSources["cloudflare.retryDelay"] = ConfigSourceEnv
		}
	}
}

// loadFeatureFlags loads feature flags from environment
func (cl *ConfigLoader) loadFeatureFlags(config *Config) {
	config.Features = &FeatureFlags{
		CustomFlags: make(map[string]bool),
	}

	if enableWebhooks := os.Getenv("CONFIG_FEATURES_ENABLE_WEBHOOKS"); enableWebhooks != "" {
		if enabled, err := strconv.ParseBool(enableWebhooks); err == nil {
			config.Features.EnableWebhooks = enabled
			config.ConfigSources["features.enableWebhooks"] = ConfigSourceEnv
		}
	}

	if enableMetrics := os.Getenv("CONFIG_FEATURES_ENABLE_METRICS"); enableMetrics != "" {
		if enabled, err := strconv.ParseBool(enableMetrics); err == nil {
			config.Features.EnableMetrics = enabled
			config.ConfigSources["features.enableMetrics"] = ConfigSourceEnv
		}
	}

	if enableTracing := os.Getenv("CONFIG_FEATURES_ENABLE_TRACING"); enableTracing != "" {
		if enabled, err := strconv.ParseBool(enableTracing); err == nil {
			config.Features.EnableTracing = enabled
			config.ConfigSources["features.enableTracing"] = ConfigSourceEnv
		}
	}

	if experimentalFeatures := os.Getenv("CONFIG_FEATURES_EXPERIMENTAL_FEATURES"); experimentalFeatures != "" {
		if enabled, err := strconv.ParseBool(experimentalFeatures); err == nil {
			config.Features.ExperimentalFeatures = enabled
			config.ConfigSources["features.experimentalFeatures"] = ConfigSourceEnv
		}
	}
}

// loadPerformanceConfig loads performance configuration from environment
func (cl *ConfigLoader) loadPerformanceConfig(config *Config) {
	if maxConcurrent := os.Getenv("CONFIG_PERFORMANCE_MAX_CONCURRENT_RECONCILES"); maxConcurrent != "" {
		if maxVal, err := strconv.Atoi(maxConcurrent); err == nil {
			config.Performance.MaxConcurrentReconciles = maxVal
			config.ConfigSources["performance.maxConcurrentReconciles"] = ConfigSourceEnv
		}
	}

	if resyncPeriod := os.Getenv("CONFIG_PERFORMANCE_RESYNC_PERIOD"); resyncPeriod != "" {
		if duration, err := time.ParseDuration(resyncPeriod); err == nil {
			config.Performance.ResyncPeriod = duration
			config.ConfigSources["performance.resyncPeriod"] = ConfigSourceEnv
		}
	}

	if leaseDuration := os.Getenv("CONFIG_PERFORMANCE_LEADER_ELECTION_LEASE_DURATION"); leaseDuration != "" {
		if duration, err := time.ParseDuration(leaseDuration); err == nil {
			config.Performance.LeaderElectionLeaseDuration = duration
			config.ConfigSources["performance.leaderElectionLeaseDuration"] = ConfigSourceEnv
		}
	}

	if renewDeadline := os.Getenv("CONFIG_PERFORMANCE_LEADER_ELECTION_RENEW_DEADLINE"); renewDeadline != "" {
		if duration, err := time.ParseDuration(renewDeadline); err == nil {
			config.Performance.LeaderElectionRenewDeadline = duration
			config.ConfigSources["performance.leaderElectionRenewDeadline"] = ConfigSourceEnv
		}
	}

	if retryPeriod := os.Getenv("CONFIG_PERFORMANCE_LEADER_ELECTION_RETRY_PERIOD"); retryPeriod != "" {
		if duration, err := time.ParseDuration(retryPeriod); err == nil {
			config.Performance.LeaderElectionRetryPeriod = duration
			config.ConfigSources["performance.leaderElectionRetryPeriod"] = ConfigSourceEnv
		}
	}
}

// LoadFromConfigMap loads configuration from a Kubernetes ConfigMap
func (cl *ConfigLoader) LoadFromConfigMap(ctx context.Context, name string) (*Config, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("kubernetes client is required for loading from ConfigMap")
	}

	configMap := &corev1.ConfigMap{}
	err := cl.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: cl.namespace,
	}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("ConfigMap %s/%s not found", cl.namespace, name)
		}
		return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", cl.namespace, name, err)
	}

	config := &Config{
		ConfigSources: make(map[string]ConfigSource),
	}

	// Try to load from structured config files first
	for key, value := range configMap.Data {
		switch key {
		case "config.json":
			tempConfig := &Config{}
			if err := json.Unmarshal([]byte(value), tempConfig); err == nil {
				config = config.Merge(tempConfig)
				// Mark all fields as coming from ConfigMap
				cl.markFieldsFromSource(config, ConfigSourceConfigMap)
				return config, nil
			}
		case "config.yaml", "config.yml":
			tempConfig := &Config{}
			if err := yaml.Unmarshal([]byte(value), tempConfig); err == nil {
				config = config.Merge(tempConfig)
				// Mark all fields as coming from ConfigMap
				cl.markFieldsFromSource(config, ConfigSourceConfigMap)
				return config, nil
			}
		}
	}

	// Load from individual keys
	if env, exists := configMap.Data["environment"]; exists {
		config.Environment = env
		config.ConfigSources["environment"] = ConfigSourceConfigMap
	}

	if logLevel, exists := configMap.Data["log-level"]; exists {
		config.Operator.LogLevel = logLevel
		config.ConfigSources["operator.logLevel"] = ConfigSourceConfigMap
	}

	if reconcileInterval, exists := configMap.Data["reconcile-interval"]; exists {
		if duration, err := time.ParseDuration(reconcileInterval); err == nil {
			config.Operator.ReconcileInterval = duration
			config.ConfigSources["operator.reconcileInterval"] = ConfigSourceConfigMap
		}
	}

	if apiTimeout, exists := configMap.Data["api-timeout"]; exists {
		if duration, err := time.ParseDuration(apiTimeout); err == nil {
			config.Cloudflare.APITimeout = duration
			config.ConfigSources["cloudflare.apiTimeout"] = ConfigSourceConfigMap
		}
	}

	if rateLimitRPS, exists := configMap.Data["rate-limit-rps"]; exists {
		if rps, err := strconv.Atoi(rateLimitRPS); err == nil {
			config.Cloudflare.RateLimitRPS = rps
			config.ConfigSources["cloudflare.rateLimitRPS"] = ConfigSourceConfigMap
		}
	}

	return config, nil
}

// LoadFromSecret loads configuration from a Kubernetes Secret
func (cl *ConfigLoader) LoadFromSecret(ctx context.Context, name string) (*Config, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("kubernetes client is required for loading from Secret")
	}

	secret := &corev1.Secret{}
	err := cl.client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: cl.namespace,
	}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %s/%s not found", cl.namespace, name)
		}
		return nil, fmt.Errorf("failed to get Secret %s/%s: %w", cl.namespace, name, err)
	}

	config := &Config{
		ConfigSources: make(map[string]ConfigSource),
	}

	// Try to load from structured config files first
	for key, value := range secret.Data {
		switch key {
		case "config.json":
			tempConfig := &Config{}
			if err := json.Unmarshal(value, tempConfig); err == nil {
				config = config.Merge(tempConfig)
				// Mark all fields as coming from Secret
				cl.markFieldsFromSource(config, ConfigSourceSecret)
				return config, nil
			}
		case "config.yaml", "config.yml":
			tempConfig := &Config{}
			if err := yaml.Unmarshal(value, tempConfig); err == nil {
				config = config.Merge(tempConfig)
				// Mark all fields as coming from Secret
				cl.markFieldsFromSource(config, ConfigSourceSecret)
				return config, nil
			}
		}
	}

	// Load from individual keys
	if env, exists := secret.Data["environment"]; exists {
		config.Environment = string(env)
		config.ConfigSources["environment"] = ConfigSourceSecret
	}

	if apiTimeout, exists := secret.Data["api-timeout"]; exists {
		if duration, err := time.ParseDuration(string(apiTimeout)); err == nil {
			config.Cloudflare.APITimeout = duration
			config.ConfigSources["cloudflare.apiTimeout"] = ConfigSourceSecret
		}
	}

	if rateLimitRPS, exists := secret.Data["rate-limit-rps"]; exists {
		if rps, err := strconv.Atoi(string(rateLimitRPS)); err == nil {
			config.Cloudflare.RateLimitRPS = rps
			config.ConfigSources["cloudflare.rateLimitRPS"] = ConfigSourceSecret
		}
	}

	return config, nil
}

// LoadWithPriority loads configuration from multiple sources with priority:
// Environment variables > ConfigMap > Secret > File > Defaults
func (cl *ConfigLoader) LoadWithPriority(ctx context.Context, options LoadOptions) (
	*Config, map[string]ConfigSource, error) {
	if err := options.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid load options: %w", err)
	}

	// Start with defaults
	config := NewConfig()
	sources := make(map[string]ConfigSource)

	// Mark all default values
	cl.markFieldsFromSource(config, ConfigSourceDefault)
	for key := range config.ConfigSources {
		sources[key] = ConfigSourceDefault
	}

	// Load from file (lowest priority after defaults)
	if options.FilePath != "" {
		fileConfig, err := cl.LoadFromFile(options.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load from file: %w", err)
		}
		config = config.Merge(fileConfig)
		cl.markFieldsFromSource(fileConfig, ConfigSourceFile)
		for key := range fileConfig.ConfigSources {
			sources[key] = ConfigSourceFile
		}
	}

	// Load from Secret
	if options.SecretName != "" {
		secretConfig, err := cl.LoadFromSecret(ctx, options.SecretName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load from Secret: %w", err)
		}
		config = config.Merge(secretConfig)
		for key, source := range secretConfig.ConfigSources {
			sources[key] = source
		}
	}

	// Load from ConfigMap
	if options.ConfigMapName != "" {
		configMapConfig, err := cl.LoadFromConfigMap(ctx, options.ConfigMapName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load from ConfigMap: %w", err)
		}
		config = config.Merge(configMapConfig)
		for key, source := range configMapConfig.ConfigSources {
			sources[key] = source
		}
	}

	// Load from environment variables (highest priority)
	if options.LoadFromEnv {
		envConfig, err := cl.LoadFromEnv()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load from environment: %w", err)
		}
		config = config.Merge(envConfig)
		for key, source := range envConfig.ConfigSources {
			sources[key] = source
		}
	}

	// Validate if requested
	if options.ValidateConfig {
		if err := config.Validate(); err != nil {
			return nil, nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	return config, sources, nil
}

// WatchConfig watches for configuration changes and returns channels for updates
func (cl *ConfigLoader) WatchConfig(ctx context.Context, options WatchOptions) (<-chan *Config, <-chan error) {
	if err := options.Validate(); err != nil {
		errorChan := make(chan error, 1)
		errorChan <- fmt.Errorf("invalid watch options: %w", err)
		close(errorChan)
		return nil, errorChan
	}

	configChan := make(chan *Config, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(configChan)
		defer close(errorChan)

		ticker := time.NewTicker(options.Interval)
		defer ticker.Stop()

		var lastConfig *Config

		// Load initial configuration
		loadOptions := LoadOptions{
			ConfigMapName:  options.ConfigMapName,
			SecretName:     options.SecretName,
			LoadFromEnv:    true,
			ValidateConfig: true,
		}

		config, _, err := cl.LoadWithPriority(ctx, loadOptions)
		if err != nil {
			select {
			case errorChan <- err:
			case <-ctx.Done():
				return
			}
			return
		}

		lastConfig = config
		select {
		case configChan <- config:
		case <-ctx.Done():
			return
		}

		// Watch for changes
		for {
			select {
			case <-ticker.C:
				newConfig, _, err := cl.LoadWithPriority(ctx, loadOptions)
				if err != nil {
					select {
					case errorChan <- err:
					case <-ctx.Done():
						return
					}
					continue
				}

				// Check if configuration changed (simple string comparison)
				lastJSON, _ := lastConfig.ToJSON()
				newJSON, _ := newConfig.ToJSON()
				if string(lastJSON) != string(newJSON) {
					lastConfig = newConfig
					select {
					case configChan <- newConfig:
					case <-ctx.Done():
						return
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return configChan, errorChan
}

// markFieldsFromSource marks all non-zero fields in config as coming from the specified source
func (cl *ConfigLoader) markFieldsFromSource(config *Config, source ConfigSource) {
	if config.ConfigSources == nil {
		config.ConfigSources = make(map[string]ConfigSource)
	}

	if config.Environment != "" {
		config.ConfigSources["environment"] = source
	}
	if config.Operator.LogLevel != "" {
		config.ConfigSources["operator.logLevel"] = source
	}
	if config.Operator.ReconcileInterval != 0 {
		config.ConfigSources["operator.reconcileInterval"] = source
	}
	if config.Operator.MetricsBindAddress != "" {
		config.ConfigSources["operator.metricsBindAddress"] = source
	}
	if config.Operator.HealthProbeBindAddress != "" {
		config.ConfigSources["operator.healthProbeBindAddress"] = source
	}
	if config.Cloudflare.APITimeout != 0 {
		config.ConfigSources["cloudflare.apiTimeout"] = source
	}
	if config.Cloudflare.RateLimitRPS != 0 {
		config.ConfigSources["cloudflare.rateLimitRPS"] = source
	}
	if config.Performance.MaxConcurrentReconciles != 0 {
		config.ConfigSources["performance.maxConcurrentReconciles"] = source
	}
	if config.Performance.ResyncPeriod != 0 {
		config.ConfigSources["performance.resyncPeriod"] = source
	}
	if config.Performance.LeaderElectionLeaseDuration != 0 {
		config.ConfigSources["performance.leaderElectionLeaseDuration"] = source
	}
}

// Validate validates the load options
func (opts *LoadOptions) Validate() error {
	if opts.FilePath == "" && opts.ConfigMapName == "" && opts.SecretName == "" && !opts.LoadFromEnv {
		return fmt.Errorf("at least one configuration source must be specified")
	}
	return nil
}

// Validate validates the watch options
func (opts *WatchOptions) Validate() error {
	if opts.ConfigMapName == "" && opts.SecretName == "" {
		return fmt.Errorf("at least one resource to watch must be specified")
	}

	if opts.Interval < 1*time.Second {
		return fmt.Errorf("watch interval must be at least 1 second, got: %v", opts.Interval)
	}

	return nil
}
