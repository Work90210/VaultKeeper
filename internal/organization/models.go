package organization

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type OrgRole string

const (
	RoleOwner  OrgRole = "owner"
	RoleAdmin  OrgRole = "admin"
	RoleMember OrgRole = "member"
)

func (r OrgRole) IsValid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	}
	return false
}

func (r OrgRole) CanManageMembers() bool {
	return r == RoleOwner || r == RoleAdmin
}

func (r OrgRole) CanManageCases() bool {
	return r == RoleOwner || r == RoleAdmin
}

type MemberStatus string

const (
	StatusActive    MemberStatus = "active"
	StatusInvited   MemberStatus = "invited"
	StatusSuspended MemberStatus = "suspended"
	StatusRemoved   MemberStatus = "removed"
)

type InviteStatus string

const (
	InvitePending  InviteStatus = "pending"
	InviteAccepted InviteStatus = "accepted"
	InviteDeclined InviteStatus = "declined"
	InviteExpired  InviteStatus = "expired"
	InviteRevoked  InviteStatus = "revoked"
)

type Organization struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	LogoAssetID *uuid.UUID      `json:"logo_asset_id,omitempty"`
	Settings    json.RawMessage `json:"settings"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
}

type Membership struct {
	ID             uuid.UUID    `json:"id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	UserID         string       `json:"user_id"`
	Role           OrgRole      `json:"role"`
	Status         MemberStatus `json:"status"`
	JoinedAt       *time.Time   `json:"joined_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	// Denormalized fields populated by joins.
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

type Invitation struct {
	ID             uuid.UUID    `json:"id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	Email          string       `json:"email"`
	Role           OrgRole      `json:"role"`
	TokenHash      string       `json:"-"`
	Status         InviteStatus `json:"status"`
	ExpiresAt      time.Time    `json:"expires_at"`
	InvitedBy      string       `json:"invited_by"`
	AcceptedBy     *string      `json:"accepted_by,omitempty"`
	AcceptedAt     *time.Time   `json:"accepted_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

type CreateOrgInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateOrgInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type InviteInput struct {
	Email string  `json:"email"`
	Role  OrgRole `json:"role"`
}

type AcceptInviteInput struct {
	Token string `json:"token"`
}

type TransferOwnershipInput struct {
	TargetUserID string `json:"target_user_id"`
}

type UpdateMemberRoleInput struct {
	Role OrgRole `json:"role"`
}

// OrgWithRole is returned when listing a user's organizations.
type OrgWithRole struct {
	Organization
	Role        OrgRole `json:"role"`
	MemberCount int     `json:"member_count"`
	CaseCount   int     `json:"case_count"`
}
