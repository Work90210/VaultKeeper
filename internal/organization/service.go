package organization

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

var (
	ErrInviteExpired    = errors.New("invitation has expired")
	ErrInviteNotPending = errors.New("invitation is no longer pending")
	ErrEmailMismatch    = errors.New("invitation email does not match your account")
	ErrLastOwner        = errors.New("cannot remove or demote last owner")
	ErrActiveCases      = errors.New("organization has active or legal-hold cases; archive all cases before deletion")
	ErrSelfRemove       = errors.New("cannot remove yourself; transfer ownership first")
)

const inviteExpiryDuration = 7 * 24 * time.Hour

// CaseStatusChecker checks if an org has cases that prevent deletion.
type CaseStatusChecker interface {
	HasActiveCases(ctx context.Context, orgID uuid.UUID) (bool, error)
}

// RoleDefSeeder seeds default case role definitions for a new organization.
type RoleDefSeeder interface {
	SeedDefaults(ctx context.Context, orgID uuid.UUID) error
}

type Service struct {
	pool          *pgxpool.Pool
	orgRepo       OrgRepository
	memberRepo    MembershipRepository
	inviteRepo    InvitationRepository
	authz         *OrgAuthzService
	caseChecker   CaseStatusChecker
	roleDefSeeder RoleDefSeeder
	logger        *slog.Logger
}

func NewService(
	pool *pgxpool.Pool,
	orgRepo OrgRepository,
	memberRepo MembershipRepository,
	inviteRepo InvitationRepository,
	authz *OrgAuthzService,
	logger *slog.Logger,
) *Service {
	return &Service{
		pool:       pool,
		orgRepo:    orgRepo,
		memberRepo: memberRepo,
		inviteRepo: inviteRepo,
		authz:      authz,
		logger:     logger,
	}
}

func (s *Service) WithCaseStatusChecker(c CaseStatusChecker) *Service {
	if c == nil {
		panic("organization.Service requires a non-nil CaseStatusChecker")
	}
	s.caseChecker = c
	return s
}

func (s *Service) WithRoleDefSeeder(seeder RoleDefSeeder) *Service {
	s.roleDefSeeder = seeder
	return s
}

// CreateOrg creates a new organization and adds the caller as owner.
func (s *Service) CreateOrg(ctx context.Context, ac auth.AuthContext, input CreateOrgInput) (Organization, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Organization{}, fmt.Errorf("organization name is required")
	}
	if len(input.Name) > 255 {
		return Organization{}, fmt.Errorf("organization name too long (max 255 characters)")
	}
	if len(input.Description) > 2000 {
		return Organization{}, fmt.Errorf("organization description too long (max 2000 characters)")
	}

	slug := generateSlug(input.Name)

	org, err := s.orgRepo.Create(ctx, Organization{
		Name:        strings.TrimSpace(input.Name),
		Slug:        slug,
		Description: strings.TrimSpace(input.Description),
		Settings:    []byte("{}"),
		CreatedBy:   ac.UserID,
	})
	if err != nil {
		return Organization{}, fmt.Errorf("create organization: %w", err)
	}

	now := time.Now()
	_, err = s.memberRepo.Upsert(ctx, Membership{
		OrganizationID: org.ID,
		UserID:         ac.UserID,
		Role:           RoleOwner,
		Status:         StatusActive,
		JoinedAt:       &now,
	})
	if err != nil {
		return Organization{}, fmt.Errorf("add creator as owner: %w", err)
	}

	// Seed default case role definitions for the new org.
	if s.roleDefSeeder != nil {
		if seedErr := s.roleDefSeeder.SeedDefaults(ctx, org.ID); seedErr != nil {
			s.logger.Warn("failed to seed default role definitions", "org_id", org.ID, "error", seedErr)
		}
	}

	return org, nil
}

func (s *Service) GetOrg(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) (Organization, error) {
	if err := s.authz.RequireOrgMember(ctx, ac, orgID); err != nil {
		return Organization{}, err
	}
	return s.orgRepo.GetByID(ctx, orgID)
}

