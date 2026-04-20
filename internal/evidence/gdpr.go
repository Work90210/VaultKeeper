package evidence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Erasure request status values.
const (
	ErasureStatusReady            = "ready"
	ErasureStatusConflictPending  = "conflict_pending"
	ErasureStatusResolvedPreserve = "resolved_preserve"
	ErasureStatusResolvedErase    = "resolved_erase"
)

// Erasure decision values.
const (
	ErasureDecisionPreserve = "preserve"
	ErasureDecisionErase    = "erase"
)

// ErasureRequest is the GDPR "right to be forgotten" workflow record.
type ErasureRequest struct {
	ID          uuid.UUID  `json:"id"`
	EvidenceID  uuid.UUID  `json:"evidence_id"`
	RequestedBy string     `json:"requested_by"`
	Rationale   string     `json:"rationale"`
	Status      string     `json:"status"`
	Decision    *string    `json:"decision,omitempty"`
	DecidedBy   *string    `json:"decided_by,omitempty"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ConflictReport summarises blockers found when creating an erasure request.
// An empty report (HasConflict == false) means the request can proceed
// immediately; otherwise the request is stored in conflict_pending status
// until an operator resolves it.
type ConflictReport struct {
	HasConflict    bool       `json:"has_conflict"`
	LegalHold      bool       `json:"legal_hold"`
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
	CaseActive     bool       `json:"case_active"`
	Details        []string   `json:"details"`
}

// ErasureRepository is the persistence surface for GDPR erasure requests.
// In production this is satisfied by PGRepository; tests use an in-memory fake.
type ErasureRepository interface {
	CreateErasureRequest(ctx context.Context, req ErasureRequest) (ErasureRequest, error)
	FindErasureRequest(ctx context.Context, id uuid.UUID) (ErasureRequest, error)
	UpdateErasureDecision(ctx context.Context, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error)
}

// scopedErasureRepository is an optional extension of ErasureRepository
// satisfied by PGRepository. When the underlying repo implements this
// interface the service uses case-scoped SQL to prevent cross-case IDOR.
// In-memory test fakes do not implement it, so the service falls back
// gracefully to the unscoped ErasureRepository methods.
type scopedErasureRepository interface {
	// FindCaseIDForErasureRequest returns the case_id for the evidence item
	// referenced by the given erasure request. Used to bootstrap case-scoped
	// operations when only the erasure request ID is known.
	FindCaseIDForErasureRequest(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// FindErasureRequestScoped loads an erasure request by ID, restricted to
	// the given case. Returns ErrNotFound for cross-case lookups.
	FindErasureRequestScoped(ctx context.Context, caseID, id uuid.UUID) (ErasureRequest, error)
	// UpdateErasureDecisionScoped transitions an erasure request restricted
	// to the given case. Returns ErrNotFound for cross-case updates.
	UpdateErasureDecisionScoped(ctx context.Context, caseID, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error)
}

// WithErasureRepo injects the erasure-request persistence. Returns the
// service for chaining.
func (s *Service) WithErasureRepo(repo ErasureRepository) *Service {
	s.erasureRepo = repo
	return s
}

// FindErasureRequest loads an erasure request by ID.
//
// When the underlying repository satisfies scopedErasureRepository the
// lookup is case-scoped via a SQL JOIN on evidence_items so a caller
// cannot read an erasure request that belongs to a different case (IDOR
// prevention). In-memory test fakes fall back to the unscoped path.
func (s *Service) FindErasureRequest(ctx context.Context, id uuid.UUID) (ErasureRequest, error) {
	if s.erasureRepo == nil {
		return ErasureRequest{}, fmt.Errorf("erasure repository not configured")
	}
	if scoped, ok := s.erasureRepo.(scopedErasureRepository); ok {
		caseID, err := scoped.FindCaseIDForErasureRequest(ctx, id)
		if err != nil {
			return ErasureRequest{}, fmt.Errorf("resolve case for erasure request: %w", err)
		}
		return scoped.FindErasureRequestScoped(ctx, caseID, id)
	}
	return s.erasureRepo.FindErasureRequest(ctx, id)
}

// CreateErasureRequest opens a GDPR erasure workflow for an evidence item.
// It builds a ConflictReport by checking legal hold, retention, and case
// status. If there are no conflicts the request is persisted with status
// "ready"; otherwise status is "conflict_pending" and the report is
// returned alongside so the caller can surface the blockers.
func (s *Service) CreateErasureRequest(ctx context.Context, evidenceID uuid.UUID, requestedBy, rationale string) (ErasureRequest, ConflictReport, error) {
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return ErasureRequest{}, ConflictReport{}, &ValidationError{Field: "rationale", Message: "rationale is required"}
	}
	if len(rationale) > 2000 {
		return ErasureRequest{}, ConflictReport{}, &ValidationError{Field: "rationale", Message: "rationale exceeds maximum length of 2000 characters"}
	}
	if strings.TrimSpace(requestedBy) == "" {
		return ErasureRequest{}, ConflictReport{}, &ValidationError{Field: "requested_by", Message: "requested_by is required"}
	}
	if s.erasureRepo == nil {
		return ErasureRequest{}, ConflictReport{}, fmt.Errorf("erasure repository not configured")
	}

	// Use FindByIDIncludingDestroyed so erasure requests can be created even
	// for items that are already destroyed (idempotency). The GDPR handler
	// enforces its own authorization gate before calling this method.
	var (
		item EvidenceItem
		err  error
	)
	if sr := s.getScoped(); sr != nil {
		item, err = sr.FindByIDIncludingDestroyed(ctx, evidenceID)
	} else {
		item, err = s.repo.FindByID(ctx, evidenceID)
	}
	if err != nil {
		return ErasureRequest{}, ConflictReport{}, err
	}

	report := s.buildConflictReport(ctx, item)
	status := ErasureStatusReady
	if report.HasConflict {
		status = ErasureStatusConflictPending
	}

	req := ErasureRequest{
		ID:          uuid.New(),
		EvidenceID:  evidenceID,
		RequestedBy: requestedBy,
		Rationale:   rationale,
		Status:      status,
		CreatedAt:   time.Now(),
	}
	created, err := s.erasureRepo.CreateErasureRequest(ctx, req)
	if err != nil {
		return ErasureRequest{}, ConflictReport{}, fmt.Errorf("persist erasure request: %w", err)
	}

	s.recordCustodyEvent(ctx, item.CaseID, item.ID, "gdpr_erasure_requested", requestedBy, map[string]string{
		"request_id": created.ID.String(),
		"status":     created.Status,
		"rationale":  rationale,
	})

	return created, report, nil
}

// ResolveErasureConflict closes a pending erasure request with either
// "preserve" or "erase". A rationale is required for audit. On "erase"
// this calls DestroyEvidence with authority = "GDPR erasure — <rationale>".
func (s *Service) ResolveErasureConflict(ctx context.Context, requestID uuid.UUID, decision, decidedBy, rationale string) error {
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return &ValidationError{Field: "rationale", Message: "rationale is required"}
	}
	if len(rationale) > 2000 {
		return &ValidationError{Field: "rationale", Message: "rationale exceeds maximum length of 2000 characters"}
	}
	if decision != ErasureDecisionPreserve && decision != ErasureDecisionErase {
		return &ValidationError{Field: "decision", Message: "decision must be preserve or erase"}
	}
	if strings.TrimSpace(decidedBy) == "" {
		return &ValidationError{Field: "decided_by", Message: "decided_by is required"}
	}
	if s.erasureRepo == nil {
		return fmt.Errorf("erasure repository not configured")
	}

	// When the repository supports scoped queries (production PGRepository),
	// resolve the owning case first so both the find and the update are
	// case-scoped, preventing cross-case IDOR. In-memory test fakes fall
	// back to the unscoped ErasureRepository methods.
	var (
		req    ErasureRequest
		caseID uuid.UUID
		err    error
	)
	if scoped, ok := s.erasureRepo.(scopedErasureRepository); ok {
		caseID, err = scoped.FindCaseIDForErasureRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("find erasure request: %w", err)
		}
		req, err = scoped.FindErasureRequestScoped(ctx, caseID, requestID)
		if err != nil {
			return fmt.Errorf("find erasure request: %w", err)
		}
	} else {
		req, err = s.erasureRepo.FindErasureRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("find erasure request: %w", err)
		}
	}
	if req.Status == ErasureStatusResolvedErase || req.Status == ErasureStatusResolvedPreserve {
		return &ValidationError{Field: "status", Message: "erasure request is already resolved"}
	}
	if req.Status != ErasureStatusConflictPending {
		return &ValidationError{Field: "status", Message: "only conflict_pending erasure requests can be resolved via this endpoint"}
	}

	// Load the evidence up front so we can emit the custody event regardless
	// of which branch we take. Use FindByIDIncludingDestroyed because the
	// item may already be destroyed (idempotency on re-resolve attempts).
	var item EvidenceItem
	if sr := s.getScoped(); sr != nil {
		item, err = sr.FindByIDIncludingDestroyed(ctx, req.EvidenceID)
	} else {
		item, err = s.repo.FindByID(ctx, req.EvidenceID)
	}
	if err != nil {
		return fmt.Errorf("load evidence for erasure resolution: %w", err)
	}

	// ERASE BRANCH
	//
	// Ordering fix (Sprint 9 review HIGH): previously the code updated the
	// erasure decision BEFORE calling DestroyEvidence. If destruction
	// failed — e.g. a legal hold was re-applied between the conflict
	// report and the resolution — the request was persisted as
	// resolved_erase but the evidence was still intact, with no recovery
	// path. Now we destroy first, then persist the decision only on
	// success. If destruction fails the request stays in
	// conflict_pending and the operator can retry.
	//
	// Legal authority override: the admin has explicitly decided "erase"
	// after seeing the ConflictReport, which means the override is
	// intentional. DestroyEvidence re-checks legal hold and retention,
	// which would race with the conflict report. Use the internal
	// destroyEvidenceOverride helper that skips those guards and records
	// a gdpr_override=true marker in the custody event so the audit
	// trail is explicit about the override.
	if decision == ErasureDecisionErase {
		authority := "GDPR erasure — " + rationale
		if err := s.destroyEvidenceOverride(ctx, DestroyEvidenceInput{
			EvidenceID: req.EvidenceID,
			ActorID:    decidedBy,
			Authority:  authority,
		}); err != nil {
			return fmt.Errorf("destroy after erasure decision: %w", err)
		}
	}

	// Persist the decision AFTER destruction succeeds (erase) or
	// unconditionally (preserve).
	newStatus := ErasureStatusResolvedPreserve
	if decision == ErasureDecisionErase {
		newStatus = ErasureStatusResolvedErase
	}
	var updated ErasureRequest
	if scoped, ok := s.erasureRepo.(scopedErasureRepository); ok {
		updated, err = scoped.UpdateErasureDecisionScoped(ctx, caseID, requestID, newStatus, decision, decidedBy, time.Now())
	} else {
		updated, err = s.erasureRepo.UpdateErasureDecision(ctx, requestID, newStatus, decision, decidedBy, time.Now())
	}
	if err != nil {
		// At this point the evidence may already be destroyed (erase path).
		// Return an error so the operator can investigate — the custody
		// chain is the durable record of what happened.
		return fmt.Errorf("record erasure decision: %w", err)
	}

	s.recordCustodyEvent(ctx, item.CaseID, item.ID, "gdpr_conflict_resolved", decidedBy, map[string]string{
		"request_id": updated.ID.String(),
		"decision":   decision,
		"rationale":  rationale,
	})

	return nil
}

// destroyEvidenceOverride is the internal destruction path used by GDPR
// conflict resolution when the admin has explicitly chosen "erase" after
// reviewing a conflict report. Unlike DestroyEvidence, it skips the
// legal-hold and retention guards — the override is the whole point of
// resolving a conflict — but it still enforces the authority requirement,
// the DB-first ordering, and records the custody event with an explicit
// override marker.
//
// This MUST NOT be reachable from any HTTP handler. Only ResolveErasureConflict
// should call it.
func (s *Service) destroyEvidenceOverride(ctx context.Context, input DestroyEvidenceInput) error {
	if err := validateDestroyEvidenceInput(input); err != nil {
		return err
	}

	// Use FindByIDIncludingDestroyed for idempotency: the item may already
	// be destroyed from a prior run. The caller (ResolveErasureConflict)
	// has already authorized this override path.
	var item EvidenceItem
	if sr := s.getScoped(); sr != nil {
		var fetchErr error
		item, fetchErr = sr.FindByIDIncludingDestroyed(ctx, input.EvidenceID)
		if fetchErr != nil {
			return fmt.Errorf("find evidence for override destruction: %w", fetchErr)
		}
	} else {
		var fetchErr error
		item, fetchErr = s.repo.FindByID(ctx, input.EvidenceID)
		if fetchErr != nil {
			return fmt.Errorf("find evidence for override destruction: %w", fetchErr)
		}
	}
	if item.DestroyedAt != nil {
		return nil // idempotent
	}

	storageKey := derefStr(item.StorageKey)
	thumbnailKey := derefStr(item.ThumbnailKey)

	dr, ok := s.repo.(DestroyerRepository)
	if !ok {
		return fmt.Errorf("repository does not implement DestroyerRepository")
	}
	if err := dr.DestroyWithAuthority(ctx, item.ID, input.Authority, input.ActorID); err != nil {
		return fmt.Errorf("mark destroyed (override): %w", err)
	}

	s.recordCustodyEvent(ctx, item.CaseID, item.ID, "destroyed", input.ActorID, map[string]string{
		"authority":           input.Authority,
		"hash_at_destruction": item.SHA256Hash,
		"filename":            item.Filename,
		"gdpr_override":       "true",
	})

	if storageKey != "" {
		if err := s.storage.DeleteObject(ctx, storageKey); err != nil {
			s.logger.Error("orphaned storage key after GDPR override destruction",
				"evidence_id", item.ID, "storage_key", storageKey, "error", err)
		}
	}
	if thumbnailKey != "" {
		if err := s.storage.DeleteObject(ctx, thumbnailKey); err != nil {
			s.logger.Warn("failed to delete thumbnail during GDPR override destruction",
				"evidence_id", item.ID, "error", err)
		}
	}

	return nil
}

// buildConflictReport inspects the item + case for the three blockers we
// care about: legal hold, active retention, and non-archived case.
func (s *Service) buildConflictReport(ctx context.Context, item EvidenceItem) ConflictReport {
	report := ConflictReport{}

	// Legal hold
	if s.legalHoldChecker != nil {
		if err := s.legalHoldChecker.EnsureNotOnHold(ctx, item.CaseID); err != nil {
			if errors.Is(err, ErrLegalHoldActive) {
				report.LegalHold = true
				report.Details = append(report.Details, "case is under legal hold")
			}
		}
	} else if s.cases != nil {
		if held, err := s.cases.GetLegalHold(ctx, item.CaseID); err == nil && held {
			report.LegalHold = true
			report.Details = append(report.Details, "case is under legal hold")
		}
	}

	// Retention
	var caseRetention *time.Time
	if crr, ok := s.repo.(CaseRetentionReader); ok {
		caseRetention, _ = crr.GetCaseRetention(ctx, item.CaseID)
	}
	if err := CheckRetention(item, caseRetention, time.Now()); err != nil {
		eff := EffectiveRetention(item.RetentionUntil, caseRetention)
		report.RetentionUntil = eff
		report.Details = append(report.Details, fmt.Sprintf("retention period active until %s", eff.Format(time.RFC3339)))
	}

	// Case status — only "archived" cases are conflict-free for erasure.
	if s.cases != nil {
		if status, err := s.cases.GetStatus(ctx, item.CaseID); err == nil && status != "archived" {
			report.CaseActive = true
			report.Details = append(report.Details, fmt.Sprintf("case is %s (not archived)", status))
		}
	}

	report.HasConflict = report.LegalHold || report.RetentionUntil != nil || report.CaseActive
	return report
}
