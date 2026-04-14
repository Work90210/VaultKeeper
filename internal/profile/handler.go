package profile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// OrgLister lists a user's organizations. Injected to avoid circular dependency.
type OrgLister interface {
	ListUserOrgs(ctx context.Context, userID string) (any, error)
}

// CaseLister lists a user's cases. Injected to avoid circular dependency.
type CaseLister interface {
	ListUserCases(ctx context.Context, userID string) (any, error)
}

type Handler struct {
	repo       Repository
	audit      auth.AuditLogger
	orgLister  OrgLister
	caseLister CaseLister
}

func NewHandler(repo Repository, audit auth.AuditLogger) *Handler {
	return &Handler{repo: repo, audit: audit}
}

func (h *Handler) SetOrgLister(l OrgLister) { h.orgLister = l }
func (h *Handler) SetCaseLister(l CaseLister) { h.caseLister = l }

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/me", func(r chi.Router) {
		r.Get("/", h.GetProfile)
		r.Patch("/", h.UpdateProfile)
		r.Get("/organizations", h.ListMyOrganizations)
		r.Get("/cases", h.ListMyCases)
	})
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	p, err := h.repo.GetByUserID(r.Context(), ac.UserID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			// Auto-create a default profile from auth context.
			p = Profile{
				UserID:      ac.UserID,
				DisplayName: ac.Username,
				Timezone:    "UTC",
			}
			p, err = h.repo.Upsert(r.Context(), p)
			if err != nil {
				httputil.RespondError(w, http.StatusInternalServerError, "failed to create profile")
				return
			}
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to load profile")
			return
		}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"profile": p,
		"email":   ac.Email,
		"role":    ac.SystemRole.String(),
	})
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var input UpdateProfileInput
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if err := json.Unmarshal(body, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	p := Profile{UserID: ac.UserID}
	if input.DisplayName != nil {
		p.DisplayName = *input.DisplayName
	}
	if input.Bio != nil {
		p.Bio = *input.Bio
	}
	if input.Timezone != nil {
		p.Timezone = *input.Timezone
	}
	if input.AvatarURL != nil {
		p.AvatarURL = *input.AvatarURL
	}

	result, err := h.repo.Upsert(r.Context(), p)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (h *Handler) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.orgLister == nil {
		httputil.RespondError(w, http.StatusNotImplemented, "org lister not configured")
		return
	}

	orgs, err := h.orgLister.ListUserOrgs(r.Context(), ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, orgs)
}

func (h *Handler) ListMyCases(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.caseLister == nil {
		httputil.RespondError(w, http.StatusNotImplemented, "case lister not configured")
		return
	}

	cases, err := h.caseLister.ListUserCases(r.Context(), ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list cases")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, cases)
}
