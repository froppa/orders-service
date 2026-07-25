package store

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
	"github.com/froppa/orders-service/internal/observability"
)

type OrdersStore struct {
	metrics *observability.Metrics
	tracer  trace.Tracer
}

func NewOrdersStore(metrics *observability.Metrics, tracer trace.Tracer) *OrdersStore {
	return &OrdersStore{metrics: metrics, tracer: tracer}
}

func (s *OrdersStore) Create(ctx context.Context, q ports.DBTX, order orders.Order) error {
	ctx, span := s.tracer.Start(ctx, "store.orders.create")
	defer span.End()
	_, err := q.ExecContext(ctx, `INSERT INTO orders (id, customer_id, amount_cents, status, created_at) VALUES ($1, $2, $3, $4, $5)`,
		order.ID, order.CustomerID, order.AmountCents, order.Status, order.CreatedAt)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("orders_create").Inc()
		span.RecordError(err)
		return err
	}
	span.SetAttributes(attribute.String("order.id", order.ID))
	return nil
}

func (s *OrdersStore) GetByID(ctx context.Context, q ports.DBTX, id string) (orders.Order, error) {
	ctx, span := s.tracer.Start(ctx, "store.orders.get")
	defer span.End()
	var order orders.Order
	err := q.QueryRowContext(ctx, `SELECT id, customer_id, amount_cents, status, created_at FROM orders WHERE id = $1`, id).
		Scan(&order.ID, &order.CustomerID, &order.AmountCents, &order.Status, &order.CreatedAt)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("orders_get").Inc()
		if err == sql.ErrNoRows {
			return orders.Order{}, orders.ErrNotFound
		}
		span.RecordError(err)
		return orders.Order{}, err
	}
	span.SetAttributes(attribute.String("order.id", order.ID))
	return order, nil
}
