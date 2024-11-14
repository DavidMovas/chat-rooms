package main

import (
	"context"
	"github.com/DavidMovas/chat-rooms/internal/config"
	"github.com/DavidMovas/chat-rooms/internal/server"
	"log/slog"
	"os"
	"os/signal"
)

func main() {
	cfg, err := config.NewConfig()
	failOrError(err, "failed to load config")

	srv, err := server.NewServer(cfg)
	failOrError(err, "failed to create server")

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig

		ctx, cansel := signal.NotifyContext(context.Background(), os.Interrupt)
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
