package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamName constants — single source of truth for all stream names.
const (
	StreamEmails = "stream:emails"
	StreamJobs   = "stream:jobs"
)

// EventType identifies what kind of job a message represents.
type EventType string

const (
	EventWelcomeEmail        EventType = "email.welcome"
	EventPasswordResetEmail  EventType = "email.password_reset"
	EventPasswordChanged     EventType = "email.password_changed"
	EventLoginNotification   EventType = "email.login_notification"
)

// Message is the envelope published to a stream.
// Payload is JSON-encoded so each handler can decode into its own struct.
type Message struct {
	ID        string          `json:"id"` // Redis-assigned stream entry ID
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Attempt   int             `json:"attempt"` // incremented on each retry
	CreatedAt time.Time       `json:"created_at"`
}

// Producer publishes jobs to Redis Streams.
type Producer struct {
	rdb *redis.Client
}

func NewProducer(rdb *redis.Client) *Producer {
	return &Producer{rdb: rdb}
}

// Publish serialises payload and appends it to the named stream.
// It returns the Redis stream entry ID for tracing.
func (p *Producer) Publish(ctx context.Context, stream string, eventType EventType, payload interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("queue.Publish marshal: %w", err)
	}

	// Redis XADD — "*" means auto-generate the entry ID.
	// MaxLen trims the stream to ~10 000 entries to bound memory.
	id, err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: 10_000,
		Approx: true, // "~10000" — faster, slight overshoot is fine
		Values: map[string]interface{}{
			"type":       string(eventType),
			"payload":    string(data),
			"attempt":    "1",
			"created_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()

	if err != nil {
		return "", fmt.Errorf("queue.Publish XADD %s: %w", stream, err)
	}
	return id, nil
}
