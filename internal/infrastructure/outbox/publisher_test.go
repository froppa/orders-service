package outbox

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestLogPublisher(t *testing.T) {
	publisher := NewLogPublisher(zap.NewNop())
	if err := publisher.Publish(context.Background(), "orders.created", "order-1", []byte(`{"id":"order-1"}`)); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
}
