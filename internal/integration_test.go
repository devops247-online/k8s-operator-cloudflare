package internal

import (
	"context"
	"testing"

	"github.com/devops247-online/k8s-operator-cloudflare/internal/logging"
	"github.com/devops247-online/k8s-operator-cloudflare/internal/tracing"
)

// TestLoggingTracingIntegration tests that logging and tracing work together correctly
func TestLoggingTracingIntegration(t *testing.T) {
	// Setup tracing first
	tracingConfig := tracing.NewEnvironmentConfig("development")
	tracingConfig.Enabled = true
	tracingProvider, err := tracing.SetupGlobalTracer(tracingConfig)
	if err != nil {
		t.Fatalf("Failed to setup tracing: %v", err)
	}
	defer func() {
		if err := tracingProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown tracing: %v", err)
		}
	}()

	// Setup logging
	loggingConfig := logging.NewEnvironmentConfig("development")
	err = logging.SetupGlobalLogger(loggingConfig)
	if err != nil {
		t.Fatalf("Failed to setup logging: %v", err)
	}

	// Create context with tracing span
	ctx, span := tracing.StartSpan(context.Background(), "integration-test")
	defer span.End()

	// Verify trace IDs can be extracted
	traceID := tracing.GetTraceID(ctx)
	spanID := tracing.GetSpanID(ctx)

	if traceID == "" {
		t.Error("Expected non-empty trace ID")
	}
	if spanID == "" {
		t.Error("Expected non-empty span ID")
	}

	// Create request context for logging
	reqCtx := logging.RequestContext{
		RequestID: logging.GenerateRequestID(),
		Operation: "integration-test",
		Resource:  "test-resource",
		Namespace: "test-namespace",
		TraceID:   traceID,
		SpanID:    spanID,
	}

	// Add request context to context
	ctx = logging.ContextWithRequestInfo(ctx, reqCtx)

	// Create logger from config and enrich with context
	logger, err := logging.LoggerFromConfig("development")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	enrichedLogger := logging.EnrichLogger(ctx, logger)

	// Test that enriched logger includes trace information
	enrichedLogger.Info("Test integration message")

	// Test logr integration
	logrLogger, err := logging.LogrFromConfig("development")
	if err != nil {
		t.Fatalf("Failed to create logr logger: %v", err)
	}

	enrichedLogrLogger := logging.EnrichLogr(ctx, logrLogger)
	enrichedLogrLogger.Info("Test logr integration message", "test.field", "test-value")

	// Verify request context can be retrieved
	retrievedReqCtx, ok := logging.RequestInfoFromContext(ctx)
	if !ok {
		t.Error("Expected to retrieve request context from context")
	}
	if retrievedReqCtx.TraceID != traceID {
		t.Errorf("Expected trace ID %s, got %s", traceID, retrievedReqCtx.TraceID)
	}
	if retrievedReqCtx.SpanID != spanID {
		t.Errorf("Expected span ID %s, got %s", spanID, retrievedReqCtx.SpanID)
	}

	// Add attributes to span to test tracing functionality
	span.SetAttributes(
		tracing.StringAttribute("integration.test", "success"),
		tracing.StringAttribute("trace.id", traceID),
		tracing.StringAttribute("span.id", spanID),
	)
}

// TestLoggingTracingMiddleware tests the middleware integration
func TestLoggingTracingMiddleware(t *testing.T) {
	// Setup minimal logging and tracing
	loggingConfig := logging.NewEnvironmentConfig("development")
	err := logging.SetupGlobalLogger(loggingConfig)
	if err != nil {
		t.Fatalf("Failed to setup logging: %v", err)
	}

	tracingConfig := tracing.NewEnvironmentConfig("development")
	tracingConfig.Enabled = true
	tracingProvider, err := tracing.SetupGlobalTracer(tracingConfig)
	if err != nil {
		t.Fatalf("Failed to setup tracing: %v", err)
	}
	defer func() {
		if err := tracingProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown tracing: %v", err)
		}
	}()

	// Test context enrichment workflow
	ctx := context.Background()

	// Start tracing span
	ctx, span := tracing.StartSpan(ctx, "middleware-test")
	defer span.End()

	// Create request context
	reqCtx := logging.RequestContext{
		RequestID: logging.GenerateRequestID(),
		Operation: "middleware-test",
		Resource:  "test-resource",
		Namespace: "test-namespace",
		TraceID:   tracing.GetTraceID(ctx),
		SpanID:    tracing.GetSpanID(ctx),
	}

	// Enrich context
	ctx = logging.ContextWithRequestInfo(ctx, reqCtx)

	// Test that logr can be retrieved and used
	logrLogger := logging.LogrFromContext(ctx)

	// Test enrichment
	enrichedLogr := logging.EnrichLogr(ctx, logrLogger)
	enrichedLogr.Info("Middleware integration test message")

	// Verify all components work together
	if reqCtx.TraceID == "" {
		t.Error("Expected non-empty trace ID in request context")
	}
	if reqCtx.SpanID == "" {
		t.Error("Expected non-empty span ID in request context")
	}
}