func (s *Service) UpdateOrg(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, input UpdateOrgInput) (Organization, error) {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return Organization{}, err
	}
	return s.orgRepo.Update(ctx, orgID, input)
}

func (s *Service) DeleteOrg(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) error {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner); err != nil {
		return err
	}

	if s.caseChecker != nil {
		hasActive, err := s.caseChecker.HasActiveCases(ctx, orgID)
		if err != nil {
			return fmt.Errorf("check active cases: %w", err)
		}
		if hasActive {
			return ErrActiveCases
		}
	}

	return s.orgRepo.SoftDelete(ctx, orgID)
}

func (s *Service) ListUserOrgs(ctx context.Context, userID string) ([]OrgWithRole, error) {
	return s.orgRepo.ListForUser(ctx, userID)
}

// --- Members ---

func (s *Service) ListMembers(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) ([]Membership, error) {
	if err := s.authz.RequireOrgMember(ctx, ac, orgID); err != nil {
		return nil, err
	}
	return s.memberRepo.ListMembers(ctx, orgID)
}

func (s *Service) UpdateMemberRole(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, targetUserID string, newRole OrgRole) error {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return err
	}

	if !newRole.IsValid() {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	existing, err := s.memberRepo.GetMembership(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}

	// Cannot demote owner if they are the last one.
	if existing.Role == RoleOwner && newRole != RoleOwner {
		count, err := s.memberRepo.CountOwners(ctx, orgID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastOwner
		}
	}

	// Only owners can promote to owner or demote from owner.
	if newRole == RoleOwner || existing.Role == RoleOwner {
		if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner); err != nil {
			return err
		}
	}

	_, err = s.memberRepo.Upsert(ctx, Membership{
		OrganizationID: orgID,
		UserID:         targetUserID,
		Role:           newRole,
		Status:         StatusActive,
		JoinedAt:       existing.JoinedAt,
	})
	return err
}

func (s *Service) RemoveMember(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, targetUserID string) error {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return err
	}

	if ac.UserID == targetUserID {
		return ErrSelfRemove
	}

	existing, err := s.memberRepo.GetMembership(ctx, orgID, targetUserID)
	if err != nil {
		return err
	}

	if existing.Role == RoleOwner {
		count, err := s.memberRepo.CountOwners(ctx, orgID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastOwner
		}
		// Only owners can remove other owners.
		if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner); err != nil {
			return err
		}
	}

	return s.memberRepo.Remove(ctx, orgID, targetUserID)
}

func (s *Service) TransferOwnership(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, targetUserID string) error {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner); err != nil {
		return err
	}

	// Prevent self-transfer: demoting yourself to admin with no other owner would
	// silently break the invariant that an org always has at least one owner.
	if targetUserID == ac.UserID {
		return fmt.Errorf("cannot transfer ownership to yourself")
	}

	// Verify target is an active member.
	target, err := s.memberRepo.GetMembership(ctx, orgID, targetUserID)
	if err != nil {
		return fmt.Errorf("target user: %w", err)
	}
	if target.Status != StatusActive {
		return fmt.Errorf("target user is not an active member")
	}

	// Promote and demote atomically so a partial failure cannot leave the org
	// with zero owners or two owners when only one was intended.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transfer-ownership transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // intentional rollback on any non-commit path

	_, err = tx.Exec(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (organization_id, user_id) DO UPDATE
		 SET role = EXCLUDED.role, status = EXCLUDED.status,
		     joined_at = COALESCE(EXCLUDED.joined_at, organization_memberships.joined_at),
		     updated_at = now()`,
		orgID, targetUserID, RoleOwner, StatusActive, target.JoinedAt,
	)
	if err != nil {
		return fmt.Errorf("promote target to owner: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
		 VALUES ($1, $2, $3, $4, now())
		 ON CONFLICT (organization_id, user_id) DO UPDATE
		 SET role = EXCLUDED.role, status = EXCLUDED.status,
		     updated_at = now()`,
		orgID, ac.UserID, RoleAdmin, StatusActive,
	)
	if err != nil {
		return fmt.Errorf("demote caller to admin: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transfer-ownership transaction: %w", err)
	}
	return nil
}

// --- Invitations ---

