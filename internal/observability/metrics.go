package observability

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	Registry            *prometheus.Registry
	HTTPRequestCount    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	OutboxDispatched    prometheus.Counter
	DBErrors            *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		Registry: registry,
		HTTPRequestCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "orders_service_http_requests_total", Help: "Total HTTP requests."},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "orders_service_http_request_duration_seconds",
				Help:    "HTTP request latency.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		OutboxDispatched: prometheus.NewCounter(
			prometheus.CounterOpts{Name: "orders_service_outbox_dispatched_total", Help: "Total dispatched outbox events."},
		),
		DBErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{Name: "orders_service_db_errors_total", Help: "Total DB errors."},
			[]string{"operation"},
		),
	}
	registry.MustRegister(metrics.HTTPRequestCount, metrics.HTTPRequestDuration, metrics.OutboxDispatched, metrics.DBErrors)
	return metrics
}
