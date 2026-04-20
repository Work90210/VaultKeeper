package investigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Analysis type constants.
const (
	AnalysisFactualFinding       = "factual_finding"
	AnalysisPatternAnalysis      = "pattern_analysis"
	AnalysisTimelineRecon        = "timeline_reconstruction"
	AnalysisGeographic           = "geographic_analysis"
	AnalysisNetwork              = "network_analysis"
	AnalysisLegalAssessment      = "legal_assessment"
	AnalysisCredibility          = "credibility_assessment"
	AnalysisGapIdentification    = "gap_identification"
	AnalysisHypothesisTesting    = "hypothesis_testing"
	AnalysisOther                = "other"
)

var validAnalysisTypes = map[string]bool{
	AnalysisFactualFinding: true, AnalysisPatternAnalysis: true,
	AnalysisTimelineRecon: true, AnalysisGeographic: true,
	AnalysisNetwork: true, AnalysisLegalAssessment: true,
	AnalysisCredibility: true, AnalysisGapIdentification: true,
	AnalysisHypothesisTesting: true, AnalysisOther: true,
}

// Analysis status constants.
const (
	AnalysisStatusDraft      = "draft"
	AnalysisStatusInReview   = "in_review"
	AnalysisStatusApproved   = "approved"
	AnalysisStatusSuperseded = "superseded"
)

var validAnalysisStatuses = map[string]bool{
	AnalysisStatusDraft: true, AnalysisStatusInReview: true,
	AnalysisStatusApproved: true, AnalysisStatusSuperseded: true,
}

// AnalysisNote represents documented analytical reasoning (Berkeley Protocol Phase 6).
type AnalysisNote struct {
	ID                     uuid.UUID    `json:"id"`
	CaseID                 uuid.UUID    `json:"case_id"`
	Title                  string       `json:"title"`
	AnalysisType           string       `json:"analysis_type"`
	Content                string       `json:"content"`
	Methodology            *string      `json:"methodology,omitempty"`
	RelatedEvidenceIDs     []uuid.UUID  `json:"related_evidence_ids"`
	RelatedInquiryIDs      []uuid.UUID  `json:"related_inquiry_ids"`
	RelatedAssessmentIDs   []uuid.UUID  `json:"related_assessment_ids"`
	RelatedVerificationIDs []uuid.UUID  `json:"related_verification_ids"`
	Status                 string       `json:"status"`
	SupersededBy           *uuid.UUID   `json:"superseded_by,omitempty"`
	AuthorID               uuid.UUID    `json:"author_id"`
	ReviewerID             *uuid.UUID   `json:"reviewer_id,omitempty"`
	ReviewedAt             *time.Time   `json:"reviewed_at,omitempty"`
	CreatedAt              time.Time    `json:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at"`
}

// AnalysisNoteInput is the validated input for creating/updating an analysis note.
type AnalysisNoteInput struct {
	Title                  string      `json:"title"`
	AnalysisType           string      `json:"analysis_type"`
	Content                string      `json:"content"`
	Methodology            *string     `json:"methodology"`
	RelatedEvidenceIDs     []string    `json:"related_evidence_ids"`
	RelatedInquiryIDs      []string    `json:"related_inquiry_ids"`
	RelatedAssessmentIDs   []string    `json:"related_assessment_ids"`
	RelatedVerificationIDs []string    `json:"related_verification_ids"`
}

func ValidateAnalysisNoteInput(input AnalysisNoteInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Field: "title", Message: "title is required"}
	}
	if !validAnalysisTypes[input.AnalysisType] {
		return &ValidationError{Field: "analysis_type", Message: fmt.Sprintf("invalid analysis type: %s", input.AnalysisType)}
	}
	if strings.TrimSpace(input.Content) == "" {
		return &ValidationError{Field: "content", Message: "content is required"}
	}
	if len(input.Title) > 500 {
		return &ValidationError{Field: "title", Message: "title exceeds maximum length"}
	}
	const maxRelatedIDs = 100
	if len(input.RelatedEvidenceIDs) > maxRelatedIDs || len(input.RelatedInquiryIDs) > maxRelatedIDs ||
		len(input.RelatedAssessmentIDs) > maxRelatedIDs || len(input.RelatedVerificationIDs) > maxRelatedIDs {
		return &ValidationError{Field: "related_ids", Message: "too many related items (max 100 per type)"}
	}
	if len(input.Content) > 50000 {
		return &ValidationError{Field: "content", Message: "content exceeds maximum length"}
	}
	for i, id := range input.RelatedEvidenceIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("related_evidence_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	for i, id := range input.RelatedInquiryIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("related_inquiry_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	for i, id := range input.RelatedAssessmentIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("related_assessment_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	for i, id := range input.RelatedVerificationIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("related_verification_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	return nil
}
