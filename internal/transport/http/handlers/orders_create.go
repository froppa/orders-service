package handlers

import (
	"net/http"

	"github.com/froppa/orders-service/internal/application/commands"
)

type OrdersCreateHandler struct {
	useCase *commands.CreateOrderHandler
}

type createOrderRequest struct {
	CustomerID  string `json:"customer_id"`
	AmountCents int    `json:"amount_cents"`
}

func NewOrdersCreateHandler(useCase *commands.CreateOrderHandler) *OrdersCreateHandler {
	return &OrdersCreateHandler{useCase: useCase}
}

func (h *OrdersCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", nil)
		return
	}
	order, err := h.useCase.Handle(r.Context(), commands.CreateOrderCommand{
		CustomerID:  req.CustomerID,
		AmountCents: req.AmountCents,
	})
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message, nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     order.ID,
		"status": order.Status,
	})
}
