package investigation

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InquiryLog represents a documented search session (Berkeley Protocol Phase 1).
type InquiryLog struct {
	ID                uuid.UUID  `json:"id"`
	CaseID            uuid.UUID  `json:"case_id"`
	EvidenceID        *uuid.UUID `json:"evidence_id,omitempty"`
	SearchStrategy    string     `json:"search_strategy"`
	SearchKeywords    []string   `json:"search_keywords"`
	SearchOperators   string     `json:"search_operators,omitempty"`
	SearchTool        string     `json:"search_tool"`
	SearchToolVersion *string    `json:"search_tool_version,omitempty"`
	SearchURL         *string    `json:"search_url,omitempty"`
	SearchStartedAt   time.Time  `json:"search_started_at"`
	SearchEndedAt     *time.Time `json:"search_ended_at,omitempty"`
	ResultsCount      *int       `json:"results_count,omitempty"`
	ResultsRelevant   *int       `json:"results_relevant,omitempty"`
	ResultsCollected  *int       `json:"results_collected,omitempty"`
	Objective         string     `json:"objective"`
	Notes             *string    `json:"notes,omitempty"`
	AssignedTo        *uuid.UUID `json:"assigned_to,omitempty"`
	Priority          string     `json:"priority"`
	SealedStatus      string     `json:"sealed_status"`
	SealedAt          *time.Time `json:"sealed_at,omitempty"`
	PerformedBy       uuid.UUID  `json:"performed_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// InquiryLogInput is the validated input for creating/updating an inquiry log.
type InquiryLogInput struct {
	EvidenceID        *string  `json:"evidence_id"`
	SearchStrategy    string   `json:"search_strategy"`
	SearchKeywords    []string `json:"search_keywords"`
	SearchOperators   *string  `json:"search_operators"`
	SearchTool        string   `json:"search_tool"`
	SearchToolVersion *string  `json:"search_tool_version"`
	SearchURL         *string  `json:"search_url"`
	SearchStartedAt   string   `json:"search_started_at"`
	SearchEndedAt     *string  `json:"search_ended_at"`
	ResultsCount      *int     `json:"results_count"`
	ResultsRelevant   *int     `json:"results_relevant"`
	ResultsCollected  *int     `json:"results_collected"`
	Objective         string   `json:"objective"`
	Notes             *string  `json:"notes"`
	AssignedTo        *string  `json:"assigned_to"`
	Priority          *string  `json:"priority"`
}

func ValidateInquiryLogInput(input InquiryLogInput) error {
	if strings.TrimSpace(input.SearchStrategy) == "" {
		return &ValidationError{Field: "search_strategy", Message: "search strategy is required"}
	}
	if strings.TrimSpace(input.SearchTool) == "" {
		return &ValidationError{Field: "search_tool", Message: "search tool is required"}
	}
	if strings.TrimSpace(input.Objective) == "" {
		return &ValidationError{Field: "objective", Message: "objective is required"}
	}
	if input.SearchStartedAt == "" {
		return &ValidationError{Field: "search_started_at", Message: "search start time is required"}
	}
	if _, err := time.Parse(time.RFC3339, input.SearchStartedAt); err != nil {
		return &ValidationError{Field: "search_started_at", Message: "must be RFC 3339 format"}
	}
	if input.SearchEndedAt != nil && *input.SearchEndedAt != "" {
		endedAt, err := time.Parse(time.RFC3339, *input.SearchEndedAt)
		if err != nil {
			return &ValidationError{Field: "search_ended_at", Message: "must be RFC 3339 format"}
		}
		startedAt, _ := time.Parse(time.RFC3339, input.SearchStartedAt)
		if endedAt.Before(startedAt) {
			return &ValidationError{Field: "search_ended_at", Message: "end time cannot be before start time"}
		}
	}
	if input.SearchURL != nil && *input.SearchURL != "" {
		if err := validateHTTPURL(*input.SearchURL, "search_url"); err != nil {
			return err
		}
	}
	if input.ResultsRelevant != nil && input.ResultsCount != nil && *input.ResultsRelevant > *input.ResultsCount {
		return &ValidationError{Field: "results_relevant", Message: "relevant results cannot exceed total results"}
	}
	if len(input.SearchKeywords) > 200 {
		return &ValidationError{Field: "search_keywords", Message: "too many keywords (max 200)"}
	}
	if len(input.SearchStrategy) > 10000 {
		return &ValidationError{Field: "search_strategy", Message: "search strategy exceeds maximum length"}
	}
	if len(input.Objective) > 5000 {
		return &ValidationError{Field: "objective", Message: "objective exceeds maximum length"}
	}
	if input.AssignedTo != nil && *input.AssignedTo != "" {
		if _, err := uuid.Parse(*input.AssignedTo); err != nil {
			return &ValidationError{Field: "assigned_to", Message: "must be a valid UUID"}
		}
	}
	validPriorities := map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}
	if input.Priority != nil && *input.Priority != "" && !validPriorities[*input.Priority] {
		return &ValidationError{Field: "priority", Message: "must be one of: low, normal, high, urgent"}
	}
	return nil
}

func validateHTTPURL(u string, field string) error {
	parsed, err := url.Parse(u)
	if err != nil {
		return &ValidationError{Field: field, Message: "invalid URL"}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return &ValidationError{Field: field, Message: "URL must use http or https scheme"}
	}
	return nil
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
