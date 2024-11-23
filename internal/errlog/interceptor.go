package errlog

import (
	"context"
	"github.com/DavidMovas/chat-rooms/internal/log"
	"google.golang.org/grpc"
)

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that logs errors.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		res, err := handler(ctx, req)
		if err != nil {
			logger := log.FromContext(ctx)
			logger.Error("failed to handle request", "method", info.FullMethod, "error", err)
		}
		return res, err
	}
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor that logs errors.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(src interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(src, stream)
		if err != nil {
			logger := log.FromContext(stream.Context())
			logger.Error("failed to handle request", "method", info.FullMethod, "error", err)
		}
		return err
	}
}
