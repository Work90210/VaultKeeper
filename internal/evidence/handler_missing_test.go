package evidence

// Tests for the three handler functions that were previously at 0% coverage:
//   - SetRedactionService (line 48)
//   - UploadNewVersion   (line 423)
//   - ApplyRedactions    (line 496)
//   - PreviewRedactions  (line 531)
//
// Pattern mirrors handler_test.go: httptest + chi router + mockRepo / mockStorage.

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)


// ---------------------------------------------------------------------------
// mockRedactionService — satisfies the interface used by the handler so we can
// inject either success or error responses without a real MuPDF/pdfcpu chain.
// ---------------------------------------------------------------------------

// mockRedactionService wraps a real RedactionService but can be replaced by a
// stub.  Because ApplyRedactions and PreviewRedactions delegate directly to
// h.redaction, we need a concrete *RedactionService.
//
// For "nil redaction service" tests we simply don't call SetRedactionService.
//
// For "service returns error" tests we set up the backing repo/storage so that
// the service call fails at a known point.

// newTestHandlerWithRedaction builds a Handler wired to a real RedactionService
// backed by mock dependencies.
func newTestHandlerWithRedaction(t *testing.T) (*Handler, *mockRepo, *mockStorage) {
	t.Helper()
	repo := newMockRepo()
	storage := newMockStorage()
	caseLookup := &mockCaseLookup{}
	logger := newDiscardLogger(t)

	svc := newServiceWith(repo, storage, caseLookup, logger)
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, &mockCustody{}, logger)

	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	handler.SetRedactionService(rs)

	return handler, repo, storage
}

func newDiscardLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// SetRedactionService
// ---------------------------------------------------------------------------

// TestHandler_RegisterRoutes_AllPathsMounted exercises the RegisterRoutes method
// to confirm it mounts all expected paths without panicking.
// It checks paths that require auth (no auth context → 500) so the response is
// unambiguously not a routing 404.
func TestHandler_RegisterRoutes_AllPathsMounted(t *testing.T) {
	h, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r) // Must not panic

	// These routes all check auth first (returning 500 when missing auth context),
	// so a non-404 confirms the route is mounted.
	id := uuid.New()
	paths := []string{
		"/api/cases/" + uuid.NewString() + "/evidence",
		"/api/evidence/" + id.String() + "/download",
		"/api/evidence/" + id.String() + "/custody",
		"/api/evidence/" + id.String() + "/version",
		"/api/evidence/" + id.String() + "/redact",
		"/api/evidence/" + id.String() + "/redact/preview",
	}
	methods := []string{
		http.MethodGet,
		http.MethodGet,
		http.MethodGet,
		http.MethodPost,
		http.MethodPost,
		http.MethodPost,
	}

	for i, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(methods[i], path, nil)
			// No auth context: every auth-gated handler returns 500 or 503
			// (not 404 which would indicate the route was never mounted)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusNotFound {
				t.Errorf("path %s returned 404 — route may not be mounted", path)
			}
		})
	}
}

func TestHandler_SetRedactionService(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	// Before setting, redaction is nil.
	if handler.redaction != nil {
		t.Fatal("expected redaction to be nil before SetRedactionService")
	}

	repo := newMockRepo()
	storage := newMockStorage()
	caseLookup := &mockCaseLookup{}
	svc := newServiceWith(repo, storage, caseLookup, nil)
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, &mockCustody{}, nil)

	handler.SetRedactionService(rs)

	if handler.redaction == nil {
		t.Fatal("expected redaction to be set after SetRedactionService")
	}
	if handler.redaction != rs {
		t.Error("handler.redaction should be the same pointer passed to SetRedactionService")
	}
}

// newTestHandlerWithStorage creates a plain handler (no RedactionService) plus
// a concrete storage reference so tests can pre-populate objects.
func newTestHandlerWithStorage(t *testing.T) (*Handler, *mockRepo, *mockStorage) {
	t.Helper()
	repo := newMockRepo()
	storage := newMockStorage()
	caseLookup := &mockCaseLookup{}
	svc := newServiceWith(repo, storage, caseLookup, nil)
	h := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	return h, repo, storage
}

