package queries

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type GetOrderHandler struct {
	Transactor ports.Transactor
	Orders     ports.OrderRepository
	Tracer     trace.Tracer
}

func NewGetOrderHandler(transactor ports.Transactor, ordersRepo ports.OrderRepository, tracer trace.Tracer) *GetOrderHandler {
	return &GetOrderHandler{
		Transactor: transactor,
		Orders:     ordersRepo,
		Tracer:     tracer,
	}
}

func (h *GetOrderHandler) Handle(ctx context.Context, id string) (orders.Order, error) {
	ctx, span := h.Tracer.Start(ctx, "application.get_order")
	defer span.End()
	return h.Orders.GetByID(ctx, h.Transactor.DB(), id)
}
