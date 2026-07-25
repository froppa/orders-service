package commands

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type invalidClock struct{}

func (invalidClock) Now() time.Time { return time.Now().UTC() }

type invalidTransactor struct{}

func (invalidTransactor) DB() ports.DBTX             { return nil }
func (invalidTransactor) Ping(context.Context) error { return nil }
func (invalidTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type invalidOrdersRepo struct{}

func (invalidOrdersRepo) Create(context.Context, ports.DBTX, orders.Order) error { return nil }
func (invalidOrdersRepo) GetByID(context.Context, ports.DBTX, string) (orders.Order, error) {
	return orders.Order{}, nil
}

type invalidOutboxRepo struct{}

func (invalidOutboxRepo) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (invalidOutboxRepo) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}
func (invalidOutboxRepo) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

func TestCreateOrderHandlerRejectsInvalidInput(t *testing.T) {
	handler := NewCreateOrderHandler(
		invalidClock{},
		invalidTransactor{},
		invalidOrdersRepo{},
		invalidOutboxRepo{},
		noop.NewTracerProvider().Tracer("test"),
	)

	if _, err := handler.Handle(context.Background(), CreateOrderCommand{CustomerID: "", AmountCents: 10}); err != orders.ErrInvalidCustomerID {
		t.Fatalf("expected ErrInvalidCustomerID, got %v", err)
	}
}
