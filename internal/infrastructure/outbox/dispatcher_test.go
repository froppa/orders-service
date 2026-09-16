package outbox

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/observability"
)

type dispatcherClock struct {
	now time.Time
}

func (c dispatcherClock) Now() time.Time { return c.now }

type dispatcherTransactor struct{}

func (dispatcherTransactor) DB() ports.DBTX             { return nil }
func (dispatcherTransactor) Ping(context.Context) error { return nil }
func (dispatcherTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type dispatcherOutbox struct {
	messages []ports.OutboxMessage
	marked   []string
}

func (o *dispatcherOutbox) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (o *dispatcherOutbox) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return o.messages, nil
}

func (o *dispatcherOutbox) MarkDispatched(_ context.Context, _ ports.DBTX, id string, _ time.Time) error {
	o.marked = append(o.marked, id)
	return nil
}

type dispatcherPublisher struct {
	topics []string
}

func (p *dispatcherPublisher) Publish(_ context.Context, topic, _ string, _ []byte) error {
	p.topics = append(p.topics, topic)
	return nil
}

func TestDispatcherRunOnce(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	repo := &dispatcherOutbox{
		messages: []ports.OutboxMessage{{
			ID:        "evt-1",
			Topic:     "orders.created",
			Key:       "order-1",
			Payload:   []byte(`{"id":"order-1"}`),
			CreatedAt: now,
		}},
	}
	publisher := &dispatcherPublisher{}
	metrics := observability.NewMetrics()
	dispatcher := NewDispatcher(
		dispatcherClock{now: now},
		dispatcherTransactor{},
		repo,
		publisher,
		zap.NewNop(),
		metrics,
		noop.NewTracerProvider().Tracer("test"),
	)

	if err := dispatcher.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(repo.marked) != 1 || repo.marked[0] != "evt-1" {
		t.Fatalf("marked = %v", repo.marked)
	}
	if len(publisher.topics) != 1 || publisher.topics[0] != "orders.created" {
		t.Fatalf("topics = %v", publisher.topics)
	}
}
