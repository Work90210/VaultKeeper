package evidence

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
)

func TestValidateRedactionAreas_Empty(t *testing.T) {
	err := validateRedactionAreas(nil)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestValidateRedactionAreas_Valid(t *testing.T) {
	err := validateRedactionAreas([]RedactionArea{
		{PageNumber: 0, X: 10, Y: 20, Width: 30, Height: 40, Reason: "privacy"},
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRedactionAreas_InvalidCoords(t *testing.T) {
	tests := []struct {
		name string
		area RedactionArea
	}{
		{"negative X", RedactionArea{X: -1, Y: 10, Width: 10, Height: 10, Reason: "r"}},
		{"X > 100", RedactionArea{X: 101, Y: 10, Width: 10, Height: 10, Reason: "r"}},
		{"negative Y", RedactionArea{X: 10, Y: -1, Width: 10, Height: 10, Reason: "r"}},
		{"Y > 100", RedactionArea{X: 10, Y: 101, Width: 10, Height: 10, Reason: "r"}},
		{"zero width", RedactionArea{X: 10, Y: 10, Width: 0, Height: 10, Reason: "r"}},
		{"width > 100", RedactionArea{X: 10, Y: 10, Width: 101, Height: 10, Reason: "r"}},
		{"zero height", RedactionArea{X: 10, Y: 10, Width: 10, Height: 0, Reason: "r"}},
		{"height > 100", RedactionArea{X: 10, Y: 10, Width: 10, Height: 101, Reason: "r"}},
		{"X+Width > 100", RedactionArea{X: 60, Y: 10, Width: 50, Height: 10, Reason: "r"}},
		{"Y+Height > 100", RedactionArea{X: 10, Y: 60, Width: 10, Height: 50, Reason: "r"}},
		{"empty reason", RedactionArea{X: 10, Y: 10, Width: 10, Height: 10, Reason: ""}},
		{"whitespace reason", RedactionArea{X: 10, Y: 10, Width: 10, Height: 10, Reason: "  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRedactionAreas([]RedactionArea{tt.area})
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("expected ValidationError for %s, got %v", tt.name, err)
			}
		})
	}
}

func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with white
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestRedactImage_JPEG(t *testing.T) {
	data := createTestJPEG(100, 100)
	areas := []RedactionArea{
		{X: 10, Y: 10, Width: 30, Height: 30, Reason: "test"},
	}

	result, err := redactImage(data, "image/jpeg", areas)
	if err != nil {
		t.Fatalf("redactImage: %v", err)
	}

	// Verify it's a valid JPEG
	_, err = jpeg.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatalf("result is not valid JPEG: %v", err)
	}

	// Verify redacted area has black pixels
	img, _ := jpeg.Decode(bytes.NewReader(result))
	// Center of redaction area (25, 25)
	r, g, b, _ := img.At(25, 25).RGBA()
	// JPEG compression may not produce exactly 0, but should be very dark
	if r > 1000 || g > 1000 || b > 1000 {
		t.Errorf("redacted area pixel not black enough: r=%d g=%d b=%d", r, g, b)
	}
}

func TestRedactImage_PNG(t *testing.T) {
	data := createTestPNG(100, 100)
	areas := []RedactionArea{
		{X: 10, Y: 10, Width: 30, Height: 30, Reason: "test"},
	}

	result, err := redactImage(data, "image/png", areas)
	if err != nil {
		t.Fatalf("redactImage: %v", err)
	}

	// Verify it's a valid PNG
	img, err := png.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatalf("result is not valid PNG: %v", err)
	}

	// Verify redacted pixel is exactly black
	r, g, b, a := img.At(25, 25).RGBA()
	if r != 0 || g != 0 || b != 0 || a != 0xFFFF {
		t.Errorf("redacted pixel not black: r=%d g=%d b=%d a=%d", r, g, b, a)
	}

	// Verify non-redacted pixel is still white
	r2, g2, b2, _ := img.At(1, 1).RGBA()
	if r2 != 0xFFFF || g2 != 0xFFFF || b2 != 0xFFFF {
		t.Errorf("non-redacted pixel changed: r=%d g=%d b=%d", r2, g2, b2)
	}
}

func TestRedactImage_MultipleAreas(t *testing.T) {
	data := createTestPNG(200, 200)
	areas := []RedactionArea{
		{X: 0, Y: 0, Width: 25, Height: 25, Reason: "area1"},
		{X: 50, Y: 50, Width: 25, Height: 25, Reason: "area2"},
	}

	result, err := redactImage(data, "image/png", areas)
	if err != nil {
		t.Fatalf("redactImage: %v", err)
	}

	img, _ := png.Decode(bytes.NewReader(result))
	// Both areas should be black
	r1, g1, b1, _ := img.At(10, 10).RGBA()
	r2, g2, b2, _ := img.At(125, 125).RGBA()
	if r1 != 0 || g1 != 0 || b1 != 0 {
		t.Error("area1 not black")
	}
	if r2 != 0 || g2 != 0 || b2 != 0 {
		t.Error("area2 not black")
	}
}

func TestRedactPDF_InvalidData_ReturnsError(t *testing.T) {
	_, err := redactPDF([]byte("not a pdf"), []RedactionArea{
		{PageNumber: 1, X: 10, Y: 10, Width: 20, Height: 20, Reason: "pii"},
	})
	if err == nil {
		t.Fatal("expected error for invalid PDF data, got nil")
	}
}

func TestRedactImage_InvalidData(t *testing.T) {
	_, err := redactImage([]byte("not an image"), "image/jpeg", []RedactionArea{
		{X: 10, Y: 10, Width: 10, Height: 10, Reason: "r"},
	})
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
}

// ---------------------------------------------------------------------------
// redactPDF – minimal real PDF tests
// ---------------------------------------------------------------------------

// minimalPDF is a structurally valid PDF that pdfcpu can parse and annotate.
const minimalPDF = "%PDF-1.4\n" +
	"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n" +
	"3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R>>endobj\n" +
	"xref\n" +
	"0 4\n" +
	"0000000000 65535 f \n" +
	"0000000009 00000 n \n" +
	"0000000058 00000 n \n" +
	"0000000115 00000 n \n" +
	"trailer<</Size 4/Root 1 0 R>>\n" +
	"startxref\n" +
	"190\n" +
	"%%EOF"

func TestRedactPDF_ValidPDF_ReturnsBytes(t *testing.T) {
	areas := []RedactionArea{
		{PageNumber: 1, X: 10, Y: 10, Width: 20, Height: 20, Reason: "sensitive"},
	}

	result, err := redactPDF([]byte(minimalPDF), areas)
	if err != nil {
		t.Fatalf("redactPDF returned unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("redactPDF returned empty result")
	}
	// Result must start with the PDF header.
	if !bytes.HasPrefix(result, []byte("%PDF-")) {
		t.Errorf("result does not look like a PDF (first 10 bytes: %q)", result[:min(10, len(result))])
	}
}

func TestRedactPDF_PageBeyondCount_Skipped(t *testing.T) {
	// PageNumber 99 does not exist in the single-page PDF; the area should
	// be silently skipped and the call should still succeed.
	areas := []RedactionArea{
		{PageNumber: 99, X: 10, Y: 10, Width: 20, Height: 20, Reason: "skip me"},
	}

	result, err := redactPDF([]byte(minimalPDF), areas)
	if err != nil {
		t.Fatalf("redactPDF with out-of-range page returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("redactPDF returned empty result")
	}
}

func TestRedactPDF_PageZero_DefaultsToPageOne(t *testing.T) {
	// PageNumber 0 should fall back to page 1 (the only page).
	areas := []RedactionArea{
		{PageNumber: 0, X: 5, Y: 5, Width: 10, Height: 10, Reason: "zero page"},
	}

	result, err := redactPDF([]byte(minimalPDF), areas)
	if err != nil {
		t.Fatalf("redactPDF with page 0 returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("redactPDF returned empty result")
	}
}

func TestRedactPDF_MultipleAreas(t *testing.T) {
	areas := []RedactionArea{
		{PageNumber: 1, X: 0, Y: 0, Width: 25, Height: 25, Reason: "top-left"},
		{PageNumber: 1, X: 50, Y: 50, Width: 25, Height: 25, Reason: "center"},
	}

	result, err := redactPDF([]byte(minimalPDF), areas)
	if err != nil {
		t.Fatalf("redactPDF with multiple areas returned error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("redactPDF returned empty result")
	}
}

// min is a local helper to stay compatible with Go < 1.21 in case of older CI.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// validateRedactionAreas – additional edge-case table
// ---------------------------------------------------------------------------

func TestValidateRedactionAreas_MultipleAreas_FirstInvalid(t *testing.T) {
	areas := []RedactionArea{
		{X: -1, Y: 10, Width: 10, Height: 10, Reason: "r"}, // invalid
		{X: 10, Y: 10, Width: 10, Height: 10, Reason: "r"}, // valid
	}
	err := validateRedactionAreas(areas)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for first invalid area, got %v", err)
	}
}

func TestValidateRedactionAreas_SecondAreaInvalid(t *testing.T) {
	areas := []RedactionArea{
		{X: 10, Y: 10, Width: 10, Height: 10, Reason: "ok"},
		{X: 10, Y: 10, Width: 10, Height: 10, Reason: ""}, // missing reason
	}
	err := validateRedactionAreas(areas)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for second invalid area, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ApplyRedactions / PreviewRedactions – mock-based unit tests
// ---------------------------------------------------------------------------

// mockEvidenceGetter implements the minimal interface used by RedactionService
// to retrieve evidence. We build a real *Service backed by mocks so that
// rs.evidenceSvc.Get() resolves correctly.

func newTestRedactionService(t *testing.T) (*RedactionService, *mockRepo, *mockStorage) {
	t.Helper()

	repo := newMockRepo()
	storage := newMockStorage()
	custody := &mockCustody{}

	svc, _, _, _ := newTestService(t)
	// Swap the injected repo/storage so we control items & objects.
	svc.repo = repo
	svc.storage = storage

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, custody, logger)
	return rs, repo, storage
}

func storeEvidenceItem(repo *mockRepo, storage *mockStorage, mimeType string, content []byte) EvidenceItem {
	key := "cases/test/" + uuid.New().String()
	storage.objects[key] = content
	id := uuid.New()
	caseID := uuid.New()
	evidNum := "EV-001"
	item := EvidenceItem{
		ID:             id,
		CaseID:         caseID,
		EvidenceNumber: &evidNum,
		Filename:       "test-file",
		OriginalName:   "test-file",
		StorageKey:     &key,
		MimeType:       mimeType,
		SizeBytes:      int64(len(content)),
		SHA256Hash:     "abc123",
		Classification: ClassificationRestricted,
		Tags:           []string{},
		IsCurrent:      true,
		Version:        1,
		TSAStatus:      TSAStatusDisabled,
	}
	repo.items[id] = item
	return item
}

func TestApplyRedactions_UnsupportedMIMEType(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "application/zip", []byte("zip content"))

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for unsupported MIME type, got %v", err)
	}
}

func TestApplyRedactions_EmptyAreas_ValidationError(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(100, 100))

	_, err := rs.ApplyRedactions(context.Background(), item.ID, nil, uuid.New().String())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty areas, got %v", err)
	}
}

