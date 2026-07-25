package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/froppa/orders-service/internal/application/queries"
)

type OrdersGetHandler struct {
	useCase *queries.GetOrderHandler
}

func NewOrdersGetHandler(useCase *queries.GetOrderHandler) *OrdersGetHandler {
	return &OrdersGetHandler{useCase: useCase}
}

func (h *OrdersGetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	order, err := h.useCase.Handle(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		status, code, message := mapDomainError(err)
		writeError(w, status, code, message, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           order.ID,
		"customer_id":  order.CustomerID,
		"amount_cents": order.AmountCents,
		"status":       order.Status,
	})
}
