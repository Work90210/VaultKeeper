package investigation

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Source credibility levels.
const (
	CredibilityEstablished = "established"
	CredibilityCredible    = "credible"
	CredibilityUncertain   = "uncertain"
	CredibilityUnreliable  = "unreliable"
	CredibilityUnassessed  = "unassessed"
)

var validCredibilities = map[string]bool{
	CredibilityEstablished: true, CredibilityCredible: true,
	CredibilityUncertain: true, CredibilityUnreliable: true,
	CredibilityUnassessed: true,
}

// Assessment recommendation actions.
const (
	RecommendCollect      = "collect"
	RecommendMonitor      = "monitor"
	RecommendDeprioritize = "deprioritize"
	RecommendDiscard      = "discard"
)

var validRecommendations = map[string]bool{
	RecommendCollect: true, RecommendMonitor: true,
	RecommendDeprioritize: true, RecommendDiscard: true,
}

// EvidenceAssessment represents a preliminary assessment (Berkeley Protocol Phase 2).
type EvidenceAssessment struct {
	ID                    uuid.UUID  `json:"id"`
	EvidenceID            uuid.UUID  `json:"evidence_id"`
	CaseID                uuid.UUID  `json:"case_id"`
	RelevanceScore        int        `json:"relevance_score"`
	RelevanceRationale    string     `json:"relevance_rationale"`
	ReliabilityScore      int        `json:"reliability_score"`
	ReliabilityRationale  string     `json:"reliability_rationale"`
	SourceCredibility     string     `json:"source_credibility"`
	MisleadingIndicators  []string   `json:"misleading_indicators"`
	Recommendation        string     `json:"recommendation"`
	Methodology           *string    `json:"methodology,omitempty"`
	AssessedBy            uuid.UUID  `json:"assessed_by"`
	ReviewedBy            *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt            *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// AssessmentInput is the validated input for creating/updating an assessment.
type AssessmentInput struct {
	RelevanceScore       int      `json:"relevance_score"`
	RelevanceRationale   string   `json:"relevance_rationale"`
	ReliabilityScore     int      `json:"reliability_score"`
	ReliabilityRationale string   `json:"reliability_rationale"`
	SourceCredibility    string   `json:"source_credibility"`
	MisleadingIndicators []string `json:"misleading_indicators"`
	Recommendation       string   `json:"recommendation"`
	Methodology          *string  `json:"methodology"`
}

func ValidateAssessmentInput(input AssessmentInput) error {
	if input.RelevanceScore < 1 || input.RelevanceScore > 5 {
		return &ValidationError{Field: "relevance_score", Message: "must be between 1 and 5"}
	}
	if strings.TrimSpace(input.RelevanceRationale) == "" {
		return &ValidationError{Field: "relevance_rationale", Message: "relevance rationale is required"}
	}
	if input.ReliabilityScore < 1 || input.ReliabilityScore > 5 {
		return &ValidationError{Field: "reliability_score", Message: "must be between 1 and 5"}
	}
	if strings.TrimSpace(input.ReliabilityRationale) == "" {
		return &ValidationError{Field: "reliability_rationale", Message: "reliability rationale is required"}
	}
	if !validCredibilities[input.SourceCredibility] {
		return &ValidationError{Field: "source_credibility", Message: fmt.Sprintf("invalid source credibility: %s", input.SourceCredibility)}
	}
	if !validRecommendations[input.Recommendation] {
		return &ValidationError{Field: "recommendation", Message: fmt.Sprintf("invalid recommendation: %s", input.Recommendation)}
	}
	if len(input.RelevanceRationale) > 10000 {
		return &ValidationError{Field: "relevance_rationale", Message: "exceeds maximum length"}
	}
	if len(input.ReliabilityRationale) > 10000 {
		return &ValidationError{Field: "reliability_rationale", Message: "exceeds maximum length"}
	}
	return nil
}
