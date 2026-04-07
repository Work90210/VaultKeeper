package backup

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// Handler exposes admin endpoints for backup management.
type Handler struct {
	runner *BackupRunner
	logger *slog.Logger
	audit  auth.AuditLogger
}

// NewHandler creates a backup Handler.
func NewHandler(runner *BackupRunner, logger *slog.Logger, audit auth.AuditLogger) *Handler {
	return &Handler{
		runner: runner,
		logger: logger,
		audit:  audit,
	}
}

// RegisterRoutes mounts backup admin routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/admin/backups", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Post("/run", h.RunBackup)
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Get("/", h.ListBackups)
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Get("/{id}/verify", h.VerifyBackup)
	})
}

// RunBackup triggers an immediate backup and returns the result.
func (h *Handler) RunBackup(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.logger.Info("manual backup triggered", "user_id", ac.UserID)

	result, err := h.runner.RunBackup(r.Context())
	if err != nil {
		h.logger.Error("manual backup failed", "user_id", ac.UserID, "error", err, "detail", result.ErrorMessage)
		httputil.RespondError(w, http.StatusInternalServerError, "backup failed")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"id":           result.ID,
		"status":       result.Status,
		"started_at":   result.StartedAt,
		"completed_at": result.CompletedAt,
		"file_count":   result.FileCount,
		"total_size":   result.TotalSize,
	})
}

// VerifyBackup checks that a backup file exists and its checksum is valid.
func (h *Handler) VerifyBackup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	backupID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid backup ID")
		return
	}

	if err := h.runner.VerifyBackup(r.Context(), backupID); err != nil {
		h.logger.Error("backup verification failed", "backup_id", backupID, "error", err)
		httputil.RespondError(w, http.StatusUnprocessableEntity, "verification failed")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"id":       backupID,
		"verified": true,
	})
}

// ListBackups returns the history of backup runs.
func (h *Handler) ListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.runner.ListBackups(r.Context())
	if err != nil {
		h.logger.Error("list backups failed", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to list backups")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, backups)
}
