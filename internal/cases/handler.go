package cases

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

type Handler struct {
	service *Service
	audit   auth.AuditLogger
}

func NewHandler(service *Service, audit auth.AuditLogger) *Handler {
	return &Handler{service: service, audit: audit}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/", h.Create)
		r.Get("/", h.List)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Patch("/", h.Update)
			r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/archive", h.Archive)
			r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/legal-hold", h.SetLegalHold)
		})
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var input CreateCaseInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.CreateCase(r.Context(), input, ac.UserID, ac.Username)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, result)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	filter := CaseFilter{
		UserID:       ac.UserID,
		SystemAdmin:  ac.SystemRole >= auth.RoleSystemAdmin,
		Jurisdiction: r.URL.Query().Get("jurisdiction"),
		SearchQuery:  r.URL.Query().Get("q"),
	}
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		filter.Status = strings.Split(statusParam, ",")
	}

	page := parsePagination(r)

	result, err := h.service.ListCases(r.Context(), filter, page)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondPaginated(w, http.StatusOK, result.Items, result.TotalCount, result.NextCursor, result.HasMore)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	c, err := h.service.GetCase(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, c)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := parseUUID(r, "id")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	var input UpdateCaseInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.UpdateCase(r.Context(), id, input, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := parseUUID(r, "id")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	if err := h.service.ArchiveCase(r.Context(), id, ac.UserID); err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (h *Handler) SetLegalHold(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := parseUUID(r, "id")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	var body struct {
		Hold bool `json:"hold"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.SetLegalHold(r.Context(), id, body.Hold, ac.UserID); err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]bool{"legal_hold": body.Hold})
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

	// Check if there's more data (body exceeded limit)
	var extra json.RawMessage
	if decoder.More() {
		if err := decoder.Decode(&extra); err == nil {
			return &ValidationError{Field: "body", Message: "request body too large"}
		}
	}

	return nil
}

func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	return uuid.Parse(raw)
}

func parsePagination(r *http.Request) Pagination {
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		for _, c := range l {
			if c >= '0' && c <= '9' {
				limit = limit*10 + int(c-'0')
			} else {
				limit = DefaultPageLimit
				break
			}
		}
	}
	return Pagination{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	}
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
	if strings.Contains(err.Error(), "reference code already exists") {
		httputil.RespondError(w, http.StatusConflict, "reference code already exists")
		return
	}
	httputil.RespondError(w, http.StatusInternalServerError, "internal error")
}
