package config

import (
	"os"
	"time"
)

type Config struct {
	ServiceName          string
	Env                  string
	HTTPAddr             string
	DBDSN                string
	ShutdownTimeout      time.Duration
	WorkerInterval       time.Duration
	OTELExporterEndpoint string
	LogLevel             string
}

func Load() Config {
	return Config{
		ServiceName:          getEnv("SERVICE_NAME", "orders-service"),
		Env:                  getEnv("APP_ENV", "local"),
		HTTPAddr:             getEnv("HTTP_ADDR", ":8080"),
		DBDSN:                getEnv("DATABASE_URL", "postgres://orders:orders@localhost:5432/orders?sslmode=disable"),
		ShutdownTimeout:      getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerInterval:       getEnvDuration("WORKER_INTERVAL", 2*time.Second),
		OTELExporterEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
