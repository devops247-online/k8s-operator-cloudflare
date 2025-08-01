package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// ConfigSource represents the source of configuration
//
//nolint:revive // ConfigSource name is intentional for this package
type ConfigSource int

const (
	// ConfigSourceDefault indicates default configuration
	ConfigSourceDefault ConfigSource = iota
	// ConfigSourceEnv indicates configuration from environment variables
	ConfigSourceEnv
	// ConfigSourceFile indicates configuration from file
	ConfigSourceFile
	// ConfigSourceConfigMap indicates configuration from ConfigMap
	ConfigSourceConfigMap
	// ConfigSourceSecret indicates configuration from Secret
	ConfigSourceSecret
)

// String returns the string representation of ConfigSource
func (cs ConfigSource) String() string {
	switch cs {
	case ConfigSourceDefault:
		return "default"
	case ConfigSourceEnv:
		return "env"
	case ConfigSourceFile:
		return "file"
	case ConfigSourceConfigMap:
		return "configmap"
	case ConfigSourceSecret:
		return "secret"
	default:
		return "unknown"
	}
}

// Config represents the complete operator configuration
type Config struct {
	// Environment specifies the deployment environment (production, staging, development)
	Environment string `json:"environment" yaml:"environment"`

	// Operator contains operator-specific configuration
	Operator OperatorConfig `json:"operator" yaml:"operator"`

	// Cloudflare contains Cloudflare API configuration
	Cloudflare CloudflareConfig `json:"cloudflare" yaml:"cloudflare"`

	// Features contains feature flags
	Features *FeatureFlags `json:"features,omitempty" yaml:"features,omitempty"`

	// Performance contains performance tuning configuration
	Performance PerformanceConfig `json:"performance" yaml:"performance"`

	// ConfigSources tracks where each configuration value came from
	ConfigSources map[string]ConfigSource `json:"-" yaml:"-"`
}

// OperatorConfig contains operator-specific settings
type OperatorConfig struct {
	// LogLevel specifies the logging level (debug, info, warn, error)
	LogLevel string `json:"logLevel" yaml:"logLevel"`

	// ReconcileInterval specifies how often to reconcile resources
	ReconcileInterval time.Duration `json:"reconcileInterval" yaml:"reconcileInterval"`

	// MetricsBindAddress is the address the metric endpoint binds to
	MetricsBindAddress string `json:"metricsBindAddress,omitempty" yaml:"metricsBindAddress,omitempty"`

	// HealthProbeBindAddress is the address the health probe endpoint binds to
	HealthProbeBindAddress string `json:"healthProbeBindAddress,omitempty" yaml:"healthProbeBindAddress,omitempty"`

	// LeaderElection enables leader election for controller manager
	LeaderElection bool `json:"leaderElection" yaml:"leaderElection"`
}

// CloudflareConfig contains Cloudflare API settings
type CloudflareConfig struct {
	// APITimeout specifies the timeout for Cloudflare API calls
	APITimeout time.Duration `json:"apiTimeout" yaml:"apiTimeout"`

	// RateLimitRPS specifies the rate limit in requests per second
	RateLimitRPS int `json:"rateLimitRPS" yaml:"rateLimitRPS"`

	// RetryAttempts specifies the number of retry attempts for failed API calls
	RetryAttempts int `json:"retryAttempts,omitempty" yaml:"retryAttempts,omitempty"`

	// RetryDelay specifies the delay between retry attempts
	RetryDelay time.Duration `json:"retryDelay,omitempty" yaml:"retryDelay,omitempty"`
}

// FeatureFlags contains feature toggle configuration
type FeatureFlags struct {
	// EnableWebhooks enables admission webhooks
	EnableWebhooks bool `json:"enableWebhooks" yaml:"enableWebhooks"`

	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool `json:"enableMetrics" yaml:"enableMetrics"`

	// EnableTracing enables distributed tracing
	EnableTracing bool `json:"enableTracing" yaml:"enableTracing"`

	// ExperimentalFeatures enables experimental features
	ExperimentalFeatures bool `json:"experimentalFeatures" yaml:"experimentalFeatures"`

	// CustomFlags allows for additional feature flags
	CustomFlags map[string]bool `json:"customFlags,omitempty" yaml:"customFlags,omitempty"`
}

