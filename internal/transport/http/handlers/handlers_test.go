package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/froppa/orders-service/internal/application/commands"
	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/application/queries"
	"github.com/froppa/orders-service/internal/domain/orders"
	"github.com/froppa/orders-service/internal/observability"
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
	ready := NewHealthHandler(zap.NewNop(), func(context.Context) error { return nil })
	healthResp := httptest.NewRecorder()
	ready.Healthz(healthResp, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d", healthResp.Code)
	}

	notReady := NewHealthHandler(zap.NewNop(), func(context.Context) error { return context.DeadlineExceeded })
	readyResp := httptest.NewRecorder()
	notReady.Readyz(readyResp, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", readyResp.Code)
	}
}

func TestReadyzLogsCauseInsteadOfReturningIt(t *testing.T) {
	const causeDetail = "10.0.0.7:5432"
	core, logs := observer.New(zapcore.DebugLevel)
	handler := NewHealthHandler(zap.New(core), func(context.Context) error {
		return errors.New("dial tcp " + causeDetail + ": connect: connection refused")
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	req = req.WithContext(observability.WithRequestID(req.Context(), "req-1"))
	resp := httptest.NewRecorder()
	handler.Readyz(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.Code)
	}
	if strings.Contains(resp.Body.String(), causeDetail) {
		t.Fatalf("response body leaked the readiness error: %s", resp.Body.String())
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Error.Code != "not_ready" || body.Error.Message != "service is not ready" {
		t.Fatalf("error = %+v", body.Error)
	}
	if body.Error.Details != nil {
		t.Fatalf("details = %v, want nil", body.Error.Details)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %s, want warn", entry.Level)
	}
	if strings.Contains(entry.Message, causeDetail) {
		t.Fatalf("log message embeds the cause instead of a zap.Error field: %s", entry.Message)
	}
	fields := entry.ContextMap()
	if loggedErr, _ := fields["error"].(string); !strings.Contains(loggedErr, causeDetail) {
		t.Fatalf("error field = %v, want the readiness error", fields["error"])
	}
	if fields["request_id"] != "req-1" {
		t.Fatalf("request_id = %v, want req-1", fields["request_id"])
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
