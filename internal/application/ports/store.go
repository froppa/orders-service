package ports

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/froppa/orders-service/internal/domain/orders"
)

var ErrIdempotencyKeyNotFound = errors.New("idempotency key not found")

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Transactor interface {
	DB() DBTX
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx DBTX) error) error
	Ping(ctx context.Context) error
}

type OrderRepository interface {
	Create(ctx context.Context, q DBTX, order orders.Order) error
	GetByID(ctx context.Context, q DBTX, id string) (orders.Order, error)
}

type OutboxMessage struct {
	ID           string
	Topic        string
	Key          string
	Payload      []byte
	CreatedAt    time.Time
	DispatchedAt sql.NullTime
}

type OutboxRepository interface {
	Add(ctx context.Context, q DBTX, message OutboxMessage) error
	ListPending(ctx context.Context, q DBTX, limit int) ([]OutboxMessage, error)
	MarkDispatched(ctx context.Context, q DBTX, id string, dispatchedAt time.Time) error
}

type IdempotencyRecord struct {
	Key          string
	RequestHash  string
	ResponseCode int
	ResponseBody []byte
	CreatedAt    time.Time
}

type IdempotencyRepository interface {
	Get(ctx context.Context, q DBTX, key string) (IdempotencyRecord, error)
	Reserve(ctx context.Context, q DBTX, record IdempotencyRecord) (bool, error)
	Finalize(ctx context.Context, q DBTX, key string, responseCode int, responseBody []byte) error
}