// PerformanceConfig contains performance tuning settings
type PerformanceConfig struct {
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles
	MaxConcurrentReconciles int `json:"maxConcurrentReconciles" yaml:"maxConcurrentReconciles"`

	// ReconcileTimeout is the timeout for individual reconcile operations
	ReconcileTimeout time.Duration `json:"reconcileTimeout" yaml:"reconcileTimeout"`

	// RequeueInterval is the interval for requeueing successful reconciles
	RequeueInterval time.Duration `json:"requeueInterval" yaml:"requeueInterval"`

	// RequeueIntervalOnError is the interval for requeueing failed reconciles
	RequeueIntervalOnError time.Duration `json:"requeueIntervalOnError" yaml:"requeueIntervalOnError"`

	// ResyncPeriod is the resync period for informers
	ResyncPeriod time.Duration `json:"resyncPeriod" yaml:"resyncPeriod"`

	// LeaderElectionLeaseDuration is the duration that non-leader candidates will wait to force acquire leadership
	LeaderElectionLeaseDuration time.Duration `json:"leaderElectionLeaseDuration" yaml:"leaderElectionLeaseDuration"`

	// LeaderElectionRenewDeadline is the duration that the acting leader will retry refreshing leadership
	LeaderElectionRenewDeadline time.Duration `json:"leaderElectionRenewDeadline,omitempty" yaml:"leaderElectionRenewDeadline,omitempty"`

	// LeaderElectionRetryPeriod is the duration the LeaderElector clients should wait between tries
	LeaderElectionRetryPeriod time.Duration `json:"leaderElectionRetryPeriod,omitempty" yaml:"leaderElectionRetryPeriod,omitempty"`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Environment: ProductionEnv,
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
			CustomFlags:          make(map[string]bool),
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
		ConfigSources: make(map[string]ConfigSource),
	}
}

// GetEnvironmentDefaults returns default configuration for a specific environment
func GetEnvironmentDefaults(env string) *Config {
	base := NewConfig()
	base.Environment = env

	switch env {
	case ProductionEnv:
		// Production defaults are already set in NewConfig()
		return base

	case StagingEnv:
		base.Operator.LogLevel = DebugLevel
		base.Operator.ReconcileInterval = 1 * time.Minute
		base.Cloudflare.APITimeout = 15 * time.Second
		base.Cloudflare.RateLimitRPS = 5
		return base

	case DevelopmentEnv:
		base.Operator.LogLevel = DebugLevel
		base.Operator.ReconcileInterval = 30 * time.Second
		base.Cloudflare.APITimeout = 10 * time.Second
		base.Cloudflare.RateLimitRPS = 2
		base.Features.ExperimentalFeatures = true
		return base

	default:
		return base
	}
}

// Merge merges another config into this one, with the other config taking precedence
func (c *Config) Merge(other *Config) *Config {
	if other == nil {
		return c.Copy()
	}

	result := c.Copy()

	// Merge basic fields
	if other.Environment != "" {
		result.Environment = other.Environment
	}

	// Merge operator config
	if other.Operator.LogLevel != "" {
		result.Operator.LogLevel = other.Operator.LogLevel
	}
	if other.Operator.ReconcileInterval != 0 {
		result.Operator.ReconcileInterval = other.Operator.ReconcileInterval
	}
	if other.Operator.MetricsBindAddress != "" {
		result.Operator.MetricsBindAddress = other.Operator.MetricsBindAddress
	}
	if other.Operator.HealthProbeBindAddress != "" {
		result.Operator.HealthProbeBindAddress = other.Operator.HealthProbeBindAddress
	}

	// Merge cloudflare config
	if other.Cloudflare.APITimeout != 0 {
		result.Cloudflare.APITimeout = other.Cloudflare.APITimeout
	}
	if other.Cloudflare.RateLimitRPS != 0 {
		result.Cloudflare.RateLimitRPS = other.Cloudflare.RateLimitRPS
	}
	if other.Cloudflare.RetryAttempts != 0 {
		result.Cloudflare.RetryAttempts = other.Cloudflare.RetryAttempts
	}
	if other.Cloudflare.RetryDelay != 0 {
		result.Cloudflare.RetryDelay = other.Cloudflare.RetryDelay
	}

	// Merge feature flags
	if other.Features != nil {
		if result.Features == nil {
			result.Features = &FeatureFlags{
				CustomFlags: make(map[string]bool),
			}
		}
		if other.Features.EnableWebhooks {
			result.Features.EnableWebhooks = true
		}
		if other.Features.EnableMetrics {
			result.Features.EnableMetrics = true
		}
		if other.Features.EnableTracing {
			result.Features.EnableTracing = true
		}
		if other.Features.ExperimentalFeatures {
			result.Features.ExperimentalFeatures = true
		}
		// Merge custom flags
		for k, v := range other.Features.CustomFlags {
			result.Features.CustomFlags[k] = v
		}
	}

	// Merge performance config
	if other.Performance.MaxConcurrentReconciles != 0 {
		result.Performance.MaxConcurrentReconciles = other.Performance.MaxConcurrentReconciles
	}
	if other.Performance.ReconcileTimeout != 0 {
		result.Performance.ReconcileTimeout = other.Performance.ReconcileTimeout
	}
	if other.Performance.RequeueInterval != 0 {
		result.Performance.RequeueInterval = other.Performance.RequeueInterval
	}
	if other.Performance.RequeueIntervalOnError != 0 {
		result.Performance.RequeueIntervalOnError = other.Performance.RequeueIntervalOnError
	}
	if other.Performance.ResyncPeriod != 0 {
		result.Performance.ResyncPeriod = other.Performance.ResyncPeriod
	}
	if other.Performance.LeaderElectionLeaseDuration != 0 {
		result.Performance.LeaderElectionLeaseDuration = other.Performance.LeaderElectionLeaseDuration
	}
	if other.Performance.LeaderElectionRenewDeadline != 0 {
		result.Performance.LeaderElectionRenewDeadline = other.Performance.LeaderElectionRenewDeadline
	}
	if other.Performance.LeaderElectionRetryPeriod != 0 {
		result.Performance.LeaderElectionRetryPeriod = other.Performance.LeaderElectionRetryPeriod
	}

	return result
}

