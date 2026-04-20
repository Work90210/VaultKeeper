package investigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Verification type constants.
const (
	VerTypeSourceAuth     = "source_authentication"
	VerTypeContentVerify  = "content_verification"
	VerTypeReverseImage   = "reverse_image_search"
	VerTypeGeolocation    = "geolocation_verification"
	VerTypeChronolocation = "chronolocation"
	VerTypeMetadata       = "metadata_analysis"
	VerTypeWitness        = "witness_corroboration"
	VerTypeExpert         = "expert_analysis"
	VerTypeCrossRef       = "open_source_cross_reference"
	VerTypeOther          = "other"
)

var validVerificationTypes = map[string]bool{
	VerTypeSourceAuth: true, VerTypeContentVerify: true, VerTypeReverseImage: true,
	VerTypeGeolocation: true, VerTypeChronolocation: true, VerTypeMetadata: true,
	VerTypeWitness: true, VerTypeExpert: true, VerTypeCrossRef: true, VerTypeOther: true,
}

// Finding constants.
const (
	FindingAuthentic        = "authentic"
	FindingLikelyAuthentic  = "likely_authentic"
	FindingInconclusive     = "inconclusive"
	FindingLikelyManipulated = "likely_manipulated"
	FindingManipulated      = "manipulated"
	FindingUnableToVerify   = "unable_to_verify"
)

var validFindings = map[string]bool{
	FindingAuthentic: true, FindingLikelyAuthentic: true,
	FindingInconclusive: true, FindingLikelyManipulated: true,
	FindingManipulated: true, FindingUnableToVerify: true,
}

// Confidence level constants.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

var validConfidenceLevels = map[string]bool{
	ConfidenceHigh: true, ConfidenceMedium: true, ConfidenceLow: true,
}

// VerificationRecord represents a structured verification record (Berkeley Protocol Phase 5).
type VerificationRecord struct {
	ID               uuid.UUID  `json:"id"`
	EvidenceID       uuid.UUID  `json:"evidence_id"`
	CaseID           uuid.UUID  `json:"case_id"`
	VerificationType string     `json:"verification_type"`
	Methodology      string     `json:"methodology"`
	ToolsUsed        []string   `json:"tools_used"`
	SourcesConsulted []string   `json:"sources_consulted"`
	Finding          string     `json:"finding"`
	FindingRationale string     `json:"finding_rationale"`
	ConfidenceLevel  string     `json:"confidence_level"`
	Limitations      *string    `json:"limitations,omitempty"`
	Caveats          []string   `json:"caveats"`
	VerifiedBy       uuid.UUID  `json:"verified_by"`
	Reviewer         *uuid.UUID `json:"reviewer,omitempty"`
	ReviewerApproved *bool      `json:"reviewer_approved,omitempty"`
	ReviewerNotes    *string    `json:"reviewer_notes,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// VerificationRecordInput is the validated input for creating a verification record.
type VerificationRecordInput struct {
	VerificationType string   `json:"verification_type"`
	Methodology      string   `json:"methodology"`
	ToolsUsed        []string `json:"tools_used"`
	SourcesConsulted []string `json:"sources_consulted"`
	Finding          string   `json:"finding"`
	FindingRationale string   `json:"finding_rationale"`
	ConfidenceLevel  string   `json:"confidence_level"`
	Limitations      *string  `json:"limitations"`
	Caveats          []string `json:"caveats"`
}

func ValidateVerificationRecordInput(input VerificationRecordInput) error {
	if !validVerificationTypes[input.VerificationType] {
		return &ValidationError{Field: "verification_type", Message: fmt.Sprintf("invalid verification type: %s", input.VerificationType)}
	}
	if strings.TrimSpace(input.Methodology) == "" {
		return &ValidationError{Field: "methodology", Message: "methodology is required"}
	}
	if !validFindings[input.Finding] {
		return &ValidationError{Field: "finding", Message: fmt.Sprintf("invalid finding: %s", input.Finding)}
	}
	if strings.TrimSpace(input.FindingRationale) == "" {
		return &ValidationError{Field: "finding_rationale", Message: "finding rationale is required"}
	}
	if !validConfidenceLevels[input.ConfidenceLevel] {
		return &ValidationError{Field: "confidence_level", Message: fmt.Sprintf("invalid confidence level: %s", input.ConfidenceLevel)}
	}
	if len(input.Methodology) > 10000 {
		return &ValidationError{Field: "methodology", Message: "exceeds maximum length"}
	}
	if len(input.FindingRationale) > 10000 {
		return &ValidationError{Field: "finding_rationale", Message: "exceeds maximum length"}
	}
	return nil
}

// ShouldAutoVerify returns true when the finding warrants auto-upgrading
// the capture metadata verification_status to 'verified'.
func (v VerificationRecordInput) ShouldAutoVerify() bool {
	return v.Finding == FindingAuthentic && v.ConfidenceLevel == ConfidenceHigh
}
