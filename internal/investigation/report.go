package investigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Report type constants.
const (
	ReportInterim       = "interim"
	ReportFinal         = "final"
	ReportSupplementary = "supplementary"
	ReportExpertOpinion = "expert_opinion"
)

var validReportTypes = map[string]bool{
	ReportInterim: true, ReportFinal: true,
	ReportSupplementary: true, ReportExpertOpinion: true,
}

// Report status constants.
const (
	ReportStatusDraft     = "draft"
	ReportStatusInReview  = "in_review"
	ReportStatusApproved  = "approved"
	ReportStatusPublished = "published"
	ReportStatusWithdrawn = "withdrawn"
)

var validReportStatuses = map[string]bool{
	ReportStatusDraft: true, ReportStatusInReview: true,
	ReportStatusApproved: true, ReportStatusPublished: true,
	ReportStatusWithdrawn: true,
}

// Report section type constants.
const (
	SectionPurpose        = "purpose"
	SectionMethodology    = "methodology"
	SectionFindings       = "findings"
	SectionEvidenceSummary = "evidence_summary"
	SectionAnalysis       = "analysis"
	SectionConclusions    = "conclusions"
	SectionRecommendations = "recommendations"
	SectionLimitations    = "limitations"
	SectionAppendix       = "appendix"
	SectionCustom         = "custom"
)

var validSectionTypes = map[string]bool{
	SectionPurpose: true, SectionMethodology: true, SectionFindings: true,
	SectionEvidenceSummary: true, SectionAnalysis: true, SectionConclusions: true,
	SectionRecommendations: true, SectionLimitations: true,
	SectionAppendix: true, SectionCustom: true,
}

// ReportSection is a single section of a report.
type ReportSection struct {
	SectionType string `json:"section_type"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Order       int    `json:"order"`
}

// InvestigationReport represents a structured investigation report (Berkeley Protocol R1, R3).
type InvestigationReport struct {
	ID                    uuid.UUID       `json:"id"`
	CaseID                uuid.UUID       `json:"case_id"`
	Title                 string          `json:"title"`
	ReportType            string          `json:"report_type"`
	Sections              []ReportSection `json:"sections"`
	Limitations           []string        `json:"limitations"`
	Caveats               []string        `json:"caveats"`
	Assumptions           []string        `json:"assumptions"`
	ReferencedEvidenceIDs []uuid.UUID     `json:"referenced_evidence_ids"`
	ReferencedAnalysisIDs []uuid.UUID     `json:"referenced_analysis_ids"`
	Status                string          `json:"status"`
	AuthorID              uuid.UUID       `json:"author_id"`
	ReviewerID            *uuid.UUID      `json:"reviewer_id,omitempty"`
	ReviewedAt            *time.Time      `json:"reviewed_at,omitempty"`
	ApprovedBy            *uuid.UUID      `json:"approved_by,omitempty"`
	ApprovedAt            *time.Time      `json:"approved_at,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// ReportInput is the validated input for creating/updating a report.
type ReportInput struct {
	Title                 string          `json:"title"`
	ReportType            string          `json:"report_type"`
	Sections              []ReportSection `json:"sections"`
	Limitations           []string        `json:"limitations"`
	Caveats               []string        `json:"caveats"`
	Assumptions           []string        `json:"assumptions"`
	ReferencedEvidenceIDs []string        `json:"referenced_evidence_ids"`
	ReferencedAnalysisIDs []string        `json:"referenced_analysis_ids"`
}

func ValidateReportInput(input ReportInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return &ValidationError{Field: "title", Message: "title is required"}
	}
	if !validReportTypes[input.ReportType] {
		return &ValidationError{Field: "report_type", Message: fmt.Sprintf("invalid report type: %s", input.ReportType)}
	}
	if len(input.Sections) == 0 {
		return &ValidationError{Field: "sections", Message: "at least one section is required"}
	}
	for i, s := range input.Sections {
		if !validSectionTypes[s.SectionType] {
			return &ValidationError{Field: fmt.Sprintf("sections[%d].section_type", i), Message: fmt.Sprintf("invalid section type: %s", s.SectionType)}
		}
		if strings.TrimSpace(s.Title) == "" {
			return &ValidationError{Field: fmt.Sprintf("sections[%d].title", i), Message: "section title is required"}
		}
		if len(s.Content) > 50000 {
			return &ValidationError{Field: fmt.Sprintf("sections[%d].content", i), Message: "section content exceeds maximum length"}
		}
	}
	if len(input.Title) > 500 {
		return &ValidationError{Field: "title", Message: "title exceeds maximum length"}
	}
	for i, id := range input.ReferencedEvidenceIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("referenced_evidence_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	for i, id := range input.ReferencedAnalysisIDs {
		if _, err := uuid.Parse(id); err != nil {
			return &ValidationError{Field: fmt.Sprintf("referenced_analysis_ids[%d]", i), Message: "invalid UUID"}
		}
	}
	return nil
}
