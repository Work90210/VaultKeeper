package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// ---------------------------------------------------------------------------
// PreviewRedactions — file too large
// ---------------------------------------------------------------------------

func TestPreviewRedactions_FileTooLarge(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	key := "cases/test/" + uuid.New().String()
	storage.objects[key] = []byte("data")
	id := uuid.New()
	caseID := uuid.New()
	evidNum := "EV-BIG-PREVIEW"
	item := EvidenceItem{
		ID:             id,
		CaseID:         caseID,
		EvidenceNumber: &evidNum,
		Filename:       "big.png",
		OriginalName:   "big.png",
		StorageKey:     &key,
		MimeType:       "image/png",
		SizeBytes:      maxRedactionSize + 1, // exceeds limit
		SHA256Hash:     "abc",
		Classification: ClassificationRestricted,
		Tags:           []string{},
		IsCurrent:      true,
		Version:        1,
		TSAStatus:      TSAStatusDisabled,
	}
	repo.items[id] = item

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "size check"},
	}
	_, _, err := rs.PreviewRedactions(context.Background(), id, areas)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for oversized file in preview, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// recordCustody — custody == nil (direct unit test of the method)
// ---------------------------------------------------------------------------

func TestRecordCustody_NilCustody(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Pass nil custody explicitly so the nil guard in recordCustody is hit.
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, nil, logger)

	// Call recordCustody directly — must not panic when custody is nil.
	rs.recordCustody(context.Background(), uuid.New(), uuid.New(), "test_action", "actor", map[string]string{"k": "v"})
}

// recordCustody — custody returns an error (logger.Error path)
func TestRecordCustody_CustodyError(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	errCustody := &errorCustody{err: errors.New("custody backend unavailable")}
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, errCustody, logger)

	// Should not panic; error is logged, not returned.
	rs.recordCustody(context.Background(), uuid.New(), uuid.New(), "test_action", "actor", nil)
}

// errorCustody is a CustodyRecorder that always returns an error.
type errorCustody struct {
	err error
}

