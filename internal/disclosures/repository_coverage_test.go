package disclosures

// Gap-filler tests to bring internal/disclosures to 100%. Targets the
// uncovered branches in Create (success return + ErrNotFound), FindByID
// (ErrNotFound branch + evidenceIDs append loop), FindByCase (cursor
// decode + error + WHERE cursor condition), and EvidenceBelongsToCase
// (zero-length input + count mismatch). Also calls NewRepository to
// cover the one-line constructor.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestNewRepository covers the single-line Postgres repo constructor.
func TestNewRepository(t *testing.T) {
	repo := NewRepository(&pgxpool.Pool{})
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

// ---- Create: happy-path success return ----

func TestCreate_HappyPath(t *testing.T) {
	createdIDs := []uuid.UUID{uuid.New(), uuid.New()}
	idx := 0
	tx := &fakeTx{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			id := createdIDs[idx]
			idx++
			return &scanRow{fillFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = id
				return nil
			}}
		},
	}
	pool := &mockPool{tx: tx}
	repo := &PGRepository{pool: pool}

	d := Disclosure{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New(), uuid.New()},
		DisclosedTo: "defence counsel",
		DisclosedBy: uuid.New(),
	}
	out, err := repo.Create(context.Background(), d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.ID != createdIDs[0] {
		t.Errorf("ID = %s, want %s (first inserted)", out.ID, createdIDs[0])
	}
	if len(out.EvidenceIDs) != 2 {
		t.Errorf("EvidenceIDs len = %d, want 2", len(out.EvidenceIDs))
	}
}

// scanRow is a pgx.Row whose Scan calls a user-supplied fill function.
type scanRow struct {
	fillFn func(dest ...any) error
	err    error
}

func (r *scanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.fillFn != nil {
		return r.fillFn(dest...)
	}
	return nil
}

// ---- FindByID: ErrNotFound branch + evidence-ID append loop ----

func TestFindByID_NotFound(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// fakeRowsMulti streams N scripted rows so the append loop runs at least
// twice and covers the inner slice growth.
type fakeRowsMulti struct {
	ids []uuid.UUID
	idx int
	err error
}

func (r *fakeRowsMulti) Close()                                       {}
func (r *fakeRowsMulti) Err() error                                   { return r.err }
func (r *fakeRowsMulti) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRowsMulti) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRowsMulti) RawValues() [][]byte                          { return nil }
func (r *fakeRowsMulti) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRowsMulti) Conn() *pgx.Conn                              { return nil }
func (r *fakeRowsMulti) Next() bool {
	if r.idx >= len(r.ids) {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRowsMulti) Scan(dest ...any) error {
	*(dest[0].(*uuid.UUID)) = r.ids[r.idx-1]
	return nil
}

// ---- FindByID: evidence-IDs append loop with multiple rows ----

// disclosureScanRow fills a FindByID single-row scan.
type disclosureScanRow struct {
	d         Disclosure
	err       error
}

func (r *disclosureScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*uuid.UUID)) = r.d.ID
	*(dest[1].(*uuid.UUID)) = r.d.CaseID
	*(dest[2].(*uuid.UUID)) = uuid.New() // evidence_id (unused in aggregate flow)
	*(dest[3].(*string)) = r.d.DisclosedTo
	*(dest[4].(*uuid.UUID)) = r.d.DisclosedBy
	*(dest[5].(*time.Time)) = r.d.DisclosedAt
	// Notes is *string in the DB column but scans into a sql.NullString-style
	// behaviour via pgx — the repository uses `&d.Notes` with type string,
	// so we fill it with an empty string.
	*(dest[6].(*string)) = r.d.Notes
	*(dest[7].(*bool)) = r.d.Redacted
	return nil
}

func TestFindByID_AggregateEvidenceIDs(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	d := Disclosure{
		ID:          uuid.New(),
		CaseID:      uuid.New(),
		DisclosedTo: "counsel",
		DisclosedBy: uuid.New(),
		DisclosedAt: time.Now(),
	}
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &disclosureScanRow{d: d}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRowsMulti{ids: ids}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	out, err := repo.FindByID(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.EvidenceIDs) != 3 {
		t.Errorf("EvidenceIDs len = %d, want 3", len(out.EvidenceIDs))
	}
}

