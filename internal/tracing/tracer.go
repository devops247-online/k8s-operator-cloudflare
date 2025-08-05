package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Provider wraps the OpenTelemetry TracerProvider
type Provider struct {
	provider *trace.TracerProvider
	config   Config
}

// NewProvider creates a new tracing provider with the given configuration
func NewProvider(config Config) (*Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tracing config: %w", err)
	}

	if !config.IsEnabled() {
		// Return a no-op provider
		return &Provider{
			provider: trace.NewTracerProvider(),
			config:   config,
		}, nil
	}

	// Create resource with service information
	res, err := createResource(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	exporter, err := createExporter(config.Exporter)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create sampler
	sampler, err := createSampler(config.Sampling)
	if err != nil {
		return nil, fmt.Errorf("failed to create sampler: %w", err)
	}

	// Create tracer provider
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(sampler),
	)

	return &Provider{
		provider: tracerProvider,
		config:   config,
	}, nil
}

// Tracer returns a tracer for the given name
func (p *Provider) Tracer(name string, opts ...oteltrace.TracerOption) oteltrace.Tracer {
	return p.provider.Tracer(name, opts...)
}

// Shutdown shuts down the tracer provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.provider == nil {
		return nil
	}
	return p.provider.Shutdown(ctx)
}

// IsEnabled returns true if tracing is enabled
func (p *Provider) IsEnabled() bool {
	return p.config.IsEnabled()
}

// Config returns the tracing configuration
func (p *Provider) Config() Config {
	return p.config
}

// SetupGlobalTracer sets up the global tracer provider
func SetupGlobalTracer(config Config) (*Provider, error) {
	provider, err := NewProvider(config)
	if err != nil {
		return nil, err
	}

	// Set global tracer provider
	otel.SetTracerProvider(provider.provider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// TracerFromConfig creates a tracer from environment variables and config
func TracerFromConfig(environment string) (*Provider, error) {
	config := NewEnvironmentConfig(environment)

	// Override with environment variables if present
	if enabled := os.Getenv("TRACING_ENABLED"); enabled != "" {
		config.Enabled = enabled == "true"
	}
	if serviceName := os.Getenv("TRACING_SERVICE_NAME"); serviceName != "" {
		config.ServiceName = serviceName
	}
	if serviceVersion := os.Getenv("TRACING_SERVICE_VERSION"); serviceVersion != "" {
		config.ServiceVersion = serviceVersion
	}
	if exporterType := os.Getenv("TRACING_EXPORTER_TYPE"); exporterType != "" {
		config.Exporter.Type = exporterType
	}
	if endpoint := os.Getenv("TRACING_EXPORTER_ENDPOINT"); endpoint != "" {
		config.Exporter.Endpoint = endpoint
	}
	if samplingType := os.Getenv("TRACING_SAMPLING_TYPE"); samplingType != "" {
		config.Sampling.Type = samplingType
	}

	return NewProvider(config)
}

// createResource creates OpenTelemetry resource with service information
func createResource(config Config) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(config.GetServiceName()),
			semconv.ServiceVersion(config.GetServiceVersion()),
			semconv.DeploymentEnvironment(config.Environment),
		),
	}

	// Add custom resource attributes
	if len(config.ResourceAttributes) > 0 {
		customAttrs := make([]attribute.KeyValue, 0, len(config.ResourceAttributes))
		for key, value := range config.ResourceAttributes {
			customAttrs = append(customAttrs, attribute.String(key, value))
		}
		attrs = append(attrs, resource.WithAttributes(customAttrs...))
	}

	return resource.New(context.Background(), attrs...)
}

// createExporter creates the appropriate trace exporter
func createExporter(config ExporterConfig) (trace.SpanExporter, error) {
	switch config.Type {
	case "otlp":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(config.Endpoint),
			otlptracehttp.WithTimeout(config.Timeout),
		}

		if config.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		if len(config.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(config.Headers))
		}

		if config.Compression == "gzip" {
			opts = append(opts, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
		}

		return otlptracehttp.New(context.Background(), opts...)

	case "console":
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)

	case "none":
		// Return a no-op exporter
		return &noopExporter{}, nil

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.Type)
	}
}

// createSampler creates the appropriate trace sampler
func createSampler(config SamplingConfig) (trace.Sampler, error) {
	switch config.Type {
	case "always":
		return trace.AlwaysSample(), nil
	case "never":
		return trace.NeverSample(), nil
	case "trace_id_ratio":
		return trace.TraceIDRatioBased(config.Ratio), nil
	case "parent_based":
		return trace.ParentBased(trace.TraceIDRatioBased(config.Ratio)), nil
	default:
		return nil, fmt.Errorf("unsupported sampling type: %s", config.Type)
	}
}

// noopExporter is a no-op span exporter
type noopExporter struct{}

func (e *noopExporter) ExportSpans(_ context.Context, _ []trace.ReadOnlySpan) error {
	return nil
}

func (e *noopExporter) Shutdown(_ context.Context) error {
	return nil
}

// StartSpan starts a new span with the given name and options
func StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	tracer := otel.Tracer("k8s-operator-cloudflare")
	return tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) oteltrace.Span {
	return oteltrace.SpanFromContext(ctx)
}

// ContextWithSpan returns a new context with the given span
func ContextWithSpan(ctx context.Context, span oteltrace.Span) context.Context {
	return oteltrace.ContextWithSpan(ctx, span)
}

// GetTraceID returns the trace ID from the current span in context
func GetTraceID(ctx context.Context) string {
	span := SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID returns the span ID from the current span in context
func GetSpanID(ctx context.Context) string {
	span := SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}

// StringAttribute creates a string attribute for tracing
func StringAttribute(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}

// IntAttribute creates an int attribute for tracing
func IntAttribute(key string, value int) attribute.KeyValue {
	return attribute.Int(key, value)
}

// BoolAttribute creates a bool attribute for tracing
func BoolAttribute(key string, value bool) attribute.KeyValue {
	return attribute.Bool(key, value)
}
