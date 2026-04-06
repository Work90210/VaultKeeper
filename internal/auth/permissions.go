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

