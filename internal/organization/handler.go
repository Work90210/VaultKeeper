package organization

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// OrgCaseLister lists cases for an org. Injected to avoid circular dependency.
type OrgCaseLister interface {
	ListOrgCases(ctx context.Context, orgID uuid.UUID, userID string, isAdmin bool) (any, error)
}

// OrgCaseAssignmentLister lists all case role assignments across an org.
// Injected to avoid circular dependency with the cases package.
type OrgCaseAssignmentLister interface {
	ListOrgCaseAssignments(ctx context.Context, orgID uuid.UUID) (any, error)
}

// InviteEmailer sends invitation emails. Injected to avoid circular dep with notifications.
type InviteEmailer interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

type Handler struct {
	service              *Service
	audit                auth.AuditLogger
	caseLister           OrgCaseLister
	caseAssignmentLister OrgCaseAssignmentLister
	emailer              InviteEmailer
	appURL               string
}

func NewHandler(service *Service, audit auth.AuditLogger) *Handler {
	return &Handler{service: service, audit: audit}
}

func (h *Handler) SetCaseLister(l OrgCaseLister) { h.caseLister = l }

func (h *Handler) SetCaseAssignmentLister(l OrgCaseAssignmentLister) { h.caseAssignmentLister = l }

func (h *Handler) SetInviteEmailer(e InviteEmailer, appURL string) {
	h.emailer = e
	h.appURL = appURL
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/organizations", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Route("/{orgId}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Patch("/", h.Update)
			r.Delete("/", h.Delete)
			r.Get("/members", h.ListMembers)
			r.Patch("/members/{userId}", h.UpdateMemberRole)
			r.Delete("/members/{userId}", h.RemoveMember)
			r.Post("/ownership-transfer", h.TransferOwnership)
			r.Post("/invitations", h.InviteMember)
			r.Get("/invitations", h.ListInvitations)
			r.Delete("/invitations/{inviteId}", h.RevokeInvitation)
			r.Get("/cases", h.ListOrgCases)
			r.Get("/case-assignments", h.ListCaseAssignments)
		})
	})
	r.Post("/api/invitations/accept", h.AcceptInvitation)
	r.Post("/api/invitations/decline", h.DeclineInvitation)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var input CreateOrgInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	org, err := h.service.CreateOrg(r.Context(), ac, input)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, org)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgs, err := h.service.ListUserOrgs(r.Context(), ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, orgs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	org, err := h.service.GetOrg(r.Context(), ac, orgID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, org)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	var input UpdateOrgInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	org, err := h.service.UpdateOrg(r.Context(), ac, orgID, input)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, org)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	if err := h.service.DeleteOrg(r.Context(), ac, orgID); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	members, err := h.service.ListMembers(r.Context(), ac, orgID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, members)
}

func (h *Handler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	targetUserID := chi.URLParam(r, "userId")

	var input UpdateMemberRoleInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateMemberRole(r.Context(), ac, orgID, targetUserID, input.Role); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	targetUserID := chi.URLParam(r, "userId")

	if err := h.service.RemoveMember(r.Context(), ac, orgID, targetUserID); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	var input TransferOwnershipInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.TransferOwnership(r.Context(), ac, orgID, input.TargetUserID); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	var input InviteInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	inv, rawToken, err := h.service.InviteMember(r.Context(), ac, orgID, input)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	// Send invitation email (fire-and-forget, don't block the response).
	if h.emailer != nil && h.appURL != "" {
		org, _ := h.service.orgRepo.GetByID(r.Context(), orgID)
		orgName := org.Name
		if orgName == "" {
			orgName = "your organization"
		}
		inviteURL := h.appURL + "/en/invite?token=" + rawToken
		safeInviteURL := html.EscapeString(inviteURL)

		safeOrgName := html.EscapeString(orgName)
		safeRole := html.EscapeString(string(inv.Role))
		subject := "You've been invited to " + orgName + " on VaultKeeper"
		textBody := "You've been invited to join " + orgName + " on VaultKeeper as " + string(inv.Role) + ".\n\n" +
			"Accept the invitation: " + inviteURL + "\n\n" +
			"This invitation expires in 7 days."
		htmlBody := `<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;max-width:480px;margin:0 auto;padding:32px 0">` +
			`<h2 style="color:#1a1a1a;font-size:20px;margin:0 0 16px">You've been invited to ` + safeOrgName + `</h2>` +
			`<p style="color:#555;font-size:15px;line-height:1.6;margin:0 0 24px">` +
			`You've been invited to join <strong>` + safeOrgName + `</strong> on VaultKeeper as <strong>` + safeRole + `</strong>.</p>` +
			`<a href="` + safeInviteURL + `" style="display:inline-block;padding:12px 24px;background:#b38a4e;color:#fff;text-decoration:none;border-radius:6px;font-weight:600;font-size:14px">Accept Invitation</a>` +
			`<p style="color:#999;font-size:13px;margin:24px 0 0">This invitation expires in 7 days. If you didn't expect this, you can ignore it.</p>` +
			`</div>`

		_ = h.emailer.Send(r.Context(), inv.Email, subject, htmlBody, textBody)
	}

	httputil.RespondJSON(w, http.StatusCreated, map[string]any{
		"invitation": inv,
	})
}

