package disclosures

// repository_mock_test.go provides in-process mock implementations of the
// dbPool and pgx.Tx interfaces so that DB error branches in PGRepository
// can be reached without a live database.
//
// All pgx.Row, pgx.Rows, and pgx.Tx methods that are NOT exercised by these
// tests return zero values or no-ops so that the compiler is satisfied.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Minimal pgx.Row implementation
// ---------------------------------------------------------------------------

type fakeRow struct {
	err error
	// When err is nil, Scan fills each dest pointer with uuid.Nil.
}

func (f *fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	// Fill with zero uuid values so callers can proceed.
	for _, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = uuid.Nil
		case *string:
			*v = ""
		case *bool:
			*v = false
		case *int:
			*v = 0
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Minimal pgx.Rows implementation
// ---------------------------------------------------------------------------

type fakeRows struct {
	err      error
	rowCount int // how many rows to return before done
	current  int
	scanErr  error // error to return on Scan
	rowsErr  error // error to return on Err()
}

func (f *fakeRows) Next() bool {
	if f.err != nil {
		return false
	}
	if f.current < f.rowCount {
		f.current++
		return true
	}
	return false
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	for _, d := range dest {
		if v, ok := d.(*uuid.UUID); ok {
			*v = uuid.New()
		}
	}
	return nil
}

func (f *fakeRows) Err() error           { return f.rowsErr }
func (f *fakeRows) Close()               {}
func (f *fakeRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (f *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (f *fakeRows) Values() ([]any, error) { return nil, nil }
func (f *fakeRows) RawValues() [][]byte    { return nil }
func (f *fakeRows) Conn() *pgx.Conn        { return nil }

// ---------------------------------------------------------------------------
// Minimal pgx.Tx implementation
// ---------------------------------------------------------------------------

type fakeTx struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	commitErr  error
}

func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error)  { return nil, nil }
func (f *fakeTx) Commit(ctx context.Context) error           { return f.commitErr }
func (f *fakeTx) Rollback(ctx context.Context) error         { return nil }
func (f *fakeTx) CopyFrom(ctx context.Context, tn pgx.Identifier, cn []string, rs pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &fakeRows{}, nil
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if f.queryRowFn != nil {
		return f.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{}
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

// ---------------------------------------------------------------------------
// Mock dbPool
// ---------------------------------------------------------------------------

type mockPool struct {
	beginTxErr  error
	tx          pgx.Tx // returned by BeginTx when beginTxErr == nil
	queryRowFn  func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn     func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	callCount   int // incremented on each QueryRow call
}

func (m *mockPool) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if m.beginTxErr != nil {
		return nil, m.beginTxErr
	}
	return m.tx, nil
}

func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.callCount++
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{}
}

func (m *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &fakeRows{}, nil
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// newMockRepo returns a PGRepository backed by the given pool mock.
func newMockPGRepo(pool dbPool) *PGRepository {
	return &PGRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// PGRepository.Create error paths
// ---------------------------------------------------------------------------

func TestPGRepository_Create_BeginTxError(t *testing.T) {
	pool := &mockPool{beginTxErr: errors.New("cannot open tx")}
	repo := newMockPGRepo(pool)

	_, err := repo.Create(context.Background(), Disclosure{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence",
		DisclosedBy: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected error from BeginTx, got nil")
	}
}

func TestPGRepository_Create_InsertRowError(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: errors.New("constraint violation")}
		},
	}
	pool := &mockPool{tx: tx}
	repo := newMockPGRepo(pool)

	_, err := repo.Create(context.Background(), Disclosure{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence",
		DisclosedBy: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected error from INSERT row, got nil")
	}
}

func TestPGRepository_Create_CommitError(t *testing.T) {
	// INSERT succeeds but Commit fails.
	tx := &fakeTx{
		commitErr: errors.New("network partition"),
	}
	pool := &mockPool{tx: tx}
	repo := newMockPGRepo(pool)

	_, err := repo.Create(context.Background(), Disclosure{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence",
		DisclosedBy: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected error from Commit, got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByID error paths
// ---------------------------------------------------------------------------

func TestPGRepository_FindByID_QueryRowError(t *testing.T) {
	// Return a non-ErrNoRows error on the first QueryRow call.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: errors.New("db timeout")}
		},
	}
	repo := newMockPGRepo(pool)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from QueryRow, got nil")
	}
}

func TestPGRepository_FindByID_AggregateQueryError(t *testing.T) {
	// First QueryRow succeeds (returns row data), second Query (aggregate) fails.
	callCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			callCount++
			return &fakeRow{} // succeeds: fills with zero values
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("aggregate query failed")
		},
	}
	repo := newMockPGRepo(pool)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from aggregate Query, got nil")
	}
}

func TestPGRepository_FindByID_ScanEvidenceRowError(t *testing.T) {
	// First QueryRow succeeds; aggregate Query returns rows but Scan fails.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{rowCount: 1, scanErr: errors.New("scan failed")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from rows.Scan, got nil")
	}
}

func TestPGRepository_FindByID_RowsErrError(t *testing.T) {
	// First QueryRow succeeds; aggregate rows iteration completes but Err() returns error.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{rowCount: 0, rowsErr: errors.New("cursor error")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from rows.Err(), got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByCase error paths
// ---------------------------------------------------------------------------

func TestPGRepository_FindByCase_CountQueryError(t *testing.T) {
	// COUNT QueryRow fails.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: errors.New("count query failed")}
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from count QueryRow, got nil")
	}
}

