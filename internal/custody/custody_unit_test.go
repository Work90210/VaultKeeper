package custody

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// pgx fakes (duplicated from custody_errors_test.go which has integration tag)
// ---------------------------------------------------------------------------

type unitFakeRow struct {
	scanErr error
	values  []any
}

func (r *unitFakeRow) Scan(dest ...any) error {
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

type unitFakeRows struct {
	scanErr error
	rowsErr error
	events  []Event
	pos     int
	closed  bool
}

func (r *unitFakeRows) Close()                                       { r.closed = true }
func (r *unitFakeRows) Err() error                                   { return r.rowsErr }
func (r *unitFakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *unitFakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *unitFakeRows) RawValues() [][]byte                          { return nil }
func (r *unitFakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *unitFakeRows) Conn() *pgx.Conn                              { return nil }
func (r *unitFakeRows) Next() bool {
	if r.scanErr != nil {
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

func (r *unitFakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.pos == 0 || r.pos > len(r.events) {
		return errors.New("unitFakeRows: Scan out of range")
	}
	e := r.events[r.pos-1]
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

type unitFakeTx struct {
	pgx.Tx

	execErr       error
	queryRow      pgx.Row
	commitErr     error
	execCallCount int
	execErrOnCall int
}

func (t *unitFakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	t.execCallCount++
	if t.execErrOnCall > 0 && t.execCallCount == t.execErrOnCall {
		return pgconn.CommandTag{}, t.execErr
	}
	if t.execErrOnCall == 0 && t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *unitFakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if t.queryRow != nil {
		return t.queryRow
	}
	return &unitFakeRow{scanErr: pgx.ErrNoRows}
}

func (t *unitFakeTx) Commit(_ context.Context) error    { return t.commitErr }
func (t *unitFakeTx) Rollback(_ context.Context) error   { return nil }
func (t *unitFakeTx) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, errors.New("not implemented")
}
func (t *unitFakeTx) Conn() *pgx.Conn { return nil }

type unitFakePool struct {
	beginTxErr error
	tx         *unitFakeTx
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (p *unitFakePool) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	if p.beginTxErr != nil {
		return nil, p.beginTxErr
	}
	return p.tx, nil
}

func (p *unitFakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &unitFakeRow{values: []any{0}}
}

func (p *unitFakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return &unitFakeRows{}, nil
}

var _ = fmt.Sprintf // avoid unused import

// ---------------------------------------------------------------------------
// chain.go — VerifyCaseChain unit tests with fakes (no DB)
// ---------------------------------------------------------------------------

// TestChainVerifier_EmptyChain_NoDB covers the empty-chain branch using fakes.
func TestChainVerifier_EmptyChain_NoDB(t *testing.T) {
	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil // no events
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	result, err := verifier.VerifyCaseChain(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("empty chain should be valid")
	}
	if result.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0", result.TotalEntries)
	}
	if result.VerifiedAt.IsZero() {
		t.Error("VerifiedAt should be set")
	}
}

// TestChainVerifier_ValidChain_NoDB covers VerifyCaseChain with a valid chain.
func TestChainVerifier_ValidChain_NoDB(t *testing.T) {
	caseID := uuid.New()
	e1 := Event{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CaseID:      caseID,
		EvidenceID:  uuid.Nil,
		Action:      "created",
		ActorUserID: "user-1",
		Detail:      `{"key":"val"}`,
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	e1.PreviousHash = ""
	e1.HashValue = ComputeLogHash("", e1)

	e2 := Event{
		ID:          uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CaseID:      caseID,
		EvidenceID:  uuid.Nil,
		Action:      "updated",
		ActorUserID: "user-2",
		Detail:      `{}`,
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC),
	}
	e2.PreviousHash = e1.HashValue
	e2.HashValue = ComputeLogHash(e1.HashValue, e2)

	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: []Event{e1, e2}}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	result, err := verifier.VerifyCaseChain(context.Background(), caseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("chain should be valid, breaks: %+v", result.Breaks)
	}
	if result.TotalEntries != 2 {
		t.Errorf("TotalEntries = %d, want 2", result.TotalEntries)
	}
}

// TestChainVerifier_BrokenHashValue_NoDB covers the hash_value mismatch branch.
func TestChainVerifier_BrokenHashValue_NoDB(t *testing.T) {
	caseID := uuid.New()
	e1 := Event{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CaseID:      caseID,
		Action:      "created",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	e1.PreviousHash = ""
	e1.HashValue = "tampered-hash" // Wrong hash

	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: []Event{e1}}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	result, err := verifier.VerifyCaseChain(context.Background(), caseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid when hash value is tampered")
	}
	if len(result.Breaks) == 0 {
		t.Error("expected at least one ChainBreak")
	}
	found := false
	for _, b := range result.Breaks {
		if b.ActualHash == "tampered-hash" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected break with ActualHash='tampered-hash', got %+v", result.Breaks)
	}
}

// TestChainVerifier_BrokenPreviousHash_NoDB covers the previous_hash mismatch branch.
func TestChainVerifier_BrokenPreviousHash_NoDB(t *testing.T) {
	caseID := uuid.New()
	e1 := Event{
		ID:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		CaseID:      caseID,
		Action:      "created",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	e1.PreviousHash = ""
	e1.HashValue = ComputeLogHash("", e1)

	e2 := Event{
		ID:          uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CaseID:      caseID,
		Action:      "updated",
		ActorUserID: "user-2",
		Detail:      "{}",
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC),
	}
	e2.PreviousHash = "wrong-previous-hash" // Broken link
	e2.HashValue = ComputeLogHash(e1.HashValue, e2)

	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: []Event{e1, e2}}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	result, err := verifier.VerifyCaseChain(context.Background(), caseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid when previous_hash is tampered")
	}
	found := false
	for _, b := range result.Breaks {
		if b.ActualHash == "wrong-previous-hash" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected break with ActualHash='wrong-previous-hash', got %+v", result.Breaks)
	}
}

