package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type CaseRoleLoader interface {
	LoadCaseRole(ctx context.Context, caseID, userID string) (CaseRole, error)
}

// OrgMemberVerifier checks whether a user is an active member of the org that
// owns a given case. Injected into RequireCaseRoleWithOrg to enforce org boundaries.
type OrgMemberVerifier interface {
	VerifyCaseOrgMembership(ctx context.Context, caseID, userID string) error
}

var ErrNoCaseRole = errors.New("no case role assigned")

func RequireSystemRole(minimum SystemRole, audit AuditLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := GetAuthContext(r.Context())
			if !ok {
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}

			if ac.SystemRole < minimum {
				if audit != nil {
					audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, minimum.String(), ac.SystemRole.String(), ac.IPAddress)
				}
				respondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireCaseRoleWithOrg extends RequireCaseRole with an org membership check.
// If orgVerifier is non-nil, the caller must be an active member of the case's org.
func RequireCaseRoleWithOrg(loader CaseRoleLoader, orgVerifier OrgMemberVerifier, audit AuditLogger, allowed ...CaseRole) func(http.Handler) http.Handler {
	allowedSet := make(map[CaseRole]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := GetAuthContext(r.Context())
			if !ok {
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}

			if ac.SystemRole >= RoleSystemAdmin {
				next.ServeHTTP(w, r)
				return
			}

			caseID := chi.URLParam(r, "id")
			if caseID == "" {
				respondError(w, http.StatusBadRequest, "case ID required")
				return
			}

			// Org membership gate.
			if orgVerifier != nil {
				if err := orgVerifier.VerifyCaseOrgMembership(r.Context(), caseID, ac.UserID); err != nil {
					if audit != nil {
						audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "org_member", "none", ac.IPAddress)
					}
					respondError(w, http.StatusForbidden, "insufficient permissions")
					return
				}
			}

			role, err := loader.LoadCaseRole(r.Context(), caseID, ac.UserID)
			if err != nil {
				if errors.Is(err, ErrNoCaseRole) {
					if audit != nil {
						audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "case_role", "none", ac.IPAddress)
					}
					respondError(w, http.StatusForbidden, "insufficient permissions")
					return
				}
				respondError(w, http.StatusInternalServerError, "authorization check failed")
				return
			}

			if _, ok := allowedSet[role]; !ok {
				if audit != nil {
					audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "case_role", string(role), ac.IPAddress)
				}
				respondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			ctx := WithCaseRole(r.Context(), role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireCaseRole(loader CaseRoleLoader, audit AuditLogger, allowed ...CaseRole) func(http.Handler) http.Handler {
	allowedSet := make(map[CaseRole]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := GetAuthContext(r.Context())
			if !ok {
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}

			// System admins bypass case role checks
			if ac.SystemRole >= RoleSystemAdmin {
				next.ServeHTTP(w, r)
				return
			}

			caseID := chi.URLParam(r, "id")
			if caseID == "" {
				respondError(w, http.StatusBadRequest, "case ID required")
				return
			}

			role, err := loader.LoadCaseRole(r.Context(), caseID, ac.UserID)
			if err != nil {
				if errors.Is(err, ErrNoCaseRole) {
					if audit != nil {
						audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "case_role", "none", ac.IPAddress)
					}
					respondError(w, http.StatusForbidden, "insufficient permissions")
					return
				}
				respondError(w, http.StatusInternalServerError, "authorization check failed")
				return
			}

			if _, ok := allowedSet[role]; !ok {
				if audit != nil {
					audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "case_role", string(role), ac.IPAddress)
				}
				respondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			ctx := WithCaseRole(r.Context(), role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

