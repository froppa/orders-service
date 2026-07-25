package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/froppa/orders-service/internal/config"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func NewLogger(cfg config.Config) *zap.Logger {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(parseLevel(cfg.LogLevel))
	zapCfg.OutputPaths = []string{"stdout"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}
	logger, err := zapCfg.Build(zap.AddCaller())
	if err != nil {
		return zap.NewNop()
	}
	return logger
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	fields := make([]zap.Field, 0, 3)
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		fields = append(fields,
			zap.String("trace_id", span.TraceID().String()),
			zap.String("span_id", span.SpanID().String()),
		)
	}
	if len(fields) == 0 {
		return logger
	}
	return logger.With(fields...)
}

func parseLevel(value string) zapcore.Level {
	switch value {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
