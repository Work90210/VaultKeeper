package roledefs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,48}[a-z0-9]$`)

// OrgMembershipChecker verifies whether a user is an active member of an organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
	// GetOrgRole returns the caller's org-level role string (e.g. "owner", "admin", "member").
	// Implementations should return an error when the user is not a member.
	GetOrgRole(ctx context.Context, orgID uuid.UUID, userID string) (string, error)
}

type Handler struct {
	repo       *PgRepository
	orgChecker OrgMembershipChecker
}

func NewHandler(repo *PgRepository) *Handler {
	return &Handler{repo: repo}
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// all role-definition endpoints require that the caller is a member of the org.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker) {
	h.orgChecker = checker
}

// requireOrgMembership verifies the caller is authenticated and is an active
// member of orgID. System admins bypass the membership check. Returns true if
// access is allowed, false if a response has already been written.
func (h *Handler) requireOrgMembership(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) bool {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return true
	}
	if h.orgChecker == nil {
		// Fail closed: checker not wired, deny access.
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	isMember, err := h.orgChecker.IsActiveMember(r.Context(), orgID, ac.UserID)
	if err != nil || !isMember {
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	return true
}

// requireOrgAdmin verifies the caller is authenticated and holds an admin or
// owner role in the org. System admins bypass the check. Returns true if access
// is allowed, false if a response has already been written.
func (h *Handler) requireOrgAdmin(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) bool {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return true
	}
	if h.orgChecker == nil {
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	role, err := h.orgChecker.GetOrgRole(r.Context(), orgID, ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	if role != "owner" && role != "admin" {
		httputil.RespondError(w, http.StatusForbidden, "org admin role required")
		return false
	}
	return true
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/organizations/{orgId}/role-definitions", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{roleDefId}", h.Get)
		r.Patch("/{roleDefId}", h.Update)
		r.Delete("/{roleDefId}", h.Delete)
		r.Post("/{roleDefId}/reset", h.ResetToDefault)
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgMembership(w, r, orgID) {
		return
	}

	defs, err := h.repo.ListByOrg(r.Context(), orgID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, defs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgMembership(w, r, orgID) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "roleDefId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid role definition ID")
		return
	}

	def, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "role definition not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if def.OrganizationID != orgID {
		httputil.RespondError(w, http.StatusNotFound, "role definition not found")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, def)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgAdmin(w, r, orgID) {
		return
	}

	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}

	slug := toSlug(name)
	if !slugRe.MatchString(slug) {
		httputil.RespondError(w, http.StatusBadRequest, "name produces an invalid slug; use only letters, numbers, and spaces")
		return
	}

	perms := sanitizePermissions(input.Permissions)

	def, err := h.repo.Create(r.Context(), RoleDefinition{
		OrganizationID: orgID,
		Name:           name,
		Slug:           slug,
		Description:    strings.TrimSpace(input.Description),
		Permissions:    perms,
		IsDefault:      false,
		IsSystem:       false,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			httputil.RespondError(w, http.StatusConflict, "a role with this name already exists")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, def)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgAdmin(w, r, orgID) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "roleDefId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid role definition ID")
		return
	}

	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			httputil.RespondError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		input.Name = &trimmed
	}

	if input.Permissions != nil {
		cleaned := sanitizePermissions(input.Permissions)
		input.Permissions = cleaned
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "role definition not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.OrganizationID != orgID {
		httputil.RespondError(w, http.StatusNotFound, "role definition not found")
		return
	}

	def, err := h.repo.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "role definition not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, def)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgAdmin(w, r, orgID) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "roleDefId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid role definition ID")
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "role definition not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.OrganizationID != orgID {
		httputil.RespondError(w, http.StatusNotFound, "role definition not found")
		return
	}

	if existing.IsSystem {
		httputil.RespondError(w, http.StatusForbidden, "system roles cannot be deleted; you can edit their permissions or reset to defaults")
		return
	}

	inUse, err := h.repo.IsInUse(r.Context(), id)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if inUse {
		httputil.RespondError(w, http.StatusConflict, "role is assigned to active cases and cannot be deleted")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) ResetToDefault(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid org ID")
		return
	}
	if !h.requireOrgAdmin(w, r, orgID) {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "roleDefId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid role definition ID")
		return
	}

	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "role definition not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.OrganizationID != orgID {
		httputil.RespondError(w, http.StatusNotFound, "role definition not found")
		return
	}

	if !existing.IsDefault {
		httputil.RespondError(w, http.StatusBadRequest, "only default roles can be reset")
		return
	}

	// Find matching default
	var defaultPerms map[Permission]bool
	var defaultDesc string
	var defaultName string
	for _, def := range DefaultRoleDefinitions() {
		if def.Slug == existing.Slug {
			defaultPerms = def.Permissions
			defaultDesc = def.Description
			defaultName = def.Name
			break
		}
	}
	if defaultPerms == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "default definition not found for slug")
		return
	}

	updated, err := h.repo.Update(r.Context(), id, UpdateInput{
		Name:        &defaultName,
		Description: &defaultDesc,
		Permissions: defaultPerms,
	})
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, updated)
}

// sanitizePermissions ensures only known permissions are included and all
// permissions have an explicit boolean value.
func sanitizePermissions(input map[Permission]bool) map[Permission]bool {
	result := make(map[Permission]bool, len(AllPermissions))
	for _, p := range AllPermissions {
		result[p] = input[p]
	}
	return result
}

func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	// Strip non-alphanum/underscore
	var b strings.Builder
	for _, ch := range s {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

