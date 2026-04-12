package evidence

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// requireCaseMembership verifies that the caller has any role on the target
// case (or is a system admin). Sprint 9 hardening: tag management endpoints
// previously checked only the system role, which let a case_admin on one
// case mutate tags in any other case. Returns true if the caller may proceed.
func (h *Handler) requireCaseMembership(ctx context.Context, caseID uuid.UUID) bool {
	ac, ok := auth.GetAuthContext(ctx)
	if !ok {
		return false
	}
	if ac.SystemRole == auth.RoleSystemAdmin {
		return true
	}
	if h.caseRoleLoader == nil {
		return false
	}
	_, err := h.caseRoleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
	return err == nil
}

// TagAutocomplete returns up to 20 distinct tags for a case matching a
// case-insensitive prefix.
//
// GET /api/evidence/tags/autocomplete?case_id=<uuid>&q=<prefix>&limit=<n>
func (h *Handler) TagAutocomplete(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseID, err := uuid.Parse(r.URL.Query().Get("case_id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case_id")
		return
	}

	// Sprint 9: autocomplete could be used to enumerate tag vocabulary
	// across cases — gate behind case membership.
	if !h.requireCaseMembership(r.Context(), caseID) {
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}

	q := r.URL.Query().Get("q")

	limit := MaxTagAutocompleteLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	tags, err := h.service.AutocompleteTags(r.Context(), caseID, q, limit)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// tagRenameRequest is the JSON body for POST /api/evidence/tags/rename.
type tagRenameRequest struct {
	CaseID uuid.UUID `json:"case_id"`
	Old    string    `json:"old"`
	New    string    `json:"new"`
}

// TagRename rewrites every occurrence of `old` to `new` for evidence in a
// single case. Requires case_admin.
func (h *Handler) TagRename(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var body tagRenameRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !h.requireCaseMembership(r.Context(), body.CaseID) {
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}

	count, err := h.service.RenameTag(r.Context(), body.CaseID, body.Old, body.New, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"rows_affected": count,
		"old":           body.Old,
		"new":           body.New,
	})
}

// tagMergeRequest is the JSON body for POST /api/evidence/tags/merge.
type tagMergeRequest struct {
	CaseID  uuid.UUID `json:"case_id"`
	Sources []string  `json:"sources"`
	Target  string    `json:"target"`
}

// TagMerge collapses one or more source tags into a target tag across the
// case. Requires case_admin.
func (h *Handler) TagMerge(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var body tagMergeRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !h.requireCaseMembership(r.Context(), body.CaseID) {
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}

	count, err := h.service.MergeTags(r.Context(), body.CaseID, body.Sources, body.Target, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"rows_affected": count,
		"sources":       body.Sources,
		"target":        body.Target,
	})
}

// tagDeleteRequest is the JSON body for POST /api/evidence/tags/delete.
type tagDeleteRequest struct {
	CaseID uuid.UUID `json:"case_id"`
	Tag    string    `json:"tag"`
}

// TagDelete removes a tag from every evidence item in a case. Requires
// case_admin.
func (h *Handler) TagDelete(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var body tagDeleteRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !h.requireCaseMembership(r.Context(), body.CaseID) {
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}

	count, err := h.service.DeleteTag(r.Context(), body.CaseID, body.Tag, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"rows_affected": count,
		"tag":           body.Tag,
	})
}
