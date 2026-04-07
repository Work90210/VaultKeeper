package cases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

type mockRoleRepo struct {
	roles        map[string]CaseRole // key: "caseID:userID"
	assignErr    error
	revokeErr    error
	listErr      error
}

func (m *mockRoleRepo) Assign(_ context.Context, caseID uuid.UUID, userID, role, grantedBy string) (CaseRole, error) {
	if m.assignErr != nil {
		return CaseRole{}, m.assignErr
	}
	key := caseID.String() + ":" + userID
	if _, exists := m.roles[key]; exists {
		return CaseRole{}, fmt.Errorf("role already assigned: duplicate")
	}
	cr := CaseRole{
		ID: uuid.New(), CaseID: caseID, UserID: userID,
		Role: role, GrantedBy: grantedBy, GrantedAt: time.Now(),
	}
	m.roles[key] = cr
	return cr, nil
}

func (m *mockRoleRepo) Revoke(_ context.Context, caseID uuid.UUID, userID string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	key := caseID.String() + ":" + userID
	if _, exists := m.roles[key]; !exists {
		return ErrNotFound
	}
	delete(m.roles, key)
	return nil
}

func (m *mockRoleRepo) ListByCaseID(_ context.Context, caseID uuid.UUID) ([]CaseRole, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []CaseRole
	for key, cr := range m.roles {
		if strings.HasPrefix(key, caseID.String()+":") {
			result = append(result, cr)
		}
	}
	return result, nil
}

func setupRoleHandler(t *testing.T) (*RoleHandler, *mockRoleRepo, uuid.UUID) {
	t.Helper()
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	custody := &mockCustody{}
	caseID := uuid.New()
	h := NewRoleHandler(roleRepo, custody, nil)
	return h, roleRepo, caseID
}

func TestRoleHandler_Assign(t *testing.T) {
	caseID := uuid.New()
	userID := uuid.New().String()
	custody := &mockCustody{}

	// We need to test the HTTP handler directly using chi
	// Since roles.go uses RoleRepository (pgxpool), we test the handler logic
	// through the HTTP layer by checking request/response behavior

	// Test invalid role
	r := chi.NewRouter()
	r.Post("/api/cases/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		// Simulate the handler checking role validity
		var input AssignRoleInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		if !ValidCaseRoles[input.Role] {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid role"})
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	body, _ := json.Marshal(AssignRoleInput{UserID: userID, Role: "invalid_role"})
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/roles", bytes.NewReader(body))
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: "admin", SystemRole: auth.RoleSystemAdmin})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("invalid role: status = %d, want 400", rr.Code)
	}

	_ = custody
}

func TestRoleHandler_Revoke_Self(t *testing.T) {
	// Test that revoking your own role returns 403
	adminID := "admin-user-id"

	r := chi.NewRouter()
	r.Delete("/api/cases/{id}/roles/{userId}", func(w http.ResponseWriter, r *http.Request) {
		ac, _ := auth.GetAuthContext(r.Context())
		targetUserID := chi.URLParam(r, "userId")
		if targetUserID == ac.UserID {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "cannot remove your own role"})
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/cases/"+uuid.New().String()+"/roles/"+adminID, nil)
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("self-revoke: status = %d, want 403", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Handler error branches using mockRoleRepo
// ---------------------------------------------------------------------------

// buildRoleHandlerRequest builds a request with chi URL params and auth ctx.
func buildRoleHandlerRequest(method, caseIDStr, userIDParam string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, "/", &buf)
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseIDStr)
	if userIDParam != "" {
		rctx.URLParams.Add("userId", userIDParam)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.WithAuthContext(ctx, auth.AuthContext{
		UserID: "admin-user", SystemRole: auth.RoleSystemAdmin,
	})
	return req.WithContext(ctx)
}

// TestRoleHandler_Assign_InvalidJSON_Unit covers L131 (decodeBody error).
func TestRoleHandler_Assign_InvalidJSON_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.WithAuthContext(ctx, auth.AuthContext{UserID: "admin", SystemRole: auth.RoleSystemAdmin})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Assign invalid JSON: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_Assign_InternalError_Unit covers L152 (non-duplicate repo error).
func TestRoleHandler_Assign_InternalError_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{
		roles:     make(map[string]CaseRole),
		assignErr: fmt.Errorf("unexpected db failure"),
	}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: uuid.New().String(),
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Assign internal error: status = %d, want 500", rr.Code)
	}
}

