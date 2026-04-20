package roledefs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("role definition not found")

type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]RoleDefinition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, slug, description, permissions, is_default, is_system, created_at, updated_at
		 FROM case_role_definitions
		 WHERE organization_id = $1
		 ORDER BY is_system DESC, name ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list role definitions: %w", err)
	}
	defer rows.Close()
	return scanAll(rows)
}

func (r *PgRepository) GetByID(ctx context.Context, id uuid.UUID) (RoleDefinition, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, slug, description, permissions, is_default, is_system, created_at, updated_at
		 FROM case_role_definitions WHERE id = $1`, id)
	return scanOne(row)
}

func (r *PgRepository) GetByOrgAndSlug(ctx context.Context, orgID uuid.UUID, slug string) (RoleDefinition, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, slug, description, permissions, is_default, is_system, created_at, updated_at
		 FROM case_role_definitions WHERE organization_id = $1 AND slug = $2`, orgID, slug)
	return scanOne(row)
}

func (r *PgRepository) Create(ctx context.Context, rd RoleDefinition) (RoleDefinition, error) {
	permsJSON, err := json.Marshal(rd.Permissions)
	if err != nil {
		return RoleDefinition{}, fmt.Errorf("marshal permissions: %w", err)
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO case_role_definitions (organization_id, name, slug, description, permissions, is_default, is_system)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, name, slug, description, permissions, is_default, is_system, created_at, updated_at`,
		rd.OrganizationID, rd.Name, rd.Slug, rd.Description, permsJSON, rd.IsDefault, rd.IsSystem)
	return scanOne(row)
}

func (r *PgRepository) Update(ctx context.Context, id uuid.UUID, input UpdateInput) (RoleDefinition, error) {
	setClauses := []string{"updated_at = now()"}
	args := []any{}
	argIdx := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *input.Description)
		argIdx++
	}
	if input.Permissions != nil {
		permsJSON, err := json.Marshal(input.Permissions)
		if err != nil {
			return RoleDefinition{}, fmt.Errorf("marshal permissions: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("permissions = $%d", argIdx))
		args = append(args, permsJSON)
		argIdx++
	}

	args = append(args, id)
	sql := fmt.Sprintf(
		`UPDATE case_role_definitions SET %s WHERE id = $%d
		 RETURNING id, organization_id, name, slug, description, permissions, is_default, is_system, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx)

	row := r.pool.QueryRow(ctx, sql, args...)
	return scanOne(row)
}

func (r *PgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM case_role_definitions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete role definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgRepository) IsInUse(ctx context.Context, roleDefID uuid.UUID) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM case_roles WHERE role_definition_id = $1`, roleDefID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check role definition usage: %w", err)
	}
	return count > 0, nil
}

// SeedDefaults inserts all default role definitions for an org, skipping any
// that already exist (idempotent).
func (r *PgRepository) SeedDefaults(ctx context.Context, orgID uuid.UUID) error {
	defaults := DefaultRoleDefinitions()
	for _, def := range defaults {
		permsJSON, err := json.Marshal(def.Permissions)
		if err != nil {
			return fmt.Errorf("marshal default permissions: %w", err)
		}
		_, err = r.pool.Exec(ctx,
			`INSERT INTO case_role_definitions (organization_id, name, slug, description, permissions, is_default, is_system)
			 VALUES ($1, $2, $3, $4, $5, true, true)
			 ON CONFLICT (organization_id, slug) DO NOTHING`,
			orgID, def.Name, def.Slug, def.Description, permsJSON)
		if err != nil {
			return fmt.Errorf("seed default role %q: %w", def.Slug, err)
		}
	}
	return nil
}

// LoadCasePermissions returns the permission map for a user's role in a given case.
// Used by permission-checking middleware.
func (r *PgRepository) LoadCasePermissions(ctx context.Context, caseID, userID string) (map[string]bool, error) {
	var permsJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT crd.permissions
		 FROM case_roles cr
		 JOIN case_role_definitions crd ON crd.id = cr.role_definition_id
		 WHERE cr.case_id = $1 AND cr.user_id = $2
		 LIMIT 1`, caseID, userID).Scan(&permsJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load case permissions: %w", err)
	}
	var perms map[string]bool
	if err := json.Unmarshal(permsJSON, &perms); err != nil {
		return nil, fmt.Errorf("unmarshal permissions: %w", err)
	}
	return perms, nil
}

func scanOne(row pgx.Row) (RoleDefinition, error) {
	var rd RoleDefinition
	var permsJSON []byte
	err := row.Scan(
		&rd.ID, &rd.OrganizationID, &rd.Name, &rd.Slug, &rd.Description,
		&permsJSON, &rd.IsDefault, &rd.IsSystem, &rd.CreatedAt, &rd.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoleDefinition{}, ErrNotFound
		}
		return RoleDefinition{}, fmt.Errorf("scan role definition: %w", err)
	}
	rd.Permissions = make(map[Permission]bool)
	if len(permsJSON) > 0 {
		var raw map[string]bool
		if err := json.Unmarshal(permsJSON, &raw); err != nil {
			return RoleDefinition{}, fmt.Errorf("unmarshal permissions: %w", err)
		}
		for k, v := range raw {
			rd.Permissions[Permission(k)] = v
		}
	}
	return rd, nil
}

func scanAll(rows pgx.Rows) ([]RoleDefinition, error) {
	var result []RoleDefinition
	for rows.Next() {
		var rd RoleDefinition
		var permsJSON []byte
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&rd.ID, &rd.OrganizationID, &rd.Name, &rd.Slug, &rd.Description,
			&permsJSON, &rd.IsDefault, &rd.IsSystem, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan role definition: %w", err)
		}
		rd.CreatedAt = createdAt
		rd.UpdatedAt = updatedAt
		rd.Permissions = make(map[Permission]bool)
		if len(permsJSON) > 0 {
			var raw map[string]bool
			if err := json.Unmarshal(permsJSON, &raw); err != nil {
				return nil, fmt.Errorf("unmarshal permissions: %w", err)
			}
			for k, v := range raw {
				rd.Permissions[Permission(k)] = v
			}
		}
		result = append(result, rd)
	}
	return result, rows.Err()
}
