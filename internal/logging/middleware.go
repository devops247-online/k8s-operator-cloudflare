package logging

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RequestContext contains common fields for request logging
type RequestContext struct {
	RequestID string
	UserAgent string
	RemoteIP  string
	Operation string
	Resource  string
	Namespace string
	StartTime time.Time
	TraceID   string
	SpanID    string
}

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// RequestContextKey is the key for storing RequestContext in context
	RequestContextKey ContextKey = "request_context"
	// LoggerContextKey is the key for storing logger in context
	LoggerContextKey ContextKey = "logger"
)

// ContextWithRequestInfo adds request information to the context
func ContextWithRequestInfo(ctx context.Context, reqCtx RequestContext) context.Context {
	return context.WithValue(ctx, RequestContextKey, reqCtx)
}

// RequestInfoFromContext extracts request information from context
func RequestInfoFromContext(ctx context.Context) (RequestContext, bool) {
	reqCtx, ok := ctx.Value(RequestContextKey).(RequestContext)
	return reqCtx, ok
}

// ContextWithLogger adds a logger to the context
func ContextWithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, LoggerContextKey, logger)
}

// LoggerFromContext extracts logger from context
func LoggerFromContext(ctx context.Context) (*zap.Logger, bool) {
	logger, ok := ctx.Value(LoggerContextKey).(*zap.Logger)
	return logger, ok
}

// ContextWithLogr adds a logr.Logger to the context
func ContextWithLogr(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, LoggerContextKey, logger)
}

// LogrFromContext extracts logr.Logger from context, falling back to default if not found
func LogrFromContext(ctx context.Context) logr.Logger {
	if logger, ok := ctx.Value(LoggerContextKey).(logr.Logger); ok {
		return logger
	}
	// Return a no-op logger if none found
	return logr.Discard()
}

// EnrichLogger adds request context fields to the logger
func EnrichLogger(ctx context.Context, logger *zap.Logger) *zap.Logger {
	// Add trace context
	logger = WithTraceContext(ctx, logger)

	// Add request context if available
	if reqCtx, ok := RequestInfoFromContext(ctx); ok {
		fields := make([]zap.Field, 0, 8)

		if reqCtx.RequestID != "" {
			fields = append(fields, zap.String("request.id", reqCtx.RequestID))
		}
		if reqCtx.Operation != "" {
			fields = append(fields, zap.String("operation", reqCtx.Operation))
		}
		if reqCtx.Resource != "" {
			fields = append(fields, zap.String("resource", reqCtx.Resource))
		}
		if reqCtx.Namespace != "" {
			fields = append(fields, zap.String("namespace", reqCtx.Namespace))
		}
		if reqCtx.UserAgent != "" {
			fields = append(fields, zap.String("user_agent", reqCtx.UserAgent))
		}
		if reqCtx.RemoteIP != "" {
			fields = append(fields, zap.String("remote_ip", reqCtx.RemoteIP))
		}
		if !reqCtx.StartTime.IsZero() {
			fields = append(fields, zap.Time("start_time", reqCtx.StartTime))
		}

		if len(fields) > 0 {
			logger = logger.With(fields...)
		}
	}

	return logger
}

// EnrichLogr adds request context fields to the logr.Logger
func EnrichLogr(ctx context.Context, logger logr.Logger) logr.Logger {
	// Add trace context
	logger = WithTraceContextLogr(ctx, logger)

	// Add request context if available
	if reqCtx, ok := RequestInfoFromContext(ctx); ok {
		values := make([]interface{}, 0, 16)

		if reqCtx.RequestID != "" {
			values = append(values, "request.id", reqCtx.RequestID)
		}
		if reqCtx.Operation != "" {
			values = append(values, "operation", reqCtx.Operation)
		}
		if reqCtx.Resource != "" {
			values = append(values, "resource", reqCtx.Resource)
		}
		if reqCtx.Namespace != "" {
			values = append(values, "namespace", reqCtx.Namespace)
		}
		if reqCtx.UserAgent != "" {
			values = append(values, "user_agent", reqCtx.UserAgent)
		}
		if reqCtx.RemoteIP != "" {
			values = append(values, "remote_ip", reqCtx.RemoteIP)
		}
		if !reqCtx.StartTime.IsZero() {
			values = append(values, "start_time", reqCtx.StartTime.Format(time.RFC3339))
		}

		if len(values) > 0 {
			logger = logger.WithValues(values...)
		}
	}

	return logger
}

// ReconcilerMiddleware wraps a reconciler with structured logging
func ReconcilerMiddleware(logger *zap.Logger, reconciler reconcile.Reconciler) reconcile.Reconciler {
	return reconcile.Func(func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		startTime := time.Now()

		// Create request context
		reqCtx := RequestContext{
			RequestID: generateRequestID(),
			Operation: "reconcile",
			Resource:  req.Name,
			Namespace: req.Namespace,
			StartTime: startTime,
		}

		// Add trace context if available
		reqCtx.TraceID = GetTraceID(ctx)
		reqCtx.SpanID = GetSpanID(ctx)

		// Enrich context
		ctx = ContextWithRequestInfo(ctx, reqCtx)
		enrichedLogger := EnrichLogger(ctx, logger)
		ctx = ContextWithLogger(ctx, enrichedLogger)

		// Log request start
		enrichedLogger.Info("Starting reconciliation",
			zap.Duration("timeout", time.Duration(0)), // Will be set by controller-runtime
		)

		// Execute reconciler
		result, err := reconciler.Reconcile(ctx, req)

		// Log completion
		duration := time.Since(startTime)
		if err != nil {
			enrichedLogger.Error("Reconciliation failed",
				zap.Error(err),
				zap.Duration("duration", duration),
				zap.Duration("requeue_after", result.RequeueAfter),
			)
		} else {
			enrichedLogger.Info("Reconciliation completed",
				zap.Duration("duration", duration),
				zap.Duration("requeue_after", result.RequeueAfter),
			)
		}

		return result, err
	})
}

// generateRequestID generates a simple request ID
// In production, you might want to use UUID or other methods
func generateRequestID() string {
	return time.Now().Format("20060102150405.000000")
}

// GenerateRequestID generates a simple request ID (public function)
func GenerateRequestID() string {
	return generateRequestID()
}
