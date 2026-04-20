package organization

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// OrgMemberAdapter bridges the MembershipRepository to the cases.OrgMemberChecker
// interface, avoiding circular imports.
type OrgMemberAdapter struct {
	MemberRepo MembershipRepository
}

func (a *OrgMemberAdapter) IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error) {
	m, err := a.MemberRepo.GetMembership(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, ErrMembershipNotFound) {
			return false, nil
		}
		return false, err
	}
	return m.Status == StatusActive, nil
}

func (a *OrgMemberAdapter) GetRole(ctx context.Context, orgID uuid.UUID, userID string) (string, error) {
	m, err := a.MemberRepo.GetMembership(ctx, orgID, userID)
	if err != nil {
		return "", err
	}
	if m.Status != StatusActive {
		return "", ErrNotOrgMember
	}
	return string(m.Role), nil
}

// GetOrgRole satisfies the roledefs.OrgMembershipChecker interface. It returns
// the caller's org role (e.g. "owner", "admin", "member") or an error when the
// user is not an active member.
func (a *OrgMemberAdapter) GetOrgRole(ctx context.Context, orgID uuid.UUID, userID string) (string, error) {
	return a.GetRole(ctx, orgID, userID)
}
