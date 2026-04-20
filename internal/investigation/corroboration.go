package investigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Claim type constants.
const (
	ClaimEventOccurrence         = "event_occurrence"
	ClaimIdentityConfirmation    = "identity_confirmation"
	ClaimLocationConfirmation    = "location_confirmation"
	ClaimTimelineConfirmation    = "timeline_confirmation"
	ClaimPatternOfConduct        = "pattern_of_conduct"
	ClaimContextualCorroboration = "contextual_corroboration"
	ClaimOther                   = "other"
)

var validClaimTypes = map[string]bool{
	ClaimEventOccurrence: true, ClaimIdentityConfirmation: true,
	ClaimLocationConfirmation: true, ClaimTimelineConfirmation: true,
	ClaimPatternOfConduct: true, ClaimContextualCorroboration: true,
	ClaimOther: true,
}

// Strength constants.
const (
	StrengthStrong    = "strong"
	StrengthModerate  = "moderate"
	StrengthWeak      = "weak"
	StrengthContested = "contested"
)

var validStrengths = map[string]bool{
	StrengthStrong: true, StrengthModerate: true,
	StrengthWeak: true, StrengthContested: true,
}

// Role in claim constants.
const (
	RolePrimary       = "primary"
	RoleSupporting    = "supporting"
	RoleContextual    = "contextual"
	RoleContradicting = "contradicting"
)

var validRolesInClaim = map[string]bool{
	RolePrimary: true, RoleSupporting: true,
	RoleContextual: true, RoleContradicting: true,
}

// CorroborationClaim represents a claim supported by multiple evidence items.
type CorroborationClaim struct {
	ID            uuid.UUID                `json:"id"`
	CaseID        uuid.UUID                `json:"case_id"`
	ClaimSummary  string                   `json:"claim_summary"`
	ClaimType     string                   `json:"claim_type"`
	Strength      string                   `json:"strength"`
	AnalysisNotes *string                  `json:"analysis_notes,omitempty"`
	Evidence      []CorroborationEvidence  `json:"evidence"`
	CreatedBy     uuid.UUID                `json:"created_by"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
}

// CorroborationEvidence links an evidence item to a claim.
type CorroborationEvidence struct {
	ID                uuid.UUID `json:"id"`
	ClaimID           uuid.UUID `json:"claim_id"`
	EvidenceID        uuid.UUID `json:"evidence_id"`
	RoleInClaim       string    `json:"role_in_claim"`
	ContributionNotes *string   `json:"contribution_notes,omitempty"`
	AddedBy           uuid.UUID `json:"added_by"`
	CreatedAt         time.Time `json:"created_at"`
}

// CorroborationClaimInput is the validated input for creating a claim.
type CorroborationClaimInput struct {
	ClaimSummary  string                          `json:"claim_summary"`
	ClaimType     string                          `json:"claim_type"`
	Strength      string                          `json:"strength"`
	AnalysisNotes *string                         `json:"analysis_notes"`
	Evidence      []CorroborationEvidenceInput     `json:"evidence"`
}

// CorroborationEvidenceInput is the input for linking evidence to a claim.
type CorroborationEvidenceInput struct {
	EvidenceID        string  `json:"evidence_id"`
	RoleInClaim       string  `json:"role_in_claim"`
	ContributionNotes *string `json:"contribution_notes"`
}

func ValidateCorroborationClaimInput(input CorroborationClaimInput) error {
	if strings.TrimSpace(input.ClaimSummary) == "" {
		return &ValidationError{Field: "claim_summary", Message: "claim summary is required"}
	}
	if !validClaimTypes[input.ClaimType] {
		return &ValidationError{Field: "claim_type", Message: fmt.Sprintf("invalid claim type: %s", input.ClaimType)}
	}
	if !validStrengths[input.Strength] {
		return &ValidationError{Field: "strength", Message: fmt.Sprintf("invalid strength: %s", input.Strength)}
	}
	if len(input.Evidence) < 2 {
		return &ValidationError{Field: "evidence", Message: "corroboration requires at least 2 evidence items"}
	}
	seen := make(map[string]bool)
	for i, e := range input.Evidence {
		if _, err := uuid.Parse(e.EvidenceID); err != nil {
			return &ValidationError{Field: fmt.Sprintf("evidence[%d].evidence_id", i), Message: "invalid UUID"}
		}
		if !validRolesInClaim[e.RoleInClaim] {
			return &ValidationError{Field: fmt.Sprintf("evidence[%d].role_in_claim", i), Message: fmt.Sprintf("invalid role: %s", e.RoleInClaim)}
		}
		if seen[e.EvidenceID] {
			return &ValidationError{Field: fmt.Sprintf("evidence[%d].evidence_id", i), Message: "duplicate evidence item"}
		}
		seen[e.EvidenceID] = true
	}
	if len(input.ClaimSummary) > 10000 {
		return &ValidationError{Field: "claim_summary", Message: "exceeds maximum length"}
	}
	return nil
}
