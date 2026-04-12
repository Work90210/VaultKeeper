package evidence

// Unit-level coverage tests for pages_handler.go. Uses the dbPool
// abstraction (injected via newPagesHandlerFromPool) to exercise
// lookupEvidenceInfo, GetPageCount, and GetPage without requiring a
// Postgres container.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// poolForPages wires a mockDBPool whose QueryRow scan populates the
// (case_id, storage_key) destinations used by lookupEvidenceInfo.
func poolForPages(caseID uuid.UUID, storageKey *string, scanErr error) *mockDBPool {
	return &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{
				scanErr: scanErr,
				scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = caseID
					*(dest[1].(**string)) = storageKey
					return nil
				},
			}
		},
	}
}

func newUnitPagesHandler(t *testing.T, pool dbPool, storage ObjectStorage) *PagesHandler {
	t.Helper()
	return newPagesHandlerFromPool(pool, storage,
		&mockDraftRoleLoader{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

// ---- lookupEvidenceInfo ----

func TestUnit_LookupEvidenceInfo_Success(t *testing.T) {
	wantCase := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(wantCase, &key, nil)
	h := newUnitPagesHandler(t, pool, newMockStorage())
	gotCase, gotKey, err := h.lookupEvidenceInfo(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if gotCase != wantCase || gotKey != key {
		t.Errorf("got (%s, %s), want (%s, %s)", gotCase, gotKey, wantCase, key)
	}
}

func TestUnit_LookupEvidenceInfo_ScanError(t *testing.T) {
	pool := poolForPages(uuid.Nil, nil, errors.New("db down"))
	h := newUnitPagesHandler(t, pool, newMockStorage())
	_, _, err := h.lookupEvidenceInfo(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error")
	}
}

func TestUnit_LookupEvidenceInfo_NilStorageKey(t *testing.T) {
	pool := poolForPages(uuid.New(), nil, nil)
	h := newUnitPagesHandler(t, pool, newMockStorage())
	_, _, err := h.lookupEvidenceInfo(context.Background(), uuid.New())
	if err == nil || err.Error() != "evidence has no storage key" {
		t.Errorf("want no-storage-key error, got %v", err)
	}
}

// ---- GetPageCount ----

func TestUnit_GetPageCount_EvidenceNotFound(t *testing.T) {
	pool := poolForPages(uuid.Nil, nil, pgx.ErrNoRows)
	h := newUnitPagesHandler(t, pool, newMockStorage())
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestUnit_GetPageCount_LookupError(t *testing.T) {
	pool := poolForPages(uuid.Nil, nil, errors.New("db down"))
	h := newUnitPagesHandler(t, pool, newMockStorage())
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPageCount_RoleCheckForbidden(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	h := newPagesHandlerFromPool(pool, newMockStorage(),
		&mockDraftRoleLoader{err: auth.ErrNoCaseRole},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesUserContext(req, uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", w.Code)
	}
}

func TestUnit_GetPageCount_RoleCheckError(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	h := newPagesHandlerFromPool(pool, newMockStorage(),
		&mockDraftRoleLoader{err: errors.New("role backend down")},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesUserContext(req, uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPageCount_CacheHit(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	// Pre-populate the page_count cache for this evidence ID.
	storage := pagesStorageWithJSON("page-cache/"+evidenceID.String()+"/page_count.json", 42)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestUnit_GetPageCount_StorageFetchError(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	// Storage returns an error for any GetObject call.
	storage := newMockStorage()
	storage.getErr = errors.New("storage unavailable")
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPageCount_InvalidPDF(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte("not a pdf")
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPageCount_Success(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// ---- GetPage ----

func TestUnit_GetPage_EvidenceNotFound(t *testing.T) {
	pool := poolForPages(uuid.Nil, nil, pgx.ErrNoRows)
	h := newUnitPagesHandler(t, pool, newMockStorage())
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestUnit_GetPage_LookupError(t *testing.T) {
	pool := poolForPages(uuid.Nil, nil, errors.New("db down"))
	h := newUnitPagesHandler(t, pool, newMockStorage())
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPage_RoleCheckForbidden(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	h := newPagesHandlerFromPool(pool, newMockStorage(),
		&mockDraftRoleLoader{err: auth.ErrNoCaseRole},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesUserContext(req, uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", w.Code)
	}
}

func TestUnit_GetPage_RoleCheckError(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	h := newPagesHandlerFromPool(pool, newMockStorage(),
		&mockDraftRoleLoader{err: errors.New("role backend down")},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesUserContext(req, uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPage_CacheHit(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	cacheKey := "page-cache/" + evidenceID.String() + "/1_150.jpg"
	storage := pagesStorageWithJPEG(cacheKey, []byte("fake-jpeg-bytes"))
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestUnit_GetPage_StorageFetchError(t *testing.T) {
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.getErr = errors.New("storage down")
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPage_RenderError_PageOutOfRange(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	// Page 99 does not exist in the single-page minimalPDF → render returns
	// error → handler responds 404.
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/99", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestUnit_GetPage_Success(t *testing.T) {
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}
