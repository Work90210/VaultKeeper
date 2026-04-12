package evidence

// Final gap-fillers for Sprint 9 handler.go methods. Pushes enforceItemAccess,
// loadCallerCaseRole, logClassifiedRead, and the error branches of
// Get/Download/GetThumbnail/GetVersionHistory/UpdateMetadata/Destroy to
// 100% plus the last missing branches in destruction.go, gdpr.go,
// gdpr_handler.go, and tag_handler.go.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---- destruction.go: checkLegalHold cases fallback (held=true) ----

func TestCheckLegalHold_CasesFallback_Held(t *testing.T) {
	svc := &Service{cases: &mockCaseLookup{legalHold: true}}
	err := svc.checkLegalHold(context.Background(), uuid.New())
	if !errors.Is(err, ErrLegalHoldActive) {
		t.Errorf("want ErrLegalHoldActive via cases fallback, got %v", err)
	}
}

// ---- handler.go: loadCallerCaseRole branches ----

func TestLoadCallerCaseRole_NoAuthContext(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	role, ok := h.loadCallerCaseRole(context.Background(), uuid.New())
	if ok {
		t.Errorf("want !ok, got ok=true role=%q", role)
	}
}

func TestLoadCallerCaseRole_SystemAdminBypass(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil) // no loader
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "admin",
		SystemRole: auth.RoleSystemAdmin,
	})
	role, ok := h.loadCallerCaseRole(ctx, uuid.New())
	if !ok || role != RoleJudge {
		t.Errorf("admin bypass should return judge, got role=%q ok=%v", role, ok)
	}
}

func TestLoadCallerCaseRole_NilLoaderNonAdmin(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user",
		SystemRole: auth.RoleAPIService,
	})
	_, ok := h.loadCallerCaseRole(ctx, uuid.New())
	if ok {
		t.Error("nil loader + non-admin must return !ok")
	}
}

func TestLoadCallerCaseRole_LoaderError(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("lookup failed")}
	h, _ := newTagHandlerTest(t, loader)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user",
		SystemRole: auth.RoleAPIService,
	})
	_, ok := h.loadCallerCaseRole(ctx, uuid.New())
	if ok {
		t.Error("loader error must return !ok")
	}
}

func TestLoadCallerCaseRole_LoaderSuccess(t *testing.T) {
	loader := &stubCaseRoleLoader{role: auth.CaseRoleDefence}
	h, _ := newTagHandlerTest(t, loader)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user",
		SystemRole: auth.RoleAPIService,
	})
	role, ok := h.loadCallerCaseRole(ctx, uuid.New())
	if !ok || role != string(auth.CaseRoleDefence) {
		t.Errorf("got role=%q ok=%v", role, ok)
	}
}

// ---- handler.go: enforceItemAccess branches ----

func TestEnforceItemAccess_SystemAdminFastPath(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "admin",
		SystemRole: auth.RoleSystemAdmin,
	})
	// Classified item — admin bypass should allow.
	item := EvidenceItem{ID: uuid.New(), CaseID: uuid.New(), Classification: ClassificationConfidential}
	if !h.enforceItemAccess(ctx, item) {
		t.Error("system admin must bypass matrix")
	}
}

func TestEnforceItemAccess_NoRoleDenies(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil) // no loader
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user",
		SystemRole: auth.RoleAPIService,
	})
	item := EvidenceItem{ID: uuid.New(), CaseID: uuid.New(), Classification: ClassificationRestricted}
	if h.enforceItemAccess(ctx, item) {
		t.Error("no case role must deny")
	}
}

func TestEnforceItemAccess_DefaultClassificationEmpty(t *testing.T) {
	loader := &stubCaseRoleLoader{role: auth.CaseRoleInvestigator}
	h, _ := newTagHandlerTest(t, loader)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user",
		SystemRole: auth.RoleAPIService,
	})
	// Empty classification → falls back to "restricted"
	item := EvidenceItem{ID: uuid.New(), CaseID: uuid.New(), Classification: ""}
	if !h.enforceItemAccess(ctx, item) {
		t.Error("empty classification should default to restricted and be visible to investigator")
	}
}

func TestEnforceItemAccess_DefenceDeniedOnConfidential(t *testing.T) {
	loader := &stubCaseRoleLoader{role: auth.CaseRoleDefence}
	h, _ := newTagHandlerTest(t, loader)
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "defence-user",
		SystemRole: auth.RoleAPIService,
	})
	item := EvidenceItem{ID: uuid.New(), CaseID: uuid.New(), Classification: ClassificationConfidential}
	if h.enforceItemAccess(ctx, item) {
		t.Error("defence must not see confidential")
	}
}

// ---- handler.go: logClassifiedRead ----

func TestLogClassifiedRead_NonClassifiedIsNoOp(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	item := EvidenceItem{ID: uuid.New(), CaseID: uuid.New(), Classification: ClassificationPublic}
	// Should not panic; should simply return early.
	h.logClassifiedRead(context.Background(), item, "investigator", "user-id")
}

