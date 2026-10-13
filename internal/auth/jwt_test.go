package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func TestNewJWT(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		ttl     time.Duration
		wantErr bool
	}{
		{name: "valid", secret: testSecret, ttl: time.Hour},
		{name: "empty secret", secret: "", ttl: time.Hour, wantErr: true},
		{name: "zero ttl", secret: testSecret, ttl: 0, wantErr: true},
		{name: "negative ttl", secret: testSecret, ttl: -time.Second, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j, err := NewJWT(tt.secret, tt.ttl)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, j)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, j)
		})
	}
}

// sign builds a token outside JWT.Issue to produce malformed variants.
func sign(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func TestJWT_Verify(t *testing.T) {
	j, err := NewJWT(testSecret, time.Hour)
	require.NoError(t, err)
	now := time.Now()

	tests := []struct {
		name    string
		token   func(t *testing.T) string
		wantErr bool
	}{
		{
			name:  "issued by the same JWT",
			token: func(t *testing.T) string { s, err := j.Issue("user-1", now); require.NoError(t, err); return s },
		},
		{
			name: "expired",
			token: func(t *testing.T) string {
				s, err := j.Issue("user-1", now.Add(-2*time.Hour))
				require.NoError(t, err)
				return s
			},
			wantErr: true,
		},
		{
			name: "wrong secret",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodHS256, "other-secret", jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))})
			},
			wantErr: true,
		},
		{
			name: "unexpected signing method",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodHS512, testSecret, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))})
			},
			wantErr: true,
		},
		{
			name: "missing exp",
			token: func(t *testing.T) string {
				return sign(t, jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "user-1"})
			},
			wantErr: true,
		},
		{name: "garbage", token: func(*testing.T) string { return "not.a.token" }, wantErr: true},
		{name: "empty", token: func(*testing.T) string { return "" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := j.Verify(tt.token(t))
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidToken)
				return
			}
			require.NoError(t, err)
		})
	}
}
