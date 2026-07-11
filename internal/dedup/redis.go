package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "inspect:dedup:"

// Deduplicator gives the Kafka consumer idempotency. The first time an event_id
// is seen it is marked in Redis (SET NX EX); a redelivery of the same event finds
// the key and is skipped. No DB inbox table — at-least-once delivery + this.
type Deduplicator struct {
	rdb *redis.Client
	ttl time.Duration
}

func New(rdb *redis.Client, ttl time.Duration) *Deduplicator {
	return &Deduplicator{rdb: rdb, ttl: ttl}
}

// Seen atomically marks eventID as processed and reports whether it was already
// marked. A true result means "duplicate — skip".
func (d *Deduplicator) Seen(ctx context.Context, eventID string) (bool, error) {
	ok, err := d.rdb.SetNX(ctx, keyPrefix+eventID, 1, d.ttl).Result()
	if err != nil {
		return false, err
	}
	// SetNX returns true when the key was set (i.e. NOT seen before).
	return !ok, nil
}

// Forget removes the mark so the event reprocesses on redelivery. Call it when
// processing failed after Seen already claimed the id.
func (d *Deduplicator) Forget(ctx context.Context, eventID string) error {
	if err := d.rdb.Del(ctx, keyPrefix+eventID).Err(); err != nil {
		return fmt.Errorf("forget dedup key: %w", err)
	}
	return nil
}
