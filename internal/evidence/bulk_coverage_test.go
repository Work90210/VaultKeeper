package evidence

// bulk_coverage_test.go targets every uncovered branch in the Sprint 10
// evidence surface (bulk.go, bulk_repository.go, bulk_service.go,
// bulk_handler.go, migrate_adapter.go).

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// --- bulk.go error branches ---

func TestExtractBulkZIP_NotAValidArchive(t *testing.T) {
	// Garbage bytes that aren't a zip.
	data := []byte("not a zip file at all")
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Errorf("want ErrZipRejected, got %v", err)
	}
}

func TestExtractBulkZIP_OnlyMetadataNoIngestableFiles(t *testing.T) {
	// Archive contains only _metadata.csv — after extraction, out.Files
	// is empty and the "no ingestable files" branch fires.
	data := bulkTestZip(t, map[string]string{
		"_metadata.csv": "filename,title\na.txt,Alpha\n",
	})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !strings.Contains(err.Error(), "no ingestable files") {
		t.Errorf("want no-ingestable-files error, got %v", err)
	}
}

func TestExtractBulkZIP_DirectoryEntriesSkipped(t *testing.T) {
	// Archive with a directory entry followed by a file; the directory
	// should be skipped (the IsDir() branch) and only the file processed.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Create a directory entry explicitly.
	dh := &zip.FileHeader{Name: "subdir/", Method: zip.Store}
	dh.SetMode(0o755 | fs.ModeDir)
	if _, err := w.CreateHeader(dh); err != nil {
		t.Fatal(err)
	}
	// Regular file.
	f, _ := w.Create("subdir/file.txt")
	_, _ = f.Write([]byte("hello"))
	_ = w.Close()
	data := buf.Bytes()

	bulk, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err != nil {
		t.Fatalf("ExtractBulkZIP: %v", err)
	}
	if len(bulk.Files) != 1 {
		t.Errorf("Files = %d, want 1", len(bulk.Files))
	}
}

func TestExtractBulkZIP_SymlinkEntry(t *testing.T) {
	// Construct a zip entry with ModeSymlink set explicitly. ExtractBulkZIP
	// must reject it.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link.txt", Method: zip.Store}
	h.SetMode(0o777 | fs.ModeSymlink)
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("/etc/passwd")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	data := buf.Bytes()

	_, err = ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !strings.Contains(err.Error(), "symlinks not permitted") {
		t.Errorf("want symlink rejection, got %v", err)
	}
}

func TestExtractBulkZIP_CumulativeSizeExceeded(t *testing.T) {
	// Two files, each within per-file limit, but cumulative exceeds
	// maxUpload * BulkMaxUncompressedRatio.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Each file 600 bytes; ratio 10; per-file limit 1000; cumulative 1200 > 1000*10=10000 NO.
	// Need the ratio to kick in before per-file does.
	// maxUpload=100, ratio=10, cumulative cap=1000. Two files of 600 each:
	// first: 600 <= 100*10=1000? Yes... wait per-file cap is maxUpload=100.
	// So 600 exceeds per-file. Won't hit cumulative.
	// Need per-file under maxUpload and cumulative over maxUpload*10.
	// With maxUpload=100, cumulative cap = 1000. 11 files of 100 bytes each = 1100 > 1000.
	for i := 0; i < 11; i++ {
		f, _ := w.Create(fmt.Sprintf("f%02d.txt", i))
		_, _ = f.Write(bytes.Repeat([]byte{'x'}, 100))
	}
	_ = w.Close()
	data := buf.Bytes()
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 100)
	if err == nil || !strings.Contains(err.Error(), "total uncompressed") {
		t.Errorf("want cumulative size error, got %v", err)
	}
}

func TestExtractBulkZIP_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before entering the loop
	data := bulkTestZip(t, map[string]string{"a.txt": "A"})
	_, err := ExtractBulkZIP(ctx, bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil {
		t.Error("want context error")
	}
}

