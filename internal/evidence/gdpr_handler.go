package evidence

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// GDPRRouteRegistrar mounts the GDPR erasure endpoints on the router. It is
// a separate RouteRegistrar (vs. Handler.RegisterRoutes) so these routes can
// be wired independently from the main evidence handler without modifying
// handler.go — the two handlers share the same package-level Handler type.
type GDPRRouteRegistrar struct {
	Handler *Handler
	Audit   auth.AuditLogger
}

// RegisterRoutes implements server.RouteRegistrar.
func (g *GDPRRouteRegistrar) RegisterRoutes(r chi.Router) {
	// POST /api/evidence/{id}/erasure-requests — case_admin or higher.
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, g.Audit)).
		Post("/api/evidence/{id}/erasure-requests", g.Handler.CreateErasureRequest)

	// POST /api/erasure-requests/{id}/resolve — case_admin or higher.
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, g.Audit)).
		Post("/api/erasure-requests/{id}/resolve", g.Handler.ResolveErasureRequest)
}

// ensureEvidenceOrgMembership verifies that the caller is a member of the
// organization that owns the case containing the given evidence item.
// Returns true if the caller may proceed.
func (h *Handler) ensureEvidenceOrgMembership(ctx context.Context, evidenceID uuid.UUID) bool {
	if h.orgChecker == nil || h.caseLookupOrg == nil {
		return true // not wired — skip check (backwards compat)
	}
	ac, ok := auth.GetAuthContext(ctx)
	if !ok {
		return false
	}
	if ac.SystemRole == auth.RoleSystemAdmin {
		return true
	}
	item, err := h.service.Get(ctx, evidenceID)
	if err != nil {
		return false
	}
	orgID, err := h.caseLookupOrg(ctx, item.CaseID)
	if err != nil {
		return false
	}
	isMember, err := h.orgChecker.IsActiveMember(ctx, orgID, ac.UserID)
	return err == nil && isMember
}

// createErasureRequestBody is the POST body for opening a GDPR erasure
// request against an evidence item.
type createErasureRequestBody struct {
	RequestedBy string `json:"requested_by"`
	Rationale   string `json:"rationale"`
}

// createErasureRequestResponse is the success envelope for
// CreateErasureRequest. The conflict field is always present so clients can
// branch on conflict.has_conflict without a nil check.
type createErasureRequestResponse struct {
	Request  ErasureRequest `json:"request"`
	Conflict ConflictReport `json:"conflict"`
}

// CreateErasureRequest handles POST /api/evidence/{id}/erasure-requests.
// It opens a GDPR "right to be forgotten" workflow for a single evidence
// item.
//
// Actor identity is taken from the authenticated session (ac.UserID, a
// UUID). The body.requested_by field is accepted for backwards compat
// with existing clients but is IGNORED — allowing it to flow into the
// custody log would (a) let a caller impersonate another user in the
// audit trail, and (b) break the custody_log.actor_user_id UUID column
// constraint whenever the client sent a username string.
func (h *Handler) CreateErasureRequest(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	if !h.ensureEvidenceOrgMembership(r.Context(), id) {
		httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	var body createErasureRequestBody
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Sprint 9: actor identity ALWAYS comes from the authenticated session,
	// never from the request body. The custody log column is a UUID and
	// accepting a free-form body string causes SQL insert failures (and
	// would let a caller impersonate another user in the audit trail).
	// body.RequestedBy is ignored for the custody record — the service
	// always records ac.UserID as the actor.
	_ = body.RequestedBy

	req, report, err := h.service.CreateErasureRequest(r.Context(), id, ac.UserID, body.Rationale)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, createErasureRequestResponse{
		Request:  req,
		Conflict: report,
	})
}

// resolveErasureRequestBody is the POST body for closing a pending erasure
// request with either "preserve" or "erase".
type resolveErasureRequestBody struct {
	Decision  string `json:"decision"`
	DecidedBy string `json:"decided_by"`
	Rationale string `json:"rationale"`
}

// ResolveErasureRequest handles POST /api/erasure-requests/{id}/resolve.
// It records a decision on a conflict_pending erasure request and (on
// decision=erase) invokes DestroyEvidence with a GDPR authority string.
func (h *Handler) ResolveErasureRequest(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	// Org membership gate: resolve erasure request → evidence → case → org.
	if h.orgChecker != nil && h.caseLookupOrg != nil {
		erasureReq, err := h.service.FindErasureRequest(r.Context(), id)
		if err != nil {
			httputil.RespondError(w, http.StatusNotFound, "erasure request not found")
			return
		}
		if !h.ensureEvidenceOrgMembership(r.Context(), erasureReq.EvidenceID) {
			httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
			return
		}
	}

	var body resolveErasureRequestBody
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Sprint 9: actor identity ALWAYS comes from the authenticated session.
	// See CreateErasureRequest for rationale. body.DecidedBy is ignored.
	_ = body.DecidedBy

	if err := h.service.ResolveErasureConflict(r.Context(), id, body.Decision, ac.UserID, body.Rationale); err != nil {
		// Surface legal-hold / retention-active conflicts explicitly.
		if errors.Is(err, ErrLegalHoldActive) || errors.Is(err, ErrRetentionActive) {
			httputil.RespondError(w, http.StatusConflict, err.Error())
			return
		}
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
