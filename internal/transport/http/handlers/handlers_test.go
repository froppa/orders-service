package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/froppa/orders-service/internal/application/commands"
	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/application/queries"
	"github.com/froppa/orders-service/internal/domain/orders"
)

type handlerClock struct {
	now time.Time
}

func (c handlerClock) Now() time.Time { return c.now }

type handlerTransactor struct{}

func (handlerTransactor) DB() ports.DBTX             { return nil }
func (handlerTransactor) Ping(context.Context) error { return nil }
func (handlerTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type handlerOrdersRepo struct {
	order orders.Order
}

func (r *handlerOrdersRepo) Create(_ context.Context, _ ports.DBTX, order orders.Order) error {
	r.order = order
	return nil
}
func (r *handlerOrdersRepo) GetByID(_ context.Context, _ ports.DBTX, id string) (orders.Order, error) {
	if r.order.ID != id {
		return orders.Order{}, orders.ErrNotFound
	}
	return r.order, nil
}

type handlerOutboxRepo struct{}

func (handlerOutboxRepo) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (handlerOutboxRepo) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}
func (handlerOutboxRepo) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

func TestHealthHandlers(t *testing.T) {
	ready := NewHealthHandler(func(context.Context) error { return nil })
	healthResp := httptest.NewRecorder()
	ready.Healthz(healthResp, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d", healthResp.Code)
	}

	notReady := NewHealthHandler(func(context.Context) error { return context.DeadlineExceeded })
	readyResp := httptest.NewRecorder()
	notReady.Readyz(readyResp, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", readyResp.Code)
	}
}

func TestOrdersHandlers(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	ordersRepo := &handlerOrdersRepo{}
	transactor := handlerTransactor{}
	tracer := noop.NewTracerProvider().Tracer("test")
	create := NewOrdersCreateHandler(commands.NewCreateOrderHandler(handlerClock{now: now}, transactor, ordersRepo, handlerOutboxRepo{}, tracer))

	createReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-1","amount_cents":500}`))
	createResp := httptest.NewRecorder()
	create.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", createResp.Code, createResp.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	orderID := created["id"].(string)

	get := NewOrdersGetHandler(queries.NewGetOrderHandler(transactor, ordersRepo, tracer))
	getReq := httptest.NewRequest(http.MethodGet, "/v1/orders/"+orderID, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", orderID)
	getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, routeCtx))
	getResp := httptest.NewRecorder()
	get.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
	}
}
