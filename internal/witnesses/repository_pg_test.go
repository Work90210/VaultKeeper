package witnesses

// Tests for PGRepository using mock implementations of the dbPool, pgx.Row,
// and pgx.Rows interfaces. No real database is required.
//
// These tests bring repository.go from 0% to near-100% coverage by exercising
// every repository method and every branch through fakes that implement the
// narrow pgx interfaces consumed by the repository.

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
// pgx.Row mock
// ---------------------------------------------------------------------------

// mockRow implements pgx.Row.
type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFn != nil {
		return m.scanFn(dest...)
	}
	return nil
}

// successRow returns a mockRow whose Scan populates destinations with the
// fields of the given Witness (in the column order used by witnessColumns).
func successRow(w Witness) *mockRow {
	return &mockRow{
		scanFn: func(dest ...any) error {
			return populateDest(dest, w)
		},
	}
}

func errRow(err error) *mockRow {
	return &mockRow{scanFn: func(...any) error { return err }}
}

// populateDest assigns a fixed witness into the Scan destinations expected by
// scanWitness / scanWitnessRows. The column order mirrors witnessColumns:
//
//	id, case_id, witness_code, pseudonym, full_name_encrypted,
//	contact_info_encrypted, location_encrypted, protection_status,
//	statement_summary, related_evidence, judge_identity_visible,
//	created_by, created_at, updated_at
func populateDest(dest []any, w Witness) error {
	if len(dest) != 14 {
		return fmt.Errorf("populateDest: expected 14 destinations, got %d", len(dest))
	}
	assign(dest[0], w.ID)
	assign(dest[1], w.CaseID)
	assign(dest[2], w.WitnessCode)
	assign(dest[3], w.WitnessCode) // pseudonym == witness_code in the INSERT
	assign(dest[4], w.FullNameEncrypted)
	assign(dest[5], w.ContactInfoEncrypted)
	assign(dest[6], w.LocationEncrypted)
	assign(dest[7], w.ProtectionStatus)
	assign(dest[8], w.StatementSummary)
	assign(dest[9], w.RelatedEvidence)
	assign(dest[10], w.JudgeIdentityVisible)
	assign(dest[11], w.CreatedBy)
	assign(dest[12], w.CreatedAt)
	assign(dest[13], w.UpdatedAt)
	return nil
}

// assign copies src into *dst using a type switch for the types actually used
// in the witness Scan call. Panics on unknown types — kept intentional so test
// failures are loud.
func assign(dst, src any) {
	switch d := dst.(type) {
	case *uuid.UUID:
		*d = src.(uuid.UUID)
	case *[]uuid.UUID:
		if src == nil {
			*d = nil
		} else {
			*d = src.([]uuid.UUID)
		}
	case *[]byte:
		if src == nil {
			*d = nil
		} else {
			*d = src.([]byte)
		}
	case *string:
		*d = src.(string)
	case *bool:
		*d = src.(bool)
	case *time.Time:
		*d = src.(time.Time)
	case *int:
		*d = src.(int)
	default:
		panic(fmt.Sprintf("assign: unhandled type %T", dst))
	}
}

// ---------------------------------------------------------------------------
// pgx.Rows mock
// ---------------------------------------------------------------------------

// mockRows implements pgx.Rows.
type mockRows struct {
	witnesses []Witness
	pos       int
	scanErr   error // if set, returned by Scan on the first call
	rowsErr   error // returned by Err()
}

func (m *mockRows) Next() bool           { m.pos++; return m.pos <= len(m.witnesses) }
func (m *mockRows) Close()               {}
func (m *mockRows) Err() error           { return m.rowsErr }
func (m *mockRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Values() ([]any, error)     { return nil, nil }
func (m *mockRows) RawValues() [][]byte        { return nil }
func (m *mockRows) Conn() *pgx.Conn            { return nil }

func (m *mockRows) Scan(dest ...any) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	return populateDest(dest, m.witnesses[m.pos-1])
}

// ---------------------------------------------------------------------------
// dbPool mock
// ---------------------------------------------------------------------------

