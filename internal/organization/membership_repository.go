package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMembershipNotFound = errors.New("membership not found")

type MembershipRepository interface {
	GetMembership(ctx context.Context, orgID uuid.UUID, userID string) (Membership, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]Membership, error)
	Upsert(ctx context.Context, m Membership) (Membership, error)
	Remove(ctx context.Context, orgID uuid.UUID, userID string) error
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
}

type PGMembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *PGMembershipRepository {
	return &PGMembershipRepository{pool: pool}
}

func (r *PGMembershipRepository) GetMembership(ctx context.Context, orgID uuid.UUID, userID string) (Membership, error) {
	var m Membership
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, user_id, role, status, joined_at, created_at, updated_at
		 FROM organization_memberships
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.Status,
		&m.JoinedAt, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Membership{}, ErrMembershipNotFound
		}
		return Membership{}, fmt.Errorf("get membership: %w", err)
	}
	return m, nil
}

func (r *PGMembershipRepository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]Membership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT om.id, om.organization_id, om.user_id, om.role, om.status, om.joined_at,
		        om.created_at, om.updated_at,
		        COALESCE(up.display_name, '') AS display_name
		 FROM organization_memberships om
		 LEFT JOIN user_profiles up ON up.user_id = om.user_id
		 WHERE om.organization_id = $1 AND om.status = 'active'
		 ORDER BY om.role, om.joined_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.Status,
			&m.JoinedAt, &m.CreatedAt, &m.UpdatedAt, &m.DisplayName); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *PGMembershipRepository) Upsert(ctx context.Context, m Membership) (Membership, error) {
	var result Membership
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (organization_id, user_id) DO UPDATE
		 SET role = EXCLUDED.role, status = EXCLUDED.status,
		     joined_at = COALESCE(EXCLUDED.joined_at, organization_memberships.joined_at),
		     updated_at = now()
		 RETURNING id, organization_id, user_id, role, status, joined_at, created_at, updated_at`,
		m.OrganizationID, m.UserID, m.Role, m.Status, m.JoinedAt,
	).Scan(&result.ID, &result.OrganizationID, &result.UserID, &result.Role,
		&result.Status, &result.JoinedAt, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return Membership{}, fmt.Errorf("upsert membership: %w", err)
	}
	return result, nil
}

func (r *PGMembershipRepository) Remove(ctx context.Context, orgID uuid.UUID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organization_memberships SET status = 'removed', updated_at = now()
		 WHERE organization_id = $1 AND user_id = $2 AND status = 'active'`,
		orgID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMembershipNotFound
	}
	return nil
}

func (r *PGMembershipRepository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM organization_memberships
		 WHERE organization_id = $1 AND role = 'owner' AND status = 'active'`,
		orgID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count owners: %w", err)
	}
	return count, nil
}
