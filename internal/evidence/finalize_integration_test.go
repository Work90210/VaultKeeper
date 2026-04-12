//go:build integration

package evidence

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// inMemStorage, noopCustody, and createSmallPNG are defined in
// test_helpers_test.go (untagged) so they are available to both unit and
// integration test builds.

func TestIntegration_FinalizeFromDraft_Success(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}

	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	// Seed case + evidence
	caseID := seedCase(t, pool, "CR-FIN-001")
	counter, err := repo.IncrementEvidenceCounter(ctx, caseID)
	if err != nil {
		t.Fatalf("increment counter: %v", err)
	}

	pngData := createSmallPNG(100, 100)
	storageKey := "evidence/test/original.png"
	storage.objects[storageKey] = pngData

	evidNum := "CR-FIN-001-" + strings.Repeat("0", 4) + string(rune('0'+counter))
	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: evidNum,
		Filename:       "original.png",
		OriginalName:   "original.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     strings.Repeat("a", 64),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "test-user",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create original: %v", err)
	}

	// Create a draft
	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Defence Q1", PurposeDisclosureDefence, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Save areas to the draft
	areas := draftState{
		Areas: []draftArea{
			{ID: uuid.New().String(), Page: 0, X: 10, Y: 10, W: 20, H: 20, Reason: "witness identity", Author: "tester"},
		},
	}
	areasJSON, _ := json.Marshal(areas)
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	// Finalize
	actorID := uuid.New()
	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID:     original.ID,
		DraftID:        draft.ID,
		Description:    "Q1 defence disclosure copy",
		Classification: ClassificationRestricted,
		ActorID:        actorID.String(),
		ActorName:      "test-prosecutor",
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft: %v", err)
	}

	// Verify result
	if result.NewEvidenceID == uuid.Nil {
		t.Fatal("new evidence ID is nil")
	}
	if result.OriginalID != original.ID {
		t.Errorf("original ID mismatch: got %s, want %s", result.OriginalID, original.ID)
	}
	if result.RedactionCount != 1 {
		t.Errorf("redaction count: got %d, want 1", result.RedactionCount)
	}
	if result.NewHash == "" {
		t.Error("new hash is empty")
	}

	// Verify the new evidence item
	newEvidence, err := repo.FindByID(ctx, result.NewEvidenceID)
	if err != nil {
		t.Fatalf("find new evidence: %v", err)
	}
	if newEvidence.ParentID == nil || *newEvidence.ParentID != original.ID {
		t.Error("parent_id should point to original")
	}
	if newEvidence.IsCurrent {
		t.Error("redacted derivative should not be current")
	}
	if newEvidence.RedactionName == nil || *newEvidence.RedactionName != "Defence Q1" {
		t.Errorf("redaction_name: got %v, want 'Defence Q1'", newEvidence.RedactionName)
	}
	if newEvidence.RedactionPurpose == nil || *newEvidence.RedactionPurpose != PurposeDisclosureDefence {
		t.Errorf("redaction_purpose: got %v, want disclosure_defence", newEvidence.RedactionPurpose)
	}
	if newEvidence.RedactionAreaCount == nil || *newEvidence.RedactionAreaCount != 1 {
		t.Errorf("redaction_area_count: got %v, want 1", newEvidence.RedactionAreaCount)
	}
	if newEvidence.RedactionAuthorID == nil || *newEvidence.RedactionAuthorID != actorID {
		t.Errorf("redaction_author_id: got %v, want %s", newEvidence.RedactionAuthorID, actorID)
	}
	if newEvidence.RedactionFinalizedAt == nil {
		t.Error("redaction_finalized_at should be set")
	}

	// Verify evidence number format
	if !strings.Contains(derefStr(newEvidence.EvidenceNumber), "-R-DEFENCE-") {
		t.Errorf("evidence number should contain -R-DEFENCE-, got %s", derefStr(newEvidence.EvidenceNumber))
	}

	// Verify draft is marked as applied
	appliedDraft, _, err := repo.FindDraftByID(ctx, draft.ID, original.ID)
	if err != nil {
		t.Fatalf("find applied draft: %v", err)
	}
	if appliedDraft.Status != "applied" {
		t.Errorf("draft status: got %s, want applied", appliedDraft.Status)
	}

	// Verify the redacted file was stored
	foundFile := false
	for key := range storage.objects {
		if strings.Contains(key, "redacted_") {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Error("redacted file not found in storage")
	}

	// Verify ListFinalizedRedactions returns it
	finalized, err := repo.ListFinalizedRedactions(ctx, original.ID)
	if err != nil {
		t.Fatalf("list finalized: %v", err)
	}
	if len(finalized) != 1 {
		t.Fatalf("expected 1 finalized, got %d", len(finalized))
	}
	if finalized[0].Name != "Defence Q1" {
		t.Errorf("finalized name: got %s, want Defence Q1", finalized[0].Name)
	}
}

