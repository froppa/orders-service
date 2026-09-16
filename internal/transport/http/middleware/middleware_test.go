package middleware

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/observability"
	"go.uber.org/zap"
)

type middlewareTransactor struct {
	tx ports.DBTX
}

func (t middlewareTransactor) DB() ports.DBTX             { return t.tx }
func (t middlewareTransactor) Ping(context.Context) error { return nil }
func (t middlewareTransactor) WithinTx(ctx context.Context, fn func(context.Context, ports.DBTX) error) error {
	return fn(ctx, t.tx)
}

type middlewareDBTX struct{}

func (middlewareDBTX) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (middlewareDBTX) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (middlewareDBTX) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

type memoryIdempotency struct {
	records map[string]ports.IdempotencyRecord
}

func (m *memoryIdempotency) Get(_ context.Context, _ ports.DBTX, key string) (ports.IdempotencyRecord, error) {
	record, ok := m.records[key]
	if ok {
		return record, nil
	}
	return ports.IdempotencyRecord{}, ports.ErrIdempotencyKeyNotFound
}
func (m *memoryIdempotency) Reserve(_ context.Context, _ ports.DBTX, record ports.IdempotencyRecord) (bool, error) {
	if _, ok := m.records[record.Key]; ok {
		return false, nil
	}
	m.records[record.Key] = record
	return true, nil
}
func (m *memoryIdempotency) Finalize(_ context.Context, _ ports.DBTX, key string, responseCode int, responseBody []byte) error {
	record := m.records[key]
	record.ResponseCode = responseCode
	record.ResponseBody = responseBody
	m.records[key] = record
	return nil
}

func TestAuth(t *testing.T) {
	handler := Auth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
	if resp.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", resp.Header().Get("WWW-Authenticate"))
	}

	for _, header := range []string{"Bearer token", "bearer token", "BEARER token"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusNoContent {
			t.Fatalf("status for %q = %d", header, resp.Code)
		}
	}

	for _, header := range []string{"Bearer", "Bearer   ", "Basic token"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("status for %q = %d", header, resp.Code)
		}
	}
}

func TestRequestID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if observability.RequestIDFromContext(r.Context()) == "" {
			t.Fatal("expected request id in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected request id")
	}
}

func TestRecovery(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestLoggingAndTracing(t *testing.T) {
	logger := zap.NewNop()
	metrics := observability.NewMetrics()
	handler := RequestID(Tracing("orders-service")(Logging(logger, metrics, "orders-service")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/x", nil))
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestIdempotency(t *testing.T) {
	repo := &memoryIdempotency{records: make(map[string]ports.IdempotencyRecord)}
	logger := zap.NewNop()
	handler := Idempotency(repo, middlewareTransactor{tx: middlewareDBTX{}}, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{"id": "order-1"})
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-1","amount_cents":500}`))
	req.Header.Set("Idempotency-Key", "req-1")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("first status = %d", resp.Code)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-1","amount_cents":500}`))
	replayReq.Header.Set("Idempotency-Key", "req-1")
	replayResp := httptest.NewRecorder()
	handler.ServeHTTP(replayResp, replayReq)
	if replayResp.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatal("expected replay header")
	}
}

func TestIdempotencyConflict(t *testing.T) {
	repo := &memoryIdempotency{records: map[string]ports.IdempotencyRecord{
		"req-1": {
			Key:          "req-1",
			RequestHash:  "other",
			ResponseCode: http.StatusCreated,
			ResponseBody: []byte(`{"id":"order-1"}`),
			CreatedAt:    time.Now().UTC(),
		},
	}}
	logger := zap.NewNop()
	handler := Idempotency(repo, middlewareTransactor{tx: middlewareDBTX{}}, logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewBufferString(`{"customer_id":"cust-1","amount_cents":500}`))
	req.Header.Set("Idempotency-Key", "req-1")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d", resp.Code)
	}
}
