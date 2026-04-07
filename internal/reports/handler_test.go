package reports

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// mockAuditLogger satisfies auth.AuditLogger.
type mockAuditLogger struct{}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

// --- Handler test helpers ---

func newTestHandler(custMock *mockCustodySource, evMock *mockEvidenceSource, caseMock *mockCaseSource) *Handler {
	gen := NewCustodyReportGenerator(custMock, evMock, caseMock)
	return NewHandler(gen, &mockAuditLogger{})
}

func reqWithChiParamAndAuth(method, path string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.WithAuthContext(ctx, auth.AuthContext{
		UserID:     "user-1",
		SystemRole: auth.RoleCaseAdmin,
	})
	return r.WithContext(ctx)
}

func reqWithChiParam(method, path string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// --- Tests for ExportCaseCustody ---

func TestExportCaseCustody_Success(t *testing.T) {
	caseRec := sampleCaseRecord()
	item := sampleEvidenceItem()
	events := buildValidChain(2)

	h := newTestHandler(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: []evidence.EvidenceItem{item}, findByCaseTotal: 1},
		&mockCaseSource{caseRecord: caseRec},
	)

	caseID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/cases/"+caseID.String()+"/custody/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCaseCustody(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, caseRec.ReferenceCode) {
		t.Errorf("Content-Disposition should contain reference code %q, got %q", caseRec.ReferenceCode, cd)
	}
	if !strings.Contains(cd, ".pdf") {
		t.Errorf("Content-Disposition should contain .pdf, got %q", cd)
	}
	if cl := w.Header().Get("Content-Length"); cl == "" || cl == "0" {
		t.Error("expected non-zero Content-Length")
	}
}

func TestExportCaseCustody_InvalidUUID(t *testing.T) {
	h := newTestHandler(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	r := reqWithChiParamAndAuth("GET", "/api/cases/not-a-uuid/custody/export", map[string]string{"id": "not-a-uuid"})
	w := httptest.NewRecorder()

	h.ExportCaseCustody(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestExportCaseCustody_RefCodeError(t *testing.T) {
	h := newTestHandler(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{err: fmt.Errorf("case not found")},
	)

	caseID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/cases/"+caseID.String()+"/custody/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCaseCustody(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestExportCaseCustody_GeneratorError(t *testing.T) {
	// Case repo succeeds for GetReferenceCode but custody repo fails for GenerateCustodyPDF
	h := newTestHandler(
		&mockCustodySource{allByCaseErr: fmt.Errorf("custody db error")},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	caseID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/cases/"+caseID.String()+"/custody/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCaseCustody(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Tests for ExportEvidenceCustody ---

func TestExportEvidenceCustody_Success(t *testing.T) {
	item := sampleEvidenceItem()
	caseRec := sampleCaseRecord()
	events := buildValidChain(1)

	h := newTestHandler(
		&mockCustodySource{byEvidenceEvents: events, byEvidenceTotal: 1},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: caseRec},
	)

	evID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/evidence/"+evID.String()+"/custody/export", map[string]string{"id": evID.String()})
	w := httptest.NewRecorder()

	h.ExportEvidenceCustody(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, ".pdf") {
		t.Errorf("Content-Disposition should contain .pdf, got %q", cd)
	}
}

func TestExportEvidenceCustody_InvalidUUID(t *testing.T) {
	h := newTestHandler(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	r := reqWithChiParamAndAuth("GET", "/api/evidence/bad-id/custody/export", map[string]string{"id": "bad-id"})
	w := httptest.NewRecorder()

	h.ExportEvidenceCustody(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestExportEvidenceCustody_GeneratorError(t *testing.T) {
	h := newTestHandler(
		&mockCustodySource{},
		&mockEvidenceSource{findByIDErr: fmt.Errorf("not found")},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	evID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/evidence/"+evID.String()+"/custody/export", map[string]string{"id": evID.String()})
	w := httptest.NewRecorder()

	h.ExportEvidenceCustody(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- Tests for w.Write error paths ---

// failWriter is an http.ResponseWriter whose Write always returns an error.
type failWriter struct {
	header http.Header
	code   int
}

func newFailWriter() *failWriter {
	return &failWriter{header: http.Header{}}
}

func (fw *failWriter) Header() http.Header         { return fw.header }
func (fw *failWriter) WriteHeader(code int)         { fw.code = code }
func (fw *failWriter) Write(_ []byte) (int, error)  { return 0, fmt.Errorf("write failed") }

func TestExportCaseCustody_WriteError(t *testing.T) {
	caseRec := sampleCaseRecord()
	item := sampleEvidenceItem()
	events := buildValidChain(2)

	h := newTestHandler(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: []evidence.EvidenceItem{item}, findByCaseTotal: 1},
		&mockCaseSource{caseRecord: caseRec},
	)

	caseID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/cases/"+caseID.String()+"/custody/export", map[string]string{"id": caseID.String()})
	w := newFailWriter()

	// Should not panic; the write error is logged but doesn't change the response.
	h.ExportCaseCustody(w, r)
}

func TestExportEvidenceCustody_WriteError(t *testing.T) {
	item := sampleEvidenceItem()
	caseRec := sampleCaseRecord()
	events := buildValidChain(1)

	h := newTestHandler(
		&mockCustodySource{byEvidenceEvents: events, byEvidenceTotal: 1},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: caseRec},
	)

	evID := uuid.New()
	r := reqWithChiParamAndAuth("GET", "/api/evidence/"+evID.String()+"/custody/export", map[string]string{"id": evID.String()})
	w := newFailWriter()

	// Should not panic; the write error is logged but doesn't change the response.
	h.ExportEvidenceCustody(w, r)
}

// --- Tests for RegisterRoutes ---

func TestRegisterRoutes(t *testing.T) {
	h := newTestHandler(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Verify routes are registered by walking the router
	routeFound := false
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == "GET" && (strings.Contains(route, "/custody/export") || strings.Contains(route, "export")) {
			routeFound = true
		}
		return nil
	})

	if !routeFound {
		t.Error("expected custody export route to be registered")
	}
}