func (c *errorCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return c.err
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — non-PGRepository type assertion fails
// ---------------------------------------------------------------------------

// nonPGRepo satisfies the Repository interface but is NOT *PGRepository, so
// FinalizeFromDraft's type assertion will fail.
type nonPGRepo struct {
	*mockRepo
}

func TestFinalizeFromDraft_NonPGRepository(t *testing.T) {
	repo := newMockRepo()
	storage := &inMemStorage{objects: make(map[string][]byte)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc, _, _, _ := newTestService(t)
	// Inject a non-*PGRepository implementation.
	svc.repo = &nonPGRepo{mockRepo: repo}
	svc.storage = storage

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, &noopCustody{}, logger)

	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: uuid.New(),
		DraftID:    uuid.New(),
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error when repo is not *PGRepository")
	}
	if err.Error() != "finalize requires PGRepository" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require postgres container
// ---------------------------------------------------------------------------

// newFinalizeIntegrationSvc constructs a Service wired to a real postgres pool.
// Mirrors the pattern in finalize_integration_test.go.
func newFinalizeIntegrationSvc(
	t *testing.T,
	repo *PGRepository,
	storage ObjectStorage,
	tsa integrity.TimestampAuthority,
	custody CustodyRecorder,
	caseLookup CaseLookup,
	logger *slog.Logger,
) *Service {
	t.Helper()
	thumbGen := NewThumbnailGenerator()
	return NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
}

// hashStr64 returns a 64-char hex string by repeating a single char.
func hashStr64(ch string) string {
	return strings.Repeat(ch, 64)
}

// encodeDraftAreas marshals a draftState to JSON for use in UpdateDraft.
func encodeDraftAreas(state draftState) []byte {
	b, _ := json.Marshal(state)
	return b
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — unsupported MIME type (integration, real postgres)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_UnsupportedMIME(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-MIME-001")
	content := []byte("plain text content")
	storageKey := "evidence/mime/file.txt"
	storage.objects[storageKey] = content

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-MIME-001",
		Filename:       "file.txt",
		OriginalName:   "file.txt",
		StorageKey:     storageKey,
		MimeType:       "text/plain", // unsupported for redaction
		SizeBytes:      int64(len(content)),
		SHA256Hash:     hashStr64("a"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "MIME Test", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error for unsupported MIME type")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for unsupported MIME, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — classification override
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_ClassificationOverride(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-CLASS-001")
	pngData := createSmallPNG(100, 100)
	storageKey := "evidence/class/file.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-CLASS-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("c"),
		Classification: ClassificationRestricted, // original classification
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Class Override", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "class test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	actorID := uuid.New()
	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID:     original.ID,
		DraftID:        draft.ID,
		ActorID:        actorID.String(),
		ActorName:      "tester",
		Classification: ClassificationConfidential, // override to higher classification
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft with classification override: %v", err)
	}

	newEvidence, err := repo.FindByID(ctx, result.NewEvidenceID)
	if err != nil {
		t.Fatalf("find new evidence: %v", err)
	}
	if newEvidence.Classification != ClassificationConfidential {
		t.Errorf("classification = %q, want %q", newEvidence.Classification, ClassificationConfidential)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — storage cleanup triggered on invalid actor ID
//
// This exercises the storageCleanup() closure path: PutObject succeeds, then
// uuid.Parse(input.ActorID) fails, causing storageCleanup() to be called
// which deletes the already-stored redacted file before returning the error.
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_StorageCleanupOnActorParseError(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	trackingStorage := &trackableStorage{inMemStorage: &inMemStorage{objects: make(map[string][]byte)}}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, trackingStorage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, trackingStorage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-CLEANUP-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/cleanup/file.png"
	trackingStorage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-CLEANUP-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("d"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Cleanup", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "cleanup"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	putCountBefore := trackingStorage.putCount

	// Intentionally bad actor ID → triggers storageCleanup after PutObject succeeds.
	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    "not-a-valid-uuid",
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error with invalid actor ID")
	}

	// Verify that PutObject was called (file was written) then cleaned up.
	if trackingStorage.putCount == putCountBefore {
		t.Error("expected PutObject to be called before cleanup")
	}
	if trackingStorage.deleteCount == 0 {
		t.Error("expected DeleteObject to be called as part of storage cleanup")
	}
}

// trackableStorage wraps inMemStorage and counts PutObject and DeleteObject calls.
type trackableStorage struct {
	*inMemStorage
	putCount    int
	deleteCount int
}

func (s *trackableStorage) PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, mime string) error {
	s.putCount++
	return s.inMemStorage.PutObject(ctx, key, reader, size, mime)
}

func (s *trackableStorage) DeleteObject(ctx context.Context, key string) error {
	s.deleteCount++
	return s.inMemStorage.DeleteObject(ctx, key)
}

func (s *trackableStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	return s.inMemStorage.GetObject(ctx, key)
}

func (s *trackableStorage) StatObject(ctx context.Context, key string) (int64, error) {
	return s.inMemStorage.StatObject(ctx, key)
}

// ---------------------------------------------------------------------------
// ApplyRedactions — PDF path (covers application/pdf branch)
// ---------------------------------------------------------------------------

func TestApplyRedactions_PDF_Success(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	pdfData := []byte(minimalPDF)
	item := storeEvidenceItem(repo, storage, "application/pdf", pdfData)

	areas := []RedactionArea{
		{PageNumber: 1, X: 10, Y: 10, Width: 20, Height: 20, Reason: "redact pdf"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions PDF: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
}

// ---------------------------------------------------------------------------
// PreviewRedactions — PDF path
// ---------------------------------------------------------------------------

func TestPreviewRedactions_PDF_Success(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	pdfData := []byte(minimalPDF)
	item := storeEvidenceItem(repo, storage, "application/pdf", pdfData)

	areas := []RedactionArea{
		{PageNumber: 1, X: 5, Y: 5, Width: 15, Height: 15, Reason: "preview pdf"},
	}
	reader, mimeType, err := rs.PreviewRedactions(context.Background(), item.ID, areas)
	if err != nil {
		t.Fatalf("PreviewRedactions PDF: %v", err)
	}
	defer reader.Close()
	if mimeType != "application/pdf" {
		t.Errorf("mimeType = %q, want application/pdf", mimeType)
	}
	data, _ := io.ReadAll(reader)
	if len(data) == 0 {
		t.Error("expected non-empty PDF preview data")
	}
}

// ---------------------------------------------------------------------------
// ApplyRedactions — UpdateVersionFields/MarkNonCurrent error paths
// (lines 176-181: errors are logged only, not returned)
// ---------------------------------------------------------------------------

func TestApplyRedactions_UpdateVersionFieldsError_StillSucceeds(t *testing.T) {
	// Inject a repo whose UpdateVersionFields returns an error to cover the
	// logger.Error path on line 176-178. The overall operation should still succeed.
	repo := newMockRepo()
	storage := newMockStorage()
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, &mockCustody{}, logger)

	// Override UpdateVersionFields to return an error.
	repo.updateVersionFieldsFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ int) error {
		return errors.New("update version fields error")
	}

	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))
	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "version-field-error-test"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions should succeed despite UpdateVersionFields error: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
}

func TestApplyRedactions_MarkNonCurrentError_StillSucceeds(t *testing.T) {
	// Inject a repo whose MarkNonCurrent returns an error to cover the
	// logger.Error path on lines 179-181. Operation should still succeed.
	repo := newMockRepo()
	storage := newMockStorage()
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, &mockCustody{}, logger)

	// Override MarkNonCurrent to return an error.
	repo.markNonCurrentFn = func(_ context.Context, _ uuid.UUID) error {
		return errors.New("mark non-current error")
	}

	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))
	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "non-current-error-test"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions should succeed despite MarkNonCurrent error: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — destroyed evidence (integration)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_DestroyedEvidence(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-DEST-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/destroyed/file.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-DEST-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("e"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Destroyed Test", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "destroy test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	// Mark the evidence as destroyed before finalizing.
	if err := repo.MarkDestroyed(ctx, original.ID, "test destruction", "tester"); err != nil {
		t.Fatalf("mark destroyed: %v", err)
	}

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error when finalizing destroyed evidence")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for destroyed evidence, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — file too large (integration)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_FileTooLarge(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-SIZE-001")
	storageKey := "evidence/size/file.png"
	storage.objects[storageKey] = []byte("tiny") // actual content is tiny

	// Create evidence with SizeBytes exceeding the limit so the size check fires.
	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-SIZE-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      maxRedactionSize + 1,
		SHA256Hash:     hashStr64("f"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Size Test", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "size test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error when file is too large")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for file too large, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — invalid JSON in yjs_state (integration)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_InvalidYjsState(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-YJS-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/yjs/file.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-YJS-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("g"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "YJS Test", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Write invalid JSON directly into yjs_state — bypasses UpdateDraft validation.
	invalidJSON := []byte("{not valid json :")
	_, err = pool.Exec(ctx,
		`UPDATE redaction_drafts SET yjs_state = $1 WHERE id = $2`,
		invalidJSON, draft.ID,
	)
	if err != nil {
		t.Fatalf("inject invalid yjs_state: %v", err)
	}

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error with invalid yjs_state JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal draft state") {
		t.Errorf("expected 'unmarshal draft state' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — GetObject error (storage returns error)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_StorageGetError(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	// Use a storage that always fails GetObject.
	failingStorage := &failGetStorage{inMemStorage: &inMemStorage{objects: make(map[string][]byte)}}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, failingStorage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, failingStorage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-SGET-001")
	storageKey := "evidence/sget/file.png"
	failingStorage.objects[storageKey] = createSmallPNG(50, 50)

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-SGET-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      100,
		SHA256Hash:     hashStr64("h"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "GetErr", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "storage error"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	// Enable the get error now that the draft is ready.
	failingStorage.failGet = true

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error when storage GetObject fails")
	}
	if !strings.Contains(err.Error(), "download original file") {
		t.Errorf("expected 'download original file' error, got: %v", err)
	}
}

// failGetStorage wraps inMemStorage but can be configured to fail GetObject.
type failGetStorage struct {
	*inMemStorage
	failGet bool
}

func (s *failGetStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	if s.failGet {
		return nil, 0, "", errors.New("storage get error")
	}
	return s.inMemStorage.GetObject(ctx, key)
}

func (s *failGetStorage) PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, mime string) error {
	return s.inMemStorage.PutObject(ctx, key, reader, size, mime)
}

func (s *failGetStorage) DeleteObject(ctx context.Context, key string) error {
	return s.inMemStorage.DeleteObject(ctx, key)
}

func (s *failGetStorage) StatObject(ctx context.Context, key string) (int64, error) {
	return s.inMemStorage.StatObject(ctx, key)
}

// Ensure time import is used (for destroyed evidence test which needs time.Time).
var _ = time.Now

// ---------------------------------------------------------------------------
// FinalizeFromDraft — TSA error path (integration, uses mockTSA)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_TSAError_StillSucceeds(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	// TSA that always returns an error — finalize should still succeed (warn only).
	tsa := &mockTSA{err: errors.New("tsa down")}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-TSA-ERR-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/tsaerr/file.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-TSA-ERR-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("t"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "TSA Err", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "tsa error test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft should succeed despite TSA error: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
}

// FinalizeFromDraft — TSA succeeds with token (covers token != nil branch)
func TestIntegration_FinalizeFromDraft_TSASuccess_TokenSet(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	// TSA that returns a token.
	tsa := &mockTSA{token: []byte("tsa-token"), name: "test-tsa"}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-TSA-OK-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/tsaok/file.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-TSA-OK-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("u"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "TSA OK", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "tsa ok test"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft with TSA token: %v", err)
	}

	newEvidence, err := repo.FindByID(ctx, result.NewEvidenceID)
	if err != nil {
		t.Fatalf("find new evidence: %v", err)
	}
	if newEvidence.TSAStatus != TSAStatusStamped {
		t.Errorf("TSAStatus = %q, want %q", newEvidence.TSAStatus, TSAStatusStamped)
	}
}

// FinalizeFromDraft — PutObject fails (storage cleanup fires)
func TestIntegration_FinalizeFromDraft_StoragePutError(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	// GetObject succeeds, PutObject fails.
	failPutStorage := &failPutStore{inMemStorage: &inMemStorage{objects: make(map[string][]byte)}}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, failPutStorage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, failPutStorage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-PUT-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/put/file.png"
	failPutStorage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-PUT-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("v"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Put Err", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "put error"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	// Enable put error before finalizing.
	failPutStorage.failPut = true

	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err == nil {
		t.Fatal("expected error when PutObject fails")
	}
	if !strings.Contains(err.Error(), "store redacted file") {
		t.Errorf("expected 'store redacted file' error, got: %v", err)
	}
}

// failPutStore allows GetObject but rejects PutObject on demand.
type failPutStore struct {
	*inMemStorage
	failPut bool
}

func (s *failPutStore) PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, mime string) error {
	if s.failPut {
		return errors.New("storage write error")
	}
	return s.inMemStorage.PutObject(ctx, key, reader, size, mime)
}

func (s *failPutStore) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	return s.inMemStorage.GetObject(ctx, key)
}

