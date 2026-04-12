//go:build integration

package evidence

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// createTestEvidence creates an evidence item via the repository and returns its ID.
func createTestEvidence(t *testing.T, repo *PGRepository, caseID uuid.UUID, evidenceNumber, hash string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	ev, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: evidenceNumber,
		Filename:       evidenceNumber + ".pdf",
		OriginalName:   evidenceNumber + ".pdf",
		StorageKey:     "keys/" + evidenceNumber,
		MimeType:       "application/pdf",
		SizeBytes:      512,
		SHA256Hash:     hash,
		Classification: ClassificationPublic,
		Description:    "test evidence " + evidenceNumber,
		Tags:           []string{},
		UploadedBy:     "00000000-0000-4000-8000-000000000001",
		TSAStatus:      TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("createTestEvidence %s: %v", evidenceNumber, err)
	}
	return ev.ID
}

// --- Test 1: CreateDraft + FindDraftByID round-trip ---

func TestIntegration_Draft_CreateAndFindByID(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-001")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-001", strings.Repeat("1", 64))

	createdBy := uuid.New().String()
	draft, err := repo.CreateDraft(ctx, evID, caseID, "Defence Redaction v1", PurposeDisclosureDefence, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	if draft.ID == uuid.Nil {
		t.Fatal("expected non-nil draft ID")
	}
	if draft.EvidenceID != evID {
		t.Errorf("EvidenceID = %s, want %s", draft.EvidenceID, evID)
	}
	if draft.CaseID != caseID {
		t.Errorf("CaseID = %s, want %s", draft.CaseID, caseID)
	}
	if draft.Name != "Defence Redaction v1" {
		t.Errorf("Name = %q, want %q", draft.Name, "Defence Redaction v1")
	}
	if draft.Purpose != PurposeDisclosureDefence {
		t.Errorf("Purpose = %q, want %q", draft.Purpose, PurposeDisclosureDefence)
	}
	if draft.Status != "draft" {
		t.Errorf("Status = %q, want %q", draft.Status, "draft")
	}
	if draft.AreaCount != 0 {
		t.Errorf("AreaCount = %d, want 0", draft.AreaCount)
	}

	// Round-trip: FindDraftByID should return the same draft.
	found, yjsState, err := repo.FindDraftByID(ctx, draft.ID, evID)
	if err != nil {
		t.Fatalf("FindDraftByID: %v", err)
	}
	if found.ID != draft.ID {
		t.Errorf("found.ID = %s, want %s", found.ID, draft.ID)
	}
	if found.Name != draft.Name {
		t.Errorf("found.Name = %q, want %q", found.Name, draft.Name)
	}
	if found.Status != "draft" {
		t.Errorf("found.Status = %q, want draft", found.Status)
	}
	// yjs_state is NULL on creation.
	if yjsState != nil {
		t.Errorf("expected nil yjs_state on fresh draft, got %d bytes", len(yjsState))
	}
}

// --- Test 2: CreateDraft with duplicate name → unique constraint error ---

func TestIntegration_Draft_CreateDuplicate_UniqueConstraintError(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-002")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-002", strings.Repeat("2", 64))

	createdBy := uuid.New().String()
	_, err := repo.CreateDraft(ctx, evID, caseID, "Shared Name", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("first CreateDraft: %v", err)
	}

	// Second draft with the same name on the same evidence item should fail
	// (unique index: evidence_id, lower(name) WHERE status != 'discarded').
	_, err = repo.CreateDraft(ctx, evID, caseID, "Shared Name", PurposePublicRelease, createdBy)
	if err == nil {
		t.Fatal("expected error for duplicate draft name, got nil")
	}
	if !strings.Contains(err.Error(), "create redaction draft") {
		t.Errorf("error should wrap 'create redaction draft', got: %v", err)
	}
}

// --- Test 3: ListDrafts excludes discarded drafts ---

