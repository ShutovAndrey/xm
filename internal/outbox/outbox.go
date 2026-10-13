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
