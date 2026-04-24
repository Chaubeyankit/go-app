package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ankit.chaubey/myapp/pkg/logger"
)

const (
	defaultBlockDuration = 5 * time.Second
	defaultBatchSize     = 10
	maxRetries           = 3
	dlqSuffix            = ":dlq"
)

// HandlerFunc processes a single decoded message.
// Return nil to ACK. Return an error to trigger retry.
type HandlerFunc func(ctx context.Context, msg Message) error

// Consumer reads from a Redis Stream consumer group and dispatches jobs.
type Consumer struct {
	rdb      *redis.Client
	stream   string
	group    string
	consumer string // unique per worker instance (e.g. "worker-1")
	handlers map[EventType]HandlerFunc
}

func NewConsumer(rdb *redis.Client, stream, group, consumerName string) *Consumer {
	return &Consumer{
		rdb:      rdb,
		stream:   stream,
		group:    group,
		consumer: consumerName,
		handlers: make(map[EventType]HandlerFunc),
	}
}

// Register binds an EventType to a handler function.
func (c *Consumer) Register(eventType EventType, fn HandlerFunc) {
	c.handlers[eventType] = fn
}

// EnsureGroup creates the consumer group if it doesn't exist.
// "0" means read from the very beginning on first start.
func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		// BUSYGROUP means the group already exists — that's fine
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("XGroupCreateMkStream: %w", err)
		}
	}
	return nil
}

// Run is the main poll loop. It blocks reading from the stream until ctx is cancelled.
// Call this in a goroutine; it exits cleanly when ctx is done.
func (c *Consumer) Run(ctx context.Context) {
	logger.Info("consumer started",
		zap.String("stream", c.stream),
		zap.String("group", c.group),
		zap.String("consumer", c.consumer),
	)

	// First pass: re-claim any pending messages from a previous crashed instance
	c.reclaimPending(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("consumer shutting down", zap.String("consumer", c.consumer))
			return
		default:
		}

		entries, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumer,
			Streams:  []string{c.stream, ">"}, // ">" = only new, undelivered messages
			Count:    defaultBatchSize,
			Block:    defaultBlockDuration,
			NoAck:    false,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			logger.Error("XReadGroup error",
				zap.String("stream", c.stream),
				zap.Error(err),
			)
			time.Sleep(500 * time.Millisecond) // brief pause before retry
			continue
		}

		for _, stream := range entries {
			for _, entry := range stream.Messages {
				c.processEntry(ctx, entry)
			}
		}
	}
}

func (c *Consumer) processEntry(ctx context.Context, entry redis.XMessage) {
	msg, err := decodeEntry(entry)
	if err != nil {
		logger.Error("decode stream entry failed",
			zap.String("entry_id", entry.ID),
			zap.Error(err),
		)
		// ACK malformed messages — retrying won't help
		c.ack(ctx, entry.ID)
		return
	}

	log := logger.WithContext(ctx).With(
		zap.String("event_type", string(msg.Type)),
		zap.String("entry_id", entry.ID),
		zap.Int("attempt", msg.Attempt),
	)

	handler, ok := c.handlers[msg.Type]
	if !ok {
		log.Warn("no handler registered for event type — skipping")
		c.ack(ctx, entry.ID)
		return
	}

	if err := handler(ctx, msg); err != nil {
		log.Warn("handler failed", zap.Error(err))
		c.handleFailure(ctx, entry, msg, err)
		return
	}

	c.ack(ctx, entry.ID)
	log.Info("job processed successfully")
}

func (c *Consumer) handleFailure(ctx context.Context, entry redis.XMessage, msg Message, handlerErr error) {
	if msg.Attempt >= maxRetries {
		logger.Error("job exceeded max retries — sending to DLQ",
			zap.String("entry_id", entry.ID),
			zap.String("type", string(msg.Type)),
			zap.Int("attempt", msg.Attempt),
			zap.Error(handlerErr),
		)
		c.sendToDLQ(ctx, msg, handlerErr)
		c.ack(ctx, entry.ID) // remove from the main stream
		return
	}

	// Exponential backoff: re-publish with incremented attempt count.
	// We ACK the old entry and publish a new one — this is the
	// standard pattern since Redis Streams don't natively support delayed retry.
	backoff := exponentialBackoff(msg.Attempt)
	logger.Warn("scheduling retry",
		zap.String("entry_id", entry.ID),
		zap.Int("attempt", msg.Attempt),
		zap.Duration("backoff", backoff),
	)

	go func() {
		time.Sleep(backoff)
		retryMsg := msg
		retryMsg.Attempt++
		c.republish(context.Background(), retryMsg)
	}()

	c.ack(ctx, entry.ID)
}

func (c *Consumer) ack(ctx context.Context, id string) {
	if err := c.rdb.XAck(ctx, c.stream, c.group, id).Err(); err != nil {
		logger.Error("XACK failed", zap.String("id", id), zap.Error(err))
	}
}

func (c *Consumer) republish(ctx context.Context, msg Message) {
	data, _ := json.Marshal(msg.Payload)
	_, err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: c.stream,
		MaxLen: 10_000,
		Approx: true,
		Values: map[string]interface{}{
			"type":       string(msg.Type),
			"payload":    string(data),
			"attempt":    fmt.Sprintf("%d", msg.Attempt),
			"created_at": msg.CreatedAt.Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		logger.Error("republish failed", zap.Error(err))
	}
}

func (c *Consumer) sendToDLQ(ctx context.Context, msg Message, cause error) {
	dlqStream := c.stream + dlqSuffix
	data, _ := json.Marshal(msg)
	c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqStream,
		MaxLen: 50_000,
		Approx: true,
		Values: map[string]interface{}{
			"message":   string(data),
			"error":     cause.Error(),
			"failed_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

// reclaimPending re-delivers messages that were delivered to this consumer group
// but never ACKed — happens after a crash or restart.
func (c *Consumer) reclaimPending(ctx context.Context) {
	entries, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  2 * time.Minute,
		Start:    "0-0",
		Count:    100,
	}).Result()

	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Warn("XAutoClaim error", zap.Error(err))
		return
	}

	if len(entries) > 0 {
		logger.Info("reclaimed pending messages",
			zap.Int("count", len(entries)),
			zap.String("stream", c.stream),
		)
		for _, entry := range entries {
			c.processEntry(ctx, entry)
		}
	}
}

func decodeEntry(entry redis.XMessage) (Message, error) {
	var msg Message
	msg.ID = entry.ID

	if t, ok := entry.Values["type"].(string); ok {
		msg.Type = EventType(t)
	}
	if p, ok := entry.Values["payload"].(string); ok {
		msg.Payload = json.RawMessage(p)
	}
	if a, ok := entry.Values["attempt"].(string); ok {
		fmt.Sscanf(a, "%d", &msg.Attempt)
	}
	if ts, ok := entry.Values["created_at"].(string); ok {
		msg.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	}
	if msg.Attempt == 0 {
		msg.Attempt = 1
	}
	return msg, nil
}

func exponentialBackoff(attempt int) time.Duration {
	// 2s, 4s, 8s — with ±20% jitter to avoid thundering herd
	base := time.Duration(1<<uint(attempt)) * time.Second
	jitter := time.Duration(float64(base) * 0.2)
	return base + jitter
}
