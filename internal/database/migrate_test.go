package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratedatabase "github.com/golang-migrate/migrate/v4/database"
	"github.com/jackc/pgx/v5/pgconn"
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

// stubRawPoolAcquirer implements rawPoolAcquirer without a real Postgres pool.
type stubRawPoolAcquirer struct {
	conn pgxAcquiredConn
	err  error
}

func (s *stubRawPoolAcquirer) Acquire(_ context.Context) (pgxAcquiredConn, error) {
	return s.conn, s.err
}

// TestPgxPool_AcquireSuccess exercises the pgxPool.Acquire success path using a
// stub rawPoolAcquirer so no live Postgres connection is needed.
func TestPgxPool_AcquireSuccess(t *testing.T) {
	stub := &stubPgxAcquiredConn{}
	p := &pgxPool{pool: &stubRawPoolAcquirer{conn: stub}}

	conn, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil migrationConn")
	}

	// Verify the returned conn delegates correctly.
	_, execErr := conn.Exec(context.Background(), "SELECT 1")
	if execErr != nil {
		t.Fatalf("Exec: %v", execErr)
	}
	conn.Release()
	if !stub.released {
		t.Fatal("Release was not forwarded")
	}
}

func TestPgxPool_AcquireError(t *testing.T) {
	want := errors.New("acquire-error")
	p := &pgxPool{pool: &stubRawPoolAcquirer{err: want}}
	_, err := p.Acquire(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestPgxPool_AcquireError_LivePool(t *testing.T) {
	p := prodMakePool(dummyPool(t))
	_, err := p.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error from unreachable dummy pool")
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

// TestProdMakeFactory_DriverSuccessNewInstanceError stubs makePostgresDriver to
// succeed (bypassing the need for a live DB) so that the newMigrateInstance
// call on line 88 of migrate.go is reached. This covers the previously 0%
// success-path statement in prodMakeFactory.
func TestProdMakeFactory_DriverSuccessNewInstanceError(t *testing.T) {
	origDriver := makePostgresDriver
	origInstance := newMigrateInstance
	defer func() {
		makePostgresDriver = origDriver
		newMigrateInstance = origInstance
	}()

	wantErr := errors.New("new-instance-error")
	makePostgresDriver = func(_ *sql.DB) (migratedatabase.Driver, error) {
		return nil, nil // simulate success; drv value unused by newMigrateInstance stub
	}
	newMigrateInstance = func(src, dbname string, drv migratedatabase.Driver) (*migrate.Migrate, error) {
		return nil, wantErr
	}

	_, err := prodMakeFactory("file:///x", noopDB(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// --- wrapPgxConn ---

// stubPgxAcquiredConn implements pgxAcquiredConn without a real Postgres connection.
type stubPgxAcquiredConn struct {
	tag      pgconn.CommandTag
	execErr  error
	released bool
}

func (s *stubPgxAcquiredConn) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return s.tag, s.execErr
}
func (s *stubPgxAcquiredConn) Release() { s.released = true }

// TestWrapPgxConn_Exec confirms that wrapPgxConn delegates Exec to the underlying conn.
func TestWrapPgxConn_Exec(t *testing.T) {
	stub := &stubPgxAcquiredConn{}
	conn := wrapPgxConn(stub)

	_, err := conn.Exec(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestWrapPgxConn_ExecError confirms that wrapPgxConn propagates errors from Exec.
func TestWrapPgxConn_ExecError(t *testing.T) {
	want := errors.New("exec failed")
	stub := &stubPgxAcquiredConn{execErr: want}
	conn := wrapPgxConn(stub)

	_, err := conn.Exec(context.Background(), "SELECT 1")
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestWrapPgxConn_Release confirms that wrapPgxConn delegates Release to the underlying conn.
func TestWrapPgxConn_Release(t *testing.T) {
	stub := &stubPgxAcquiredConn{}
	conn := wrapPgxConn(stub)

	conn.Release()

	if !stub.released {
		t.Fatal("Release was not forwarded to the underlying conn")
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
