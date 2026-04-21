package investigation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CustodyRecorder logs custody chain events.
type CustodyRecorder interface {
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// CaptureMetadataUpdater updates capture metadata verification status.
type CaptureMetadataUpdater interface {
	UpdateVerificationStatus(ctx context.Context, evidenceID uuid.UUID, status string) error
}

// EvidenceOwnerChecker resolves who uploaded an evidence item (CRIT-04: prevent self-verification).
type EvidenceOwnerChecker interface {
	GetUploadedBy(ctx context.Context, evidenceID uuid.UUID) (string, error)
}

// CaseMembershipChecker verifies a user has a role on a case (HIGH-01: cross-case isolation).
type CaseMembershipChecker interface {
	HasRoleOnCase(ctx context.Context, caseID uuid.UUID, userID string) (bool, error)
}

// CaseRoleResolver resolves a user's case-level role (e.g. prosecutor, judge) for authorization.
type CaseRoleResolver interface {
	GetCaseRole(ctx context.Context, caseID uuid.UUID, userID string) (string, error)
}

// InvestigationNotifier emits notification events from the investigation subsystem.
// Implementations should adapt this to the notification service's NotificationEvent struct.
type InvestigationNotifier interface {
	NotifyInvestigation(ctx context.Context, eventType string, caseID uuid.UUID, title, body string)
}

// Service orchestrates the investigation subsystem.
type Service struct {
	repo              Repository
	custody           CustodyRecorder
	logger            *slog.Logger
	captureMetadata   CaptureMetadataUpdater
	evidenceOwner     EvidenceOwnerChecker     // optional — for self-verification prevention
	caseMembership    CaseMembershipChecker    // optional — for cross-case isolation
	caseRoleResolver  CaseRoleResolver         // optional — for case-level role authorization
	notifier          InvestigationNotifier    // optional — for notification events
}

// NewService creates a new investigation service.
func NewService(repo Repository, custody CustodyRecorder, logger *slog.Logger) *Service {
	return &Service{repo: repo, custody: custody, logger: logger}
}

// WithCaptureMetadataUpdater injects the capture metadata updater for auto-verification.
func (s *Service) WithCaptureMetadataUpdater(updater CaptureMetadataUpdater) *Service {
	s.captureMetadata = updater
	return s
}

// WithEvidenceOwnerChecker injects the evidence owner lookup for self-verification prevention.
func (s *Service) WithEvidenceOwnerChecker(checker EvidenceOwnerChecker) *Service {
	s.evidenceOwner = checker
	return s
}

// WithCaseMembershipChecker injects the case membership verifier for cross-case isolation.
func (s *Service) WithCaseMembershipChecker(checker CaseMembershipChecker) *Service {
	s.caseMembership = checker
	return s
}

// WithCaseRoleResolver injects the case role resolver for authorization checks.
func (s *Service) WithCaseRoleResolver(resolver CaseRoleResolver) *Service {
	s.caseRoleResolver = resolver
	return s
}

// WithNotifier injects the notification emitter.
func (s *Service) WithNotifier(notifier InvestigationNotifier) *Service {
	s.notifier = notifier
	return s
}

// GetCaseRole returns the caller's case-level role string (e.g. "prosecutor",
// "judge", "investigator"). Returns an empty string and a non-nil error when no
// resolver is configured or the user has no role on the case.
func (s *Service) GetCaseRole(ctx context.Context, caseID uuid.UUID, userID string) (string, error) {
	if s.caseRoleResolver == nil {
		return "", fmt.Errorf("case role resolver not configured")
	}
	return s.caseRoleResolver.GetCaseRole(ctx, caseID, userID)
}

func (s *Service) notify(ctx context.Context, eventType string, caseID uuid.UUID, title, body string) {
	if s.notifier == nil {
		return
	}
	s.notifier.NotifyInvestigation(ctx, eventType, caseID, title, body)
}

// isPrivilegedOnCase checks if the actor has a prosecutor, judge, or system_admin role.
// Falls back to system role check if no CaseRoleResolver is configured.
func (s *Service) isPrivilegedOnCase(ctx context.Context, caseID uuid.UUID, actorID, actorSystemRole string) bool {
	// System admin always has privilege
	if actorSystemRole == "system_admin" {
		return true
	}
	// Try case-level role resolution
	if s.caseRoleResolver != nil {
		caseRole, err := s.caseRoleResolver.GetCaseRole(ctx, caseID, actorID)
		if err == nil {
			return caseRole == "prosecutor" || caseRole == "judge"
		}
	}
	// Fallback: check system role (legacy behavior)
	return actorSystemRole == "prosecutor" || actorSystemRole == "judge"
}

// checkCaseMembership verifies the actor has access to the given case. Returns nil if
// the checker is not configured (graceful degradation during initial wiring).
// System admin bypass is handled in the CaseMembershipChecker adapter.
func (s *Service) checkCaseMembership(ctx context.Context, caseID uuid.UUID, actorID string) error {
	if s.caseMembership == nil {
		return nil
	}
	ok, err := s.caseMembership.HasRoleOnCase(ctx, caseID, actorID)
	if err != nil {
		return fmt.Errorf("check case membership: %w", err)
	}
	if !ok {
		return ErrNotFound // 404 not 403 to prevent enumeration
	}
	return nil
}

func (s *Service) recordCaseEvent(ctx context.Context, caseID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordCaseEvent(ctx, caseID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event", "action", action, "error", err)
	}
}

func (s *Service) recordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event", "action", action, "error", err)
	}
}

