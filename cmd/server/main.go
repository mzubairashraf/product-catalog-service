// Command server runs the Product Catalog gRPC service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/mzubairashraf/product-catalog-service/internal/bootstrap"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/clock"
	"github.com/mzubairashraf/product-catalog-service/internal/transport/grpcapi"
	productpb "github.com/mzubairashraf/product-catalog-service/proto/product/v1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	// SPANNER_DATABASE is the full database path. The Spanner client itself
	// honours SPANNER_EMULATOR_HOST, so the same binary runs against the
	// emulator or a real instance without a code path of its own.
	database := os.Getenv("SPANNER_DATABASE")
	if database == "" {
		return errors.New("SPANNER_DATABASE is required, e.g. projects/test-project/instances/test-instance/databases/product-catalog")
	}
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = "127.0.0.1:50051"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := spanner.NewClient(ctx, database)
	if err != nil {
		return fmt.Errorf("connecting to Spanner: %w", err)
	}
	defer client.Close()

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcapi.Recovery(logger), grpcapi.Logging(logger)))
	productpb.RegisterProductServiceServer(server, grpcapi.NewServer(bootstrap.NewApplication(client, clock.NewSystemClock()), logger))

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	reflection.Register(server) // lets grpcurl discover the API without the .proto

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() { errc <- server.Serve(listener) }()
	logger.Info("listening", slog.String("addr", listener.Addr().String()), slog.String("database", database),
		slog.Bool("emulator", os.Getenv("SPANNER_EMULATOR_HOST") != ""))

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	// Stop accepting calls and let in-flight ones finish, with a bound.
	logger.Info("shutting down")
	healthServer.Shutdown()
	done := make(chan struct{})
	go func() { server.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		server.Stop()
	}
	return nil
}
