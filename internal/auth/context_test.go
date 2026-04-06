package auth

import (
	"context"
	"net/http"
	"testing"
)

func TestSystemRoleString(t *testing.T) {
	tests := []struct {
		role SystemRole
		want string
	}{
		{RoleSystemAdmin, "system_admin"},
		{RoleCaseAdmin, "case_admin"},
		{RoleUser, "user"},
		{RoleAPIService, "api_service"},
		{SystemRole(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.role.String(); got != tt.want {
			t.Errorf("SystemRole(%d).String() = %q, want %q", tt.role, got, tt.want)
		}
	}
}

func TestParseSystemRole(t *testing.T) {
	tests := []struct {
		input  string
		want   SystemRole
		wantOK bool
	}{
		{"system_admin", RoleSystemAdmin, true},
		{"case_admin", RoleCaseAdmin, true},
		{"user", RoleUser, true},
		{"api_service", RoleAPIService, true},
		{"unknown_role", RoleNone, false},
		{"", RoleNone, false},
	}

	for _, tt := range tests {
		got, ok := ParseSystemRole(tt.input)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseSystemRole(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestSystemRoleHierarchy(t *testing.T) {
	if !(RoleSystemAdmin > RoleCaseAdmin) {
		t.Error("system_admin should be higher than case_admin")
	}
	if !(RoleCaseAdmin > RoleUser) {
		t.Error("case_admin should be higher than user")
	}
	if !(RoleUser > RoleAPIService) {
		t.Error("user should be higher than api_service")
	}
}

func TestAuthContextRoundTrip(t *testing.T) {
	ac := AuthContext{
		UserID:     "user-123",
		Email:      "test@example.com",
		Username:   "testuser",
		SystemRole: RoleCaseAdmin,
		SessionID:  "sess-456",
	}

	ctx := WithAuthContext(context.Background(), ac)
	got, ok := GetAuthContext(ctx)
	if !ok {
		t.Fatal("expected AuthContext in context")
	}

	if got.UserID != ac.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, ac.UserID)
	}
	if got.SystemRole != ac.SystemRole {
		t.Errorf("SystemRole = %v, want %v", got.SystemRole, ac.SystemRole)
	}
}

func TestGetAuthContextMissing(t *testing.T) {
	_, ok := GetAuthContext(context.Background())
	if ok {
		t.Error("expected no AuthContext in empty context")
	}
}

func TestCaseRoleRoundTrip(t *testing.T) {
	ctx := WithCaseRole(context.Background(), CaseRoleProsecutor)
	got, ok := GetCaseRole(ctx)
	if !ok {
		t.Fatal("expected CaseRole in context")
	}
	if got != CaseRoleProsecutor {
		t.Errorf("CaseRole = %q, want %q", got, CaseRoleProsecutor)
	}
}

func TestGetCaseRoleMissing(t *testing.T) {
	_, ok := GetCaseRole(context.Background())
	if ok {
		t.Error("expected no CaseRole in empty context")
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1"},
			remoteAddr: "127.0.0.1:1234",
			want:       "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For chain",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1"},
			remoteAddr: "127.0.0.1:1234",
			want:       "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "198.51.100.2"},
			remoteAddr: "127.0.0.1:1234",
			want:       "198.51.100.2",
		},
		{
			name:       "RemoteAddr with port",
			headers:    map[string]string{},
			remoteAddr: "127.0.0.1:1234",
			want:       "127.0.0.1",
		},
		{
			name:       "RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "127.0.0.1",
			want:       "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			if got := GetClientIP(r); got != tt.want {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
