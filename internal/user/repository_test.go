package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/shutovAndrey/xm/internal/db/dbtest"
)

func TestRepository_Authenticate(t *testing.T) {
	tests := []struct {
		name      string
		login     string
		password  string
		wantErrIs error
	}{
		{name: "valid credentials", login: "alice", password: "secret"},
		{name: "wrong password", login: "alice", password: "nope", wantErrIs: ErrInvalidCredentials},
		{name: "empty password", login: "alice", password: "", wantErrIs: ErrInvalidCredentials},
		{name: "unknown login", login: "bob", password: "secret", wantErrIs: ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := dbtest.Pool(t)
			var seededID string
			err := pool.QueryRow(context.Background(), `
                          INSERT INTO users (login, salt, password)
                          SELECT 'alice', s, crypt('secret', s) FROM gen_salt('bf') AS s
                          RETURNING id`).Scan(&seededID)
			require.NoError(t, err)

			id, err := NewRepository(pool).Authenticate(context.Background(), tt.login, tt.password)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Empty(t, id)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, seededID, id)
		})
	}
}
