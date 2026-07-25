package commands

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

type fakeTransactor struct {
	tx ports.DBTX
}

func (t fakeTransactor) DB() ports.DBTX             { return t.tx }
func (t fakeTransactor) Ping(context.Context) error { return nil }
func (t fakeTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, t.tx)
}

type fakeOrdersRepo struct {
	created orders.Order
}

func (r *fakeOrdersRepo) Create(_ context.Context, _ ports.DBTX, order orders.Order) error {
	r.created = order
	return nil
}

func (r *fakeOrdersRepo) GetByID(context.Context, ports.DBTX, string) (orders.Order, error) {
	return orders.Order{}, nil
}

type fakeOutboxRepo struct {
	message ports.OutboxMessage
}

func (r *fakeOutboxRepo) Add(_ context.Context, _ ports.DBTX, message ports.OutboxMessage) error {
	r.message = message
	return nil
}

func (r *fakeOutboxRepo) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}

func (r *fakeOutboxRepo) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

func TestCreateOrderHandler(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	ordersRepo := &fakeOrdersRepo{}
	outboxRepo := &fakeOutboxRepo{}
	handler := NewCreateOrderHandler(fakeClock{now: now}, fakeTransactor{}, ordersRepo, outboxRepo, noop.NewTracerProvider().Tracer("test"))

	order, err := handler.Handle(context.Background(), CreateOrderCommand{
		CustomerID:  "cust-123",
		AmountCents: 1599,
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if ordersRepo.created.ID != order.ID {
		t.Fatalf("created order id = %q want %q", ordersRepo.created.ID, order.ID)
	}
	if outboxRepo.message.Topic != "orders.created" {
		t.Fatalf("outbox topic = %q", outboxRepo.message.Topic)
	}

	var payload map[string]any
	if err := json.Unmarshal(outboxRepo.message.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["customer_id"] != "cust-123" {
		t.Fatalf("payload customer_id = %v", payload["customer_id"])
	}
}
