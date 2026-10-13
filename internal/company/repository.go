package company

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, c Company) (id string, createdAt time.Time, err error) {
	err = r.pool.QueryRow(ctx, `
          INSERT INTO companies (name, description, employees_count, is_registered, type)
          VALUES ($1, $2, $3, $4, $5)
          RETURNING id, created_at`,
		c.Name, c.Description, c.EmployeesCount, c.IsRegistered, c.Type).Scan(&id, &createdAt)
	if err != nil {
		return "", time.Time{}, mapErr(err)
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

func (r *Repository) Update(ctx context.Context, id string, upd Company, employeesDelta int32) error {
	tag, err := r.pool.Exec(ctx, `
          UPDATE companies
          SET name = $2, description = $3,
              employees_count = employees_count + $4,
              is_registered = $5, type = $6
          WHERE id = $1`,
		id, upd.Name, upd.Description, employeesDelta, upd.IsRegistered, upd.Type)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM companies WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case isUniqueViolation(err):
		return ErrNameTaken
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
