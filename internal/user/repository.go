package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidCredentials is returned for both unknown login and wrong password.
var ErrInvalidCredentials = errors.New("invalid login or password")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Authenticate checks the password in the database (pgcrypto) and returns the user id.
func (r *Repository) Authenticate(ctx context.Context, login, password string) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
          SELECT id
          FROM users
          WHERE login = $1 AND password = crypt($2, salt)`, login, password).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	return id, nil
}
