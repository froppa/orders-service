package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/froppa/orders-service/internal/config"
)

func TestWithContextAddsRequestID(t *testing.T) {
	logger := zap.NewNop()
	ctx := WithRequestID(context.Background(), "req-1")
	enriched := WithContext(ctx, logger)
	if enriched == nil {
		t.Fatal("expected logger")
	}
	if RequestIDFromContext(ctx) != "req-1" {
		t.Fatalf("request id = %q", RequestIDFromContext(ctx))
	}
}

func TestWithContextHandlesTrace(t *testing.T) {
	ctx, span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "span")
	defer span.End()
	logger := WithContext(ctx, zap.NewNop())
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestNewLoggerAndParseLevel(t *testing.T) {
	if NewLogger(config.Config{LogLevel: "debug"}) == nil {
		t.Fatal("expected logger")
	}
	if parseLevel("debug") != zapcore.DebugLevel {
		t.Fatal("expected debug level")
	}
	if parseLevel("warn") != zapcore.WarnLevel {
		t.Fatal("expected warn level")
	}
	if parseLevel("error") != zapcore.ErrorLevel {
		t.Fatal("expected error level")
	}
	if parseLevel("other") != zapcore.InfoLevel {
		t.Fatal("expected info level")
	}
}
