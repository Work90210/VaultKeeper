package evidence

// Coverage tests for the draft/retention/destruction/CreateWithTx
// methods on PGRepository, driven via mockDBPool and mockTx.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// draftScan fills a RedactionDraft result row.
func draftScan(d RedactionDraft, yjs []byte, includeYjs bool) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*uuid.UUID) = d.ID
		*dest[1].(*uuid.UUID) = d.EvidenceID
		*dest[2].(*uuid.UUID) = d.CaseID
		*dest[3].(*string) = d.Name
		*dest[4].(*RedactionPurpose) = d.Purpose
		*dest[5].(*int) = d.AreaCount
		*dest[6].(*string) = d.CreatedBy
		*dest[7].(*string) = d.Status
		*dest[8].(*time.Time) = d.LastSavedAt
		*dest[9].(*time.Time) = d.CreatedAt
		if includeYjs && len(dest) > 10 {
			*dest[10].(*[]byte) = yjs
		}
		return nil
	}
}

// ---- CreateDraft ----

func TestCreateDraft_Success(t *testing.T) {
	want := RedactionDraft{ID: uuid.New(), EvidenceID: uuid.New(), CaseID: uuid.New(), Name: "n", Purpose: RedactionPurpose("internal_review")}
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: draftScan(want, nil, false)}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.CreateDraft(context.Background(), want.EvidenceID, want.CaseID, "n", want.Purpose, "creator")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID mismatch")
	}
}

func TestCreateDraft_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("insert failed")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.CreateDraft(context.Background(), uuid.New(), uuid.New(), "n", "internal_review", "actor")
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- FindDraftByID ----

func TestFindDraftByID_Success(t *testing.T) {
	want := RedactionDraft{ID: uuid.New(), EvidenceID: uuid.New()}
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: draftScan(want, []byte("yjs"), true)}
		},
	}
	repo := &PGRepository{pool: pool}
	got, yjs, err := repo.FindDraftByID(context.Background(), want.ID, want.EvidenceID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != want.ID || string(yjs) != "yjs" {
		t.Errorf("mismatch")
	}
}

func TestFindDraftByID_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindDraftByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestFindDraftByID_OtherError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db down")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindDraftByID(context.Background(), uuid.New(), uuid.New())
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("want generic error, got %v", err)
	}
}

// ---- ListDrafts ----

type draftRows struct {
	drafts []RedactionDraft
	idx    int
	err    error
}

func (r *draftRows) Close()                                       {}
func (r *draftRows) Err() error                                   { return r.err }
func (r *draftRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *draftRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *draftRows) RawValues() [][]byte                          { return nil }
func (r *draftRows) Values() ([]any, error)                       { return nil, nil }
func (r *draftRows) Conn() *pgx.Conn                              { return nil }
func (r *draftRows) Next() bool {
	if r.idx >= len(r.drafts) {
		return false
	}
	r.idx++
	return true
}
func (r *draftRows) Scan(dest ...any) error {
	return draftScan(r.drafts[r.idx-1], nil, false)(dest...)
}

func TestListDrafts_Success(t *testing.T) {
	want := []RedactionDraft{
		{ID: uuid.New(), Name: "d1"},
		{ID: uuid.New(), Name: "d2"},
	}
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &draftRows{drafts: want}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.ListDrafts(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d", len(got))
	}
}

func TestListDrafts_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.ListDrafts(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- UpdateDraft ----

func TestUpdateDraft_WithNameAndPurpose(t *testing.T) {
	want := RedactionDraft{ID: uuid.New()}
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: draftScan(want, nil, false)}
		},
	}
	repo := &PGRepository{pool: pool}
	name := "updated"
	purpose := RedactionPurpose("internal_review")
	_, err := repo.UpdateDraft(context.Background(), uuid.New(), uuid.New(), []byte("yjs"), 5, &name, &purpose)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestUpdateDraft_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("update failed")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.UpdateDraft(context.Background(), uuid.New(), uuid.New(), []byte("y"), 1, nil, nil)
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- DiscardDraft ----

func TestDiscardDraft_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: pool}
	if err := repo.DiscardDraft(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestDiscardDraft_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.DiscardDraft(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestDiscardDraft_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec failed")
		},
	}
	repo := &PGRepository{pool: pool}
	if err := repo.DiscardDraft(context.Background(), uuid.New(), uuid.New()); err == nil {
		t.Fatal("want error")
	}
}

// ---- LockDraftForFinalize + MarkDraftApplied (tx-based) ----

func TestLockDraftForFinalize_Error(t *testing.T) {
	tx := &mockTx{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("lock failed")}
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}
	_, _, err := repo.LockDraftForFinalize(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("want error")
	}
}

func TestMarkDraftApplied_Error(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("mark applied failed")
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}
	if err := repo.MarkDraftApplied(context.Background(), tx, uuid.New(), uuid.New()); err == nil {
		t.Fatal("want error")
	}
}

// ---- ListFinalizedRedactions ----

type finalizedRows struct {
	items []FinalizedRedaction
	idx   int
	err   error
}

