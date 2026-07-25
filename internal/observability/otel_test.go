package observability

import (
	"context"
	"testing"

	"github.com/froppa/orders-service/internal/config"
	"go.uber.org/zap/zapcore"
)

func TestInitOTEL(t *testing.T) {
	shutdown, err := InitOTEL(context.Background(), config.Config{
		ServiceName: "orders-service",
		Env:         "test",
	})
	if err != nil {
		t.Fatalf("InitOTEL() error = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestNewMetricsAndParseLevel(t *testing.T) {
	metrics := NewMetrics()
	if metrics.Registry == nil {
		t.Fatal("expected registry")
	}
	if parseLevel("debug") != zapcore.DebugLevel {
		t.Fatalf("unexpected debug level")
	}
	if parseLevel("warn") != zapcore.WarnLevel {
		t.Fatalf("unexpected warn level")
	}
}
