package evidence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPGRepository_ImplementsRepository(t *testing.T) {
	var _ Repository = (*PGRepository)(nil)
}

func TestDecodeCursor_Roundtrip(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	cursor := encodeCursor(id)
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	decoded, err := decodeCursor(cursor)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if decoded != id {
		t.Errorf("roundtrip failed: got %s, want %s", decoded, id)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	if _, err := decodeCursor("!!!not-base64!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	if _, err := decodeCursor("bm90LWEtdXVpZA"); err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestPrefixColumns(t *testing.T) {
	result := prefixColumns("e", "id, name, created_at")
	expected := "e.id, e.name, e.created_at"
	if result != expected {
		t.Errorf("prefixColumns = %q, want %q", result, expected)
	}
}

// --- Mock DB Pool ---

// mockRow implements pgx.Row for testing.
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

// mockRows implements pgx.Rows for testing.
type mockRows struct {
	closed   bool
	nextVals []bool
	nextIdx  int
	scanErr  error
	scanFn   func(dest ...any) error
	errVal   error
	// pgx.Rows has many methods; implement the ones called by our code
}

func (r *mockRows) Close()                                         {}
func (r *mockRows) Err() error                                     { return r.errVal }
func (r *mockRows) CommandTag() pgconn.CommandTag                  { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *mockRows) RawValues() [][]byte                            { return nil }
func (r *mockRows) Values() ([]any, error)                         { return nil, nil }
func (r *mockRows) Conn() *pgx.Conn                                { return nil }

func (r *mockRows) Next() bool {
	if r.nextIdx >= len(r.nextVals) {
		return false
	}
	val := r.nextVals[r.nextIdx]
	r.nextIdx++
	return val
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return r.scanErr
}

// mockTx implements pgx.Tx for testing.
type mockTx struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	commitErr  error
	rollbackErr error
}

func (t *mockTx) Begin(_ context.Context) (pgx.Tx, error)              { return nil, nil }
func (t *mockTx) Commit(_ context.Context) error                       { return t.commitErr }
func (t *mockTx) Rollback(_ context.Context) error                     { return t.rollbackErr }
func (t *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects                       { return pgx.LargeObjects{} }
func (t *mockTx) Prepare(_ context.Context, _ string, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *mockTx) Conn() *pgx.Conn { return nil }

func (t *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.execFn != nil {
		return t.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{}
}

type mockDBPool struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	beginTxFn  func(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

func (p *mockDBPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanErr: pgx.ErrNoRows}
}

func (p *mockDBPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (p *mockDBPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if p.execFn != nil {
		return p.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (p *mockDBPool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	if p.beginTxFn != nil {
		return p.beginTxFn(ctx, txOptions)
	}
	return &mockTx{}, nil
}

// --- Repository Tests ---

func TestPGRepository_NewRepository(t *testing.T) {
	// NewRepository accepts *pgxpool.Pool which is nil in unit tests.
	// Verify the constructor returns a non-nil struct.
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil PGRepository")
	}
}

func TestPGRepository_FindByID_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FindByID_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db connection error")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("should not be ErrNotFound for generic DB error")
	}
}

func TestPGRepository_FindByID_Success(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					// Fill in the 28 fields matching evidenceColumns
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil // thumbnail_key
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "description"
					*(dest[12].(*[]string)) = []string{"tag1"}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil // parent_id
					*dest[18].(*[]byte) = nil     // tsa_token
					*dest[19].(**string) = nil     // tsa_name
					*dest[20].(**time.Time) = nil  // tsa_timestamp
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0           // tsa_retry_count
					*dest[23].(**time.Time) = nil   // tsa_last_retry
					*dest[24].(*[]byte) = nil       // exif_data
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil   // destroyed_at
					*dest[28].(**string) = nil      // destroyed_by
					*dest[29].(**string) = nil      // destroy_reason
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
	}
	repo := &PGRepository{pool: pool}

	item, err := repo.FindByID(context.Background(), expectedID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if item.ID != expectedID {
		t.Errorf("ID = %s, want %s", item.ID, expectedID)
	}
	if item.Filename != fname {
		t.Errorf("Filename = %q, want %q", item.Filename, fname)
	}
}

func TestPGRepository_Create_Success(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
	}
	repo := &PGRepository{pool: pool}

	input := CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: evNum,
		Filename:       fname,
		OriginalName:   fname,
		StorageKey:     sKey,
		MimeType:       "application/pdf",
		SizeBytes:      1024,
		SHA256Hash:     "abc123",
		Classification: "restricted",
		Description:    "test",
		Tags:           []string{},
		UploadedBy:     "user-1",
		TSAStatus:      "disabled",
	}

	item, err := repo.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if item.ID != expectedID {
		t.Errorf("ID = %s, want %s", item.ID, expectedID)
	}
}

func TestPGRepository_Create_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("insert error")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.Create(context.Background(), CreateEvidenceInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_Update_NoChanges(t *testing.T) {
	// Update with empty changes should call FindByID
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
	}
	repo := &PGRepository{pool: pool}

	// No fields set in updates
	item, err := repo.Update(context.Background(), expectedID, EvidenceUpdate{})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if item.ID != expectedID {
		t.Errorf("ID = %s, want %s", item.ID, expectedID)
	}
}

func TestPGRepository_Update_WithFields(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}

	desc := "new desc"
	class := "confidential"
	_, err := repo.Update(context.Background(), uuid.New(), EvidenceUpdate{
		Description:    &desc,
		Classification: &class,
		Tags:           []string{"tag1"},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent ID, got %v", err)
	}
}

func TestPGRepository_Update_Success(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "confidential"
					*dest[11].(*string) = "updated desc"
					*(dest[12].(*[]string)) = []string{"tag1"}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
	}
	repo := &PGRepository{pool: pool}

	desc := "updated desc"
	item, err := repo.Update(context.Background(), expectedID, EvidenceUpdate{
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if item.Description != "updated desc" {
		t.Errorf("Description = %q, want %q", item.Description, "updated desc")
	}
}

func TestPGRepository_Update_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}
	repo := &PGRepository{pool: pool}

	desc := "new desc"
	_, err := repo.Update(context.Background(), uuid.New(), EvidenceUpdate{
		Description: &desc,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_MarkDestroyed_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.MarkDestroyed(context.Background(), uuid.New(), "reason", "admin")
	if err != nil {
		t.Fatalf("MarkDestroyed error: %v", err)
	}
}

func TestPGRepository_MarkDestroyed_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.MarkDestroyed(context.Background(), uuid.New(), "reason", "admin")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepository_MarkDestroyed_DBError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.MarkDestroyed(context.Background(), uuid.New(), "reason", "admin")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FindByHash_Success(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.FindByHash(context.Background(), uuid.New(), "abc123")
	if err != nil {
		t.Fatalf("FindByHash error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPGRepository_FindByHash_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindByHash(context.Background(), uuid.New(), "abc123")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FindPendingTSA_Success(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.FindPendingTSA(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindPendingTSA error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPGRepository_FindPendingTSA_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindPendingTSA(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_UpdateTSAResult(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.UpdateTSAResult(context.Background(), uuid.New(), []byte("token"), "tsa1", time.Now())
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_IncrementTSARetry(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.IncrementTSARetry(context.Background(), uuid.New())
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_MarkTSAFailed(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.MarkTSAFailed(context.Background(), uuid.New())
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_UpdateThumbnailKey(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.UpdateThumbnailKey(context.Background(), uuid.New(), "thumb/key")
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_UpdateVersionFields(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.UpdateVersionFields(context.Background(), uuid.New(), uuid.New(), 2)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_MarkPreviousVersions(t *testing.T) {
	tests := []struct {
		name    string
		execErr error
		wantErr bool
	}{
		{"success", nil, false},
		{"db error", errors.New("db error"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &mockDBPool{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					if tt.execErr != nil {
						return pgconn.CommandTag{}, tt.execErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			repo := &PGRepository{pool: pool}

			err := repo.MarkPreviousVersions(context.Background(), uuid.New())
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPGRepository_TryAdvisoryLock(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				},
			}
		},
	}
	repo := &PGRepository{pool: pool}

	acquired, err := repo.TryAdvisoryLock(context.Background(), 12345)
	if err != nil {
		t.Fatalf("TryAdvisoryLock error: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}
}

func TestPGRepository_TryAdvisoryLock_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.TryAdvisoryLock(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_ReleaseAdvisoryLock(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), nil
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.ReleaseAdvisoryLock(context.Background(), 12345)
	if err != nil {
		t.Fatalf("ReleaseAdvisoryLock error: %v", err)
	}
}

func TestPGRepository_ReleaseAdvisoryLock_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.ReleaseAdvisoryLock(context.Background(), 12345)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_IncrementEvidenceCounter_BeginTxError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return nil, errors.New("cannot begin tx")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.IncrementEvidenceCounter(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_IncrementEvidenceCounter_LockError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.CommandTag{}, errors.New("lock error")
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.IncrementEvidenceCounter(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_IncrementEvidenceCounter_QueryError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(""), nil
				},
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{scanErr: errors.New("query error")}
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.IncrementEvidenceCounter(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_IncrementEvidenceCounter_CommitError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(""), nil
				},
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{
						scanFn: func(dest ...any) error {
							*dest[0].(*int) = 5
							return nil
						},
					}
				},
				commitErr: errors.New("commit error"),
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.IncrementEvidenceCounter(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_IncrementEvidenceCounter_Success(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{
				execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag(""), nil
				},
				queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return &mockRow{
						scanFn: func(dest ...any) error {
							*dest[0].(*int) = 42
							return nil
						},
					}
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	counter, err := repo.IncrementEvidenceCounter(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("IncrementEvidenceCounter error: %v", err)
	}
	if counter != 42 {
		t.Errorf("counter = %d, want 42", counter)
	}
}

func TestPGRepository_FindVersionHistory_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindVersionHistory(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepository_FindByCase_QueryError(t *testing.T) {
	callCount := 0
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// First call is the count query
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			callCount++
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: uuid.New(),
	}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FindByCase_CountError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("count error")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: uuid.New(),
	}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FindByCase_WithFilters(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID:           uuid.New(),
		Classification:   "restricted",
		MimeType:         "image/jpeg",
		Tags:             []string{"tag1"},
		SearchQuery:      "search",
		CurrentOnly:      true,
		IncludeDestroyed: false,
		UserRole:         "defence",
	}, Pagination{Limit: 10, Cursor: encodeCursor(uuid.New())})
	if err != nil {
		t.Fatalf("FindByCase error: %v", err)
	}
}

func TestPGRepository_FindByCase_InvalidCursor(t *testing.T) {
	pool := &mockDBPool{}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: uuid.New(),
	}, Pagination{Limit: 10, Cursor: "!!!invalid!!!"})
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}

func TestPGRepository_FindByCase_SuccessWithRows(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	scanRow := func(dest ...any) error {
		*dest[0].(*uuid.UUID) = expectedID
		*dest[1].(*uuid.UUID) = caseID
		*dest[2].(**string) = &evNum
		*dest[3].(*string) = fname
		*dest[4].(*string) = fname
		*dest[5].(**string) = &sKey
		*dest[6].(**string) = nil
		*dest[7].(*string) = "application/pdf"
		*dest[8].(*int64) = 1024
		*dest[9].(*string) = "abc123"
		*dest[10].(*string) = "restricted"
		*dest[11].(*string) = "test"
		*(dest[12].(*[]string)) = []string{}
		*dest[13].(*string) = "user-1"
		*dest[14].(*string) = "Test User" // uploaded_by_name
		*dest[15].(*bool) = true
		*dest[16].(*int) = 1
		*dest[17].(**uuid.UUID) = nil
		*dest[18].(*[]byte) = nil
		*dest[19].(**string) = nil
		*dest[20].(**time.Time) = nil
		*dest[21].(*string) = "disabled"
		*dest[22].(*int) = 0
		*dest[23].(**time.Time) = nil
		*dest[24].(*[]byte) = nil
		*dest[25].(*string) = ""        // source
		*dest[26].(**time.Time) = nil   // source_date
		*dest[27].(**time.Time) = nil
		*dest[28].(**string) = nil
		*dest[29].(**string) = nil
		*dest[30].(*time.Time) = now
		return nil
	}

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 1
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn:   scanRow,
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, total, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: caseID,
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindByCase error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("items = %d, want 1", len(items))
	}
}

func TestPGRepository_FindByCase_ScanError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 1
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					return errors.New("scan error")
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: uuid.New(),
	}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from scan")
	}
}

func TestPGRepository_FindByHash_WithRows(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.FindByHash(context.Background(), caseID, "abc123")
	if err != nil {
		t.Fatalf("FindByHash error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestPGRepository_FindPendingTSA_WithRows(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()

	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(*string) = "abc123"
					*dest[3].(*int) = 0
					*dest[4].(*time.Time) = now
					return nil
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.FindPendingTSA(context.Background(), 10)
	if err != nil {
		t.Fatalf("FindPendingTSA error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestPGRepository_FindPendingTSA_ScanError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					return errors.New("scan error")
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindPendingTSA(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error from scan")
	}
}

func TestPGRepository_FindVersionHistory_Success(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	scanRow := func(dest ...any) error {
		*dest[0].(*uuid.UUID) = expectedID
		*dest[1].(*uuid.UUID) = caseID
		*dest[2].(**string) = &evNum
		*dest[3].(*string) = fname
		*dest[4].(*string) = fname
		*dest[5].(**string) = &sKey
		*dest[6].(**string) = nil
		*dest[7].(*string) = "application/pdf"
		*dest[8].(*int64) = 1024
		*dest[9].(*string) = "abc123"
		*dest[10].(*string) = "restricted"
		*dest[11].(*string) = "test"
		*(dest[12].(*[]string)) = []string{}
		*dest[13].(*string) = "user-1"
		*dest[14].(*string) = "Test User" // uploaded_by_name
		*dest[15].(*bool) = true
		*dest[16].(*int) = 1
		*dest[17].(**uuid.UUID) = nil
		*dest[18].(*[]byte) = nil
		*dest[19].(**string) = nil
		*dest[20].(**time.Time) = nil
		*dest[21].(*string) = "disabled"
		*dest[22].(*int) = 0
		*dest[23].(**time.Time) = nil
		*dest[24].(*[]byte) = nil
		*dest[25].(*string) = ""        // source
		*dest[26].(**time.Time) = nil   // source_date
		*dest[27].(**time.Time) = nil
		*dest[28].(**string) = nil
		*dest[29].(**string) = nil
		*dest[30].(*time.Time) = now
		return nil
	}

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: scanRow}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn:   scanRow,
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.FindVersionHistory(context.Background(), expectedID)
	if err != nil {
		t.Fatalf("FindVersionHistory error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestPGRepository_FindVersionHistory_WithParentID(t *testing.T) {
	expectedID := uuid.New()
	parentID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 2
					*dest[17].(**uuid.UUID) = &parentID // Has parent
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindVersionHistory(context.Background(), expectedID)
	if err != nil {
		t.Fatalf("FindVersionHistory error: %v", err)
	}
}

func TestPGRepository_FindVersionHistory_QueryError(t *testing.T) {
	expectedID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = expectedID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindVersionHistory(context.Background(), expectedID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FindByCase_TrimExtraResults(t *testing.T) {
	expectedID1 := uuid.New()
	expectedID2 := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	callCount := 0
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 5 // total more than limit
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			// Return limit+1 items to test trimming
			ids := []uuid.UUID{expectedID1, expectedID2}
			return &mockRows{
				nextVals: []bool{true, true, false},
				scanFn: func(dest ...any) error {
					idx := callCount
					callCount++
					if idx >= len(ids) {
						idx = len(ids) - 1
					}
					*dest[0].(*uuid.UUID) = ids[idx]
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &evNum
					*dest[3].(*string) = fname
					*dest[4].(*string) = fname
					*dest[5].(**string) = &sKey
					*dest[6].(**string) = nil
					*dest[7].(*string) = "application/pdf"
					*dest[8].(*int64) = 1024
					*dest[9].(*string) = "abc123"
					*dest[10].(*string) = "restricted"
					*dest[11].(*string) = "test"
					*(dest[12].(*[]string)) = []string{}
					*dest[13].(*string) = "user-1"
					*dest[14].(*string) = "Test User" // uploaded_by_name
					*dest[15].(*bool) = true
					*dest[16].(*int) = 1
					*dest[17].(**uuid.UUID) = nil
					*dest[18].(*[]byte) = nil
					*dest[19].(**string) = nil
					*dest[20].(**time.Time) = nil
					*dest[21].(*string) = "disabled"
					*dest[22].(*int) = 0
					*dest[23].(**time.Time) = nil
					*dest[24].(*[]byte) = nil
					*dest[25].(*string) = ""        // source
					*dest[26].(**time.Time) = nil   // source_date
					*dest[27].(**time.Time) = nil
					*dest[28].(**string) = nil
					*dest[29].(**string) = nil
					*dest[30].(*time.Time) = now
					return nil
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	// Use limit=1 so that 2 results > limit triggers trimming
	items, total, err := repo.FindByCase(context.Background(), EvidenceFilter{
		CaseID: caseID,
	}, Pagination{Limit: 1})
	if err != nil {
		t.Fatalf("FindByCase error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(items) != 1 {
		t.Errorf("items = %d, want 1 (trimmed)", len(items))
	}
}

func TestPGRepository_scanEvidenceRows_RowError(t *testing.T) {
	rows := &mockRows{
		nextVals: []bool{false},
		errVal:   errors.New("rows error"),
	}

	_, err := scanEvidenceRows(rows)
	if err == nil {
		t.Fatal("expected error from rows.Err()")
	}
}

func TestPGRepository_scanEvidenceRows_NilTags(t *testing.T) {
	now := time.Now()
	expectedID := uuid.New()
	caseID := uuid.New()
	evNum := "CASE-00001"
	fname := "test.pdf"
	sKey := "evidence/key"

	rows := &mockRows{
		nextVals: []bool{true, false},
		scanFn: func(dest ...any) error {
			*dest[0].(*uuid.UUID) = expectedID
			*dest[1].(*uuid.UUID) = caseID
			*dest[2].(**string) = &evNum
			*dest[3].(*string) = fname
			*dest[4].(*string) = fname
			*dest[5].(**string) = &sKey
			*dest[6].(**string) = nil
			*dest[7].(*string) = "application/pdf"
			*dest[8].(*int64) = 1024
			*dest[9].(*string) = "abc123"
			*dest[10].(*string) = "restricted"
			*dest[11].(*string) = "test"
			*(dest[12].(*[]string)) = nil // nil tags
			*dest[13].(*string) = "user-1"
			*dest[14].(*string) = "Test User" // uploaded_by_name
			*dest[15].(*bool) = true
			*dest[16].(*int) = 1
			*dest[17].(**uuid.UUID) = nil
			*dest[18].(*[]byte) = nil
			*dest[19].(**string) = nil
			*dest[20].(**time.Time) = nil
			*dest[21].(*string) = "disabled"
			*dest[22].(*int) = 0
			*dest[23].(**time.Time) = nil
			*dest[24].(*[]byte) = nil
			*dest[25].(*string) = ""        // source
			*dest[26].(**time.Time) = nil   // source_date
			*dest[27].(**time.Time) = nil
			*dest[28].(**string) = nil
			*dest[29].(**string) = nil
			*dest[30].(*time.Time) = now
			return nil
		},
	}

	items, err := scanEvidenceRows(rows)
	if err != nil {
		t.Fatalf("scanEvidenceRows error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	// Nil tags should be converted to empty slice
	if items[0].Tags == nil {
		t.Error("expected non-nil tags")
	}
}

func TestPGRepository_ListByCaseForExport_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListByCaseForExport(context.Background(), uuid.New(), "prosecution")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_ListByCaseForExport_Empty(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.ListByCaseForExport(context.Background(), uuid.New(), "prosecution")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestPGRepository_ListByCaseForExport_DefenceRole(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			// Verify the defence role adds the INNER JOIN clause
			if !contains(sql, "INNER JOIN disclosures") {
				t.Errorf("expected defence role to add INNER JOIN disclosures clause")
			}
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListByCaseForExport(context.Background(), uuid.New(), "defence")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPGRepository_ListForVerification_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListForVerification(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_ListForVerification_Empty(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.ListForVerification(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestPGRepository_ListForVerification_Success(t *testing.T) {
	testID := uuid.New()
	caseID := uuid.New()
	storageKey := "evidence/test/file.pdf"

	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanFn: func(dest ...any) error {
					*dest[0].(*uuid.UUID) = testID
					*dest[1].(*uuid.UUID) = caseID
					*dest[2].(**string) = &storageKey
					*dest[3].(*string) = "abc123hash"
					*dest[4].(*[]byte) = nil
					*dest[5].(*string) = "stamped"
					*dest[6].(*string) = "file.pdf"
					return nil
				},
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	items, err := repo.ListForVerification(context.Background(), caseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != testID {
		t.Errorf("ID = %s, want %s", items[0].ID, testID)
	}
	if items[0].StorageKey != storageKey {
		t.Errorf("StorageKey = %q, want %q", items[0].StorageKey, storageKey)
	}
}

func TestPGRepository_ListForVerification_ScanError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true, false},
				scanErr:  errors.New("scan error"),
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListForVerification(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGRepository_FlagIntegrityWarning_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.FlagIntegrityWarning(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPGRepository_FlagIntegrityWarning_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.FlagIntegrityWarning(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

// contains is a helper to check substring presence (avoids importing strings in test).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
