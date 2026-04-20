package roledefs

import (
	"time"

	"github.com/google/uuid"
)

// Permission represents a granular action a case role can perform.
type Permission string

const (
	PermViewEvidence        Permission = "view_evidence"
	PermUploadEvidence      Permission = "upload_evidence"
	PermEditEvidence        Permission = "edit_evidence"
	PermDeleteEvidence      Permission = "delete_evidence"
	PermViewWitnesses       Permission = "view_witnesses"
	PermManageWitnesses     Permission = "manage_witnesses"
	PermViewDisclosures     Permission = "view_disclosures"
	PermManageDisclosures   Permission = "manage_disclosures"
	PermManageCase          Permission = "manage_case"
	PermManageMembers       Permission = "manage_members"
	PermExport              Permission = "export"
	PermManageInvestigation Permission = "manage_investigation"
)

// AllPermissions enumerates every defined permission.
var AllPermissions = []Permission{
	PermViewEvidence, PermUploadEvidence, PermEditEvidence, PermDeleteEvidence,
	PermViewWitnesses, PermManageWitnesses,
	PermViewDisclosures, PermManageDisclosures,
	PermManageCase, PermManageMembers,
	PermExport, PermManageInvestigation,
}

// RoleDefinition is a per-organization role template with granular permissions.
type RoleDefinition struct {
	ID             uuid.UUID          `json:"id"`
	OrganizationID uuid.UUID          `json:"organization_id"`
	Name           string             `json:"name"`
	Slug           string             `json:"slug"`
	Description    string             `json:"description"`
	Permissions    map[Permission]bool `json:"permissions"`
	IsDefault      bool               `json:"is_default"`
	IsSystem       bool               `json:"is_system"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// CreateInput is the payload for creating a custom role definition.
type CreateInput struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Permissions map[Permission]bool `json:"permissions"`
}

// UpdateInput is the payload for editing a role definition's permissions.
type UpdateInput struct {
	Name        *string            `json:"name,omitempty"`
	Description *string            `json:"description,omitempty"`
	Permissions map[Permission]bool `json:"permissions,omitempty"`
}

// DefaultRoleDefinitions returns the system default roles seeded per org.
func DefaultRoleDefinitions() []RoleDefinition {
	return []RoleDefinition{
		{
			Name: "Lead Investigator", Slug: "lead_investigator",
			Description: "Full case access including member and case management",
			Permissions: allTrue(),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Investigator", Slug: "investigator",
			Description: "Evidence collection and witness management",
			Permissions: permSet(
				PermViewEvidence, PermUploadEvidence, PermEditEvidence,
				PermViewWitnesses, PermManageWitnesses,
				PermViewDisclosures, PermExport, PermManageInvestigation,
			),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Prosecutor", Slug: "prosecutor",
			Description: "Prosecution team with disclosure management",
			Permissions: permSet(
				PermViewEvidence, PermViewWitnesses,
				PermViewDisclosures, PermManageDisclosures, PermExport,
			),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Defence", Slug: "defence",
			Description: "Defence counsel with read access to disclosed materials",
			Permissions: permSet(
				PermViewEvidence, PermViewDisclosures, PermExport,
			),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Judge", Slug: "judge",
			Description: "Judicial oversight with full read access",
			Permissions: permSet(
				PermViewEvidence, PermViewWitnesses, PermViewDisclosures, PermExport,
			),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Observer", Slug: "observer",
			Description: "Read-only access to evidence and disclosures",
			Permissions: permSet(PermViewEvidence, PermViewDisclosures),
			IsDefault: true, IsSystem: true,
		},
		{
			Name: "Victim Representative", Slug: "victim_representative",
			Description: "Victim participation with limited access",
			Permissions: permSet(PermViewEvidence, PermViewDisclosures),
			IsDefault: true, IsSystem: true,
		},
	}
}

func allTrue() map[Permission]bool {
	m := make(map[Permission]bool, len(AllPermissions))
	for _, p := range AllPermissions {
		m[p] = true
	}
	return m
}

func permSet(perms ...Permission) map[Permission]bool {
	m := make(map[Permission]bool, len(AllPermissions))
	for _, p := range AllPermissions {
		m[p] = false
	}
	for _, p := range perms {
		m[p] = true
	}
	return m
}
