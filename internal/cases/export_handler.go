package cases

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// OrgMembershipChecker verifies whether a user belongs to a case's organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
}

// ExportHandler serves case export ZIP downloads.
type ExportHandler struct {
	exportService *ExportService
	audit         auth.AuditLogger
	orgChecker    OrgMembershipChecker
	caseLookupOrg func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
}

// NewExportHandler creates a new export handler.
func NewExportHandler(exportService *ExportService, audit auth.AuditLogger) *ExportHandler {
	return &ExportHandler{exportService: exportService, audit: audit}
}

// SetOrgMembershipChecker wires the org membership checker. When set, case
// export requires that the caller is a member of the case's organization.
func (h *ExportHandler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// ensureOrgMembership verifies that the caller belongs to the case's org.
func (h *ExportHandler) ensureOrgMembership(ctx context.Context, caseID uuid.UUID) bool {
	if h.orgChecker == nil || h.caseLookupOrg == nil {
		return true
	}
	ac, ok := auth.GetAuthContext(ctx)
	if !ok {
		return false
	}
	if ac.SystemRole == auth.RoleSystemAdmin {
		return true
	}
	orgID, err := h.caseLookupOrg(ctx, caseID)
	if err != nil {
		return false
	}
	isMember, err := h.orgChecker.IsActiveMember(ctx, orgID, ac.UserID)
	return err == nil && isMember
}

// RegisterRoutes mounts the export endpoint on the router.
func (h *ExportHandler) RegisterRoutes(r chi.Router) {
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Get("/api/cases/{id}/export", h.ExportCase)
}

// ExportCase streams a ZIP archive of the case to the HTTP response.
func (h *ExportHandler) ExportCase(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	if !h.ensureOrgMembership(r.Context(), caseID) {
		httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	// Determine the user's case role for filtering (defence sees only disclosed)
	userRole := ""
	if role, ok := auth.GetCaseRole(r.Context()); ok {
		userRole = string(role)
	}

	refCode, err := h.exportService.GetReferenceCode(r.Context(), caseID)
	if err != nil {
		slog.Error("failed to get reference code for export", "case_id", caseID, "error", err)
		httputil.RespondError(w, http.StatusNotFound, "case not found")
		return
	}

	filename := fmt.Sprintf("%s-export.zip", refCode)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := h.exportService.ExportCase(r.Context(), caseID, userRole, ac.UserID, w); err != nil {
		slog.Error("case export failed", "case_id", caseID, "error", err)
		// Headers already sent; we can't change the status code.
		// The incomplete ZIP will signal the error to the client.
		return
	}
}
