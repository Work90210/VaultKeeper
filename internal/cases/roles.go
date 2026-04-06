package cases

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

type RoleRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

func (r *RoleRepository) Assign(ctx context.Context, caseID uuid.UUID, userID, role, grantedBy string) (CaseRole, error) {
	var cr CaseRole
	err := r.pool.QueryRow(ctx,
		`INSERT INTO case_roles (case_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, case_id, user_id, role, granted_by, granted_at`,
		caseID, userID, role, grantedBy,
	).Scan(&cr.ID, &cr.CaseID, &cr.UserID, &cr.Role, &cr.GrantedBy, &cr.GrantedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return CaseRole{}, fmt.Errorf("role already assigned: %w", err)
		}
		return CaseRole{}, fmt.Errorf("assign case role: %w", err)
	}
	return cr, nil
}

func (r *RoleRepository) Revoke(ctx context.Context, caseID uuid.UUID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM case_roles WHERE case_id = $1 AND user_id = $2`,
		caseID, userID,
	)
	if err != nil {
		return fmt.Errorf("revoke case role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RoleRepository) ListByCaseID(ctx context.Context, caseID uuid.UUID) ([]CaseRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, case_id, user_id, role, granted_by, granted_at
		 FROM case_roles WHERE case_id = $1 ORDER BY granted_at`,
		caseID,
	)
	if err != nil {
		return nil, fmt.Errorf("list case roles: %w", err)
	}
	defer rows.Close()

	var roles []CaseRole
	for rows.Next() {
		var cr CaseRole
		if err := rows.Scan(&cr.ID, &cr.CaseID, &cr.UserID, &cr.Role, &cr.GrantedBy, &cr.GrantedAt); err != nil {
			return nil, fmt.Errorf("scan case role: %w", err)
		}
		roles = append(roles, cr)
	}
	return roles, rows.Err()
}

func (r *RoleRepository) LoadCaseRole(ctx context.Context, caseID, userID string) (auth.CaseRole, error) {
	cid, err := uuid.Parse(caseID)
	if err != nil {
		return "", fmt.Errorf("parse case ID: %w", err)
	}
	var role string
	err = r.pool.QueryRow(ctx,
		`SELECT role FROM case_roles WHERE case_id = $1 AND user_id = $2 LIMIT 1`,
		cid, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", auth.ErrNoCaseRole
		}
		return "", fmt.Errorf("load case role: %w", err)
	}
	return auth.CaseRole(role), nil
}

type RoleHandler struct {
	roles   *RoleRepository
	custody CustodyRecorder
	audit   auth.AuditLogger
}

func NewRoleHandler(roles *RoleRepository, custody CustodyRecorder, audit auth.AuditLogger) *RoleHandler {
	return &RoleHandler{roles: roles, custody: custody, audit: audit}
}

func (h *RoleHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{id}/roles", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/", h.Assign)
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Delete("/{userId}", h.Revoke)
		r.Get("/", h.List)
	})
}

func (h *RoleHandler) Assign(w http.ResponseWriter, r *http.Request) {
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

	var input AssignRoleInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if !ValidCaseRoles[input.Role] {
		httputil.RespondError(w, http.StatusBadRequest, "invalid role")
		return
	}

	if input.UserID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	cr, err := h.roles.Assign(r.Context(), caseID, input.UserID, input.Role, ac.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "role already assigned") {
			httputil.RespondError(w, http.StatusConflict, "role already assigned")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.custody != nil {
		_ = h.custody.RecordCaseEvent(r.Context(), caseID, "role_granted", ac.UserID, map[string]string{
			"user_id":    input.UserID,
			"role":       input.Role,
			"granted_by": ac.UserID,
		})
	}

	httputil.RespondJSON(w, http.StatusCreated, cr)
}

func (h *RoleHandler) Revoke(w http.ResponseWriter, r *http.Request) {
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

	targetUserID := chi.URLParam(r, "userId")
	if targetUserID == ac.UserID {
		httputil.RespondError(w, http.StatusForbidden, "cannot remove your own role")
		return
	}

	// Get the role before revoking for custody logging
	roles, err := h.roles.ListByCaseID(r.Context(), caseID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	var revokedRole string
	for _, cr := range roles {
		if cr.UserID == targetUserID {
			revokedRole = cr.Role
			break
		}
	}

	if err := h.roles.Revoke(r.Context(), caseID, targetUserID); err != nil {
		if err == ErrNotFound {
			httputil.RespondError(w, http.StatusNotFound, "role assignment not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.custody != nil {
		_ = h.custody.RecordCaseEvent(r.Context(), caseID, "role_revoked", ac.UserID, map[string]string{
			"user_id":    targetUserID,
			"role":       revokedRole,
			"revoked_by": ac.UserID,
		})
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	roles, err := h.roles.ListByCaseID(r.Context(), caseID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, roles)
}
