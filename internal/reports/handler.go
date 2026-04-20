package reports

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// OrgMembershipChecker verifies whether a user belongs to a case's organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
}

// Handler serves PDF report downloads.
type Handler struct {
	generator          *CustodyReportGenerator
	audit              auth.AuditLogger
	orgChecker         OrgMembershipChecker
	caseLookupOrg      func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
	evidenceCaseLookup func(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error) // returns case ID for an evidence item
}

// NewHandler creates a new reports handler.
func NewHandler(generator *CustodyReportGenerator, audit auth.AuditLogger) *Handler {
	return &Handler{generator: generator, audit: audit}
}

// SetOrgMembershipChecker wires the org membership checker. When set, report
// access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(
	checker OrgMembershipChecker,
	caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error),
	evidenceCaseLookup func(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error),
) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
	h.evidenceCaseLookup = evidenceCaseLookup
}

// ensureOrgMembership verifies that the caller belongs to the case's org.
// Returns true if the caller may proceed.
func (h *Handler) ensureOrgMembership(ctx context.Context, caseID uuid.UUID) bool {
	if h.orgChecker == nil || h.caseLookupOrg == nil {
		return false // not wired — deny access (fail closed)
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

// RegisterRoutes mounts report endpoints on the router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Get("/api/evidence/{id}/custody/export", h.ExportEvidenceCustody)
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Get("/api/cases/{id}/custody/export", h.ExportCaseCustody)
}

// ExportCaseCustody generates and serves a custody chain PDF for an entire case.
func (h *Handler) ExportCaseCustody(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	if !h.ensureOrgMembership(r.Context(), caseID) {
		httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	refCode, err := h.generator.GetReferenceCode(r.Context(), caseID)
	if err != nil {
		slog.Error("failed to load case reference code", "case_id", caseID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to generate report")
		return
	}

	pdfBytes, err := h.generator.GenerateCustodyPDF(r.Context(), caseID)
	if err != nil {
		slog.Error("failed to generate case custody PDF", "case_id", caseID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to generate report")
		return
	}

	filename := fmt.Sprintf("custody-report-%s.pdf", sanitizeFilename(refCode))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	if _, err := w.Write(pdfBytes); err != nil {
		slog.Error("failed to write custody PDF response", "case_id", caseID, "error", err)
	}
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer(`"`, "", `\`, "", "\r", "", "\n", "", ";", "", "=", "", "/", "-")
	return r.Replace(s)
}

// ExportEvidenceCustody generates and serves a custody chain PDF for a single evidence item.
func (h *Handler) ExportEvidenceCustody(w http.ResponseWriter, r *http.Request) {
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Org membership gate: resolve evidence → case → org.
	if h.orgChecker == nil || h.evidenceCaseLookup == nil {
		httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}
	{
		caseID, err := h.evidenceCaseLookup(r.Context(), evidenceID)
		if err != nil {
			httputil.RespondError(w, http.StatusNotFound, "evidence not found")
			return
		}
		if !h.ensureOrgMembership(r.Context(), caseID) {
			httputil.RespondError(w, http.StatusForbidden, "not a member of this organization")
			return
		}
	}

	pdfBytes, err := h.generator.GenerateEvidenceCustodyPDF(r.Context(), evidenceID)
	if err != nil {
		slog.Error("failed to generate evidence custody PDF", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to generate report")
		return
	}

	filename := fmt.Sprintf("evidence-custody-%s.pdf", evidenceID)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	if _, err := w.Write(pdfBytes); err != nil {
		slog.Error("failed to write evidence custody PDF response", "evidence_id", evidenceID, "error", err)
	}
}
