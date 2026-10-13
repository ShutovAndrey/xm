package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	health_server "google.golang.org/grpc/health"
	health "google.golang.org/grpc/health/grpc_health_v1"

	companyv1 "gitlab.com/shutovAndrey/xm/gen/company/v1"
	"gitlab.com/shutovAndrey/xm/internal/company"
	"gitlab.com/shutovAndrey/xm/internal/db"
	"gitlab.com/shutovAndrey/xm/internal/grpcserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("application stopped with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to database")

	if err := db.Migrate(dsn); err != nil {
		return err
	}
	logger.Info("migrations applied")

	grpcServer := grpc.NewServer()
	companyRepo := company.NewRepository(pool)
	companyv1.RegisterCompanyServiceServer(grpcServer, grpcserver.NewCompanyServiceServer(companyRepo))

	healthServer := health_server.NewServer()
	health.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", health.HealthCheckResponse_SERVING)

	addr := ":" + envOr("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("grpc server started", "addr", addr)
		errCh <- grpcServer.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		healthServer.Shutdown()

		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			grpcServer.Stop()
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
