package logging

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new zap logger with the given configuration
func NewLogger(config Config) (*zap.Logger, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logging config: %w", err)
	}

	// Create zap config based on our config
	var zapConfig zap.Config
	if config.Development {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// Set log level
	zapConfig.Level = zap.NewAtomicLevelAt(config.GetLevel())

	// Set encoding format
	if config.IsJSON() {
		zapConfig.Encoding = "json"
		// Add trace context fields to JSON encoder
		zapConfig.EncoderConfig.TimeKey = "timestamp"
		zapConfig.EncoderConfig.LevelKey = "level"
		zapConfig.EncoderConfig.NameKey = "logger"
		zapConfig.EncoderConfig.CallerKey = "caller"
		zapConfig.EncoderConfig.MessageKey = "message"
		zapConfig.EncoderConfig.StacktraceKey = "stacktrace"
	} else {
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Set output paths
	zapConfig.OutputPaths = config.Outputs
	zapConfig.ErrorOutputPaths = config.Outputs

	// Configure sampling if enabled
	if config.Sampling.Enabled {
		zapConfig.Sampling = &zap.SamplingConfig{
			Initial:    config.Sampling.Initial,
			Thereafter: config.Sampling.Thereafter,
		}
	}

	// Build the logger
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// NewLogrLogger creates a new logr.Logger with the given configuration
func NewLogrLogger(config Config) (logr.Logger, error) {
	zapLogger, err := NewLogger(config)
	if err != nil {
		return logr.Logger{}, err
	}

	// Convert zap logger to logr.Logger
	return zapr.NewLogger(zapLogger), nil
}

// GetTraceID extracts the trace ID from the context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID extracts the span ID from the context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}

// WithTraceContext adds trace context fields to the logger
func WithTraceContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	traceID := GetTraceID(ctx)
	spanID := GetSpanID(ctx)

	fields := make([]zap.Field, 0, 2)
	if traceID != "" {
		fields = append(fields, zap.String("trace.id", traceID))
	}
	if spanID != "" {
		fields = append(fields, zap.String("span.id", spanID))
	}

	if len(fields) > 0 {
		return logger.With(fields...)
	}

	return logger
}

// WithTraceContextLogr adds trace context fields to the logr.Logger
func WithTraceContextLogr(ctx context.Context, logger logr.Logger) logr.Logger {
	traceID := GetTraceID(ctx)
	spanID := GetSpanID(ctx)

	values := make([]interface{}, 0, 4)
	if traceID != "" {
		values = append(values, "trace.id", traceID)
	}
	if spanID != "" {
		values = append(values, "span.id", spanID)
	}

	if len(values) > 0 {
		return logger.WithValues(values...)
	}

	return logger
}

// SetupGlobalLogger sets up the global logger with the given configuration
func SetupGlobalLogger(config Config) error {
	logger, err := NewLogger(config)
	if err != nil {
		return err
	}

	// Replace global logger
	zap.ReplaceGlobals(logger)
	return nil
}

// SyncLogger safely syncs the logger, handling any errors
func SyncLogger(logger *zap.Logger) {
	if logger == nil {
		return
	}

	// Sync can fail on stdout/stderr, which is expected
	_ = logger.Sync()
}

// LoggerFromConfig creates a logger from environment variables and config
func LoggerFromConfig(environment string) (*zap.Logger, error) {
	config := NewEnvironmentConfig(environment)

	// Override with environment variables if present
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = format
	}
	if os.Getenv("LOG_DEVELOPMENT") == "true" {
		config.Development = true
	}

	return NewLogger(config)
}

// LogrFromConfig creates a logr.Logger from environment variables and config
func LogrFromConfig(environment string) (logr.Logger, error) {
	zapLogger, err := LoggerFromConfig(environment)
	if err != nil {
		return logr.Logger{}, err
	}
	return zapr.NewLogger(zapLogger), nil
}
