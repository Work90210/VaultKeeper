package cases

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

// mockAuditLoggerExport satisfies auth.AuditLogger for export handler tests.
type mockAuditLoggerExport struct{}

func (m *mockAuditLoggerExport) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

// --- Handler test helpers ---

func newExportHandler(
	evMock *mockEvidenceExporter,
	custMock *mockCustodyExporter,
	caseMock *mockCaseExporter,
	dlMock *mockFileDownloader,
	logMock *mockExportCustodyLogger,
) *ExportHandler {
	svc := NewExportService(evMock, custMock, caseMock, dlMock, logMock)
	return NewExportHandler(svc, &mockAuditLoggerExport{})
}

func exportReqWithAuth(method, path string, params map[string]string) *http.Request {
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
	ctx = auth.WithCaseRole(ctx, auth.CaseRoleInvestigator)
	return r.WithContext(ctx)
}

func exportReqNoAuth(method, path string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// --- Tests for ExportCase handler ---

func TestExportHandler_ExportCase_Success(t *testing.T) {
	c := sampleCase()

	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	r := exportReqWithAuth("GET", "/api/cases/"+caseID.String()+"/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCase(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, c.ReferenceCode) {
		t.Errorf("Content-Disposition should contain reference code %q, got %q", c.ReferenceCode, cd)
	}
	if !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition should contain .zip, got %q", cd)
	}
}

func TestExportHandler_ExportCase_InvalidUUID(t *testing.T) {
	c := sampleCase()
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	r := exportReqWithAuth("GET", "/api/cases/not-a-uuid/export", map[string]string{"id": "not-a-uuid"})
	w := httptest.NewRecorder()

	h.ExportCase(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestExportHandler_ExportCase_NoAuthContext(t *testing.T) {
	c := sampleCase()
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	r := exportReqNoAuth("GET", "/api/cases/"+caseID.String()+"/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCase(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for missing auth context, got %d", w.Code)
	}
}

func TestExportHandler_ExportCase_RefCodeError(t *testing.T) {
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{err: fmt.Errorf("case not found")},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	r := exportReqWithAuth("GET", "/api/cases/"+caseID.String()+"/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCase(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestExportHandler_ExportCase_WithCaseRole(t *testing.T) {
	c := sampleCase()
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	r := exportReqWithAuth("GET", "/api/cases/"+caseID.String()+"/export", map[string]string{"id": caseID.String()})
	// Add a case role to context
	ctx := auth.WithCaseRole(r.Context(), auth.CaseRoleDefence)
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ExportCase(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportHandler_ExportCase_NoCaseRole(t *testing.T) {
	c := sampleCase()
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	// Build request with auth but without case role
	r := httptest.NewRequest("GET", "/api/cases/"+caseID.String()+"/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.WithAuthContext(ctx, auth.AuthContext{
		UserID:     "user-1",
		SystemRole: auth.RoleCaseAdmin,
	})
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ExportCase(w, r)

	// Should still succeed (case role is optional, defaults to empty string)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Test ExportCase handler when export itself fails after headers sent ---

func TestExportHandler_ExportCase_ExportFails(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	// Evidence repo succeeds but file download fails — triggers error after headers sent.
	h := newExportHandler(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{err: fmt.Errorf("s3 failure")},
		&mockExportCustodyLogger{},
	)

	caseID := uuid.New()
	r := exportReqWithAuth("GET", "/api/cases/"+caseID.String()+"/export", map[string]string{"id": caseID.String()})
	w := httptest.NewRecorder()

	h.ExportCase(w, r)

	// Headers already sent, so status is 200 even though export failed mid-stream.
	if w.Code != http.StatusOK {
		t.Logf("status = %d (expected 200 since headers already sent)", w.Code)
	}
}

// --- Test RegisterRoutes ---

func TestExportHandler_RegisterRoutes(t *testing.T) {
	c := sampleCase()
	h := newExportHandler(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	routeFound := false
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == "GET" && strings.Contains(route, "/export") {
			routeFound = true
		}
		return nil
	})

	if !routeFound {
		t.Error("expected export route to be registered")
	}
}
