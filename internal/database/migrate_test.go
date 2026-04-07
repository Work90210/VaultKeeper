package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationAdvisoryLockKey(t *testing.T) {
	if migrationAdvisoryLockKey != 884422110019 {
		t.Fatalf("got %d", migrationAdvisoryLockKey)
	}
}

type mockRunner struct {
	upFn    func() error
	closeFn func() (error, error)
	closed  bool
}

func (m *mockRunner) Up() error {
	if m.upFn != nil {
		return m.upFn()
	}
	return nil
}
func (m *mockRunner) Close() (error, error) {
	m.closed = true
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil, nil
}

func noopDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("noopDriver", "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type noopDriver struct{}

func (noopDriver) Open(_ string) (driver.Conn, error) { return noopDriverConn{}, nil }

type noopDriverConn struct{}

func (noopDriverConn) Prepare(_ string) (driver.Stmt, error) { return nil, errors.New("noop") }
func (noopDriverConn) Close() error                          { return nil }
func (noopDriverConn) Begin() (driver.Tx, error)             { return nil, errors.New("noop") }

func init() { sql.Register("noopDriver", noopDriver{}) }

func tc() *funcConn {
	return &funcConn{
		execFn:    func(context.Context, string, ...any) (any, error) { return nil, nil },
		releaseFn: func() {},
	}
}

func hp(t *testing.T) (*funcConn, *mockRunner, migrationPool, dbOpener, migrateFactory) {
	t.Helper()
	c := tc()
	r := &mockRunner{}
	return c, r,
		&funcPool{acquireFn: func(context.Context) (migrationConn, error) { return c, nil }},
		&funcOpener{openFn: func() *sql.DB { return noopDB(t) }},
		func(string, *sql.DB) (migrateRunner, error) { return r, nil }
}

func dummyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// --- doMigrate ---

func TestDoMigrate_AcquireError(t *testing.T) {
	want := errors.New("acq")
	p := &funcPool{acquireFn: func(context.Context) (migrationConn, error) { return nil, want }}
	if err := doMigrate(context.Background(), p, &funcOpener{openFn: func() *sql.DB { return noopDB(t) }}, "/m", nil); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestDoMigrate_LockError(t *testing.T) {
	want := errors.New("lock")
	released := false
	c := &funcConn{execFn: func(context.Context, string, ...any) (any, error) { return nil, want }, releaseFn: func() { released = true }}
	p := &funcPool{acquireFn: func(context.Context) (migrationConn, error) { return c, nil }}
	if err := doMigrate(context.Background(), p, &funcOpener{openFn: func() *sql.DB { return noopDB(t) }}, "/m", nil); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if !released {
		t.Fatal("not released")
	}
}

func TestDoMigrate_FactoryError(t *testing.T) {
	_, _, p, o, _ := hp(t)
	want := errors.New("fac")
	if err := doMigrate(context.Background(), p, o, "/m", func(string, *sql.DB) (migrateRunner, error) { return nil, want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestDoMigrate_UpError(t *testing.T) {
	_, r, p, o, f := hp(t)
	want := errors.New("up")
	r.upFn = func() error { return want }
	if err := doMigrate(context.Background(), p, o, "/m", f); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestDoMigrate_ErrNoChange(t *testing.T) {
	_, r, p, o, f := hp(t)
	r.upFn = func() error { return migrate.ErrNoChange }
	if err := doMigrate(context.Background(), p, o, "/m", f); err != nil {
		t.Fatal(err)
	}
}

func TestDoMigrate_Success(t *testing.T) {
	_, r, p, o, f := hp(t)
	if err := doMigrate(context.Background(), p, o, "/m", f); err != nil {
		t.Fatal(err)
	}
	if !r.closed {
		t.Fatal("not closed")
	}
}

func TestDoMigrate_CloseErrors(t *testing.T) {
	_, r, p, o, f := hp(t)
	r.closeFn = func() (error, error) { return errors.New("s"), errors.New("d") }
	if err := doMigrate(context.Background(), p, o, "/m", f); err != nil {
		t.Fatal(err)
	}
}

func TestDoMigrate_CloseErrConnDone(t *testing.T) {
	_, r, p, o, f := hp(t)
	r.closeFn = func() (error, error) { return sql.ErrConnDone, sql.ErrConnDone }
	if err := doMigrate(context.Background(), p, o, "/m", f); err != nil {
		t.Fatal(err)
	}
}

func TestDoMigrate_SourceURL(t *testing.T) {
	_, _, p, o, _ := hp(t)
	var got string
	f := func(s string, _ *sql.DB) (migrateRunner, error) { got = s; return &mockRunner{}, nil }
	if err := doMigrate(context.Background(), p, o, "/a/../m", f); err != nil {
		t.Fatal(err)
	}
	if got != "file:///m" {
		t.Fatalf("got %q", got)
	}
}

// --- pgx adapter types ---

func TestPgxPool_AcquireError(t *testing.T) {
	p := &pgxPool{pool: dummyPool(t)}
	_, err := p.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPgxOpener_OpenDB(t *testing.T) {
	o := &pgxOpener{pool: dummyPool(t)}
	db := o.OpenDB()
	if db == nil {
		t.Fatal("nil db")
	}
	_ = db.Close()
}

func TestProdMakePool(t *testing.T) {
	p := prodMakePool(dummyPool(t))
	if p == nil {
		t.Fatal("nil")
	}
	_, err := p.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error from dummy pool")
	}
}

func TestProdMakeOpener(t *testing.T) {
	o := prodMakeOpener(dummyPool(t))
	db := o.OpenDB()
	if db == nil {
		t.Fatal("nil")
	}
	_ = db.Close()
}

func TestProdMakeFactory_Error(t *testing.T) {
	_, err := prodMakeFactory("file:///x", noopDB(t))
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- RunMigrations ---

func TestRunMigrations_Success(t *testing.T) {
	origP, origO, origF := makePool, makeOpener, makeFactory
	defer func() { makePool, makeOpener, makeFactory = origP, origO, origF }()

	_, r, p, o, f := hp(t)
	makePool = func(*pgxpool.Pool) migrationPool { return p }
	makeOpener = func(*pgxpool.Pool) dbOpener { return o }
	makeFactory = f

	if err := RunMigrations(context.Background(), nil, "/m"); err != nil {
		t.Fatal(err)
	}
	if !r.closed {
		t.Fatal("not closed")
	}
}

func TestRunMigrations_ErrorFromBogusPool(t *testing.T) {
	if err := RunMigrations(context.Background(), dummyPool(t), "/x"); err == nil {
		t.Fatal("expected error")
	}
}
