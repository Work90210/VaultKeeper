package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/config"
	"github.com/vaultkeeper/vaultkeeper/internal/database"
	"github.com/vaultkeeper/vaultkeeper/internal/logging"
	"github.com/vaultkeeper/vaultkeeper/internal/server"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	logger := logging.NewLogger(cfg.AppEnv, cfg.LogLevel, os.Stdout)
	logger.Info("configuration loaded", "config", cfg.String())
	for _, warning := range cfg.Warnings {
		logger.Warn("configuration warning", "warning", warning)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	httpServer := server.NewHTTPServer(cfg, logger, version)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server starting", "addr", httpServer.Addr, "version", version)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped cleanly")
	return nil
}
