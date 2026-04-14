package apikeys

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

// Handler serves the API key management endpoints.
type Handler struct {
	repo  Repository
	audit auth.AuditLogger
}

// NewHandler creates a new API key handler.
func NewHandler(repo Repository, audit auth.AuditLogger) *Handler {
	return &Handler{repo: repo, audit: audit}
}

// RegisterRoutes implements server.RouteRegistrar.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/settings/api-keys", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Delete("/{id}", h.Revoke)
	})
}

// List returns all non-revoked API keys for the authenticated user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keys, err := h.repo.ListByUser(r.Context(), ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, keys)
}

// Create generates a new API key and returns the raw key exactly once.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	var input CreateKeyInput
	if err := json.Unmarshal(body, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(input.Name) > 100 {
		httputil.RespondError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return
	}

	if input.Permissions != "read" && input.Permissions != "read_write" {
		httputil.RespondError(w, http.StatusBadRequest, "permissions must be 'read' or 'read_write'")
		return
	}

	result, err := h.repo.Create(r.Context(), ac.UserID, input)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, result)
}

// Revoke soft-deletes an API key by setting revoked_at.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	err = h.repo.Revoke(r.Context(), keyID, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "api key not found")
			return
		}
		if errors.Is(err, ErrNotOwner) {
			httputil.RespondError(w, http.StatusForbidden, "not authorized to revoke this key")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "failed to revoke api key")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
