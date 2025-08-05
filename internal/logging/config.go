// Package logging provides structured logging functionality for the Cloudflare DNS Operator.
// It implements Zap-based JSON logging with configurable levels, sampling, and environment-specific overrides.
package logging

import (
	"fmt"
	"strings"

	"go.uber.org/zap/zapcore"
)

const (
	// FormatConsole is the console log format
	FormatConsole = "console"
	// FormatJSON is the JSON log format
	FormatJSON = "json"
)

// Config represents the logging configuration
type Config struct {
	// Level sets the logging level (debug, info, warn, error)
	Level string `yaml:"level" json:"level"`

	// Format sets the log format (json, console)
	Format string `yaml:"format" json:"format"`

	// Development enables development mode (caller info, stack traces)
	Development bool `yaml:"development" json:"development"`

	// Sampling configuration for log sampling
	Sampling SamplingConfig `yaml:"sampling" json:"sampling"`

	// Outputs specifies where logs should be written
	Outputs []string `yaml:"outputs" json:"outputs"`
}

// SamplingConfig represents log sampling configuration
type SamplingConfig struct {
	// Enabled turns sampling on or off
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Initial number of log entries to always log per second
	Initial int `yaml:"initial" json:"initial"`

	// Thereafter, every Mth log entry will be logged
	Thereafter int `yaml:"thereafter" json:"thereafter"`
}

// NewDefaultConfig returns a default logging configuration
func NewDefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      FormatJSON,
		Development: false,
		Sampling: SamplingConfig{
			Enabled:    false,
			Initial:    100,
			Thereafter: 100,
		},
		Outputs: []string{"stdout"},
	}
}

// NewEnvironmentConfig returns a logging configuration for the given environment
func NewEnvironmentConfig(environment string) Config {
	config := NewDefaultConfig()

	switch strings.ToLower(environment) {
	case "development", "dev":
		config.Level = "debug"
		config.Format = FormatConsole
		config.Development = true
	case "production", "prod":
		config.Level = "info"
		config.Format = FormatJSON
		config.Development = false
		config.Sampling.Enabled = true
	default:
		// Default to production settings for unknown environments
		config.Level = "info"
		config.Format = FormatJSON
		config.Development = false
	}

	return config
}

// Validate validates the logging configuration
func (c *Config) Validate() error {
	// Validate log level
	if c.GetLevel() == zapcore.InvalidLevel {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	// Validate format
	if c.Format != FormatJSON && c.Format != FormatConsole {
		return fmt.Errorf("invalid log format: %s, must be 'json' or 'console'", c.Format)
	}

	// Validate sampling config
	if err := c.Sampling.Validate(); err != nil {
		return fmt.Errorf("invalid sampling config: %w", err)
	}

	// Validate outputs
	if len(c.Outputs) == 0 {
		c.Outputs = []string{"stdout"}
	}

	return nil
}

// GetLevel returns the zapcore log level
func (c *Config) GetLevel() zapcore.Level {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(c.Level)); err != nil {
		return zapcore.InvalidLevel
	}
	return level
}

// IsJSON returns true if the format is JSON
func (c *Config) IsJSON() bool {
	return c.Format == FormatJSON
}

// String returns a string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf("level=%s, format=%s, development=%t, sampling=%t",
		c.Level, c.Format, c.Development, c.Sampling.Enabled)
}

// Validate validates the sampling configuration
func (s *SamplingConfig) Validate() error {
	if !s.Enabled {
		return nil
	}

	if s.Initial < 0 {
		return fmt.Errorf("sampling initial must be >= 0, got %d", s.Initial)
	}

	if s.Thereafter < 0 {
		return fmt.Errorf("sampling thereafter must be >= 0, got %d", s.Thereafter)
	}

	return nil
}
