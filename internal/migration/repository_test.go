package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockRow implements pgx.Row for repository tests.
type mockRow struct {
	scanErr error
	scanFn  func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return r.scanErr
}

// mockRows implements the subset of pgx.Rows PGRepository.ListByCase uses.
type mockRows struct {
	nextVals []bool
	idx      int
	scanFn   func(dest ...any) error
	errVal   error
}

func (r *mockRows) Close()                                       {}
func (r *mockRows) Err() error                                   { return r.errVal }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) RawValues() [][]byte                          { return nil }
func (r *mockRows) Values() ([]any, error)                       { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn                              { return nil }

func (r *mockRows) Next() bool {
	if r.idx >= len(r.nextVals) {
		return false
	}
	v := r.nextVals[r.idx]
	r.idx++
	return v
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

// mockPool satisfies the migration package's dbPool interface.
type mockPool struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (p *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanErr: pgx.ErrNoRows}
}
func (p *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}
func (p *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if p.execFn != nil {
		return p.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func TestPGRepository_Create_Success(t *testing.T) {
	now := time.Now().UTC()
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*time.Time) = now
				return nil
			}}
		},
	}
	r := &PGRepository{pool: pool}
	rec, err := r.Create(context.Background(), Record{
		CaseID:        uuid.New(),
		SourceSystem:  "RelativityOne",
		TotalItems:    1,
		ManifestHash:  "abc",
		MigrationHash: "abc",
		PerformedBy:   "tester",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Error("ID should have been generated")
	}
	if rec.Status != StatusInProgress {
		t.Errorf("Status = %q", rec.Status)
	}
	if rec.CreatedAt != now {
		t.Errorf("CreatedAt = %v", rec.CreatedAt)
	}
}

func TestPGRepository_Create_DBError(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("constraint violation")}
		},
	}
	r := &PGRepository{pool: pool}
	if _, err := r.Create(context.Background(), Record{PerformedBy: "t"}); err == nil {
		t.Error("want error")
	}
}

func TestPGRepository_FinalizeSuccess(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	r := &PGRepository{pool: pool}
	ts := time.Now()
	err := r.FinalizeSuccess(context.Background(), uuid.New(), 5, 0, "abc", []byte("token"), "TSA", &ts)
	if err != nil {
		t.Errorf("FinalizeSuccess: %v", err)
	}
}

func TestPGRepository_FinalizeSuccess_NotFound(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	r := &PGRepository{pool: pool}
	err := r.FinalizeSuccess(context.Background(), uuid.New(), 0, 0, "abc", nil, "", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FinalizeFailure_InvalidStatus(t *testing.T) {
	r := &PGRepository{pool: &mockPool{}}
	err := r.FinalizeFailure(context.Background(), uuid.New(), StatusCompleted)
	if err == nil || !contains(err.Error(), "invalid failure status") {
		t.Errorf("want invalid-status error, got %v", err)
	}
}

func TestPGRepository_FinalizeFailure_Success(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	r := &PGRepository{pool: pool}
	if err := r.FinalizeFailure(context.Background(), uuid.New(), StatusFailed); err != nil {
		t.Errorf("FinalizeFailure: %v", err)
	}
}

func TestPGRepository_Delete_OnlyRemovesInProgress(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	r := &PGRepository{pool: pool}
	if err := r.Delete(context.Background(), uuid.New()); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

func TestPGRepository_Delete_NotInProgress(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("DELETE 0"), nil
		},
	}
	r := &PGRepository{pool: pool}
	if err := r.Delete(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FindByID_Success(t *testing.T) {
	wantID := uuid.New()
	caseID := uuid.New()
	now := time.Now().UTC()
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = wantID
				*dest[1].(*uuid.UUID) = caseID
				*dest[2].(*string) = "RelativityOne"
				*dest[3].(*int) = 10
				*dest[4].(*int) = 10
				*dest[5].(*int) = 0
				*dest[6].(*string) = "mighash"
				*dest[7].(*string) = "manhash"
				*dest[8].(*[]byte) = []byte("token")
				*dest[9].(*string) = "TSA"
				*dest[10].(**time.Time) = &now
				*dest[11].(*string) = "tester"
				*dest[12].(*string) = "completed"
				*dest[13].(*time.Time) = now
				*dest[14].(**time.Time) = &now
				*dest[15].(*time.Time) = now
				return nil
			}}
		},
	}
	r := &PGRepository{pool: pool}
	rec, err := r.FindByID(context.Background(), wantID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if rec.ID != wantID || rec.Status != StatusCompleted || rec.SourceSystem != "RelativityOne" {
		t.Errorf("rec = %+v", rec)
	}
}

func TestPGRepository_FindByID_NotFound(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	r := &PGRepository{pool: pool}
	if _, err := r.FindByID(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FindByID_OtherError(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db down")}
		},
	}
	r := &PGRepository{pool: pool}
	_, err := r.FindByID(context.Background(), uuid.New())
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("want generic error, got %v", err)
	}
}

func TestPGRepository_ListByCase(t *testing.T) {
	now := time.Now().UTC()
	callCount := 0
	pool := &mockPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					callCount++
					*dest[0].(*uuid.UUID) = uuid.New()
					*dest[1].(*uuid.UUID) = uuid.New()
					*dest[2].(*string) = "RelativityOne"
					*dest[3].(*int) = 1
					*dest[4].(*int) = 1
					*dest[5].(*int) = 0
					*dest[6].(*string) = "h"
					*dest[7].(*string) = "h"
					*dest[8].(*[]byte) = nil
					*dest[9].(*string) = ""
					*dest[10].(**time.Time) = nil
					*dest[11].(*string) = "t"
					*dest[12].(*string) = "completed"
					*dest[13].(*time.Time) = now
					*dest[14].(**time.Time) = nil
					*dest[15].(*time.Time) = now
					return nil
				},
			}, nil
		},
	}
	r := &PGRepository{pool: pool}
	recs, err := r.ListByCase(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListByCase: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("len = %d, want 1", len(recs))
	}
}

func TestPGRepository_IsProcessed_Hit(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 1
				return nil
			}}
		},
	}
	r := &PGRepository{pool: pool}
	ok, err := r.IsProcessed(context.Background(), uuid.New(), "a.txt")
	if err != nil || !ok {
		t.Errorf("ok=%v err=%v", ok, err)
	}
}

func TestPGRepository_IsProcessed_Miss(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	r := &PGRepository{pool: pool}
	ok, err := r.IsProcessed(context.Background(), uuid.New(), "a.txt")
	if err != nil || ok {
		t.Errorf("ok=%v err=%v", ok, err)
	}
}

func TestPGRepository_MarkProcessed(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
	}
	r := &PGRepository{pool: pool}
	if err := r.MarkProcessed(context.Background(), uuid.New(), "a.txt"); err != nil {
		t.Errorf("MarkProcessed: %v", err)
	}
}

func TestPGRepository_MarkProcessed_DBError(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		},
	}
	r := &PGRepository{pool: pool}
	if err := r.MarkProcessed(context.Background(), uuid.New(), "a.txt"); err == nil {
		t.Error("want error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
