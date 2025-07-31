package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Tests to improve validation.go coverage
func TestConfig_Validate_AdditionalCases(t *testing.T) {
	t.Run("validates bind addresses - edge cases", func(t *testing.T) {
		config := NewConfig()
		config.Operator.MetricsBindAddress = "127.0.0.1:9090"
		config.Operator.HealthProbeBindAddress = "127.0.0.1:9091"

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("fails on same bind addresses", func(t *testing.T) {
		config := NewConfig()
		config.Operator.MetricsBindAddress = ":8080"
		config.Operator.HealthProbeBindAddress = ":8080"

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use the same bind address")
	})

	t.Run("allows same bind addresses with port 0", func(t *testing.T) {
		config := NewConfig()
		config.Operator.MetricsBindAddress = ":0"
		config.Operator.HealthProbeBindAddress = ":0"

		err := config.Validate()
		assert.NoError(t, err) // port 0 means OS chooses, so no conflict
	})
}

func TestValidateEnvironmentSpecific_AdditionalCases(t *testing.T) {
	t.Run("production - rate limit validation", func(t *testing.T) {
		config := GetEnvironmentDefaults("production")
		config.Cloudflare.RateLimitRPS = 100 // too high

		err := config.validateEnvironmentSpecific()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit <= 50 RPS")
	})

	t.Run("production - experimental features disabled", func(t *testing.T) {
		config := GetEnvironmentDefaults("production")
		config.Features.ExperimentalFeatures = true

		err := config.validateEnvironmentSpecific()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "experimental features should not be enabled in production")
	})

	t.Run("staging - minimum reconcile interval", func(t *testing.T) {
		config := GetEnvironmentDefaults("staging")
		config.Operator.ReconcileInterval = 5 * time.Second // too low

		err := config.validateEnvironmentSpecific()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "staging environment requires reconcile interval of at least 10s")
	})

	t.Run("development - minimum reconcile interval", func(t *testing.T) {
		config := GetEnvironmentDefaults("development")
		config.Operator.ReconcileInterval = 2 * time.Second // too low

		err := config.validateEnvironmentSpecific()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "development environment requires reconcile interval of at least 5s")
	})

	t.Run("test environment - no additional validations", func(t *testing.T) {
		config := GetEnvironmentDefaults("test")
		config.Operator.ReconcileInterval = 1 * time.Second // very low, but allowed in test

		err := config.validateEnvironmentSpecific()
		assert.NoError(t, err)
	})
}

func TestValidateCrossFields_AdditionalCases(t *testing.T) {
	t.Run("leader election timing - renew deadline >= lease duration", func(t *testing.T) {
		config := NewConfig()
		config.Performance.LeaderElectionLeaseDuration = 15 * time.Second
		config.Performance.LeaderElectionRenewDeadline = 15 * time.Second // equal, should fail

		err := config.validateCrossFields()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "renew deadline")
		assert.Contains(t, err.Error(), "must be less than lease duration")
	})

	t.Run("leader election timing - retry period >= renew deadline", func(t *testing.T) {
		config := NewConfig()
		config.Performance.LeaderElectionLeaseDuration = 30 * time.Second
		config.Performance.LeaderElectionRenewDeadline = 20 * time.Second
		config.Performance.LeaderElectionRetryPeriod = 20 * time.Second // equal, should fail

		err := config.validateCrossFields()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retry period")
		assert.Contains(t, err.Error(), "must be less than renew deadline")
	})

	t.Run("retry configuration - total retry time >= API timeout", func(t *testing.T) {
		config := NewConfig()
		config.Cloudflare.APITimeout = 10 * time.Second
		config.Cloudflare.RetryAttempts = 5
		config.Cloudflare.RetryDelay = 3 * time.Second // 5 * 3s = 15s > 10s timeout

		err := config.validateCrossFields()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "total retry time")
		assert.Contains(t, err.Error(), "should be less than API timeout")
	})

	t.Run("bind addresses - different hosts same port", func(t *testing.T) {
		config := NewConfig()
		config.Operator.MetricsBindAddress = "127.0.0.1:8080"
		config.Operator.HealthProbeBindAddress = "localhost:8080"

		// This should pass as hosts are different (even though they resolve to same)
		err := config.validateCrossFields()
		assert.NoError(t, err)
	})

	t.Run("bind addresses - malformed addresses", func(t *testing.T) {
		config := NewConfig()
		config.Operator.MetricsBindAddress = "invalid-address"
		config.Operator.HealthProbeBindAddress = ":8081"

		// Should fail due to malformed metrics address, not cross-field validation
		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metrics bind address")
	})
}