func TestIntegration_Draft_ListDrafts_ExcludesDiscarded(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-003")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-003", strings.Repeat("3", 64))

	createdBy := uuid.New().String()
	draft1, err := repo.CreateDraft(ctx, evID, caseID, "Active Draft", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	// Baseline: one draft exists before discard.
	drafts, err := repo.ListDrafts(ctx, evID)
	if err != nil {
		t.Fatalf("ListDrafts before discard: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft before discard, got %d", len(drafts))
	}

	// Discard the draft.
	if err := repo.DiscardDraft(ctx, draft1.ID, evID); err != nil {
		t.Fatalf("DiscardDraft: %v", err)
	}

	// After discard, list should be empty.
	drafts, err = repo.ListDrafts(ctx, evID)
	if err != nil {
		t.Fatalf("ListDrafts after discard: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("expected 0 drafts after discard, got %d", len(drafts))
	}
}

// --- Test 4: UpdateDraft with name/purpose change ---

func TestIntegration_Draft_UpdateDraft_NameAndPurposeChange(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-004")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-004", strings.Repeat("4", 64))

	createdBy := uuid.New().String()
	draft, err := repo.CreateDraft(ctx, evID, caseID, "Original Name", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	newName := "Renamed Draft"
	newPurpose := PurposeDisclosureDefence
	yjsState := []byte("fake-yjs-state")
	areaCount := 3

	updated, err := repo.UpdateDraft(ctx, draft.ID, evID, yjsState, areaCount, &newName, &newPurpose)
	if err != nil {
		t.Fatalf("UpdateDraft: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Name = %q, want %q", updated.Name, newName)
	}
	if updated.Purpose != newPurpose {
		t.Errorf("Purpose = %q, want %q", updated.Purpose, newPurpose)
	}
	if updated.AreaCount != areaCount {
		t.Errorf("AreaCount = %d, want %d", updated.AreaCount, areaCount)
	}
	if updated.Status != "draft" {
		t.Errorf("Status = %q, want draft", updated.Status)
	}

	// Verify yjs_state was persisted via FindDraftByID.
	_, retrievedYjs, err := repo.FindDraftByID(ctx, draft.ID, evID)
	if err != nil {
		t.Fatalf("FindDraftByID after update: %v", err)
	}
	if string(retrievedYjs) != string(yjsState) {
		t.Errorf("yjs_state = %q, want %q", retrievedYjs, yjsState)
	}
}

// --- Test 5: UpdateDraft with wrong evidenceID → no rows matched → error ---

func TestIntegration_Draft_UpdateDraft_WrongEvidenceID(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-005")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-005", strings.Repeat("5", 64))

	createdBy := uuid.New().String()
	draft, err := repo.CreateDraft(ctx, evID, caseID, "My Draft", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	wrongEvID := uuid.New()
	_, err = repo.UpdateDraft(ctx, draft.ID, wrongEvID, []byte("state"), 1, nil, nil)
	if err == nil {
		t.Fatal("expected error when using wrong evidenceID, got nil")
	}
}

// --- Test 6: DiscardDraft + verify ListDrafts no longer returns it ---

func TestIntegration_Draft_DiscardDraft_RemovedFromList(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-006")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-006", strings.Repeat("6", 64))

	createdBy := uuid.New().String()

	d1, err := repo.CreateDraft(ctx, evID, caseID, "Draft Alpha", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft alpha: %v", err)
	}
	d2, err := repo.CreateDraft(ctx, evID, caseID, "Draft Beta", PurposeCourtSubmission, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft beta: %v", err)
	}

	// Discard only the first draft.
	if err := repo.DiscardDraft(ctx, d1.ID, evID); err != nil {
		t.Fatalf("DiscardDraft: %v", err)
	}

	drafts, err := repo.ListDrafts(ctx, evID)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 remaining draft, got %d", len(drafts))
	}
	if drafts[0].ID != d2.ID {
		t.Errorf("remaining draft ID = %s, want %s", drafts[0].ID, d2.ID)
	}
}

// --- Test 7: DiscardDraft with wrong evidenceID → ErrNotFound ---