// --- parseBulkMetadataCSV error branches ---

func TestParseBulkMetadataCSV_ErrorBranches(t *testing.T) {
	// Nil reader → "empty metadata csv" when Read returns io.EOF.
	if _, err := parseBulkMetadataCSV(strings.NewReader("")); err == nil {
		t.Error("want empty csv error")
	}
	// Missing filename column.
	if _, err := parseBulkMetadataCSV(strings.NewReader("title\nOnly\n")); err == nil {
		t.Error("want missing filename column error")
	}
	// Unterminated quoted field → csv parse error.
	body := "filename,title\nfoo.pdf,\"unterminated\n"
	if _, err := parseBulkMetadataCSV(strings.NewReader(body)); err == nil {
		t.Error("want csv parse error")
	}
	// Row with empty filename cell is skipped silently; no error.
	valid := "filename,title\n,SkippedEmpty\nfoo.pdf,Real\n"
	meta, err := parseBulkMetadataCSV(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := meta[""]; ok {
		t.Error("empty filename should be skipped")
	}
	if meta["foo.pdf"].Title != "Real" {
		t.Errorf("Real row missing: %+v", meta)
	}
	// RFC3339 source_date variant (hits the else-if branch).
	withDate := "filename,source_date\nfoo.pdf,2024-03-15T12:00:00Z\n"
	meta, err = parseBulkMetadataCSV(strings.NewReader(withDate))
	if err != nil {
		t.Fatal(err)
	}
	if meta["foo.pdf"].SourceDate == nil {
		t.Error("RFC3339 SourceDate not parsed")
	}
	// Date-only source_date variant (hits the first if branch).
	withDateOnly := "filename,source_date\nfoo.pdf,2024-03-15\n"
	meta, err = parseBulkMetadataCSV(strings.NewReader(withDateOnly))
	if err != nil {
		t.Fatal(err)
	}
	if meta["foo.pdf"].SourceDate == nil {
		t.Error("date-only SourceDate not parsed")
	}
	// Unparseable source_date is silently dropped (neither branch taken).
	withBadDate := "filename,source_date\nfoo.pdf,not-a-date\n"
	meta, err = parseBulkMetadataCSV(strings.NewReader(withBadDate))
	if err != nil {
		t.Fatal(err)
	}
	if meta["foo.pdf"].SourceDate != nil {
		t.Error("bad date should be dropped")
	}
}

// bulkFailingReader returns a fixed error on Read. Used to trigger the
// non-EOF error branch in parseBulkMetadataCSV's header Read call.
type bulkFailingReader struct{ err error }

func (r *bulkFailingReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestParseBulkMetadataCSV_HeaderReadError(t *testing.T) {
	_, err := parseBulkMetadataCSV(&bulkFailingReader{err: errors.New("io down")})
	if err == nil {
		t.Error("want non-EOF header read error")
	}
}

// corruptMetadataCSV constructs an archive where the _metadata.csv entry
// is present but has a bogus header that parseBulkMetadataCSV rejects.
func TestExtractBulkZIP_InvalidMetadataCSV(t *testing.T) {
	data := bulkTestZip(t, map[string]string{
		"a.txt":         "A",
		"_metadata.csv": "title\nOnly\n", // missing filename column
	})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Errorf("want metadata csv error, got %v", err)
	}
}

// --- sanitizeZipEntryName remaining branches ---

func TestSanitizeZipEntryName_NullByte(t *testing.T) {
	if _, err := sanitizeZipEntryName("foo\x00bar"); err == nil {
		t.Error("want null byte rejection")
	}
}

// splitBulkTags empty input already covered; exercise with trailing separators.
func TestSplitBulkTags_TrailingSeparators(t *testing.T) {
	got := splitBulkTags("a,,b,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v", got)
	}
}

// --- bulk_repository.go: NewBulkJobRepository constructor ---

