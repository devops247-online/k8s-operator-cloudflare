package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid disabled config",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid enabled config with console exporter",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type:    "console",
					Timeout: 10 * time.Second,
				},
				Sampling: SamplingConfig{
					Type: "always",
				},
			},
			wantErr: false,
		},
		{
			name: "valid enabled config with none exporter",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type:    "none",
					Timeout: 10 * time.Second,
				},
				Sampling: SamplingConfig{
					Type: "never",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config",
			config: Config{
				Enabled:     true,
				ServiceName: "", // Missing service name
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.config.Enabled, provider.IsEnabled())

				// Test shutdown
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err = provider.Shutdown(ctx)
				assert.NoError(t, err)
			}
		})
	}
}

func TestProviderTracer(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := NewProvider(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("test-tracer")
	assert.NotNil(t, tracer)

	// Test creating a span
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestProviderConfig(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := NewProvider(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	retrievedConfig := provider.Config()
	assert.Equal(t, config.Enabled, retrievedConfig.Enabled)
	assert.Equal(t, config.ServiceName, retrievedConfig.ServiceName)
}

func TestSetupGlobalTracer(t *testing.T) {
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := SetupGlobalTracer(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Test that global tracer is set
	globalTracer := otel.Tracer("test")
	assert.NotNil(t, globalTracer)

	// Test span creation with global tracer
	ctx, span := globalTracer.Start(context.Background(), "global-test-span")
	assert.NotNil(t, span)
	span.End()
	_ = ctx
}

func TestTracerFromConfig(t *testing.T) {
	// Test with environment variables
	t.Setenv("TRACING_ENABLED", "true")
	t.Setenv("TRACING_SERVICE_NAME", "env-service")
	t.Setenv("TRACING_SERVICE_VERSION", "2.0.0")
	t.Setenv("TRACING_EXPORTER_TYPE", "none")
	t.Setenv("TRACING_SAMPLING_TYPE", "never")

	provider, err := TracerFromConfig("production")
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	config := provider.Config()
	assert.True(t, config.Enabled)
	assert.Equal(t, "env-service", config.ServiceName)
	assert.Equal(t, "2.0.0", config.ServiceVersion)
	assert.Equal(t, "none", config.Exporter.Type)
	assert.Equal(t, "never", config.Sampling.Type)
}

func TestTracerFromConfigDisabled(t *testing.T) {
	t.Setenv("TRACING_ENABLED", "false")

	provider, err := TracerFromConfig("development")
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	assert.False(t, provider.IsEnabled())
}

func TestCreateResource(t *testing.T) {
	config := Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		ResourceAttributes: map[string]string{
			"custom.key": "custom.value",
		},
	}

	resource, err := createResource(config)
	require.NoError(t, err)
	assert.NotNil(t, resource)

	// Check that resource has expected attributes
	attrs := resource.Attributes()
	foundService := false
	foundVersion := false
	foundEnv := false
	foundCustom := false

	for _, attr := range attrs {
		switch attr.Key {
		case "service.name":
			foundService = true
			assert.Equal(t, "test-service", attr.Value.AsString())
		case "service.version":
			foundVersion = true
			assert.Equal(t, "1.0.0", attr.Value.AsString())
		case "deployment.environment":
			foundEnv = true
			assert.Equal(t, "test", attr.Value.AsString())
		case "custom.key":
			foundCustom = true
			assert.Equal(t, "custom.value", attr.Value.AsString())
		}
	}

	assert.True(t, foundService, "service.name attribute not found")
	assert.True(t, foundVersion, "service.version attribute not found")
	assert.True(t, foundEnv, "deployment.environment attribute not found")
	assert.True(t, foundCustom, "custom.key attribute not found")
}

func TestCreateResourceWithoutCustomAttrs(t *testing.T) {
	config := Config{
		ServiceName:        "test-service",
		ServiceVersion:     "1.0.0",
		Environment:        "test",
		ResourceAttributes: nil, // No custom attributes
	}

	resource, err := createResource(config)
	require.NoError(t, err)
	assert.NotNil(t, resource)
}

func TestCreateExporter(t *testing.T) {
	tests := []struct {
		name     string
		config   ExporterConfig
		wantErr  bool
		wantType string
	}{
		{
			name: "console exporter",
			config: ExporterConfig{
				Type:    "console",
				Timeout: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "none exporter",
			config: ExporterConfig{
				Type:    "none",
				Timeout: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "otlp exporter",
			config: ExporterConfig{
				Type:     "otlp",
				Endpoint: "http://localhost:4318/v1/traces",
				Timeout:  10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "otlp exporter with headers and compression",
			config: ExporterConfig{
				Type:     "otlp",
				Endpoint: "http://localhost:4318/v1/traces",
				Headers: map[string]string{
					"Authorization": "Bearer token",
				},
				Compression: "gzip",
				Timeout:     10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "unsupported exporter",
			config: ExporterConfig{
				Type: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := createExporter(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, exporter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exporter)

				// Test shutdown
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err = exporter.Shutdown(ctx)
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateSampler(t *testing.T) {
	tests := []struct {
		name    string
		config  SamplingConfig
		wantErr bool
	}{
		{
			name: "always sampler",
			config: SamplingConfig{
				Type: "always",
			},
			wantErr: false,
		},
		{
			name: "never sampler",
			config: SamplingConfig{
				Type: "never",
			},
			wantErr: false,
		},
		{
			name: "trace_id_ratio sampler",
			config: SamplingConfig{
				Type:  "trace_id_ratio",
				Ratio: 0.5,
			},
			wantErr: false,
		},
		{
			name: "parent_based sampler",
			config: SamplingConfig{
				Type:  "parent_based",
				Ratio: 0.8,
			},
			wantErr: false,
		},
		{
			name: "unsupported sampler",
			config: SamplingConfig{
				Type: "unsupported",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler, err := createSampler(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, sampler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, sampler)
			}
		})
	}
}

func TestNoopExporter(t *testing.T) {
	exporter := &noopExporter{}

	// Test ExportSpans
	err := exporter.ExportSpans(context.Background(), nil)
	assert.NoError(t, err)

	// Test Shutdown
	err = exporter.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestStartSpan(t *testing.T) {
	// Setup global tracer first
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := SetupGlobalTracer(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Test StartSpan function
	ctx, span := StartSpan(context.Background(), "test-span")
	assert.NotNil(t, span)

	// Verify span is in context
	spanFromCtx := SpanFromContext(ctx)
	assert.Equal(t, span, spanFromCtx)

	span.End()
}

func TestSpanFromContext(t *testing.T) {
	// Test with context without span
	span := SpanFromContext(context.Background())
	assert.NotNil(t, span)
	assert.False(t, span.SpanContext().IsValid())
}

func TestContextWithSpan(t *testing.T) {
	// Setup global tracer first
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := SetupGlobalTracer(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create a span
	_, span := StartSpan(context.Background(), "test-span")
	defer span.End()

	// Test ContextWithSpan
	ctx := ContextWithSpan(context.Background(), span)
	spanFromCtx := SpanFromContext(ctx)
	assert.Equal(t, span, spanFromCtx)
}

func TestGetTraceIDAndSpanID(t *testing.T) {
	// Test with context without span
	ctx := context.Background()
	traceID := GetTraceID(ctx)
	assert.Equal(t, "", traceID)

	spanID := GetSpanID(ctx)
	assert.Equal(t, "", spanID)

	// Setup global tracer for testing with real spans
	config := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Exporter: ExporterConfig{
			Type:    "none",
			Timeout: 10 * time.Second,
		},
		Sampling: SamplingConfig{
			Type: "always",
		},
	}

	provider, err := SetupGlobalTracer(config)
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create a span and test with valid context
	ctx, span := StartSpan(context.Background(), "test-span")
	defer span.End()

	traceID = GetTraceID(ctx)
	spanID = GetSpanID(ctx)

	if span.SpanContext().IsValid() {
		assert.NotEqual(t, "", traceID)
		assert.NotEqual(t, "", spanID)
		assert.Equal(t, span.SpanContext().TraceID().String(), traceID)
		assert.Equal(t, span.SpanContext().SpanID().String(), spanID)
	}
}

func TestProviderShutdownNilProvider(t *testing.T) {
	provider := &Provider{provider: nil}
	err := provider.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTracerFromConfigWithEndpoint(t *testing.T) {
	t.Setenv("TRACING_ENABLED", "true")
	t.Setenv("TRACING_EXPORTER_ENDPOINT", "http://localhost:4318/v1/traces")

	provider, err := TracerFromConfig("staging")
	require.NoError(t, err)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	config := provider.Config()
	assert.Equal(t, "http://localhost:4318/v1/traces", config.Exporter.Endpoint)
}