func (s *failPutStore) DeleteObject(ctx context.Context, key string) error {
	return s.inMemStorage.DeleteObject(ctx, key)
}

func (s *failPutStore) StatObject(ctx context.Context, key string) (int64, error) {
	return s.inMemStorage.StatObject(ctx, key)
}

// FinalizeFromDraft — storageCleanup DeleteObject fails (warns only, line 543-545)
func TestIntegration_FinalizeFromDraft_StorageCleanupDeleteFails(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	// PutObject succeeds, but DeleteObject (called via storageCleanup) fails.
	// This happens when actor ID is invalid: put succeeds, then actor parse fails,
	// then cleanup tries DeleteObject which also fails.
	failDelStorage := &failDelStore{inMemStorage: &inMemStorage{objects: make(map[string][]byte)}}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, failDelStorage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, failDelStorage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-DEL-001")
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/del/file.png"
	failDelStorage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-DEL-001",
		Filename:       "file.png",
		OriginalName:   "file.png",
		StorageKey:     storageKey,
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		SHA256Hash:     hashStr64("w"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Del Err", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 0, X: 5, Y: 5, W: 10, H: 10, Reason: "delete error"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	// Enable delete failure — cleanup will warn but not return an error to caller.
	failDelStorage.failDel = true

	// Bad actor ID triggers storageCleanup → DeleteObject fails → logger.Warn
	_, err = rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    "not-a-valid-uuid",
		ActorName:  "tester",
	})
	// The operation itself must fail (invalid actor ID), but must not panic on DeleteObject error.
	if err == nil {
		t.Fatal("expected error with invalid actor ID")
	}
}

