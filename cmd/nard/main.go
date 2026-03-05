package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gocools-LLC/nard.gocools/internal/apiserver"
	"github.com/gocools-LLC/nard.gocools/internal/runtime/agent"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("nard exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	addr := envOrDefault("NARD_HTTP_ADDR", ":8082")

	nodeAgent := agent.NewAgent(agent.Config{
		NodeID:            envOrDefault("NARD_NODE_ID", "node-local"),
		HeartbeatInterval: 1 * time.Second,
		Capabilities: agent.Capability{
			CPUCores: 2,
			MemoryMB: 2048,
			Labels: map[string]string{
				"role": "edge",
			},
		},
	})
	if err := nodeAgent.Start(context.Background()); err != nil {
		return err
	}
	defer nodeAgent.Stop(context.Background())

	srv := apiserver.New(apiserver.Config{
		Addr:    addr,
		Version: version,
		Logger:  logger,
		Agent:   nodeAgent,
	})

	serverErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting nard service", "addr", addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrCh:
		return err
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("nard service stopped")
	return nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