type mockPool struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return errRow(pgx.ErrNoRows)
}

func (m *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

// ---------------------------------------------------------------------------
// Helper: build a PGRepository backed by a mockPool (bypasses NewRepository
// which requires a real *pgxpool.Pool).
// ---------------------------------------------------------------------------

func newTestRepo(pool *mockPool) *PGRepository {
	return &PGRepository{pool: pool}
}

// sampleWitness returns a fully-populated Witness for scanning tests.
func sampleWitness() Witness {
	return Witness{
		ID:                   uuid.New(),
		CaseID:               uuid.New(),
		WitnessCode:          "W-001",
		FullNameEncrypted:    []byte("enc-name"),
		ContactInfoEncrypted: []byte("enc-contact"),
		LocationEncrypted:    []byte("enc-location"),
		ProtectionStatus:     "standard",
		StatementSummary:     "Saw the incident",
		RelatedEvidence:      []uuid.UUID{uuid.New()},
		JudgeIdentityVisible: false,
		CreatedBy:            uuid.New(),
		CreatedAt:            time.Now().UTC().Truncate(time.Second),
		UpdatedAt:            time.Now().UTC().Truncate(time.Second),
	}
}

// ---------------------------------------------------------------------------
// NewRepository
// ---------------------------------------------------------------------------

func TestNewRepository_ReturnsNonNil(t *testing.T) {
	// NewRepository accepts a *pgxpool.Pool which we cannot construct without
	// a real database, so we test the PGRepository struct path by constructing
	// it directly through the internal helper used throughout these tests.
	repo := newTestRepo(&mockPool{})
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// ---------------------------------------------------------------------------
// scanWitness
// ---------------------------------------------------------------------------

func TestScanWitness_Success(t *testing.T) {
	want := sampleWitness()
	row := successRow(want)
	got, err := scanWitness(row)
	if err != nil {
		t.Fatalf("scanWitness: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %v, want %v", got.ID, want.ID)
	}
	if got.WitnessCode != want.WitnessCode {
		t.Errorf("WitnessCode: got %q, want %q", got.WitnessCode, want.WitnessCode)
	}
}

func TestScanWitness_Error(t *testing.T) {
	scanErr := errors.New("scan failed")
	row := errRow(scanErr)
	_, err := scanWitness(row)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScanWitness_NilRelatedEvidence(t *testing.T) {
	// When the database returns NULL for related_evidence, it must be normalised
	// to an empty slice rather than nil.
	w := sampleWitness()
	w.RelatedEvidence = nil
	row := successRow(w)
	got, err := scanWitness(row)
	if err != nil {
		t.Fatalf("scanWitness: %v", err)
	}
	if got.RelatedEvidence == nil {
		t.Error("RelatedEvidence should be empty slice, not nil")
	}
}

// ---------------------------------------------------------------------------
// scanWitnessRows
// ---------------------------------------------------------------------------

func TestScanWitnessRows_Empty(t *testing.T) {
	rows := &mockRows{}
	items, err := scanWitnessRows(rows)
	if err != nil {
		t.Fatalf("scanWitnessRows: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestScanWitnessRows_MultipleWitnesses(t *testing.T) {
	w1 := sampleWitness()
	w2 := sampleWitness()
	rows := &mockRows{witnesses: []Witness{w1, w2}}

	items, err := scanWitnessRows(rows)
	if err != nil {
		t.Fatalf("scanWitnessRows: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != w1.ID {
		t.Errorf("items[0].ID: got %v, want %v", items[0].ID, w1.ID)
	}
	if items[1].ID != w2.ID {
		t.Errorf("items[1].ID: got %v, want %v", items[1].ID, w2.ID)
	}
}

func TestScanWitnessRows_ScanError(t *testing.T) {
	scanErr := errors.New("row scan failed")
	rows := &mockRows{
		witnesses: []Witness{sampleWitness()},
		scanErr:   scanErr,
	}
	_, err := scanWitnessRows(rows)
	if err == nil {
		t.Fatal("expected error from scan, got nil")
	}
}

func TestScanWitnessRows_RowsErr(t *testing.T) {
	iterErr := errors.New("iteration error")
	rows := &mockRows{rowsErr: iterErr}
	_, err := scanWitnessRows(rows)
	if !errors.Is(err, iterErr) {
		t.Errorf("expected iterErr, got %v", err)
	}
}

func TestScanWitnessRows_NilRelatedEvidence(t *testing.T) {
	w := sampleWitness()
	w.RelatedEvidence = nil
	rows := &mockRows{witnesses: []Witness{w}}

	items, err := scanWitnessRows(rows)
	if err != nil {
		t.Fatalf("scanWitnessRows: %v", err)
	}
	if items[0].RelatedEvidence == nil {
		t.Error("RelatedEvidence should be empty slice, not nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.Create
// ---------------------------------------------------------------------------

func TestPGRepository_Create_Success(t *testing.T) {
	want := sampleWitness()
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return successRow(want)
		},
	}
	repo := newTestRepo(pool)
	got, err := repo.Create(context.Background(), want)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %v, want %v", got.ID, want.ID)
	}
}

func TestPGRepository_Create_ScanError(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(errors.New("scan error"))
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.Create(context.Background(), sampleWitness())
	if err == nil {
		t.Fatal("expected error from Create, got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByID
// ---------------------------------------------------------------------------

func TestPGRepository_FindByID_Success(t *testing.T) {
	want := sampleWitness()
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return successRow(want)
		},
	}
	repo := newTestRepo(pool)
	got, err := repo.FindByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %v, want %v", got.ID, want.ID)
	}
}

func TestPGRepository_FindByID_NotFound(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(pgx.ErrNoRows)
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FindByID_OtherError(t *testing.T) {
	dbErr := errors.New("connection reset")
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(dbErr)
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("should not be ErrNotFound for generic errors")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindByCase
// ---------------------------------------------------------------------------

func TestPGRepository_FindByCase_Success(t *testing.T) {
	caseID := uuid.New()
	w1 := sampleWitness()
	w1.CaseID = caseID
	w2 := sampleWitness()
	w2.CaseID = caseID

	callCount := 0
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// Called for COUNT(*).
			callCount++
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 2
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{witnesses: []Witness{w1, w2}}, nil
		},
	}
	repo := newTestRepo(pool)
	items, total, err := repo.FindByCase(context.Background(), caseID, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase: %v", err)
	}
	if total != 2 {
		t.Errorf("total: got %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("items: got %d, want 2", len(items))
	}
}

func TestPGRepository_FindByCase_CountError(t *testing.T) {
	countErr := errors.New("count query failed")
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(countErr)
		},
	}
	repo := newTestRepo(pool)
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 50})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPGRepository_FindByCase_QueryError(t *testing.T) {
	queryErr := errors.New("query failed")
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 0
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, queryErr
		},
	}
	repo := newTestRepo(pool)
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 50})
	if !errors.Is(err, queryErr) {
		t.Errorf("expected queryErr, got %v", err)
	}
}