// newServiceWith creates a Service from explicit dependencies (for reuse across tests).
func newServiceWith(repo Repository, storage ObjectStorage, caseLookup CaseLookup, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, caseLookup,
		&noopThumbGen{}, logger, 100*1024*1024,
	)
}

// ---------------------------------------------------------------------------
// UploadNewVersion
// ---------------------------------------------------------------------------

// buildVersionMultipart creates a multipart body with a "file" field and the
// client_sha256 form field required by the upload hash validation.
// Returns the buffer, content-type, and the SHA-256 hex of content (for the header).
func buildVersionMultipart(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write part: %v", err)
	}
	w.WriteField("classification", "restricted")
	w.WriteField("description", "new version")
	w.WriteField("client_sha256", sha256Hex(content))
	w.Close()
	return &buf, w.FormDataContentType()
}

func TestHandler_UploadNewVersion_NoAuth(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	buf, ct := buildVersionMultipart(t, "v2.pdf", "content")
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.NewString()+"/version", buf)
	req.Header.Set("Content-Type", ct)
	// No auth context

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_InvalidID(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	buf, ct := buildVersionMultipart(t, "v2.pdf", "content")
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/not-a-uuid/version", buf)
	req.Header.Set("Content-Type", ct)
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_NoFileField(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	// Multipart with no "file" field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("classification", "restricted")
	mw.Close()

	parentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_InvalidMultipart(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	// Send non-multipart body — ParseMultipartForm will fail
	parentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version",
		strings.NewReader("not-multipart"))
	req.Header.Set("Content-Type", "text/plain")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_ParentNotFound(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	// Parent ID does not exist in the mock repo
	const vContent = "content"
	buf, ct := buildVersionMultipart(t, "v2.pdf", vContent)
	parentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", buf)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Content-SHA256", sha256Hex(vContent))
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_Success(t *testing.T) {
	handler, repo, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	// Seed a parent evidence item
	parentID := uuid.New()
	caseID := uuid.New()
	repo.items[parentID] = EvidenceItem{
		ID:      parentID,
		CaseID:  caseID,
		Version: 1,
		Tags:    []string{},
	}

	const vContent = "new file content"
	buf, ct := buildVersionMultipart(t, "v2.pdf", vContent)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", buf)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Content-SHA256", sha256Hex(vContent))
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_WithTagsJSON(t *testing.T) {
	handler, repo, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	parentID := uuid.New()
	caseID := uuid.New()
	repo.items[parentID] = EvidenceItem{
		ID:      parentID,
		CaseID:  caseID,
		Version: 1,
		Tags:    []string{},
	}

	const vContent = "file content"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "v2.pdf")
	part.Write([]byte(vContent))
	mw.WriteField("classification", "restricted")
	mw.WriteField("tags", `["tag1","tag2"]`)
	mw.WriteField("source_date", "2024-01-15")
	mw.WriteField("client_sha256", sha256Hex(vContent))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Content-SHA256", sha256Hex(vContent))
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_WithTagsCSV(t *testing.T) {
	handler, repo, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	parentID := uuid.New()
	caseID := uuid.New()
	repo.items[parentID] = EvidenceItem{
		ID:      parentID,
		CaseID:  caseID,
		Version: 1,
		Tags:    []string{},
	}

	const vContent = "file content"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "v2.pdf")
	part.Write([]byte(vContent))
	mw.WriteField("classification", "restricted")
	// Invalid JSON → fallback to comma-separated
	mw.WriteField("tags", "tag1,tag2,tag3")
	mw.WriteField("client_sha256", sha256Hex(vContent))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Content-SHA256", sha256Hex(vContent))
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UploadNewVersion_WithSourceDateISO(t *testing.T) {
	handler, repo, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/version", handler.UploadNewVersion)
	})

	parentID := uuid.New()
	caseID := uuid.New()
	repo.items[parentID] = EvidenceItem{
		ID:      parentID,
		CaseID:  caseID,
		Version: 1,
		Tags:    []string{},
	}

	const vContent = "file content"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "v2.pdf")
	part.Write([]byte(vContent))
	mw.WriteField("classification", "restricted")
	// RFC3339 source_date
	mw.WriteField("source_date", "2024-06-15T12:00:00Z")
	mw.WriteField("client_sha256", sha256Hex(vContent))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+parentID.String()+"/version", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Content-SHA256", sha256Hex(vContent))
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ApplyRedactions
// ---------------------------------------------------------------------------

