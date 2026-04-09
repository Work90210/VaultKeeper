package disclosures

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

type mockRoleLoader struct {
	role auth.CaseRole
	err  error
}

func (m *mockRoleLoader) LoadCaseRole(_ context.Context, _, _ string) (auth.CaseRole, error) {
	return m.role, m.err
}

type mockAuditLogger struct{}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

type mockDisclosureRepo struct {
	createFn         func(ctx context.Context, d Disclosure) (Disclosure, error)
	findByIDFn       func(ctx context.Context, id uuid.UUID) (Disclosure, error)
	findByCaseFn     func(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error)
	evidenceBelongsFn func(ctx context.Context, caseID uuid.UUID, ids []uuid.UUID) (bool, error)
}

func (m *mockDisclosureRepo) Create(ctx context.Context, d Disclosure) (Disclosure, error) {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	d.ID = uuid.New()
	return d, nil
}

func (m *mockDisclosureRepo) FindByID(ctx context.Context, id uuid.UUID) (Disclosure, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return Disclosure{}, ErrNotFound
}

func (m *mockDisclosureRepo) FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error) {
	if m.findByCaseFn != nil {
		return m.findByCaseFn(ctx, caseID, page)
	}
	return nil, 0, nil
}

func (m *mockDisclosureRepo) EvidenceBelongsToCase(ctx context.Context, caseID uuid.UUID, ids []uuid.UUID) (bool, error) {
	if m.evidenceBelongsFn != nil {
		return m.evidenceBelongsFn(ctx, caseID, ids)
	}
	return true, nil
}

func newTestDisclosureHandler(repo Repository, role auth.CaseRole) *Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, nil, nil, logger)
	roleLoader := &mockRoleLoader{role: role}
	return NewHandler(svc, roleLoader, &mockAuditLogger{})
}

func withAuthCtx(r *http.Request, role auth.SystemRole) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     uuid.New().String(),
		Username:   "testuser",
		SystemRole: role,
	})
	return r.WithContext(ctx)
}

func TestDisclosureHandler_Create_Success(t *testing.T) {
	caseID := uuid.New()
	repo := &mockDisclosureRepo{
		createFn: func(_ context.Context, d Disclosure) (Disclosure, error) {
			d.ID = uuid.New()
			return d, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"evidence_ids": []string{uuid.New().String()},
		"disclosed_to": "defence",
		"notes":        "test disclosure",
	})

	req := httptest.NewRequest("POST", "/api/cases/"+caseID.String()+"/disclosures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201. body: %s", w.Code, w.Body.String())
	}
}

func TestDisclosureHandler_Create_NonProsecutor(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleDefence)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"evidence_ids": []string{uuid.New().String()},
		"disclosed_to": "defence",
	})

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/disclosures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", w.Code)
	}
}

