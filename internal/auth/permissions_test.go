package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type mockCaseRoleLoader struct {
	roles   map[string]CaseRole // key: "caseID:userID"
	failErr error               // if set, always return this error
}

func (m *mockCaseRoleLoader) LoadCaseRole(_ context.Context, caseID, userID string) (CaseRole, error) {
	key := caseID + ":" + userID
	if m.failErr != nil {
		return "", m.failErr
	}
	role, ok := m.roles[key]
	if !ok {
		return "", ErrNoCaseRole
	}
	return role, nil
}

func requestWithAuth(method, path string, ac AuthContext) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := WithAuthContext(req.Context(), ac)
	return req.WithContext(ctx)
}

func TestRequireSystemRole(t *testing.T) {
	tests := []struct {
		name     string
		minimum  SystemRole
		userRole SystemRole
		wantCode int
	}{
		{"system_admin accessing system_admin endpoint", RoleSystemAdmin, RoleSystemAdmin, http.StatusOK},
		{"system_admin accessing case_admin endpoint", RoleCaseAdmin, RoleSystemAdmin, http.StatusOK},
		{"system_admin accessing user endpoint", RoleUser, RoleSystemAdmin, http.StatusOK},
		{"case_admin accessing case_admin endpoint", RoleCaseAdmin, RoleCaseAdmin, http.StatusOK},
		{"case_admin accessing user endpoint", RoleUser, RoleCaseAdmin, http.StatusOK},
		{"case_admin accessing system_admin endpoint", RoleSystemAdmin, RoleCaseAdmin, http.StatusForbidden},
		{"user accessing user endpoint", RoleUser, RoleUser, http.StatusOK},
		{"user accessing case_admin endpoint", RoleCaseAdmin, RoleUser, http.StatusForbidden},
		{"user accessing system_admin endpoint", RoleSystemAdmin, RoleUser, http.StatusForbidden},
		{"api_service accessing user endpoint", RoleUser, RoleAPIService, http.StatusForbidden},
		{"api_service accessing api_service endpoint", RoleAPIService, RoleAPIService, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireSystemRole(tt.minimum, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := requestWithAuth(http.MethodGet, "/api/test", AuthContext{
				UserID:     "user-123",
				SystemRole: tt.userRole,
			})
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantCode)
			}

			if tt.wantCode == http.StatusForbidden {
				var body map[string]any
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if body["error"] != "insufficient permissions" {
					t.Errorf("error = %q, want %q", body["error"], "insufficient permissions")
				}
			}
		})
	}
}

