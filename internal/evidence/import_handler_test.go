package evidence

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// fakeImportRunner records whether Run was called and returns a
// canned result or error.
type fakeImportRunner struct {
	called atomic.Int32
	result ImportRunResult
	err    error
}

func (f *fakeImportRunner) Run(_ context.Context, _ ImportRunInput) (ImportRunResult, error) {
	f.called.Add(1)
	if f.err != nil {
		return ImportRunResult{}, f.err
	}
	return f.result, nil
}

// buildImportHandler wires an ImportHandler backed by in-memory fakes.
func buildImportHandler(t *testing.T, runner ImportRunner) *ImportHandler {
	t.Helper()
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10*1024*1024)
	return NewImportHandler(bulkSvc, runner, noopAudit{}, discardLogger(), 10*1024*1024)
}

// buildZip builds an in-memory ZIP with the given entries.
func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// importMultipartRequest builds an authenticated POST with the archive
// and optional form fields.
func importMultipartRequest(t *testing.T, caseID string, archive []byte, filename string, extraFields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range extraFields {
		_ = mw.WriteField(k, v)
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
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/import", &body)
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

func TestImport_BulkPath_WithoutManifest(t *testing.T) {
	runner := &fakeImportRunner{}
	h := buildImportHandler(t, runner)
	zipData := buildZip(t, map[string]string{
		"a.txt": "alpha",
		"b.txt": "bravo",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "plain.zip", map[string]string{
		"classification": "restricted",
	})
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if runner.called.Load() != 0 {
		t.Error("migration runner should not be called when manifest.csv is absent")
	}
	var env struct {
		Data ImportResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Kind != "bulk" {
		t.Errorf("Kind = %q, want bulk", env.Data.Kind)
	}
	if env.Data.BulkJob == nil || env.Data.BulkJob.Processed != 2 {
		t.Errorf("BulkJob = %+v", env.Data.BulkJob)
	}
}

func TestImport_MigrationPath_WithManifest(t *testing.T) {
	fixedID := uuid.New()
	ts := time.Now().UTC()
	runner := &fakeImportRunner{
		result: ImportRunResult{
			MigrationID:  fixedID,
			TotalItems:   2,
			MatchedItems: 2,
			Status:       "completed",
			TSAName:      "FakeTSA",
			TSATimestamp: &ts,
		},
	}
	h := buildImportHandler(t, runner)

	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename,sha256_hash\na.txt," + strings.Repeat("a", 64) + "\n",
		"a.txt":        "alpha",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "verified.zip", map[string]string{
		"source_system":    "RelativityOne",
		"halt_on_mismatch": "true",
	})
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if runner.called.Load() != 1 {
		t.Errorf("runner called %d times, want 1", runner.called.Load())
	}
	var env struct {
		Data ImportResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Data.Kind != "migration" {
		t.Errorf("Kind = %q, want migration", env.Data.Kind)
	}
	if env.Data.Migration == nil || env.Data.Migration.ID != fixedID {
		t.Errorf("Migration = %+v", env.Data.Migration)
	}
	if env.Data.Migration.TSAName != "FakeTSA" {
		t.Errorf("TSAName = %q", env.Data.Migration.TSAName)
	}
}

func TestImport_MigrationRunnerNil_FallsBackToBulk(t *testing.T) {
	h := buildImportHandler(t, nil) // migration runner is nil
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "alpha",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "fallback.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var env struct {
		Data ImportResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Data.Kind != "bulk" {
		t.Errorf("Kind = %q, want bulk fallback", env.Data.Kind)
	}
}

func TestImport_HashMismatchReturns409(t *testing.T) {
	runner := &fakeImportRunner{
		err: errors.New("hash mismatch for a.txt"),
	}
	h := buildImportHandler(t, runner)
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "alpha",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "bad.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestImport_ManifestInvalidReturns400(t *testing.T) {
	runner := &fakeImportRunner{
		err: errors.New("parse manifest: manifest is empty"),
	}
	h := buildImportHandler(t, runner)
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "broken",
		"a.txt":        "alpha",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "bad.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestImport_MigrationRunnerGenericError(t *testing.T) {
	runner := &fakeImportRunner{
		err: errors.New("unexpected server crash"),
	}
	h := buildImportHandler(t, runner)
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "alpha",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "bad.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestImport_Unauthenticated(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	r := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/evidence/import", bytes.NewReader([]byte("x")))
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestImport_InvalidCaseID(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	r := importMultipartRequest(t, "not-a-uuid", buildZip(t, map[string]string{"a.txt": "x"}), "x.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestImport_InvalidMultipart(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	caseID := uuid.New().String()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/import",
		strings.NewReader("not multipart"))
	r.Header.Set("Content-Type", "text/plain")
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestImport_MissingArchiveField(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	caseID := uuid.New().String()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("source_system", "Test")
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestImport_ArchiveTooLarge(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10)
	h := &ImportHandler{
		bulk:       bulkSvc,
		migration:  &fakeImportRunner{},
		logger:     discardLogger(),
		audit:      noopAudit{},
		maxArchive: 20,
		tempBase:   os.TempDir(),
	}
	big := buildZip(t, map[string]string{"x.txt": strings.Repeat("A", 200)})
	r := importMultipartRequest(t, uuid.New().String(), big, "big.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestImport_BulkRejectedArchive(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	// Malformed zip bytes
	r := importMultipartRequest(t, uuid.New().String(), []byte("not a zip"), "bad.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}

func TestImport_BulkPath_EmptyZipReturnsErrZipRejected(t *testing.T) {
	// An empty archive passes detectManifestCSV (returns false) and
	// falls through to bulk.Submit, which rejects it with ErrZipRejected
	// via ExtractBulkZIP's "archive is empty" guard. Exercises the
	// ErrZipRejected branch in the bulk error path.
	h := buildImportHandler(t, &fakeImportRunner{})
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	_ = w.Close()
	r := importMultipartRequest(t, uuid.New().String(), buf.Bytes(), "empty.zip", nil)
	rec := httptest.NewRecorder()
	h.Import(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestImport_MigrationPath_ExtractionFailsAfterDetect(t *testing.T) {
	// Archive passes detectManifestCSV (manifest.csv is present in the
	// central directory) but ExtractBulkZIP rejects it because one of
	// the entries has the symlink mode bit set. Exercises the
	// extraction-error branch inside runMigrationFlow.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// Plain manifest.csv so detectManifestCSV returns true.
	mf, _ := zw.Create("manifest.csv")
	_, _ = mf.Write([]byte("filename\na.txt\n"))
	// Second entry with symlink mode set → ExtractBulkZIP rejects.
	h, _ := zw.CreateHeader(&zip.FileHeader{Name: "a.txt", Method: zip.Store})
	_ = h
	// Manually build a symlink entry.
	fh := &zip.FileHeader{Name: "link", Method: zip.Store}
	fh.SetMode(0o777 | os.ModeSymlink)
	lw, _ := zw.CreateHeader(fh)
	_, _ = lw.Write([]byte("/etc/passwd"))
	_ = zw.Close()

	handler := buildImportHandler(t, &fakeImportRunner{})
	r := importMultipartRequest(t, uuid.New().String(), buf.Bytes(), "bad.zip", nil)
	rec := httptest.NewRecorder()
	handler.Import(rec, r)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestImport_BulkSubmitInternalError(t *testing.T) {
	// Force bulk.Submit to fail with a non-ErrZipRejected error by
	// using a repo that errors on Create.
	evSvc, _, _, _ := newTestService(t)
	base := newFakeBulkRepo()
	repo := &createErrorBulkRepo{fakeBulkRepo: base}
	bulkSvc := NewBulkService(repo, evSvc, discardLogger(), 10*1024*1024)
	h := NewImportHandler(bulkSvc, &fakeImportRunner{}, noopAudit{}, discardLogger(), 10*1024*1024)

	zipData := buildZip(t, map[string]string{"a.txt": "A"})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "bulk.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", w.Code)
	}
}

func TestImportHandler_RegisterRoutes(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	h.RegisterRoutes(chi.NewRouter())
}

func TestNewImportHandler_DefaultsLogger(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 1024)
	h := NewImportHandler(bulkSvc, &fakeImportRunner{}, noopAudit{}, nil, 1024)
	if h.logger == nil {
		t.Error("logger should default to slog.Default")
	}
}

func TestNewImportHandler_ZeroMaxUpload(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 0)
	h := NewImportHandler(bulkSvc, &fakeImportRunner{}, noopAudit{}, discardLogger(), 0)
	if h.maxArchive != 1<<30 {
		t.Errorf("maxArchive = %d, want 1GB fallback", h.maxArchive)
	}
}

func TestDetectManifestCSV(t *testing.T) {
	withManifest := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "A",
	})
	got, err := detectManifestCSV(withManifest)
	if err != nil || !got {
		t.Errorf("want true, got=%v err=%v", got, err)
	}

	without := buildZip(t, map[string]string{"a.txt": "A"})
	got, err = detectManifestCSV(without)
	if err != nil || got {
		t.Errorf("want false, got=%v err=%v", got, err)
	}

	// Handles "./manifest.csv" form emitted by some zip tools.
	dotSlash := buildZip(t, map[string]string{
		"./manifest.csv": "filename\n",
		"a.txt":          "A",
	})
	got, _ = detectManifestCSV(dotSlash)
	if !got {
		t.Error("want true for ./manifest.csv")
	}

	// Invalid zip bytes.
	if _, err := detectManifestCSV([]byte("not a zip")); err == nil {
		t.Error("want error for invalid zip")
	}
}

func TestExtractArchiveToTempDir(t *testing.T) {
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "hello",
		"sub/b.txt":    "nested",
	})
	dir, err := extractArchiveToTempDir(t.TempDir(), zipData)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	defer os.RemoveAll(dir)
	for _, rel := range []string{"manifest.csv", "a.txt", "sub/b.txt"} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestExtractArchiveToTempDir_RejectedArchive(t *testing.T) {
	if _, err := extractArchiveToTempDir(t.TempDir(), []byte("not a zip")); err == nil {
		t.Error("want extraction error")
	}
}

// Verify the temp dir is cleaned up even on a migration runner error.
func TestImport_TempDirCleanedUpOnMigrationError(t *testing.T) {
	runner := &fakeImportRunner{err: errors.New("boom")}
	tempBase := t.TempDir()
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10*1024*1024)
	h := &ImportHandler{
		bulk:       bulkSvc,
		migration:  runner,
		logger:     discardLogger(),
		audit:      noopAudit{},
		maxArchive: 10 * 1024 * 1024,
		tempBase:   tempBase,
	}
	zipData := buildZip(t, map[string]string{
		"manifest.csv": "filename\na.txt\n",
		"a.txt":        "A",
	})
	r := importMultipartRequest(t, uuid.New().String(), zipData, "bad.zip", nil)
	w := httptest.NewRecorder()
	h.Import(w, r)
	// Confirm no leftover vk-import-* dirs in the base.
	entries, _ := os.ReadDir(tempBase)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "vk-import-") {
			t.Errorf("temp dir not cleaned up: %s", e.Name())
		}
	}
}

// Ensure io.ReadAll from a truncated reader doesn't crash the handler.
// Hitting the decode-error path via a stream cut mid-multipart.
func TestImport_ReadArchiveNoPanic(t *testing.T) {
	h := buildImportHandler(t, &fakeImportRunner{})
	// A pre-built multipart body with a truncated file content.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("archive", "a.zip")
	_, _ = fw.Write([]byte{0x50, 0x4B}) // PK header fragment, not a full zip
	_ = mw.Close()
	caseID := uuid.New().String()
	r := httptest.NewRequest("POST", "/api/cases/"+caseID+"/evidence/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID: "tester", SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("caseID", caseID)
	r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.Import(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

var _ = io.Discard // keep io import if refactor removes other uses
