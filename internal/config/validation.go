package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate environment
	if err := ValidateEnvironment(c.Environment); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Validate operator configuration
	if err := c.Operator.IsValid(); err != nil {
		return fmt.Errorf("operator configuration validation failed: %w", err)
	}

	// Validate bind addresses
	if c.Operator.MetricsBindAddress != "" {
		if err := ValidateBindAddress(c.Operator.MetricsBindAddress); err != nil {
			return fmt.Errorf("invalid metrics bind address: %w", err)
		}
	}

	if c.Operator.HealthProbeBindAddress != "" {
		if err := ValidateBindAddress(c.Operator.HealthProbeBindAddress); err != nil {
			return fmt.Errorf("invalid health probe bind address: %w", err)
		}
	}

	// Validate cloudflare configuration
	if err := c.Cloudflare.IsValid(); err != nil {
		return fmt.Errorf("cloudflare configuration validation failed: %w", err)
	}

	// Validate performance configuration
	if err := c.Performance.IsValid(); err != nil {
		return fmt.Errorf("performance configuration validation failed: %w", err)
	}

	// Validate feature flags
	if err := c.Features.Validate(); err != nil {
		return fmt.Errorf("feature flags validation failed: %w", err)
	}

	// Environment-specific validation
	if err := c.validateEnvironmentSpecific(); err != nil {
		return fmt.Errorf("environment-specific validation failed: %w", err)
	}

	// Cross-field validation
	if err := c.validateCrossFields(); err != nil {
		return fmt.Errorf("cross-field validation failed: %w", err)
	}

	return nil
}

// ValidateEnvironment validates the environment string
func ValidateEnvironment(env string) error {
	if env == "" {
		return fmt.Errorf("environment cannot be empty")
	}

	validEnvironments := map[string]bool{
		ProductionEnv: true,
		"staging":     true,
		"development": true,
		TestEnv:       true,
	}

	if !validEnvironments[env] {
		return fmt.Errorf("invalid environment: %s, must be one of: production, staging, development, test", env)
	}

	return nil
}

// ValidateLogLevel validates the log level string
func ValidateLogLevel(level string) error {
	if level == "" {
		return fmt.Errorf("log level cannot be empty")
	}

	validLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"panic": true,
	}

	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s, must be one of: trace, debug, info, warn, error, fatal, panic", level)
	}

	return nil
}

// ValidateDuration validates a duration against min/max bounds
func ValidateDuration(duration, minDur, maxDur time.Duration, fieldName string) error {
	if duration <= 0 {
		return fmt.Errorf("%s must be positive, got: %v", fieldName, duration)
	}

	if duration < minDur {
		return fmt.Errorf("%s must be at least %v, got: %v", fieldName, minDur, duration)
	}

	if maxDur > 0 && duration > maxDur {
		return fmt.Errorf("%s must be at most %v, got: %v", fieldName, maxDur, duration)
	}

	return nil
}

// ValidatePositiveInt validates that an integer is positive
func ValidatePositiveInt(value int, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive, got: %d", fieldName, value)
	}
	return nil
}

// ValidatePortRange validates that a port is in valid range (1-65535)
func ValidatePortRange(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", port)
	}
	return nil
}

// ValidateBindAddress validates bind address format
func ValidateBindAddress(address string) error {
	if address == "" {
		return fmt.Errorf("bind address cannot be empty")
	}

	// Handle special case of just ":port"
	if strings.HasPrefix(address, ":") {
		portStr := address[1:]
		if portStr == "" {
			return fmt.Errorf("bind address cannot be just ':'")
		}
		if portStr == "0" {
			// ":0" is valid (let OS choose port)
			return nil
		}
	}

	// Try to parse as host:port
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid bind address format: %s", address)
	}

	// Validate host (can be empty for ":port" format)
	if host != "" {
		// Check if it's a valid IP address
		if ip := net.ParseIP(host); ip == nil {
			// If not an IP, could be hostname - basic validation
			if strings.Contains(host, " ") || strings.Contains(host, "\t") {
				return fmt.Errorf("invalid host in bind address: %s", host)
			}
		}
	}

	// Validate port
	if port != "0" { // "0" means let OS choose port
		if _, err := net.LookupPort("tcp", port); err != nil {
			return fmt.Errorf("invalid port in bind address: %s", port)
		}
	}

	return nil
}

