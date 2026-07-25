# orders-service

A small Go service for creating and retrieving orders, built as a showcase of a
hexagonal-architecture HTTP service with an outbox-backed event dispatcher.

## Design

- **Domain / application / infrastructure split.** `internal/domain` holds the
  `Order` aggregate and its invariants, `internal/application` holds use cases
  (`commands.CreateOrder`, `queries.GetOrder`) behind port interfaces, and
  `internal/infrastructure` provides the Postgres and outbox implementations of
  those ports. `internal/transport/http` never talks to Postgres directly.
- **Transactional outbox.** Writes to the orders table and the outbox table
  happen in the same transaction (`internal/infrastructure/store`), and a
  background dispatcher (`internal/infrastructure/outbox`) polls and publishes
  outbox rows on an interval, so an order is never persisted without its event
  eventually being published.
- **Idempotent writes.** `POST /v1/orders` is guarded by an idempotency-key
  middleware (`internal/transport/http/middleware/idempotency.go`) backed by a
  dedicated store, so retried requests are safe.
- **Observability built in.** Structured logging (zap), Prometheus metrics,
  and OpenTelemetry tracing are wired through `internal/observability` and
  attached as HTTP middleware.

## API

| Method | Path              | Description                     |
|--------|-------------------|----------------------------------|
| POST   | `/v1/orders`       | Create an order (idempotent)     |
| GET    | `/v1/orders/{id}`  | Fetch an order by ID             |
| GET    | `/healthz`         | Liveness probe                   |
| GET    | `/readyz`          | Readiness probe                  |
| GET    | `/metrics`         | Prometheus metrics               |

## Running locally

```bash
docker compose up --build
```

This starts Postgres and the service on `:8080` with migrations applied on
boot.

Without Docker:

```bash
make migrate   # apply migrations against DATABASE_URL
make run
```

## Configuration

All configuration is via environment variables (see `internal/config`):

| Variable                       | Default                                                        |
|---------------------------------|-----------------------------------------------------------------|
| `SERVICE_NAME`                  | `orders-service`                                                |
| `APP_ENV`                       | `local`                                                          |
| `HTTP_ADDR`                     | `:8080`                                                          |
| `DATABASE_URL`                  | `postgres://orders:orders@localhost:5432/orders?sslmode=disable` |
| `SHUTDOWN_TIMEOUT`               | `10s`                                                            |
| `WORKER_INTERVAL`                | `2s`                                                             |
| `OTEL_EXPORTER_OTLP_ENDPOINT`    | *(unset — traces log to stdout)*                                 |
| `LOG_LEVEL`                      | `info`                                                           |

## Testing

```bash
make test
```

Runs `go test ./...` plus a coverage gate (80%) over the core packages
(excluding `cmd/` and the wiring package).
