package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

const migrationAdvisoryLockKey int64 = 884422110019

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsPath string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey)
	}()

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migration driver: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", filepath.Clean(migrationsPath))
	runner, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		sourceErr, dbErr := runner.Close()
		if sourceErr != nil && !errors.Is(sourceErr, sql.ErrConnDone) {
			_ = sourceErr
		}
		if dbErr != nil && !errors.Is(dbErr, sql.ErrConnDone) {
			_ = dbErr
		}
	}()

	if err := runner.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
