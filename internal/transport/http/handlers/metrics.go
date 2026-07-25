package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/froppa/orders-service/internal/observability"
)

func Metrics(metrics *observability.Metrics) http.Handler {
	return promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
}