func TestDisclosureHandler_Create_NoAuth(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestDisclosureHandler_Create_InvalidCaseID(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/invalid/disclosures", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestDisclosureHandler_List_Success(t *testing.T) {
	repo := &mockDisclosureRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Disclosure, int, error) {
			return []Disclosure{{ID: uuid.New(), EvidenceIDs: []uuid.UUID{}}}, 1, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestDisclosureHandler_List_NoAuth(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestDisclosureHandler_List_NoRole(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	h.roleLoader = &mockRoleLoader{err: auth.ErrNoCaseRole}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", w.Code)
	}
}

func TestDisclosureHandler_Get_Success(t *testing.T) {
	dID := uuid.New()
	caseID := uuid.New()
	repo := &mockDisclosureRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Disclosure, error) {
			return Disclosure{ID: dID, CaseID: caseID, EvidenceIDs: []uuid.UUID{uuid.New()}}, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/"+dID.String(), nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestDisclosureHandler_Get_NotFound(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/"+uuid.New().String(), nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestDisclosureHandler_Get_InvalidID(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/invalid", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestDisclosureHandler_Get_NoAuth(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Additional handler tests – gaps identified by coverage analysis
// ---------------------------------------------------------------------------

// TestDisclosureHandler_Create_InvalidBody verifies that a malformed JSON body
// results in a 400 response from the Create endpoint.
func TestDisclosureHandler_Create_InvalidBody(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Send a string that cannot be decoded into the expected struct.
	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/disclosures",
		bytes.NewBufferString("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_Create_SystemAdmin verifies that a system admin user
// (whose getCaseRole short-circuits to prosecutor) can create a disclosure.
func TestDisclosureHandler_Create_SystemAdmin(t *testing.T) {
	caseID := uuid.New()
	repo := &mockDisclosureRepo{
		createFn: func(_ context.Context, d Disclosure) (Disclosure, error) {
			d.ID = uuid.New()
			return d, nil
		},
	}
	// Role loader would return an error for a regular user, but system admin
	// bypasses it entirely.
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	h.roleLoader = &mockRoleLoader{err: errors.New("should not be called")}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"evidence_ids": []string{uuid.New().String()},
		"disclosed_to": "defence",
		"notes":        "admin disclosure",
	})

	req := httptest.NewRequest("POST", "/api/cases/"+caseID.String()+"/disclosures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Use RoleSystemAdmin so getCaseRole returns prosecutor without calling roleLoader.
	req = withAuthCtx(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_List_InvalidCaseID verifies that a non-UUID case ID in
// the URL returns 400.
func TestDisclosureHandler_List_InvalidCaseID(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/not-a-uuid/disclosures", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_Get_RoleError verifies that when the role loader fails
// after a successful disclosure fetch, the handler returns 403.
func TestDisclosureHandler_Get_RoleError(t *testing.T) {
	dID := uuid.New()
	caseID := uuid.New()
	repo := &mockDisclosureRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Disclosure, error) {
			return Disclosure{ID: dID, CaseID: caseID, EvidenceIDs: []uuid.UUID{}}, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	// Override loader to return a role error after disclosure is found.
	h.roleLoader = &mockRoleLoader{err: auth.ErrNoCaseRole}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/"+dID.String(), nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_Get_SystemAdmin verifies that a system admin can view
// any disclosure regardless of case role.
func TestDisclosureHandler_Get_SystemAdmin(t *testing.T) {
	dID := uuid.New()
	caseID := uuid.New()
	repo := &mockDisclosureRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Disclosure, error) {
			return Disclosure{ID: dID, CaseID: caseID, EvidenceIDs: []uuid.UUID{}}, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	// Loader would fail for a regular user; system admin bypasses it.
	h.roleLoader = &mockRoleLoader{err: errors.New("should not be called")}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/disclosures/"+dID.String(), nil)
	req = withAuthCtx(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_List_SystemAdmin verifies that a system admin can list
// disclosures on any case without a case role.
func TestDisclosureHandler_List_SystemAdmin(t *testing.T) {
	repo := &mockDisclosureRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Disclosure, int, error) {
			return []Disclosure{{ID: uuid.New(), EvidenceIDs: []uuid.UUID{}}}, 1, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	h.roleLoader = &mockRoleLoader{err: errors.New("should not be called")}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	req = withAuthCtx(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_List_Pagination verifies the pagination limit query
// parameter is applied (exercises parsePagination with a custom limit).
func TestDisclosureHandler_List_Pagination(t *testing.T) {
	repo := &mockDisclosureRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, page Pagination) ([]Disclosure, int, error) {
			if page.Limit != 5 {
				return nil, 0, nil
			}
			items := make([]Disclosure, 5)
			for i := range items {
				items[i] = Disclosure{ID: uuid.New(), EvidenceIDs: []uuid.UUID{}}
			}
			return items, 5, nil
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures?limit=5", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestRespondServiceError_Disclosure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{"validation", &ValidationError{Field: "f", Message: "m"}, http.StatusBadRequest},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"internal", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondServiceError(w, tt.err)
			if w.Code != tt.code {
				t.Errorf("got %d, want %d", w.Code, tt.code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Remaining handler gap tests
// ---------------------------------------------------------------------------

// TestDisclosureHandler_Create_GetCaseRoleError covers the path where the
// role loader returns an error for a non-admin user making a Create request.
func TestDisclosureHandler_Create_GetCaseRoleError(t *testing.T) {
	h := newTestDisclosureHandler(&mockDisclosureRepo{}, auth.CaseRoleProsecutor)
	h.roleLoader = &mockRoleLoader{err: auth.ErrNoCaseRole}
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"evidence_ids": []string{uuid.New().String()},
		"disclosed_to": "defence",
	})

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/disclosures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_Create_ServiceError covers the path where
// service.Create returns an error (e.g. repo failure).
func TestDisclosureHandler_Create_ServiceError(t *testing.T) {
	repo := &mockDisclosureRepo{
		createFn: func(_ context.Context, _ Disclosure) (Disclosure, error) {
			return Disclosure{}, errors.New("db failure")
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{
		"evidence_ids": []string{uuid.New().String()},
		"disclosed_to": "defence",
	})

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/disclosures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

// TestDisclosureHandler_List_ServiceError covers the path where
// service.List returns an error.
func TestDisclosureHandler_List_ServiceError(t *testing.T) {
	repo := &mockDisclosureRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Disclosure, int, error) {
			return nil, 0, errors.New("db timeout")
		},
	}
	h := newTestDisclosureHandler(repo, auth.CaseRoleProsecutor)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/disclosures", nil)
	req = withAuthCtx(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

// TestDecodeBody_BodyTooLarge covers the io.ErrUnexpectedEOF / "unexpected end"
// branch in decodeBody by sending a body that exceeds MaxBodySize.
func TestDecodeBody_BodyTooLarge(t *testing.T) {
	// Build a JSON string larger than MaxBodySize. The io.LimitReader will
	// cut the stream mid-read, producing an io.ErrUnexpectedEOF when the
	// JSON decoder tries to read the truncated object.
	huge := `{"evidence_ids":["` + strings.Repeat("a", MaxBodySize) + `"]}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(huge))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		EvidenceIDs []string `json:"evidence_ids"`
	}
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "body" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "body")
	}
}
