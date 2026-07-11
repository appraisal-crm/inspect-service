---
description: Adds a Kafka producer or consumer to a service. Use when wiring a service into the event bus.
disable-model-invocation: true
argument-hint: <event-name> out|in <service-name> e.g. inspect.completed out inspect-service
---

# Kafka integration: $ARGUMENTS

## Event map

| Event                    | Producer        | Consumers                              |
|--------------------------|-----------------|----------------------------------------|
| `request.created`        | request-service | notification-service                   |
| `request.status_changed` | request-service | notification-service, inspect-service  |
| `inspect.completed`      | inspect-service | request-service, notification-service  |
| `report.ready`           | review-service  | notification-service                   |

---

## If PRODUCER (out)

Never publish to Kafka from a handler/service — that's a dual-write and loses
events on crash. Write the event to the `outbox` table in the SAME tx as the
state change; a relay worker publishes it (ADR-007).

**1. `internal/domain/events.go`** — define the topic, event type and payload:
```go
const TopicInspectEvents = "inspect.events"
const EventTypeInspectCompleted = "inspect.completed"

// Payload for EventEnvelope.Data on EventTypeInspectCompleted.
type InspectCompletedData struct {
    InspectorID uuid.UUID `json:"inspector_id"`
    CompletedAt time.Time `json:"completed_at"`
    PhotoCount  int       `json:"photo_count"`
}
```

**2. Write to `outbox` in the SAME tx as the state change** — not to Kafka:
```go
tx, err := pool.Begin(ctx)
// ... UPDATE ... the aggregate ...
event := domain.NewInspectCompletedEvent(...)
// ... INSERT INTO outbox (event_id, topic, key, payload, ...) ... within tx ...
if err := tx.Commit(ctx); err != nil { return err } // both rows, or neither
```

**3. `internal/outbox/`** — relay worker publishes on a ticker via
`segmentio/kafka-go` (ADR-007), then marks rows sent:
```go
//   SELECT ... FROM outbox WHERE published_at IS NULL
//   producer.Publish(...)  → on success: UPDATE ... SET published_at = now()
// Kafka down → rows wait and retry. Crash after publish but before marking →
// republished → duplicate → consumers dedup by event_id (at-least-once).
```

---

## If CONSUMER (in)

**1. `internal/kafka/consumer.go`** — consumer group:
```go
type Consumer struct {
    reader  *kafka.Reader
    service *Service
}
func (c *Consumer) Run(ctx context.Context) error // blocking loop, exits on ctx cancel
```

**2. Idempotency** — track processed events:
```sql
CREATE TABLE processed_events (
    event_id    TEXT PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Check before processing; skip if already seen.

**3. Wire into main.go** — run in a goroutine with graceful shutdown via context cancellation.

---

## ENV variables to add in config.go

```go
KafkaBrokers:      os.Getenv("KAFKA_BROKERS"),       // e.g. "kafka:9092"
OutboxPollInterval: os.Getenv("OUTBOX_POLL_INTERVAL"), // relay tick, e.g. "1s" (producer)
KafkaGroupID:      os.Getenv("KAFKA_GROUP_ID"),      // consumer only, e.g. "inspect-service-group"
```

## Tests

- **Producer**: assert the service writes the `outbox` row in the same tx as the
  state change (mock repo / inspect the row). No publisher is called from the service.
- **Consumer**: define the handler over an interface and unit-test idempotency
  (second delivery of the same `event_id` is a no-op).