func TestNewBulkJobRepository(t *testing.T) {
	// Pass nil pool — constructor should not dereference it.
	r := NewBulkJobRepository(nil)
	if r == nil {
		t.Error("NewBulkJobRepository returned nil")
	}
}

// --- bulk_service.go error branches ---

func TestBulkService_Submit_ValidationErrors(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 1024*1024)
	ctx := context.Background()

	// Missing case_id.
	if _, err := bulkSvc.Submit(ctx, BulkSubmitInput{ArchiveBytes: []byte("x")}); err == nil {
		t.Error("want case_id error")
	}
	// Missing archive.
	if _, err := bulkSvc.Submit(ctx, BulkSubmitInput{CaseID: uuid.New()}); err == nil {
		t.Error("want archive error")
	}
}

// createErrorBulkRepo returns an error from Create to exercise the
// early-failure branch in Submit.
type createErrorBulkRepo struct{ *fakeBulkRepo }

func (c *createErrorBulkRepo) Create(_ context.Context, _ uuid.UUID, _, _ string) (BulkJob, error) {
	return BulkJob{}, errors.New("db down")
}

func TestBulkService_Submit_CreateError(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &createErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	data := bulkTestZip(t, map[string]string{"a.txt": "A"})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Errorf("want create error, got %v", err)
	}
}

// progressErrorBulkRepo fails UpdateProgress so the soft-log branch fires.
type progressErrorBulkRepo struct{ *fakeBulkRepo }

func (p *progressErrorBulkRepo) UpdateProgress(_ context.Context, _ uuid.UUID, _, _, _ int, _ string) error {
	return errors.New("progress update down")
}

func TestBulkService_Submit_ProgressUpdateFailsSoftly(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &progressErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	data := bulkTestZip(t, map[string]string{"a.txt": "A"})
	// Submit should still complete the archive processing even though
	// every UpdateProgress call errors.
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err != nil {
		t.Errorf("Submit: %v", err)
	}
}

// appendErrorBulkRepo fails AppendError so the error-log branch fires.
type appendErrorBulkRepo struct{ *fakeBulkRepo }

func (a *appendErrorBulkRepo) AppendError(_ context.Context, _ uuid.UUID, _ BulkJobError) error {
	return errors.New("append error down")
}

func TestBulkService_Submit_AppendErrorFailsSoftly(t *testing.T) {
	// A corrupt classification triggers a per-file failure that would
	// normally be appended via AppendError. The append failure must be
	// logged but not abort the batch.
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &appendErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	data := bulkTestZip(t, map[string]string{
		"bad.txt":       "B",
		"_metadata.csv": "filename,classification\nbad.txt,not_real\n",
	})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err != nil {
		t.Errorf("Submit: %v", err)
	}
}

// finalizeErrorBulkRepo fails Finalize so the finalize-log branch fires.
type finalizeErrorBulkRepo struct{ *fakeBulkRepo }

func (f *finalizeErrorBulkRepo) Finalize(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("finalize down")
}

func TestBulkService_Submit_FinalizeFailsSoftly(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &finalizeErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	data := bulkTestZip(t, map[string]string{"a.txt": "A"})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err != nil {
		t.Errorf("Submit: %v", err)
	}
}

// hashErrorBulkRepo fails SetArchiveHash so that soft-log branch fires.
type hashErrorBulkRepo struct{ *fakeBulkRepo }

func (h *hashErrorBulkRepo) SetArchiveHash(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("hash persist down")
}

func TestBulkService_Submit_SetArchiveHashFailsSoftly(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &hashErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	data := bulkTestZip(t, map[string]string{"a.txt": "A"})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err != nil {
		t.Errorf("Submit: %v", err)
	}
}