func TestApplyRedactions_EvidenceNotFound(t *testing.T) {
	rs, _, _ := newTestRedactionService(t)

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, err := rs.ApplyRedactions(context.Background(), uuid.New(), areas, uuid.New().String())
	if err == nil {
		t.Fatal("expected error for non-existent evidence, got nil")
	}
}

func TestApplyRedactions_DestroyedEvidence(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(100, 100))

	// Mark as destroyed
	now := time.Now()
	item.DestroyedAt = &now
	repo.items[item.ID] = item

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for destroyed evidence, got %v", err)
	}
}

func TestApplyRedactions_JPEG_Success(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/jpeg", createTestJPEG(100, 100))

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "pii"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions: %v", err)
	}
	if result.NewEvidenceID == uuid.Nil {
		t.Error("expected non-nil new evidence ID")
	}
	if result.OriginalID != item.ID {
		t.Errorf("OriginalID = %s, want %s", result.OriginalID, item.ID)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
	if result.NewHash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestApplyRedactions_PNG_Success(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(100, 100))

	areas := []RedactionArea{
		{PageNumber: 0, X: 0, Y: 0, Width: 50, Height: 50, Reason: "classified"},
		{PageNumber: 0, X: 50, Y: 50, Width: 40, Height: 40, Reason: "sensitive"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions: %v", err)
	}
	if result.RedactionCount != 2 {
		t.Errorf("RedactionCount = %d, want 2", result.RedactionCount)
	}
}

func TestApplyRedactions_StorageGetError(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/jpeg", createTestJPEG(50, 50))

	// Force storage.GetObject to fail.
	storage.getErr = errors.New("storage unreachable")

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when storage get fails")
	}
}