// Copy creates a deep copy of the configuration
func (c *Config) Copy() *Config {
	if c == nil {
		return nil
	}

	result := &Config{
		Environment:   c.Environment,
		Operator:      c.Operator,
		Cloudflare:    c.Cloudflare,
		Performance:   c.Performance,
		ConfigSources: make(map[string]ConfigSource),
	}

	// Deep copy feature flags
	if c.Features != nil {
		result.Features = &FeatureFlags{
			EnableWebhooks:       c.Features.EnableWebhooks,
			EnableMetrics:        c.Features.EnableMetrics,
			EnableTracing:        c.Features.EnableTracing,
			ExperimentalFeatures: c.Features.ExperimentalFeatures,
			CustomFlags:          make(map[string]bool),
		}
		for k, v := range c.Features.CustomFlags {
			result.Features.CustomFlags[k] = v
		}
	}

	// Copy config sources
	for k, v := range c.ConfigSources {
		result.ConfigSources[k] = v
	}

	return result
}

// ToJSON converts the configuration to JSON
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// FromJSON loads configuration from JSON
func (c *Config) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// IsValid validates the operator configuration
func (oc *OperatorConfig) IsValid() error {
	if err := ValidateLogLevel(oc.LogLevel); err != nil {
		return err
	}

	if err := ValidateDuration(oc.ReconcileInterval, 1*time.Second, 1*time.Hour, "reconcile interval"); err != nil {
		return err
	}

	return nil
}

// IsValid validates the cloudflare configuration
func (cc *CloudflareConfig) IsValid() error {
	if err := ValidateDuration(cc.APITimeout, 1*time.Second, 5*time.Minute, "API timeout"); err != nil {
		return err
	}

	if err := ValidatePositiveInt(cc.RateLimitRPS, "rate limit RPS"); err != nil {
		return err
	}

	if cc.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative, got: %d", cc.RetryAttempts)
	}

	if cc.RetryDelay > 0 {
		if err := ValidateDuration(cc.RetryDelay, 100*time.Millisecond, 30*time.Second, "retry delay"); err != nil {
			return err
		}
	}

	return nil
}

// IsValid validates the performance configuration
func (pc *PerformanceConfig) IsValid() error {
	if err := ValidatePositiveInt(pc.MaxConcurrentReconciles, "max concurrent reconciles"); err != nil {
		return err
	}

	if err := ValidateDuration(pc.ReconcileTimeout, 1*time.Second, 30*time.Minute, "reconcile timeout"); err != nil {
		return err
	}

	if err := ValidateDuration(pc.RequeueInterval, 1*time.Second, 24*time.Hour, "requeue interval"); err != nil {
		return err
	}

	if err := ValidateDuration(pc.RequeueIntervalOnError, 1*time.Second, 1*time.Hour, "requeue interval on error"); err != nil {
		return err
	}

	if err := ValidateDuration(pc.ResyncPeriod, 1*time.Minute, 24*time.Hour, "resync period"); err != nil {
		return err
	}

	if err := ValidateDuration(pc.LeaderElectionLeaseDuration, 1*time.Second, 5*time.Minute, "leader election lease duration"); err != nil {
		return err
	}

	if pc.LeaderElectionRenewDeadline > 0 {
		if err := ValidateDuration(pc.LeaderElectionRenewDeadline, 1*time.Second, 1*time.Minute, "leader election renew deadline"); err != nil {
			return err
		}
	}

	if pc.LeaderElectionRetryPeriod > 0 {
		if err := ValidateDuration(pc.LeaderElectionRetryPeriod, 100*time.Millisecond, 30*time.Second, "leader election retry period"); err != nil {
			return err
		}
	}

	return nil
}
