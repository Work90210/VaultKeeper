//go:build integration

package custody

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createTestEvidence inserts a minimal evidence_items row linked to caseID and
// returns the new evidence ID. Used by tests that exercise the evidence-scoped
// list path.
func createTestEvidence(t *testing.T, pool *pgxpool.Pool, caseID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO evidence_items
		   (case_id, filename, original_name, mime_type, size_bytes, sha256_hash, uploaded_by)
		 VALUES ($1, 'test.pdf', 'test.pdf', 'application/pdf', 1024, $2, $3)
		 RETURNING id`,
		caseID,
		"sha256-"+uuid.New().String(),
		uuid.New(),
	).Scan(&id)
	if err != nil {
		t.Fatalf("createTestEvidence: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Logger.Record
// ---------------------------------------------------------------------------

// TestLogger_Record covers the Record method on Logger, which was previously
// untested (only RecordCaseEvent was exercised).
func TestLogger_Record(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	logger := NewLogger(repo)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	event := Event{
		CaseID:      caseID,
		EvidenceID:  uuid.Nil,
		Action:      "evidence_uploaded",
		ActorUserID: uuid.New().String(),
		Detail:      `{"filename":"report.pdf"}`,
		Timestamp:   time.Now().UTC(),
	}

	if err := logger.Record(ctx, event); err != nil {
		t.Fatalf("Record: %v", err)
	}

	events, total, err := repo.ListByCase(ctx, caseID, 10, "")
	if err != nil {
		t.Fatalf("ListByCase after Record: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(events) != 1 {
		t.Fatalf("len = %d, want 1", len(events))
	}
	if events[0].Action != "evidence_uploaded" {
		t.Errorf("Action = %q, want %q", events[0].Action, "evidence_uploaded")
	}
}

// ---------------------------------------------------------------------------
// ListByEvidence (previously zero coverage)
// ---------------------------------------------------------------------------

// TestRepository_ListByEvidence_Happy exercises the normal ListByEvidence path.
func TestRepository_ListByEvidence_Happy(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)
	evidenceID := createTestEvidence(t, pool, caseID)

	for i := 0; i < 2; i++ {
		err := repo.Insert(ctx, Event{
			CaseID:      caseID,
			EvidenceID:  evidenceID,
			Action:      "evidence_tagged",
			ActorUserID: uuid.New().String(),
			Detail:      `{}`,
			Timestamp:   time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}

	events, total, err := repo.ListByEvidence(ctx, evidenceID, 10, "")
	if err != nil {
		t.Fatalf("ListByEvidence: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}
}

// ---------------------------------------------------------------------------
// Invalid cursor — base64 decode failure
// ---------------------------------------------------------------------------

// TestRepository_ListByCase_InvalidCursor_Base64 exercises the base64 decode
// error branch in list() when the cursor string is not valid base64.
func TestRepository_ListByCase_InvalidCursor_Base64(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	_, _, err := repo.ListByCase(ctx, caseID, 10, "!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 cursor, got nil")
	}
}

// TestRepository_ListByEvidence_InvalidCursor_Base64 exercises the same base64
// error branch via the ListByEvidence path.
func TestRepository_ListByEvidence_InvalidCursor_Base64(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)
	evidenceID := createTestEvidence(t, pool, caseID)

	_, _, err := repo.ListByEvidence(ctx, evidenceID, 10, "!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 cursor, got nil")
	}
}

// ---------------------------------------------------------------------------
// Invalid cursor — valid base64 but not a UUID
// ---------------------------------------------------------------------------

// TestRepository_ListByCase_InvalidCursor_UUID exercises the uuid.Parse error
// branch: the cursor is valid base64 but does not decode to a UUID string.
func TestRepository_ListByCase_InvalidCursor_UUID(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	notUUID := base64.RawURLEncoding.EncodeToString([]byte("not-a-uuid"))
	_, _, err := repo.ListByCase(ctx, caseID, 10, notUUID)
	if err == nil {
		t.Fatal("expected UUID parse error for non-UUID cursor, got nil")
	}
}

// TestRepository_ListByEvidence_InvalidCursor_UUID exercises the same UUID parse
// error branch via ListByEvidence.
func TestRepository_ListByEvidence_InvalidCursor_UUID(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)
	evidenceID := createTestEvidence(t, pool, caseID)

	notUUID := base64.RawURLEncoding.EncodeToString([]byte("not-a-uuid"))
	_, _, err := repo.ListByEvidence(ctx, evidenceID, 10, notUUID)
	if err == nil {
		t.Fatal("expected UUID parse error for non-UUID cursor, got nil")
	}
}

// ---------------------------------------------------------------------------
// Valid cursor (pagination happy path)
// ---------------------------------------------------------------------------

// TestRepository_ListByCase_WithValidCursor exercises the full cursor pagination
// code path (the conditions block that appends the id < $N clause).
func TestRepository_ListByCase_WithValidCursor(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	var firstID uuid.UUID
	for i := 0; i < 3; i++ {
		id := uuid.New()
		if i == 0 {
			firstID = id
		}
		err := repo.Insert(ctx, Event{
			ID:          id,
			CaseID:      caseID,
			EvidenceID:  uuid.Nil,
			Action:      "event",
			ActorUserID: uuid.New().String(),
			Detail:      `{}`,
			Timestamp:   time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}

	// Use the first-inserted ID as a cursor; the list orders DESC so rows with
	// id < firstID would be returned (likely empty, but the branch is exercised).
	cursor := base64.RawURLEncoding.EncodeToString([]byte(firstID.String()))

	_, total, err := repo.ListByCase(ctx, caseID, 10, cursor)
	if err != nil {
		t.Fatalf("ListByCase with cursor: %v", err)
	}
	// total reflects the full case count regardless of cursor
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

// ---------------------------------------------------------------------------
// VerifyCaseChain — previousHash mismatch branch
// ---------------------------------------------------------------------------

// TestChainVerifier_PreviousHashMismatch covers the branch in VerifyCaseChain
// where e.PreviousHash != previousHash (lines 56-64 of chain.go). We corrupt
// the previous_hash column directly in the database after inserting clean events.
func TestChainVerifier_PreviousHashMismatch(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	verifier := NewChainVerifier(repo)
	ctx := context.Background()
	caseID := createTestCase(t, pool)

	// Insert 2 events to establish a two-entry chain.
	for i := 0; i < 2; i++ {
		err := repo.Insert(ctx, Event{
			CaseID:      caseID,
			EvidenceID:  uuid.Nil,
			Action:      "event",
			ActorUserID: uuid.New().String(),
			Detail:      `{}`,
			Timestamp:   time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("Insert %d: %v", i, err)
		}
	}

	// Collect the event IDs ordered ascending (same order the verifier sees them).
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

	if len(ids) < 2 {
		t.Fatalf("expected 2 events, got %d", len(ids))
	}

	// Corrupt the previous_hash of the second entry so the verifier detects a
	// mismatch between e.PreviousHash and the running previousHash variable.
	_, err = pool.Exec(ctx,
		`UPDATE custody_log SET previous_hash = 'corrupted-previous-hash' WHERE id = $1`,
		ids[1],
	)
	if err != nil {
		t.Fatalf("corrupt previous_hash: %v", err)
	}

	result, err := verifier.VerifyCaseChain(ctx, caseID)
	if err != nil {
		t.Fatalf("VerifyCaseChain: %v", err)
	}
	if result.Valid {
		t.Error("chain should be invalid after corrupting previous_hash")
	}
	if len(result.Breaks) == 0 {
		t.Error("expected at least one ChainBreak")
	}

	// At least one break should capture "corrupted-previous-hash" as the ActualHash
	// (the value stored in previous_hash that does not match what the verifier
	// computed as the correct previous hash).
	found := false
	for _, b := range result.Breaks {
		if b.ActualHash == "corrupted-previous-hash" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no ChainBreak with ActualHash == 'corrupted-previous-hash'; breaks: %+v", result.Breaks)
	}
}
