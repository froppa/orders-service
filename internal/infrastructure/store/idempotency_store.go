package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"go.opentelemetry.io/otel/trace"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/infrastructure/db"
	"github.com/froppa/orders-service/internal/observability"
)

type IdempotencyStore struct {
	metrics *observability.Metrics
	tracer  trace.Tracer
}

func NewIdempotencyStore(metrics *observability.Metrics, tracer trace.Tracer) *IdempotencyStore {
	return &IdempotencyStore{metrics: metrics, tracer: tracer}
}

func (s *IdempotencyStore) Get(ctx context.Context, q ports.DBTX, key string) (ports.IdempotencyRecord, error) {
	ctx, span := s.tracer.Start(ctx, "store.idempotency.get")
	defer span.End()
	var record ports.IdempotencyRecord
	var responseBody []byte
	err := q.QueryRowContext(ctx, `SELECT key, request_hash, response_code, response_body, created_at FROM idempotency_keys WHERE key = $1`, key).
		Scan(&record.Key, &record.RequestHash, &record.ResponseCode, &responseBody, &record.CreatedAt)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("idempotency_get").Inc()
		if err == sql.ErrNoRows {
			return ports.IdempotencyRecord{}, ports.ErrIdempotencyKeyNotFound
		}
		span.RecordError(err)
		return ports.IdempotencyRecord{}, err
	}
	record.ResponseBody = responseBody
	return record, nil
}

func (s *IdempotencyStore) Reserve(ctx context.Context, q ports.DBTX, record ports.IdempotencyRecord) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "store.idempotency.reserve")
	defer span.End()
	body := record.ResponseBody
	if len(body) == 0 {
		body = []byte(`{}`)
	}
	if !json.Valid(body) {
		body = []byte(`{}`)
	}
	_, err := q.ExecContext(ctx, `INSERT INTO idempotency_keys (key, request_hash, response_code, response_body, created_at) VALUES ($1, $2, $3, $4, $5)`,
		record.Key, record.RequestHash, record.ResponseCode, body, record.CreatedAt.UTC())
	if err != nil {
		if db.IsUniqueViolation(err) {
			return false, nil
		}
		s.metrics.DBErrors.WithLabelValues("idempotency_reserve").Inc()
		span.RecordError(err)
		return false, err
	}
	return true, nil
}

func (s *IdempotencyStore) Finalize(ctx context.Context, q ports.DBTX, key string, responseCode int, responseBody []byte) error {
	ctx, span := s.tracer.Start(ctx, "store.idempotency.finalize")
	defer span.End()
	if !json.Valid(responseBody) {
		responseBody = []byte(`{"error":{"code":"invalid_snapshot","message":"response body was not valid json"}}`)
	}
	_, err := q.ExecContext(ctx, `UPDATE idempotency_keys SET response_code = $2, response_body = $3 WHERE key = $1`, key, responseCode, responseBody)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("idempotency_finalize").Inc()
		span.RecordError(err)
	}
	return err
}
