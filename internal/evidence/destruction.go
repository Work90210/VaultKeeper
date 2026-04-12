package evidence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/apperrors"
)

// ErrLegalHoldActive is the evidence-package sentinel returned when
// destruction is blocked by an active legal hold.
//
// It is an alias for apperrors.ErrLegalHoldActive so that errors.Is
// matches regardless of which package a caller imports. The legal-hold
// adapter in internal/app no longer needs to translate between distinct
// values — both cases.ErrLegalHoldActive and evidence.ErrLegalHoldActive
// point at the same underlying error.
var ErrLegalHoldActive = apperrors.ErrLegalHoldActive

// MinDestructionAuthorityLength is the minimum character count for a
// destruction authority string. Sprint 9 spec: 10 characters.
const MinDestructionAuthorityLength = 10

// DestroyEvidenceInput is the validated input for audited destruction.
// Authority is a free-text legal citation (court order number, statute
// reference, etc.) required for the destruction record.
//
// ⚠️ Rendering contract: the Authority string is operator-supplied free
// text. Any UI surface that renders it MUST use React text nodes (or the
// equivalent auto-escaping primitive in the target framework). Do NOT
// pass it to `dangerouslySetInnerHTML`, `v-html`, or any raw HTML sink —
// it is an XSS vector otherwise. The DB column also enforces a 2000-char
// cap (migration 018) so the rendering surface cannot be abused with
// multi-megabyte payloads.
type DestroyEvidenceInput struct {
	EvidenceID uuid.UUID
	ActorID    string
	Authority  string
}

// CaseRetentionReader loads the case-level retention floor. Implemented
// by PGRepository via GetCaseRetention.
type CaseRetentionReader interface {
	GetCaseRetention(ctx context.Context, caseID uuid.UUID) (*time.Time, error)
}

// DestroyerRepository is the narrow repository surface DestroyEvidence needs.
type DestroyerRepository interface {
	DestroyWithAuthority(ctx context.Context, id uuid.UUID, authority, actorID string) error
}

// DestroyEvidence performs an audited physical destruction of an evidence
// item, subject to legal-hold and retention checks. On success the file is
// removed from object storage, the DB record is marked destroyed with the
// cited authority, and a custody event "destroyed" is emitted.
func (s *Service) DestroyEvidence(ctx context.Context, input DestroyEvidenceInput) error {
	if err := validateDestroyEvidenceInput(input); err != nil {
		return err
	}

	item, err := s.repo.FindByID(ctx, input.EvidenceID)
	if err != nil {
		return err
	}
	if item.DestroyedAt != nil {
		return nil // idempotent
	}

	// Legal hold guard — must come before any mutation. If no checker is
	// injected we fall back to the CaseLookup.GetLegalHold shortcut so
	// production still gets protection even before the adapter is wired.
	if err := s.checkLegalHold(ctx, item.CaseID); err != nil {
		return err
	}

	// Retention guard.
	var caseRetention *time.Time
	if crr, ok := s.repo.(CaseRetentionReader); ok {
		caseRetention, err = crr.GetCaseRetention(ctx, item.CaseID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("load case retention: %w", err)
		}
	}
	if err := CheckRetention(item, caseRetention, time.Now()); err != nil {
		return err
	}

	// Capture storage keys before the DB update nulls them out. The update
	// is the commit point; storage deletion is a post-commit side effect
	// that can be retried via an orphan-cleanup job if it fails.
	storageKey := derefStr(item.StorageKey)
	thumbnailKey := derefStr(item.ThumbnailKey)

	// DB first: marking the row destroyed is reversible (we still have the
	// storage object) and is the durable audit anchor. If this fails, the
	// file is still intact and the caller can retry.
	dr, ok := s.repo.(DestroyerRepository)
	if !ok {
		return fmt.Errorf("repository does not implement DestroyerRepository")
	}
	if err := dr.DestroyWithAuthority(ctx, item.ID, input.Authority, input.ActorID); err != nil {
		return fmt.Errorf("mark destroyed: %w", err)
	}

	// Custody event — records the hash at destruction so the chain is
	// cryptographically anchored to what existed before erasure.
	s.recordCustodyEvent(ctx, item.CaseID, item.ID, "destroyed", input.ActorID, map[string]string{
		"authority":           input.Authority,
		"hash_at_destruction": item.SHA256Hash,
		"filename":            item.Filename,
	})

	// Storage deletion AFTER the DB commit. Failures here leave orphaned
	// objects but the audit trail is intact; an external cleanup job can
	// reconcile via the destroyed_at + null-storage_key fingerprint.
	if storageKey != "" {
		if err := s.storage.DeleteObject(ctx, storageKey); err != nil {
			s.logger.Error("orphaned storage key after destruction",
				"evidence_id", item.ID, "storage_key", storageKey, "error", err)
		}
	}
	if thumbnailKey != "" {
		if err := s.storage.DeleteObject(ctx, thumbnailKey); err != nil {
			s.logger.Warn("failed to delete thumbnail during destruction",
				"evidence_id", item.ID, "error", err)
		}
	}

	// TODO: dedicated DestructionNotifier. Previously this code fired
	// retentionNotifier.NotifyRetentionExpiring here — that was the wrong
	// interface and left recipients with misleading notifications. A real
	// DestructionNotifier should be added and wired via cmd/server/main.go.

	return nil
}

// checkLegalHold consults the injected LegalHoldChecker if present,
// otherwise falls back to CaseLookup.GetLegalHold. Returns
// cases.ErrLegalHoldActive (or a wrapping ValidationError) when blocked.
func (s *Service) checkLegalHold(ctx context.Context, caseID uuid.UUID) error {
	if s.legalHoldChecker != nil {
		if err := s.legalHoldChecker.EnsureNotOnHold(ctx, caseID); err != nil {
			return err
		}
		return nil
	}
	if s.cases == nil {
		return nil
	}
	held, err := s.cases.GetLegalHold(ctx, caseID)
	if err != nil {
		return fmt.Errorf("check legal hold: %w", err)
	}
	if held {
		return ErrLegalHoldActive
	}
	return nil
}

func validateDestroyEvidenceInput(input DestroyEvidenceInput) error {
	if input.EvidenceID == uuid.Nil {
		return &ValidationError{Field: "evidence_id", Message: "evidence ID is required"}
	}
	if strings.TrimSpace(input.ActorID) == "" {
		return &ValidationError{Field: "actor_id", Message: "actor ID is required"}
	}
	authority := strings.TrimSpace(input.Authority)
	if authority == "" {
		return &ValidationError{Field: "authority", Message: "destruction authority is required"}
	}
	if len(authority) < MinDestructionAuthorityLength {
		return &ValidationError{
			Field:   "authority",
			Message: fmt.Sprintf("destruction authority must be at least %d characters", MinDestructionAuthorityLength),
		}
	}
	return nil
}