// --- Inquiry Logs (Phase 1) ---

func (s *Service) CreateInquiryLog(ctx context.Context, caseID uuid.UUID, input InquiryLogInput, actorID string) (InquiryLog, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return InquiryLog{}, err
	}

	if err := ValidateInquiryLogInput(input); err != nil {
		return InquiryLog{}, err
	}

	startedAt, err := time.Parse(time.RFC3339, input.SearchStartedAt)
	if err != nil {
		return InquiryLog{}, &ValidationError{Field: "search_started_at", Message: "invalid RFC3339 timestamp"}
	}
	var endedAt *time.Time
	if input.SearchEndedAt != nil && *input.SearchEndedAt != "" {
		t, err := time.Parse(time.RFC3339, *input.SearchEndedAt)
		if err != nil {
			return InquiryLog{}, &ValidationError{Field: "search_ended_at", Message: "invalid RFC3339 timestamp"}
		}
		endedAt = &t
	}
	var evidenceID *uuid.UUID
	if input.EvidenceID != nil && *input.EvidenceID != "" {
		parsed, err := uuid.Parse(*input.EvidenceID)
		if err != nil {
			return InquiryLog{}, &ValidationError{Field: "evidence_id", Message: "invalid UUID"}
		}
		evidenceID = &parsed
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return InquiryLog{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	var assignedTo *uuid.UUID
	if input.AssignedTo != nil && *input.AssignedTo != "" {
		parsed, parseErr := uuid.Parse(*input.AssignedTo)
		if parseErr != nil {
			return InquiryLog{}, &ValidationError{Field: "assigned_to", Message: "invalid UUID"}
		}
		assignedTo = &parsed
	}
	priority := "normal"
	if input.Priority != nil && *input.Priority != "" {
		priority = *input.Priority
	}

	log := InquiryLog{
		CaseID:            caseID,
		EvidenceID:        evidenceID,
		SearchStrategy:    input.SearchStrategy,
		SearchKeywords:    input.SearchKeywords,
		SearchOperators:   derefStr(input.SearchOperators),
		SearchTool:        input.SearchTool,
		SearchToolVersion: input.SearchToolVersion,
		SearchURL:         input.SearchURL,
		SearchStartedAt:   startedAt,
		SearchEndedAt:     endedAt,
		ResultsCount:      input.ResultsCount,
		ResultsRelevant:   input.ResultsRelevant,
		ResultsCollected:  input.ResultsCollected,
		Objective:         input.Objective,
		Notes:             input.Notes,
		AssignedTo:        assignedTo,
		Priority:          priority,
		SealedStatus:      "active",
		PerformedBy:       actorUUID,
	}

	created, err := s.repo.CreateInquiryLog(ctx, log)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("create inquiry log: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "inquiry_log_created", actorID, map[string]string{
		"inquiry_log_id": created.ID.String(),
		"search_tool":    input.SearchTool,
		"objective":      truncate(input.Objective, 100),
	})
	return created, nil
}

func (s *Service) ListInquiryLogs(ctx context.Context, caseID uuid.UUID, limit, offset int, actorID string) ([]InquiryLog, int, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListInquiryLogs(ctx, caseID, limit, offset)
}

func (s *Service) UpdateInquiryLog(ctx context.Context, id uuid.UUID, input InquiryLogInput, actorID string) (InquiryLog, error) {
	if err := ValidateInquiryLogInput(input); err != nil {
		return InquiryLog{}, err
	}

	existing, err := s.repo.GetInquiryLog(ctx, id)
	if err != nil {
		return InquiryLog{}, err
	}
	if err := s.checkCaseMembership(ctx, existing.CaseID, actorID); err != nil {
		return InquiryLog{}, err
	}

	startedAt, err := time.Parse(time.RFC3339, input.SearchStartedAt)
	if err != nil {
		return InquiryLog{}, &ValidationError{Field: "search_started_at", Message: "invalid RFC3339 timestamp"}
	}
	var endedAt *time.Time
	if input.SearchEndedAt != nil && *input.SearchEndedAt != "" {
		t, err := time.Parse(time.RFC3339, *input.SearchEndedAt)
		if err != nil {
			return InquiryLog{}, &ValidationError{Field: "search_ended_at", Message: "invalid RFC3339 timestamp"}
		}
		endedAt = &t
	}

	existing.SearchStrategy = input.SearchStrategy
	existing.SearchKeywords = input.SearchKeywords
	existing.SearchOperators = derefStr(input.SearchOperators)
	existing.SearchTool = input.SearchTool
	existing.SearchToolVersion = input.SearchToolVersion
	existing.SearchURL = input.SearchURL
	existing.SearchStartedAt = startedAt
	existing.SearchEndedAt = endedAt
	existing.ResultsCount = input.ResultsCount
	existing.ResultsRelevant = input.ResultsRelevant
	existing.ResultsCollected = input.ResultsCollected
	existing.Objective = input.Objective
	existing.Notes = input.Notes
	if input.AssignedTo != nil && *input.AssignedTo != "" {
		parsed, parseErr := uuid.Parse(*input.AssignedTo)
		if parseErr != nil {
			return InquiryLog{}, &ValidationError{Field: "assigned_to", Message: "invalid UUID"}
		}
		existing.AssignedTo = &parsed
	} else if input.AssignedTo != nil {
		existing.AssignedTo = nil
	}
	if input.Priority != nil && *input.Priority != "" {
		existing.Priority = *input.Priority
	}

	updated, err := s.repo.UpdateInquiryLog(ctx, id, existing.CaseID, existing)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("update inquiry log: %w", err)
	}

	s.recordCaseEvent(ctx, existing.CaseID, "inquiry_log_updated", actorID, map[string]string{
		"inquiry_log_id": id.String(),
	})
	return updated, nil
}

