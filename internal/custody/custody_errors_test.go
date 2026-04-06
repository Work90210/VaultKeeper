//go:build integration

package custody

// custody_errors_test.go exercises every error branch in repository.go and the
// VerifyCaseChain error path in chain.go that cannot be reached through normal
// integration tests (cancelled context, fake pool injections, direct list()
// calls, limit clamping, and hash-value tamper scenarios).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Minimal pgx fakes
// ---------------------------------------------------------------------------

// fakeRow implements pgx.Row. Scan returns scanErr when set.
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
		switch p := d.(type) {
		case *string:
			if s, ok := r.values[i].(string); ok {
				*p = s
			}
		case *int:
			if n, ok := r.values[i].(int); ok {
				*p = n
			}
		}
	}
	return nil
}

// fakeRows implements pgx.Rows. It returns a controlled sequence of rows
// and optionally a final rows.Err.
type fakeRows struct {
	scanErr  error
	rowsErr  error
	events   []Event
	pos      int
	closed   bool
}

func (r *fakeRows) Close()                                         { r.closed = true }
func (r *fakeRows) Err() error                                     { return r.rowsErr }
func (r *fakeRows) CommandTag() pgconn.CommandTag                  { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *fakeRows) RawValues() [][]byte                            { return nil }
func (r *fakeRows) Values() ([]any, error)                         { return nil, nil }
func (r *fakeRows) Conn() *pgx.Conn                                { return nil }
func (r *fakeRows) Next() bool {
	if r.scanErr != nil {
		// Return true once to trigger Scan, which will then error.
		if r.pos == 0 {
			r.pos++
			return true
		}
		return false
	}
	if r.pos < len(r.events) {
		r.pos++
		return true
	}
	return false
}
func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.pos == 0 || r.pos > len(r.events) {
		return errors.New("fakeRows: Scan out of range")
	}
	e := r.events[r.pos-1]
	// Map fields in the order the SELECT statements use:
	// id, case_id, evidence_id, action, actor_user_id, detail,
	// hash_value, previous_hash, timestamp
	if len(dest) == 9 {
		*dest[0].(*uuid.UUID) = e.ID
		*dest[1].(*uuid.UUID) = e.CaseID
		*dest[2].(*uuid.UUID) = e.EvidenceID
		*dest[3].(*string) = e.Action
		*dest[4].(*string) = e.ActorUserID
		*dest[5].(*string) = e.Detail
		*dest[6].(*string) = e.HashValue
		*dest[7].(*string) = e.PreviousHash
		*dest[8].(*time.Time) = e.Timestamp
	}
	return nil
}

// fakeTx implements pgx.Tx. Only the methods used by PGRepository are wired;
// the rest delegate to the embedded nil interface and will panic if called
// (they should not be called in these tests).
type fakeTx struct {
	pgx.Tx // embedded to satisfy the interface; unused methods will panic

	execErr    error // returned by Exec
	queryRow   pgx.Row
	commitErr  error
	rollbackErr error

	execCallCount int
	execErrOnCall int // if > 0, return execErr only on this call number (1-based)
}

func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execCallCount++
	if t.execErrOnCall > 0 {
		if t.execCallCount == t.execErrOnCall {
			return pgconn.CommandTag{}, t.execErr
		}
		return pgconn.CommandTag{}, nil
	}
	if t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRow != nil {
		return t.queryRow
	}
	// Default: return ErrNoRows (no previous hash)
	return &fakeRow{scanErr: pgx.ErrNoRows}
}

func (t *fakeTx) Commit(ctx context.Context) error  { return t.commitErr }
func (t *fakeTx) Rollback(ctx context.Context) error { return t.rollbackErr }

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return nil, errors.New("not implemented") }
func (t *fakeTx) Conn() *pgx.Conn                           { return nil }

// fakePool implements dbPool with configurable failure points.
type fakePool struct {
	beginTxErr error
	tx         *fakeTx

	// For QueryRow (used by list's count query)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	// For Query (used by list's main query and ListAllByCase)
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

	queryCallCount int
	queryErrOnCall int // if > 0, return query error only on this call number
}

func (p *fakePool) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if p.beginTxErr != nil {
		return nil, p.beginTxErr
	}
	return p.tx, nil
}

func (p *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	// Default: return count = 0
	return &fakeRow{values: []any{0}}
}

