package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/DavidMovas/chat-rooms/internal/errlog"

	"github.com/DavidMovas/chat-rooms/apis/chat"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/DavidMovas/chat-rooms/internal/log"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	cfg        *config.Config
	rdb        *redis.Client
	listener   net.Listener
	grpcServer *grpc.Server
	closers    []func() error
}

func NewServer(cfg *config.Config, rdb *redis.Client) (*Server, error) {
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			log.UnaryServerInterceptor(),
			errlog.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			log.StreamServerInterceptor(),
			errlog.StreamServerInterceptor(),
		),
	)

	s := NewStorage(rdb, cfg)
	h := NewChatServer(s, cfg)
	chat.RegisterChatServiceServer(grpcServer, h)
	if cfg.Local {
		reflection.Register(grpcServer)
	}

	return &Server{
		cfg:        cfg,
		grpcServer: grpcServer,
		rdb:        rdb,
	}, nil
}

func (s *Server) Start() error {
	logger, err := log.SetupLogger(s.cfg.Local, s.cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}

	slog.SetDefault(logger)

	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.closers = append(s.closers, s.listener.Close, s.rdb.Close)

	logger.Info("server started", "port", s.cfg.Port)
	return s.grpcServer.Serve(s.listener)
}

func (s *Server) Stop(ctx context.Context) error {
	stopped := make(chan struct{})

	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.Stop()
	case <-stopped:
	}

	return withClosers(s.closers, nil)
}

func (s *Server) Port() (int, error) {
	if s.listener == nil || s.listener.Addr() == nil {
		return 0, fmt.Errorf("server is not running")
	}

	return s.listener.Addr().(*net.TCPAddr).Port, nil
}

func withClosers(closers []func() error, err error) error {
	errs := []error{err}

	for i := len(closers) - 1; i >= 0; i-- {
		if err = closers[i](); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
