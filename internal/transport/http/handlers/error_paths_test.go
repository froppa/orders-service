package handlers

import (
	"bytes"
	"context"
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
	"github.com/froppa/orders-service/internal/observability"
)

type errorTransactor struct{}

func (errorTransactor) DB() ports.DBTX             { return nil }
func (errorTransactor) Ping(context.Context) error { return nil }
func (errorTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, nil)
}

type errorOrdersRepo struct{}

func (errorOrdersRepo) Create(context.Context, ports.DBTX, orders.Order) error { return nil }
func (errorOrdersRepo) GetByID(context.Context, ports.DBTX, string) (orders.Order, error) {
	return orders.Order{}, orders.ErrNotFound
}

type errorOutboxRepo struct{}

func (errorOutboxRepo) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (errorOutboxRepo) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}

func (errorOutboxRepo) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

type errorClock struct{}

func (errorClock) Now() time.Time { return time.Now().UTC() }

func TestCreateOrderInvalidJSON(t *testing.T) {
	handler := NewOrdersCreateHandler(commands.NewCreateOrderHandler(
		errorClock{},
		errorTransactor{},
		errorOrdersRepo{},
		errorOutboxRepo{},
		noop.NewTracerProvider().Tracer("test"),
	))
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":`))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestCreateOrderInvalidDomainRequest(t *testing.T) {
	handler := NewOrdersCreateHandler(commands.NewCreateOrderHandler(
		errorClock{},
		errorTransactor{},
		errorOrdersRepo{},
		errorOutboxRepo{},
		noop.NewTracerProvider().Tracer("test"),
	))
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"","amount_cents":500}`))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestGetOrderNotFound(t *testing.T) {
	handler := NewOrdersGetHandler(queries.NewGetOrderHandler(errorTransactor{}, errorOrdersRepo{}, noop.NewTracerProvider().Tracer("test")))
	req := httptest.NewRequest(http.MethodGet, "/v1/orders/missing", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestMetricsHandler(t *testing.T) {
	resp := httptest.NewRecorder()
	Metrics(observability.NewMetrics()).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
}
