package company

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.com/shutovAndrey/xm/internal/outbox"
)

const (
	EventCreated = "company.created"
	EventUpdated = "company.updated"
	EventDeleted = "company.deleted"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts the company and a company.created event in one transaction.
func (r *Repository) Create(ctx context.Context, c Company) (id string, createdAt time.Time, err error) {
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
                  INSERT INTO companies (name, description, employees_count, is_registered, type)
                  VALUES ($1, $2, $3, $4, $5)
                  RETURNING id, created_at`,
			c.Name, c.Description, c.EmployeesCount, c.IsRegistered, c.Type).Scan(&id, &createdAt)
		if err != nil {
			return mapErr(err)
		}
		c.ID = id
		return addEvent(ctx, tx, EventCreated, id, companyPayload(c))
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return id, createdAt, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (Company, error) {
	row := r.pool.QueryRow(ctx, `
          SELECT id, name, description, employees_count, is_registered, type
          FROM companies
          WHERE id = $1`, id)

	var c Company
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.EmployeesCount, &c.IsRegistered, &c.Type)
	if err != nil {
		return Company{}, mapErr(err)
	}
	return c, nil
}

// Update replaces the company and records a company.updated event with the
// resulting state (employees_count after applying the delta).
func (r *Repository) Update(ctx context.Context, id string, upd Company, employeesDelta int32) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var c Company
		err := tx.QueryRow(ctx, `
                  UPDATE companies
                  SET name = $2, description = $3,
                      employees_count = employees_count + $4,
                      is_registered = $5, type = $6
                  WHERE id = $1
                  RETURNING id, name, description, employees_count, is_registered, type`,
			id, upd.Name, upd.Description, employeesDelta, upd.IsRegistered, upd.Type).
			Scan(&c.ID, &c.Name, &c.Description, &c.EmployeesCount, &c.IsRegistered, &c.Type)
		if err != nil {
			return mapErr(err)
		}
		return addEvent(ctx, tx, EventUpdated, c.ID, companyPayload(c))
	})
}

// Delete removes the company and records a company.deleted event with its id.
func (r *Repository) Delete(ctx context.Context, id string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM companies WHERE id = $1`, id)
		if err != nil {
			return mapErr(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return addEvent(ctx, tx, EventDeleted, id, deletedPayload{ID: id})
	})
}

// companyPayload is the full company state published with created/updated events.
// Field order and types must match Company so it stays convertible.
type companyPayload struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	EmployeesCount int32   `json:"employees_count"`
	IsRegistered   bool    `json:"is_registered"`
	Type           Type    `json:"type"`
}

type deletedPayload struct {
	ID string `json:"id"`
}

func addEvent(ctx context.Context, tx pgx.Tx, eventType, aggregateID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	return outbox.Add(ctx, tx, outbox.Event{AggregateID: aggregateID, Type: eventType, Payload: data})
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case isPgCode(err, pgerrcode.UniqueViolation):
		return ErrNameTaken
	case isPgCode(err, pgerrcode.InvalidTextRepresentation):
		return ErrInvalidID
	}
	return err
}

func isPgCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
