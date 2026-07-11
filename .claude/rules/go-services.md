---
paths:
  - "**/*.go"
---

# Go service conventions

## Error handling

Define domain errors in `internal/domain/errors.go` as typed sentinel errors.
Map them to HTTP status codes in the handler layer only — never in service or repository.

```go
// domain/errors.go
var (
    ErrNotFound      = errors.New("not found")
    ErrForbidden     = errors.New("forbidden")
    ErrInvalidStatus = errors.New("invalid status transition")
    ErrConflict      = errors.New("conflict") // optimistic locking
)
```

## Handler structure

```go
func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
    // 1. Extract path/query params
    // 2. Extract JWT claims from context (middleware already validated)
    // 3. Call service
    // 4. Map domain error → HTTP status
    // 5. httputil.WriteJSON(w, http.StatusOK, resp)
}
```

## Middleware

- JWT validation: via Keycloak JWKS endpoint only — no shared secrets in services
- Roles from JWT claims: `realm_access.roles`
- `RequireRoles("appraiser", "admin")` — variadic, passes if user has any of the listed roles

## Tests

- Test the service layer using a mock repository (interface-based)
- Table-driven tests for state machine transitions — mandatory
- Do not test handlers directly unless there is a strong reason

## Kafka — transactional outbox

Never publish to Kafka from a handler/service directly — that's a dual-write and
loses events on crash. Write the event to the `outbox` table in the SAME DB
transaction as the state change; a background relay worker publishes it and marks
it sent.

```go
// Same transaction as the state change:
tx, err := pool.Begin(ctx)
// ... UPDATE requests ... (optimistic lock: WHERE status = oldStatus)
// ... INSERT INTO outbox (event_id, topic, key, payload, ...) ...
if err := tx.Commit(ctx); err != nil { return err } // both rows commit, or neither

// A separate relay worker (background goroutine) publishes:
//   SELECT ... FROM outbox WHERE published_at IS NULL
//   producer.Publish(...)  → on success: UPDATE ... SET published_at = now()
// Kafka down → events wait in outbox and retry. Crash after publish but
// before marking → republished → duplicate → consumers dedup by event_id.
```

- Broker runs in **KRaft mode** (no Zookeeper)
- Message key = aggregate id (per-aggregate ordering within a partition)
- at-least-once delivery → consumers idempotent by `event_id`
