package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// pgx fakes
// ---------------------------------------------------------------------------

type fakeRow struct {
	scanErr error
	values  []any
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		if i >= len(r.values) {
			break
		}
		if p, ok := d.(*string); ok {
			if s, ok := r.values[i].(string); ok {
				*p = s
			}
		}
	}
	return nil
}

type fakeTx struct {
	pgx.Tx

	execCallCount int
	execErr       error
	execErrOnCall int // 1-based; 0 means always error if execErr set
	queryRow      pgx.Row
	commitErr     error
}

func (t *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	t.execCallCount++
	if t.execErrOnCall > 0 && t.execCallCount == t.execErrOnCall {
		return pgconn.CommandTag{}, t.execErr
	}
	if t.execErrOnCall == 0 && t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if t.queryRow != nil {
		return t.queryRow
	}
	return &fakeRow{scanErr: pgx.ErrNoRows}
}

func (t *fakeTx) Commit(_ context.Context) error  { return t.commitErr }
func (t *fakeTx) Rollback(_ context.Context) error { return nil }
func (t *fakeTx) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}
func (t *fakeTx) Conn() *pgx.Conn { return nil }

type fakePool struct {
	beginTxErr error
	tx         *fakeTx
}

func (p *fakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	if p.beginTxErr != nil {
		return nil, p.beginTxErr
	}
	return p.tx, nil
}

// newTestLogger creates a Logger with a fakePool for unit testing.
func newTestLogger(pool *fakePool) *Logger {
	return &Logger{pool: pool}
}

type fakeQueryRower struct {
	row pgx.Row
}

func (q *fakeQueryRower) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return q.row
}

// ---------------------------------------------------------------------------
// getLastHash tests (accepts queryRower interface)
// ---------------------------------------------------------------------------