func TestBulkService_Submit_ContextCanceled(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 1024*1024)
	// Build a zip with multiple files so the per-file loop has a chance
	// to observe the cancelled context.
	entries := map[string]string{}
	for i := 0; i < 5; i++ {
		entries[fmt.Sprintf("f%d.txt", i)] = "x"
	}
	data := bulkTestZip(t, entries)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := bulkSvc.Submit(ctx, BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err == nil {
		t.Error("want context error from loop")
	}
}

func TestBulkService_Submit_ExtractionFailure(t *testing.T) {
	// An archive that extracts fine into the zip.Reader but has an entry
	// exceeding maxUpload — Submit should Finalize as failed.
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10)
	data := bulkTestZip(t, map[string]string{"big.txt": strings.Repeat("X", 100)})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       uuid.New(),
		ArchiveBytes: data,
	})
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Errorf("want ErrZipRejected, got %v", err)
	}
}

// --- bulk_handler.go HTTP layer ---

// buildBulkHandler wires a real BulkHandler backed by in-memory fakes.
func buildBulkHandler(t *testing.T) (*BulkHandler, *BulkService) {
	t.Helper()
	evSvc, _, _, _ := newTestService(t)
	repo := newFakeBulkRepo()
	svc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	h := NewBulkHandler(svc, noopAudit{}, discardLogger(), 1024*1024)
	return h, svc
}

