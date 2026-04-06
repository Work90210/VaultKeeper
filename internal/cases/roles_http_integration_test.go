//go:build integration

package cases

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

func TestRoleHandler_Integration(t *testing.T) {
	pool := testPool(t)
	caseRepo := NewRepository(pool)
	roleRepo := NewRoleRepository(pool)
	custody := &mockCustody{}

	adminID := uuid.New().String()

	// Create a case
	c, err := caseRepo.Create(t.Context(), Case{
		ReferenceCode: "RHT-TST-" + uuid.New().String()[:4],
		Title:         "Role Handler Test", Status: StatusActive, CreatedBy: adminID,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := NewRoleHandler(roleRepo, custody, nil)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	userID := uuid.New().String()

	// Assign role
	t.Run("assign valid role", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: userID, Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/api/cases/"+c.ID.String()+"/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// Assign duplicate
	t.Run("assign duplicate", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: userID, Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/api/cases/"+c.ID.String()+"/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// Assign invalid role
	t.Run("assign invalid role", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: uuid.New().String(), Role: "invalid"})
		req := httptest.NewRequest(http.MethodPost, "/api/cases/"+c.ID.String()+"/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	// Assign empty user_id
	t.Run("assign empty user_id", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: "", Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/api/cases/"+c.ID.String()+"/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	// List roles
	t.Run("list roles", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/cases/"+c.ID.String()+"/roles", nil)
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})

	// Revoke own role → 403
	t.Run("revoke own role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/cases/"+c.ID.String()+"/roles/"+adminID, nil)
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rr.Code)
		}
	})

	// Revoke other user's role
	t.Run("revoke role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/cases/"+c.ID.String()+"/roles/"+userID, nil)
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
		}
	})

	// Revoke non-existent
	t.Run("revoke non-existent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/cases/"+c.ID.String()+"/roles/"+uuid.New().String(), nil)
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rr.Code)
		}
	})

	// Invalid case ID
	t.Run("invalid case id", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: uuid.New().String(), Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/api/cases/not-uuid/roles", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: adminID, SystemRole: auth.RoleSystemAdmin})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rr.Code)
		}
	})

	// No auth context
	t.Run("no auth context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/cases/"+c.ID.String()+"/roles", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		// This should still work since List doesn't require auth context check
		// (it reads the case ID from URL param)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d", rr.Code)
		}
	})

	// Custody events logged
	if len(custody.events) < 2 {
		t.Errorf("expected at least 2 custody events (assign + revoke), got %d", len(custody.events))
	}
}

