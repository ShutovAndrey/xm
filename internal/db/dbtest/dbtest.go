// Package dbtest gives integration tests an isolated, fully migrated Postgres
// schema. Tests are skipped when TEST_DATABASE_URL is not set.
package dbtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"gitlab.com/shutovAndrey/xm/internal/db"
)

// Pool creates a fresh schema on TEST_DATABASE_URL, applies all migrations to
// it and returns a pool whose search_path points there. The schema is dropped
// when the test finishes, so tests and packages can run in parallel.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("test_%d_%d", os.Getpid(), time.Now().UnixNano())

	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
		require.NoError(t, admin.Close(context.Background()))
	})

	// Extensions are database-wide; create pgcrypto once in public so the
	// migration's CREATE EXTENSION IF NOT EXISTS is a no-op in every schema.
	_, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pgcrypto SCHEMA public")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	schemaDSN := u.String()

	require.NoError(t, db.Migrate(schemaDSN))

	pool, err := db.Connect(ctx, schemaDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
