package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

var (
	ErrForbidden      = errors.New("insufficient permissions")
	ErrNotOrgMember   = errors.New("not a member of this organization")
)

type OrgAuthzService struct {
	memberRepo MembershipRepository
}

func NewOrgAuthzService(memberRepo MembershipRepository) *OrgAuthzService {
	return &OrgAuthzService{memberRepo: memberRepo}
}

// RequireOrgRole checks that the caller has one of the allowed org roles.
// System admins bypass all checks.
func (s *OrgAuthzService) RequireOrgRole(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID, allowed ...OrgRole) error {
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return nil
	}

	m, err := s.memberRepo.GetMembership(ctx, orgID, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrMembershipNotFound) {
			return ErrNotOrgMember
		}
		return fmt.Errorf("check org role: %w", err)
	}

	if m.Status != StatusActive {
		return ErrNotOrgMember
	}

	for _, role := range allowed {
		if m.Role == role {
			return nil
		}
	}

	return ErrForbidden
}

// RequireOrgMember checks that the caller is an active member of the org.
func (s *OrgAuthzService) RequireOrgMember(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) error {
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return nil
	}

	m, err := s.memberRepo.GetMembership(ctx, orgID, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrMembershipNotFound) {
			return ErrNotOrgMember
		}
		return fmt.Errorf("check org membership: %w", err)
	}

	if m.Status != StatusActive {
		return ErrNotOrgMember
	}

	return nil
}

// GetCallerOrgRole returns the caller's role in the org, or empty string if not a member.
// System admins get RoleOwner equivalent.
func (s *OrgAuthzService) GetCallerOrgRole(ctx context.Context, ac auth.AuthContext, orgID uuid.UUID) (OrgRole, error) {
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return RoleOwner, nil
	}

	m, err := s.memberRepo.GetMembership(ctx, orgID, ac.UserID)
	if err != nil {
		if errors.Is(err, ErrMembershipNotFound) {
			return "", ErrNotOrgMember
		}
		return "", fmt.Errorf("get caller org role: %w", err)
	}

	if m.Status != StatusActive {
		return "", ErrNotOrgMember
	}

	return m.Role, nil
}
