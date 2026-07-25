package app

import (
	"context"
	"testing"
	"time"

	"github.com/froppa/orders-service/internal/config"
)

func TestBuildFailsOnUnavailableDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Build(ctx, config.Config{
		ServiceName:     "orders-service",
		Env:             "test",
		HTTPAddr:        "127.0.0.1:0",
		DBDSN:           "postgres://orders:orders@127.0.0.1:1/orders?sslmode=disable",
		ShutdownTimeout: time.Second,
		WorkerInterval:  10 * time.Millisecond,
		LogLevel:        "debug",
	})
	if err == nil {
		t.Fatal("expected build error")
	}
}

func TestCloseWithNilDB(t *testing.T) {
	application := &App{}
	if err := application.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
