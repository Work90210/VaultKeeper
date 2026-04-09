package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
// GetPageCount — evidence not found (requires real DB via integration test)
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_NotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	storage := newMockStorage()
	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	evidenceID := uuid.New() // Does not exist in DB
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — cache hit (admin user, no DB needed)
// We seed the cache and set up a mockStorage for the lookupEvidenceInfo row.
// Since lookupEvidenceInfo uses db.QueryRow which requires a real pool,
// we test this path via the integration route below.
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_CacheHit(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-CACHE-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/doc.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-001", "doc.pdf", "doc.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("a", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	// Pre-populate cache
	cacheKey := fmt.Sprintf("page-cache/%s/page_count.json", evidenceID)
	storage := pagesStorageWithJSON(cacheKey, 5)
	// Also put the PDF so lookupEvidenceInfo can find it (though cache wins first)
	storage.objects[storageKey] = []byte("placeholder")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Data struct {
			PageCount int `json:"page_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.PageCount != 5 {
		t.Errorf("page_count = %d, want 5", resp.Data.PageCount)
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — storage error after evidence found
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_StorageError(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-STORERR-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/doc2.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-002", "doc2.pdf", "doc2.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("b", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	// Storage returns error on GetObject (cache miss because key is not present, then PDF fetch fails)
	storage := newMockStorage()
	storage.getErr = fmt.Errorf("storage unavailable")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — role check: regular user, no case role → 403
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_ForbiddenUser(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-FORBID-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/doc3.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-003", "doc3.pdf", "doc3.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("c", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	// Role loader returns ErrNoCaseRole
	roleLoader := &mockDraftRoleLoader{err: auth.ErrNoCaseRole}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	userID := "00000000-0000-4000-8000-000000000088"
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesUserContext(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — role check: role loader internal error → 500
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_RoleLoaderError(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-ROLEERR-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/doc4.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-004", "doc4.pdf", "doc4.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("d", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	// Role loader returns an unexpected internal error
	roleLoader := &mockDraftRoleLoader{err: fmt.Errorf("role service down")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	userID := "00000000-0000-4000-8000-000000000087"
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesUserContext(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
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
// GetPage — evidence not found (DB row missing)
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_NotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	storage := newMockStorage()
	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	evidenceID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — role check: forbidden user
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_ForbiddenUser(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PFORBID-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/page-forbidden.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-P001", "page-forbidden.pdf", "page-forbidden.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("e", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	roleLoader := &mockDraftRoleLoader{err: auth.ErrNoCaseRole}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	userID := "00000000-0000-4000-8000-000000000086"
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesUserContext(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — role check: role loader internal error
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_RoleLoaderError(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PROLEERR-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/page-roleerr.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-P002", "page-roleerr.pdf", "page-roleerr.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("f", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	roleLoader := &mockDraftRoleLoader{err: fmt.Errorf("internal role error")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	userID := "00000000-0000-4000-8000-000000000085"
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesUserContext(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — page cache hit (serves from cache, skips PDF render)
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_CacheHit(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PCACHE-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/page-cached.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-P003", "page-cached.pdf", "page-cached.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("a", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	dpi := 150
	pageNum := 1
	cacheKey := fmt.Sprintf("page-cache/%s/%d_%d.jpg", evidenceID, pageNum, dpi)
	fakeJPEG := []byte("JFIF-fake-jpeg-bytes")

	storage := newMockStorage()
	storage.objects[cacheKey] = fakeJPEG
	storage.objects[storageKey] = []byte("placeholder-pdf")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/evidence/%s/pages/%d", evidenceID, pageNum), nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", w.Header().Get("Content-Type"))
	}
	if !bytes.Equal(w.Body.Bytes(), fakeJPEG) {
		t.Error("response body does not match cached JPEG data")
	}
}

// ---------------------------------------------------------------------------
// GetPage — storage error when fetching PDF for rendering
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_StorageError(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PSTORE-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/page-storerr.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-P004", "page-storerr.pdf", "page-storerr.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("b", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	storage.getErr = fmt.Errorf("storage down")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/evidence/%s/pages/1", evidenceID), nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPage — invalid PDF data → render failure → 404
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_InvalidPDF(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PINVPDF-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/page-invalid.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-P005", "page-invalid.pdf", "page-invalid.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("c", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	// Store non-PDF data so that fitz.NewFromMemory fails
	storage := newMockStorage()
	storage.objects[storageKey] = []byte("this is not a pdf file at all")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/evidence/%s/pages/1", evidenceID), nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetPageCount — invalid PDF data → 500
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_InvalidPDF(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-CINVPDF-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/count-invalid.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-C001", "count-invalid.pdf", "count-invalid.pdf", storageKey,
		"application/pdf", 1024, strings.Repeat("d", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	// Store non-PDF data so that fitz.NewFromMemory fails
	storage := newMockStorage()
	storage.objects[storageKey] = []byte("not a pdf")

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// lookupEvidenceInfo — nil storage_key path
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_NilStorageKey(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-NILKEY-001")
	evidenceID := uuid.New()

	// Insert evidence with NULL storage_key (all non-null columns must be provided)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,NULL,$6,$7,$8,$9,$10,$11,$12)`,
		evidenceID, caseID, "EV-PG-NK001", "nilkey.pdf", "nilkey.pdf",
		"application/pdf", 1024, strings.Repeat("e", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence with null storage_key: %v", err)
	}

	storage := newMockStorage()
	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 500 because lookupEvidenceInfo returns an error for nil storage_key
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
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

// ---------------------------------------------------------------------------
// GetPageCount — success path with a real PDF (no cache, fetches from storage)
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPageCount_Success(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-SUCCESS-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/real-doc.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-S001", "real-doc.pdf", "real-doc.pdf", storageKey,
		"application/pdf", len(minimalPDF), strings.Repeat("f", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Data struct {
			PageCount int `json:"page_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.PageCount < 1 {
		t.Errorf("page_count = %d, want >= 1", resp.Data.PageCount)
	}
}

// ---------------------------------------------------------------------------
// GetPage — success path with a real PDF
// ---------------------------------------------------------------------------

func TestPagesHandler_GetPage_Success(t *testing.T) {
	pool := startPostgresContainer(t)

	caseID := seedCase(t, pool, "CR-PG-PSUCC-001")
	evidenceID := uuid.New()
	storageKey := "evidence/test/real-page.pdf"

	_, err := pool.Exec(context.Background(),
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-PG-PS001", "real-page.pdf", "real-page.pdf", storageKey,
		"application/pdf", len(minimalPDF), strings.Repeat("g", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}

	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)

	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewPagesHandler(pool, storage, roleLoader, logger)
	r := registerPagesRoutes(h)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/evidence/%s/pages/1", evidenceID), nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", w.Header().Get("Content-Type"))
	}
	if len(w.Body.Bytes()) == 0 {
		t.Error("expected non-empty JPEG body")
	}
}

