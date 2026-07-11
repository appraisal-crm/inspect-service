package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// batchSize caps how many rows one poll publishes, bounding work per tick.
const batchSize = 100

// Relay is the polling publisher: on each tick it reads unpublished outbox rows
// and publishes them to Kafka, marking each sent on success.
type Relay struct {
	db       *pgxpool.Pool
	producer *Producer
	interval time.Duration
}

func NewRelay(db *pgxpool.Pool, producer *Producer, interval time.Duration) *Relay {
	return &Relay{db: db, producer: producer, interval: interval}
}

// Run polls until ctx is cancelled. Blocking; call it in a goroutine.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.publishBatch(ctx); err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "outbox relay batch failed", "error", err)
			}
		}
	}
}

type outboxRow struct {
	id      int64
	topic   string
	key     string
	payload []byte
}

func (r *Relay) publishBatch(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `
		SELECT id, topic, aggregate_id, payload
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1
	`, batchSize)
	if err != nil {
		return err
	}

	batch := make([]outboxRow, 0, batchSize)
	for rows.Next() {
		var row outboxRow
		var aggregateID uuid.UUID
		if err := rows.Scan(&row.id, &row.topic, &aggregateID, &row.payload); err != nil {
			rows.Close()
			return err
		}
		row.key = aggregateID.String()
		batch = append(batch, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Publish in id order so per-aggregate ordering is preserved. On a publish
	// error we stop; the remaining rows stay unpublished and retry next tick.
	for _, row := range batch {
		if err := r.producer.Publish(ctx, row.topic, row.key, row.payload); err != nil {
			return err
		}
		if _, err := r.db.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE id = $1`, row.id); err != nil {
			return err
		}
	}
	return nil
}
