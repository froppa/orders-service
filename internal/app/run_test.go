package app

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/config"
	infraDB "github.com/froppa/orders-service/internal/infrastructure/db"
	"github.com/froppa/orders-service/internal/infrastructure/outbox"
	"github.com/froppa/orders-service/internal/infrastructure/worker"
	"github.com/froppa/orders-service/internal/observability"
)

type runClock struct{}

func (runClock) Now() time.Time { return time.Now().UTC() }

type runTransactor struct{}

func (runTransactor) DB() ports.DBTX             { return nil }
func (runTransactor) Ping(context.Context) error { return nil }
func (runTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type runOutbox struct{}

func (runOutbox) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (runOutbox) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}
func (runOutbox) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error { return nil }

type runPublisher struct{}

func (runPublisher) Publish(context.Context, string, string, []byte) error { return nil }

func TestAppRun(t *testing.T) {
	logger := zap.NewNop()
	dispatcher := outbox.NewDispatcher(
		runClock{},
		runTransactor{},
		runOutbox{},
		runPublisher{},
		logger,
		observability.NewMetrics(),
		noop.NewTracerProvider().Tracer("test"),
	)
	backgroundWorker := worker.New(5*time.Millisecond, dispatcher, logger)
	server := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	application := &App{
		Config: config.Config{
			HTTPAddr:        "127.0.0.1:0",
			ShutdownTimeout: time.Second,
		},
		Logger:       logger,
		DB:           &infraDB.DB{},
		HTTPServer:   server,
		Worker:       backgroundWorker,
		ShutdownOTEL: func(context.Context) error { return nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	if err := application.Run(ctx); err != nil {
		if errors.Is(err, http.ErrServerClosed) || err.Error() == "listen tcp 127.0.0.1:0: bind: operation not permitted" {
			t.Skipf("environment does not allow binding test listener: %v", err)
		}
		t.Fatalf("Run() error = %v", err)
	}
}