// ---------------------------------------------------------------------------
// FinalizeFromDraft — PDF evidence (covers the application/pdf branch)
// ---------------------------------------------------------------------------

func TestIntegration_FinalizeFromDraft_PDFEvidence(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)

	svc := newFinalizeIntegrationSvc(t, repo, storage, tsa, custody, caseLookup, logger)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseID := seedCase(t, pool, "CR-PDF-FIN-001")
	pdfData := []byte(minimalPDF)
	storageKey := "evidence/pdffin/file.pdf"
	storage.objects[storageKey] = pdfData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-PDF-FIN-001",
		Filename:       "file.pdf",
		OriginalName:   "file.pdf",
		StorageKey:     storageKey,
		MimeType:       "application/pdf",
		SizeBytes:      int64(len(pdfData)),
		SHA256Hash:     hashStr64("p"),
		Classification: ClassificationRestricted,
		Tags:           []string{},
		UploadedBy:     uuid.New().String(),
		UploadedByName: "tester",
		TSAStatus:      TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "PDF Draft", PurposeCourtSubmission, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	areasJSON := encodeDraftAreas(draftState{
		Areas: []draftArea{{ID: "1", Page: 1, X: 10, Y: 10, W: 20, H: 20, Reason: "pdf redaction"}},
	})
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	result, err := rs.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID: original.ID,
		DraftID:    draft.ID,
		ActorID:    uuid.New().String(),
		ActorName:  "tester",
	})
	if err != nil {
		t.Fatalf("FinalizeFromDraft for PDF: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}

	newEvidence, err := repo.FindByID(ctx, result.NewEvidenceID)
	if err != nil {
		t.Fatalf("find new evidence: %v", err)
	}
	if newEvidence.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want application/pdf", newEvidence.MimeType)
	}
}

// failDelStore allows PutObject but rejects DeleteObject on demand.
type failDelStore struct {
	*inMemStorage
	failDel bool
}

func (s *failDelStore) PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, mime string) error {
	return s.inMemStorage.PutObject(ctx, key, reader, size, mime)
}

func (s *failDelStore) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	return s.inMemStorage.GetObject(ctx, key)
}

func (s *failDelStore) DeleteObject(ctx context.Context, key string) error {
	if s.failDel {
		return errors.New("delete error")
	}
	return s.inMemStorage.DeleteObject(ctx, key)
}

func (s *failDelStore) StatObject(ctx context.Context, key string) (int64, error) {
	return s.inMemStorage.StatObject(ctx, key)
}
