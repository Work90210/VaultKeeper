package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvitationNotFound = errors.New("invitation not found")

type InvitationRepository interface {
	Create(ctx context.Context, inv Invitation) (Invitation, error)
	GetByTokenHash(ctx context.Context, hash string) (Invitation, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Invitation, error)
	MarkAccepted(ctx context.Context, inviteID uuid.UUID, userID string) error
	MarkDeclined(ctx context.Context, inviteID uuid.UUID) error
	Revoke(ctx context.Context, orgID uuid.UUID, inviteID uuid.UUID) error
}

type PGInvitationRepository struct {
	pool *pgxpool.Pool
}

func NewInvitationRepository(pool *pgxpool.Pool) *PGInvitationRepository {
	return &PGInvitationRepository{pool: pool}
}

func (r *PGInvitationRepository) Create(ctx context.Context, inv Invitation) (Invitation, error) {
	var result Invitation
	err := r.pool.QueryRow(ctx,
		`INSERT INTO organization_invitations
		    (organization_id, email, role, token_hash, invited_by, status, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, email, role, token_hash, status, expires_at,
		           invited_by, accepted_by, accepted_at, created_at`,
		inv.OrganizationID, inv.Email, inv.Role, inv.TokenHash,
		inv.InvitedBy, InvitePending, inv.ExpiresAt,
	).Scan(&result.ID, &result.OrganizationID, &result.Email, &result.Role,
		&result.TokenHash, &result.Status, &result.ExpiresAt,
		&result.InvitedBy, &result.AcceptedBy, &result.AcceptedAt, &result.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return Invitation{}, fmt.Errorf("pending invitation already exists for this email")
		}
		return Invitation{}, fmt.Errorf("create invitation: %w", err)
	}
	return result, nil
}

func (r *PGInvitationRepository) GetByTokenHash(ctx context.Context, hash string) (Invitation, error) {
	var inv Invitation
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, email, role, token_hash, status, expires_at,
		        invited_by, accepted_by, accepted_at, created_at
		 FROM organization_invitations
		 WHERE token_hash = $1 AND status = 'pending' AND expires_at > now()`,
		hash,
	).Scan(&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Role,
		&inv.TokenHash, &inv.Status, &inv.ExpiresAt,
		&inv.InvitedBy, &inv.AcceptedBy, &inv.AcceptedAt, &inv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invitation{}, ErrInvitationNotFound
		}
		return Invitation{}, fmt.Errorf("get invitation by token: %w", err)
	}
	return inv, nil
}

func (r *PGInvitationRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Invitation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, email, role, token_hash, status, expires_at,
		        invited_by, accepted_by, accepted_at, created_at
		 FROM organization_invitations
		 WHERE organization_id = $1 AND status = 'pending'
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	var invites []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Role,
			&inv.TokenHash, &inv.Status, &inv.ExpiresAt,
			&inv.InvitedBy, &inv.AcceptedBy, &inv.AcceptedAt, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (r *PGInvitationRepository) MarkAccepted(ctx context.Context, inviteID uuid.UUID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organization_invitations
		 SET status = 'accepted', accepted_by = $2, accepted_at = now()
		 WHERE id = $1 AND status = 'pending'`,
		inviteID, userID)
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (r *PGInvitationRepository) MarkDeclined(ctx context.Context, inviteID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organization_invitations SET status = 'declined' WHERE id = $1 AND status = 'pending'`,
		inviteID)
	if err != nil {
		return fmt.Errorf("mark invitation declined: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

func (r *PGInvitationRepository) Revoke(ctx context.Context, orgID uuid.UUID, inviteID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organization_invitations SET status = 'revoked' WHERE id = $1 AND organization_id = $2 AND status = 'pending'`,
		inviteID, orgID)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInvitationNotFound
	}
	return nil
}
