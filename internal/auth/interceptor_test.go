package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryInterceptor(t *testing.T) {
	const (
		protected   = "/svc.v1.Service/Protected"
		unprotected = "/svc.v1.Service/Public"
	)
	j, err := NewJWT(testSecret, time.Hour)
	require.NoError(t, err)
	valid, err := j.Issue("user-1", time.Now())
	require.NoError(t, err)
	expired, err := j.Issue("user-1", time.Now().Add(-2*time.Hour))
	require.NoError(t, err)

	tests := []struct {
		name     string
		method   string
		md       metadata.MD // nil = no incoming metadata at all
		wantCode codes.Code  // codes.OK means the handler must run
	}{
		{name: "unprotected method without token", method: unprotected, wantCode: codes.OK},
		{name: "unprotected method ignores garbage token", method: unprotected, md: metadata.Pairs("authorization", "Bearer garbage"), wantCode: codes.OK},
		{name: "protected without metadata", method: protected, wantCode: codes.Unauthenticated},
		{name: "protected without authorization header", method: protected, md: metadata.Pairs("x-other", "1"), wantCode: codes.Unauthenticated},
		{name: "wrong scheme", method: protected, md: metadata.Pairs("authorization", "Basic "+valid), wantCode: codes.Unauthenticated},
		{name: "bearer without token", method: protected, md: metadata.Pairs("authorization", "Bearer "), wantCode: codes.Unauthenticated},
		{name: "invalid token", method: protected, md: metadata.Pairs("authorization", "Bearer garbage"), wantCode: codes.Unauthenticated},
		{name: "expired token", method: protected, md: metadata.Pairs("authorization", "Bearer "+expired), wantCode: codes.Unauthenticated},
		{name: "valid token", method: protected, md: metadata.Pairs("authorization", "Bearer "+valid), wantCode: codes.OK},
		{name: "scheme is case-insensitive", method: protected, md: metadata.Pairs("authorization", "bearer "+valid), wantCode: codes.OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			called := false
			handler := func(context.Context, any) (any, error) {
				called = true
				return "ok", nil
			}

			resp, err := UnaryInterceptor(j, protected)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: tt.method}, handler)

			if tt.wantCode == codes.OK {
				require.NoError(t, err)
				assert.True(t, called, "handler must be called")
				assert.Equal(t, "ok", resp)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err))
			assert.False(t, called, "handler must not be called")
			assert.Nil(t, resp)
		})
	}
}
