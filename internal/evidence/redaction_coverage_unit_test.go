package evidence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
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
