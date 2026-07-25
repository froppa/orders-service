// orders-service exposes a synchronous command API backed by Postgres and a DB outbox.
//
// Local examples:
// curl -i http://localhost:8080/healthz
//
//	curl -i -X POST http://localhost:8080/v1/orders \
//	  -H 'Authorization: Bearer local-token' \
//	  -H 'Idempotency-Key: create-order-1' \
//	  -H 'Content-Type: application/json' \
//	  -d '{"customer_id":"cust-123","amount_cents":1299}'
//
//	curl -i http://localhost:8080/v1/orders/<order-id> \
//	  -H 'Authorization: Bearer local-token'
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/froppa/orders-service/internal/app"
	"github.com/froppa/orders-service/internal/config"
)

func main() {
	mode := flag.String("mode", "run", "Mode: run or migrate")
	flag.Parse()

	cfg := config.Load()
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.Build(rootCtx, cfg)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}
	defer application.Close()

	switch *mode {
	case "migrate":
		if err := application.Migrate(rootCtx); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	case "run":
		if err := application.Run(rootCtx); err != nil {
			log.Fatalf("run: %v", err)
		}
	default:
		log.Fatalf("unsupported mode %q", *mode)
	}
}