// authedMultipartRequest builds an authenticated POST with a multipart
// body containing a single "archive" file field.
func authedMultipartRequest(t *testing.T, caseID string, archive []byte, filename, classification string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if classification != "" {
		_ = mw.WriteField("classification", classification)
	}
	fw, err := mw.CreateFormFile("archive", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(archive); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/bulk", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "tester",
		Username:   "tester",
		SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// noopAudit for the evidence package — auth.AuditLogger has one method.
type noopAudit struct{}

func (noopAudit) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

func TestBulkHandler_Submit_HappyPath(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	archive := bulkTestZip(t, map[string]string{"a.txt": "A"})
	r := authedMultipartRequest(t, caseID, archive, "bulk.zip", ClassificationRestricted)
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestBulkHandler_Submit_Unauthenticated(t *testing.T) {
	h, _ := buildBulkHandler(t)
	archive := bulkTestZip(t, map[string]string{"a.txt": "A"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("archive", "bulk.zip")
	_, _ = fw.Write(archive)
	_ = mw.Close()
	caseID := uuid.New().String()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/bulk", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBulkHandler_Submit_InvalidCaseID(t *testing.T) {
	h, _ := buildBulkHandler(t)
	archive := bulkTestZip(t, map[string]string{"a.txt": "A"})
	r := authedMultipartRequest(t, "not-a-uuid", archive, "bulk.zip", "")
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkHandler_Submit_InvalidMultipart(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/bulk",
		strings.NewReader("not multipart"))
	r.Header.Set("Content-Type", "text/plain")
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkHandler_Submit_MissingArchiveField(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("classification", "restricted")
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/bulk", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkHandler_Submit_ArchiveTooLarge(t *testing.T) {
	// Create a handler with a tiny maxArchive so the cap fires.
	evSvc, _, _, _ := newTestService(t)
	repo := newFakeBulkRepo()
	svc := NewBulkService(repo, evSvc, discardLogger(), 1024)
	h := &BulkHandler{
		svc:        svc,
		audit:      noopAudit{},
		logger:     discardLogger(),
		maxArchive: 10, // tiny
	}
	caseID := uuid.New().String()
	archive := bulkTestZip(t, map[string]string{"a.txt": "AAAAAAAAAAAAAAAAAAA"})
	r := authedMultipartRequest(t, caseID, archive, "bulk.zip", "")
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 413 or 400", w.Code)
	}
}

func TestBulkHandler_Submit_InternalError(t *testing.T) {
	// A fake repo whose Create fails forces Submit to return a non-
	// ErrZipRejected error, exercising the 500 fall-through branch.
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &createErrorBulkRepo{fakeBulkRepo: base}
	svc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	h := NewBulkHandler(svc, noopAudit{}, discardLogger(), 1024*1024)

	caseID := uuid.New().String()
	archive := bulkTestZip(t, map[string]string{"a.txt": "A"})
	r := authedMultipartRequest(t, caseID, archive, "bulk.zip", "")
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestBulkHandler_Submit_ZipRejected(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	// Malformed archive bytes.
	r := authedMultipartRequest(t, caseID, []byte("not a zip"), "bad.zip", "")
	w := httptest.NewRecorder()
	h.Submit(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkHandler_Status_HappyPath(t *testing.T) {
	h, svc := buildBulkHandler(t)
	caseID := uuid.New()
	// Seed a real job via Submit.
	archive := bulkTestZip(t, map[string]string{"a.txt": "A"})
	job, err := svc.Submit(context.Background(), BulkSubmitInput{
		CaseID:       caseID,
		ArchiveBytes: archive,
		UploadedBy:   "tester",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("GET",
		"/api/cases/"+caseID.String()+"/evidence/bulk/"+job.ID.String()+"/status", nil)
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID.String())
	rctx.URLParams.Add("jobID", job.ID.String())
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestBulkHandler_Status_InvalidCaseID(t *testing.T) {
	h, _ := buildBulkHandler(t)
	r := httptest.NewRequest("GET", "/api/cases/bad/evidence/bulk/"+uuid.New().String()+"/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", "not-a-uuid")
	rctx.URLParams.Add("jobID", uuid.New().String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestBulkHandler_Status_InvalidJobID(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	r := httptest.NewRequest("GET", "/api/cases/"+caseID+"/evidence/bulk/bad/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	rctx.URLParams.Add("jobID", "not-a-uuid")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestBulkHandler_Status_NotFound(t *testing.T) {
	h, _ := buildBulkHandler(t)
	caseID := uuid.New().String()
	r := httptest.NewRequest("GET",
		"/api/cases/"+caseID+"/evidence/bulk/"+uuid.New().String()+"/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	rctx.URLParams.Add("jobID", uuid.New().String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// getErrorBulkRepo fails FindByID with a generic error.
type getErrorBulkRepo struct{ *fakeBulkRepo }

func (g *getErrorBulkRepo) FindByID(_ context.Context, _, _ uuid.UUID) (BulkJob, error) {
	return BulkJob{}, errors.New("db down")
}

func TestBulkHandler_Status_InternalError(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &getErrorBulkRepo{fakeBulkRepo: base}
	svc := NewBulkService(repo, evSvc, discardLogger(), 1024*1024)
	h := NewBulkHandler(svc, noopAudit{}, discardLogger(), 1024*1024)

	caseID := uuid.New().String()
	r := httptest.NewRequest("GET",
		"/api/cases/"+caseID+"/evidence/bulk/"+uuid.New().String()+"/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	rctx.URLParams.Add("jobID", uuid.New().String())
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Status(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestBulkHandler_RegisterRoutes_CoverageFill(t *testing.T) {
	h, _ := buildBulkHandler(t)
	router := chi.NewRouter()
	h.RegisterRoutes(router)
}

func TestNewBulkHandler_DefaultsLogger(t *testing.T) {
	svc := &BulkService{}
	h := NewBulkHandler(svc, noopAudit{}, nil, 1024)
	if h.logger == nil {
		t.Error("logger should default to slog.Default")
	}
}

func TestNewBulkHandler_ZeroMaxUpload(t *testing.T) {
	svc := &BulkService{}
	h := NewBulkHandler(svc, noopAudit{}, discardLogger(), 0)
	if h.maxArchive != 1<<30 {
		t.Errorf("maxArchive = %d, want 1GB fallback", h.maxArchive)
	}
}

// --- bulk_repository.go: real PG code paths we can mock ---

// pgBulkMockRow implements pgx.Row for the bulk repository tests.
// Reusing the shape from the existing repository_test.go mockRow.

// TestPGBulkJobRepository_UpdateProgress_DBError — using the shared mockDBPool.
func TestPGBulkJobRepository_UpdateProgress_DBError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.UpdateProgress(context.Background(), uuid.New(), 0, 0, 0, BulkStatusProcessing); err == nil {
		t.Error("want error")
	}
}

func TestPGBulkJobRepository_UpdateProgress_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.UpdateProgress(context.Background(), uuid.New(), 0, 0, 0, BulkStatusProcessing); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestPGBulkJobRepository_AppendError_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.AppendError(context.Background(), uuid.New(), BulkJobError{Filename: "f", Reason: "r"}); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestPGBulkJobRepository_Finalize_DBError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.Finalize(context.Background(), uuid.New(), BulkStatusCompleted); err == nil {
		t.Error("want error")
	}
}

func TestPGBulkJobRepository_Finalize_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.Finalize(context.Background(), uuid.New(), BulkStatusCompleted); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestPGBulkJobRepository_SetArchiveHash_DBError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db down")
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.SetArchiveHash(context.Background(), uuid.New(), strings.Repeat("a", 64)); err == nil {
		t.Error("want error")
	}
}

func TestPGBulkJobRepository_SetArchiveHash_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if err := r.SetArchiveHash(context.Background(), uuid.New(), strings.Repeat("a", 64)); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestPGBulkJobRepository_Create_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db down")}
		},
	}
	r := &PGBulkJobRepository{pool: pool}
	if _, err := r.Create(context.Background(), uuid.New(), "admin", "key"); err == nil {
		t.Error("want error")
	}
}

// --- migrate_adapter.go hash-drift branch ---

// hashDriftService wraps a mockRepo so Upload returns an evidence item
// with a different hash than expected, triggering the drift assertion.
// Actually, evidence.Service.Upload uses the real service path; we'll
// call StoreMigratedFile directly with a mismatching ComputedHash.

func TestStoreMigratedFile_HashDriftRejected(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// Upload reads and hashes the provided Reader. If we pass in.ComputedHash
	// that doesn't match the real content hash, StoreMigratedFile asserts
	// drift and returns an error.
	ctx := context.Background()
	_, err := svc.StoreMigratedFile(ctx, MigrationStoreInput{
		CaseID:         uuid.New(),
		Filename:       "a.txt",
		OriginalName:   "a.txt",
		Reader:         strings.NewReader("hello"),
		SizeBytes:      5,
		ComputedHash:   strings.Repeat("0", 64), // deliberately wrong
		Classification: ClassificationRestricted,
		UploadedBy:     "tester",
	})
	if err == nil || !strings.Contains(err.Error(), "hash drift") {
		t.Errorf("want hash drift error, got %v", err)
	}
}

func TestStoreMigratedFile_HappyPath(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// Correct hash of "hello".
	wantHash := hex.EncodeToString(sha256Sum([]byte("hello")))
	res, err := svc.StoreMigratedFile(context.Background(), MigrationStoreInput{
		CaseID:         uuid.New(),
		Filename:       "a.txt",
		OriginalName:   "a.txt",
		Reader:         strings.NewReader("hello"),
		SizeBytes:      5,
		ComputedHash:   wantHash,
		Classification: ClassificationRestricted,
		UploadedBy:     "tester",
		Source:         "RelativityOne",
		SourceHash:     wantHash,
		SourceDate:     bulkPtrTime(time.Now()),
		CustodyDetail:  map[string]string{"manifest_entry": "row 1"},
	})
	if err != nil {
		t.Fatalf("StoreMigratedFile: %v", err)
	}
	if res.EvidenceID == uuid.Nil {
		t.Error("EvidenceID should be set")
	}
}

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func bulkPtrTime(t time.Time) *time.Time { return &t }
