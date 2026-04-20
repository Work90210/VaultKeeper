package witnesses

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// CustodyRecorder logs custody chain events.
type CustodyRecorder interface {
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Service orchestrates witness operations with identity encryption and role-based filtering.
type Service struct {
	repo      Repository
	encryptor *Encryptor
	custody   CustodyRecorder
	logger    *slog.Logger
}

// NewService creates a new witness service.
func NewService(repo Repository, encryptor *Encryptor, custody CustodyRecorder, logger *slog.Logger) *Service {
	return &Service{
		repo:      repo,
		encryptor: encryptor,
		custody:   custody,
		logger:    logger,
	}
}

// GetCaseID returns the case ID for a witness, for authorization checks.
// In production it issues a lightweight SELECT case_id query via the scoped
// repo extension. Test fakes fall back to the full FindByID.
func (s *Service) GetCaseID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	if scoped, ok := s.repo.(scopedWitnessRepo); ok {
		return scoped.FindCaseID(ctx, id)
	}
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	return w.CaseID, nil
}

// Create creates a new witness with encrypted identity fields.
func (s *Service) Create(ctx context.Context, input CreateWitnessInput) (WitnessView, error) {
	if err := s.validateCreateInput(input); err != nil {
		return WitnessView{}, err
	}

	witnessID := uuid.New()

	fullNameEnc, err := s.encryptor.EncryptField(input.FullName, witnessID.String(), "full_name")
	if err != nil {
		// unreachable: EncryptField only errors when the underlying Encryptor.Encrypt
		// fails; with a correctly initialised Encryptor that path is unreachable
		// (see encryption.go unreachable annotations).
		return WitnessView{}, fmt.Errorf("encrypt full name: %w", err)
	}

	contactInfoEnc, err := s.encryptor.EncryptField(input.ContactInfo, witnessID.String(), "contact_info")
	if err != nil {
		// unreachable: same reasoning as full_name above.
		return WitnessView{}, fmt.Errorf("encrypt contact info: %w", err)
	}

	locationEnc, err := s.encryptor.EncryptField(input.Location, witnessID.String(), "location")
	if err != nil {
		// unreachable: same reasoning as full_name above.
		return WitnessView{}, fmt.Errorf("encrypt location: %w", err)
	}

	createdBy, err := uuid.Parse(input.CreatedBy)
	if err != nil {
		return WitnessView{}, &ValidationError{Field: "created_by", Message: "invalid user ID"}
	}

	relEvidence := input.RelatedEvidence
	if relEvidence == nil {
		relEvidence = []uuid.UUID{}
	}

	w := Witness{
		ID:                   witnessID,
		CaseID:               input.CaseID,
		WitnessCode:          input.WitnessCode,
		FullNameEncrypted:    fullNameEnc,
		ContactInfoEncrypted: contactInfoEnc,
		LocationEncrypted:    locationEnc,
		ProtectionStatus:     input.ProtectionStatus,
		StatementSummary:     input.StatementSummary,
		RelatedEvidence:      relEvidence,
		CreatedBy:            createdBy,
	}

	created, err := s.repo.Create(ctx, w)
	if err != nil {
		return WitnessView{}, fmt.Errorf("create witness: %w", err)
	}

	s.recordCustodyEvent(ctx, input.CaseID, "witness_created", input.CreatedBy, map[string]string{
		"witness_code":      input.WitnessCode,
		"protection_status": input.ProtectionStatus,
	})

	return s.toView(created, true), nil
}

