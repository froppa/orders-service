package app

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/commands"
	"github.com/froppa/orders-service/internal/application/queries"
	"github.com/froppa/orders-service/internal/config"
	infraDB "github.com/froppa/orders-service/internal/infrastructure/db"
	"github.com/froppa/orders-service/internal/infrastructure/outbox"
	"github.com/froppa/orders-service/internal/infrastructure/store"
	"github.com/froppa/orders-service/internal/infrastructure/worker"
	"github.com/froppa/orders-service/internal/observability"
	"github.com/froppa/orders-service/internal/provenance"
	httptransport "github.com/froppa/orders-service/internal/transport/http"
	handlers "github.com/froppa/orders-service/internal/transport/http/handlers"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func Build(ctx context.Context, cfg config.Config) (*App, error) {
	logger := observability.NewLogger(cfg).With(
		zap.String("service", cfg.ServiceName),
		zap.String("env", cfg.Env),
	)
	logger.Info("initializing orders-service", provenance.Fields()...)
	shutdownOTEL, err := observability.InitOTEL(ctx, cfg)
	if err != nil {
		return nil, err
	}
	metrics := observability.NewMetrics()
	database, err := infraDB.Open(ctx, cfg.DBDSN)
	if err != nil {
		_ = shutdownOTEL(context.Background())
		return nil, err
	}

	if err := database.Migrate(ctx); err != nil {
		_ = shutdownOTEL(context.Background())
		_ = database.Close()
		return nil, err
	}

	tracer := otel.Tracer(cfg.ServiceName)
	clock := systemClock{}
	ordersStore := store.NewOrdersStore(metrics, tracer)
	idempotencyStore := store.NewIdempotencyStore(metrics, tracer)
	outboxStore := store.NewOutboxStore(metrics, tracer)
	publisher := outbox.NewLogPublisher(logger)

	createOrder := commands.NewCreateOrderHandler(clock, database, ordersStore, outboxStore, tracer)
	getOrder := queries.NewGetOrderHandler(database, ordersStore, tracer)
	dispatcher := outbox.NewDispatcher(clock, database, outboxStore, publisher, logger, metrics, tracer)
	backgroundWorker := worker.New(cfg.WorkerInterval, dispatcher, logger)
	health := handlers.NewHealthHandler(logger, func(ctx context.Context) error {
		return database.Ping(ctx)
	})
	server := httptransport.NewServer(httptransport.Dependencies{
		Config:        cfg,
		Logger:        logger,
		Metrics:       metrics,
		Idempotency:   idempotencyStore,
		Transactor:    database,
		HealthHandler: health,
		CreateOrder:   handlers.NewOrdersCreateHandler(createOrder),
		GetOrder:      handlers.NewOrdersGetHandler(getOrder),
	})

	return &App{
		Config:       cfg,
		Logger:       logger,
		DB:           database,
		HTTPServer:   server,
		Worker:       backgroundWorker,
		ShutdownOTEL: shutdownOTEL,
	}, nil
}