func TestPGRepository_FindByCase_WithCursor(t *testing.T) {
	caseID := uuid.New()
	cursorID := uuid.New()
	cursor := encodeCursor(cursorID)

	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 1
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{witnesses: []Witness{sampleWitness()}}, nil
		},
	}
	repo := newTestRepo(pool)
	_, _, err := repo.FindByCase(context.Background(), caseID, Pagination{Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindByCase with cursor: %v", err)
	}
}

func TestPGRepository_FindByCase_InvalidCursor(t *testing.T) {
	pool := &mockPool{}
	repo := newTestRepo(pool)
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 10, Cursor: "!!!invalid!!!"})
	if err == nil {
		t.Fatal("expected error for invalid cursor, got nil")
	}
}

func TestPGRepository_FindByCase_LimitTruncation(t *testing.T) {
	// When the DB returns limit+1 rows (indicating a next page), the repository
	// must truncate to limit items.
	caseID := uuid.New()
	limit := 2
	witnesses := []Witness{sampleWitness(), sampleWitness(), sampleWitness()} // limit+1

	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 10
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{witnesses: witnesses}, nil
		},
	}
	repo := newTestRepo(pool)
	items, _, err := repo.FindByCase(context.Background(), caseID, Pagination{Limit: limit})
	if err != nil {
		t.Fatalf("FindByCase: %v", err)
	}
	if len(items) != limit {
		t.Errorf("expected %d items after truncation, got %d", limit, len(items))
	}
}

