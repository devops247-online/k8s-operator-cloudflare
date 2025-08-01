// Package tracing provides distributed tracing functionality for the Cloudflare DNS Operator.
// It implements OpenTelemetry-based tracing with configurable exporters and sampling strategies.
package tracing

import (
	"fmt"
	"strings"
	"time"
)

// Config represents the tracing configuration
type Config struct {
	// Enabled controls whether tracing is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// ServiceName is the name of the service for tracing
	ServiceName string `yaml:"service_name" json:"service_name"`

	// ServiceVersion is the version of the service
	ServiceVersion string `yaml:"service_version" json:"service_version"`

	// Environment is the deployment environment (dev, staging, prod)
	Environment string `yaml:"environment" json:"environment"`

	// Exporter configuration
	Exporter ExporterConfig `yaml:"exporter" json:"exporter"`

	// Sampling configuration
	Sampling SamplingConfig `yaml:"sampling" json:"sampling"`

	// Resource attributes
	ResourceAttributes map[string]string `yaml:"resource_attributes" json:"resource_attributes"`
}

// ExporterConfig represents exporter configuration
type ExporterConfig struct {
	// Type of exporter (otlp, jaeger, console, none)
	Type string `yaml:"type" json:"type"`

	// Endpoint for OTLP/Jaeger exporter
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// Insecure connection (for development)
	Insecure bool `yaml:"insecure" json:"insecure"`

	// Headers for OTLP exporter
	Headers map[string]string `yaml:"headers" json:"headers"`

	// Timeout for export operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// Compression type (gzip, none)
	Compression string `yaml:"compression" json:"compression"`
}

// SamplingConfig represents sampling configuration
type SamplingConfig struct {
	// Type of sampling (always, never, trace_id_ratio, parent_based)
	Type string `yaml:"type" json:"type"`

	// Ratio for trace_id_ratio sampling (0.0 to 1.0)
	Ratio float64 `yaml:"ratio" json:"ratio"`
}

// NewDefaultConfig returns a default tracing configuration
func NewDefaultConfig() Config {
	return Config{
		Enabled:        false,
		ServiceName:    "k8s-operator-cloudflare",
		ServiceVersion: "0.1.0",
		Environment:    "development",
		Exporter: ExporterConfig{
			Type:        "console",
			Endpoint:    "",
			Insecure:    true,
			Headers:     make(map[string]string),
			Timeout:     10 * time.Second,
			Compression: "none",
		},
		Sampling: SamplingConfig{
			Type:  "parent_based",
			Ratio: 1.0,
		},
		ResourceAttributes: make(map[string]string),
	}
}

// NewEnvironmentConfig returns a tracing configuration for the given environment
func NewEnvironmentConfig(environment string) Config {
	config := NewDefaultConfig()
	config.Environment = environment

	switch strings.ToLower(environment) {
	case "development", "dev":
		config.Enabled = true
		config.Exporter.Type = "console"
		config.Sampling.Type = "always"
		config.Sampling.Ratio = 1.0
	case "staging":
		config.Enabled = true
		config.Exporter.Type = "otlp"
		config.Exporter.Endpoint = "http://localhost:4318/v1/traces"
		config.Sampling.Type = "trace_id_ratio"
		config.Sampling.Ratio = 0.5
	case "production", "prod":
		config.Enabled = true
		config.Exporter.Type = "otlp"
		config.Exporter.Endpoint = "http://localhost:4318/v1/traces"
		config.Exporter.Insecure = false
		config.Sampling.Type = "trace_id_ratio"
		config.Sampling.Ratio = 0.1
	default:
		// Default to development settings for unknown environments
		config.Enabled = false
	}

	return config
}

// Validate validates the tracing configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if tracing is disabled
	}

	// Validate service name
	if c.ServiceName == "" {
		return fmt.Errorf("service_name is required when tracing is enabled")
	}

	// Validate exporter
	if err := c.Exporter.Validate(); err != nil {
		return fmt.Errorf("invalid exporter config: %w", err)
	}

	// Validate sampling
	if err := c.Sampling.Validate(); err != nil {
		return fmt.Errorf("invalid sampling config: %w", err)
	}

	return nil
}

// IsEnabled returns true if tracing is enabled
func (c *Config) IsEnabled() bool {
	return c.Enabled
}

// GetServiceName returns the service name, with fallback
func (c *Config) GetServiceName() string {
	if c.ServiceName != "" {
		return c.ServiceName
	}
	return "k8s-operator-cloudflare"
}

// GetServiceVersion returns the service version, with fallback
func (c *Config) GetServiceVersion() string {
	if c.ServiceVersion != "" {
		return c.ServiceVersion
	}
	return "unknown"
}

// String returns a string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf("enabled=%t, service=%s, version=%s, environment=%s, exporter=%s",
		c.Enabled, c.GetServiceName(), c.GetServiceVersion(), c.Environment, c.Exporter.Type)
}

// Validate validates the exporter configuration
func (e *ExporterConfig) Validate() error {
	// Validate exporter type
	validTypes := []string{"otlp", "jaeger", "console", "none"}
	valid := false
	for _, t := range validTypes {
		if e.Type == t {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid exporter type: %s, must be one of %v", e.Type, validTypes)
	}

	// Validate endpoint for OTLP and Jaeger
	if (e.Type == "otlp" || e.Type == "jaeger") && e.Endpoint == "" {
		return fmt.Errorf("endpoint is required for %s exporter", e.Type)
	}

	// Validate timeout
	if e.Timeout <= 0 {
		e.Timeout = 10 * time.Second // Set default
	}

	// Validate compression
	if e.Compression != "" && e.Compression != "gzip" && e.Compression != "none" {
		return fmt.Errorf("invalid compression: %s, must be 'gzip' or 'none'", e.Compression)
	}

	return nil
}

// Validate validates the sampling configuration
func (s *SamplingConfig) Validate() error {
	// Validate sampling type
	validTypes := []string{"always", "never", "trace_id_ratio", "parent_based"}
	valid := false
	for _, t := range validTypes {
		if s.Type == t {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid sampling type: %s, must be one of %v", s.Type, validTypes)
	}

	// Validate ratio for trace_id_ratio sampling
	if s.Type == "trace_id_ratio" {
		if s.Ratio < 0.0 || s.Ratio > 1.0 {
			return fmt.Errorf("sampling ratio must be between 0.0 and 1.0, got %f", s.Ratio)
		}
	}

	return nil
}
