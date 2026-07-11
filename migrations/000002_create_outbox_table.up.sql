-- Transactional outbox (ADR-007): domain events are written here in the SAME tx
-- as the state change. A relay worker publishes unsent rows to Kafka and stamps
-- published_at. id is monotonic so the relay can publish in insertion order.
CREATE TABLE IF NOT EXISTS outbox (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id     UUID NOT NULL UNIQUE,
    topic        TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

-- Relay poll: unpublished rows, oldest first (partial index keeps it small).
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;
