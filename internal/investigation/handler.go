package investigation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

const maxBodySize = 1 << 20 // 1MB

// OrgMembershipChecker verifies whether a user belongs to a case's organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
}

// EvidenceCaseResolver resolves the case that owns an evidence item.
// Used to prevent IDOR when creating records scoped to evidence.
type EvidenceCaseResolver interface {
	GetCaseIDByEvidence(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error)
}

// Handler provides HTTP endpoints for the investigation subsystem.
type Handler struct {
	service              *Service
	audit                auth.AuditLogger
	orgChecker           OrgMembershipChecker                                          // optional — org boundary enforcement
	caseLookupOrg        func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) // returns org ID for a case
	evidenceCaseResolver EvidenceCaseResolver                                          // optional — resolves case from evidence ID
}

// NewHandler creates a new investigation HTTP handler.
func NewHandler(service *Service, audit auth.AuditLogger) *Handler {
	return &Handler{service: service, audit: audit}
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// investigation access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// SetEvidenceCaseResolver wires the evidence-to-case resolver. When set,
// CreateAssessment and CreateVerificationRecord look up the case server-side
// rather than accepting it from the request body, preventing IDOR attacks.
func (h *Handler) SetEvidenceCaseResolver(resolver EvidenceCaseResolver) {
	h.evidenceCaseResolver = resolver
}

// requireOrgMembership verifies that the caller belongs to the organization that
// owns the given case. System admins bypass this check. Returns true if access is
// allowed, false if a 403 was written to the response.
func (h *Handler) requireOrgMembership(w http.ResponseWriter, r *http.Request, ac *auth.AuthContext, caseID uuid.UUID) bool {
	if ac.SystemRole == auth.RoleSystemAdmin {
		return true
	}
	if h.orgChecker == nil || h.caseLookupOrg == nil {
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	orgID, err := h.caseLookupOrg(r.Context(), caseID)
	if err != nil {
		slog.Warn("org lookup failed for case", "case_id", caseID, "error", err)
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	isMember, err := h.orgChecker.IsActiveMember(r.Context(), orgID, ac.UserID)
	if err != nil || !isMember {
		slog.Warn("org membership check failed",
			"case_id", caseID, "org_id", orgID, "user_id", ac.UserID, "error", err)
		httputil.RespondError(w, http.StatusForbidden, "access denied")
		return false
	}
	return true
}

// RegisterRoutes mounts investigation routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Phase 1: Inquiry Logs
	r.Route("/api/cases/{caseID}/inquiry-logs", func(r chi.Router) {
		r.Post("/", h.CreateInquiryLog)
		r.Get("/", h.ListInquiryLogs)
	})
	r.Route("/api/inquiry-logs/{id}", func(r chi.Router) {
		r.Get("/", h.GetInquiryLog)
		r.Put("/", h.UpdateInquiryLog)
		r.Delete("/", h.DeleteInquiryLog)
	})

	// Phase 2: Assessments
	r.Route("/api/evidence/{evidenceID}/assessments", func(r chi.Router) {
		r.Post("/", h.CreateAssessment)
		r.Get("/", h.ListAssessments)
	})
	r.Get("/api/assessments/{id}", h.GetAssessment)
	r.Get("/api/cases/{caseID}/assessments", h.ListAssessmentsByCase)

	// Phase 5: Verification Records
	r.Route("/api/evidence/{evidenceID}/verifications", func(r chi.Router) {
		r.Post("/", h.CreateVerificationRecord)
		r.Get("/", h.ListVerificationRecords)
	})
	r.Get("/api/verifications/{id}", h.GetVerificationRecord)
	r.Get("/api/cases/{caseID}/verifications", h.ListVerificationsByCase)

	// Phase 5: Corroboration
	r.Route("/api/cases/{caseID}/corroborations", func(r chi.Router) {
		r.Post("/", h.CreateCorroborationClaim)
		r.Get("/", h.ListCorroborationClaims)
	})
	r.Route("/api/corroborations/{id}", func(r chi.Router) {
		r.Get("/", h.GetCorroborationClaim)
		r.Post("/evidence", h.AddEvidenceToClaim)
		r.Delete("/evidence/{evidenceID}", h.RemoveEvidenceFromClaim)
	})
	r.Get("/api/evidence/{evidenceID}/corroborations", h.GetClaimsByEvidence)

	// Phase 6: Analysis Notes
	r.Route("/api/cases/{caseID}/analysis-notes", func(r chi.Router) {
		r.Post("/", h.CreateAnalysisNote)
		r.Get("/", h.ListAnalysisNotes)
	})
	r.Route("/api/analysis-notes/{id}", func(r chi.Router) {
		r.Get("/", h.GetAnalysisNote)
		r.Put("/", h.UpdateAnalysisNote)
	})

	// Templates (Annexes 1-3)
	r.Get("/api/templates", h.ListTemplates)
	r.Get("/api/templates/{id}", h.GetTemplate)
	r.Route("/api/cases/{caseID}/template-instances", func(r chi.Router) {
		r.Post("/", h.CreateTemplateInstance)
		r.Get("/", h.ListTemplateInstances)
	})
	r.Route("/api/template-instances/{id}", func(r chi.Router) {
		r.Get("/", h.GetTemplateInstance)
		r.Put("/", h.UpdateTemplateInstance)
	})

	// Reports (R1, R3)
	r.Route("/api/cases/{caseID}/reports", func(r chi.Router) {
		r.Post("/", h.CreateReport)
		r.Get("/", h.ListReports)
	})
	r.Route("/api/reports/{id}", func(r chi.Router) {
		r.Get("/", h.GetReport)
		r.Put("/", h.UpdateReport)
		r.Post("/status", h.TransitionReportStatus)
		r.Post("/publish", h.PublishReport)
	})

	// Safety Profiles (P4, S2)
	r.Route("/api/cases/{caseID}/safety-profiles", func(r chi.Router) {
		r.Get("/", h.ListSafetyProfiles)
		r.Get("/mine", h.GetMySafetyProfile)
		r.Put("/{userID}", h.UpsertSafetyProfile)
	})
}

// --- Inquiry Logs ---

func (h *Handler) CreateInquiryLog(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	var input InquiryLogInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.CreateInquiryLog(r.Context(), caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListInquiryLogs(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	limit, offset := parsePagination(r)
	logs, total, err := h.service.ListInquiryLogs(r.Context(), caseID, limit, offset, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondPaginated(w, http.StatusOK, logs, total, "", false)
}

func (h *Handler) GetInquiryLog(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	log, err := h.service.GetInquiryLog(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, log.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, log)
}

func (h *Handler) UpdateInquiryLog(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetInquiryLog(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	var input InquiryLogInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.service.UpdateInquiryLog(r.Context(), id, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteInquiryLog(w http.ResponseWriter, r *http.Request) {
	httputil.RespondError(w, http.StatusNotImplemented, "not implemented")
}

// --- Assessments ---

func (h *Handler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	evidenceID, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Resolve case_id server-side from the evidence item to prevent IDOR.
	if h.evidenceCaseResolver == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "service not configured")
		return
	}
	caseID, err := h.evidenceCaseResolver.GetCaseIDByEvidence(r.Context(), evidenceID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}

	var input AssessmentInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.service.CreateAssessment(r.Context(), evidenceID, caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	evidenceID, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}
	if h.evidenceCaseResolver == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "evidence case resolver not configured")
		return
	}
	caseID, err := h.evidenceCaseResolver.GetCaseIDByEvidence(r.Context(), evidenceID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	assessments, err := h.service.GetAssessmentsByEvidence(r.Context(), caseID, evidenceID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, assessments)
}

func (h *Handler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	assessment, err := h.service.GetAssessment(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, assessment.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, assessment)
}

// --- Verification Records ---

func (h *Handler) CreateVerificationRecord(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	evidenceID, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Resolve case_id server-side from the evidence item to prevent IDOR.
	if h.evidenceCaseResolver == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "service not configured")
		return
	}
	caseID, err := h.evidenceCaseResolver.GetCaseIDByEvidence(r.Context(), evidenceID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}

	var input VerificationRecordInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve the caller's case role (e.g. "investigator", "prosecutor") so that the
	// service can enforce case-level permissions rather than system-level roles.
	caseRole, err := h.service.GetCaseRole(r.Context(), caseID, ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient role")
		return
	}

	created, err := h.service.CreateVerificationRecord(r.Context(), evidenceID, caseID, input, ac.UserID, caseRole)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListVerificationRecords(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	evidenceID, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}
	if h.evidenceCaseResolver == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "service not configured")
		return
	}
	caseID, err := h.evidenceCaseResolver.GetCaseIDByEvidence(r.Context(), evidenceID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	records, err := h.service.ListVerificationRecords(r.Context(), evidenceID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, records)
}

func (h *Handler) GetVerificationRecord(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	record, err := h.service.GetVerificationRecord(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, record.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, record)
}

// --- Corroboration ---

func (h *Handler) CreateCorroborationClaim(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	var input CorroborationClaimInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.CreateCorroborationClaim(r.Context(), caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListCorroborationClaims(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	claims, err := h.service.ListCorroborationClaims(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, claims)
}

func (h *Handler) GetCorroborationClaim(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	claim, err := h.service.GetCorroborationClaim(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, claim.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, claim)
}

func (h *Handler) AddEvidenceToClaim(w http.ResponseWriter, r *http.Request) {
	httputil.RespondError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handler) RemoveEvidenceFromClaim(w http.ResponseWriter, r *http.Request) {
	httputil.RespondError(w, http.StatusNotImplemented, "not implemented")
}

func (h *Handler) GetClaimsByEvidence(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	evidenceID, err := uuid.Parse(chi.URLParam(r, "evidenceID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}
	if h.evidenceCaseResolver == nil {
		httputil.RespondError(w, http.StatusInternalServerError, "service not configured")
		return
	}
	caseID, err := h.evidenceCaseResolver.GetCaseIDByEvidence(r.Context(), evidenceID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	claims, err := h.service.GetClaimsByEvidence(r.Context(), evidenceID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, claims)
}

// --- Analysis Notes ---

func (h *Handler) CreateAnalysisNote(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	var input AnalysisNoteInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.CreateAnalysisNote(r.Context(), caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListAnalysisNotes(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	limit, offset := parsePagination(r)
	notes, total, err := h.service.ListAnalysisNotes(r.Context(), caseID, limit, offset, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondPaginated(w, http.StatusOK, notes, total, "", false)
}

func (h *Handler) GetAnalysisNote(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	note, err := h.service.GetAnalysisNote(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, note.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, note)
}

func (h *Handler) UpdateAnalysisNote(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetAnalysisNote(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	var input AnalysisNoteInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.service.UpdateAnalysisNote(r.Context(), id, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, updated)
}

// --- Templates ---

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if requireAuth(w, r) == nil {
		return
	}
	templateType := r.URL.Query().Get("type")
	templates, err := h.service.ListTemplates(r.Context(), templateType)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, templates)
}

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	if requireAuth(w, r) == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	tmpl, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, tmpl)
}

func (h *Handler) CreateTemplateInstance(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	var input TemplateInstanceInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.CreateTemplateInstance(r.Context(), caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListTemplateInstances(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	instances, err := h.service.ListTemplateInstances(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, instances)
}

func (h *Handler) GetTemplateInstance(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	inst, err := h.service.GetTemplateInstance(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, inst.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, inst)
}

func (h *Handler) UpdateTemplateInstance(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetTemplateInstance(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	var body struct {
		Content map[string]any `json:"content"`
		Status  string         `json:"status"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ValidateTemplateInstanceStatusInput(TemplateInstanceStatusInput{Status: body.Status}); err != nil {
		respondError(w, err)
		return
	}
	updated, err := h.service.UpdateTemplateInstance(r.Context(), id, body.Content, body.Status, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, updated)
}

// --- Reports ---

func (h *Handler) CreateReport(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	var input ReportInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.CreateReport(r.Context(), caseID, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, created)
}

func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	reports, err := h.service.ListReports(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, reports)
}

func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	report, err := h.service.GetReport(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, report.CaseID) {
		return
	}
	httputil.RespondJSON(w, http.StatusOK, report)
}

func (h *Handler) PublishReport(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetReport(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	// Resolve the caller's case role so the service enforces case-level permissions.
	publishCaseRole, err := h.service.GetCaseRole(r.Context(), existing.CaseID, ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient role")
		return
	}
	published, err := h.service.PublishReport(r.Context(), id, ac.UserID, publishCaseRole)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, published)
}

// --- Safety Profiles ---

func (h *Handler) ListSafetyProfiles(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	// CRIT-02: enforce case role — only prosecutor/judge/investigator can list safety profiles.
	// Must load the case role (not system role) because CanReadSafetyProfile checks
	// against case-level roles ("prosecutor", "judge", "investigator").
	caseRole, err := h.service.GetCaseRole(r.Context(), caseID, ac.UserID)
	if err != nil || !CanReadSafetyProfile(caseRole) {
		slog.Warn("safety profile list access denied",
			"actor", ac.UserID, "case_id", caseID, "case_role", caseRole)
		httputil.RespondError(w, http.StatusForbidden, "insufficient role")
		return
	}
	profiles, err := h.service.ListSafetyProfiles(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, profiles)
}

func (h *Handler) GetMySafetyProfile(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	userID, err := uuid.Parse(ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "invalid user ID")
		return
	}
	profile, err := h.service.GetSafetyProfile(r.Context(), caseID, userID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, profile)
}

func (h *Handler) UpsertSafetyProfile(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, &ac, caseID) {
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid user ID")
		return
	}
	var input SafetyProfileInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// CRIT-02: load the caller's CASE role so CanWriteSafetyProfile can compare
	// against "prosecutor"/"judge" rather than system roles.
	actorCaseRole, err := h.service.GetCaseRole(r.Context(), caseID, ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient role")
		return
	}
	profile, warnings, err := h.service.UpsertSafetyProfile(r.Context(), caseID, userID, input, ac.UserID, actorCaseRole)
	if err != nil {
		respondError(w, err)
		return
	}
	response := map[string]any{"data": profile}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}
	httputil.RespondJSON(w, http.StatusOK, response)
}

func (h *Handler) ListAssessmentsByCase(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	assessments, err := h.service.ListAssessmentsByCase(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, assessments)
}

func (h *Handler) ListVerificationsByCase(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}
	if !h.requireOrgMembership(w, r, ac, caseID) {
		return
	}
	verifications, err := h.service.ListVerificationsByCase(r.Context(), caseID, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, verifications)
}

func (h *Handler) TransitionReportStatus(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetReport(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Resolve the caller's case role so the service enforces case-level permissions.
	transitionCaseRole, err := h.service.GetCaseRole(r.Context(), existing.CaseID, ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient role")
		return
	}
	updated, err := h.service.TransitionReportStatus(r.Context(), id, body.Status, ac.UserID, transitionCaseRole)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, updated)
}

func (h *Handler) UpdateReport(w http.ResponseWriter, r *http.Request) {
	ac := requireAuth(w, r)
	if ac == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid ID")
		return
	}
	existing, err := h.service.GetReport(r.Context(), id, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !h.requireOrgMembership(w, r, ac, existing.CaseID) {
		return
	}
	var input ReportInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.service.UpdateReportContent(r.Context(), id, input, ac.UserID)
	if err != nil {
		respondError(w, err)
		return
	}
	httputil.RespondJSON(w, http.StatusOK, updated)
}

// --- Helpers ---

// requireAuth extracts auth context or writes 401. Returns nil if unauthorized.
func requireAuth(w http.ResponseWriter, r *http.Request) *auth.AuthContext {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return nil
	}
	return &ac
}

func decodeBody(r *http.Request, dst any) error {
	limited := io.LimitReader(r.Body, maxBodySize+1)
	dec := json.NewDecoder(limited)
	if err := dec.Decode(dst); err != nil {
		return &ValidationError{Field: "body", Message: "invalid request body"}
	}
	// Check if body exceeded the limit
	if dec.More() {
		return &ValidationError{Field: "body", Message: "request body exceeds size limit"}
	}
	return nil
}

const maxPageSize = 200

func parsePagination(r *http.Request) (int, int) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 && parsed < 1000000 {
			offset = parsed
		}
	}
	return limit, offset
}

func respondError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		httputil.RespondError(w, http.StatusNotFound, "not found")
		return
	}
	httputil.RespondError(w, http.StatusInternalServerError, "internal error")
}
