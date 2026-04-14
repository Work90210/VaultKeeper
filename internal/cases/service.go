package cases

import (
	"context"
	"fmt"
	"html"
	"log/slog"
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
	notifier          MemberNotifier
	logger            *slog.Logger
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

// SetMemberNotifier wires an outbound notifier used by SetLegalHold to alert
// all case members on hold state changes. Best-effort — a nil notifier is a
// valid configuration and simply disables notifications. Safe to call once
// during server bootstrap, before the service handles requests.
//
// TODO(wiring): cmd/server/main.go should construct an adapter around
// notifications.Service and call SetMemberNotifier on the case service after
// the notification service is instantiated. Until that is done, hold-change
// notifications are dropped in production but covered by fakes in tests.
func (s *Service) SetMemberNotifier(n MemberNotifier) {
	s.notifier = n
}

// SetLogger injects an optional structured logger. When nil, best-effort
// notification failures in SetLegalHold are silently swallowed.
func (s *Service) SetLogger(l *slog.Logger) {
	s.logger = l
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (s *Service) CreateCase(ctx context.Context, input CreateCaseInput, createdBy, createdByName string) (Case, error) {
	if err := s.validateCreateInput(input); err != nil {
		return Case{}, err
	}

	orgID, err := uuid.Parse(input.OrganizationID)
	if err != nil {
		return Case{}, &ValidationError{Field: "organization_id", Message: "valid organization ID is required"}
	}

	c := Case{
		OrganizationID: orgID,
		ReferenceCode:  strings.TrimSpace(input.ReferenceCode),
		Title:          html.EscapeString(strings.TrimSpace(input.Title)),
		Description:    html.EscapeString(strings.TrimSpace(input.Description)),
		Jurisdiction:   html.EscapeString(strings.TrimSpace(input.Jurisdiction)),
		Status:         StatusActive,
		CreatedBy:      createdBy,
		CreatedByName:  createdByName,
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
	// Block all updates to archived cases
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Case{}, err
	}
	if current.Status == StatusArchived {
		return Case{}, &ValidationError{Field: "status", Message: "cannot update an archived case"}
	}

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

	if c.Status == StatusArchived {
		return &ValidationError{Field: "status", Message: "cannot change legal hold on an archived case"}
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

	// Best-effort: notify all case members of the hold state change. The
	// notifier implementation is responsible for fanning out to members —
	// failures are logged but do not fail the toggle.
	if s.notifier != nil {
		if nerr := s.notifier.NotifyLegalHoldChanged(ctx, id, hold, setBy); nerr != nil {
			if s.logger != nil {
				s.logger.Warn("notify legal hold change",
					"case_id", id, "new_state", hold, "error", nerr)
			}
		}
	}

	return nil
}

func (s *Service) Handover(ctx context.Context, caseID uuid.UUID, input HandoverInput, actorUserID string, orgChecker OrgMemberChecker, roleStore CaseRoleStore) error {
	if input.FromUserID == "" {
		return &ValidationError{Field: "from_user_id", Message: "from_user_id is required"}
	}
	if input.ToUserID == "" {
		return &ValidationError{Field: "to_user_id", Message: "to_user_id is required"}
	}
	if input.FromUserID == input.ToUserID {
		return &ValidationError{Field: "to_user_id", Message: "from and to user must differ"}
	}
	if len(input.NewRoles) == 0 {
		return &ValidationError{Field: "new_roles", Message: "at least one role is required"}
	}
	for _, role := range input.NewRoles {
		if !ValidCaseRoles[role] {
			return &ValidationError{Field: "new_roles", Message: fmt.Sprintf("invalid role: %s", role)}
		}
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return &ValidationError{Field: "reason", Message: "reason is required"}
	}

	c, err := s.repo.FindByID(ctx, caseID)
	if err != nil {
		return err
	}
	if c.Status == StatusArchived {
		return &ValidationError{Field: "status", Message: "cannot handover an archived case"}
	}

	// Verify both users are members of the case's org.
	fromMember, err := orgChecker.IsActiveMember(ctx, c.OrganizationID, input.FromUserID)
	if err != nil {
		return fmt.Errorf("check from_user org membership: %w", err)
	}
	if !fromMember {
		return &ValidationError{Field: "from_user_id", Message: "user is not a member of the case organization"}
	}

	toMember, err := orgChecker.IsActiveMember(ctx, c.OrganizationID, input.ToUserID)
	if err != nil {
		return fmt.Errorf("check to_user org membership: %w", err)
	}
	if !toMember {
		return &ValidationError{Field: "to_user_id", Message: "user is not a member of the case organization"}
	}

	// Remove from_user's case roles (unless preserving).
	if !input.PreserveExistingRoles {
		if err := roleStore.Revoke(ctx, caseID, input.FromUserID); err != nil {
			if err != ErrNotFound {
				return fmt.Errorf("revoke from_user roles: %w", err)
			}
			// No existing role to revoke — that's fine.
		}
	}

	// Assign new roles to to_user.
	for _, role := range input.NewRoles {
		if _, err := roleStore.Assign(ctx, caseID, input.ToUserID, role, actorUserID); err != nil {
			if strings.Contains(err.Error(), "role already assigned") {
				continue // idempotent
			}
			return fmt.Errorf("assign to_user role %s: %w", role, err)
		}
	}

	// Record custody log entry.
	if s.custody != nil {
		_ = s.custody.RecordCaseEvent(ctx, caseID, "case_handover", actorUserID, map[string]string{
			"from_user_id":           input.FromUserID,
			"to_user_id":             input.ToUserID,
			"new_roles":              strings.Join(input.NewRoles, ","),
			"preserve_existing_roles": fmt.Sprintf("%v", input.PreserveExistingRoles),
			"reason":                 reason,
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