func (s *Service) InviteMember(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, input InviteInput) (Invitation, string, error) {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return Invitation{}, "", err
	}

	if !input.Role.IsValid() || input.Role == RoleOwner {
		return Invitation{}, "", fmt.Errorf("invalid invite role: %s (must be admin or member)", input.Role)
	}

	rawToken, tokenHash, err := generateInviteToken()
	if err != nil {
		return Invitation{}, "", fmt.Errorf("generate token: %w", err)
	}

	inv, err := s.inviteRepo.Create(ctx, Invitation{
		OrganizationID: orgID,
		Email:          strings.TrimSpace(strings.ToLower(input.Email)),
		Role:           input.Role,
		TokenHash:      tokenHash,
		InvitedBy:      ac.UserID,
		ExpiresAt:      time.Now().Add(inviteExpiryDuration),
	})
	if err != nil {
		return Invitation{}, "", err
	}

	return inv, rawToken, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, ac auth.AuthContext, rawToken string) (Organization, error) {
	tokenHash := hashToken(rawToken)

	inv, err := s.inviteRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return Organization{}, err
	}

	if inv.Status != InvitePending {
		return Organization{}, ErrInviteNotPending
	}

	if time.Now().After(inv.ExpiresAt) {
		return Organization{}, ErrInviteExpired
	}

	if !strings.EqualFold(inv.Email, ac.Email) {
		return Organization{}, ErrEmailMismatch
	}

	// Atomically create membership and mark the invitation accepted so the
	// invite cannot be reused if the second write fails.
	if err := s.acceptInvitationTx(ctx, inv, ac.UserID); err != nil {
		return Organization{}, err
	}

	return s.orgRepo.GetByID(ctx, inv.OrganizationID)
}

// acceptInvitationTx wraps the membership upsert and the invitation status
// update in a single database transaction. If either operation fails the
// transaction is rolled back, preventing a reusable dangling invite.
func (s *Service) acceptInvitationTx(ctx context.Context, inv Invitation, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin accept-invitation transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback on any non-commit path is intentional

	now := time.Now()
	_, err = tx.Exec(ctx,
		`INSERT INTO organization_memberships (organization_id, user_id, role, status, joined_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (organization_id, user_id) DO UPDATE
		 SET role = EXCLUDED.role, status = EXCLUDED.status,
		     joined_at = COALESCE(EXCLUDED.joined_at, organization_memberships.joined_at),
		     updated_at = now()`,
		inv.OrganizationID, userID, inv.Role, StatusActive, &now,
	)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE organization_invitations
		 SET status = 'accepted', accepted_by = $2, accepted_at = now()
		 WHERE id = $1 AND status = 'pending'`,
		inv.ID, userID,
	)
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrInviteNotPending
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit accept-invitation transaction: %w", err)
	}
	return nil
}

func (s *Service) DeclineInvitation(ctx context.Context, ac auth.AuthContext, rawToken string) error {
	tokenHash := hashToken(rawToken)

	inv, err := s.inviteRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return err
	}

	if inv.Status != InvitePending {
		return ErrInviteNotPending
	}

	if !strings.EqualFold(inv.Email, ac.Email) {
		return ErrEmailMismatch
	}

	return s.inviteRepo.MarkDeclined(ctx, inv.ID)
}

func (s *Service) ListInvitations(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) ([]Invitation, error) {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return nil, err
	}
	return s.inviteRepo.ListByOrg(ctx, orgID)
}

func (s *Service) RevokeInvitation(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, inviteID uuid.UUID) error {
	if err := s.authz.RequireOrgRole(ctx, ac, orgID, RoleOwner, RoleAdmin); err != nil {
		return err
	}
	return s.inviteRepo.Revoke(ctx, orgID, inviteID)
}

// --- Helpers ---

func generateInviteToken() (raw string, hash string, err error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", "", fmt.Errorf("generate random token: %w", err)
	}
	raw = hex.EncodeToString(token)
	hash = hashToken(raw)
	return raw, hash, nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateSlug(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, slug)
	// Collapse multiple hyphens.
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = uuid.New().String()[:8]
	}
	return slug
}
