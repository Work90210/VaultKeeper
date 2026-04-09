package witnesses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
func (m *mockAuditLogger) LogAuthEvent(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func newTestHandler(repo Repository, role auth.CaseRole) *Handler {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)
	roleLoader := &mockRoleLoader{role: role}
	return NewHandler(svc, roleLoader, &mockAuditLogger{})
}

func withAuth(r *http.Request, role auth.SystemRole) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     uuid.New().String(),
		Username:   "testuser",
		SystemRole: role,
	})
	return r.WithContext(ctx)
}

func TestHandler_Create_Success(t *testing.T) {
	caseID := uuid.New()
	repo := &mockWitnessRepo{
		createFn: func(_ context.Context, w Witness) (Witness, error) {
			w.ID = uuid.New()
			return w, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)

	body, _ := json.Marshal(map[string]any{
		"witness_code":      "W-001",
		"protection_status": "standard",
	})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/"+caseID.String()+"/witnesses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d. body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandler_Create_NoAuth(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestHandler_Create_InvalidCaseID(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/invalid/witnesses", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestHandler_Create_ServiceError(t *testing.T) {
	caseID := uuid.New()
	repo := &mockWitnessRepo{
		createFn: func(_ context.Context, _ Witness) (Witness, error) {
			return Witness{}, errors.New("db error")
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)

	body, _ := json.Marshal(map[string]any{
		"witness_code":      "W-001",
		"protection_status": "standard",
	})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/"+caseID.String()+"/witnesses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Create_WrongRole(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleDefence)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{"witness_code": "W", "protection_status": "standard"})
	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/witnesses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", w.Code)
	}
}

func TestHandler_List_Success(t *testing.T) {
	repo := &mockWitnessRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
			return []Witness{{WitnessCode: "W-001", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}}, 1, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleDefence)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_Success(t *testing.T) {
	wID := uuid.New()
	caseID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: caseID, WitnessCode: "W-001", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+wID.String(), nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+uuid.New().String(), nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

func TestHandler_Get_InvalidID(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/invalid", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestHandler_Update_Success(t *testing.T) {
	wID := uuid.New()
	caseID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: caseID, WitnessCode: "W-001", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, w Witness) (Witness, error) {
			return w, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{"protection_status": "protected"})
	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_WrongRole(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleObserver)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{"protection_status": "protected"})
	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", w.Code)
	}
}

