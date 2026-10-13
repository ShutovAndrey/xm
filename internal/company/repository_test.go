package company

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/shutovAndrey/xm/internal/db/dbtest"
)

const (
	unknownID   = "00000000-0000-0000-0000-000000000000"
	malformedID = "not-a-uuid"
)

type event struct {
	Type    string
	Payload map[string]any
}

func events(t *testing.T, pool *pgxpool.Pool) []event {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT event_type, payload FROM outbox ORDER BY id`)
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (event, error) {
		var e event
		var raw json.RawMessage
		if err := r.Scan(&e.Type, &raw); err != nil {
			return e, err
		}
		return e, json.Unmarshal(raw, &e.Payload)
	})
	require.NoError(t, err)
	return out
}

func seed(t *testing.T, repo *Repository, c Company) string {
	t.Helper()
	id, _, err := repo.Create(context.Background(), c)
	require.NoError(t, err)
	return id
}

func ptr(s string) *string { return &s }

func TestRepository_Create(t *testing.T) {
	tests := []struct {
		name       string
		seed       []Company
		input      Company
		wantErrIs  error
		wantEvents int // outbox rows after the call
	}{
		{
			name:       "creates company and event",
			input:      Company{Name: "acme", Description: ptr("d"), EmployeesCount: 5, IsRegistered: true, Type: "Cooperative"},
			wantEvents: 1,
		},
		{
			name:       "duplicate name",
			seed:       []Company{{Name: "acme", Type: "Cooperative"}},
			input:      Company{Name: "acme", Type: "NonProfit"},
			wantErrIs:  ErrNameTaken,
			wantEvents: 1, // only the seed event
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := dbtest.Pool(t)
			repo := NewRepository(pool)
			for _, c := range tt.seed {
				seed(t, repo, c)
			}

			id, createdAt, err := repo.Create(context.Background(), tt.input)

			evs := events(t, pool)
			require.Len(t, evs, tt.wantEvents)
			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				return
			}
			require.NoError(t, err)
			assert.Len(t, id, 36)
			assert.WithinDuration(t, time.Now(), createdAt, time.Minute)

			got, err := repo.GetByID(context.Background(), id)
			require.NoError(t, err)
			tt.input.ID = id
			assert.Equal(t, tt.input, got)

			last := evs[len(evs)-1]
			assert.Equal(t, EventCreated, last.Type)
			assert.Equal(t, id, last.Payload["id"])
			assert.Equal(t, tt.input.Name, last.Payload["name"])
			assert.InDelta(t, float64(tt.input.EmployeesCount), last.Payload["employees_count"], 0)
		})
	}
}

func TestRepository_GetByID(t *testing.T) {
	tests := []struct {
		name      string
		id        func(seeded string) string
		wantErrIs error
	}{
		{name: "existing", id: func(s string) string { return s }},
		{name: "unknown id", id: func(string) string { return unknownID }, wantErrIs: ErrNotFound},
		{name: "malformed id", id: func(string) string { return malformedID }, wantErrIs: ErrInvalidID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(dbtest.Pool(t))
			want := Company{Name: "acme", EmployeesCount: 3, Type: "Corporations"}
			want.ID = seed(t, repo, want)

			got, err := repo.GetByID(context.Background(), tt.id(want.ID))

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestRepository_Update(t *testing.T) {
	existing := Company{Name: "acme", Description: ptr("old"), EmployeesCount: 10, IsRegistered: true, Type: "Corporations"}
	other := Company{Name: "taken", Type: "NonProfit"}

	tests := []struct {
		name      string
		id        func(seeded string) string
		upd       Company
		delta     int32
		wantErrIs error
		want      Company // state after update, ID filled in by the test
	}{
		{
			name:  "replaces fields and applies negative delta",
			id:    func(s string) string { return s },
			upd:   Company{Name: "acme2", IsRegistered: false, Type: "NonProfit"}, // Description nil clears it
			delta: -3,
			want:  Company{Name: "acme2", EmployeesCount: 7, IsRegistered: false, Type: "NonProfit"},
		},
		{
			name:  "positive delta keeps other fields as sent",
			id:    func(s string) string { return s },
			upd:   Company{Name: "acme", Description: ptr("new"), IsRegistered: true, Type: "Corporations"},
			delta: 5,
			want:  Company{Name: "acme", Description: ptr("new"), EmployeesCount: 15, IsRegistered: true, Type: "Corporations"},
		},
		{name: "unknown id", id: func(string) string { return unknownID }, upd: existing, wantErrIs: ErrNotFound},
		{name: "malformed id", id: func(string) string { return malformedID }, upd: existing, wantErrIs: ErrInvalidID},
		{
			name:      "name taken by another company",
			id:        func(s string) string { return s },
			upd:       Company{Name: other.Name, Type: "Corporations"},
			wantErrIs: ErrNameTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := dbtest.Pool(t)
			repo := NewRepository(pool)
			id := seed(t, repo, existing)
			seed(t, repo, other)
			before := len(events(t, pool))

			err := repo.Update(context.Background(), tt.id(id), tt.upd, tt.delta)

			evs := events(t, pool)
			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Len(t, evs, before, "failed update must not add an event")
				return
			}
			require.NoError(t, err)

			got, err := repo.GetByID(context.Background(), id)
			require.NoError(t, err)
			tt.want.ID = id
			assert.Equal(t, tt.want, got)

			require.Len(t, evs, before+1)
			last := evs[len(evs)-1]
			assert.Equal(t, EventUpdated, last.Type)
			assert.InDelta(t, float64(tt.want.EmployeesCount), last.Payload["employees_count"], 0, "event carries the resulting count")
			assert.Equal(t, tt.want.IsRegistered, last.Payload["is_registered"])
			assert.Contains(t, last.Payload, "description", "all fields are always present")
		})
	}
}

func TestRepository_Delete(t *testing.T) {
	tests := []struct {
		name      string
		id        func(seeded string) string
		wantErrIs error
	}{
		{name: "existing", id: func(s string) string { return s }},
		{name: "unknown id", id: func(string) string { return unknownID }, wantErrIs: ErrNotFound},
		{name: "malformed id", id: func(string) string { return malformedID }, wantErrIs: ErrInvalidID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := dbtest.Pool(t)
			repo := NewRepository(pool)
			id := seed(t, repo, Company{Name: "acme", Type: "Corporations"})

			err := repo.Delete(context.Background(), tt.id(id))

			evs := events(t, pool)
			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Len(t, evs, 1, "failed delete must not add an event")
				return
			}
			require.NoError(t, err)

			_, err = repo.GetByID(context.Background(), id)
			require.ErrorIs(t, err, ErrNotFound)

			require.Len(t, evs, 2)
			assert.Equal(t, EventDeleted, evs[1].Type)
			assert.Equal(t, map[string]any{"id": id}, evs[1].Payload, "deleted event carries only the id")
		})
	}
}
