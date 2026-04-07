package reports

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// Handler serves PDF report downloads.
type Handler struct {
	generator *CustodyReportGenerator
	audit     auth.AuditLogger
}

// NewHandler creates a new reports handler.
func NewHandler(generator *CustodyReportGenerator, audit auth.AuditLogger) *Handler {
	return &Handler{generator: generator, audit: audit}
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

	filename := fmt.Sprintf("custody-report-%s.pdf", refCode)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	if _, err := w.Write(pdfBytes); err != nil {
		slog.Error("failed to write custody PDF response", "case_id", caseID, "error", err)
	}
}

// ExportEvidenceCustody generates and serves a custody chain PDF for a single evidence item.
func (h *Handler) ExportEvidenceCustody(w http.ResponseWriter, r *http.Request) {
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
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