func (s *Service) GetInquiryLog(ctx context.Context, id uuid.UUID, actorID string) (InquiryLog, error) {
	log, err := s.repo.GetInquiryLog(ctx, id)
	if err != nil {
		return InquiryLog{}, err
	}
	if err := s.checkCaseMembership(ctx, log.CaseID, actorID); err != nil {
		return InquiryLog{}, err
	}
	return log, nil
}

// LockInquiryLog sets sealed_status = "locked" on the given inquiry log.
// A lock indicates the log is under review and should not be edited.
// The reason is appended to the existing notes field.
func (s *Service) LockInquiryLog(ctx context.Context, id uuid.UUID, reason, actorID string) (InquiryLog, error) {
	log, err := s.repo.GetInquiryLog(ctx, id)
	if err != nil {
		return InquiryLog{}, err
	}
	if err := s.checkCaseMembership(ctx, log.CaseID, actorID); err != nil {
		return InquiryLog{}, err
	}
	if log.SealedStatus == "complete" {
		return InquiryLog{}, &ValidationError{Field: "sealed_status", Message: "cannot lock a completed inquiry log"}
	}

	var noteAppend *string
	if strings.TrimSpace(reason) != "" {
		s := "[LOCKED] " + strings.TrimSpace(reason)
		noteAppend = &s
	}

	updated, err := s.repo.SetInquiryLogSealedStatus(ctx, id, "locked", nil, noteAppend)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("lock inquiry log: %w", err)
	}

	s.recordCaseEvent(ctx, log.CaseID, "inquiry_log_locked", actorID, map[string]string{
		"inquiry_log_id": id.String(),
	})
	return updated, nil
}

// SealInquiryLog sets sealed_status = "complete" and records sealed_at = now() on the given
// inquiry log. A sealed log is considered final; the note is appended to the notes field.
func (s *Service) SealInquiryLog(ctx context.Context, id uuid.UUID, note, actorID string) (InquiryLog, error) {
	log, err := s.repo.GetInquiryLog(ctx, id)
	if err != nil {
		return InquiryLog{}, err
	}
	if err := s.checkCaseMembership(ctx, log.CaseID, actorID); err != nil {
		return InquiryLog{}, err
	}
	if log.SealedStatus == "complete" {
		return InquiryLog{}, &ValidationError{Field: "sealed_status", Message: "inquiry log is already sealed"}
	}

	now := time.Now().UTC()
	var noteAppend *string
	if strings.TrimSpace(note) != "" {
		s := "[SEALED] " + strings.TrimSpace(note)
		noteAppend = &s
	}

	updated, err := s.repo.SetInquiryLogSealedStatus(ctx, id, "complete", &now, noteAppend)
	if err != nil {
		return InquiryLog{}, fmt.Errorf("seal inquiry log: %w", err)
	}

	s.recordCaseEvent(ctx, log.CaseID, "inquiry_log_sealed", actorID, map[string]string{
		"inquiry_log_id": id.String(),
	})
	return updated, nil
}

// --- Assessments (Phase 2) ---

