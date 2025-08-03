package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestContextWithRequestInfo(t *testing.T) {
	ctx := context.Background()

	reqCtx := RequestContext{
		RequestID: "test-123",
		Operation: "create",
		Resource:  "test-resource",
		Namespace: "default",
		StartTime: time.Now(),
	}

	newCtx := ContextWithRequestInfo(ctx, reqCtx)

	retrieved, ok := RequestInfoFromContext(newCtx)
	require.True(t, ok)
	assert.Equal(t, reqCtx.RequestID, retrieved.RequestID)
	assert.Equal(t, reqCtx.Operation, retrieved.Operation)
	assert.Equal(t, reqCtx.Resource, retrieved.Resource)
	assert.Equal(t, reqCtx.Namespace, retrieved.Namespace)
}

func TestRequestInfoFromContextNotFound(t *testing.T) {
	ctx := context.Background()

	_, ok := RequestInfoFromContext(ctx)
	assert.False(t, ok)
}

func TestContextWithLogger(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	newCtx := ContextWithLogger(ctx, logger)

	retrieved, ok := LoggerFromContext(newCtx)
	require.True(t, ok)
	assert.Equal(t, logger, retrieved)
}

func TestLoggerFromContextNotFound(t *testing.T) {
	ctx := context.Background()

	_, ok := LoggerFromContext(ctx)
	assert.False(t, ok)
}

func TestContextWithLogr(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()

	newCtx := ContextWithLogr(ctx, logger)

	retrieved := LogrFromContext(newCtx)
	assert.NotNil(t, retrieved)
}

func TestLogrFromContextNotFound(t *testing.T) {
	ctx := context.Background()

	logger := LogrFromContext(ctx)
	assert.NotNil(t, logger) // Should return discard logger
}

func TestEnrichLogger(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	reqCtx := RequestContext{
		RequestID: "test-123",
		Operation: "update",
		Resource:  "test-resource",
		Namespace: "default",
		UserAgent: "test-agent",
		RemoteIP:  "127.0.0.1",
		StartTime: time.Now(),
	}

	ctx := ContextWithRequestInfo(context.Background(), reqCtx)
	enrichedLogger := EnrichLogger(ctx, logger)

	enrichedLogger.Info("test message")

	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "test message", logs[0].Message)

	// Check that context fields were added
	contextMap := make(map[string]interface{})
	for _, field := range logs[0].Context {
		contextMap[field.Key] = field.Interface
	}

	assert.Contains(t, contextMap, "request.id")
	assert.Contains(t, contextMap, "operation")
	assert.Contains(t, contextMap, "resource")
	assert.Contains(t, contextMap, "namespace")
}

func TestEnrichLoggerWithEmptyFields(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	// Request context with empty fields
	reqCtx := RequestContext{
		RequestID: "", // Empty
		Operation: "", // Empty
	}

	ctx := ContextWithRequestInfo(context.Background(), reqCtx)
	enrichedLogger := EnrichLogger(ctx, logger)

	enrichedLogger.Info("test message")

	logs := observedLogs.All()
	require.Len(t, logs, 1)

	// Should not add empty fields
	for _, field := range logs[0].Context {
		assert.NotEqual(t, "", field.String)
	}
}

func TestEnrichLoggerWithoutRequestContext(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	ctx := context.Background() // No request context
	enrichedLogger := EnrichLogger(ctx, logger)

	enrichedLogger.Info("test message")

	logs := observedLogs.All()
	require.Len(t, logs, 1)
	assert.Equal(t, "test message", logs[0].Message)
}

func TestEnrichLogr(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logger, err := NewLogrLogger(config)
	require.NoError(t, err)

	reqCtx := RequestContext{
		RequestID: "test-456",
		Operation: "delete",
		Resource:  "test-resource",
		Namespace: "kube-system",
		StartTime: time.Now(),
	}

	ctx := ContextWithRequestInfo(context.Background(), reqCtx)
	enrichedLogger := EnrichLogr(ctx, logger)

	enrichedLogger.Info("test logr message")
	// Can't easily verify logr output without additional setup
	assert.NotNil(t, enrichedLogger)
}