func (r *finalizedRows) Close()                                       {}
func (r *finalizedRows) Err() error                                   { return r.err }
func (r *finalizedRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *finalizedRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *finalizedRows) RawValues() [][]byte                          { return nil }
func (r *finalizedRows) Values() ([]any, error)                       { return nil, nil }
func (r *finalizedRows) Conn() *pgx.Conn                              { return nil }
func (r *finalizedRows) Next() bool {
	if r.idx >= len(r.items) {
		return false
	}
	r.idx++
	return true
}
func (r *finalizedRows) Scan(dest ...any) error {
	it := r.items[r.idx-1]
	*dest[0].(*uuid.UUID) = it.ID
	*dest[1].(*string) = it.EvidenceNumber
	*dest[2].(*string) = it.Name
	*dest[3].(*RedactionPurpose) = it.Purpose
	*dest[4].(*int) = it.AreaCount
	*dest[5].(*string) = it.Author
	*dest[6].(*time.Time) = it.FinalizedAt
	return nil
}

func TestListFinalizedRedactions_Success(t *testing.T) {
	want := []FinalizedRedaction{
		{ID: uuid.New(), EvidenceNumber: "E1", Name: "redact1"},
	}
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &finalizedRows{items: want}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.ListFinalizedRedactions(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d", len(got))
	}
}

func TestListFinalizedRedactions_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.ListFinalizedRedactions(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- CheckEvidenceNumberExists ----

func TestCheckEvidenceNumberExists_True(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	exists, err := repo.CheckEvidenceNumberExists(context.Background(), "ICC-001")
	if err != nil || !exists {
		t.Errorf("got %v err=%v", exists, err)
	}
}

func TestCheckEvidenceNumberExists_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db err")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.CheckEvidenceNumberExists(context.Background(), "ICC-001")
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- CreateWithTx ----

func TestCreateWithTx_Success(t *testing.T) {
	id := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	tx := &mockTx{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = id
				*dest[1].(*uuid.UUID) = caseID
				// Fill remaining fields with zero values.
				evNum := "E1"
				*dest[2].(**string) = &evNum
				*dest[3].(*string) = "file.pdf"
				*dest[4].(*string) = "file.pdf"
				*dest[5].(**string) = nil
				*dest[6].(**string) = nil
				*dest[7].(*string) = "application/pdf"
				*dest[8].(*int64) = 0
				*dest[9].(*string) = "hash"
				*dest[10].(*string) = "restricted"
				*dest[11].(*string) = ""
				*(dest[12].(*[]string)) = []string{}
				*dest[13].(*string) = ""
				*dest[14].(*string) = ""
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
				*dest[25].(*string) = ""
				*dest[26].(**time.Time) = nil
				*dest[27].(**string) = nil
				*dest[28].(**time.Time) = nil
				*dest[29].(**string) = nil
				*dest[30].(**string) = nil
				*dest[31].(*time.Time) = now
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}
	_, err := repo.CreateWithTx(context.Background(), tx, CreateEvidenceInput{
		CaseID: caseID, Filename: "file.pdf", OriginalName: "file.pdf",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

// ---- FindExpiringRetention ----

type expiringRows struct {
	items []ExpiringRetentionItem
	idx   int
	err   error
}

func (r *expiringRows) Close()                                       {}
func (r *expiringRows) Err() error                                   { return r.err }
func (r *expiringRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *expiringRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *expiringRows) RawValues() [][]byte                          { return nil }
func (r *expiringRows) Values() ([]any, error)                       { return nil, nil }
func (r *expiringRows) Conn() *pgx.Conn                              { return nil }
func (r *expiringRows) Next() bool {
	if r.idx >= len(r.items) {
		return false
	}
	r.idx++
	return true
}
func (r *expiringRows) Scan(dest ...any) error {
	it := r.items[r.idx-1]
	*dest[0].(*uuid.UUID) = it.EvidenceID
	*dest[1].(*uuid.UUID) = it.CaseID
	*dest[2].(*string) = it.EvidenceNumber
	*dest[3].(*time.Time) = it.RetentionUntil
	return nil
}

func TestFindExpiringRetention_Success(t *testing.T) {
	want := []ExpiringRetentionItem{{EvidenceID: uuid.New(), CaseID: uuid.New()}}
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &expiringRows{items: want}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.FindExpiringRetention(context.Background(), time.Now())
	if err != nil || len(got) != 1 {
		t.Errorf("got %v err=%v", got, err)
	}
}

func TestFindExpiringRetention_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query err")
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.FindExpiringRetention(context.Background(), time.Now())
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- GetCaseRetention ----

func TestGetCaseRetention_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.GetCaseRetention(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestGetCaseRetention_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db err")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.GetCaseRetention(context.Background(), uuid.New())
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("want generic error, got %v", err)
	}
}

func TestGetCaseRetention_Success(t *testing.T) {
	when := time.Now()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(**time.Time)) = &when
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.GetCaseRetention(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil || !got.Equal(when) {
		t.Errorf("got %v, want %v", got, when)
	}
}

// ---- DestroyWithAuthority ----

func TestDestroyWithAuthority_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: pool}
	if err := repo.DestroyWithAuthority(context.Background(), uuid.New(), "authority", "actor"); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestDestroyWithAuthority_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.DestroyWithAuthority(context.Background(), uuid.New(), "authority", "actor")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestDestroyWithAuthority_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec err")
		},
	}
	repo := &PGRepository{pool: pool}
	if err := repo.DestroyWithAuthority(context.Background(), uuid.New(), "authority", "actor"); err == nil {
		t.Fatal("want error")
	}
}

// ---- Update with ExpectedClassification conflict ----

func TestUpdate_ExpectedClassificationConflict(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	expected := "restricted"
	newClass := "confidential"
	_, err := repo.Update(context.Background(), uuid.New(), EvidenceUpdate{
		Classification:         &newClass,
		ExpectedClassification: &expected,
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("want ErrConflict, got %v", err)
	}
}
