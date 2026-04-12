package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---------------------------------------------------------------------------
// Test helpers specific to PagesHandler
// ---------------------------------------------------------------------------

// newPagesHandler builds a PagesHandler backed by a mock storage and mock role
// loader. It does NOT require a real database — all tests that need DB rows use
// the integration path via startPostgresContainer.
func newPagesHandlerMock(storage ObjectStorage, roleLoader CaseRoleChecker) *PagesHandler {
	return NewPagesHandler(nil, storage, roleLoader, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// registerPagesRoutes mounts the PagesHandler routes on a fresh chi router.
func registerPagesRoutes(h *PagesHandler) chi.Router {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// withPagesAdminContext attaches a system-admin auth context so role checks are
// skipped (SystemRole >= RoleSystemAdmin bypasses LoadCaseRole).
func withPagesAdminContext(r *http.Request) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "00000000-0000-4000-8000-000000000099",
		Username:   "admin",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

// withPagesUserContext attaches a regular-user auth context.
func withPagesUserContext(r *http.Request, userID string) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     userID,
		Username:   "regular-user",
		SystemRole: auth.RoleUser,
	})
	return r.WithContext(ctx)
}

// pagesStorageWithJSON returns a mockStorage (from service_test.go) pre-loaded
// with a JSON page-count blob at cacheKey.
func pagesStorageWithJSON(cacheKey string, count int) *mockStorage {
	s := newMockStorage()
	payload, _ := json.Marshal(map[string]int{"page_count": count})
	s.objects[cacheKey] = payload
	return s
}

// pagesStorageWithJPEG returns a mockStorage pre-loaded with a JPEG blob at cacheKey.
func pagesStorageWithJPEG(cacheKey string, data []byte) *mockStorage {
	s := newMockStorage()
	s.objects[cacheKey] = data
	return s
}

// ---------------------------------------------------------------------------
// NewPagesHandler / RegisterRoutes
// ---------------------------------------------------------------------------

func TestNewPagesHandler(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	if h == nil {
		t.Fatal("expected non-nil PagesHandler")
	}
}

func TestPagesHandler_RegisterRoutes(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

// ---------------------------------------------------------------------------
// parsePageDPI — pure function, no HTTP setup needed
// ---------------------------------------------------------------------------

func TestParsePageDPI_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	dpi, err := parsePageDPI(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dpi != defaultPageDPI {
		t.Errorf("dpi = %d, want %d", dpi, defaultPageDPI)
	}
}

func TestParsePageDPI_Valid(t *testing.T) {
	tests := []struct {
		raw  string
		want int
	}{
		{"72", minPageDPI},
		{"150", 150},
		{"300", maxPageDPI},
		{"200", 200},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?dpi="+tt.raw, nil)
			dpi, err := parsePageDPI(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dpi != tt.want {
				t.Errorf("dpi = %d, want %d", dpi, tt.want)
			}
		})
	}
}

func TestParsePageDPI_InvalidString(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?dpi=abc", nil)
	_, err := parsePageDPI(req)
	if err == nil {
		t.Fatal("expected error for non-numeric dpi")
	}
}

func TestParsePageDPI_TooLow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?dpi=1", nil)
	_, err := parsePageDPI(req)
	if err == nil {
		t.Fatal("expected error for dpi below minimum")
	}
}

func TestParsePageDPI_TooHigh(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?dpi=9999", nil)
	_, err := parsePageDPI(req)
	if err == nil {
		t.Fatal("expected error for dpi above maximum")
	}
}

// ---------------------------------------------------------------------------
// readCachedPageCount
// ---------------------------------------------------------------------------

func TestReadCachedPageCount_Hit(t *testing.T) {
	cacheKey := "page-cache/some-id/page_count.json"
	storage := pagesStorageWithJSON(cacheKey, 7)
	h := newPagesHandlerMock(storage, &mockDraftRoleLoader{})

	cached, count := h.readCachedPageCount(context.Background(), cacheKey)
	if !cached {
		t.Fatal("expected cache hit")
	}
	if count != 7 {
		t.Errorf("page_count = %d, want 7", count)
	}
}

