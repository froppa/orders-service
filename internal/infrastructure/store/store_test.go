package store

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/domain/orders"
	"github.com/froppa/orders-service/internal/observability"
)

func TestOrdersStoreCreateAndGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	metrics := observability.NewMetrics()
	store := NewOrdersStore(metrics, noop.NewTracerProvider().Tracer("test"))
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	order := orders.Order{
		ID:          "11111111-1111-1111-1111-111111111111",
		CustomerID:  "cust-123",
		AmountCents: 1499,
		Status:      orders.StatusCreated,
		CreatedAt:   now,
	}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO orders (id, customer_id, amount_cents, status, created_at) VALUES ($1, $2, $3, $4, $5)`)).
		WithArgs(order.ID, order.CustomerID, order.AmountCents, order.Status, order.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.Create(context.Background(), db, order); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rows := sqlmock.NewRows([]string{"id", "customer_id", "amount_cents", "status", "created_at"}).
		AddRow(order.ID, order.CustomerID, order.AmountCents, order.Status, order.CreatedAt)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, customer_id, amount_cents, status, created_at FROM orders WHERE id = $1`)).
		WithArgs(order.ID).
		WillReturnRows(rows)
	got, err := store.GetByID(context.Background(), db, order.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != order.ID {
		t.Fatalf("GetByID().ID = %q", got.ID)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, customer_id, amount_cents, status, created_at FROM orders WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.GetByID(context.Background(), db, "missing"); err != orders.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestIdempotencyStoreLifecycle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewIdempotencyStore(observability.NewMetrics(), noop.NewTracerProvider().Tracer("test"))
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	record := ports.IdempotencyRecord{
		Key:          "req-1",
		RequestHash:  "hash",
		ResponseCode: 0,
		ResponseBody: []byte(`{}`),
		CreatedAt:    now,
	}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO idempotency_keys (key, request_hash, response_code, response_body, created_at) VALUES ($1, $2, $3, $4, $5)`)).
		WithArgs(record.Key, record.RequestHash, record.ResponseCode, record.ResponseBody, record.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	reserved, err := store.Reserve(context.Background(), db, record)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if !reserved {
		t.Fatal("expected reserved=true")
	}

	rows := sqlmock.NewRows([]string{"key", "request_hash", "response_code", "response_body", "created_at"}).
		AddRow(record.Key, record.RequestHash, 201, []byte(`{"id":"1","status":"created"}`), record.CreatedAt)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT key, request_hash, response_code, response_body, created_at FROM idempotency_keys WHERE key = $1`)).
		WithArgs(record.Key).
		WillReturnRows(rows)
	got, err := store.Get(context.Background(), db, record.Key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ResponseCode != 201 {
		t.Fatalf("ResponseCode = %d", got.ResponseCode)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT key, request_hash, response_code, response_body, created_at FROM idempotency_keys WHERE key = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)
	if _, err := store.Get(context.Background(), db, "missing"); err != ports.ErrIdempotencyKeyNotFound {
		t.Fatalf("expected not found, got %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE idempotency_keys SET response_code = $2, response_body = $3 WHERE key = $1`)).
		WithArgs(record.Key, 201, []byte(`{"id":"1","status":"created"}`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.Finalize(context.Background(), db, record.Key, 201, []byte(`{"id":"1","status":"created"}`)); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestOutboxStoreListAndMark(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	store := NewOutboxStore(observability.NewMetrics(), noop.NewTracerProvider().Tracer("test"))
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"id", "topic", "key", "payload", "created_at", "dispatched_at"}).
		AddRow("evt-1", "orders.created", "order-1", []byte(`{"id":"order-1"}`), now, sql.NullTime{})
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, topic, key, payload, created_at, dispatched_at FROM outbox WHERE dispatched_at IS NULL ORDER BY created_at ASC LIMIT $1`)).
		WithArgs(10).
		WillReturnRows(rows)
	messages, err := store.ListPending(context.Background(), db, 10)
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d", len(messages))
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO outbox (id, topic, key, payload, created_at, dispatched_at) VALUES ($1, $2, $3, $4, $5, NULL)`)).
		WithArgs("evt-1", "orders.created", "order-1", []byte(`{"id":"order-1"}`), now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.Add(context.Background(), db, ports.OutboxMessage{
		ID:        "evt-1",
		Topic:     "orders.created",
		Key:       "order-1",
		Payload:   []byte(`{"id":"order-1"}`),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE outbox SET dispatched_at = $2 WHERE id = $1`)).
		WithArgs("evt-1", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.MarkDispatched(context.Background(), db, "evt-1", now); err != nil {
		t.Fatalf("MarkDispatched() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}