func TestGetLastHash_NoRows(t *testing.T) {
	row := &fakeRow{scanErr: pgx.ErrNoRows}
	qr := &fakeQueryRower{row: row}

	hash, err := getLastHash(context.Background(), qr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "" {
		t.Errorf("hash = %q, want empty string for no rows", hash)
	}
}

func TestGetLastHash_Success(t *testing.T) {
	row := &fakeRow{values: []any{"abc123"}}
	qr := &fakeQueryRower{row: row}

	hash, err := getLastHash(context.Background(), qr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "abc123" {
		t.Errorf("hash = %q, want %q", hash, "abc123")
	}
}

func TestGetLastHash_ScanError(t *testing.T) {
	injected := errors.New("scan error")
	row := &fakeRow{scanErr: injected}
	qr := &fakeQueryRower{row: row}

	_, err := getLastHash(context.Background(), qr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

// ---------------------------------------------------------------------------
// computeHash additional tests
// ---------------------------------------------------------------------------

func TestComputeHash_EmptyInputs(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := computeHash("", "", "", "", ts)
	if h == "" {
		t.Error("hash should not be empty for empty inputs")
	}
	if len(h) != 64 {
		t.Errorf("hash length = %d, want 64", len(h))
	}
}

func TestComputeHash_TimeDifference(t *testing.T) {
	ts1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)

	h1 := computeHash("login", "user-1", "{}", "", ts1)
	h2 := computeHash("login", "user-1", "{}", "", ts2)

	if h1 == h2 {
		t.Error("different timestamps should produce different hashes")
	}
}

func TestComputeHash_DetailDifference(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	h1 := computeHash("login", "user-1", `{"a":"1"}`, "", ts)
	h2 := computeHash("login", "user-1", `{"b":"2"}`, "", ts)

	if h1 == h2 {
		t.Error("different detail should produce different hashes")
	}
}

// ---------------------------------------------------------------------------
// NewLogger test
// ---------------------------------------------------------------------------

func TestNewLogger(t *testing.T) {
	l := NewLogger((*pgxpool.Pool)(nil))
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
}

// ---------------------------------------------------------------------------
// LogAuthEvent tests — using fakePool
// ---------------------------------------------------------------------------

func TestLogAuthEvent_Success_NoRows(t *testing.T) {
	tx := &fakeTx{
		queryRow: &fakeRow{scanErr: pgx.ErrNoRows}, // no previous hash
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		IPAddress:   "10.0.0.1",
		UserAgent:   "test-agent",
		Detail:      map[string]string{"method": "password"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogAuthEvent_Success_WithPreviousHash(t *testing.T) {
	tx := &fakeTx{
		queryRow: &fakeRow{values: []any{"prev-hash-abc"}},
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "logout",
		ActorUserID: "user-2",
		IPAddress:   "192.168.1.1",
		Detail:      map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogAuthEvent_MarshalError(t *testing.T) {
	original := marshalJSON
	t.Cleanup(func() { marshalJSON = original })

	injected := errors.New("marshal error")
	marshalJSON = func(_ any) ([]byte, error) { return nil, injected }

	pool := &fakePool{tx: &fakeTx{queryRow: &fakeRow{scanErr: pgx.ErrNoRows}}}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{"key": "value"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_BeginTxError(t *testing.T) {
	injected := errors.New("begin tx error")
	pool := &fakePool{beginTxErr: injected}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_AdvisoryLockError(t *testing.T) {
	injected := errors.New("lock error")
	tx := &fakeTx{execErr: injected} // first Exec (advisory lock) fails
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_GetLastHashError(t *testing.T) {
	injected := errors.New("query error")
	tx := &fakeTx{
		queryRow: &fakeRow{scanErr: injected}, // non-ErrNoRows
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_InsertError(t *testing.T) {
	injected := errors.New("insert error")
	tx := &fakeTx{
		execErr:       injected,
		execErrOnCall: 2, // advisory lock succeeds (call 1), INSERT fails (call 2)
		queryRow:      &fakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_CommitError(t *testing.T) {
	injected := errors.New("commit error")
	tx := &fakeTx{
		commitErr: injected,
		queryRow:  &fakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogAuthEvent_NilDetail(t *testing.T) {
	tx := &fakeTx{
		queryRow: &fakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	err := l.LogAuthEvent(context.Background(), AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		Detail:      nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LogAccessDenied tests
// ---------------------------------------------------------------------------

func TestLogAccessDenied_Success(t *testing.T) {
	tx := &fakeTx{
		queryRow: &fakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &fakePool{tx: tx}
	l := newTestLogger(pool)

	// Should not panic and should call LogAuthEvent internally
	l.LogAccessDenied(context.Background(), "user-1", "/api/cases", "admin", "user", "10.0.0.1")
}

func TestLogAccessDenied_InternalError(t *testing.T) {
	pool := &fakePool{beginTxErr: errors.New("tx error")}
	l := newTestLogger(pool)

	// LogAccessDenied swallows the error (assigns to _), should not panic
	l.LogAccessDenied(context.Background(), "user-1", "/api/cases", "admin", "user", "10.0.0.1")
}

// ---------------------------------------------------------------------------
// AuthEvent structure tests
// ---------------------------------------------------------------------------

func TestAuthEvent_EmptyDetail(t *testing.T) {
	event := AuthEvent{
		Action:      "login",
		ActorUserID: "user-1",
		IPAddress:   "10.0.0.1",
		UserAgent:   "test",
		Detail:      nil,
	}
	if event.Action != "login" {
		t.Errorf("Action = %q", event.Action)
	}
}

func TestAuthEvent_MultipleDetailFields(t *testing.T) {
	event := AuthEvent{
		Action:      "access_denied",
		ActorUserID: "user-123",
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0",
		Detail: map[string]string{
			"endpoint":      "/api/cases",
			"required_role": "case_admin",
			"actual_role":   "user",
			"method":        "POST",
		},
	}
	if len(event.Detail) != 4 {
		t.Errorf("Detail length = %d, want 4", len(event.Detail))
	}
}
