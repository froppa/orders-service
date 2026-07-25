package middleware

import (
	"net/http"
	"time"

	"github.com/froppa/orders-service/internal/observability"
	"go.uber.org/zap"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func Logging(logger *zap.Logger, metrics *observability.Metrics, serviceName string) func(http.Handler) http.Handler {
	_ = serviceName
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(writer, r)

			duration := time.Since(start)
			statusLabel := http.StatusText(writer.status)
			metrics.HTTPRequestCount.WithLabelValues(r.Method, r.URL.Path, statusLabel).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

			observability.WithContext(r.Context(), logger).Info("http request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", writer.status),
				zap.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	}
}
