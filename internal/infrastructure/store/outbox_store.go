package store

import (
	"context"
	"database/sql"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/observability"
)

type OutboxStore struct {
	metrics *observability.Metrics
	tracer  trace.Tracer
}

func NewOutboxStore(metrics *observability.Metrics, tracer trace.Tracer) *OutboxStore {
	return &OutboxStore{metrics: metrics, tracer: tracer}
}

func (s *OutboxStore) Add(ctx context.Context, q ports.DBTX, message ports.OutboxMessage) error {
	ctx, span := s.tracer.Start(ctx, "store.outbox.add")
	defer span.End()
	_, err := q.ExecContext(ctx, `INSERT INTO outbox (id, topic, key, payload, created_at, dispatched_at) VALUES ($1, $2, $3, $4, $5, NULL)`,
		message.ID, message.Topic, message.Key, message.Payload, message.CreatedAt)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("outbox_add").Inc()
		span.RecordError(err)
	}
	return err
}

func (s *OutboxStore) ListPending(ctx context.Context, q ports.DBTX, limit int) ([]ports.OutboxMessage, error) {
	ctx, span := s.tracer.Start(ctx, "store.outbox.list_pending")
	defer span.End()
	rows, err := q.QueryContext(ctx, `SELECT id, topic, key, payload, created_at, dispatched_at FROM outbox WHERE dispatched_at IS NULL ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("outbox_list_pending").Inc()
		span.RecordError(err)
		return nil, err
	}
	defer rows.Close()

	messages := make([]ports.OutboxMessage, 0, limit)
	for rows.Next() {
		var message ports.OutboxMessage
		if err := rows.Scan(&message.ID, &message.Topic, &message.Key, &message.Payload, &message.CreatedAt, &message.DispatchedAt); err != nil {
			s.metrics.DBErrors.WithLabelValues("outbox_scan_pending").Inc()
			span.RecordError(err)
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *OutboxStore) MarkDispatched(ctx context.Context, q ports.DBTX, id string, dispatchedAt time.Time) error {
	ctx, span := s.tracer.Start(ctx, "store.outbox.mark_dispatched")
	defer span.End()
	_, err := q.ExecContext(ctx, `UPDATE outbox SET dispatched_at = $2 WHERE id = $1`, id, dispatchedAt)
	if err != nil {
		s.metrics.DBErrors.WithLabelValues("outbox_mark_dispatched").Inc()
		span.RecordError(err)
	}
	return err
}

var (
	_ = sql.NullTime{}
)