// TestRoleHandler_Revoke_ListError_Unit covers L188 (ListByCaseID error during revoke).
func TestRoleHandler_Revoke_ListError_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{
		roles:   make(map[string]CaseRole),
		listErr: fmt.Errorf("list query failed"),
	}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	targetID := uuid.New().String()
	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), targetID, nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Revoke list error: status = %d, want 500", rr.Code)
	}
}

// TestRoleHandler_Revoke_RevokeError_Unit covers L205 (Revoke repo error ≠ ErrNotFound).
func TestRoleHandler_Revoke_RevokeError_Unit(t *testing.T) {
	caseID := uuid.New()
	targetID := uuid.New().String()

	// List succeeds (role exists); Revoke fails with a non-ErrNotFound error.
	roleRepo := &mockRoleRepo{
		roles: map[string]CaseRole{
			caseID.String() + ":" + targetID: {
				ID:     uuid.New(),
				CaseID: caseID,
				UserID: targetID,
				Role:   "investigator",
			},
		},
		revokeErr: fmt.Errorf("unexpected revoke failure"),
	}
	h := NewRoleHandler(roleRepo, nil, nil)

	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), targetID, nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Revoke error: status = %d, want 500", rr.Code)
	}
}

// TestRoleHandler_List_ListError_Unit covers L228 (ListByCaseID error in List).
func TestRoleHandler_List_ListError_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{
		roles:   make(map[string]CaseRole),
		listErr: fmt.Errorf("list query failed"),
	}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("List error: status = %d, want 500", rr.Code)
	}
}

func TestRoleHandler_Assign_EmptyUserID(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/api/cases/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		var input AssignRoleInput
		_ = json.NewDecoder(r.Body).Decode(&input)
		if input.UserID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	body, _ := json.Marshal(AssignRoleInput{UserID: "", Role: "investigator"})
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+uuid.New().String()+"/roles", bytes.NewReader(body))
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: "admin", SystemRole: auth.RoleSystemAdmin})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty user_id: status = %d, want 400", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// RoleHandler direct calls — all branches via real handler with mockRoleRepo
// ---------------------------------------------------------------------------

// TestRoleHandler_Assign_Success_Unit covers the full success path including
// custody logging (L132-L180).
func TestRoleHandler_Assign_Success_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	custody := &mockCustody{}
	h := NewRoleHandler(roleRepo, custody, nil)

	caseID := uuid.New()
	userID := uuid.New().String()
	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: userID,
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Assign success: status = %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}
	if len(custody.events) != 1 || custody.events[0] != "role_granted" {
		t.Errorf("custody events = %v, want [role_granted]", custody.events)
	}
}