func TestPGRepository_FindByCase_ScanRowsError(t *testing.T) {
	scanErr := errors.New("bad scan")
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*int) = 1
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				witnesses: []Witness{sampleWitness()},
				scanErr:   scanErr,
			}, nil
		},
	}
	repo := newTestRepo(pool)
	_, _, err := repo.FindByCase(context.Background(), uuid.New(), Pagination{Limit: 50})
	if err == nil {
		t.Fatal("expected scan error, got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.Update
// ---------------------------------------------------------------------------

func TestPGRepository_Update_Success(t *testing.T) {
	want := sampleWitness()
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return successRow(want)
		},
	}
	repo := newTestRepo(pool)
	got, err := repo.Update(context.Background(), want.ID, want)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %v, want %v", got.ID, want.ID)
	}
}

func TestPGRepository_Update_NotFound(t *testing.T) {
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(pgx.ErrNoRows)
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.Update(context.Background(), uuid.New(), sampleWitness())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepository_Update_OtherError(t *testing.T) {
	dbErr := errors.New("constraint violation")
	pool := &mockPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return errRow(dbErr)
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.Update(context.Background(), uuid.New(), sampleWitness())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("should not be ErrNotFound for generic errors")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.FindAll
// ---------------------------------------------------------------------------

func TestPGRepository_FindAll_Success(t *testing.T) {
	w1 := sampleWitness()
	w2 := sampleWitness()
	pool := &mockPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{witnesses: []Witness{w1, w2}}, nil
		},
	}
	repo := newTestRepo(pool)
	items, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestPGRepository_FindAll_Empty(t *testing.T) {
	pool := &mockPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}
	repo := newTestRepo(pool)
	items, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPGRepository_FindAll_QueryError(t *testing.T) {
	queryErr := errors.New("db error")
	pool := &mockPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, queryErr
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.FindAll(context.Background())
	if !errors.Is(err, queryErr) {
		t.Errorf("expected queryErr, got %v", err)
	}
}

func TestPGRepository_FindAll_ScanError(t *testing.T) {
	scanErr := errors.New("scan failed")
	pool := &mockPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				witnesses: []Witness{sampleWitness()},
				scanErr:   scanErr,
			}, nil
		},
	}
	repo := newTestRepo(pool)
	_, err := repo.FindAll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// PGRepository.UpdateEncryptedFields
// ---------------------------------------------------------------------------

func TestPGRepository_UpdateEncryptedFields_Success(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}
	repo := newTestRepo(pool)
	err := repo.UpdateEncryptedFields(
		context.Background(), uuid.New(),
		[]byte("enc-name"), []byte("enc-contact"), []byte("enc-location"),
	)
	if err != nil {
		t.Fatalf("UpdateEncryptedFields: %v", err)
	}
}

func TestPGRepository_UpdateEncryptedFields_ExecError(t *testing.T) {
	execErr := errors.New("exec failed")
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, execErr
		},
	}
	repo := newTestRepo(pool)
	err := repo.UpdateEncryptedFields(
		context.Background(), uuid.New(),
		nil, nil, nil,
	)
	if !errors.Is(err, execErr) {
		t.Errorf("expected execErr, got %v", err)
	}
}

func TestPGRepository_UpdateEncryptedFields_NilFields(t *testing.T) {
	pool := &mockPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}
	repo := newTestRepo(pool)
	err := repo.UpdateEncryptedFields(context.Background(), uuid.New(), nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateEncryptedFields with nil fields: %v", err)
	}
}