// Get retrieves a witness with identity filtered by the caller's role.
// In production, resolves the case_id first then fetches with case scoping
// to prevent cross-case IDOR. Test fakes fall back to the unscoped FindByID.
func (s *Service) Get(ctx context.Context, id uuid.UUID, caseRole auth.CaseRole) (WitnessView, error) {
	var w Witness
	var err error
	if scoped, ok := s.repo.(scopedWitnessRepo); ok {
		caseID, caseErr := scoped.FindCaseID(ctx, id)
		if caseErr != nil {
			return WitnessView{}, fmt.Errorf("resolve case for witness: %w", caseErr)
		}
		w, err = scoped.FindByIDScoped(ctx, caseID, id)
	} else {
		w, err = s.repo.FindByID(ctx, id)
	}
	if err != nil {
		return WitnessView{}, err
	}

	showIdentity := s.canViewIdentity(w, caseRole)

	if showIdentity {
		actorID := "system"
		if ac, ok := auth.GetAuthContext(ctx); ok {
			actorID = ac.UserID
		}
		s.recordCustodyEvent(ctx, w.CaseID, "witness_identity_accessed", actorID, map[string]string{
			"witness_code": w.WitnessCode,
		})
	}

	if showIdentity {
		return s.toDecryptedView(w)
	}
	return s.toView(w, false), nil
}

// List retrieves witnesses for a case with identity filtered by role.
func (s *Service) List(ctx context.Context, caseID uuid.UUID, caseRole auth.CaseRole, page Pagination) ([]WitnessView, int, error) {
	witnesses, total, err := s.repo.FindByCase(ctx, caseID, page)
	if err != nil {
		return nil, 0, err
	}

	views := make([]WitnessView, 0, len(witnesses))
	for _, w := range witnesses {
		showIdentity := s.canViewIdentity(w, caseRole)
		if showIdentity {
			v, err := s.toDecryptedView(w)
			if err != nil {
				s.logger.Error("failed to decrypt witness identity", "witness_id", w.ID, "error", err)
				views = append(views, s.toView(w, false))
				continue
			}
			views = append(views, v)
		} else {
			views = append(views, s.toView(w, false))
		}
	}

	return views, total, nil
}

// Update updates a witness with re-encrypted identity fields.
// The pre-fetch is scoped by case in production to prevent cross-case IDOR.
// Test fakes fall back to the unscoped FindByID.
func (s *Service) Update(ctx context.Context, id uuid.UUID, input UpdateWitnessInput, actorID string) (WitnessView, error) {
	var existing Witness
	var err error
	if scoped, ok := s.repo.(scopedWitnessRepo); ok {
		caseID, caseErr := scoped.FindCaseID(ctx, id)
		if caseErr != nil {
			return WitnessView{}, caseErr
		}
		existing, err = scoped.FindByIDScoped(ctx, caseID, id)
	} else {
		existing, err = s.repo.FindByID(ctx, id)
	}
	if err != nil {
		return WitnessView{}, err
	}

	updated := existing

	if input.FullName != nil {
		enc, err := s.encryptor.EncryptField(input.FullName, id.String(), "full_name")
		if err != nil {
			// unreachable: EncryptField cannot fail with a correctly initialised Encryptor.
			return WitnessView{}, fmt.Errorf("encrypt full name: %w", err)
		}
		updated.FullNameEncrypted = enc
	}

	if input.ContactInfo != nil {
		enc, err := s.encryptor.EncryptField(input.ContactInfo, id.String(), "contact_info")
		if err != nil {
			// unreachable: same reasoning as full_name above.
			return WitnessView{}, fmt.Errorf("encrypt contact info: %w", err)
		}
		updated.ContactInfoEncrypted = enc
	}

	if input.Location != nil {
		enc, err := s.encryptor.EncryptField(input.Location, id.String(), "location")
		if err != nil {
			// unreachable: same reasoning as full_name above.
			return WitnessView{}, fmt.Errorf("encrypt location: %w", err)
		}
		updated.LocationEncrypted = enc
	}

	if input.ProtectionStatus != nil {
		if !ValidProtectionStatuses[*input.ProtectionStatus] {
			return WitnessView{}, &ValidationError{Field: "protection_status", Message: "invalid protection status"}
		}
		updated.ProtectionStatus = *input.ProtectionStatus
	}

	if input.StatementSummary != nil {
		updated.StatementSummary = *input.StatementSummary
	}

	if input.RelatedEvidence != nil {
		updated.RelatedEvidence = input.RelatedEvidence
	}

	if input.JudgeIdentityVisible != nil {
		updated.JudgeIdentityVisible = *input.JudgeIdentityVisible
	}

	result, err := s.repo.Update(ctx, id, updated)
	if err != nil {
		return WitnessView{}, fmt.Errorf("update witness: %w", err)
	}

	changedFields := make(map[string]string)
	if input.ProtectionStatus != nil {
		changedFields["protection_status"] = *input.ProtectionStatus
	}
	if input.FullName != nil {
		changedFields["full_name"] = "updated"
	}
	if input.ContactInfo != nil {
		changedFields["contact_info"] = "updated"
	}
	if input.Location != nil {
		changedFields["location"] = "updated"
	}
	if input.StatementSummary != nil {
		changedFields["statement_summary"] = "updated"
	}

	s.recordCustodyEvent(ctx, result.CaseID, "witness_updated", actorID, changedFields)

	v, err := s.toDecryptedView(result)
	if err != nil {
		return s.toView(result, false), nil
	}
	return v, nil
}