func (s *Service) CreateAssessment(ctx context.Context, evidenceID, caseID uuid.UUID, input AssessmentInput, actorID string) (EvidenceAssessment, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return EvidenceAssessment{}, err
	}
	if err := ValidateAssessmentInput(input); err != nil {
		return EvidenceAssessment{}, err
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return EvidenceAssessment{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	assessment := EvidenceAssessment{
		EvidenceID:           evidenceID,
		CaseID:               caseID,
		RelevanceScore:       input.RelevanceScore,
		RelevanceRationale:   input.RelevanceRationale,
		ReliabilityScore:     input.ReliabilityScore,
		ReliabilityRationale: input.ReliabilityRationale,
		SourceCredibility:    input.SourceCredibility,
		MisleadingIndicators: input.MisleadingIndicators,
		Recommendation:       input.Recommendation,
		Methodology:          input.Methodology,
		AssessedBy:           actorUUID,
	}
	if assessment.MisleadingIndicators == nil {
		assessment.MisleadingIndicators = []string{}
	}

	created, err := s.repo.CreateAssessment(ctx, assessment)
	if err != nil {
		return EvidenceAssessment{}, fmt.Errorf("create assessment: %w", err)
	}

	s.recordEvidenceEvent(ctx, caseID, evidenceID, "assessment_created", actorID, map[string]string{
		"assessment_id":  created.ID.String(),
		"recommendation": input.Recommendation,
		"relevance":      fmt.Sprintf("%d", input.RelevanceScore),
		"reliability":    fmt.Sprintf("%d", input.ReliabilityScore),
	})
	return created, nil
}

func (s *Service) GetAssessmentsByEvidence(ctx context.Context, caseID, evidenceID uuid.UUID, actorID string) ([]EvidenceAssessment, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.GetAssessmentsByEvidence(ctx, evidenceID)
}

func (s *Service) GetAssessment(ctx context.Context, id uuid.UUID, actorID string) (EvidenceAssessment, error) {
	a, err := s.repo.GetAssessment(ctx, id)
	if err != nil {
		return EvidenceAssessment{}, err
	}
	if err := s.checkCaseMembership(ctx, a.CaseID, actorID); err != nil {
		return EvidenceAssessment{}, err
	}
	return a, nil
}

// --- Verification Records (Phase 5) ---

func (s *Service) CreateVerificationRecord(ctx context.Context, evidenceID, caseID uuid.UUID, input VerificationRecordInput, actorID, actorRole string) (VerificationRecord, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return VerificationRecord{}, err
	}

	if err := ValidateVerificationRecordInput(input); err != nil {
		return VerificationRecord{}, err
	}

	// Only prosecutor/judge can create verification records
	allowedRoles := map[string]bool{"prosecutor": true, "judge": true}
	if !allowedRoles[actorRole] {
		return VerificationRecord{}, &ValidationError{
			Field:   "role",
			Message: "only prosecutor or judge can create verification records",
		}
	}

	// CRIT-04: prevent self-verification — the verifier cannot be the person who uploaded the evidence.
	// Fail closed: if the checker is not configured, refuse the request rather than silently allowing it.
	if s.evidenceOwner == nil {
		return VerificationRecord{}, fmt.Errorf("evidence owner checker not configured")
	}
	uploadedBy, err := s.evidenceOwner.GetUploadedBy(ctx, evidenceID)
	if err != nil {
		return VerificationRecord{}, fmt.Errorf("check evidence ownership: %w", err)
	}
	if uploadedBy == actorID {
		return VerificationRecord{}, &ValidationError{
			Field:   "actor_id",
			Message: "cannot verify your own evidence",
		}
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return VerificationRecord{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	record := VerificationRecord{
		EvidenceID:       evidenceID,
		CaseID:           caseID,
		VerificationType: input.VerificationType,
		Methodology:      input.Methodology,
		ToolsUsed:        input.ToolsUsed,
		SourcesConsulted: input.SourcesConsulted,
		Finding:          input.Finding,
		FindingRationale: input.FindingRationale,
		ConfidenceLevel:  input.ConfidenceLevel,
		Limitations:      input.Limitations,
		Caveats:          input.Caveats,
		VerifiedBy:       actorUUID,
	}
	if record.ToolsUsed == nil {
		record.ToolsUsed = []string{}
	}
	if record.SourcesConsulted == nil {
		record.SourcesConsulted = []string{}
	}
	if record.Caveats == nil {
		record.Caveats = []string{}
	}

	created, err := s.repo.CreateVerificationRecord(ctx, record)
	if err != nil {
		return VerificationRecord{}, fmt.Errorf("create verification record: %w", err)
	}

	s.recordEvidenceEvent(ctx, caseID, evidenceID, "verification_record_created", actorID, map[string]string{
		"verification_id":   created.ID.String(),
		"verification_type": input.VerificationType,
		"finding":           input.Finding,
		"confidence":        input.ConfidenceLevel,
	})

	// Auto-verify: if finding=authentic + confidence=high, upgrade capture metadata
	if input.ShouldAutoVerify() && s.captureMetadata != nil {
		if err := s.captureMetadata.UpdateVerificationStatus(ctx, evidenceID, "verified"); err != nil {
			s.logger.Warn("auto-verification upgrade failed", "evidence_id", evidenceID, "error", err)
		} else {
			s.recordEvidenceEvent(ctx, caseID, evidenceID, "capture_metadata_verification_changed", actorID, map[string]string{
				"new_status":    "verified",
				"triggered_by":  "verification_record",
				"finding":       input.Finding,
				"confidence":    input.ConfidenceLevel,
			})
		}
	}

	return created, nil
}

func (s *Service) ListVerificationRecords(ctx context.Context, evidenceID uuid.UUID) ([]VerificationRecord, error) {
	return s.repo.ListVerificationRecords(ctx, evidenceID)
}

func (s *Service) GetVerificationRecord(ctx context.Context, id uuid.UUID, actorID string) (VerificationRecord, error) {
	rec, err := s.repo.GetVerificationRecord(ctx, id)
	if err != nil {
		return VerificationRecord{}, err
	}
	if err := s.checkCaseMembership(ctx, rec.CaseID, actorID); err != nil {
		return VerificationRecord{}, err
	}
	return rec, nil
}

// --- Corroboration (Phase 5) ---

func (s *Service) CreateCorroborationClaim(ctx context.Context, caseID uuid.UUID, input CorroborationClaimInput, actorID string) (CorroborationClaim, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return CorroborationClaim{}, err
	}

	if err := ValidateCorroborationClaimInput(input); err != nil {
		return CorroborationClaim{}, err
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return CorroborationClaim{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	var evidence []CorroborationEvidence
	for _, e := range input.Evidence {
		eid, parseErr := uuid.Parse(e.EvidenceID)
		if parseErr != nil {
			return CorroborationClaim{}, &ValidationError{Field: "evidence_id", Message: fmt.Sprintf("invalid evidence ID: %s", e.EvidenceID)}
		}
		evidence = append(evidence, CorroborationEvidence{
			EvidenceID:        eid,
			RoleInClaim:       e.RoleInClaim,
			ContributionNotes: e.ContributionNotes,
			AddedBy:           actorUUID,
		})
	}

	claim := CorroborationClaim{
		CaseID:        caseID,
		ClaimSummary:  input.ClaimSummary,
		ClaimType:     input.ClaimType,
		Strength:      input.Strength,
		AnalysisNotes: input.AnalysisNotes,
		Evidence:      evidence,
		CreatedBy:     actorUUID,
	}

	created, err := s.repo.CreateCorroborationClaim(ctx, claim)
	if err != nil {
		return CorroborationClaim{}, fmt.Errorf("create corroboration claim: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "corroboration_claim_created", actorID, map[string]string{
		"claim_id":       created.ID.String(),
		"claim_type":     input.ClaimType,
		"evidence_count": fmt.Sprintf("%d", len(input.Evidence)),
	})
	return created, nil
}

func (s *Service) ListCorroborationClaims(ctx context.Context, caseID uuid.UUID, actorID string) ([]CorroborationClaim, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListCorroborationClaims(ctx, caseID)
}

func (s *Service) GetCorroborationClaim(ctx context.Context, id uuid.UUID, actorID string) (CorroborationClaim, error) {
	claim, err := s.repo.GetCorroborationClaim(ctx, id)
	if err != nil {
		return CorroborationClaim{}, err
	}
	if err := s.checkCaseMembership(ctx, claim.CaseID, actorID); err != nil {
		return CorroborationClaim{}, err
	}
	return claim, nil
}

func (s *Service) GetClaimsByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]CorroborationClaim, error) {
	return s.repo.GetClaimsByEvidence(ctx, evidenceID)
}

// --- Analysis Notes (Phase 6) ---

func (s *Service) CreateAnalysisNote(ctx context.Context, caseID uuid.UUID, input AnalysisNoteInput, actorID string) (AnalysisNote, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return AnalysisNote{}, err
	}

	if err := ValidateAnalysisNoteInput(input); err != nil {
		return AnalysisNote{}, err
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return AnalysisNote{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	note := AnalysisNote{
		CaseID:       caseID,
		Title:        input.Title,
		AnalysisType: input.AnalysisType,
		Content:      input.Content,
		Methodology:  input.Methodology,
		AuthorID:     actorUUID,
	}
	note.RelatedEvidenceIDs = parseUUIDs(input.RelatedEvidenceIDs)
	note.RelatedInquiryIDs = parseUUIDs(input.RelatedInquiryIDs)
	note.RelatedAssessmentIDs = parseUUIDs(input.RelatedAssessmentIDs)
	note.RelatedVerificationIDs = parseUUIDs(input.RelatedVerificationIDs)

	created, err := s.repo.CreateAnalysisNote(ctx, note)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("create analysis note: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "analysis_note_created", actorID, map[string]string{
		"note_id":       created.ID.String(),
		"analysis_type": input.AnalysisType,
		"title":         truncate(input.Title, 100),
	})
	return created, nil
}

func (s *Service) ListAnalysisNotes(ctx context.Context, caseID uuid.UUID, limit, offset int, actorID string) ([]AnalysisNote, int, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListAnalysisNotes(ctx, caseID, limit, offset)
}

func (s *Service) UpdateAnalysisNote(ctx context.Context, id uuid.UUID, input AnalysisNoteInput, actorID string) (AnalysisNote, error) {
	if err := ValidateAnalysisNoteInput(input); err != nil {
		return AnalysisNote{}, err
	}
	existing, err := s.repo.GetAnalysisNote(ctx, id)
	if err != nil {
		return AnalysisNote{}, err
	}
	if err := s.checkCaseMembership(ctx, existing.CaseID, actorID); err != nil {
		return AnalysisNote{}, err
	}
	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return AnalysisNote{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}
	if existing.AuthorID != actorUUID {
		return AnalysisNote{}, &ValidationError{Field: "author_id", Message: "only the note author can edit this note"}
	}

	existing.Title = input.Title
	existing.AnalysisType = input.AnalysisType
	existing.Content = input.Content
	existing.Methodology = input.Methodology
	existing.RelatedEvidenceIDs = parseUUIDs(input.RelatedEvidenceIDs)
	existing.RelatedInquiryIDs = parseUUIDs(input.RelatedInquiryIDs)
	existing.RelatedAssessmentIDs = parseUUIDs(input.RelatedAssessmentIDs)
	existing.RelatedVerificationIDs = parseUUIDs(input.RelatedVerificationIDs)

	updated, err := s.repo.UpdateAnalysisNote(ctx, id, existing)
	if err != nil {
		return AnalysisNote{}, fmt.Errorf("update analysis note: %w", err)
	}

	s.recordCaseEvent(ctx, existing.CaseID, "analysis_note_updated", actorID, map[string]string{
		"note_id": id.String(),
		"title":   truncate(input.Title, 100),
	})
	return updated, nil
}

func (s *Service) GetAnalysisNote(ctx context.Context, id uuid.UUID, actorID string) (AnalysisNote, error) {
	note, err := s.repo.GetAnalysisNote(ctx, id)
	if err != nil {
		return AnalysisNote{}, err
	}
	if err := s.checkCaseMembership(ctx, note.CaseID, actorID); err != nil {
		return AnalysisNote{}, err
	}
	return note, nil
}

// --- Templates (Annexes 1-3) ---

func (s *Service) ListTemplates(ctx context.Context, templateType string) ([]InvestigationTemplate, error) {
	return s.repo.ListTemplates(ctx, templateType)
}

func (s *Service) GetTemplate(ctx context.Context, id uuid.UUID) (InvestigationTemplate, error) {
	return s.repo.GetTemplate(ctx, id)
}

func (s *Service) CreateTemplateInstance(ctx context.Context, caseID uuid.UUID, input TemplateInstanceInput, actorID string) (TemplateInstance, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return TemplateInstance{}, err
	}

	if err := ValidateTemplateInstanceInput(input); err != nil {
		return TemplateInstance{}, err
	}

	templateID, err := uuid.Parse(input.TemplateID)
	if err != nil {
		return TemplateInstance{}, &ValidationError{Field: "template_id", Message: "invalid template ID"}
	}
	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return TemplateInstance{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	instance := TemplateInstance{
		TemplateID: templateID,
		CaseID:     caseID,
		Content:    input.Content,
		PreparedBy: actorUUID,
	}

	created, err := s.repo.CreateTemplateInstance(ctx, instance)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("create template instance: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "template_instance_created", actorID, map[string]string{
		"instance_id": created.ID.String(),
		"template_id": input.TemplateID,
	})
	return created, nil
}

