package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type SystemRole int

const RoleNone SystemRole = -1

const (
	RoleAPIService SystemRole = iota
	RoleUser
	RoleCaseAdmin
	RoleSystemAdmin
)

func (r SystemRole) String() string {
	switch r {
	case RoleSystemAdmin:
		return "system_admin"
	case RoleCaseAdmin:
		return "case_admin"
	case RoleUser:
		return "user"
	case RoleAPIService:
		return "api_service"
	default:
		return "unknown"
	}
}

func ParseSystemRole(s string) (SystemRole, bool) {
	switch s {
	case "system_admin":
		return RoleSystemAdmin, true
	case "case_admin":
		return RoleCaseAdmin, true
	case "user":
		return RoleUser, true
	case "api_service":
		return RoleAPIService, true
	default:
		return RoleNone, false
	}
}

type CaseRole string

const (
	CaseRoleInvestigator        CaseRole = "investigator"
	CaseRoleProsecutor          CaseRole = "prosecutor"
	CaseRoleDefence             CaseRole = "defence"
	CaseRoleJudge               CaseRole = "judge"
	CaseRoleObserver            CaseRole = "observer"
	CaseRoleVictimRepresentative CaseRole = "victim_representative"
)

type AuthContext struct {
	UserID      string
	Email       string
	Username    string
	SystemRole  SystemRole
	TokenExpiry int64
	SessionID   string
	IPAddress   string
}

type contextKey int

const (
	authContextKey contextKey = iota
	caseRoleKey
)

func WithAuthContext(ctx context.Context, ac AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, ac)
}

func GetAuthContext(ctx context.Context) (AuthContext, bool) {
	ac, ok := ctx.Value(authContextKey).(AuthContext)
	return ac, ok
}

func WithCaseRole(ctx context.Context, role CaseRole) context.Context {
	return context.WithValue(ctx, caseRoleKey, role)
}

func GetCaseRole(ctx context.Context) (CaseRole, bool) {
	role, ok := ctx.Value(caseRoleKey).(CaseRole)
	return role, ok
}

func GetClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	// Only trust proxy headers when the direct connection originates from a
	// loopback or private network address (i.e. a trusted reverse proxy).
	if isPrivateIP(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the last (rightmost) IP — the one appended by the trusted proxy.
			ip := xff
			if idx := strings.LastIndexByte(xff, ','); idx != -1 {
				ip = xff[idx+1:]
			}
			return strings.TrimSpace(ip)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	return host
}

// privateNetworks is parsed once at init to avoid re-parsing on every request.
var privateNetworks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid private CIDR: " + cidr)
		}
		privateNetworks = append(privateNetworks, network)
	}
}

func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range privateNetworks {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}