func TestIntegration_FinalizeFromDraft_AlreadyApplied(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-FIN-002")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/applied.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-APPLIED-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("b", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create original: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Test", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areas := draftState{Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "test"}}}
	areasJSON, _ := json.Marshal(areas)
	_, _ = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)

	// Finalize once
	actorID := uuid.New()
	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID, DraftID: draft.ID,
		ActorID: actorID.String(), ActorName: "user",
	})
	if err != nil {
		t.Fatalf("first finalize: %v", err)
	}

	// Try to finalize again — should fail
	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID, DraftID: draft.ID,
		ActorID: actorID.String(), ActorName: "user",
	})
	if err == nil {
		t.Fatal("expected error on double finalization")
	}
	if !strings.Contains(err.Error(), "already been applied") {
		t.Errorf("expected 'already been applied' error, got: %v", err)
	}
}

func TestIntegration_FinalizeFromDraft_WrongEvidenceID(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-FIN-003")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/wrong.png"
	storage.objects[storageKey] = pngData

	original, _ := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-WRONG-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("c", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})

	draft, _ := repo.CreateDraft(ctx, original.ID, caseID, "Draft", PurposeInternalReview, uuid.New().String())
	areas := draftState{Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "test"}}}
	areasJSON, _ := json.Marshal(areas)
	_, _ = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)

	// Finalize with wrong evidence ID
	_, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: uuid.New(), // wrong!
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(), ActorName: "user",
	})
	if err == nil {
		t.Fatal("expected error with wrong evidence ID")
	}
}

func TestIntegration_FinalizeFromDraft_InvalidActorID(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-FIN-004")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/actor.png"
	storage.objects[storageKey] = pngData

	original, _ := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-ACTOR-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("d", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})

	draft, _ := repo.CreateDraft(ctx, original.ID, caseID, "Draft", PurposeInternalReview, uuid.New().String())
	areas := draftState{Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "test"}}}
	areasJSON, _ := json.Marshal(areas)
	_, _ = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)

	// Finalize with invalid actor ID
	_, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    "not-a-uuid",
		ActorName:  "user",
	})
	if err == nil {
		t.Fatal("expected error with invalid actor ID")
	}
	var ve *ValidationError
	if !strings.Contains(err.Error(), "invalid actor ID") {
		t.Errorf("expected 'invalid actor ID' error, got: %v", err)
	}
	_ = ve
}

func TestIntegration_FinalizeFromDraft_EmptyAreas(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-FIN-005")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/empty.png"
	storage.objects[storageKey] = pngData

	original, _ := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-EMPTY-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("e", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})

	// Draft with no areas saved (yjs_state is null)
	draft, _ := repo.CreateDraft(ctx, original.ID, caseID, "Empty", PurposeInternalReview, uuid.New().String())

	_, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID, DraftID: draft.ID,
		ActorID: uuid.New().String(), ActorName: "user",
	})
	if err == nil {
		t.Fatal("expected error when finalizing draft with no areas")
	}
}

func TestIntegration_FinalizeFromDraft_ActorNameFallback(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-FIN-006")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/fallback.png"
	storage.objects[storageKey] = pngData

	actorID := uuid.New()
	original, _ := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-FALLBACK-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("f", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: actorID.String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})

	draft, _ := repo.CreateDraft(ctx, original.ID, caseID, "Fallback", PurposeInternalReview, actorID.String())
	areas := draftState{Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "test"}}}
	areasJSON, _ := json.Marshal(areas)
	_, _ = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)

	// Finalize with empty ActorName — should fallback to ActorID
	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID, DraftID: draft.ID,
		ActorID: actorID.String(), ActorName: "", // empty!
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft: %v", err)
	}

	newEvidence, _ := repo.FindByID(ctx, result.NewEvidenceID)
	if newEvidence.UploadedByName != actorID.String() {
		t.Errorf("uploaded_by_name should fallback to actor ID, got %s", newEvidence.UploadedByName)
	}
}