func (s *Service) ListTemplateInstances(ctx context.Context, caseID uuid.UUID, actorID string) ([]TemplateInstance, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListTemplateInstances(ctx, caseID)
}

func (s *Service) GetTemplateInstance(ctx context.Context, id uuid.UUID, actorID string) (TemplateInstance, error) {
	inst, err := s.repo.GetTemplateInstance(ctx, id)
	if err != nil {
		return TemplateInstance{}, err
	}
	if err := s.checkCaseMembership(ctx, inst.CaseID, actorID); err != nil {
		return TemplateInstance{}, err
	}
	return inst, nil
}

func (s *Service) UpdateTemplateInstance(ctx context.Context, id uuid.UUID, content map[string]any, status, actorID string) (TemplateInstance, error) {
	existing, err := s.repo.GetTemplateInstance(ctx, id)
	if err != nil {
		return TemplateInstance{}, err
	}
	if err := s.checkCaseMembership(ctx, existing.CaseID, actorID); err != nil {
		return TemplateInstance{}, err
	}

	updated, err := s.repo.UpdateTemplateInstance(ctx, id, existing.CaseID, content, status)
	if err != nil {
		return TemplateInstance{}, fmt.Errorf("update template instance: %w", err)
	}

	s.recordCaseEvent(ctx, existing.CaseID, "template_instance_updated", actorID, map[string]string{
		"instance_id": id.String(),
		"status":      status,
	})
	return updated, nil
}

