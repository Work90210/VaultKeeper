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

	"github.com/vaultkeeper/vaultkeeper/internal/audit"
	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/cases"
	"github.com/vaultkeeper/vaultkeeper/internal/config"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
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

	jwks := auth.NewJWKSFetcher(cfg.KeycloakURL, cfg.KeycloakRealm)
	jwksCtx, jwksCancel := context.WithTimeout(ctx, 15*time.Second)
	defer jwksCancel()
	if err := jwks.Prefetch(jwksCtx); err != nil {
		logger.Warn("JWKS prefetch failed; continuing without warm cache", "error", err)
	} else {
		logger.Info("JWKS keys prefetched successfully")
	}

	auditLogger := audit.NewLogger(pool)

	custodyRepo := custody.NewRepository(pool)
	custodyLogger := custody.NewLogger(custodyRepo)

	caseRepo := cases.NewRepository(pool)
	caseSvc, err := cases.NewService(caseRepo, custodyLogger, cfg.CaseReferenceRegex)
	if err != nil {
		return fmt.Errorf("create case service: %w", err)
	}
	caseHandler := cases.NewHandler(caseSvc, auditLogger)

	roleRepo := cases.NewRoleRepository(pool)
	roleHandler := cases.NewRoleHandler(roleRepo, custodyLogger, auditLogger)

	httpServer := server.NewHTTPServer(cfg, logger, version, jwks, auditLogger, caseHandler, roleHandler)

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
