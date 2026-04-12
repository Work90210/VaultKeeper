//go:build integration

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

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

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