// --- Reports (R1, R3) ---

func (s *Service) CreateReport(ctx context.Context, caseID uuid.UUID, input ReportInput, actorID string) (InvestigationReport, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return InvestigationReport{}, err
	}

	if err := ValidateReportInput(input); err != nil {
		return InvestigationReport{}, err
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return InvestigationReport{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}

	report := InvestigationReport{
		CaseID:      caseID,
		Title:       input.Title,
		ReportType:  input.ReportType,
		Sections:    input.Sections,
		Limitations: input.Limitations,
		Caveats:     input.Caveats,
		Assumptions: input.Assumptions,
		AuthorID:    actorUUID,
	}
	report.ReferencedEvidenceIDs = parseUUIDs(input.ReferencedEvidenceIDs)
	report.ReferencedAnalysisIDs = parseUUIDs(input.ReferencedAnalysisIDs)
	if report.Limitations == nil {
		report.Limitations = []string{}
	}
	if report.Caveats == nil {
		report.Caveats = []string{}
	}
	if report.Assumptions == nil {
		report.Assumptions = []string{}
	}

	created, err := s.repo.CreateReport(ctx, report)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("create report: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "report_created", actorID, map[string]string{
		"report_id":   created.ID.String(),
		"report_type": input.ReportType,
		"title":       truncate(input.Title, 100),
	})
	return created, nil
}

func (s *Service) ListReports(ctx context.Context, caseID uuid.UUID, actorID string) ([]InvestigationReport, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListReports(ctx, caseID)
}

func (s *Service) GetReport(ctx context.Context, id uuid.UUID, actorID string) (InvestigationReport, error) {
	report, err := s.repo.GetReport(ctx, id)
	if err != nil {
		return InvestigationReport{}, err
	}
	if err := s.checkCaseMembership(ctx, report.CaseID, actorID); err != nil {
		return InvestigationReport{}, err
	}
	return report, nil
}

