package investigation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Template type constants.
const (
	TemplateInvestigationPlan = "investigation_plan"
	TemplateThreatAssessment  = "threat_assessment"
	TemplateDigitalLandscape  = "digital_landscape"
)

var validTemplateTypes = map[string]bool{
	TemplateInvestigationPlan: true, TemplateThreatAssessment: true,
	TemplateDigitalLandscape: true,
}

// Template instance status constants.
const (
	InstanceStatusDraft     = "draft"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusArchived  = "archived"
)

var validInstanceStatuses = map[string]bool{
	InstanceStatusDraft: true, InstanceStatusActive: true,
	InstanceStatusCompleted: true, InstanceStatusArchived: true,
}

// InvestigationTemplate defines the structure of a template.
type InvestigationTemplate struct {
	ID               uuid.UUID      `json:"id"`
	TemplateType     string         `json:"template_type"`
	Name             string         `json:"name"`
	Description      *string        `json:"description,omitempty"`
	Version          int            `json:"version"`
	IsDefault        bool           `json:"is_default"`
	SchemaDefinition map[string]any `json:"schema_definition"`
	CreatedBy        *uuid.UUID     `json:"created_by,omitempty"`
	IsSystemTemplate bool           `json:"is_system_template"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// TemplateInstance is a filled-in template for a specific case.
type TemplateInstance struct {
	ID         uuid.UUID      `json:"id"`
	TemplateID uuid.UUID      `json:"template_id"`
	CaseID     uuid.UUID      `json:"case_id"`
	Content    map[string]any `json:"content"`
	Status     string         `json:"status"`
	PreparedBy uuid.UUID      `json:"prepared_by"`
	ApprovedBy *uuid.UUID     `json:"approved_by,omitempty"`
	ApprovedAt *time.Time     `json:"approved_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// TemplateInstanceInput is the validated input for creating/updating a template instance.
type TemplateInstanceInput struct {
	TemplateID string         `json:"template_id"`
	Content    map[string]any `json:"content"`
}

func ValidateTemplateInstanceInput(input TemplateInstanceInput) error {
	if _, err := uuid.Parse(input.TemplateID); err != nil {
		return &ValidationError{Field: "template_id", Message: "invalid template ID"}
	}
	if input.Content == nil {
		return &ValidationError{Field: "content", Message: "content is required"}
	}
	encoded, err := json.Marshal(input.Content)
	if err != nil {
		return &ValidationError{Field: "content", Message: "invalid content structure"}
	}
	if len(encoded) > 512*1024 {
		return &ValidationError{Field: "content", Message: "content exceeds 512 KB limit"}
	}
	return nil
}

// TemplateInstanceStatusInput is the input for updating instance status.
type TemplateInstanceStatusInput struct {
	Status string `json:"status"`
}

func ValidateTemplateInstanceStatusInput(input TemplateInstanceStatusInput) error {
	if !validInstanceStatuses[input.Status] {
		return &ValidationError{Field: "status", Message: fmt.Sprintf("invalid status: %s", input.Status)}
	}
	return nil
}

// TemplateTypeLabel returns a human-readable label for a template type.
func TemplateTypeLabel(t string) string {
	switch t {
	case TemplateInvestigationPlan:
		return "Investigation Plan"
	case TemplateThreatAssessment:
		return "Threat Assessment"
	case TemplateDigitalLandscape:
		return "Digital Landscape Assessment"
	default:
		s := strings.ReplaceAll(t, "_", " ")
		if len(s) > 0 {
			return strings.ToUpper(s[:1]) + s[1:]
		}
		return s
	}
}
