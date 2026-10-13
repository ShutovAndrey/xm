package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Event struct {
	AggregateID string
	Type        string
	Payload     json.RawMessage
}

// Add stores the event in the transaction that makes the change it describes.
func Add(ctx context.Context, tx pgx.Tx, e Event) error {
	_, err := tx.Exec(ctx, `
          INSERT INTO outbox (aggregate_id, event_type, payload)
          VALUES ($1, $2, $3)`,
		e.AggregateID, e.Type, e.Payload)
	if err != nil {
		return fmt.Errorf("add outbox event: %w", err)
	}
	return nil
}

// pending is an outbox row waiting to be published.
type pending struct {
	ID          int64
	EventID     string
	AggregateID string
	Type        string
	Payload     json.RawMessage
}

func lockPending(ctx context.Context, tx pgx.Tx, limit int) ([]pending, error) {
	rows, err := tx.Query(ctx, `
          SELECT id, event_id, aggregate_id, event_type, payload
          FROM outbox
          ORDER BY id
          LIMIT $1
          FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("select outbox: %w", err)
	}
	events, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pending])
	if err != nil {
		return nil, fmt.Errorf("scan outbox: %w", err)
	}
	return events, nil
}

func markFailed(ctx context.Context, tx pgx.Tx, id int64, cause error) error {
	_, err := tx.Exec(ctx, `
          UPDATE outbox SET attempts = attempts + 1, last_error = $2
          WHERE id = $1`, id, cause.Error())
	if err != nil {
		return fmt.Errorf("record outbox failure: %w", err)
	}
	return nil
}

func remove(ctx context.Context, tx pgx.Tx, id int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM outbox WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete outbox row: %w", err)
	}
	return nil
}