func (s *Service) PublishReport(ctx context.Context, id uuid.UUID, actorID, actorRole string) (InvestigationReport, error) {
	report, err := s.repo.GetReport(ctx, id)
	if err != nil {
		return InvestigationReport{}, err
	}

	if !s.isPrivilegedOnCase(ctx, report.CaseID, actorID, actorRole) {
		return InvestigationReport{}, &ValidationError{Field: "role", Message: "only prosecutor or judge can publish reports"}
	}

	if report.Status != ReportStatusApproved {
		return InvestigationReport{}, &ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("report must be approved before publishing; current status: %s", report.Status),
		}
	}

	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return InvestigationReport{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}
	now := time.Now().UTC()
	report.Status = ReportStatusPublished
	report.ApprovedBy = &actorUUID
	report.ApprovedAt = &now

	updated, err := s.repo.UpdateReport(ctx, id, report.CaseID, report)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("publish report: %w", err)
	}

	s.recordCaseEvent(ctx, report.CaseID, "report_published", actorID, map[string]string{
		"report_id": id.String(),
		"title":     truncate(report.Title, 100),
	})
	return updated, nil
}

// --- Bulk Queries by Case ---

func (s *Service) ListAssessmentsByCase(ctx context.Context, caseID uuid.UUID, actorID string) ([]EvidenceAssessment, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListAssessmentsByCase(ctx, caseID)
}

func (s *Service) ListVerificationsByCase(ctx context.Context, caseID uuid.UUID, actorID string) ([]VerificationRecord, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListVerificationsByCase(ctx, caseID)
}

// --- Report Status Transitions ---

func (s *Service) TransitionReportStatus(ctx context.Context, id uuid.UUID, newStatus, actorID, actorRole string) (InvestigationReport, error) {
	report, err := s.repo.GetReport(ctx, id)
	if err != nil {
		return InvestigationReport{}, err
	}
	if err := s.checkCaseMembership(ctx, report.CaseID, actorID); err != nil {
		return InvestigationReport{}, err
	}

	allowed := map[string][]string{
		ReportStatusDraft:     {ReportStatusInReview, ReportStatusWithdrawn},
		ReportStatusInReview:  {ReportStatusApproved, ReportStatusWithdrawn},
		ReportStatusApproved:  {ReportStatusPublished, ReportStatusWithdrawn},
		ReportStatusWithdrawn: {ReportStatusDraft},
	}
	validTargets, ok := allowed[report.Status]
	if !ok {
		return InvestigationReport{}, &ValidationError{Field: "status", Message: fmt.Sprintf("cannot transition from %s", report.Status)}
	}

	isValidTarget := false
	for _, target := range validTargets {
		if target == newStatus {
			isValidTarget = true
			break
		}
	}
	if !isValidTarget {
		return InvestigationReport{}, &ValidationError{Field: "status", Message: fmt.Sprintf("cannot transition from %s to %s", report.Status, newStatus)}
	}

	switch newStatus {
	case ReportStatusApproved:
		if !s.isPrivilegedOnCase(ctx, report.CaseID, actorID, actorRole) {
			return InvestigationReport{}, &ValidationError{Field: "role", Message: "only prosecutor or judge can approve reports"}
		}
	case ReportStatusPublished:
		if !s.isPrivilegedOnCase(ctx, report.CaseID, actorID, actorRole) {
			return InvestigationReport{}, &ValidationError{Field: "role", Message: "only prosecutor or judge can publish reports"}
		}
		actorUUID, parseErr := uuid.Parse(actorID)
		if parseErr != nil {
			return InvestigationReport{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
		}
		now := time.Now().UTC()
		report.ApprovedBy = &actorUUID
		report.ApprovedAt = &now
	}

	oldStatus := report.Status
	report.Status = newStatus
	updated, err := s.repo.UpdateReport(ctx, id, report.CaseID, report)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("transition report status: %w", err)
	}

	s.recordCaseEvent(ctx, report.CaseID, "report_status_transitioned", actorID, map[string]string{
		"report_id":  id.String(),
		"old_status": oldStatus,
		"new_status": newStatus,
	})

	// Emit notifications
	if newStatus == ReportStatusInReview {
		s.notify(ctx, "report_submitted_for_review", report.CaseID, "Report submitted for review", fmt.Sprintf("Report '%s' has been submitted for review", truncate(report.Title, 80)))
	}
	if newStatus == ReportStatusPublished {
		s.notify(ctx, "report_published", report.CaseID, "Report published", fmt.Sprintf("Report '%s' has been published", truncate(report.Title, 80)))
	}

	return updated, nil
}

func (s *Service) UpdateReportContent(ctx context.Context, id uuid.UUID, input ReportInput, actorID string) (InvestigationReport, error) {
	if err := ValidateReportInput(input); err != nil {
		return InvestigationReport{}, err
	}
	report, err := s.repo.GetReport(ctx, id)
	if err != nil {
		return InvestigationReport{}, err
	}
	if err := s.checkCaseMembership(ctx, report.CaseID, actorID); err != nil {
		return InvestigationReport{}, err
	}
	if report.Status != ReportStatusDraft && report.Status != ReportStatusWithdrawn {
		return InvestigationReport{}, &ValidationError{Field: "status", Message: "can only edit reports in draft or withdrawn status"}
	}

	report.Title = input.Title
	report.ReportType = input.ReportType
	report.Sections = input.Sections
	report.Limitations = input.Limitations
	report.Caveats = input.Caveats
	report.Assumptions = input.Assumptions
	report.ReferencedEvidenceIDs = parseUUIDs(input.ReferencedEvidenceIDs)
	report.ReferencedAnalysisIDs = parseUUIDs(input.ReferencedAnalysisIDs)
	if report.Limitations == nil {
		report.Limitations = []string{}
	}
	if report.Caveats == nil {
		report.Caveats = []string{}
	}
	if report.Assumptions == nil {
		report.Assumptions = []string{}
	}

	updated, err := s.repo.UpdateReport(ctx, id, report.CaseID, report)
	if err != nil {
		return InvestigationReport{}, fmt.Errorf("update report content: %w", err)
	}

	s.recordCaseEvent(ctx, report.CaseID, "report_updated", actorID, map[string]string{
		"report_id": id.String(),
		"title":     truncate(input.Title, 100),
	})
	return updated, nil
}