func TestPreviewRedactions_UnsupportedMIMEType(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "text/plain", []byte("plain text"))

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, _, err := rs.PreviewRedactions(context.Background(), item.ID, areas)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for unsupported MIME type, got %v", err)
	}
}

func TestPreviewRedactions_EmptyAreas(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(100, 100))

	_, _, err := rs.PreviewRedactions(context.Background(), item.ID, nil)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty areas, got %v", err)
	}
}

func TestPreviewRedactions_PNG_Success(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(100, 100))

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 30, Height: 30, Reason: "preview test"},
	}
	reader, mimeType, err := rs.PreviewRedactions(context.Background(), item.ID, areas)
	if err != nil {
		t.Fatalf("PreviewRedactions: %v", err)
	}
	defer reader.Close()

	if mimeType != "image/png" {
		t.Errorf("mimeType = %q, want image/png", mimeType)
	}

	data, _ := io.ReadAll(reader)
	if len(data) == 0 {
		t.Error("expected non-empty preview data")
	}
	// Ensure the result is still a valid PNG.
	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("preview result is not valid PNG: %v", err)
	}
}

func TestPreviewRedactions_EvidenceNotFound(t *testing.T) {
	rs, _, _ := newTestRedactionService(t)

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 20, Height: 20, Reason: "test"},
	}
	_, _, err := rs.PreviewRedactions(context.Background(), uuid.New(), areas)
	if err == nil {
		t.Fatal("expected error for non-existent evidence")
	}
}