// ---------------------------------------------------------------------------
// chain.go — canonicalJSON and sortedJSON
// ---------------------------------------------------------------------------

func TestCanonicalJSON_SortsKeys(t *testing.T) {
	result := canonicalJSON(`{"z":"last","a":"first","m":"middle"}`)
	expected := `{"a":"first","m":"middle","z":"last"}`
	if result != expected {
		t.Errorf("canonicalJSON = %q, want %q", result, expected)
	}
}

func TestCanonicalJSON_EmptyObject(t *testing.T) {
	result := canonicalJSON(`{}`)
	if result != "{}" {
		t.Errorf("canonicalJSON({}) = %q, want %q", result, "{}")
	}
}

func TestCanonicalJSON_InvalidJSON_ReturnsInput(t *testing.T) {
	input := "not valid json"
	result := canonicalJSON(input)
	if result != input {
		t.Errorf("canonicalJSON(invalid) = %q, want %q", result, input)
	}
}

func TestCanonicalJSON_NestedJSON(t *testing.T) {
	result := canonicalJSON(`{"b":{"d":1,"c":2},"a":"first"}`)
	if result == "" {
		t.Error("expected non-empty result for nested JSON")
	}
}

func TestSortedJSON_EmptyMap(t *testing.T) {
	result := sortedJSON(map[string]any{})
	if result != "{}" {
		t.Errorf("sortedJSON({}) = %q, want %q", result, "{}")
	}
}

// ---------------------------------------------------------------------------
// ComputeLogHash — additional edge cases
// ---------------------------------------------------------------------------

