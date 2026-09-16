package httptransport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/commands"
	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/application/queries"
	"github.com/froppa/orders-service/internal/config"
	"github.com/froppa/orders-service/internal/domain/orders"
	"github.com/froppa/orders-service/internal/observability"
	handlers "github.com/froppa/orders-service/internal/transport/http/handlers"
)

type testClock struct {
	now time.Time
}

func (c testClock) Now() time.Time { return c.now }

type memoryTransactor struct {
	tx ports.DBTX
}

func (t *memoryTransactor) DB() ports.DBTX             { return t.tx }
func (t *memoryTransactor) Ping(context.Context) error { return nil }
func (t *memoryTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, t.tx)
}

type noopDBTX struct{}

func (noopDBTX) ExecContext(context.Context, string, ...any) (sql.Result, error) { return nil, nil }
func (noopDBTX) QueryContext(context.Context, string, ...any) (*sql.Rows, error) { return nil, nil }
func (noopDBTX) QueryRowContext(context.Context, string, ...any) *sql.Row        { return nil }

type memoryOrdersRepo struct {
	orders map[string]orders.Order
}

func (r *memoryOrdersRepo) Create(_ context.Context, _ ports.DBTX, order orders.Order) error {
	r.orders[order.ID] = order
	return nil
}

func (r *memoryOrdersRepo) GetByID(_ context.Context, _ ports.DBTX, id string) (orders.Order, error) {
	order, ok := r.orders[id]
	if !ok {
		return orders.Order{}, orders.ErrNotFound
	}
	return order, nil
}

type memoryOutboxRepo struct{}

func (memoryOutboxRepo) Add(context.Context, ports.DBTX, ports.OutboxMessage) error { return nil }
func (memoryOutboxRepo) ListPending(context.Context, ports.DBTX, int) ([]ports.OutboxMessage, error) {
	return nil, nil
}
func (memoryOutboxRepo) MarkDispatched(context.Context, ports.DBTX, string, time.Time) error {
	return nil
}

type memoryIdempotencyRepo struct {
	records map[string]ports.IdempotencyRecord
}

func (r *memoryIdempotencyRepo) Get(_ context.Context, _ ports.DBTX, key string) (ports.IdempotencyRecord, error) {
	record, ok := r.records[key]
	if !ok {
		return ports.IdempotencyRecord{}, ports.ErrIdempotencyKeyNotFound
	}
	return record, nil
}

func (r *memoryIdempotencyRepo) Reserve(_ context.Context, _ ports.DBTX, record ports.IdempotencyRecord) (bool, error) {
	if _, exists := r.records[record.Key]; exists {
		return false, nil
	}
	r.records[record.Key] = record
	return true, nil
}

func (r *memoryIdempotencyRepo) Finalize(_ context.Context, _ ports.DBTX, key string, responseCode int, responseBody []byte) error {
	record := r.records[key]
	record.ResponseCode = responseCode
	record.ResponseBody = append([]byte(nil), responseBody...)
	r.records[key] = record
	return nil
}

func TestCreateOrderAndReplayIdempotency(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	cfg := config.Config{ServiceName: "orders-service", Env: "test", HTTPAddr: ":0", LogLevel: "debug"}
	logger := zap.NewNop()
	metrics := observability.NewMetrics()
	tracer := noop.NewTracerProvider().Tracer("test")

	ordersRepo := &memoryOrdersRepo{orders: make(map[string]orders.Order)}
	transactor := &memoryTransactor{tx: noopDBTX{}}
	createHandler := handlers.NewOrdersCreateHandler(commands.NewCreateOrderHandler(
		testClock{now: now},
		transactor,
		ordersRepo,
		memoryOutboxRepo{},
		tracer,
	))
	getHandler := handlers.NewOrdersGetHandler(queries.NewGetOrderHandler(transactor, ordersRepo, tracer))
	server := NewServer(Dependencies{
		Config:        cfg,
		Logger:        logger,
		Metrics:       metrics,
		Idempotency:   &memoryIdempotencyRepo{records: make(map[string]ports.IdempotencyRecord)},
		Transactor:    transactor,
		HealthHandler: handlers.NewHealthHandler(logger, func(context.Context) error { return nil }),
		CreateOrder:   createHandler,
		GetOrder:      getHandler,
	})

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-123","amount_cents":1299}`))
	firstReq.Header.Set("Authorization", "Bearer token")
	firstReq.Header.Set("Idempotency-Key", "create-1")
	firstReq.Header.Set("Content-Type", "application/json")
	firstResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(firstResp, firstReq)
	if firstResp.Code != http.StatusCreated {
		t.Fatalf("first status = %d body=%s", firstResp.Code, firstResp.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(firstResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	orderID, _ := created["id"].(string)
	if orderID == "" {
		t.Fatal("expected created order id")
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-123","amount_cents":1299}`))
	secondReq.Header.Set("Authorization", "Bearer token")
	secondReq.Header.Set("Idempotency-Key", "create-1")
	secondReq.Header.Set("Content-Type", "application/json")
	secondResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusCreated {
		t.Fatalf("second status = %d body=%s", secondResp.Code, secondResp.Body.String())
	}
	if secondResp.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("expected replay header")
	}
	if secondResp.Body.String() != firstResp.Body.String() {
		t.Fatalf("replayed body mismatch: %s != %s", secondResp.Body.String(), firstResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/orders/"+orderID, nil)
	getResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResp.Code, getResp.Body.String())
	}
}

func TestIdempotencyConflict(t *testing.T) {
	cfg := config.Config{ServiceName: "orders-service", Env: "test", HTTPAddr: ":0", LogLevel: "debug"}
	logger := zap.NewNop()
	metrics := observability.NewMetrics()
	tracer := noop.NewTracerProvider().Tracer("test")
	ordersRepo := &memoryOrdersRepo{orders: make(map[string]orders.Order)}
	transactor := &memoryTransactor{tx: noopDBTX{}}
	server := NewServer(Dependencies{
		Config:        cfg,
		Logger:        logger,
		Metrics:       metrics,
		Idempotency:   &memoryIdempotencyRepo{records: make(map[string]ports.IdempotencyRecord)},
		Transactor:    transactor,
		HealthHandler: handlers.NewHealthHandler(logger, func(context.Context) error { return nil }),
		CreateOrder:   handlers.NewOrdersCreateHandler(commands.NewCreateOrderHandler(testClock{now: time.Now()}, transactor, ordersRepo, memoryOutboxRepo{}, tracer)),
		GetOrder:      handlers.NewOrdersGetHandler(queries.NewGetOrderHandler(transactor, ordersRepo, tracer)),
	})

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-123","amount_cents":1299}`))
	firstReq.Header.Set("Authorization", "Bearer token")
	firstReq.Header.Set("Idempotency-Key", "create-1")
	firstReq.Header.Set("Content-Type", "application/json")
	firstResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(firstResp, firstReq)

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-123","amount_cents":999}`))
	secondReq.Header.Set("Authorization", "Bearer token")
	secondReq.Header.Set("Idempotency-Key", "create-1")
	secondReq.Header.Set("Content-Type", "application/json")
	secondResp := httptest.NewRecorder()
	server.Handler.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusConflict {
		t.Fatalf("second status = %d body=%s", secondResp.Code, secondResp.Body.String())
	}
}