func TestRespondServiceError(t *testing.T) {
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

func TestDecodeBody(t *testing.T) {
	body := `{"witness_code": "W-001"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	var dst struct{ WitnessCode string `json:"witness_code"` }
	err := decodeBody(req, &dst)
	if err != nil {
		t.Fatalf("decodeBody: %v", err)
	}
	if dst.WitnessCode != "W-001" {
		t.Errorf("got %q, want W-001", dst.WitnessCode)
	}
}

func TestDecodeBody_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("{invalid"))
	var dst struct{}
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeBody_BodyTooLarge(t *testing.T) {
	// Generate a body that exceeds MaxBodySize so the LimitReader triggers
	// an unexpected-EOF condition (the JSON stream is cut mid-token).
	// We write MaxBodySize+2 bytes of a JSON string value so the decoder
	// hits the limit and returns the "unexpected end" branch.
	huge := make([]byte, MaxBodySize+2)
	huge[0] = '"'
	for i := 1; i < len(huge)-1; i++ {
		huge[i] = 'a'
	}
	// Do NOT close the JSON string — let the limit reader truncate it.
	req := httptest.NewRequest("POST", "/", bytes.NewReader(huge))
	var dst struct{ V string `json:"v"` }
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "body" {
		t.Errorf("expected field=body, got %q", ve.Field)
	}
}

func TestParsePagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=25&cursor=abc", nil)
	p := parsePagination(req)
	if p.Limit != 25 {
		t.Errorf("limit: got %d, want 25", p.Limit)
	}
	if p.Cursor != "abc" {
		t.Errorf("cursor: got %q, want abc", p.Cursor)
	}
}

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	p := parsePagination(req)
	if p.Limit != DefaultPageLimit {
		t.Errorf("limit: got %d, want %d", p.Limit, DefaultPageLimit)
	}
}

func TestGetCaseRole_SystemAdmin(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleDefence)
	role, err := h.getCaseRole(context.Background(), uuid.New().String(), uuid.New().String(), auth.RoleSystemAdmin)
	if err != nil {
		t.Fatalf("getCaseRole: %v", err)
	}
	if role != auth.CaseRoleInvestigator {
		t.Errorf("got %q, want investigator", role)
	}
}

func TestGetCaseRole_NormalUser(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleProsecutor)
	role, err := h.getCaseRole(context.Background(), uuid.New().String(), uuid.New().String(), auth.RoleUser)
	if err != nil {
		t.Fatalf("getCaseRole: %v", err)
	}
	if role != auth.CaseRoleProsecutor {
		t.Errorf("got %q, want prosecutor", role)
	}
}

// ---------------------------------------------------------------------------
// List handler — additional coverage
// ---------------------------------------------------------------------------

func TestHandler_List_NoAuth(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	// No auth context.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestHandler_List_InvalidCaseID(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/invalid/witnesses", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestHandler_List_ServiceError(t *testing.T) {
	repo := &mockWitnessRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
			return nil, 0, errors.New("db failure")
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_List_NextCursorSet(t *testing.T) {
	// When the result count equals the page limit, a next cursor should be
	// included in the response.
	id := uuid.New()
	repo := &mockWitnessRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
			// Return exactly DefaultPageLimit witnesses so nextCursor is set.
			items := make([]Witness, DefaultPageLimit)
			for i := range items {
				items[i] = Witness{ID: id, WitnessCode: "W", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}
			}
			return items, DefaultPageLimit * 2, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Get handler — additional coverage
// ---------------------------------------------------------------------------

func TestHandler_Get_NoAuth(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+uuid.New().String(), nil)
	// No auth context.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestHandler_Get_InternalErrorFromGetCaseID(t *testing.T) {
	// FindByID returns a non-ErrNotFound error → should be 500.
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{}, errors.New("unexpected db error")
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+uuid.New().String(), nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_RoleError(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New()}, nil
		},
	}

	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)
	roleLoader := &mockRoleLoader{err: errors.New("no role")}
	h := NewHandler(svc, roleLoader, &mockAuditLogger{})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+wID.String(), nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Get_ServiceError(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, id uuid.UUID) (Witness, error) {
			// First call returns the case ID (from GetCaseID); second call (from Get)
			// returns an error. We simulate this by returning success on first call
			// and error on second.
			// In the actual flow, GetCaseID and Get both call FindByID.
			// We use a call counter to differentiate.
			return Witness{}, errors.New("db failure")
		},
	}
	// The first FindByID is called by GetCaseID; it must succeed to reach service.Get.
	callCount := 0
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (Witness, error) {
		callCount++
		if callCount == 1 {
			return Witness{ID: wID, CaseID: uuid.New()}, nil
		}
		return Witness{}, errors.New("second db failure")
	}

	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/witnesses/"+wID.String(), nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Update handler — additional coverage
// ---------------------------------------------------------------------------

func TestHandler_Update_NoAuth(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("PATCH", "/api/witnesses/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
}

func TestHandler_Update_InvalidID(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("PATCH", "/api/witnesses/invalid", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}

func TestHandler_Update_InternalErrorFromGetCaseID(t *testing.T) {
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{}, errors.New("unexpected db error")
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("PATCH", "/api/witnesses/"+uuid.New().String(), nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_RoleError(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New()}, nil
		},
	}

	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)
	roleLoader := &mockRoleLoader{err: errors.New("no role")}
	h := NewHandler(svc, roleLoader, &mockAuditLogger{})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_InvalidJSON(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New()}, nil
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Update_ServiceError(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ Witness) (Witness, error) {
			return Witness{}, errors.New("db update failed")
		},
	}
	h := newTestHandler(repo, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// RotateKeys handler tests
// ---------------------------------------------------------------------------

func newTestHandlerWithJob(repo Repository, role auth.CaseRole, job *KeyRotationJob) *Handler {
	h := newTestHandler(repo, role)
	h.SetRotationJob(job)
	return h
}

func makeRotationJob() *KeyRotationJob {
	oldEnc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	newKey := make([]byte, 32)
	newKey[0] = 2
	newEnc, _ := NewEncryptor(EncryptionKey{Version: 2, Key: newKey})
	repo := &mockWitnessRepo{}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	return NewKeyRotationJob(repo, oldEnc, newEnc, custody, logger)
}

func TestHandler_RotateKeys_SystemAdmin_Accepted(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/admin/witness-keys/rotate", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status: got %d, want 202. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeys_NonAdmin_Forbidden(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/admin/witness-keys/rotate", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeys_NoJob_ServiceUnavailable(t *testing.T) {
	// Handler with no rotation job configured (nil).
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/admin/witness-keys/rotate", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeys_AlreadyRunning_Conflict(t *testing.T) {
	oldEnc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	newKey := make([]byte, 32)
	newKey[0] = 2
	newEnc, _ := NewEncryptor(EncryptionKey{Version: 2, Key: newKey})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))

	// Build one witness that needs rotation so the job has work to do.
	id := uuid.New()
	fullNameEnc, _ := oldEnc.EncryptField(nil, id.String(), "full_name")
	w := Witness{
		ID:                id,
		CaseID:            uuid.New(),
		WitnessCode:       "W-BLOCK",
		FullNameEncrypted: fullNameEnc,
		ProtectionStatus:  "standard",
	}

	// blockingUpdateRepo returns witnesses immediately but stalls UpdateEncryptedFields
	// until the done channel is closed, keeping the job in Running=true state.
	doneCh := make(chan struct{})
	readyCh := make(chan struct{}) // signals that the job is inside UpdateEncryptedFields

	blockingUpdate := &blockingUpdateRepo{
		witnesses: []Witness{w},
		doneCh:    doneCh,
		readyCh:   readyCh,
	}

	job := NewKeyRotationJob(blockingUpdate, oldEnc, newEnc, nil, logger)

	go func() {
		_ = job.Run(context.Background())
	}()

	// Wait until the job is blocked in UpdateEncryptedFields (meaning Running=true).
	select {
	case <-readyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for job to start")
	}

	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/admin/witness-keys/rotate", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)

	// Unblock the background job now that we have captured the response.
	close(doneCh)

	if w2.Code != http.StatusConflict {
		t.Errorf("status: got %d, want 409. body: %s", w2.Code, w2.Body.String())
	}
}

// blockingUpdateRepo returns witnesses immediately but blocks UpdateEncryptedFields
// until doneCh is closed. It signals readyCh the first time it enters the block.
type blockingUpdateRepo struct {
	witnesses []Witness
	doneCh    chan struct{}
	readyCh   chan struct{}
	signalled bool
}

func (b *blockingUpdateRepo) FindAll(_ context.Context) ([]Witness, error) {
	return b.witnesses, nil
}

func (b *blockingUpdateRepo) UpdateEncryptedFields(_ context.Context, _ uuid.UUID, _, _, _ []byte) error {
	if !b.signalled {
		b.signalled = true
		close(b.readyCh)
	}
	<-b.doneCh
	return nil
}

func (b *blockingUpdateRepo) Create(_ context.Context, w Witness) (Witness, error) { return w, nil }
func (b *blockingUpdateRepo) FindByID(_ context.Context, _ uuid.UUID) (Witness, error) {
	return Witness{}, ErrNotFound
}
func (b *blockingUpdateRepo) FindByCase(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
	return nil, 0, nil
}
func (b *blockingUpdateRepo) Update(_ context.Context, _ uuid.UUID, w Witness) (Witness, error) {
	return w, nil
}


// ---------------------------------------------------------------------------
// RotateKeysProgress handler tests
// ---------------------------------------------------------------------------

func TestHandler_RotateKeysProgress_SystemAdmin_OK(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/admin/witness-keys/rotate/progress", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeysProgress_NonAdmin_Forbidden(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/admin/witness-keys/rotate/progress", nil)
	req = withAuth(req, auth.RoleUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeysProgress_NoJob_ServiceUnavailable(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/admin/witness-keys/rotate/progress", nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeysProgress_NoAuth(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/admin/witness-keys/rotate/progress", nil)
	// No auth context set.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RotateKeys_NoAuth(t *testing.T) {
	job := makeRotationJob()
	h := newTestHandlerWithJob(&mockWitnessRepo{}, auth.CaseRoleInvestigator, job)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/admin/witness-keys/rotate", nil)
	// No auth context set.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500. body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Create handler — invalid JSON body
// ---------------------------------------------------------------------------

func TestHandler_Create_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/witnesses", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400. body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// List — getCaseRole returns error
// ---------------------------------------------------------------------------

func TestHandler_List_RoleError(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&mockWitnessRepo{}, enc, custody, logger)

	// Role loader returns an error — simulates user not being on this case.
	roleLoader := &mockRoleLoader{err: errors.New("role not found")}
	h := NewHandler(svc, roleLoader, &mockAuditLogger{})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("GET", "/api/cases/"+uuid.New().String()+"/witnesses", nil)
	req = withAuth(req, auth.RoleUser) // not a system admin, so roleLoader is called
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}
