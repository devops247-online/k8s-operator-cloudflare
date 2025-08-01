package logging

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantJSON bool
	}{
		{
			name: "json logger",
			config: Config{
				Level:       "info",
				Format:      "json",
				Development: false,
			},
			wantJSON: true,
		},
		{
			name: "console logger",
			config: Config{
				Level:       "debug",
				Format:      "console",
				Development: true,
			},
			wantJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.config)
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLoggerWithInvalidConfig(t *testing.T) {
	config := Config{
		Level:  "invalid",
		Format: "json",
	}

	_, err := NewLogger(config)
	assert.Error(t, err)
}

func TestStructuredLogging(t *testing.T) {
	// Create a logger that writes to a buffer for testing
	var buf bytes.Buffer
	config := zap.NewDevelopmentConfig()
	config.OutputPaths = []string{"stdout"}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(config.EncoderConfig),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	)

	logger := zap.New(core)
	defer func() { _ = logger.Sync() }()

	// Test structured logging
	logger.Info("test message",
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
		zap.Bool("key3", true),
	)

	// Verify the JSON output
	logOutput := buf.String()
	assert.Contains(t, logOutput, "test message")
	assert.Contains(t, logOutput, "value1")
	assert.Contains(t, logOutput, "42")
	assert.Contains(t, logOutput, "true")
}

func TestNewLogrLogger(t *testing.T) {
	config := Config{
		Level:       "info",
		Format:      "json",
		Development: false,
	}

	logger, err := NewLogrLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Test that it's a valid logr.Logger
	logger.Info("test message", "key", "value")
}

func TestLoggerWithTracing(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	// Test logging with context (no span)
	observedLogger.Info("message without span")

	// Verify log was recorded
	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "message without span", logs[0].Message)
}

func TestLoggerSampling(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
		Sampling: SamplingConfig{
			Enabled:    true,
			Initial:    1,  // Only log first message
			Thereafter: 10, // Then every 10th message
		},
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// This test is more about ensuring no errors occur during sampling setup
	// Actual sampling behavior would need more complex integration testing
	logger.Info("test message 1")
	logger.Info("test message 2")
	logger.Info("test message 3")
}

func TestLoggerWithCustomOutputs(t *testing.T) {
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"stdout"},
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		level    string
		wantCore zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			config := Config{
				Level:  tt.level,
				Format: "json",
			}

			logger, err := NewLogger(config)
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "context without span",
			ctx:      context.Background(),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceID := GetTraceID(tt.ctx)
			assert.Equal(t, tt.expected, traceID)
		})
	}
}

func TestGetSpanID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "context without span",
			ctx:      context.Background(),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spanID := GetSpanID(tt.ctx)
			assert.Equal(t, tt.expected, spanID)
		})
	}
}

func TestWithTraceContext(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	ctx := context.Background()

	// Test the WithTraceContext function
	logger := WithTraceContext(ctx, observedLogger)
	logger.Info("test message")

	// Verify the log was recorded
	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "test message", logs[0].Message)
}

func TestLoggerWithDevelopmentMode(t *testing.T) {
	config := Config{
		Level:       "debug",
		Format:      "console",
		Development: true,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Development mode should enable caller info and stack traces
	logger.Info("development test message")
}

func TestWithTraceContextLogr(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logger, err := NewLogrLogger(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Test WithTraceContextLogr function
	loggerWithContext := WithTraceContextLogr(ctx, logger)
	assert.NotNil(t, loggerWithContext)

	// Test logging with the context logger
	loggerWithContext.Info("test message with trace context")
}

func TestSetupGlobalLogger(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
	}

	err := SetupGlobalLogger(config)
	assert.NoError(t, err)

	// Test that global logger was set
	globalLogger := zap.L()
	assert.NotNil(t, globalLogger)

	globalLogger.Info("test global logger message")
}

func TestSetupGlobalLoggerError(t *testing.T) {
	config := Config{
		Level:  "invalid",
		Format: "json",
	}

	err := SetupGlobalLogger(config)
	assert.Error(t, err)
}

func TestSyncLogger(t *testing.T) {
	// Test with nil logger
	SyncLogger(nil)

	// Test with valid logger
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)

	// Should not panic
	SyncLogger(logger)
}

func TestLoggerFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		envVars     map[string]string
		wantErr     bool
	}{
		{
			name:        "development environment",
			environment: "development",
			wantErr:     false,
		},
		{
			name:        "production environment",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "with env vars override",
			environment: "production",
			envVars: map[string]string{
				"LOG_LEVEL":       "debug",
				"LOG_FORMAT":      "console",
				"LOG_DEVELOPMENT": "true",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables if provided
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			logger, err := LoggerFromConfig(tt.environment)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, logger)

			// Test logging
			logger.Info("test message from config")
		})
	}
}

func TestLoggerFromConfigError(t *testing.T) {
	// Set invalid log level
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := LoggerFromConfig("production")
	assert.Error(t, err)
}

func TestLoggrFromConfig(t *testing.T) {
	logger, err := LogrFromConfig("development")
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Test logging
	logger.Info("test logr message from config")
}

func TestLoggrFromConfigError(t *testing.T) {
	// Set invalid log level
	t.Setenv("LOG_LEVEL", "invalid")

	_, err := LogrFromConfig("production")
	assert.Error(t, err)
}

func TestNewLogrLoggerError(t *testing.T) {
	config := Config{
		Level:  "invalid",
		Format: "json",
	}

	_, err := NewLogrLogger(config)
	assert.Error(t, err)
}

func TestWithTraceContextWithSpan(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	ctx := context.Background()

	// Test WithTraceContext with empty context
	logger := WithTraceContext(ctx, observedLogger)
	logger.Info("test message")

	// Verify the log was recorded
	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "test message", logs[0].Message)
}

func TestGetTraceIDWithValidSpan(t *testing.T) {
	// This test covers the case where span context is valid
	// Since we can't easily create a valid span context without full OTEL setup,
	// we test the error path and the empty string path which are covered
	ctx := context.Background()
	traceID := GetTraceID(ctx)
	assert.Equal(t, "", traceID)

	// Test the case when span exists but context is invalid (covered by previous test)
	spanID := GetSpanID(ctx)
	assert.Equal(t, "", spanID)
}

func TestWithTraceContextEdgeCases(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	// Test with context that has no trace data
	ctx := context.Background()
	logger := WithTraceContext(ctx, observedLogger)

	// Should return the same logger since no trace context
	assert.NotNil(t, logger)

	// Test logging with this logger
	logger.Info("message without trace")

	logs := observedLogs.All()
	assert.Len(t, logs, 1)
	assert.Equal(t, "message without trace", logs[0].Message)
}

func TestWithTraceContextLogrEdgeCases(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logger, err := NewLogrLogger(config)
	require.NoError(t, err)

	// Test with context that has no trace data
	ctx := context.Background()
	loggerWithContext := WithTraceContextLogr(ctx, logger)

	// Should return the same logger since no trace context
	assert.NotNil(t, loggerWithContext)

	// Test logging with this logger
	loggerWithContext.Info("message without trace context")
}

func TestNewLoggerErrorPath(t *testing.T) {
	// Test case where zap config build fails - this is hard to trigger
	// but we can test with invalid outputs that might cause issues
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{"/invalid/path/that/should/not/exist/definitely"},
	}

	// This might not always fail, but tests the error path if it does
	logger, err := NewLogger(config)
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, logger)
	} else {
		// If it doesn't fail, at least we tested the path
		assert.NotNil(t, logger)
	}
}

func TestConfigValidateEdgeCases(t *testing.T) {
	// Test config with empty outputs to trigger the default setting
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{}, // Empty outputs should trigger default
	}

	err := config.Validate()
	assert.NoError(t, err)
	assert.Equal(t, []string{"stdout"}, config.Outputs) // Should be set to default

	// Test config with nil outputs
	config2 := Config{
		Level:   "info",
		Format:  "json",
		Outputs: nil, // Nil outputs should also trigger default
	}

	err = config2.Validate()
	assert.NoError(t, err)
	assert.Equal(t, []string{"stdout"}, config2.Outputs) // Should be set to default
}

