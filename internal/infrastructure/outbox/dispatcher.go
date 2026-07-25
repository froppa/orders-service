package outbox

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/observability"
)

type Dispatcher struct {
	Clock      ports.Clock
	Transactor ports.Transactor
	Outbox     ports.OutboxRepository
	Publisher  ports.Publisher
	Logger     *zap.Logger
	Metrics    *observability.Metrics
	Tracer     trace.Tracer
}

func NewDispatcher(clock ports.Clock, transactor ports.Transactor, outboxRepo ports.OutboxRepository, publisher ports.Publisher, logger *zap.Logger, metrics *observability.Metrics, tracer trace.Tracer) *Dispatcher {
	return &Dispatcher{
		Clock:      clock,
		Transactor: transactor,
		Outbox:     outboxRepo,
		Publisher:  publisher,
		Logger:     logger,
		Metrics:    metrics,
		Tracer:     tracer,
	}
}

func (d *Dispatcher) RunOnce(ctx context.Context) error {
	ctx, span := d.Tracer.Start(ctx, "outbox.dispatcher.run_once")
	defer span.End()

	messages, err := d.Outbox.ListPending(ctx, d.Transactor.DB(), 50)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err := d.Publisher.Publish(ctx, message.Topic, message.Key, message.Payload); err != nil {
			return err
		}
		if err := d.Outbox.MarkDispatched(ctx, d.Transactor.DB(), message.ID, d.Clock.Now().UTC()); err != nil {
			return err
		}
		d.Metrics.OutboxDispatched.Inc()
		observability.WithContext(ctx, d.Logger).Info("outbox dispatched",
			zap.String("topic", message.Topic),
			zap.String("key", message.Key),
			zap.Time("dispatched_at", d.Clock.Now().UTC()),
		)
	}
	return nil
}

type LogPublisher struct {
	Logger *zap.Logger
}

func NewLogPublisher(logger *zap.Logger) *LogPublisher {
	return &LogPublisher{Logger: logger}
}

func (p *LogPublisher) Publish(ctx context.Context, topic, key string, payload []byte) error {
	observability.WithContext(ctx, p.Logger).Info("publishing event",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.String("payload", string(payload)),
	)
	return nil
}
