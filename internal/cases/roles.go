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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// CaseRoleStore is the interface the RoleHandler depends on. RoleRepository
// implements it; tests can substitute a lightweight mock.
type CaseRoleStore interface {
	Assign(ctx context.Context, caseID uuid.UUID, userID, role, grantedBy string, roleDefinitionID *uuid.UUID) (CaseRole, error)
	Revoke(ctx context.Context, caseID uuid.UUID, userID string) error
	ListByCaseID(ctx context.Context, caseID uuid.UUID) ([]CaseRole, error)
}

type roleDBPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type RoleRepository struct {
	pool roleDBPool
}

func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

func (r *RoleRepository) Assign(ctx context.Context, caseID uuid.UUID, userID, role, grantedBy string, roleDefinitionID *uuid.UUID) (CaseRole, error) {
	var cr CaseRole
	err := r.pool.QueryRow(ctx,
		`INSERT INTO case_roles (case_id, user_id, role, granted_by, role_definition_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, case_id, user_id, role, role_definition_id, granted_by, granted_at`,
		caseID, userID, role, grantedBy, roleDefinitionID,
	).Scan(&cr.ID, &cr.CaseID, &cr.UserID, &cr.Role, &cr.RoleDefinitionID, &cr.GrantedBy, &cr.GrantedAt)
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
		`SELECT id, case_id, user_id, role, role_definition_id, granted_by, granted_at
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
		if err := rows.Scan(&cr.ID, &cr.CaseID, &cr.UserID, &cr.Role, &cr.RoleDefinitionID, &cr.GrantedBy, &cr.GrantedAt); err != nil {
			return nil, fmt.Errorf("scan case role: %w", err)
		}
		roles = append(roles, cr)
	}
	return roles, rows.Err()
}

// ListByOrgID returns all case role assignments for cases belonging to the
// given organization, ordered by user then case title. Used by org admins
// to see who has access to what.
func (r *RoleRepository) ListByOrgID(ctx context.Context, orgID uuid.UUID) ([]CaseAssignmentView, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT cr.id, cr.case_id, cr.user_id, cr.role, cr.granted_by, cr.granted_at,
		        c.title, c.reference_code, c.status
		 FROM case_roles cr
		 JOIN cases c ON c.id = cr.case_id
		 WHERE c.organization_id = $1
		 ORDER BY cr.user_id, c.title`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list case assignments by org: %w", err)
	}
	defer rows.Close()

	var views []CaseAssignmentView
	for rows.Next() {
		var v CaseAssignmentView
		if err := rows.Scan(
			&v.ID, &v.CaseID, &v.UserID, &v.Role, &v.GrantedBy, &v.GrantedAt,
			&v.CaseTitle, &v.ReferenceCode, &v.CaseStatus,
		); err != nil {
			return nil, fmt.Errorf("scan case assignment view: %w", err)
		}
		views = append(views, v)
	}
	return views, rows.Err()
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
	roles         CaseRoleStore
	custody       CustodyRecorder
	audit         auth.AuditLogger
	orgChecker    OrgMembershipChecker
	caseLookupOrg func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
}

func NewRoleHandler(roles CaseRoleStore, custody CustodyRecorder, audit auth.AuditLogger) *RoleHandler {
	return &RoleHandler{roles: roles, custody: custody, audit: audit}
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// role assignment validates that the target user is an active member of
// the case's organization.
func (h *RoleHandler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

func (h *RoleHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{id}/roles", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/", h.Assign)
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Delete("/{userId}", h.Revoke)
		r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Get("/", h.List)
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

	if input.Role == "" {
		httputil.RespondError(w, http.StatusBadRequest, "role is required")
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

	// Verify both the actor AND target user are active members of the case's
	// organization. System admins bypass — they have cross-org access by design.
	if ac.SystemRole < auth.RoleSystemAdmin && h.orgChecker != nil && h.caseLookupOrg != nil {
		orgID, lookupErr := h.caseLookupOrg(r.Context(), caseID)
		if lookupErr != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		// Actor must be in the case's org
		actorIsMember, actorErr := h.orgChecker.IsActiveMember(r.Context(), orgID, ac.UserID)
		if actorErr != nil || !actorIsMember {
			httputil.RespondError(w, http.StatusNotFound, "not found")
			return
		}
		// Target must be in the case's org
		isMember, memberErr := h.orgChecker.IsActiveMember(r.Context(), orgID, input.UserID)
		if memberErr != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !isMember {
			httputil.RespondError(w, http.StatusBadRequest, "user is not an active member of this organization")
			return
		}
	}

	// Parse optional role_definition_id.
	var roleDefID *uuid.UUID
	if input.RoleDefinitionID != "" {
		parsed, parseErr := uuid.Parse(input.RoleDefinitionID)
		if parseErr != nil {
			httputil.RespondError(w, http.StatusBadRequest, "invalid role_definition_id")
			return
		}
		roleDefID = &parsed
	}

	cr, err := h.roles.Assign(r.Context(), caseID, input.UserID, input.Role, ac.UserID, roleDefID)
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
		if errors.Is(err, ErrNotFound) {
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