// Validate validates feature flags
func (ff *FeatureFlags) Validate() error {
	if ff == nil {
		return nil // nil feature flags are allowed
	}

	// Currently no complex validation needed for feature flags
	// This method is here for future extensibility
	return nil
}

// validateEnvironmentSpecific performs environment-specific validation
func (c *Config) validateEnvironmentSpecific() error {
	switch c.Environment {
	case ProductionEnv:
		// Production-specific validations
		if c.Operator.ReconcileInterval < 30*time.Second {
			return fmt.Errorf("production environment requires reconcile interval of at least 30s, got: %v", c.Operator.ReconcileInterval)
		}

		if c.Cloudflare.RateLimitRPS > 50 {
			return fmt.Errorf("production environment should have rate limit <= 50 RPS for safety, got: %d", c.Cloudflare.RateLimitRPS)
		}

		// Ensure stable features are enabled in production
		if c.Features != nil && c.Features.ExperimentalFeatures {
			return fmt.Errorf("experimental features should not be enabled in production")
		}

	case "staging":
		// Staging-specific validations
		if c.Operator.ReconcileInterval < 10*time.Second {
			return fmt.Errorf("staging environment requires reconcile interval of at least 10s, got: %v", c.Operator.ReconcileInterval)
		}

	case "development":
		// Development-specific validations are more relaxed
		if c.Operator.ReconcileInterval < 5*time.Second {
			return fmt.Errorf("development environment requires reconcile interval of at least 5s, got: %v", c.Operator.ReconcileInterval)
		}

	case TestEnv:
		// Test environment is most permissive
		// No additional validations
	}

	return nil
}

// validateCrossFields performs cross-field validation
func (c *Config) validateCrossFields() error {
	// Validate leader election timing relationships
	if c.Performance.LeaderElectionRenewDeadline > 0 &&
		c.Performance.LeaderElectionLeaseDuration > 0 &&
		c.Performance.LeaderElectionRenewDeadline >= c.Performance.LeaderElectionLeaseDuration {
		return fmt.Errorf("leader election renew deadline (%v) must be less than lease duration (%v)",
			c.Performance.LeaderElectionRenewDeadline, c.Performance.LeaderElectionLeaseDuration)
	}

	if c.Performance.LeaderElectionRetryPeriod > 0 &&
		c.Performance.LeaderElectionRenewDeadline > 0 &&
		c.Performance.LeaderElectionRetryPeriod >= c.Performance.LeaderElectionRenewDeadline {
		return fmt.Errorf("leader election retry period (%v) must be less than renew deadline (%v)",
			c.Performance.LeaderElectionRetryPeriod, c.Performance.LeaderElectionRenewDeadline)
	}

	// Validate bind addresses don't conflict
	if c.Operator.MetricsBindAddress != "" && c.Operator.HealthProbeBindAddress != "" {
		if c.Operator.MetricsBindAddress == c.Operator.HealthProbeBindAddress {
			// Special case: port 0 means OS chooses random port, so multiple binds to :0 are allowed
			if c.Operator.MetricsBindAddress != ":0" {
				return fmt.Errorf("metrics and health probe cannot use the same bind address: %s", c.Operator.MetricsBindAddress)
			}
		}

		// Extract ports and check for conflicts on same host
		metricsHost, metricsPort, err1 := net.SplitHostPort(c.Operator.MetricsBindAddress)
		healthHost, healthPort, err2 := net.SplitHostPort(c.Operator.HealthProbeBindAddress)

		if err1 == nil && err2 == nil && metricsPort == healthPort && metricsPort != "0" {
			// Check if they're on the same host (or one is wildcard)
			if metricsHost == healthHost || metricsHost == "" || healthHost == "" ||
				metricsHost == "0.0.0.0" || healthHost == "0.0.0.0" {
				return fmt.Errorf("metrics and health probe cannot use the same port: %s", metricsPort)
			}
		}
	}

	// Validate retry configuration
	if c.Cloudflare.RetryDelay > 0 && c.Cloudflare.APITimeout > 0 {
		maxRetryTime := time.Duration(c.Cloudflare.RetryAttempts) * c.Cloudflare.RetryDelay
		if maxRetryTime >= c.Cloudflare.APITimeout {
			return fmt.Errorf("total retry time (%v) should be less than API timeout (%v)",
				maxRetryTime, c.Cloudflare.APITimeout)
		}
	}

	return nil
}