func (h *Handler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	invitations, err := h.service.ListInvitations(r.Context(), ac, orgID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, invitations)
}

func (h *Handler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	inviteID, err := uuid.Parse(chi.URLParam(r, "inviteId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid invitation ID")
		return
	}

	if err := h.service.RevokeInvitation(r.Context(), ac, orgID, inviteID); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var input AcceptInviteInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	org, err := h.service.AcceptInvitation(r.Context(), ac, input.Token)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, org)
}

func (h *Handler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var input AcceptInviteInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.DeclineInvitation(r.Context(), ac, input.Token); err != nil {
		respondServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListOrgCases(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify caller is org member.
	orgRole, err := h.service.authz.GetCallerOrgRole(r.Context(), ac, orgID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	if h.caseLister == nil {
		httputil.RespondError(w, http.StatusNotImplemented, "case lister not configured")
		return
	}

	isAdmin := orgRole == RoleOwner || orgRole == RoleAdmin
	cases, err := h.caseLister.ListOrgCases(r.Context(), orgID, ac.UserID, isAdmin)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list cases")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, cases)
}

func (h *Handler) ListCaseAssignments(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	orgID, err := parseOrgID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid organization ID")
		return
	}

	// Verify caller is an org admin or owner.
	orgRole, err := h.service.authz.GetCallerOrgRole(r.Context(), ac, orgID)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if orgRole != RoleOwner && orgRole != RoleAdmin {
		httputil.RespondError(w, http.StatusForbidden, "org admin access required")
		return
	}

	if h.caseAssignmentLister == nil {
		httputil.RespondError(w, http.StatusNotImplemented, "case assignment lister not configured")
		return
	}

	assignments, err := h.caseAssignmentLister.ListOrgCaseAssignments(r.Context(), orgID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list case assignments")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, assignments)
}

// --- Helpers ---

func parseOrgID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "orgId"))
}

func decodeBody(r *http.Request, dest any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrOrgNotFound):
		httputil.RespondError(w, http.StatusNotFound, "organization not found")
	case errors.Is(err, ErrMembershipNotFound):
		httputil.RespondError(w, http.StatusNotFound, "membership not found")
	case errors.Is(err, ErrInvitationNotFound):
		httputil.RespondError(w, http.StatusNotFound, "invitation not found")
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrNotOrgMember):
		httputil.RespondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrLastOwner):
		httputil.RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrActiveCases):
		httputil.RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrSelfRemove):
		httputil.RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInviteExpired):
		httputil.RespondError(w, http.StatusGone, err.Error())
	case errors.Is(err, ErrInviteNotPending):
		httputil.RespondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrEmailMismatch):
		httputil.RespondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrSlugTaken):
		httputil.RespondError(w, http.StatusConflict, err.Error())
	default:
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
	}
}
