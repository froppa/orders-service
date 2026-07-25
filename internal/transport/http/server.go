package httptransport

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/config"
	"github.com/froppa/orders-service/internal/observability"
	"github.com/froppa/orders-service/internal/transport/http/handlers"
	"github.com/froppa/orders-service/internal/transport/http/middleware"
	"go.uber.org/zap"
)

type Dependencies struct {
	Config        config.Config
	Logger        *zap.Logger
	Metrics       *observability.Metrics
	Idempotency   ports.IdempotencyRepository
	Transactor    ports.Transactor
	HealthHandler *handlers.HealthHandler
	CreateOrder   http.Handler
	GetOrder      http.Handler
}

func NewServer(deps Dependencies) *http.Server {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recovery)
	router.Use(middleware.Tracing(deps.Config.ServiceName))
	router.Use(middleware.Logging(deps.Logger, deps.Metrics, deps.Config.ServiceName))

	router.Get("/healthz", deps.HealthHandler.Healthz)
	router.Get("/readyz", deps.HealthHandler.Readyz)
	router.Handle("/metrics", handlers.Metrics(deps.Metrics))

	router.Route("/v1", func(r chi.Router) {
		r.With(middleware.Auth, middleware.Idempotency(deps.Idempotency, deps.Transactor, deps.Logger)).Post("/orders", deps.CreateOrder.ServeHTTP)
		r.Get("/orders/{id}", deps.GetOrder.ServeHTTP)
	})

	return &http.Server{
		Addr:              deps.Config.HTTPAddr,
		Handler:           router,
		IdleTimeout:       60 * time.Second,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
}