// TestRoleHandler_DirectCalls exercises handler branches that are unreachable
// via the chi router (because middleware blocks them before they reach the
// handler function body) by invoking the handler methods directly.
//
// Covered branches:
//   - Assign: no auth context in request context → 500
//   - Assign: invalid case UUID in URL param → 400
//   - Revoke: no auth context → 500
//   - Revoke: invalid case UUID → 400
//   - List:   invalid case UUID → 400
func TestRoleHandler_DirectCalls(t *testing.T) {
	pool := testPool(t)
	roleRepo := NewRoleRepository(pool)
	custody := &mockCustody{}
	h := NewRoleHandler(roleRepo, custody, nil)

	// Assign: no auth context → 500
	t.Run("Assign no auth context", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: uuid.New().String(), Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(setChiParam(req.Context(), "id", uuid.New().String()))
		rr := httptest.NewRecorder()
		h.Assign(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Assign no auth: status = %d, want 500", rr.Code)
		}
	})

	// Assign: invalid case ID → 400
	t.Run("Assign invalid case ID", func(t *testing.T) {
		body, _ := json.Marshal(AssignRoleInput{UserID: uuid.New().String(), Role: "investigator"})
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(setChiParam(req.Context(), "id", "not-a-uuid"))
		req = req.WithContext(auth.WithAuthContext(req.Context(), auth.AuthContext{
			UserID: uuid.New().String(), SystemRole: auth.RoleSystemAdmin,
		}))
		rr := httptest.NewRecorder()
		h.Assign(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Assign invalid ID: status = %d, want 400", rr.Code)
		}
	})

	// Revoke: no auth context → 500
	t.Run("Revoke no auth context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(setChiParam(req.Context(), "id", uuid.New().String()))
		req = req.WithContext(setChiParam(req.Context(), "userId", uuid.New().String()))
		rr := httptest.NewRecorder()
		h.Revoke(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Revoke no auth: status = %d, want 500", rr.Code)
		}
	})

	// Revoke: invalid case ID → 400
	t.Run("Revoke invalid case ID", func(t *testing.T) {
		adminUser := uuid.New().String()
		targetUser := uuid.New().String()
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(setChiParam(req.Context(), "id", "not-a-uuid"))
		req = req.WithContext(setChiParam(req.Context(), "userId", targetUser))
		req = req.WithContext(auth.WithAuthContext(req.Context(), auth.AuthContext{
			UserID: adminUser, SystemRole: auth.RoleSystemAdmin,
		}))
		rr := httptest.NewRecorder()
		h.Revoke(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("Revoke invalid ID: status = %d, want 400", rr.Code)
		}
	})

	// List: invalid case ID → 400
	t.Run("List invalid case ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(setChiParam(req.Context(), "id", "not-a-uuid"))
		rr := httptest.NewRecorder()
		h.List(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("List invalid ID: status = %d, want 400", rr.Code)
		}
	})
}

// setChiParam injects a chi URL param into the context. Calling it multiple
// times accumulates params in the same chi.Context.
func setChiParam(ctx context.Context, key, val string) context.Context {
	rctx, ok := ctx.Value(chi.RouteCtxKey).(*chi.Context)
	if !ok || rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(key, val)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

// ---------------------------------------------------------------------------
// RoleRepository error paths via cancelled context
// ---------------------------------------------------------------------------

// TestRoleRepository_Assign_ContextCancelled covers L39:
// general insert error (not duplicate key).
func TestRoleRepository_Assign_ContextCancelled(t *testing.T) {
	pool := testPool(t)
	repo := NewRoleRepository(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.Assign(ctx, uuid.New(), uuid.New().String(), "investigator", uuid.New().String())
	if err == nil {
		t.Fatal("expected error for cancelled context in Assign, got nil")
	}
	if strings.Contains(err.Error(), "role already assigned") {
		t.Errorf("unexpected duplicate error on cancelled context: %v", err)
	}
}

// TestRoleRepository_Revoke_ContextCancelled covers L49:
// Exec error path.
func TestRoleRepository_Revoke_ContextCancelled(t *testing.T) {
	pool := testPool(t)
	repo := NewRoleRepository(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.Revoke(ctx, uuid.New(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for cancelled context in Revoke, got nil")
	}
	if err == ErrNotFound {
		t.Error("expected a context error, got ErrNotFound")
	}
}

// TestRoleRepository_ListByCaseID_ContextCancelled covers L64:
// query error path.
func TestRoleRepository_ListByCaseID_ContextCancelled(t *testing.T) {
	pool := testPool(t)
	repo := NewRoleRepository(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.ListByCaseID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for cancelled context in ListByCaseID, got nil")
	}
}

// TestRoleRepository_LoadCaseRole_ContextCancelled covers L94:
// general scan error (not ErrNoRows).
func TestRoleRepository_LoadCaseRole_ContextCancelled(t *testing.T) {
	pool := testPool(t)
	repo := NewRoleRepository(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	caseID := uuid.New()
	_, err := repo.LoadCaseRole(ctx, caseID.String(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for cancelled context in LoadCaseRole, got nil")
	}
	if err == auth.ErrNoCaseRole {
		t.Error("expected a context error, got ErrNoCaseRole")
	}
}