func TestPGRepository_FindByCase_BatchQueryError(t *testing.T) {
	// COUNT succeeds; batch Query fails.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{} // COUNT returns 0, fine
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("batch query failed")
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from batch Query, got nil")
	}
}

func TestPGRepository_FindByCase_ScanDisclosureError(t *testing.T) {
	// COUNT succeeds; batch Query returns a row that fails on Scan.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{rowCount: 1, scanErr: errors.New("scan disclosure failed")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from scan disclosure, got nil")
	}
}

func TestPGRepository_FindByCase_IterateDisclosuresError(t *testing.T) {
	// COUNT succeeds; batch rows iteration completes but Err() returns error.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{rowCount: 0, rowsErr: errors.New("iterate disclosures error")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from iterate disclosures rows.Err(), got nil")
	}
}

func TestPGRepository_FindByCase_LoadEvidenceQueryError(t *testing.T) {
	// COUNT succeeds; batch query returns 1 row with valid scan data;
	// but the per-batch evidence Query fails.
	queryCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{} // COUNT = 0 (int default)
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			queryCount++
			if queryCount == 1 {
				// Batch query: return 1 row that scans successfully into Disclosure fields.
				return &disclosureRows{}, nil
			}
			// Per-evidence query: fail.
			return nil, errors.New("load evidence query failed")
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from load evidence Query, got nil")
	}
}

func TestPGRepository_FindByCase_ScanEvidenceIDError(t *testing.T) {
	// Batch query returns 1 row; per-evidence query returns a row that Scan fails on.
	queryCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			queryCount++
			if queryCount == 1 {
				return &disclosureRows{}, nil
			}
			return &fakeRows{rowCount: 1, scanErr: errors.New("scan evidence id failed")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from scan evidence id, got nil")
	}
}

func TestPGRepository_FindByCase_IterateEvidenceError(t *testing.T) {
	// Batch query returns 1 row; per-evidence query rows.Err() fails.
	queryCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			queryCount++
			if queryCount == 1 {
				return &disclosureRows{}, nil
			}
			return &fakeRows{rowCount: 0, rowsErr: errors.New("iterate evidence error")}, nil
		},
	}
	repo := newMockPGRepo(pool)

	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from iterate evidence rows.Err(), got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByID – empty aggregate fallback (line 129)
// ---------------------------------------------------------------------------

func TestPGRepository_FindByID_EmptyAggregateUsesInitialEvidenceID(t *testing.T) {
	// First QueryRow succeeds; aggregate Query returns 0 rows (empty result).
	// The code falls back to []uuid.UUID{evidenceID} from the initial scan.
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{} // fills with uuid.Nil (zero values)
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			// Return rows that have no data rows but no error either.
			return &fakeRows{rowCount: 0}, nil
		},
	}
	repo := newMockPGRepo(pool)

	d, err := repo.FindByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Falls back: evidenceIDs should contain the initial evidenceID (uuid.Nil from fakeRow).
	if len(d.EvidenceIDs) != 1 {
		t.Errorf("EvidenceIDs len = %d, want 1 (fallback)", len(d.EvidenceIDs))
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByCase – empty per-batch evidence fallback (line 219)
// ---------------------------------------------------------------------------

func TestPGRepository_FindByCase_EmptyPerBatchEvidence(t *testing.T) {
	// Batch query returns 1 disclosure row; per-evidence query returns 0 rows.
	// The code sets eids = []uuid.UUID{} for that disclosure.
	queryCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{} // COUNT = 0 fine
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			queryCount++
			if queryCount == 1 {
				return &disclosureRows{}, nil // 1 disclosure
			}
			// Per-evidence: 0 rows, no error -> triggers eids == nil branch.
			return &fakeRows{rowCount: 0}, nil
		},
	}
	repo := newMockPGRepo(pool)

	disclosures, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(disclosures) != 1 {
		t.Fatalf("disclosures len = %d, want 1", len(disclosures))
	}
	if disclosures[0].EvidenceIDs == nil {
		t.Error("EvidenceIDs should be non-nil empty slice, not nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.EvidenceBelongsToCase error path
// ---------------------------------------------------------------------------

func TestPGRepository_EvidenceBelongsToCase_QueryRowError(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: errors.New("count evidence failed")}
		},
	}
	repo := newMockPGRepo(pool)

	_, err := repo.EvidenceBelongsToCase(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("expected error from QueryRow, got nil")
	}
}

// ---------------------------------------------------------------------------
// disclosureRows: fake pgx.Rows for the FindByCase batch SELECT
// Returns one row with all-zero UUID/string/bool values so Scan succeeds.
// ---------------------------------------------------------------------------

type disclosureRows struct {
	done bool
}

func (r *disclosureRows) Next() bool {
	if !r.done {
		r.done = true
		return true
	}
	return false
}

func (r *disclosureRows) Scan(dest ...any) error {
	for _, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = uuid.Nil
		case *string:
			*v = ""
		case *bool:
			*v = false
		}
	}
	return nil
}

func (r *disclosureRows) Err() error                              { return nil }
func (r *disclosureRows) Close()                                  {}
func (r *disclosureRows) CommandTag() pgconn.CommandTag           { return pgconn.CommandTag{} }
func (r *disclosureRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *disclosureRows) Values() ([]any, error)                  { return nil, nil }
func (r *disclosureRows) RawValues() [][]byte                     { return nil }
func (r *disclosureRows) Conn() *pgx.Conn                         { return nil }
