package witnesses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// Handler provides HTTP endpoints for witness operations.
type Handler struct {
	service       *Service
	roleLoader    auth.CaseRoleLoader
	audit         auth.AuditLogger
	rotationJob   *KeyRotationJob
	orgChecker    OrgMembershipChecker
	caseLookupOrg func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
}

// NewHandler creates a new witness HTTP handler.
func NewHandler(service *Service, roleLoader auth.CaseRoleLoader, audit auth.AuditLogger) *Handler {
	return &Handler{
		service:    service,
		roleLoader: roleLoader,
		audit:      audit,
	}
}

// RegisterRoutes mounts witness routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{caseID}/witnesses", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
	})

	r.Route("/api/witnesses/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Patch("/", h.Update)
	})

	r.Route("/api/admin/witness-keys", func(r chi.Router) {
		r.Post("/rotate", h.RotateKeys)
		r.Get("/rotate/progress", h.RotateKeysProgress)
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	// Check case role — must be investigator or prosecutor
	caseRole, err := h.getCaseRole(r.Context(), caseID.String(), ac.UserID, ac.SystemRole)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	if caseRole != auth.CaseRoleInvestigator && caseRole != auth.CaseRoleProsecutor {
		if h.audit != nil {
			h.audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "investigator", string(caseRole), ac.IPAddress)
		}
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var body struct {
		WitnessCode      string      `json:"witness_code"`
		FullName         *string     `json:"full_name"`
		ContactInfo      *string     `json:"contact_info"`
		Location         *string     `json:"location"`
		ProtectionStatus string      `json:"protection_status"`
		StatementSummary string      `json:"statement_summary"`
		RelatedEvidence  []uuid.UUID `json:"related_evidence"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := CreateWitnessInput{
		CaseID:           caseID,
		WitnessCode:      body.WitnessCode,
		FullName:         body.FullName,
		ContactInfo:      body.ContactInfo,
		Location:         body.Location,
		ProtectionStatus: body.ProtectionStatus,
		StatementSummary: body.StatementSummary,
		RelatedEvidence:  body.RelatedEvidence,
		CreatedBy:        ac.UserID,
	}

	view, err := h.service.Create(r.Context(), input)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, view)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	caseRole, err := h.getCaseRole(r.Context(), caseID.String(), ac.UserID, ac.SystemRole)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	page := parsePagination(r)
	views, total, err := h.service.List(r.Context(), caseID, caseRole, page)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	var nextCursor string
	if len(views) == page.Limit && len(views) > 0 {
		nextCursor = encodeCursor(views[len(views)-1].ID)
	}

	httputil.RespondPaginated(w, http.StatusOK, views, total, nextCursor, len(views) == page.Limit)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid witness ID")
		return
	}

	// Look up the witness case_id via service layer for role check
	witnessCaseID, err := h.service.GetCaseID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseRole, err := h.getCaseRole(r.Context(), witnessCaseID.String(), ac.UserID, ac.SystemRole)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	view, err := h.service.Get(r.Context(), id, caseRole)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, view)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid witness ID")
		return
	}

	// Look up witness case_id via service layer for role check
	witnessCaseID, err := h.service.GetCaseID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseRole, err := h.getCaseRole(r.Context(), witnessCaseID.String(), ac.UserID, ac.SystemRole)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	if caseRole != auth.CaseRoleInvestigator && caseRole != auth.CaseRoleProsecutor {
		if h.audit != nil {
			h.audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "investigator", string(caseRole), ac.IPAddress)
		}
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	var input UpdateWitnessInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	view, err := h.service.Update(r.Context(), id, input, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, view)
}

// getCaseRole resolves the user's case role, returning investigator for system admins.
func (h *Handler) getCaseRole(ctx context.Context, caseID, userID string, systemRole auth.SystemRole) (auth.CaseRole, error) {
	if systemRole >= auth.RoleSystemAdmin {
		return auth.CaseRoleInvestigator, nil
	}

	// Org membership gate: verify the caller belongs to the case's org.
	if h.orgChecker != nil && h.caseLookupOrg != nil {
		parsedCaseID, parseErr := uuid.Parse(caseID)
		if parseErr != nil {
			return "", fmt.Errorf("invalid case ID: %w", parseErr)
		}
		orgID, err := h.caseLookupOrg(ctx, parsedCaseID)
		if err != nil {
			return "", fmt.Errorf("lookup org for case: %w", err)
		}
		isMember, err := h.orgChecker.IsActiveMember(ctx, orgID, userID)
		if err != nil {
			return "", fmt.Errorf("org membership check: %w", err)
		}
		if !isMember {
			return "", auth.ErrNoCaseRole
		}
	}

	return h.roleLoader.LoadCaseRole(ctx, caseID, userID)
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// witness access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// SetRotationJob sets the key rotation job on the handler.
func (h *Handler) SetRotationJob(job *KeyRotationJob) {
	h.rotationJob = job
}

func (h *Handler) RotateKeys(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if ac.SystemRole < auth.RoleSystemAdmin {
		if h.audit != nil {
			h.audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "system_admin", ac.SystemRole.String(), ac.IPAddress)
		}
		httputil.RespondError(w, http.StatusForbidden, "system admin required")
		return
	}

	if h.rotationJob == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "key rotation not configured")
		return
	}

	progress := h.rotationJob.Progress()
	if progress.Running {
		httputil.RespondError(w, http.StatusConflict, "rotation already in progress")
		return
	}

	go h.rotationJob.Run(r.Context())

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{"status": "rotation started"})
}

func (h *Handler) RotateKeysProgress(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if ac.SystemRole < auth.RoleSystemAdmin {
		httputil.RespondError(w, http.StatusForbidden, "system admin required")
		return
	}

	if h.rotationJob == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "key rotation not configured")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, h.rotationJob.Progress())
}

func decodeBody(r *http.Request, dst any) error {
	limited := io.LimitReader(r.Body, MaxBodySize+1)
	decoder := json.NewDecoder(limited)

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected end") {
			return &ValidationError{Field: "body", Message: "request body too large"}
		}
		return &ValidationError{Field: "body", Message: "invalid JSON"}
	}

	return nil
}

func parsePagination(r *http.Request) Pagination {
	limit := DefaultPageLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	p := Pagination{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	}
	return ClampPagination(p)
}

func respondServiceError(w http.ResponseWriter, err error) {
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
