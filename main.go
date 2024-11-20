package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/DavidMovas/chat-rooms/internal/server"
	"github.com/redis/go-redis/v9"
)

const gracefulTimeout = time.Second * 10

func main() {
	cfg, err := config.NewConfig()
	failOrError(err, "failed to load config")

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	if cmd := rdb.Ping(shortContext()); cmd.Err() != nil {
		slog.Error("failed to connect to redis", "error", cmd.Err())
		os.Exit(1)
	}

	srv, err := server.NewServer(cfg, rdb)
	failOrError(err, "failed to create server")

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig

		ctx, cansel := context.WithTimeout(context.Background(), gracefulTimeout)
		defer cansel()

		if err = srv.Stop(ctx); err != nil {
			slog.Error("failed to stop server", "error", err)
		}
	}()

	if err = srv.Start(); err != nil {
		slog.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}

func failOrError(err error, msg string) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}

func shortContext() context.Context {
	ctx, f := context.WithTimeout(context.Background(), time.Second)
	_ = f
	return ctx
}
