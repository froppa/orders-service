package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("SERVICE_NAME", "svc")
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("WORKER_INTERVAL", "4s")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := Load()
	if cfg.ServiceName != "svc" || cfg.Env != "test" || cfg.HTTPAddr != ":9999" || cfg.DBDSN != "postgres://x" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.ShutdownTimeout != 3*time.Second || cfg.WorkerInterval != 4*time.Second {
		t.Fatalf("unexpected durations: %+v", cfg)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}

	_ = os.Unsetenv("SERVICE_NAME")
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "bogus")
	t.Setenv("WORKER_INTERVAL", "bogus")

	cfg := Load()
	if cfg.ServiceName != "orders-service" || cfg.Env != "local" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DBDSN == "" {
		t.Fatal("expected default dsn")
	}
}
