package cases

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type CustodyRecorder interface {
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error
}

type Service struct {
	repo              Repository
	custody           CustodyRecorder
	referenceCodeExpr *regexp.Regexp
}

func NewService(repo Repository, custody CustodyRecorder, referenceCodeRegex string) (*Service, error) {
	expr, err := regexp.Compile(referenceCodeRegex)
	if err != nil {
		return nil, fmt.Errorf("compile reference code regex: %w", err)
	}
	return &Service{
		repo:              repo,
		custody:           custody,
		referenceCodeExpr: expr,
	}, nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (s *Service) CreateCase(ctx context.Context, input CreateCaseInput, createdBy string) (Case, error) {
	if err := s.validateCreateInput(input); err != nil {
		return Case{}, err
	}

	c := Case{
		ReferenceCode: strings.TrimSpace(input.ReferenceCode),
		Title:         html.EscapeString(strings.TrimSpace(input.Title)),
		Description:   html.EscapeString(strings.TrimSpace(input.Description)),
		Jurisdiction:  html.EscapeString(strings.TrimSpace(input.Jurisdiction)),
		Status:        StatusActive,
		CreatedBy:     createdBy,
	}

	result, err := s.repo.Create(ctx, c)
	if err != nil {
		return Case{}, err
	}

	if s.custody != nil {
		_ = s.custody.RecordCaseEvent(ctx, result.ID, "case_created", createdBy, map[string]string{
			"reference_code": result.ReferenceCode,
			"title":          result.Title,
		})
	}

	return result, nil
}

func (s *Service) GetCase(ctx context.Context, id uuid.UUID) (Case, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) ListCases(ctx context.Context, filter CaseFilter, page Pagination) (PaginatedResult[Case], error) {
	items, total, err := s.repo.FindAll(ctx, filter, page)
	if err != nil {
		return PaginatedResult[Case]{}, err
	}

	page = ClampPagination(page)
	hasMore := len(items) == page.Limit && total > page.Limit

	var nextCursor string
	if hasMore && len(items) > 0 {
		nextCursor = EncodeCursor(items[len(items)-1].ID)
	}

	return PaginatedResult[Case]{
		Items:      items,
		TotalCount: total,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *Service) UpdateCase(ctx context.Context, id uuid.UUID, input UpdateCaseInput, updatedBy string) (Case, error) {
	if err := s.validateUpdateInput(ctx, id, input); err != nil {
		return Case{}, err
	}

	// Sanitize
	if input.Title != nil {
		sanitized := html.EscapeString(strings.TrimSpace(*input.Title))
		input.Title = &sanitized
	}
	if input.Description != nil {
		sanitized := html.EscapeString(strings.TrimSpace(*input.Description))
		input.Description = &sanitized
	}
	if input.Jurisdiction != nil {
		sanitized := html.EscapeString(strings.TrimSpace(*input.Jurisdiction))
		input.Jurisdiction = &sanitized
	}

	result, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return Case{}, err
	}

	if s.custody != nil {
		changed := make(map[string]string)
		if input.Title != nil {
			changed["title"] = *input.Title
		}
		if input.Description != nil {
			changed["description"] = "updated"
		}
		if input.Jurisdiction != nil {
			changed["jurisdiction"] = *input.Jurisdiction
		}
		if input.Status != nil {
			changed["status"] = *input.Status
		}
		_ = s.custody.RecordCaseEvent(ctx, id, "case_updated", updatedBy, changed)
	}

	return result, nil
}

func (s *Service) ArchiveCase(ctx context.Context, id uuid.UUID, archivedBy string) error {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if c.LegalHold {
		return &ValidationError{Field: "legal_hold", Message: "cannot archive a case with active legal hold"}
	}

	if c.Status == StatusArchived {
		return nil // idempotent
	}

	if !IsValidStatusTransition(c.Status, StatusArchived) {
		return &ValidationError{Field: "status", Message: fmt.Sprintf("cannot transition from %s to archived", c.Status)}
	}

	if err := s.repo.Archive(ctx, id); err != nil {
		return err
	}

	if s.custody != nil {
		_ = s.custody.RecordCaseEvent(ctx, id, "case_archived", archivedBy, map[string]string{})
	}

	return nil
}

func (s *Service) SetLegalHold(ctx context.Context, id uuid.UUID, hold bool, setBy string) error {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if c.LegalHold == hold {
		return nil // idempotent
	}

	if err := s.repo.SetLegalHold(ctx, id, hold); err != nil {
		return err
	}

	action := "legal_hold_set"
	if !hold {
		action = "legal_hold_released"
	}

	if s.custody != nil {
		_ = s.custody.RecordCaseEvent(ctx, id, action, setBy, map[string]string{
			"previous_value": fmt.Sprintf("%v", c.LegalHold),
		})
	}

	return nil
}

func (s *Service) validateCreateInput(input CreateCaseInput) error {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return &ValidationError{Field: "title", Message: "title is required"}
	}
	if len(title) > MaxTitleLength {
		return &ValidationError{Field: "title", Message: fmt.Sprintf("title must not exceed %d characters", MaxTitleLength)}
	}

	if len(strings.TrimSpace(input.Description)) > MaxDescriptionLength {
		return &ValidationError{Field: "description", Message: fmt.Sprintf("description must not exceed %d characters", MaxDescriptionLength)}
	}

	if len(strings.TrimSpace(input.Jurisdiction)) > MaxJurisdictionLen {
		return &ValidationError{Field: "jurisdiction", Message: fmt.Sprintf("jurisdiction must not exceed %d characters", MaxJurisdictionLen)}
	}

	ref := strings.TrimSpace(input.ReferenceCode)
	if ref == "" {
		return &ValidationError{Field: "reference_code", Message: "reference code is required"}
	}
	if !s.referenceCodeExpr.MatchString(ref) {
		return &ValidationError{Field: "reference_code", Message: "reference code must be 3-100 characters using letters, digits, hyphens, slashes, dots, or underscores (e.g. ICC-01/04-01/06, KSC-BC-2020-06)"}
	}

	return nil
}

func (s *Service) validateUpdateInput(ctx context.Context, id uuid.UUID, input UpdateCaseInput) error {
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return &ValidationError{Field: "title", Message: "title cannot be empty"}
		}
		if len(title) > MaxTitleLength {
			return &ValidationError{Field: "title", Message: fmt.Sprintf("title must not exceed %d characters", MaxTitleLength)}
		}
	}

	if input.Description != nil && len(strings.TrimSpace(*input.Description)) > MaxDescriptionLength {
		return &ValidationError{Field: "description", Message: fmt.Sprintf("description must not exceed %d characters", MaxDescriptionLength)}
	}

	if input.Jurisdiction != nil && len(strings.TrimSpace(*input.Jurisdiction)) > MaxJurisdictionLen {
		return &ValidationError{Field: "jurisdiction", Message: fmt.Sprintf("jurisdiction must not exceed %d characters", MaxJurisdictionLen)}
	}

	if input.Status != nil {
		current, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if !IsValidStatusTransition(current.Status, *input.Status) {
			return &ValidationError{
				Field:   "status",
				Message: fmt.Sprintf("cannot transition from %s to %s", current.Status, *input.Status),
			}
		}
	}

	return nil
}