func TestComputeLogHash_WithEvidenceID(t *testing.T) {
	e := Event{
		ID:          uuid.New(),
		CaseID:      uuid.New(),
		EvidenceID:  uuid.New(), // non-nil evidence ID
		Action:      "evidence_uploaded",
		ActorUserID: "user-1",
		Detail:      `{"filename":"test.pdf"}`,
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	h := ComputeLogHash("prev", e)
	if h == "" {
		t.Error("expected non-empty hash")
	}
	if len(h) != 64 {
		t.Errorf("hash length = %d, want 64", len(h))
	}
}

func TestComputeLogHash_NilEvidenceID(t *testing.T) {
	e := Event{
		ID:          uuid.New(),
		CaseID:      uuid.New(),
		EvidenceID:  uuid.Nil,
		Action:      "case_created",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	}

	h := ComputeLogHash("", e)
	if h == "" {
		t.Error("expected non-empty hash with nil evidence ID")
	}
}

// ---------------------------------------------------------------------------
// Logger unit tests (require faking the repo)
// ---------------------------------------------------------------------------

func TestNewLogger_ReturnsNonNil(t *testing.T) {
	repo := &PGRepository{pool: &unitFakePool{tx: &unitFakeTx{}}}
	l := NewLogger(repo)
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestNewChainVerifier_ReturnsNonNil(t *testing.T) {
	repo := &PGRepository{pool: &unitFakePool{tx: &unitFakeTx{}}}
	v := NewChainVerifier(repo)
	if v == nil {
		t.Fatal("NewChainVerifier returned nil")
	}
}

func TestLogger_Record_PropagatesError(t *testing.T) {
	injected := errors.New("begin tx error")
	pool := &unitFakePool{beginTxErr: injected}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.Record(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from Record, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want to wrap injected error", err)
	}
}

func TestLogger_RecordEvidenceEvent_PropagatesError(t *testing.T) {
	injected := errors.New("begin tx error")
	pool := &unitFakePool{beginTxErr: injected}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.RecordEvidenceEvent(context.Background(),
		uuid.New(), uuid.New(), "evidence_uploaded", "user-1",
		map[string]string{"filename": "test.pdf"},
	)
	if err == nil {
		t.Fatal("expected error from RecordEvidenceEvent, got nil")
	}
}

func TestLogger_RecordCaseEvent_PropagatesError(t *testing.T) {
	injected := errors.New("begin tx error")
	pool := &unitFakePool{beginTxErr: injected}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.RecordCaseEvent(context.Background(),
		uuid.New(), "case_created", "user-1",
		map[string]string{"title": "Test Case"},
	)
	if err == nil {
		t.Fatal("expected error from RecordCaseEvent, got nil")
	}
}

// TestLogger_Record_Success covers the happy path where Insert succeeds.
func TestLogger_Record_Success(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.Record(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLogger_RecordEvidenceEvent_Success covers the happy path.
func TestLogger_RecordEvidenceEvent_Success(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.RecordEvidenceEvent(context.Background(),
		uuid.New(), uuid.New(), "uploaded", "user-1",
		map[string]string{"filename": "doc.pdf"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLogger_RecordCaseEvent_Success covers the happy path.
func TestLogger_RecordCaseEvent_Success(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}
	l := NewLogger(repo)

	err := l.RecordCaseEvent(context.Background(),
		uuid.New(), "case_created", "user-1",
		map[string]string{"title": "Test"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Repository unit tests — Insert happy path through fakes
// ---------------------------------------------------------------------------

func TestRepository_Insert_Success(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_Insert_WithExistingID(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	existingID := uuid.New()
	err := repo.Insert(context.Background(), Event{
		ID:          existingID,
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_Insert_WithNilEvidenceID(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		EvidenceID:  uuid.Nil, // should be converted to nil in SQL
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_Insert_WithNonNilEvidenceID(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		EvidenceID:  uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_Insert_WithPreviousHash(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{values: []any{"previous-hash-value"}},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID:      uuid.New(),
		Action:      "test",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ChainVerification / ChainBreak model tests
// ---------------------------------------------------------------------------

func TestChainVerification_Defaults(t *testing.T) {
	v := ChainVerification{}
	if v.Valid {
		t.Error("zero-value Valid should be false")
	}
	if v.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0", v.TotalEntries)
	}
}

func TestChainBreak_Fields(t *testing.T) {
	b := ChainBreak{
		EntryID:      uuid.New(),
		Position:     5,
		ExpectedHash: "expected",
		ActualHash:   "actual",
		Timestamp:    time.Now(),
	}
	if b.Position != 5 {
		t.Errorf("Position = %d, want 5", b.Position)
	}
	if b.ExpectedHash != "expected" {
		t.Errorf("ExpectedHash = %q", b.ExpectedHash)
	}
}

// ---------------------------------------------------------------------------
// NewRepository test
// ---------------------------------------------------------------------------

func TestNewRepository_ReturnsNonNil(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

// ---------------------------------------------------------------------------
// Repository.list — unit tests with fakes
// ---------------------------------------------------------------------------

func TestRepository_list_InvalidFilterCol_Unit(t *testing.T) {
	pool := &unitFakePool{}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "bad_col", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error for invalid filter column")
	}
}

func TestRepository_list_DefaultLimit_Unit(t *testing.T) {
	caseID := uuid.New()
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	events, total, err := repo.list(context.Background(), "case_id", caseID, 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(events) != 0 {
		t.Errorf("len = %d, want 0", len(events))
	}
}

func TestRepository_list_MaxLimitClamp_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 500, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_list_WithCursor_Unit(t *testing.T) {
	cursorID := uuid.New()
	// Base64 raw URL encode the cursor UUID
	cursor := b64Encode(cursorID.String())

	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{5}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, total, err := repo.list(context.Background(), "case_id", uuid.New(), 10, cursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
}

func TestRepository_list_InvalidCursorBase64_Unit(t *testing.T) {
	pool := &unitFakePool{}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "!!invalid!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 cursor")
	}
}

func TestRepository_list_InvalidCursorUUID_Unit(t *testing.T) {
	cursor := b64Encode("not-a-uuid")
	pool := &unitFakePool{}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, cursor)
	if err == nil {
		t.Fatal("expected error for non-UUID cursor")
	}
}

func TestRepository_list_CountQueryError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{scanErr: errors.New("count error")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from count query")
	}
}

func TestRepository_list_MainQueryError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from main query")
	}
}

func TestRepository_list_ScanError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{1}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{scanErr: errors.New("scan error")}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from rows.Scan")
	}
}

func TestRepository_list_RowsErrError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{rowsErr: errors.New("rows error")}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err == nil {
		t.Fatal("expected error from rows.Err")
	}
}

func TestRepository_list_TrimExcess_Unit(t *testing.T) {
	events := make([]Event, 4) // limit will be 3, so 4 > 3
	for i := range events {
		events[i] = Event{
			ID:          uuid.New(),
			CaseID:      uuid.New(),
			Action:      "test",
			ActorUserID: "user",
			Detail:      "{}",
			Timestamp:   time.Now().UTC(),
		}
	}

	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{10}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: events}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	result, _, err := repo.list(context.Background(), "case_id", uuid.New(), 3, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("len = %d, want 3 (trimmed from 4)", len(result))
	}
}

func TestRepository_list_WithEvents_Unit(t *testing.T) {
	events := []Event{
		{
			ID:          uuid.New(),
			CaseID:      uuid.New(),
			Action:      "created",
			ActorUserID: "user-1",
			Detail:      "{}",
			Timestamp:   time.Now().UTC(),
		},
	}

	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{1}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: events}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	result, total, err := repo.list(context.Background(), "case_id", uuid.New(), 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(result) != 1 {
		t.Errorf("len = %d, want 1", len(result))
	}
	if result[0].Action != "created" {
		t.Errorf("Action = %q", result[0].Action)
	}
}

// ---------------------------------------------------------------------------
// ListByCase and ListByEvidence delegate to list — verify wiring
// ---------------------------------------------------------------------------

func TestRepository_ListByCase_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.ListByCase(context.Background(), uuid.New(), 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_ListByEvidence_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &unitFakeRow{values: []any{0}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.ListByEvidence(context.Background(), uuid.New(), 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListAllByCase — unit tests
// ---------------------------------------------------------------------------

func TestRepository_ListAllByCase_QueryError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListAllByCase(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListAllByCase")
	}
}

func TestRepository_ListAllByCase_ScanError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{scanErr: errors.New("scan error")}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListAllByCase(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected scan error from ListAllByCase")
	}
}

func TestRepository_ListAllByCase_Success_Unit(t *testing.T) {
	events := []Event{
		{ID: uuid.New(), CaseID: uuid.New(), Action: "a", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC()},
		{ID: uuid.New(), CaseID: uuid.New(), Action: "b", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC()},
	}
	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &unitFakeRows{events: events}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	result, err := repo.ListAllByCase(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("len = %d, want 2", len(result))
	}
}

// b64Encode encodes a string as base64 raw URL encoding for cursor tests.
func b64Encode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// ---------------------------------------------------------------------------
// Insert — error path unit tests (duplicates of integration tests)
// ---------------------------------------------------------------------------

func TestRepository_Insert_BeginTxError_Unit(t *testing.T) {
	pool := &unitFakePool{beginTxErr: errors.New("begin error")}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID: uuid.New(), Action: "test", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from BeginTx")
	}
}

func TestRepository_Insert_AdvisoryLockError_Unit(t *testing.T) {
	tx := &unitFakeTx{execErr: errors.New("lock error")}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID: uuid.New(), Action: "test", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from advisory lock")
	}
}

func TestRepository_Insert_GetLastHashNonNoRowsError_Unit(t *testing.T) {
	tx := &unitFakeTx{
		queryRow: &unitFakeRow{scanErr: errors.New("query error")},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID: uuid.New(), Action: "test", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from get last hash")
	}
}

func TestRepository_Insert_ExecInsertError_Unit(t *testing.T) {
	tx := &unitFakeTx{
		execErr:       errors.New("insert error"),
		execErrOnCall: 2, // advisory lock OK (call 1), INSERT fails (call 2)
		queryRow:      &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID: uuid.New(), Action: "test", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from INSERT exec")
	}
}

func TestRepository_Insert_CommitError_Unit(t *testing.T) {
	tx := &unitFakeTx{
		commitErr: errors.New("commit error"),
		queryRow:  &unitFakeRow{scanErr: pgx.ErrNoRows},
	}
	pool := &unitFakePool{tx: tx}
	repo := &PGRepository{pool: pool}

	err := repo.Insert(context.Background(), Event{
		CaseID: uuid.New(), Action: "test", ActorUserID: "u", Detail: "{}", Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error from Commit")
	}
}

// ---------------------------------------------------------------------------
// VerifyCaseChain — ListAllByCase error unit test
// ---------------------------------------------------------------------------

func TestChainVerifier_ListAllByCaseError_Unit(t *testing.T) {
	pool := &unitFakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}
	verifier := NewChainVerifier(repo)

	_, err := verifier.VerifyCaseChain(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from VerifyCaseChain when ListAllByCase fails")
	}
}