func TestHandler_ApplyRedactions_NoAuth(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	// No auth context

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ApplyRedactions_NilRedactionService(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)
	// redaction is nil by default — do NOT call SetRedactionService

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ApplyRedactions_InvalidID(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/not-a-uuid/redact",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ApplyRedactions_InvalidBody(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact",
		strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ApplyRedactions_ServiceError(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	// Evidence does not exist → ApplyRedactions will return not-found error
	id := uuid.New()
	body := `{"redactions":[{"page_number":1,"x":10,"y":10,"width":20,"height":20,"reason":"PII"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// ErrNotFound from mock repo → 404
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// PreviewRedactions
// ---------------------------------------------------------------------------

func TestHandler_PreviewRedactions_NoAuth(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact/preview",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	// No auth context

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PreviewRedactions_NilRedactionService(t *testing.T) {
	handler, _, _ := newTestHandlerWithStorage(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact/preview",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PreviewRedactions_InvalidID(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/not-a-uuid/redact/preview",
		strings.NewReader(`{"redactions":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PreviewRedactions_InvalidBody(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact/preview",
		strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PreviewRedactions_ServiceError(t *testing.T) {
	handler, _, _ := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	// Evidence does not exist → PreviewRedactions returns not-found
	id := uuid.New()
	body := `{"redactions":[{"page_number":1,"x":10,"y":10,"width":20,"height":20,"reason":"PII"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact/preview",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_PreviewRedactions_Success(t *testing.T) {
	handler, repo, storage := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/preview", handler.PreviewRedactions)
	})

	// Seed a PNG evidence item so the redaction service can load and process it
	id := uuid.New()
	caseID := uuid.New()
	storageKey := "evidence/preview/test.png"
	pngData := createSmallPNG(50, 50)
	storage.objects[storageKey] = pngData

	repo.items[id] = EvidenceItem{
		ID:         id,
		CaseID:     caseID,
		Filename:   "test.png",
		MimeType:   "image/png",
		SizeBytes:  int64(len(pngData)),
		StorageKey: &storageKey,
		Tags:       []string{},
	}

	body := `{"redactions":[{"page_number":0,"x":10,"y":10,"width":20,"height":20,"reason":"PII"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact/preview",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected non-empty Content-Type in preview response")
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("expected non-empty preview image body")
	}
}

func TestHandler_ApplyRedactions_Success(t *testing.T) {
	handler, repo, storage := newTestHandlerWithRedactionService(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact", handler.ApplyRedactions)
	})

	// Seed a PNG evidence item
	id := uuid.New()
	caseID := uuid.New()
	storageKey := "evidence/apply/test.png"
	pngData := createSmallPNG(50, 50)
	storage.objects[storageKey] = pngData

	evidenceNum := "EV-APPLY-001"
	repo.items[id] = EvidenceItem{
		ID:             id,
		CaseID:         caseID,
		Filename:       "test.png",
		MimeType:       "image/png",
		SizeBytes:      int64(len(pngData)),
		StorageKey:     &storageKey,
		EvidenceNumber: &evidenceNum,
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}

	body := `{"redactions":[{"page_number":0,"x":10,"y":10,"width":20,"height":20,"reason":"PII"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+id.String()+"/redact",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Shared helper: handler wired to a real RedactionService backed by mocks.
// ---------------------------------------------------------------------------

// newTestHandlerWithRedactionService builds a Handler with a real
// RedactionService injected. Returns the handler, backing repo, and backing
// storage so tests can pre-populate objects.
func newTestHandlerWithRedactionService(t *testing.T) (*Handler, *mockRepo, *mockStorage) {
	t.Helper()

	repo := newMockRepo()
	storage := newMockStorage()
	caseLookup := &mockCaseLookup{}
	custody := &mockCustody{}

	svc := newServiceWith(repo, storage, caseLookup, nil)
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{}, custody, nil)

	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	handler.SetRedactionService(rs)

	return handler, repo, storage
}