func TestLogClassifiedRead_ExParteWithSide(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	side := ExPartePros
	item := EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Classification: ClassificationExParte,
		ExParteSide:    &side,
	}
	// Exercises the ex_parte branch + side dereference.
	h.logClassifiedRead(context.Background(), item, "prosecutor", "user-id")
}

func TestLogClassifiedRead_Confidential(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	item := EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Classification: ClassificationConfidential,
	}
	h.logClassifiedRead(context.Background(), item, "investigator", "user-id")
}

// ---- handler.go: respondServiceError mapping ----

type fakeRespWriter struct {
	code int
	body []byte
	hdr  http.Header
}

func (f *fakeRespWriter) Header() http.Header {
	if f.hdr == nil {
		f.hdr = http.Header{}
	}
	return f.hdr
}
func (f *fakeRespWriter) Write(b []byte) (int, error) { f.body = append(f.body, b...); return len(b), nil }
func (f *fakeRespWriter) WriteHeader(code int)        { f.code = code }

func TestRespondServiceError_ErrNotFound(t *testing.T) {
	w := &fakeRespWriter{}
	respondServiceError(w, ErrNotFound)
	if w.code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.code)
	}
}

func TestRespondServiceError_ErrLegalHoldActive(t *testing.T) {
	w := &fakeRespWriter{}
	respondServiceError(w, ErrLegalHoldActive)
	if w.code != http.StatusConflict {
		t.Errorf("code = %d, want 409", w.code)
	}
}

func TestRespondServiceError_ErrRetentionActive(t *testing.T) {
	w := &fakeRespWriter{}
	respondServiceError(w, ErrRetentionActive)
	if w.code != http.StatusConflict {
		t.Errorf("code = %d, want 409", w.code)
	}
}

func TestRespondServiceError_Generic(t *testing.T) {
	w := &fakeRespWriter{}
	respondServiceError(w, errors.New("boom"))
	if w.code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.code)
	}
}

func TestRespondServiceError_ValidationError(t *testing.T) {
	w := &fakeRespWriter{}
	respondServiceError(w, &ValidationError{Field: "x", Message: "bad"})
	if w.code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.code)
	}
}

// ---- handler.go: Get/Download/GetThumbnail/GetVersionHistory/Destroy access-deny paths ----

func newHandlerWithLoader(t *testing.T, loader auth.CaseRoleLoader) (*Handler, *mockRepo) {
	t.Helper()
	h, repo := newTestHandler(t)
	if loader != nil {
		h.SetCaseRoleLoader(loader)
	}
	return h, repo
}

// nonMemberAuth produces an auth context that is NOT system admin, so
// the handler falls back to the case-role loader (which is configured
// to deny in these tests).
func nonMemberAuth(r *http.Request) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "stranger",
		SystemRole: auth.RoleAPIService,
	})
	return r.WithContext(ctx)
}

func TestHandler_Get_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), Classification: ClassificationConfidential}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/", handler.Get) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String(), nil)
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (access denied, hidden)", w.Code)
	}
}

func TestHandler_Download_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	key := "k"
	repo.items[id] = EvidenceItem{
		ID:             id,
		CaseID:         uuid.New(),
		StorageKey:     &key,
		Classification: ClassificationConfidential,
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/download", handler.Download) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/download", nil)
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_GetThumbnail_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	thumb := "thumb/key"
	repo.items[id] = EvidenceItem{
		ID:             id,
		CaseID:         uuid.New(),
		ThumbnailKey:   &thumb,
		Classification: ClassificationExParte,
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/thumbnail", handler.GetThumbnail) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/thumbnail", nil)
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_GetVersionHistory_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), Classification: ClassificationConfidential}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/versions", handler.GetVersionHistory) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/versions", nil)
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_UpdateMetadata_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), Classification: ClassificationConfidential, Tags: []string{}}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Patch("/", handler.UpdateMetadata) })
	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+id.String(), strings.NewReader(`{"description":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_Destroy_AccessDenied_404(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	handler, repo := newHandlerWithLoader(t, loader)
	id := uuid.New()
	key := "k"
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), StorageKey: &key, Classification: ClassificationConfidential, Tags: []string{}}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/", handler.Destroy) })
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+id.String(), strings.NewReader(`{"authority":"Court Order 2026-001"}`))
	req.Header.Set("Content-Type", "application/json")
	req = nonMemberAuth(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---- handler.go: UpdateMetadata / Destroy NOT found error branches ----

func TestHandler_UpdateMetadata_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Patch("/", handler.UpdateMetadata) })
	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+uuid.New().String(), strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandler_Destroy_NotFound_DeepError(t *testing.T) {
	handler, _ := newTestHandler(t)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/", handler.Destroy) })
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+uuid.New().String(), strings.NewReader(`{"authority":"Court Order 2026-001"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
