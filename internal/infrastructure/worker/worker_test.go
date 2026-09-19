package worker

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/infrastructure/outbox"
	"github.com/froppa/orders-service/internal/observability"
)

type workerClock struct{}

func (workerClock) Now() time.Time { return time.Now().UTC() }

type workerTransactor struct{}

func (workerTransactor) DB() ports.DBTX             { return nil }
func (workerTransactor) Ping(context.Context) error { return nil }
func (workerTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type workerOutbox struct {
	calls int
}

func (w *workerOutbox) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (w *workerOutbox) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	w.calls++
	return nil, nil
}

func (w *workerOutbox) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

type workerPublisher struct{}

func (workerPublisher) Publish(context.Context, string, string, []byte) error { return nil }

func TestWorkerRunStopsOnCancel(t *testing.T) {
	outboxRepo := &workerOutbox{}
	dispatcher := outbox.NewDispatcher(
		workerClock{},
		workerTransactor{},
		outboxRepo,
		workerPublisher{},
		zap.NewNop(),
		observability.NewMetrics(),
		noop.NewTracerProvider().Tracer("test"),
	)
	worker := New(5*time.Millisecond, dispatcher, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if err := worker.Run(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
	if outboxRepo.calls == 0 {
		t.Fatal("expected dispatcher to be called")
	}
}