func TestIntegration_Draft_DiscardDraft_WrongEvidenceID_ErrNotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-007")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-007", strings.Repeat("7", 64))

	createdBy := uuid.New().String()
	draft, err := repo.CreateDraft(ctx, evID, caseID, "Draft Gamma", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	wrongEvID := uuid.New()
	err = repo.DiscardDraft(ctx, draft.ID, wrongEvID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Test 8: LockDraftForFinalize + MarkDraftApplied transaction flow ---

func TestIntegration_Draft_LockAndMarkApplied(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-008")
	evID := createTestEvidence(t, repo, caseID, "EV-DR-008", strings.Repeat("8", 64))

	createdBy := uuid.New().String()
	draft, err := repo.CreateDraft(ctx, evID, caseID, "Finalize Me", PurposeCourtSubmission, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}

	// Seed yjs state so LockDraftForFinalize returns meaningful data.
	yjsPayload := []byte("court-submission-state")
	_, err = repo.UpdateDraft(ctx, draft.ID, evID, yjsPayload, 5, nil, nil)
	if err != nil {
		t.Fatalf("UpdateDraft (seed state): %v", err)
	}

	// Begin transaction for the finalize flow.
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	locked, lockedYjs, err := repo.LockDraftForFinalize(ctx, tx, draft.ID)
	if err != nil {
		t.Fatalf("LockDraftForFinalize: %v", err)
	}
	if locked.ID != draft.ID {
		t.Errorf("locked draft ID = %s, want %s", locked.ID, draft.ID)
	}
	if string(lockedYjs) != string(yjsPayload) {
		t.Errorf("locked yjs = %q, want %q", lockedYjs, yjsPayload)
	}
	if locked.Status != "draft" {
		t.Errorf("locked.Status = %q, want draft", locked.Status)
	}

	// Mark as applied within the same transaction.
	if err := repo.MarkDraftApplied(ctx, tx, draft.ID, evID); err != nil {
		t.Fatalf("MarkDraftApplied: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// After commit the draft status should be "applied".
	appliedDraft, _, err := repo.FindDraftByID(ctx, draft.ID, evID)
	if err != nil {
		t.Fatalf("FindDraftByID after applied: %v", err)
	}
	if appliedDraft.Status != "applied" {
		t.Errorf("expected status 'applied', got %q", appliedDraft.Status)
	}

	// Applied drafts are excluded from ListDrafts (status != 'discarded' — but let's
	// confirm: 'applied' also != 'discarded', so it stays in the list. If the filter
	// only excludes 'discarded', applied drafts remain visible.)
	//
	// The query is: WHERE status != 'discarded'
	// So applied drafts DO appear. Verify this is the case.
	drafts, err := repo.ListDrafts(ctx, evID)
	if err != nil {
		t.Fatalf("ListDrafts after applied: %v", err)
	}
	found := false
	for _, d := range drafts {
		if d.ID == draft.ID {
			found = true
			if d.Status != "applied" {
				t.Errorf("draft in list has status %q, want applied", d.Status)
			}
		}
	}
	if !found {
		t.Error("applied draft should still appear in ListDrafts (only discarded are excluded)")
	}
}

// --- Test 9: ListFinalizedRedactions with redaction metadata ---

func TestIntegration_Draft_ListFinalizedRedactions(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-009")
	parentEvID := createTestEvidence(t, repo, caseID, "EV-DR-009-ORIG", strings.Repeat("9", 64))

	// Before any finalized redactions, list should be empty.
	finalized, err := repo.ListFinalizedRedactions(ctx, parentEvID)
	if err != nil {
		t.Fatalf("ListFinalizedRedactions (empty): %v", err)
	}
	if len(finalized) != 0 {
		t.Errorf("expected 0 finalized redactions, got %d", len(finalized))
	}

	// Create a derivative evidence item with full redaction metadata.
	redactionName := "Defence Copy"
	redactionPurpose := PurposeDisclosureDefence
	areaCount := 7
	now := time.Now().UTC().Truncate(time.Second)
	authorID := uuid.New()

	derived, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:               caseID,
		EvidenceNumber:       "EV-DR-009-DER",
		Filename:             "derived.pdf",
		OriginalName:         "derived.pdf",
		StorageKey:           "keys/derived",
		MimeType:             "application/pdf",
		SizeBytes:            256,
		SHA256Hash:           strings.Repeat("a", 64),
		Classification:       ClassificationRestricted,
		Tags:                 []string{},
		UploadedBy:           authorID.String(),
		UploadedByName:       "Officer Smith",
		TSAStatus:            TSAStatusPending,
		RedactionName:        &redactionName,
		RedactionPurpose:     &redactionPurpose,
		RedactionAreaCount:   &areaCount,
		RedactionAuthorID:    &authorID,
		RedactionFinalizedAt: &now,
	})
	if err != nil {
		t.Fatalf("Create derivative: %v", err)
	}

	// Set parent_id so ListFinalizedRedactions picks it up.
	if err := repo.UpdateVersionFields(ctx, derived.ID, parentEvID, 1); err != nil {
		t.Fatalf("UpdateVersionFields: %v", err)
	}

	finalized, err = repo.ListFinalizedRedactions(ctx, parentEvID)
	if err != nil {
		t.Fatalf("ListFinalizedRedactions: %v", err)
	}
	if len(finalized) != 1 {
		t.Fatalf("expected 1 finalized redaction, got %d", len(finalized))
	}
	f := finalized[0]
	if f.ID != derived.ID {
		t.Errorf("finalized ID = %s, want %s", f.ID, derived.ID)
	}
	if f.Name != redactionName {
		t.Errorf("finalized Name = %q, want %q", f.Name, redactionName)
	}
	if f.Purpose != redactionPurpose {
		t.Errorf("finalized Purpose = %q, want %q", f.Purpose, redactionPurpose)
	}
	if f.AreaCount != areaCount {
		t.Errorf("finalized AreaCount = %d, want %d", f.AreaCount, areaCount)
	}
	if f.Author == "" {
		t.Error("expected non-empty finalized Author")
	}
}

// --- Test 10: GetManagementView returns both finalized + active drafts ---

func TestIntegration_Draft_GetManagementView(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-010")
	parentEvID := createTestEvidence(t, repo, caseID, "EV-DR-010-ORIG", strings.Repeat("b", 64))

	createdBy := uuid.New().String()

	// Create two active drafts.
	_, err := repo.CreateDraft(ctx, parentEvID, caseID, "Draft One", PurposeInternalReview, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft one: %v", err)
	}
	_, err = repo.CreateDraft(ctx, parentEvID, caseID, "Draft Two", PurposeDisclosureDefence, createdBy)
	if err != nil {
		t.Fatalf("CreateDraft two: %v", err)
	}

	// Create one finalized derivative.
	redName := "Finalized Copy"
	redPurpose := PurposeCourtSubmission
	redCount := 4
	now := time.Now().UTC()
	authorID := uuid.New()

	derived, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:               caseID,
		EvidenceNumber:       "EV-DR-010-DER",
		Filename:             "final.pdf",
		OriginalName:         "final.pdf",
		StorageKey:           "keys/final",
		MimeType:             "application/pdf",
		SizeBytes:            128,
		SHA256Hash:           strings.Repeat("c", 64),
		Classification:       ClassificationPublic,
		Tags:                 []string{},
		UploadedBy:           authorID.String(),
		TSAStatus:            TSAStatusPending,
		RedactionName:        &redName,
		RedactionPurpose:     &redPurpose,
		RedactionAreaCount:   &redCount,
		RedactionAuthorID:    &authorID,
		RedactionFinalizedAt: &now,
	})
	if err != nil {
		t.Fatalf("Create derived: %v", err)
	}
	if err := repo.UpdateVersionFields(ctx, derived.ID, parentEvID, 1); err != nil {
		t.Fatalf("UpdateVersionFields: %v", err)
	}

	view, err := repo.GetManagementView(ctx, parentEvID)
	if err != nil {
		t.Fatalf("GetManagementView: %v", err)
	}
	if len(view.Finalized) != 1 {
		t.Errorf("Finalized count = %d, want 1", len(view.Finalized))
	}
	if len(view.Drafts) != 2 {
		t.Errorf("Drafts count = %d, want 2", len(view.Drafts))
	}
}

