package notifications

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// Handler exposes notification endpoints over HTTP.
type Handler struct {
	service *Service
}

// NewHandler creates a Handler backed by the given Service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes mounts the notification routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/notifications", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/unread-count", h.UnreadCount)
		r.Post("/read-all", h.MarkAllRead)
		r.Patch("/{id}/read", h.MarkRead)
	})
}

// List returns paginated notifications for the authenticated user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	cursor := r.URL.Query().Get("cursor")

	items, total, err := h.service.GetUserNotifications(r.Context(), ac.UserID, limit, cursor)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hasMore := len(items) == limit
	nextCursor := ""
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = EncodeCursor(last.CreatedAt, last.ID)
	}

	httputil.RespondPaginated(w, http.StatusOK, items, total, nextCursor, hasMore)
}

// UnreadCount returns the number of unread notifications for the authenticated user.
func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	count, err := h.service.GetUnreadCount(r.Context(), ac.UserID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

// MarkAllRead marks all unread notifications as read for the authenticated user.
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.service.MarkAllRead(r.Context(), ac.UserID); err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkRead marks a single notification as read. Returns 403 if the
// notification does not belong to the authenticated user.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid notification ID")
		return
	}

	if err := h.service.MarkRead(r.Context(), id, ac.UserID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusForbidden, "notification not found or does not belong to you")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