func TestReadCachedPageCount_Miss(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	cached, count := h.readCachedPageCount(context.Background(), "missing-key")
	if cached {
		t.Fatal("expected cache miss")
	}
	if count != 0 {
		t.Errorf("count = %d on miss, want 0", count)
	}
}

func TestReadCachedPageCount_InvalidJSON(t *testing.T) {
	s := newMockStorage()
	s.objects["bad-key"] = []byte("not-json")
	h := newPagesHandlerMock(s, &mockDraftRoleLoader{})

	cached, count := h.readCachedPageCount(context.Background(), "bad-key")
	if cached {
		t.Fatal("expected false for invalid JSON")
	}
	if count != 0 {
		t.Errorf("count = %d on bad JSON, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// serveFromCache
// ---------------------------------------------------------------------------

func TestServeFromCache_Hit(t *testing.T) {
	cacheKey := "page-cache/abc/1_150.jpg"
	data := []byte("fake-jpeg-data")
	storage := pagesStorageWithJPEG(cacheKey, data)
	h := newPagesHandlerMock(storage, &mockDraftRoleLoader{})

	w := httptest.NewRecorder()
	served := h.serveFromCache(context.Background(), w, cacheKey)
	if !served {
		t.Fatal("expected serveFromCache to return true on cache hit")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", w.Header().Get("Content-Type"))
	}
	if !bytes.Equal(w.Body.Bytes(), data) {
		t.Error("body does not match stored data")
	}
}

func TestServeFromCache_Miss(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	w := httptest.NewRecorder()
	served := h.serveFromCache(context.Background(), w, "missing")
	if served {
		t.Fatal("expected serveFromCache to return false on cache miss")
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — unauthenticated
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_NoAuth(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	evidenceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	// No auth context
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — invalid UUID
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_InvalidID(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — unauthenticated
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_NoAuth(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	evidenceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// GetPage — invalid UUID
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_InvalidID(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — invalid page number
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_InvalidPageNum(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	evidenceID := uuid.New()
	tests := []struct {
		name    string
		pageNum string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/"+tt.pageNum, nil)
			req = withPagesAdminContext(req)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("pageNum=%q: status = %d, want %d; body: %s", tt.pageNum, w.Code, http.StatusBadRequest, w.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetPage — invalid DPI
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_InvalidDPI(t *testing.T) {
	h := newPagesHandlerMock(newMockStorage(), &mockDraftRoleLoader{})
	r := registerPagesRoutes(h)

	evidenceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1?dpi=9999", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// renderPDFPageAsJPEG — direct unit tests (reuses minimalPDF from redaction_test.go)
// ---------------------------------------------------------------------------

func TestRenderPDFPageAsJPEG_Success(t *testing.T) {
	// minimalPDF is defined in redaction_test.go (same package)
	result, err := renderPDFPageAsJPEG([]byte(minimalPDF), 1, defaultPageDPI)
	if err != nil {
		t.Fatalf("renderPDFPageAsJPEG returned unexpected error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty JPEG result")
	}
	// JPEG files start with the SOI marker FF D8
	if len(result) < 2 || result[0] != 0xFF || result[1] != 0xD8 {
		n := 4
		if len(result) < n {
			n = len(result)
		}
		t.Errorf("result does not look like JPEG (first bytes: % X)", result[:n])
	}
}

func TestRenderPDFPageAsJPEG_PageOutOfRange(t *testing.T) {
	// Page 99 does not exist in the single-page minimalPDF
	_, err := renderPDFPageAsJPEG([]byte(minimalPDF), 99, defaultPageDPI)
	if err == nil {
		t.Fatal("expected error for out-of-range page")
	}
}

func TestRenderPDFPageAsJPEG_InvalidPDF(t *testing.T) {
	_, err := renderPDFPageAsJPEG([]byte("not a pdf"), 1, defaultPageDPI)
	if err == nil {
		t.Fatal("expected error for invalid PDF data")
	}
}
