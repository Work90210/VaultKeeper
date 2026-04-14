package organization

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOrgNotFound      = errors.New("organization not found")
	ErrSlugTaken        = errors.New("slug already taken")
	ErrDuplicateName    = errors.New("organization name conflict")
)

type OrgRepository interface {
	Create(ctx context.Context, org Organization) (Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (Organization, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateOrgInput) (Organization, error)
	ListForUser(ctx context.Context, userID string) ([]OrgWithRole, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

type PGOrgRepository struct {
	pool *pgxpool.Pool
}

func NewOrgRepository(pool *pgxpool.Pool) *PGOrgRepository {
	return &PGOrgRepository{pool: pool}
}

func (r *PGOrgRepository) Create(ctx context.Context, org Organization) (Organization, error) {
	var result Organization
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, description, logo_asset_id, settings, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, slug, description, logo_asset_id, settings, created_by, created_at, updated_at, deleted_at`,
		org.Name, org.Slug, org.Description, org.LogoAssetID, org.Settings, org.CreatedBy,
	).Scan(&result.ID, &result.Name, &result.Slug, &result.Description,
		&result.LogoAssetID, &result.Settings, &result.CreatedBy,
		&result.CreatedAt, &result.UpdatedAt, &result.DeletedAt)
	if err != nil {
		if isUniqueViolation(err) {
			if strings.Contains(err.Error(), "slug") {
				return Organization{}, ErrSlugTaken
			}
			return Organization{}, ErrDuplicateName
		}
		return Organization{}, fmt.Errorf("insert organization: %w", err)
	}
	return result, nil
}

func (r *PGOrgRepository) GetByID(ctx context.Context, id uuid.UUID) (Organization, error) {
	var org Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, description, logo_asset_id, settings, created_by, created_at, updated_at, deleted_at
		 FROM organizations WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Description,
		&org.LogoAssetID, &org.Settings, &org.CreatedBy,
		&org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Organization{}, ErrOrgNotFound
		}
		return Organization{}, fmt.Errorf("find organization by id: %w", err)
	}
	return org, nil
}

func (r *PGOrgRepository) Update(ctx context.Context, id uuid.UUID, input UpdateOrgInput) (Organization, error) {
	var sets []string
	var args []any
	argIdx := 1

	if input.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *input.Description)
		argIdx++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf(
		`UPDATE organizations SET %s WHERE id = $%d AND deleted_at IS NULL
		 RETURNING id, name, slug, description, logo_asset_id, settings, created_by, created_at, updated_at, deleted_at`,
		strings.Join(sets, ", "), argIdx,
	)

	var org Organization
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&org.ID, &org.Name, &org.Slug, &org.Description,
		&org.LogoAssetID, &org.Settings, &org.CreatedBy,
		&org.CreatedAt, &org.UpdatedAt, &org.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Organization{}, ErrOrgNotFound
		}
		return Organization{}, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

func (r *PGOrgRepository) ListForUser(ctx context.Context, userID string) ([]OrgWithRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id, o.name, o.slug, o.description, o.logo_asset_id, o.settings,
		        o.created_by, o.created_at, o.updated_at, o.deleted_at,
		        om.role,
		        (SELECT count(*) FROM organization_memberships WHERE organization_id = o.id AND status = 'active') AS member_count,
		        (SELECT count(*) FROM cases WHERE organization_id = o.id) AS case_count
		 FROM organizations o
		 JOIN organization_memberships om ON om.organization_id = o.id
		 WHERE om.user_id = $1 AND om.status = 'active' AND o.deleted_at IS NULL
		 ORDER BY o.name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list organizations for user: %w", err)
	}
	defer rows.Close()

	var orgs []OrgWithRole
	for rows.Next() {
		var owr OrgWithRole
		if err := rows.Scan(
			&owr.ID, &owr.Name, &owr.Slug, &owr.Description,
			&owr.LogoAssetID, &owr.Settings, &owr.CreatedBy,
			&owr.CreatedAt, &owr.UpdatedAt, &owr.DeletedAt,
			&owr.Role, &owr.MemberCount, &owr.CaseCount,
		); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		orgs = append(orgs, owr)
	}
	return orgs, rows.Err()
}

func (r *PGOrgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete organization: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrgNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
