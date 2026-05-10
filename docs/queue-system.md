# Queue System Architecture

## Overview

The queue system uses **Redis Streams** for reliable, scalable message processing. It implements a producer-consumer pattern with worker pools for concurrent processing and automatic retry mechanisms.

## Architecture Components

### 1. Producer (`pkg/queue/producer.go`)

**Purpose**: Publishes notification messages to Redis Streams

**Key Features**:
- Uses Redis `XADD` command to append messages
- Auto-generates unique message IDs
- Stream size limit (10,000 entries) for memory management
- Support for multiple event types

**Stream Constants**:
```go
const (
    StreamEmails = "stream:emails"  // Main notification stream
    StreamJobs   = "stream:jobs"    // Generic job stream (unused)
)
```

**Event Types**:
```go
const (
    EventWelcomeEmail        EventType = "email.welcome"
    EventPasswordResetEmail  EventType = "email.password_reset"
    EventPasswordChanged     EventType = "email.password_changed"
    EventLoginNotification   EventType = "email.login_notification"
)
```

**Message Structure**:
```go
type Message struct {
    ID        string          `json:"id"`         // Redis-assigned ID
    Type      EventType       `json:"type"`      // Event type
    Payload   json.RawMessage `json:"payload"`    // JSON-encoded data
    Attempt   int             `json:"attempt"`    // Retry counter
    CreatedAt time.Time       `json:"created_at"` // Timestamp
}
```

**Usage Example**:
```go
producer := queue.NewProducer(rdb)
id, err := producer.Publish(ctx, queue.StreamEmails, queue.EventWelcomeEmail, payload)
```

### 2. Consumer (`pkg/queue/consumer.go`)

**Purpose**: Reads from Redis Streams and processes messages

**Key Features**:
- Uses Redis consumer groups for load balancing
- Exponential backoff retry mechanism (2s, 4s, 8s)
- Dead Letter Queue (DLQ) for failed messages
- Crash recovery with pending message reclamation
- Multiple event handlers support

**Processing Flow**:
```go
1. XReadGroup: Fetch batch of new messages
2. decodeEntry: Parse message from Redis format
3. processEntry:
   - Look up handler for event type
   - Execute handler function
   - Handle success/failure
4. On success: XACK (acknowledge)
5. On error:
   - < max retries: republish with backoff
   - >= max retries: send to DLQ
```

**Consumer Configuration**:
```go
consumer := queue.NewConsumer(rdb, stream, group, consumerName)
consumer.Register(EventType, handlerFunc)  // Register handlers
```

**Retry Strategy**:
- Max retries: 3 attempts
- Exponential backoff: 2s, 4s, 8s with ±20% jitter
- Dead Letter Queue: `stream:emails:dlq`

### 3. Worker Pool (`pkg/queue/worker_pool.go`)

**Purpose**: Manages multiple concurrent consumers

**Key Features**:
- Creates N consumers (3 in dev, 5 in production)
- Each consumer has unique name for Redis tracking
- Graceful shutdown with `sync.WaitGroup`
- Automatic load balancing across workers

**Worker Pool Structure**:
```go
type WorkerPool struct {
    consumers []*Consumer
    wg        sync.WaitGroup
}
```

**Initialization**:
```go
// Create worker pool
emailPool := &queue.WorkerPool{}
for i := 1; i <= workerCount; i++ {
    name := fmt.Sprintf("email-worker-%d", i)
    c := queue.NewConsumer(rdb, queue.StreamEmails, "email-workers", name)
    notifHandlers.Register(c)  // Register all handlers
    emailPool.Add(c)
}

// Start all workers
emailPool.Start(ctx)
```

## Message Flow

### Complete Message Lifecycle

```
1. User Action (Login/Register)
      ↓
2. Producer.Publish()
      ↓
3. Redis Stream (stream:emails)
      ↓
4. Worker Pool Reads (XReadGroup)
      ↓
5. Consumers Process (Round-robin)
      ↓
6. Handler Executes
      ↓
7. Success → XACK
      ↓
8. Failure → Retry/DLQ
```

### Redis Stream Details

**Stream Configuration**:
- Stream name: `stream:emails`
- Consumer group: `email-workers`
- Max length: 10,000 entries
- Approx trimming for performance

**Consumer Groups**:
- Each consumer has unique name (e.g., `email-worker-1`)
- Redis automatically distributes messages
- Pending tracking per consumer
- Auto-claim for crash recovery

**Message Format**:
```json
{
  "id": "1678886400000-0",
  "type": "email.welcome",
  "payload": "{\"user_id\":1,\"email\":\"user@example.com\"}",
  "attempt": "1",
  "created_at": "2023-03-15T12:00:00Z"
}
```

## Error Handling & Recovery

### Retry Mechanism
```go
// Exponential backoff with jitter
func exponentialBackoff(attempt int) time.Duration {
    base := time.Duration(1<<uint(attempt)) * time.Second
    jitter := time.Duration(float64(base) * 0.2)
    return base + jitter
}
```

### Crash Recovery
- Uses `XAutoClaim` to reclaim idle messages
- Messages idle > 2 minutes are automatically re-delivered
- No message loss during service restarts

### Dead Letter Queue
- Messages moved after 3 failed attempts
- Contains original message + error details
- Separate stream: `stream:emails:dlq`
- Size limit: 50,000 entries

## Performance & Scalability

### Horizontal Scaling
- Add more workers to handle increased load
- Redis automatically distributes messages
- No configuration changes needed

### Throughput Characteristics
- **Batch size**: 10 messages per read
- **Block duration**: 5 seconds
- **Max retries**: 3 attempts
- **Worker count**: 3 (dev) / 5 (prod)

### Memory Management
- Stream trimmed to 10,000 entries
- DLQ trimmed to 50,000 entries
- Connection pooling for Redis

## Configuration

### Environment Variables
```yaml
# Redis configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Worker configuration
WORKER_COUNT=3
# In production: WORKER_COUNT=5
```

### Example Usage

```go
// In your application initialization
rdb := redis.NewClient(&redis.Options{
    Addr:     os.Getenv("REDIS_HOST"),
    Password: os.Getenv("REDIS_PASSWORD"),
    DB:       0,
})

// Create producer and worker pool
producer := queue.NewProducer(rdb)
workerPool := queue.NewWorkerPool(rdb, queue.StreamEmails, "email-workers", 3,
    func(name string) *queue.Consumer {
        c := queue.NewConsumer(rdb, queue.StreamEmails, "email-workers", name)
        // Register handlers
        c.Register(queue.EventWelcomeEmail, welcomeHandler)
        c.Register(queue.EventPasswordResetEmail, passwordResetHandler)
        return c
    })

// Start processing
ctx := context.Background()
workerPool.Start(ctx)
```

## Monitoring

### Key Metrics to Track
- Message processing rate
- Retry rate and success rate
- DLQ queue size
- Consumer lag (pending messages)
- Error rates by event type

### Redis Commands for Monitoring
```bash
# Stream info
XINFO stream stream:emails

# Consumer group info
XINFO GROUPS stream:emails
XINFO CONSUMERS stream:emails email-workers

# Pending messages
XPENDING stream:emails email-workers
```

---

*See [Notification System Architecture](./notification-system.md) for details on how notifications are triggered and processed.*