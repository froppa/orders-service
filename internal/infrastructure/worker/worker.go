package worker

import (
	"context"
	"errors"
	"time"

	"github.com/froppa/orders-service/internal/infrastructure/outbox"
	"github.com/froppa/orders-service/internal/observability"
	"go.uber.org/zap"
)

type Worker struct {
	Interval   time.Duration
	Dispatcher *outbox.Dispatcher
	Logger     *zap.Logger
}

func New(interval time.Duration, dispatcher *outbox.Dispatcher, logger *zap.Logger) *Worker {
	return &Worker{Interval: interval, Dispatcher: dispatcher, Logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.Dispatcher.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		observability.WithContext(ctx, w.Logger).Error("worker iteration failed", zap.Error(err))
	}

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.Dispatcher.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				observability.WithContext(ctx, w.Logger).Error("worker iteration failed", zap.Error(err))
			}
		}
	}
}