// ---- FindByCase: cursor decode success + WHERE cursor condition ----

func TestFindByCase_WithValidCursor(t *testing.T) {
	countRow := &scanRow{fillFn: func(dest ...any) error {
		*(dest[0].(*int)) = 0
		return nil
	}}
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return countRow
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRowsMulti{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	cursor := encodeCursor(uuid.New())
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Cursor: cursor, Limit: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

// scriptedDiscBatch rows for the outer FindByCase query. One row with
// one batch; the inner query then returns 2 evidence IDs so the append
// loop executes.
type discBatchRows struct {
	emitted  bool
	d        Disclosure
}

func (r *discBatchRows) Close()                                       {}
func (r *discBatchRows) Err() error                                   { return nil }
func (r *discBatchRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *discBatchRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *discBatchRows) RawValues() [][]byte                          { return nil }
func (r *discBatchRows) Values() ([]any, error)                       { return nil, nil }
func (r *discBatchRows) Conn() *pgx.Conn                              { return nil }
func (r *discBatchRows) Next() bool {
	if r.emitted {
		return false
	}
	r.emitted = true
	return true
}
func (r *discBatchRows) Scan(dest ...any) error {
	*(dest[0].(*uuid.UUID)) = r.d.ID
	*(dest[1].(*uuid.UUID)) = r.d.CaseID
	*(dest[2].(*string)) = r.d.DisclosedTo
	*(dest[3].(*uuid.UUID)) = r.d.DisclosedBy
	*(dest[4].(*time.Time)) = r.d.DisclosedAt
	*(dest[5].(*string)) = r.d.Notes
	*(dest[6].(*bool)) = r.d.Redacted
	return nil
}

func TestFindByCase_PopulatesEvidenceIDs(t *testing.T) {
	// One batch with two evidence IDs — the outer query returns one row,
	// the inner evidence query returns two, exercising the eids append.
	calls := 0
	batch := Disclosure{
		ID:          uuid.New(),
		CaseID:      uuid.New(),
		DisclosedTo: "x",
		DisclosedBy: uuid.New(),
		DisclosedAt: time.Now(),
	}
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanRow{fillFn: func(dest ...any) error {
				*(dest[0].(*int)) = 1 // total
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			calls++
			if calls == 1 {
				// outer: one batch
				return &discBatchRows{d: batch}, nil
			}
			// inner: two evidence IDs
			return &fakeRowsMulti{ids: []uuid.UUID{uuid.New(), uuid.New()}}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	out, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 1 || len(out[0].EvidenceIDs) != 2 {
		t.Errorf("got %d batches with %d eids; want 1 and 2", len(out), len(out[0].EvidenceIDs))
	}
}

func TestFindByCase_InvalidCursor(t *testing.T) {
	countRow := &scanRow{fillFn: func(dest ...any) error {
		*(dest[0].(*int)) = 0
		return nil
	}}
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return countRow
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Cursor: "!!!not-base64!!!", Limit: 10})
	if err == nil {
		t.Fatal("want invalid cursor error")
	}
}

// ---- EvidenceBelongsToCase: zero-length input + count mismatch ----

func TestEvidenceBelongsToCase_Empty(t *testing.T) {
	pool := &mockPool{}
	repo := &PGRepository{pool: pool}
	ok, err := repo.EvidenceBelongsToCase(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("empty input should return false")
	}
}

func TestEvidenceBelongsToCase_CountMismatch(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanRow{fillFn: func(dest ...any) error {
				*(dest[0].(*int)) = 2 // count=2 but input has 3 IDs → mismatch
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	ok, err := repo.EvidenceBelongsToCase(context.Background(), uuid.New(),
		[]uuid.UUID{uuid.New(), uuid.New(), uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Error("count mismatch should return false")
	}
}

func TestEvidenceBelongsToCase_AllMatch(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanRow{fillFn: func(dest ...any) error {
				*(dest[0].(*int)) = 2
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	ok, err := repo.EvidenceBelongsToCase(context.Background(), uuid.New(),
		[]uuid.UUID{uuid.New(), uuid.New()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Error("all-match should return true")
	}
}
