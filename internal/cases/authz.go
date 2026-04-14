package cases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// OrgMemberChecker verifies org membership. Defined here to avoid
// a circular dependency on the organization package.
type OrgMemberChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
	GetRole(ctx context.Context, orgID uuid.UUID, userID string) (string, error)
}

type CaseAuthzService struct {
	caseRepo   Repository
	roleLoader auth.CaseRoleLoader
	orgChecker OrgMemberChecker
}

func NewCaseAuthzService(caseRepo Repository, roleLoader auth.CaseRoleLoader, orgChecker OrgMemberChecker) *CaseAuthzService {
	return &CaseAuthzService{
		caseRepo:   caseRepo,
		roleLoader: roleLoader,
		orgChecker: orgChecker,
	}
}

// CanViewCase checks if the caller can view a case.
// Access granted if: system_admin, org owner/admin, or has a case_role.
func (s *CaseAuthzService) CanViewCase(ctx context.Context, ac auth.AuthContext, caseID uuid.UUID) (bool, error) {
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return true, nil
	}

	c, err := s.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return false, fmt.Errorf("load case for authz: %w", err)
	}

	orgRole, err := s.orgChecker.GetRole(ctx, c.OrganizationID, ac.UserID)
	if err != nil {
		return false, nil
	}

	if orgRole == "owner" || orgRole == "admin" {
		return true, nil
	}

	_, err = s.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrNoCaseRole) {
			return false, nil
		}
		return false, fmt.Errorf("check case role: %w", err)
	}

	return true, nil
}

// CanManageCase checks if the caller can manage a case (update, archive, etc.).
// Only system_admin or org owner/admin.
func (s *CaseAuthzService) CanManageCase(ctx context.Context, ac auth.AuthContext, caseID uuid.UUID) (bool, error) {
	if ac.SystemRole >= auth.RoleSystemAdmin {
		return true, nil
	}

	c, err := s.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return false, fmt.Errorf("load case for authz: %w", err)
	}

	orgRole, err := s.orgChecker.GetRole(ctx, c.OrganizationID, ac.UserID)
	if err != nil {
		return false, nil
	}

	return orgRole == "owner" || orgRole == "admin", nil
}