func TestRequireSystemRole_NoAuthContext(t *testing.T) {
	handler := RequireSystemRole(RoleUser, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestRequireCaseRole(t *testing.T) {
	loader := &mockCaseRoleLoader{
		roles: map[string]CaseRole{
			"case-1:user-investigator": CaseRoleInvestigator,
			"case-1:user-prosecutor":   CaseRoleProsecutor,
			"case-1:user-defence":      CaseRoleDefence,
			"case-1:user-judge":        CaseRoleJudge,
			"case-1:user-observer":     CaseRoleObserver,
			"case-2:user-investigator": CaseRoleInvestigator,
		},
	}

	tests := []struct {
		name     string
		allowed  []CaseRole
		caseID   string
		userID   string
		sysRole  SystemRole
		wantCode int
	}{
		{
			"investigator accessing investigator endpoint",
			[]CaseRole{CaseRoleInvestigator, CaseRoleProsecutor},
			"case-1", "user-investigator", RoleUser, http.StatusOK,
		},
		{
			"prosecutor accessing investigator+prosecutor endpoint",
			[]CaseRole{CaseRoleInvestigator, CaseRoleProsecutor},
			"case-1", "user-prosecutor", RoleUser, http.StatusOK,
		},
		{
			"defence not in allowed list",
			[]CaseRole{CaseRoleInvestigator, CaseRoleProsecutor},
			"case-1", "user-defence", RoleUser, http.StatusForbidden,
		},
		{
			"user not assigned to case",
			[]CaseRole{CaseRoleInvestigator},
			"case-1", "user-unknown", RoleUser, http.StatusForbidden,
		},
		{
			"system admin bypasses case role",
			[]CaseRole{CaseRoleInvestigator},
			"case-1", "admin-user", RoleSystemAdmin, http.StatusOK,
		},
		{
			"investigator in case-2 accessing case-2",
			[]CaseRole{CaseRoleInvestigator},
			"case-2", "user-investigator", RoleUser, http.StatusOK,
		},
		{
			"investigator in case-1 not in case-3",
			[]CaseRole{CaseRoleInvestigator},
			"case-3", "user-investigator", RoleUser, http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a chi router to properly extract URL params
			r := chi.NewRouter()
			r.With(RequireCaseRole(loader, nil, tt.allowed...)).Get("/api/cases/{id}/evidence", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := requestWithAuth(http.MethodGet, fmt.Sprintf("/api/cases/%s/evidence", tt.caseID), AuthContext{
				UserID:     tt.userID,
				SystemRole: tt.sysRole,
			})
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}
}

func TestRequireCaseRole_InjectsCaseRoleIntoContext(t *testing.T) {
	loader := &mockCaseRoleLoader{
		roles: map[string]CaseRole{
			"case-1:user-1": CaseRoleProsecutor,
		},
	}

	var gotRole CaseRole
	r := chi.NewRouter()
	r.With(RequireCaseRole(loader, nil, CaseRoleProsecutor)).Get("/api/cases/{id}/test", func(_ http.ResponseWriter, r *http.Request) {
		role, ok := GetCaseRole(r.Context())
		if !ok {
			t.Error("expected CaseRole in context")
			return
		}
		gotRole = role
	})

	req := requestWithAuth(http.MethodGet, "/api/cases/case-1/test", AuthContext{
		UserID:     "user-1",
		SystemRole: RoleUser,
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if gotRole != CaseRoleProsecutor {
		t.Errorf("CaseRole = %q, want %q", gotRole, CaseRoleProsecutor)
	}
}

func TestRequireCaseRole_NoAuthContext(t *testing.T) {
	loader := &mockCaseRoleLoader{}

	r := chi.NewRouter()
	r.With(RequireCaseRole(loader, nil, CaseRoleInvestigator)).Get("/api/cases/{id}/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cases/case-1/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestRequireCaseRole_InfraFailure_Returns500(t *testing.T) {
	loader := &mockCaseRoleLoader{
		failErr: fmt.Errorf("database connection timeout"),
	}

	r := chi.NewRouter()
	r.With(RequireCaseRole(loader, nil, CaseRoleInvestigator)).Get("/api/cases/{id}/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := requestWithAuth(http.MethodGet, "/api/cases/case-1/test", AuthContext{
		UserID:     "user-1",
		SystemRole: RoleUser,
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestRequireCaseRole_AuditLogged_NoCaseRole(t *testing.T) {
	loader := &mockCaseRoleLoader{roles: map[string]CaseRole{}}
	audit := &mockAuditLogger{}

	r := chi.NewRouter()
	r.With(RequireCaseRole(loader, audit, CaseRoleInvestigator)).Get("/api/cases/{id}/evidence", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := requestWithAuth(http.MethodGet, "/api/cases/case-1/evidence", AuthContext{
		UserID:     "user-unassigned",
		SystemRole: RoleUser,
		IPAddress:  "10.0.0.1",
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
}

func TestRequireCaseRole_AuditLogged_WrongRole(t *testing.T) {
	loader := &mockCaseRoleLoader{
		roles: map[string]CaseRole{"case-1:user-observer": CaseRoleObserver},
	}
	audit := &mockAuditLogger{}

	r := chi.NewRouter()
	r.With(RequireCaseRole(loader, audit, CaseRoleInvestigator, CaseRoleProsecutor)).Get("/api/cases/{id}/evidence", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := requestWithAuth(http.MethodGet, "/api/cases/case-1/evidence", AuthContext{
		UserID:     "user-observer",
		SystemRole: RoleUser,
		IPAddress:  "10.0.0.1",
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
}

func TestRequireCaseRole_MissingCaseID(t *testing.T) {
	loader := &mockCaseRoleLoader{}
	// Use a route WITHOUT {id} param to trigger the "case ID required" path
	handler := RequireCaseRole(loader, nil, CaseRoleInvestigator)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuth(http.MethodGet, "/api/cases/evidence", AuthContext{
		UserID:     "user-1",
		SystemRole: RoleUser,
	})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPermissionMatrix(t *testing.T) {
	type endpoint struct {
		name    string
		method  string
		path    string
		minimum SystemRole
	}

	endpoints := []endpoint{
		{"create case", http.MethodPost, "/api/cases", RoleCaseAdmin},
		{"list cases", http.MethodGet, "/api/cases", RoleUser},
		{"detailed health", http.MethodGet, "/api/health/detail", RoleSystemAdmin},
		{"audit log", http.MethodGet, "/api/audit", RoleSystemAdmin},
	}

	roles := []struct {
		name string
		role SystemRole
	}{
		{"system_admin", RoleSystemAdmin},
		{"case_admin", RoleCaseAdmin},
		{"user", RoleUser},
		{"api_service", RoleAPIService},
	}

	for _, ep := range endpoints {
		for _, role := range roles {
			t.Run(fmt.Sprintf("%s/%s", ep.name, role.name), func(t *testing.T) {
				handler := RequireSystemRole(ep.minimum, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))

				req := requestWithAuth(ep.method, ep.path, AuthContext{
					UserID:     "user-123",
					SystemRole: role.role,
				})
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				wantAllowed := role.role >= ep.minimum
				gotAllowed := rr.Code == http.StatusOK

				if gotAllowed != wantAllowed {
					t.Errorf("role %s on %s: allowed=%v, want=%v (status=%d)", role.name, ep.name, gotAllowed, wantAllowed, rr.Code)
				}
			})
		}
	}
}
