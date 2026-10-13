package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryInterceptor requires a valid "authorization: Bearer <jwt>" metadata
// for the listed full method names and passes everything else through.
func UnaryInterceptor(verifier *JWT, protected ...string) grpc.UnaryServerInterceptor {
	methods := make(map[string]struct{}, len(protected))
	for _, m := range protected {
		methods[m] = struct{}{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := methods[info.FullMethod]; !ok {
			return handler(ctx, req)
		}

		token, ok := bearerToken(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}

		if err := verifier.Verify(token); err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		return handler(ctx, req)
	}
}

func bearerToken(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", false
	}
	const prefix = "bearer "
	if len(values[0]) <= len(prefix) || !strings.EqualFold(values[0][:len(prefix)], prefix) {
		return "", false
	}
	return values[0][len(prefix):], true
}
