package grpcserver

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "gitlab.com/shutovAndrey/xm/gen/auth/v1"
	"gitlab.com/shutovAndrey/xm/internal/auth"
	"gitlab.com/shutovAndrey/xm/internal/user"
)

type AuthServiceServer struct {
	authv1.UnimplementedAuthServiceServer

	users *user.Repository
	jwt   *auth.JWT
}

func NewAuthServiceServer(users *user.Repository, jwt *auth.JWT) *AuthServiceServer {
	return &AuthServiceServer{users: users, jwt: jwt}
}

func (s *AuthServiceServer) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	userID, err := s.users.Authenticate(ctx, req.GetLogin(), req.GetPassword())
	if errors.Is(err, user.ErrInvalidCredentials) {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	token, err := s.jwt.Issue(userID, time.Now())
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.LoginResponse{AccessToken: token}, nil
}
