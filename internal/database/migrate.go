package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

const migrationAdvisoryLockKey int64 = 884422110019

type migrationPool interface {
	Acquire(ctx context.Context) (migrationConn, error)
}

type migrationConn interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Release()
}

type dbOpener interface {
	OpenDB() *sql.DB
}

type migrateRunner interface {
	Up() error
	Close() (source error, database error)
}

type migrateFactory func(sourceURL string, db *sql.DB) (migrateRunner, error)

// pgxAcquiredConn is the subset of pgxpool.Conn used when wrapping into a
// migrationConn. The Exec return type matches pgxpool.Conn exactly so the
// interface is satisfied without an adapter layer.
type pgxAcquiredConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Release()
}

// wrapPgxConn adapts a pgxAcquiredConn (satisfied by *pgxpool.Conn) into a
// migrationConn. Extracted as a named function so the wrapping logic is
// independently testable without a live pool.
func wrapPgxConn(c pgxAcquiredConn) migrationConn {
	return &funcConn{
		execFn:    func(ctx context.Context, s string, a ...any) (any, error) { return c.Exec(ctx, s, a...) },
		releaseFn: c.Release,
	}
}

// hooks replaced in tests to avoid needing a live Postgres connection.
var (
	makePool    = prodMakePool
	makeOpener  = prodMakeOpener
	makeFactory = prodMakeFactory
)

func prodMakePool(pool *pgxpool.Pool) migrationPool { return &pgxPool{pool: &pgxPoolAdapter{pool: pool}} }
func prodMakeOpener(pool *pgxpool.Pool) dbOpener        { return &pgxOpener{pool: pool} }
// pgPostgresDriver is a subset of what postgres.WithInstance produces,
// allowing the call to be replaced in tests.
type pgPostgresDriver interface {
	database.Driver
}

// makePostgresDriver and newMigrateInstance are vars so the two steps inside
// prodMakeFactory can be exercised independently in unit tests.
var (
	makePostgresDriver = func(db *sql.DB) (database.Driver, error) {
		return postgres.WithInstance(db, &postgres.Config{})
	}
	newMigrateInstance = migrate.NewWithDatabaseInstance
)

func prodMakeFactory(src string, db *sql.DB) (migrateRunner, error) {
	drv, err := makePostgresDriver(db)
	if err != nil {
		return nil, fmt.Errorf("create postgres migration driver: %w", err)
	}
	return newMigrateInstance(src, "postgres", drv)
}

// --- pgx adapter types (named methods = coverable) ---

// rawPoolAcquirer is the subset of pgxpool.Pool used by pgxPool.Acquire.
// It returns pgxAcquiredConn so both *pgxpool.Pool and test stubs satisfy it.
type rawPoolAcquirer interface {
	Acquire(ctx context.Context) (pgxAcquiredConn, error)
}

type pgxPool struct{ pool rawPoolAcquirer }

func (p *pgxPool) Acquire(ctx context.Context) (migrationConn, error) {
	c, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return wrapPgxConn(c), nil
}

// pgxPoolAdapter wraps *pgxpool.Pool to satisfy rawPoolAcquirer.
// The adapter converts the concrete *pgxpool.Conn return to pgxAcquiredConn.
type pgxPoolAdapter struct{ pool *pgxpool.Pool }

func (a *pgxPoolAdapter) Acquire(ctx context.Context) (pgxAcquiredConn, error) {
	return a.pool.Acquire(ctx)
}

type pgxOpener struct{ pool *pgxpool.Pool }

func (o *pgxOpener) OpenDB() *sql.DB { return stdlib.OpenDBFromPool(o.pool) }

// --- public API ---

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsPath string) error {
	return doMigrate(ctx, makePool(pool), makeOpener(pool), migrationsPath, makeFactory)
}

func doMigrate(ctx context.Context, pool migrationPool, opener dbOpener, migrationsPath string, factory migrateFactory) error {
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

	db := opener.OpenDB()
	defer func() { _ = db.Close() }()

	sourceURL := fmt.Sprintf("file://%s", filepath.Clean(migrationsPath))
	runner, err := factory(sourceURL, db)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		srcErr, dbErr := runner.Close()
		if srcErr != nil && !errors.Is(srcErr, sql.ErrConnDone) {
			_ = srcErr
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

// --- test helper types ---

type funcPool struct{ acquireFn func(context.Context) (migrationConn, error) }

func (p *funcPool) Acquire(ctx context.Context) (migrationConn, error) { return p.acquireFn(ctx) }

type funcConn struct {
	execFn    func(context.Context, string, ...any) (any, error)
	releaseFn func()
}

func (c *funcConn) Exec(ctx context.Context, s string, a ...any) (any, error) {
	return c.execFn(ctx, s, a...)
}
func (c *funcConn) Release() { c.releaseFn() }

type funcOpener struct{ openFn func() *sql.DB }

func (o *funcOpener) OpenDB() *sql.DB { return o.openFn() }
