package commands

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type CreateOrderCommand struct {
	CustomerID  string `json:"customer_id"`
	AmountCents int    `json:"amount_cents"`
}

type CreateOrderHandler struct {
	Clock      ports.Clock
	Transactor ports.Transactor
	Orders     ports.OrderRepository
	Outbox     ports.OutboxRepository
	Tracer     trace.Tracer
}

func NewCreateOrderHandler(clock ports.Clock, transactor ports.Transactor, ordersRepo ports.OrderRepository, outboxRepo ports.OutboxRepository, tracer trace.Tracer) *CreateOrderHandler {
	return &CreateOrderHandler{
		Clock:      clock,
		Transactor: transactor,
		Orders:     ordersRepo,
		Outbox:     outboxRepo,
		Tracer:     tracer,
	}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, command CreateOrderCommand) (orders.Order, error) {
	ctx, span := h.Tracer.Start(ctx, "application.create_order")
	defer span.End()

	order, err := orders.New(command.CustomerID, command.AmountCents, h.Clock.Now())
	if err != nil {
		return orders.Order{}, err
	}

	if err := h.Transactor.WithinTx(ctx, func(ctx context.Context, tx ports.DBTX) error {
		if err := h.Orders.Create(ctx, tx, order); err != nil {
			return err
		}

		payload, err := json.Marshal(map[string]any{
			"id":           order.ID,
			"customer_id":  order.CustomerID,
			"amount_cents": order.AmountCents,
			"status":       order.Status,
			"created_at":   order.CreatedAt,
		})
		if err != nil {
			return err
		}

		return h.Outbox.Add(ctx, tx, ports.OutboxMessage{
			ID:        uuid.NewString(),
			Topic:     "orders.created",
			Key:       order.ID,
			Payload:   payload,
			CreatedAt: h.Clock.Now().UTC(),
		})
	}); err != nil {
		return orders.Order{}, err
	}

	return order, nil
}