func TestPreviewRedactions_StorageGetError(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/jpeg", createTestJPEG(50, 50))
	storage.getErr = errors.New("storage failure")

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "r"},
	}
	_, _, err := rs.PreviewRedactions(context.Background(), item.ID, areas)
	if err == nil {
		t.Fatal("expected error when storage get fails")
	}
}

// ---------------------------------------------------------------------------
// ApplyRedactions – additional paths (size limit, TSA, storage put, repo create)
// ---------------------------------------------------------------------------

// mockTSA allows controlling IssueTimestamp behaviour in tests.
type mockTSA struct {
	token []byte
	name  string
	err   error
}

func (m *mockTSA) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	return m.token, m.name, time.Time{}, m.err
}

func (m *mockTSA) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error { return nil }

func newRedactionServiceWithTSA(t *testing.T, tsa integrity.TimestampAuthority) (*RedactionService, *mockRepo, *mockStorage) {
	t.Helper()
	repo := newMockRepo()
	storage := newMockStorage()
	custody := &mockCustody{}
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rs := NewRedactionService(svc, storage, tsa, custody, logger)
	return rs, repo, storage
}

func TestApplyRedactions_FileTooLarge(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	key := "cases/test/" + uuid.New().String()
	storage.objects[key] = []byte("data")
	id := uuid.New()
	evidNum := "EV-BIG"
	item := EvidenceItem{
		ID:             id,
		CaseID:         uuid.New(),
		EvidenceNumber: &evidNum,
		Filename:       "big.pdf",
		OriginalName:   "big.pdf",
		StorageKey:     &key,
		MimeType:       "application/pdf",
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
		{PageNumber: 1, X: 10, Y: 10, Width: 10, Height: 10, Reason: "size check"},
	}
	_, err := rs.ApplyRedactions(context.Background(), id, areas, uuid.New().String())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for oversized file, got %v", err)
	}
}

func TestApplyRedactions_TSAError_StillSucceeds(t *testing.T) {
	// When TSA.IssueTimestamp returns an error the operation should succeed
	// and TSA status should be left as pending (warning is logged only).
	rs, repo, storage := newRedactionServiceWithTSA(t, &mockTSA{err: errors.New("tsa down")})
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "tsa-error-test"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions should succeed despite TSA error, got: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("RedactionCount = %d, want 1", result.RedactionCount)
	}
}

func TestApplyRedactions_TSASuccess_TokenStored(t *testing.T) {
	// When TSA.IssueTimestamp returns a non-nil token, TSA status should be stamped.
	fakeToken := []byte("fake-tsa-token")
	rs, repo, storage := newRedactionServiceWithTSA(t, &mockTSA{token: fakeToken, name: "test-tsa"})
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))

	areas := []RedactionArea{
		{PageNumber: 0, X: 5, Y: 5, Width: 10, Height: 10, Reason: "tsa-stamp-test"},
	}
	result, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions: %v", err)
	}
	// The new evidence item in the repo should have TSA status "stamped".
	newItem, ok := repo.items[result.NewEvidenceID]
	if !ok {
		t.Fatal("new evidence item not found in repo")
	}
	if newItem.TSAStatus != TSAStatusStamped {
		t.Errorf("TSAStatus = %q, want %q", newItem.TSAStatus, TSAStatusStamped)
	}
}

func TestApplyRedactions_StoragePutError(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))
	// Allow GetObject but fail PutObject.
	storage.putErr = errors.New("storage write failure")

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "put-fail"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when PutObject fails")
	}
}

func TestApplyRedactions_RepoCreateError(t *testing.T) {
	rs, repo, storage := newTestRedactionService(t)
	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))
	repo.createFn = func(_ context.Context, _ CreateEvidenceInput) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("db write failure")
	}

	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "repo-fail"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when repo.Create fails")
	}
}

func TestApplyRedactions_RecordCustody_NilCustody(t *testing.T) {
	// Verify that nil custody does not panic during custody recording.
	repo := newMockRepo()
	storage := newMockStorage()
	svc, _, _, _ := newTestService(t)
	svc.repo = repo
	svc.storage = storage
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Pass nil custody.
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, nil, logger)

	item := storeEvidenceItem(repo, storage, "image/png", createTestPNG(50, 50))
	areas := []RedactionArea{
		{PageNumber: 0, X: 10, Y: 10, Width: 10, Height: 10, Reason: "nil-custody"},
	}
	_, err := rs.ApplyRedactions(context.Background(), item.ID, areas, uuid.New().String())
	if err != nil {
		t.Fatalf("ApplyRedactions with nil custody should not fail: %v", err)
	}
}
