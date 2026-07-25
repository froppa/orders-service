package ports

import "context"

type Publisher interface {
	Publish(ctx context.Context, topic, key string, payload []byte) error
}
