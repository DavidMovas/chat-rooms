package log

import (
	"context"
	"google.golang.org/grpc"
	"log/slog"
)

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that adds a logger to the context
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		attrs := []any{slog.String("method", info.FullMethod)}
		logger := slog.Default().With(attrs...)

		ctx = WithLogger(ctx, logger)
		return handler(ctx, req)
	}
}

type wrapperStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *wrapperStream) Context() context.Context {
	return s.ctx
}

// StreamServerInterceptor returns a grpc.StreamServerInterceptor that adds a logger to the context
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(src interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		attrs := []any{slog.String("method", info.FullMethod)}
		logger := slog.Default().With(attrs...)

		ctx := WithLogger(stream.Context(), logger)
		wrapped := &wrapperStream{ServerStream: stream, ctx: ctx}
		return handler(src, wrapped)
	}
}