func TestValidateBindAddress_AdditionalCases(t *testing.T) {
	t.Run("validates IPv6 addresses", func(t *testing.T) {
		err := ValidateBindAddress("[::1]:8080")
		assert.NoError(t, err)
	})

	t.Run("validates hostname with port", func(t *testing.T) {
		err := ValidateBindAddress("example.com:8080")
		assert.NoError(t, err)
	})

	t.Run("rejects address with spaces in hostname", func(t *testing.T) {
		err := ValidateBindAddress("host name:8080")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid host")
	})

	t.Run("rejects address with tabs in hostname", func(t *testing.T) {
		err := ValidateBindAddress("host\tname:8080")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid host")
	})

	t.Run("validates port 0", func(t *testing.T) {
		err := ValidateBindAddress("localhost:0")
		assert.NoError(t, err)
	})

	t.Run("rejects invalid port", func(t *testing.T) {
		err := ValidateBindAddress("localhost:99999")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("rejects port with letters", func(t *testing.T) {
		err := ValidateBindAddress("localhost:abc")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})
}

func TestTypes_IsValid_AdditionalCases(t *testing.T) {
	t.Run("CloudflareConfig - zero retry delay is valid", func(t *testing.T) {
		config := CloudflareConfig{
			APITimeout:    30 * time.Second,
			RateLimitRPS:  10,
			RetryAttempts: 3,
			RetryDelay:    0, // zero is valid (no delay)
		}

		err := config.IsValid()
		assert.NoError(t, err)
	})

	t.Run("CloudflareConfig - retry delay validation", func(t *testing.T) {
		config := CloudflareConfig{
			APITimeout:    30 * time.Second,
			RateLimitRPS:  10,
			RetryAttempts: 3,
			RetryDelay:    50 * time.Millisecond, // too small
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retry delay")
	})

	t.Run("PerformanceConfig - optional fields zero values", func(t *testing.T) {
		config := PerformanceConfig{
			MaxConcurrentReconciles:     5,
			ReconcileTimeout:            5 * time.Minute,
			RequeueInterval:             5 * time.Minute,
			RequeueIntervalOnError:      1 * time.Minute,
			ResyncPeriod:                10 * time.Minute,
			LeaderElectionLeaseDuration: 15 * time.Second,
			// LeaderElectionRenewDeadline and LeaderElectionRetryPeriod are 0 (optional)
		}

		err := config.IsValid()
		assert.NoError(t, err)
	})

	t.Run("PerformanceConfig - invalid renew deadline", func(t *testing.T) {
		config := PerformanceConfig{
			MaxConcurrentReconciles:     5,
			ReconcileTimeout:            5 * time.Minute,
			RequeueInterval:             5 * time.Minute,
			RequeueIntervalOnError:      1 * time.Minute,
			ResyncPeriod:                10 * time.Minute,
			LeaderElectionLeaseDuration: 15 * time.Second,
			LeaderElectionRenewDeadline: 90 * time.Second, // too large
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "leader election renew deadline")
	})

	t.Run("PerformanceConfig - invalid retry period", func(t *testing.T) {
		config := PerformanceConfig{
			MaxConcurrentReconciles:     5,
			ReconcileTimeout:            5 * time.Minute,
			RequeueInterval:             5 * time.Minute,
			RequeueIntervalOnError:      1 * time.Minute,
			ResyncPeriod:                10 * time.Minute,
			LeaderElectionLeaseDuration: 15 * time.Second,
			LeaderElectionRetryPeriod:   45 * time.Second, // too large
		}

		err := config.IsValid()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "leader election retry period")
	})
}