// --- Test 11: CheckEvidenceNumberExists true/false cases ---

func TestIntegration_Draft_CheckEvidenceNumberExists(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-011")

	// A number that does not exist yet should return false.
	exists, err := repo.CheckEvidenceNumberExists(ctx, "EV-DR-011-NOTEXIST")
	if err != nil {
		t.Fatalf("CheckEvidenceNumberExists (not exist): %v", err)
	}
	if exists {
		t.Error("expected false for non-existent evidence number")
	}

	// Create evidence with a known number.
	evNum := "EV-DR-011-REAL"
	createTestEvidence(t, repo, caseID, evNum, strings.Repeat("d", 64))

	// Now the number should exist.
	exists, err = repo.CheckEvidenceNumberExists(ctx, evNum)
	if err != nil {
		t.Fatalf("CheckEvidenceNumberExists (exists): %v", err)
	}
	if !exists {
		t.Error("expected true for existing evidence number")
	}
}

// --- Test 12: CreateWithTx + SetDerivativeParentWithTx transaction flow ---

func TestIntegration_Draft_CreateWithTx_And_SetDerivativeParentWithTx(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "DR-012")
	parentEvID := createTestEvidence(t, repo, caseID, "EV-DR-012-ORIG", strings.Repeat("e", 64))

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	redName := "Witness-Protected Version"
	redPurpose := PurposeWitnessProtection
	redCount := 2
	now := time.Now().UTC()
	authorID := uuid.New()

	derived, err := repo.CreateWithTx(ctx, tx, CreateEvidenceInput{
		CaseID:               caseID,
		EvidenceNumber:       "EV-DR-012-DER",
		Filename:             "protected.pdf",
		OriginalName:         "protected.pdf",
		StorageKey:           "keys/protected",
		MimeType:             "application/pdf",
		SizeBytes:            64,
		SHA256Hash:           strings.Repeat("f", 64),
		Classification:       ClassificationConfidential,
		Tags:                 []string{},
		UploadedBy:           authorID.String(),
		TSAStatus:            TSAStatusPending,
		RedactionName:        &redName,
		RedactionPurpose:     &redPurpose,
		RedactionAreaCount:   &redCount,
		RedactionAuthorID:    &authorID,
		RedactionFinalizedAt: &now,
	})
	if err != nil {
		t.Fatalf("CreateWithTx: %v", err)
	}
	if derived.ID == uuid.Nil {
		t.Fatal("expected non-nil derived evidence ID")
	}

	// Link the derivative to the parent within the same transaction.
	if err := repo.SetDerivativeParentWithTx(ctx, tx, derived.ID, parentEvID); err != nil {
		t.Fatalf("SetDerivativeParentWithTx: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// After commit: derived should appear in ListFinalizedRedactions for the parent.
	finalized, err := repo.ListFinalizedRedactions(ctx, parentEvID)
	if err != nil {
		t.Fatalf("ListFinalizedRedactions after commit: %v", err)
	}
	if len(finalized) != 1 {
		t.Fatalf("expected 1 finalized redaction, got %d", len(finalized))
	}
	if finalized[0].ID != derived.ID {
		t.Errorf("finalized ID = %s, want %s", finalized[0].ID, derived.ID)
	}

	// Derivative should have is_current = false and parent_id = parentEvID.
	found, err := repo.FindByID(ctx, derived.ID)
	if err != nil {
		t.Fatalf("FindByID derived: %v", err)
	}
	if found.IsCurrent {
		t.Error("expected is_current = false for derivative set via SetDerivativeParentWithTx")
	}
	if found.ParentID == nil || *found.ParentID != parentEvID {
		t.Errorf("ParentID = %v, want %s", found.ParentID, parentEvID)
	}
}