// --- Safety Profiles (P4, S2) ---

func (s *Service) UpsertSafetyProfile(ctx context.Context, caseID, userID uuid.UUID, input SafetyProfileInput, actorID, actorRole string) (SafetyProfile, []SafetyProfileWarning, error) {
	if !CanWriteSafetyProfile(actorRole) {
		return SafetyProfile{}, nil, &ValidationError{Field: "role", Message: "only prosecutor or judge can manage safety profiles"}
	}

	warnings, err := ValidateSafetyProfileInput(input)
	if err != nil {
		return SafetyProfile{}, nil, err
	}

	// HIGH-05: validate target user has a role on this case
	if err := s.checkCaseMembership(ctx, caseID, userID.String()); err != nil {
		return SafetyProfile{}, nil, &ValidationError{
			Field:   "user_id",
			Message: "target user does not have a role on this case",
		}
	}

	usePseudonym := false
	if input.UsePseudonym != nil {
		usePseudonym = *input.UsePseudonym
	}
	requiredVPN := false
	if input.RequiredVPN != nil {
		requiredVPN = *input.RequiredVPN
	}
	requiredTor := false
	if input.RequiredTor != nil {
		requiredTor = *input.RequiredTor
	}
	threatLevel := ThreatLow
	if input.ThreatLevel != nil && *input.ThreatLevel != "" {
		threatLevel = *input.ThreatLevel
	}
	briefingCompleted := false
	if input.SafetyBriefingCompleted != nil {
		briefingCompleted = *input.SafetyBriefingCompleted
	}
	var briefingDate *time.Time
	if input.SafetyBriefingDate != nil && *input.SafetyBriefingDate != "" {
		t, err := time.Parse(time.RFC3339, *input.SafetyBriefingDate)
		if err != nil {
			t, _ = time.Parse("2006-01-02", *input.SafetyBriefingDate)
		}
		if !t.IsZero() {
			briefingDate = &t
		}
	}
	var officerID *uuid.UUID
	if input.SafetyOfficerID != nil && *input.SafetyOfficerID != "" {
		parsed, parseErr := uuid.Parse(*input.SafetyOfficerID)
		if parseErr != nil {
			return SafetyProfile{}, nil, &ValidationError{Field: "safety_officer_id", Message: "invalid safety officer ID"}
		}
		officerID = &parsed
	}

	profile := SafetyProfile{
		CaseID:                  caseID,
		UserID:                  userID,
		Pseudonym:               input.Pseudonym,
		UsePseudonym:            usePseudonym,
		OpsecLevel:              input.OpsecLevel,
		RequiredVPN:             requiredVPN,
		RequiredTor:             requiredTor,
		ApprovedDevices:         input.ApprovedDevices,
		ProhibitedPlatforms:     input.ProhibitedPlatforms,
		ThreatLevel:             threatLevel,
		ThreatNotes:             input.ThreatNotes,
		SafetyBriefingCompleted: briefingCompleted,
		SafetyBriefingDate:      briefingDate,
		SafetyOfficerID:         officerID,
	}
	if profile.ApprovedDevices == nil {
		profile.ApprovedDevices = []string{}
	}
	if profile.ProhibitedPlatforms == nil {
		profile.ProhibitedPlatforms = []string{}
	}

	saved, err := s.repo.UpsertSafetyProfile(ctx, profile)
	if err != nil {
		return SafetyProfile{}, nil, fmt.Errorf("upsert safety profile: %w", err)
	}

	s.recordCaseEvent(ctx, caseID, "safety_profile_updated", actorID, map[string]string{
		"target_user_id": userID.String(),
		"opsec_level":    input.OpsecLevel,
		"threat_level":   threatLevel,
	})
	s.notify(ctx, "safety_profile_updated", caseID, "Safety profile updated", fmt.Sprintf("Safety profile updated for OPSEC level %s", input.OpsecLevel))
	return saved, warnings, nil
}

func (s *Service) GetSafetyProfile(ctx context.Context, caseID, userID uuid.UUID, actorID string) (SafetyProfile, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return SafetyProfile{}, err
	}
	return s.repo.GetSafetyProfile(ctx, caseID, userID)
}

func (s *Service) ListSafetyProfiles(ctx context.Context, caseID uuid.UUID, actorID string) ([]SafetyProfile, error) {
	if err := s.checkCaseMembership(ctx, caseID, actorID); err != nil {
		return nil, err
	}
	return s.repo.ListSafetyProfiles(ctx, caseID)
}

// --- Helpers ---

func parseUUIDs(ids []string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err == nil {
			result = append(result, parsed)
		}
	}
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
