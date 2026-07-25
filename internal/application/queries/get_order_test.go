package queries

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type queryTransactor struct{}

func (queryTransactor) DB() ports.DBTX             { return nil }
func (queryTransactor) Ping(context.Context) error { return nil }
func (queryTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type queryOrdersRepo struct {
	order orders.Order
}

func (r queryOrdersRepo) Create(context.Context, ports.DBTX, orders.Order) error { return nil }
func (r queryOrdersRepo) GetByID(context.Context, ports.DBTX, string) (orders.Order, error) {
	return r.order, nil
}

func TestGetOrderHandler(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	handler := NewGetOrderHandler(queryTransactor{}, queryOrdersRepo{
		order: orders.Order{
			ID:          "order-1",
			CustomerID:  "cust-123",
			AmountCents: 100,
			Status:      orders.StatusCreated,
			CreatedAt:   now,
		},
	}, noop.NewTracerProvider().Tracer("test"))

	order, err := handler.Handle(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if order.ID != "order-1" {
		t.Fatalf("ID = %q", order.ID)
	}
}
