package disclosures

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

// Handler provides HTTP endpoints for disclosure operations.
type Handler struct {
	service       *Service
	roleLoader    auth.CaseRoleLoader
	audit         auth.AuditLogger
	orgChecker    OrgMembershipChecker
	caseLookupOrg func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
}

// NewHandler creates a new disclosure HTTP handler.
func NewHandler(service *Service, roleLoader auth.CaseRoleLoader, audit auth.AuditLogger) *Handler {
	return &Handler{
		service:    service,
		roleLoader: roleLoader,
		audit:      audit,
	}
}

// RegisterRoutes mounts disclosure routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{caseID}/disclosures", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
	})

	r.Route("/api/disclosures/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
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

	// Only prosecutors can create disclosures
	caseRole, err := h.getCaseRole(r.Context(), caseID.String(), ac.UserID, ac.SystemRole)
	if err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	if caseRole != auth.CaseRoleProsecutor {
		if h.audit != nil {
			h.audit.LogAccessDenied(r.Context(), ac.UserID, r.URL.Path, "prosecutor", string(caseRole), ac.IPAddress)
		}
		httputil.RespondError(w, http.StatusForbidden, "only prosecutors can create disclosures")
		return
	}

	var body struct {
		EvidenceIDs []uuid.UUID `json:"evidence_ids"`
		DisclosedTo string      `json:"disclosed_to"`
		Notes       string      `json:"notes"`
		Redacted    bool        `json:"redacted"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := CreateDisclosureInput{
		CaseID:      caseID,
		EvidenceIDs: body.EvidenceIDs,
		DisclosedTo: body.DisclosedTo,
		Notes:       body.Notes,
		Redacted:    body.Redacted,
	}

	disclosure, err := h.service.Create(r.Context(), input, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, disclosure)
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

	if _, err := h.getCaseRole(r.Context(), caseID.String(), ac.UserID, ac.SystemRole); err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	page := parsePagination(r)
	disclosures, total, err := h.service.List(r.Context(), caseID, page)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	var nextCursor string
	if len(disclosures) == page.Limit && len(disclosures) > 0 {
		nextCursor = encodeCursor(disclosures[len(disclosures)-1].ID)
	}

	httputil.RespondPaginated(w, http.StatusOK, disclosures, total, nextCursor, len(disclosures) == page.Limit)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid disclosure ID")
		return
	}

	disclosure, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	if _, err := h.getCaseRole(r.Context(), disclosure.CaseID.String(), ac.UserID, ac.SystemRole); err != nil {
		httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, disclosure)
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// disclosure access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// getCaseRole resolves the user's case role.
func (h *Handler) getCaseRole(ctx context.Context, caseID, userID string, systemRole auth.SystemRole) (auth.CaseRole, error) {
	if systemRole >= auth.RoleSystemAdmin {
		return auth.CaseRoleProsecutor, nil
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
