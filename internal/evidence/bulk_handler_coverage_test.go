package evidence

// Unit coverage for bulk_handler.go. Uses a fake BulkService so the
// handler wiring can be exercised without real ZIP processing.

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

func newBulkHandlerTest(svc *BulkService) *BulkHandler {
	return NewBulkHandler(svc, &mockAudit{}, slog.New(slog.NewTextHandler(io.Discard, nil)), 10*1024*1024)
}

func bulkAuthReq(method, url string, body io.Reader, contentType string, admin bool) *http.Request {
	req := httptest.NewRequest(method, url, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	sysRole := auth.RoleAPIService
	if admin {
		sysRole = auth.RoleSystemAdmin
	}
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{
		UserID:     uuid.New().String(),
		Username:   "admin",
		SystemRole: sysRole,
	})
	return req.WithContext(ctx)
}

// makeZipBody builds a minimal multipart body with an "archive" field
// containing a one-file ZIP — enough to drive the Submit happy path
// through the handler + service layer without a real on-disk fixture.
func makeZipBody(t *testing.T, includeArchive bool) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if includeArchive {
		fw, err := mw.CreateFormFile("archive", "test.zip")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		// Write a minimal valid ZIP.
		zipBuf := &bytes.Buffer{}
		zw := zip.NewWriter(zipBuf)
		f, err := zw.Create("file.txt")
		if err != nil {
			t.Fatalf("zip.Create: %v", err)
		}
		_, _ = f.Write([]byte("hello"))
		_ = zw.Close()
		_, _ = fw.Write(zipBuf.Bytes())
	}
	_ = mw.Close()
	return body, mw.FormDataContentType()
}

// ---- NewBulkHandler constructor ----

func TestNewBulkHandler_DefaultsAndLogger(t *testing.T) {
	// Pass nil logger to exercise the default assignment branch.
	h := NewBulkHandler(nil, &mockAudit{}, nil, 0)
	if h == nil {
		t.Fatal("nil handler")
	}
	if h.maxArchive <= 0 {
		t.Error("maxArchive fallback not applied")
	}
}

// ---- RegisterRoutes ----

func TestBulkHandler_RegisterRoutes(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	// Not panicking is success; the actual path serving is covered by
	// downstream handler tests.
}

// ---- Submit ----

func TestBulkSubmit_NoAuth(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}", func(r chi.Router) { r.Post("/bulk", h.Submit) })
	body, ct := makeZipBody(t, true)
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+uuid.New().String()+"/bulk", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestBulkSubmit_InvalidCaseID(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}", func(r chi.Router) { r.Post("/bulk", h.Submit) })
	body, ct := makeZipBody(t, true)
	req := bulkAuthReq(http.MethodPost, "/api/cases/not-uuid/bulk", body, ct, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkSubmit_MissingArchiveField(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}", func(r chi.Router) { r.Post("/bulk", h.Submit) })
	body, ct := makeZipBody(t, false) // no archive field
	req := bulkAuthReq(http.MethodPost, "/api/cases/"+uuid.New().String()+"/bulk", body, ct, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkSubmit_InvalidMultipart(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}", func(r chi.Router) { r.Post("/bulk", h.Submit) })
	req := bulkAuthReq(http.MethodPost, "/api/cases/"+uuid.New().String()+"/bulk",
		bytes.NewBufferString("not multipart"), "application/json", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- Status ----

type coverageFakeBulkRepo struct {
	job BulkJob
	err error
}

func (f *coverageFakeBulkRepo) Create(_ context.Context, _ uuid.UUID, _, _ string) (BulkJob, error) {
	return f.job, f.err
}
func (f *coverageFakeBulkRepo) SetArchiveHash(_ context.Context, _ uuid.UUID, _ string) error {
	return f.err
}
func (f *coverageFakeBulkRepo) UpdateProgress(_ context.Context, _ uuid.UUID, _, _, _ int, _ string) error {
	return f.err
}
func (f *coverageFakeBulkRepo) AppendError(_ context.Context, _ uuid.UUID, _ BulkJobError) error {
	return f.err
}
func (f *coverageFakeBulkRepo) Finalize(_ context.Context, _ uuid.UUID, _ string) error {
	return f.err
}
func (f *coverageFakeBulkRepo) FindByID(_ context.Context, _, _ uuid.UUID) (BulkJob, error) {
	return f.job, f.err
}

func TestBulkStatus_InvalidCaseID(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/bulk/{jobID}", func(r chi.Router) { r.Get("/", h.Status) })
	req := bulkAuthReq(http.MethodGet, "/api/cases/not-uuid/bulk/"+uuid.New().String()+"/", nil, "", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkStatus_InvalidJobID(t *testing.T) {
	h := newBulkHandlerTest(nil)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/bulk/{jobID}", func(r chi.Router) { r.Get("/", h.Status) })
	req := bulkAuthReq(http.MethodGet, "/api/cases/"+uuid.New().String()+"/bulk/not-uuid/", nil, "", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestBulkStatus_NotFound(t *testing.T) {
	svc := &BulkService{
		jobs:   &coverageFakeBulkRepo{err: ErrBulkJobNotFound},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h := newBulkHandlerTest(svc)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/bulk/{jobID}", func(r chi.Router) { r.Get("/", h.Status) })
	req := bulkAuthReq(http.MethodGet, "/api/cases/"+uuid.New().String()+"/bulk/"+uuid.New().String()+"/", nil, "", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestBulkStatus_GenericError(t *testing.T) {
	svc := &BulkService{
		jobs:   &coverageFakeBulkRepo{err: errors.New("db err")},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h := newBulkHandlerTest(svc)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/bulk/{jobID}", func(r chi.Router) { r.Get("/", h.Status) })
	req := bulkAuthReq(http.MethodGet, "/api/cases/"+uuid.New().String()+"/bulk/"+uuid.New().String()+"/", nil, "", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestBulkStatus_Success(t *testing.T) {
	job := BulkJob{ID: uuid.New(), Status: BulkStatusCompleted}
	svc := &BulkService{
		jobs:   &coverageFakeBulkRepo{job: job},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h := newBulkHandlerTest(svc)
	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/bulk/{jobID}", func(r chi.Router) { r.Get("/", h.Status) })
	req := bulkAuthReq(http.MethodGet, "/api/cases/"+uuid.New().String()+"/bulk/"+uuid.New().String()+"/", nil, "", true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ---- BulkService.Get (delegate to repo) ----

func TestBulkService_Get_Success(t *testing.T) {
	want := BulkJob{ID: uuid.New()}
	svc := &BulkService{jobs: &coverageFakeBulkRepo{job: want}}
	got, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if err != nil || got.ID != want.ID {
		t.Errorf("got %+v err=%v", got, err)
	}
}

func TestBulkService_Get_Error(t *testing.T) {
	svc := &BulkService{jobs: &coverageFakeBulkRepo{err: ErrBulkJobNotFound}}
	_, err := svc.Get(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}