// canViewIdentity determines if a case role can see witness identity.
func (s *Service) canViewIdentity(w Witness, role auth.CaseRole) bool {
	switch role {
	case auth.CaseRoleInvestigator, auth.CaseRoleProsecutor:
		return true
	case auth.CaseRoleJudge:
		return w.JudgeIdentityVisible
	default:
		return false
	}
}

// toView creates a WitnessView without decrypting identity.
func (s *Service) toView(w Witness, identityVisible bool) WitnessView {
	return WitnessView{
		ID:               w.ID,
		CaseID:           w.CaseID,
		WitnessCode:      w.WitnessCode,
		ProtectionStatus: w.ProtectionStatus,
		StatementSummary: w.StatementSummary,
		RelatedEvidence:  w.RelatedEvidence,
		IdentityVisible:  identityVisible,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	}
}

// toDecryptedView decrypts identity fields and returns a full WitnessView.
func (s *Service) toDecryptedView(w Witness) (WitnessView, error) {
	fullName, err := s.encryptor.DecryptField(w.FullNameEncrypted, w.ID.String(), "full_name")
	if err != nil {
		return WitnessView{}, fmt.Errorf("decrypt full name: %w", err)
	}

	contactInfo, err := s.encryptor.DecryptField(w.ContactInfoEncrypted, w.ID.String(), "contact_info")
	if err != nil {
		return WitnessView{}, fmt.Errorf("decrypt contact info: %w", err)
	}

	location, err := s.encryptor.DecryptField(w.LocationEncrypted, w.ID.String(), "location")
	if err != nil {
		return WitnessView{}, fmt.Errorf("decrypt location: %w", err)
	}

	return WitnessView{
		ID:               w.ID,
		CaseID:           w.CaseID,
		WitnessCode:      w.WitnessCode,
		FullName:         fullName,
		ContactInfo:      contactInfo,
		Location:         location,
		ProtectionStatus: w.ProtectionStatus,
		StatementSummary: w.StatementSummary,
		RelatedEvidence:  w.RelatedEvidence,
		IdentityVisible:  true,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	}, nil
}

func (s *Service) validateCreateInput(input CreateWitnessInput) error {
	if input.CaseID == uuid.Nil {
		return &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if strings.TrimSpace(input.WitnessCode) == "" {
		return &ValidationError{Field: "witness_code", Message: "witness code is required"}
	}
	if !ValidProtectionStatuses[input.ProtectionStatus] {
		return &ValidationError{Field: "protection_status", Message: "invalid protection status"}
	}
	return nil
}

func (s *Service) recordCustodyEvent(ctx context.Context, caseID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordCaseEvent(ctx, caseID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event", "case_id", caseID, "action", action, "error", err)
	}
}