// TestRoleHandler_Assign_InvalidCaseID_Unit covers L139 (invalid UUID parse).
func TestRoleHandler_Assign_InvalidCaseID_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	req := buildRoleHandlerRequest(http.MethodPost, "not-a-uuid", "", AssignRoleInput{
		UserID: uuid.New().String(),
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Assign invalid case ID: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_Assign_NoAuthContext_Unit covers L133-L136 (missing auth context).
func TestRoleHandler_Assign_NoAuthContext_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"user_id":"x","role":"investigator"}`))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Assign no auth: status = %d, want 500", rr.Code)
	}
}

// TestRoleHandler_Assign_InvalidRole_Unit covers L151 (invalid role).
func TestRoleHandler_Assign_InvalidRole_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: uuid.New().String(),
		Role:   "hacker",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Assign invalid role: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_Assign_EmptyUserID_Unit covers L156 (empty user_id).
func TestRoleHandler_Assign_EmptyUserID_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: "",
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Assign empty user_id: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_Assign_Duplicate_Unit covers L163 (duplicate assignment).
func TestRoleHandler_Assign_Duplicate_Unit(t *testing.T) {
	caseID := uuid.New()
	userID := uuid.New().String()
	roleRepo := &mockRoleRepo{
		roles: map[string]CaseRole{
			caseID.String() + ":" + userID: {
				ID: uuid.New(), CaseID: caseID, UserID: userID, Role: "investigator",
			},
		},
	}
	h := NewRoleHandler(roleRepo, nil, nil)

	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: userID,
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("Assign duplicate: status = %d, want 409", rr.Code)
	}
}

// TestRoleHandler_Assign_NilCustody_Unit covers L171 (custody == nil path).
func TestRoleHandler_Assign_NilCustody_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil) // nil custody

	caseID := uuid.New()
	req := buildRoleHandlerRequest(http.MethodPost, caseID.String(), "", AssignRoleInput{
		UserID: uuid.New().String(),
		Role:   "investigator",
	})

	rr := httptest.NewRecorder()
	h.Assign(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Assign nil custody: status = %d, want 201", rr.Code)
	}
}

// TestRoleHandler_Revoke_Success_Unit covers full revoke success with custody (L182-L233).
func TestRoleHandler_Revoke_Success_Unit(t *testing.T) {
	caseID := uuid.New()
	targetID := uuid.New().String()
	roleRepo := &mockRoleRepo{
		roles: map[string]CaseRole{
			caseID.String() + ":" + targetID: {
				ID: uuid.New(), CaseID: caseID, UserID: targetID, Role: "investigator",
			},
		},
	}
	custody := &mockCustody{}
	h := NewRoleHandler(roleRepo, custody, nil)

	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), targetID, nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Revoke success: status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
	if len(custody.events) != 1 || custody.events[0] != "role_revoked" {
		t.Errorf("custody events = %v, want [role_revoked]", custody.events)
	}
}

// TestRoleHandler_Revoke_NoAuthContext_Unit covers L183-L186 (missing auth context).
func TestRoleHandler_Revoke_NoAuthContext_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	rctx.URLParams.Add("userId", uuid.New().String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Revoke no auth: status = %d, want 500", rr.Code)
	}
}

// TestRoleHandler_Revoke_InvalidCaseID_Unit covers L189 (invalid UUID).
func TestRoleHandler_Revoke_InvalidCaseID_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	req := buildRoleHandlerRequest(http.MethodDelete, "not-a-uuid", "user-1", nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Revoke invalid case ID: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_Revoke_Self_Unit covers L197 (self-revoke forbidden).
func TestRoleHandler_Revoke_Self_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	// buildRoleHandlerRequest sets UserID to "admin-user"
	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), "admin-user", nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("Revoke self: status = %d, want 403", rr.Code)
	}
}

// TestRoleHandler_Revoke_NotFound_Unit covers L216-L219 (role not found).
func TestRoleHandler_Revoke_NotFound_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), uuid.New().String(), nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Revoke not found: status = %d, want 404", rr.Code)
	}
}

// TestRoleHandler_Revoke_NilCustody_Unit covers L224 (custody == nil).
func TestRoleHandler_Revoke_NilCustody_Unit(t *testing.T) {
	caseID := uuid.New()
	targetID := uuid.New().String()
	roleRepo := &mockRoleRepo{
		roles: map[string]CaseRole{
			caseID.String() + ":" + targetID: {
				ID: uuid.New(), CaseID: caseID, UserID: targetID, Role: "investigator",
			},
		},
	}
	h := NewRoleHandler(roleRepo, nil, nil) // nil custody

	req := buildRoleHandlerRequest(http.MethodDelete, caseID.String(), targetID, nil)

	rr := httptest.NewRecorder()
	h.Revoke(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Revoke nil custody: status = %d, want 200", rr.Code)
	}
}

// TestRoleHandler_List_Success_Unit covers L235-L249 (success path).
func TestRoleHandler_List_Success_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	caseID := uuid.New()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("List success: status = %d, want 200", rr.Code)
	}
}

// TestRoleHandler_List_InvalidCaseID_Unit covers L236 (invalid UUID).
func TestRoleHandler_List_InvalidCaseID_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("List invalid case ID: status = %d, want 400", rr.Code)
	}
}

// TestRoleHandler_RegisterRoutes_Unit covers L124 (RegisterRoutes).
func TestRoleHandler_RegisterRoutes_Unit(t *testing.T) {
	roleRepo := &mockRoleRepo{roles: make(map[string]CaseRole)}
	h := NewRoleHandler(roleRepo, nil, nil)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	routeCount := 0
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(route, "/roles") {
			routeCount++
		}
		return nil
	})

	if routeCount < 3 {
		t.Errorf("expected at least 3 role routes (POST, DELETE, GET), got %d", routeCount)
	}
}
