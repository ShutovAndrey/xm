package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Publisher interface {
	Publish(ctx context.Context, key, value []byte, headers map[string]string) error
}

// Relay publishes pending outbox rows in id order and deletes them once the
// broker has acknowledged. Delivery is at-least-once: a crash between publish
// and commit re-sends the batch, consumers dedup by the event-id header.
type Relay struct {
	pool      *pgxpool.Pool
	publisher Publisher
	interval  time.Duration
	batchSize int
	logger    *slog.Logger
}

func NewRelay(pool *pgxpool.Pool, publisher Publisher, interval time.Duration, batchSize int, logger *slog.Logger) *Relay {
	return &Relay{pool: pool, publisher: publisher, interval: interval, batchSize: batchSize, logger: logger}
}

// Run publishes one batch per tick until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.publishBatch(ctx); err != nil && ctx.Err() == nil {
				r.logger.Error("outbox relay", "err", err)
			}
		}
	}
}

// publishBatch locks a batch of rows, publishes them in order and deletes the
// published ones. On the first failure it records the error on that row and
// stops to keep per-aggregate order; the deletes made so far are still
// committed and the row is retried on the next tick.
func (r *Relay) publishBatch(ctx context.Context) error {
	var publishErr error

	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		events, err := lockPending(ctx, tx, r.batchSize)
		if err != nil {
			return err
		}
		for _, e := range events {
			headers := map[string]string{"event-type": e.Type, "event-id": e.EventID}
			if err := r.publisher.Publish(ctx, []byte(e.AggregateID), e.Payload, headers); err != nil {
				publishErr = fmt.Errorf("publish event %d (%s): %w", e.ID, e.Type, err)
				return markFailed(ctx, tx, e.ID, err)
			}
			if err := remove(ctx, tx, e.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return publishErr
}
