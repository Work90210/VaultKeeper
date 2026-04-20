package investigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OPSEC level constants.
const (
	OpsecStandard = "standard"
	OpsecElevated = "elevated"
	OpsecHighRisk = "high_risk"
)

var validOpsecLevels = map[string]bool{
	OpsecStandard: true, OpsecElevated: true, OpsecHighRisk: true,
}

// Threat level constants.
const (
	ThreatLow      = "low"
	ThreatMedium   = "medium"
	ThreatHigh     = "high"
	ThreatCritical = "critical"
)

var validThreatLevels = map[string]bool{
	ThreatLow: true, ThreatMedium: true,
	ThreatHigh: true, ThreatCritical: true,
}

// Roles that can read/write safety profiles (besides the investigator's own).
var safetyProfileWriteRoles = map[string]bool{
	"prosecutor": true, "judge": true,
}

var safetyProfileReadRoles = map[string]bool{
	"prosecutor": true, "judge": true, "investigator": true,
}

// SafetyProfile represents an investigator's operational security profile (Berkeley P4, S2).
type SafetyProfile struct {
	ID                      uuid.UUID  `json:"id"`
	CaseID                  uuid.UUID  `json:"case_id"`
	UserID                  uuid.UUID  `json:"user_id"`
	Pseudonym               *string    `json:"pseudonym,omitempty"`
	UsePseudonym            bool       `json:"use_pseudonym"`
	OpsecLevel              string     `json:"opsec_level"`
	RequiredVPN             bool       `json:"required_vpn"`
	RequiredTor             bool       `json:"required_tor"`
	ApprovedDevices         []string   `json:"approved_devices"`
	ProhibitedPlatforms     []string   `json:"prohibited_platforms"`
	ThreatLevel             string     `json:"threat_level"`
	ThreatNotes             *string    `json:"threat_notes,omitempty"`
	SafetyBriefingCompleted bool       `json:"safety_briefing_completed"`
	SafetyBriefingDate      *time.Time `json:"safety_briefing_date,omitempty"`
	SafetyOfficerID         *uuid.UUID `json:"safety_officer_id,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// SafetyProfileInput is the validated input for upserting a safety profile.
type SafetyProfileInput struct {
	Pseudonym               *string  `json:"pseudonym"`
	UsePseudonym            *bool    `json:"use_pseudonym"`
	OpsecLevel              string   `json:"opsec_level"`
	RequiredVPN             *bool    `json:"required_vpn"`
	RequiredTor             *bool    `json:"required_tor"`
	ApprovedDevices         []string `json:"approved_devices"`
	ProhibitedPlatforms     []string `json:"prohibited_platforms"`
	ThreatLevel             *string  `json:"threat_level"`
	ThreatNotes             *string  `json:"threat_notes"`
	SafetyBriefingCompleted *bool    `json:"safety_briefing_completed"`
	SafetyBriefingDate      *string  `json:"safety_briefing_date"`
	SafetyOfficerID         *string  `json:"safety_officer_id"`
}

// SafetyProfileWarning is a non-fatal advisory.
type SafetyProfileWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func ValidateSafetyProfileInput(input SafetyProfileInput) ([]SafetyProfileWarning, error) {
	var warnings []SafetyProfileWarning

	if !validOpsecLevels[input.OpsecLevel] {
		return nil, &ValidationError{Field: "opsec_level", Message: fmt.Sprintf("invalid opsec level: %s", input.OpsecLevel)}
	}

	if input.ThreatLevel != nil && *input.ThreatLevel != "" && !validThreatLevels[*input.ThreatLevel] {
		return nil, &ValidationError{Field: "threat_level", Message: fmt.Sprintf("invalid threat level: %s", *input.ThreatLevel)}
	}

	// Advisory warnings for elevated/high_risk without pseudonym
	if input.OpsecLevel != OpsecStandard {
		if input.Pseudonym == nil || *input.Pseudonym == "" {
			warnings = append(warnings, SafetyProfileWarning{
				Field:   "pseudonym",
				Message: "pseudonym recommended for elevated/high_risk opsec level",
			})
		}
	}

	if input.SafetyOfficerID != nil && *input.SafetyOfficerID != "" {
		if _, err := uuid.Parse(*input.SafetyOfficerID); err != nil {
			return nil, &ValidationError{Field: "safety_officer_id", Message: "invalid UUID"}
		}
	}

	if input.SafetyBriefingDate != nil && *input.SafetyBriefingDate != "" {
		if _, err := time.Parse(time.RFC3339, *input.SafetyBriefingDate); err != nil {
			// Also accept YYYY-MM-DD from HTML date inputs
			if _, err2 := time.Parse("2006-01-02", *input.SafetyBriefingDate); err2 != nil {
				return nil, &ValidationError{Field: "safety_briefing_date", Message: "must be YYYY-MM-DD or RFC 3339 format"}
			}
		}
	}

	if input.Pseudonym != nil && len(*input.Pseudonym) > 200 {
		return nil, &ValidationError{Field: "pseudonym", Message: "exceeds maximum length"}
	}

	return warnings, nil
}

// CanReadSafetyProfile checks if a role can read safety profiles.
func CanReadSafetyProfile(role string) bool {
	return safetyProfileReadRoles[role]
}

// CanWriteSafetyProfile checks if a role can write safety profiles.
func CanWriteSafetyProfile(role string) bool {
	return safetyProfileWriteRoles[role]
}
