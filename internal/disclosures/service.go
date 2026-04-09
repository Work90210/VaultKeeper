package disclosures

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// CustodyRecorder logs custody chain events.
type CustodyRecorder interface {
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// Notifier sends notifications to users.
type Notifier interface {
	NotifyDisclosure(ctx context.Context, caseID uuid.UUID, disclosedTo, title, body string) error
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Service orchestrates disclosure operations.
type Service struct {
	repo    Repository
	custody CustodyRecorder
	notify  Notifier
	logger  *slog.Logger
}

// NewService creates a new disclosure service.
func NewService(repo Repository, custody CustodyRecorder, notify Notifier, logger *slog.Logger) *Service {
	return &Service{
		repo:    repo,
		custody: custody,
		notify:  notify,
		logger:  logger,
	}
}

// Create creates a new disclosure (prosecutor shares evidence with defence).
func (s *Service) Create(ctx context.Context, input CreateDisclosureInput, actorID string) (Disclosure, error) {
	if err := s.validateCreateInput(input); err != nil {
		return Disclosure{}, err
	}

	// Verify all evidence belongs to the case
	belongs, err := s.repo.EvidenceBelongsToCase(ctx, input.CaseID, input.EvidenceIDs)
	if err != nil {
		return Disclosure{}, fmt.Errorf("check evidence ownership: %w", err)
	}
	if !belongs {
		return Disclosure{}, &ValidationError{Field: "evidence_ids", Message: "one or more evidence items do not belong to this case"}
	}

	disclosedBy, err := uuid.Parse(actorID)
	if err != nil {
		return Disclosure{}, &ValidationError{Field: "disclosed_by", Message: "invalid user ID"}
	}

	d := Disclosure{
		CaseID:      input.CaseID,
		EvidenceIDs: input.EvidenceIDs,
		DisclosedTo: input.DisclosedTo,
		DisclosedBy: disclosedBy,
		Notes:       input.Notes,
		Redacted:    input.Redacted,
	}

	created, err := s.repo.Create(ctx, d)
	if err != nil {
		return Disclosure{}, fmt.Errorf("create disclosure: %w", err)
	}

	// Record custody events for each evidence item
	for _, evidenceID := range input.EvidenceIDs {
		s.recordEvidenceCustody(ctx, input.CaseID, evidenceID, "disclosed", actorID, map[string]string{
			"disclosed_to": input.DisclosedTo,
			"redacted":     fmt.Sprintf("%t", input.Redacted),
		})
	}

	// Case-level custody event
	s.recordCaseCustody(ctx, input.CaseID, "disclosure_created", actorID, map[string]string{
		"disclosed_to":   input.DisclosedTo,
		"evidence_count": fmt.Sprintf("%d", len(input.EvidenceIDs)),
		"redacted":       fmt.Sprintf("%t", input.Redacted),
	})

	// Notify disclosed party
	if s.notify != nil {
		title := "Evidence disclosed"
		body := fmt.Sprintf("%d evidence item(s) have been disclosed to you.", len(input.EvidenceIDs))
		if err := s.notify.NotifyDisclosure(ctx, input.CaseID, input.DisclosedTo, title, body); err != nil {
			s.logger.Error("failed to notify disclosure", "case_id", input.CaseID, "error", err)
		}
	}

	return created, nil
}

// Get retrieves a disclosure by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Disclosure, error) {
	return s.repo.FindByID(ctx, id)
}

// List retrieves disclosures for a case.
func (s *Service) List(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error) {
	return s.repo.FindByCase(ctx, caseID, page)
}

func (s *Service) validateCreateInput(input CreateDisclosureInput) error {
	if input.CaseID == uuid.Nil {
		return &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if len(input.EvidenceIDs) == 0 {
		return &ValidationError{Field: "evidence_ids", Message: "at least one evidence item is required"}
	}
	if input.DisclosedTo == "" {
		return &ValidationError{Field: "disclosed_to", Message: "recipient is required"}
	}
	return nil
}

func (s *Service) recordEvidenceCustody(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event",
			"case_id", caseID, "evidence_id", evidenceID, "action", action, "error", err)
	}
}

func (s *Service) recordCaseCustody(ctx context.Context, caseID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordCaseEvent(ctx, caseID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event", "case_id", caseID, "action", action, "error", err)
	}
}