func TestEnrichLogrWithoutRequestContext(t *testing.T) {
	config := Config{
		Level:  "info",
		Format: "json",
	}

	logger, err := NewLogrLogger(config)
	require.NoError(t, err)

	ctx := context.Background() // No request context
	enrichedLogger := EnrichLogr(ctx, logger)

	enrichedLogger.Info("test logr message")
	assert.NotNil(t, enrichedLogger)
}

func TestReconcilerMiddleware(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	// Mock reconciler that succeeds
	mockReconciler := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		// Verify context has been enriched
		reqCtx, ok := RequestInfoFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "reconcile", reqCtx.Operation)
		assert.Equal(t, req.Name, reqCtx.Resource)
		assert.Equal(t, req.Namespace, reqCtx.Namespace)

		return reconcile.Result{}, nil
	})

	wrapped := ReconcilerMiddleware(logger, mockReconciler)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	result, err := wrapped.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)

	// Should have logged start and completion
	logs := observedLogs.All()
	assert.GreaterOrEqual(t, len(logs), 2)

	// Check start log
	startLog := logs[0]
	assert.Equal(t, "Starting reconciliation", startLog.Message)

	// Check completion log
	completionLog := logs[len(logs)-1]
	assert.Equal(t, "Reconciliation completed", completionLog.Message)
}

func TestReconcilerMiddlewareWithError(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	testError := errors.New("reconcile failed")

	// Mock reconciler that fails
	mockReconciler := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, testError
	})

	wrapped := ReconcilerMiddleware(logger, mockReconciler)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	result, err := wrapped.Reconcile(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, testError, err)
	assert.True(t, result.RequeueAfter > 0)

	// Should have logged start and error
	logs := observedLogs.All()
	assert.GreaterOrEqual(t, len(logs), 2)

	// Check error log
	errorLog := logs[len(logs)-1]
	assert.Equal(t, "Reconciliation failed", errorLog.Message)
}

func TestReconcilerMiddlewareWithRequeue(t *testing.T) {
	// Create observed logger for testing
	observedZapCore, _ := observer.New(zap.InfoLevel)
	logger := zap.New(observedZapCore)

	// Mock reconciler that requests requeue
	mockReconciler := reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Minute,
		}, nil
	})

	wrapped := ReconcilerMiddleware(logger, mockReconciler)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	result, err := wrapped.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0)
	assert.Equal(t, 5*time.Minute, result.RequeueAfter)
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2 := generateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// Should be in expected format (timestamp-based)
	assert.Len(t, id1, 21) // Format: 20060102150405.000000
}

func TestRequestContextAllFields(t *testing.T) {
	now := time.Now()
	reqCtx := RequestContext{
		RequestID: "req-123",
		UserAgent: "test-agent/1.0",
		RemoteIP:  "192.168.1.100",
		Operation: "create",
		Resource:  "my-resource",
		Namespace: "my-namespace",
		StartTime: now,
		TraceID:   "trace-456",
		SpanID:    "span-789",
	}

	ctx := ContextWithRequestInfo(context.Background(), reqCtx)
	retrieved, ok := RequestInfoFromContext(ctx)

	require.True(t, ok)
	assert.Equal(t, "req-123", retrieved.RequestID)
	assert.Equal(t, "test-agent/1.0", retrieved.UserAgent)
	assert.Equal(t, "192.168.1.100", retrieved.RemoteIP)
	assert.Equal(t, "create", retrieved.Operation)
	assert.Equal(t, "my-resource", retrieved.Resource)
	assert.Equal(t, "my-namespace", retrieved.Namespace)
	assert.Equal(t, now, retrieved.StartTime)
	assert.Equal(t, "trace-456", retrieved.TraceID)
	assert.Equal(t, "span-789", retrieved.SpanID)
}