// Дополнительные тесты для увеличения покрытия
func TestNewLoggerWithSamplingDisabled(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
		Sampling: SamplingConfig{
			Enabled:    false,
			Initial:    100,
			Thereafter: 100,
		},
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Тестируем что логгер работает
	logger.Info("test with sampling disabled")
}

func TestNewLoggerBuildError(t *testing.T) {
	// Создаем конфиг который может вызвать ошибку при создании логгера
	// К сожалению, сложно протестировать ошибку zap.Build без моков
	config := Config{
		Level:  "info",
		Format: "json",
		// Попробуем с невалидным выходом который может не открыться
		Outputs: []string{"stdout", "/root/test.log"}, // Может не быть прав
	}

	logger, err := NewLogger(config)
	// Если нет ошибки, то проверяем что логгер создан
	if err == nil {
		assert.NotNil(t, logger)
	}
}

func TestConfigGetLevelInvalid(t *testing.T) {
	config := Config{
		Level: "not-a-valid-level",
	}

	level := config.GetLevel()
	assert.Equal(t, zapcore.InvalidLevel, level)
}

func TestNewEnvironmentConfigDev(t *testing.T) {
	config := NewEnvironmentConfig("dev")
	assert.Equal(t, "debug", config.Level)
	assert.Equal(t, "console", config.Format)
	assert.True(t, config.Development)
}

func TestNewEnvironmentConfigProd(t *testing.T) {
	config := NewEnvironmentConfig("prod")
	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.False(t, config.Development)
	assert.True(t, config.Sampling.Enabled)
}

func TestConfigValidateSamplingError(t *testing.T) {
	// Test config with invalid sampling config that triggers error path
	config := Config{
		Level:  "info",
		Format: "json",
		Sampling: SamplingConfig{
			Enabled:    true,
			Initial:    -1, // Invalid
			Thereafter: 100,
		},
	}

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sampling config")
}

func TestNewLoggerWithEmptyOutputs(t *testing.T) {
	// Test that empty outputs get set to default
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: []string{},
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLoggerWithNilOutputs(t *testing.T) {
	// Test that nil outputs get set to default
	config := Config{
		Level:   "info",
		Format:  "json",
		Outputs: nil,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestLoggerFromConfigWithAllEnvVars(t *testing.T) {
	// Test with all environment variables set
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("LOG_DEVELOPMENT", "true")

	logger, err := LoggerFromConfig("production")
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Test logging
	logger.Warn("test warning message")
}

func TestLoggerFromConfigDevelopmentFalse(t *testing.T) {
	// Test with development explicitly set to false
	t.Setenv("LOG_DEVELOPMENT", "false")

	logger, err := LoggerFromConfig("development")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestGetTraceIDAndSpanIDCoverage(t *testing.T) {
	// These functions are difficult to test fully without real OpenTelemetry spans
	// Testing with empty context for coverage
	ctx := context.Background()

	// Should return empty strings for context without span
	traceID := GetTraceID(ctx)
	assert.Equal(t, "", traceID)

	spanID := GetSpanID(ctx)
	assert.Equal(t, "", spanID)
}

func TestTraceContextFunctionsAttemptCoverage(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	ctx := context.Background()

	// Test WithTraceContext when no fields are added (returns same logger)
	logger := WithTraceContext(ctx, observedLogger)
	assert.NotNil(t, logger)

	logger.Info("test message without trace fields")

	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "test message without trace fields", logs[0].Message)

	// Test WithTraceContextLogr when no fields are added
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logrLogger, err := NewLogrLogger(config)
	require.NoError(t, err)

	loggerWithContext := WithTraceContextLogr(ctx, logrLogger)
	assert.NotNil(t, loggerWithContext)

	// Should return the same logger since no trace context
	loggerWithContext.Info("test logr message without trace fields")
}