func (p *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	p.queryCallCount++
	if p.queryErrOnCall > 0 && p.queryCallCount == p.queryErrOnCall {
		return nil, fmt.Errorf("injected query error on call %d", p.queryErrOnCall)
	}
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var errInjected = errors.New("injected test error")

// ---------------------------------------------------------------------------
// repository.go — Insert error paths
// ---------------------------------------------------------------------------

// TestRepository_Insert_BeginTxError covers L30: pool.BeginTx returns an error.
func TestRepository_Insert_BeginTxError(t *testing.T) {
	pool := &fakePool{beginTxErr: errInjected}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from BeginTx, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// TestRepository_Insert_AdvisoryLockError covers L37: advisory lock Exec fails.
// The lock is the first Exec call inside the transaction.
func TestRepository_Insert_AdvisoryLockError(t *testing.T) {
	tx := &fakeTx{execErr: errInjected}
	pool := &fakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from advisory lock, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// TestRepository_Insert_GetLastHashError covers L47: QueryRow.Scan returns a
// non-ErrNoRows error when reading the last hash.
func TestRepository_Insert_GetLastHashError(t *testing.T) {
	tx := &fakeTx{
		// Exec (advisory lock) succeeds; QueryRow returns a non-ErrNoRows scan error.
		execErr:  nil,
		queryRow: &fakeRow{scanErr: errInjected},
	}
	pool := &fakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from getLastHash, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// TestRepository_Insert_ExecInsertError covers L67: the INSERT INTO custody_log
// Exec call fails. The advisory lock Exec succeeds (call 1), insert fails (call 2).
func TestRepository_Insert_ExecInsertError(t *testing.T) {
	tx := &fakeTx{
		execErr:    errInjected,
		execErrOnCall: 2, // fail on second Exec (the INSERT)
		queryRow:   &fakeRow{scanErr: pgx.ErrNoRows}, // no previous hash
	}
	pool := &fakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from INSERT exec, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// TestRepository_Insert_CommitError covers L71: tx.Commit returns an error.
// Both Exec calls succeed; only Commit fails.
func TestRepository_Insert_CommitError(t *testing.T) {
	tx := &fakeTx{
		commitErr: errInjected,
		queryRow:  &fakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &fakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from Commit, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// ---------------------------------------------------------------------------
// repository.go — list() validation and clamping (L86, L89, L92)
// ---------------------------------------------------------------------------

// TestRepository_list_InvalidFilterCol covers L86: filterCol is not "case_id"
// or "evidence_id".
func TestRepository_list_InvalidFilterCol(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, _, err := repo.list(ctx, "user_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error for invalid filterCol, got nil")
	}
}

// TestRepository_list_LimitZeroClamped covers L89: limit <= 0 is set to 50.
// We verify the call succeeds (no error) with limit=0 on an empty result set.
func TestRepository_list_LimitZeroClamped(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	events, total, err := repo.list(ctx, "case_id", caseID, 0, "")
	if err != nil {
		t.Fatalf("list with limit=0: %v", err)
	}
	// No events inserted; just confirm the call ran through the clamping path.
	_ = events
	_ = total
}

// TestRepository_list_LimitOverMaxClamped covers L92: limit > 200 is clamped to 200.
func TestRepository_list_LimitOverMaxClamped(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	events, total, err := repo.list(ctx, "case_id", caseID, 300, "")
	if err != nil {
		t.Fatalf("list with limit=300: %v", err)
	}
	_ = events
	_ = total
}

// ---------------------------------------------------------------------------
// repository.go — list() DB error paths (L127, L138)
// ---------------------------------------------------------------------------

// TestRepository_list_CountQueryError covers L127: the COUNT query returns an error.
// We use a fake pool whose QueryRow returns a scan error.
func TestRepository_list_CountQueryError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{scanErr: errInjected}
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from count query, got nil")
	}
}

// TestRepository_list_MainQueryError covers L138: the main SELECT query returns an error.
// The COUNT succeeds (scan returns 0); the second call (Query) fails.
func TestRepository_list_MainQueryError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// COUNT succeeds, returns 0
			return &fakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errInjected
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from main query, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// ---------------------------------------------------------------------------
// repository.go — list() rows.Scan error (L147) and rows.Err (L152)
// ---------------------------------------------------------------------------

// TestRepository_list_ScanError covers L147: rows.Scan returns an error.
func TestRepository_list_ScanError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{values: []any{1}} // COUNT = 1
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{scanErr: errInjected}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from rows.Scan, got nil")
	}
}

// TestRepository_list_RowsErr covers L152: rows.Err returns an error after
// iteration completes normally (no rows returned, but rows.Err is set).
func TestRepository_list_RowsErr(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{values: []any{0}} // COUNT = 0
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{rowsErr: errInjected}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from rows.Err, got nil")
	}
}

// ---------------------------------------------------------------------------
// repository.go — list() len(events) > limit trim (L156)
// ---------------------------------------------------------------------------

// TestRepository_list_TrimExcessEvents covers L156: when the query returns
// limit+1 rows (the sentinel), the slice is trimmed to limit.
// We insert limit+1 real rows and call list with that limit.
func TestRepository_list_TrimExcessEvents(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	const limit = 3
	for i := 0; i < limit+1; i++ {
		err := repo.Insert(ctx, Event{
			CaseID:      caseID,
			EvidenceID:  uuid.Nil,
			Action:      "trim_test",
			ActorUserID: uuid.New().String(),
			Detail:      `{}`,
			Timestamp:   time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}

	events, total, err := repo.list(ctx, "case_id", caseID, limit, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != limit+1 {
		t.Errorf("total = %d, want %d", total, limit+1)
	}
	if len(events) != limit {
		t.Errorf("len(events) = %d, want %d (trimmed)", len(events), limit)
	}
}

// ---------------------------------------------------------------------------
// repository.go — ListAllByCase error paths (L169, L178)
// ---------------------------------------------------------------------------

// TestRepository_ListAllByCase_QueryError covers L169: the SELECT query errors.
func TestRepository_ListAllByCase_QueryError(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errInjected
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListAllByCase(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListAllByCase query, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// TestRepository_ListAllByCase_ScanError covers L178: rows.Scan returns an error.
func TestRepository_ListAllByCase_ScanError(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{scanErr: errInjected}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListAllByCase(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected scan error from ListAllByCase, got nil")
	}
}

// ---------------------------------------------------------------------------
// chain.go — VerifyCaseChain error path (L28)
// ---------------------------------------------------------------------------

// TestChainVerifier_ListAllByCaseError covers chain.go L28: when ListAllByCase
// returns an error, VerifyCaseChain wraps and returns it.
func TestChainVerifier_ListAllByCaseError(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errInjected
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	_, err := verifier.VerifyCaseChain(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from VerifyCaseChain when ListAllByCase fails, got nil")
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("error = %v, want to wrap errInjected", err)
	}
}

// ---------------------------------------------------------------------------
// chain.go — hash_value mismatch detection (L45)
// ---------------------------------------------------------------------------

// TestChainVerifier_HashValueMismatch covers chain.go L45: when the stored
// hash_value does not match the recomputed expected hash, a ChainBreak is
// recorded and result.Valid is false.
func TestChainVerifier_HashValueMismatch(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	verifier := NewChainVerifier(repo)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	// Insert two clean events to form a valid chain.
	for i := 0; i < 2; i++ {
		if err := repo.Insert(ctx, Event{
			CaseID:      caseID,
			EvidenceID:  uuid.Nil,
			Action:      "event",
			ActorUserID: uuid.New().String(),
			Detail:      `{}`,
			Timestamp:   time.Now().UTC(),
		}); err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}

	// Collect event IDs in ascending order (the order VerifyCaseChain sees them).
	rows, err := pool.Query(ctx,
		`SELECT id FROM custody_log WHERE case_id = $1 ORDER BY timestamp ASC, id ASC`,
		caseID,
	)
	if err != nil {
		t.Fatalf("query event IDs: %v", err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			t.Fatalf("scan id: %v", scanErr)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatalf("rows error: %v", rows.Err())
	}
	if len(ids) < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(ids))
	}

	// Corrupt the hash_value of the first event so that ComputeLogHash produces
	// a different value than what is stored, triggering the L45 branch.
	_, err = pool.Exec(ctx,
		`UPDATE custody_log SET hash_value = 'corrupted-hash-value' WHERE id = $1`,
		ids[0],
	)
	if err != nil {
		t.Fatalf("corrupt hash_value: %v", err)
	}

	result, err := verifier.VerifyCaseChain(ctx, caseID)
	if err != nil {
		t.Fatalf("VerifyCaseChain: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid after corrupting hash_value")
	}
	if len(result.Breaks) == 0 {
		t.Error("expected at least one ChainBreak")
	}

	// Confirm that one break captures the corrupted hash as the actual value.
	found := false
	for _, b := range result.Breaks {
		if b.ActualHash == "corrupted-hash-value" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no ChainBreak with ActualHash == 'corrupted-hash-value'; breaks: %+v", result.Breaks)
	}
}
