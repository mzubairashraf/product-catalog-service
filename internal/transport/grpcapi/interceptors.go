package grpcapi

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Logging returns an interceptor that writes one line per call: method,
// status code and duration. Request payloads are not logged.
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	logger = orDiscard(logger)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		level := slog.LevelInfo
		if status.Code(err) == codes.Internal {
			level = slog.LevelError
		}
		logger.LogAttrs(ctx, level, "rpc",
			slog.String("method", info.FullMethod),
			slog.String("code", status.Code(err).String()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		return resp, err
	}
}

// Recovery returns an interceptor that turns a panic in a handler into an
// Internal status instead of crashing the process.
func Recovery(logger *slog.Logger) grpc.UnaryServerInterceptor {
	logger = orDiscard(logger)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				logger.ErrorContext(ctx, "panic in handler",
					slog.String("method", info.FullMethod), slog.Any("panic", p), slog.String("stack", string(debug.Stack())))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func orDiscard(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return logger
}
