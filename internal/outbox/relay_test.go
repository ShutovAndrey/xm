package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/shutovAndrey/xm/internal/db/dbtest"
)

var errBroker = errors.New("broker is down")

type sent struct {
	key     string
	payload string
	headers map[string]string
}

// fakePublisher records publishes and fails events whose type is in failTypes.
type fakePublisher struct {
	failTypes map[string]bool
	sent      []sent
}

func (f *fakePublisher) Publish(_ context.Context, key, value []byte, headers map[string]string) error {
	if f.failTypes[headers["event-type"]] {
		return errBroker
	}
	f.sent = append(f.sent, sent{key: string(key), payload: string(value), headers: headers})
	return nil
}

type row struct {
	Type      string
	Attempts  int
	LastError *string
}

func addEvents(t *testing.T, pool *pgxpool.Pool, events ...Event) {
	t.Helper()
	err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		for _, e := range events {
			if err := Add(context.Background(), tx, e); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func remaining(t *testing.T, pool *pgxpool.Pool) []row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT event_type, attempts, last_error FROM outbox ORDER BY id`)
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	require.NoError(t, err)
	return out
}

const (
	aggA = "11111111-1111-1111-1111-111111111111"
	aggB = "22222222-2222-2222-2222-222222222222"
)

func ev(agg, typ string) Event {
	return Event{AggregateID: agg, Type: typ, Payload: json.RawMessage(`{"t":"` + typ + `"}`)}
}

func TestRelay_PublishBatch(t *testing.T) {
	tests := []struct {
		name          string
		events        []Event
		failTypes     map[string]bool
		batchSize     int
		wantErrIs     error
		wantSent      []string // event types in publish order
		wantRemaining []row
	}{
		{
			name:      "empty outbox",
			batchSize: 10,
		},
		{
			name:      "publishes in id order and removes rows",
			events:    []Event{ev(aggA, "a.created"), ev(aggB, "b.created"), ev(aggA, "a.updated")},
			batchSize: 10,
			wantSent:  []string{"a.created", "b.created", "a.updated"},
		},
		{
			name:      "stops at first failure, keeps later rows untouched",
			events:    []Event{ev(aggA, "ok.1"), ev(aggA, "bad"), ev(aggA, "ok.2")},
			failTypes: map[string]bool{"bad": true},
			batchSize: 10,
			wantErrIs: errBroker,
			wantSent:  []string{"ok.1"},
			wantRemaining: []row{
				{Type: "bad", Attempts: 1, LastError: ptr(errBroker.Error())},
				{Type: "ok.2", Attempts: 0},
			},
		},
		{
			name:          "batch size limits one tick",
			events:        []Event{ev(aggA, "e.1"), ev(aggA, "e.2"), ev(aggA, "e.3")},
			batchSize:     2,
			wantSent:      []string{"e.1", "e.2"},
			wantRemaining: []row{{Type: "e.3"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := dbtest.Pool(t)
			addEvents(t, pool, tt.events...)
			pub := &fakePublisher{failTypes: tt.failTypes}
			r := NewRelay(pool, pub, time.Second, tt.batchSize, slog.Default())

			err := r.publishBatch(context.Background())

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, pub.sent, len(tt.wantSent))
			for i, s := range pub.sent {
				assert.Equal(t, tt.wantSent[i], s.headers["event-type"])
				assert.Equal(t, tt.events[i].AggregateID, s.key, "key must be the aggregate id")
				assert.JSONEq(t, string(tt.events[i].Payload), s.payload)
				assert.Len(t, s.headers["event-id"], 36, "event-id must be a uuid")
			}
			if len(tt.wantRemaining) == 0 {
				assert.Empty(t, remaining(t, pool))
			} else {
				assert.Equal(t, tt.wantRemaining, remaining(t, pool))
			}
		})
	}
}

func TestRelay_Run(t *testing.T) {
	pool := dbtest.Pool(t)
	addEvents(t, pool, ev(aggA, "a.created"))
	pub := &fakePublisher{}
	r := NewRelay(pool, pub, 10*time.Millisecond, 10, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
	assert.Len(t, pub.sent, 1, "event must be published on a tick")
	assert.Empty(t, remaining(t, pool))
}

func ptr(s string) *string { return &s }
