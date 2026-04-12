package cases

// Legal hold enforcement (Sprint 9 Step 2).
//
// Legal hold is a case-level flag that blocks destructive operations until
// the hold is released. The rule is: metadata edits, uploads, tag updates,
// and disclosures keep working — destructive mutations do not.
//
// EnsureNotOnHold is the canonical guard. Other services (evidence
// destruction, file replacement, case archival, case deletion) call it
// before their mutation and propagate the returned error as a 409.

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/apperrors"
)

// ErrLegalHoldActive is returned by EnsureNotOnHold when the case is under
// legal hold. Callers should surface this as an HTTP 409 Conflict and
// present the message to the operator unmodified.
//
// This is an alias for apperrors.ErrLegalHoldActive so that errors.Is
// matches across package boundaries (evidence, search, disclosures) without
// each package needing a bespoke adapter. Do not redefine this value.
var ErrLegalHoldActive = apperrors.ErrLegalHoldActive

// LegalHoldStatusReader is the narrow read-side interface the guard needs.
// It is defined at the consumer — any repository exposing a LegalHold
// lookup can satisfy it — which lets other packages (evidence, disclosures)
// inject a thin adapter without dragging in the full cases.Repository.
type LegalHoldStatusReader interface {
	// IsLegalHoldActive returns true if the case currently has legal_hold = true.
	// It must read the latest committed row — no caching.
	IsLegalHoldActive(ctx context.Context, caseID uuid.UUID) (bool, error)
}

// EnsureNotOnHold blocks any destructive mutation against a case that is
// currently under legal hold. A nil error means the caller may proceed.
//
// The check is intentionally separate from the higher-level Service so that
// evidence.Service and disclosures.Service can call it without importing
// cases.Service's full surface.
func EnsureNotOnHold(ctx context.Context, reader LegalHoldStatusReader, caseID uuid.UUID) error {
	if reader == nil {
		return fmt.Errorf("legal hold reader not configured")
	}
	active, err := reader.IsLegalHoldActive(ctx, caseID)
	if err != nil {
		return fmt.Errorf("check legal hold: %w", err)
	}
	if active {
		return ErrLegalHoldActive
	}
	return nil
}

// EnsureNotOnHold is the Service-scoped convenience: same contract as the
// package-level helper but uses the Service's own repository, so cases
// package callers don't need to pass a reader explicitly.
//
// Concurrency contract (important):
//
// This method performs a STRICT, single-column read of the legal_hold flag
// via repo.CheckLegalHoldStrict. It is strictly stronger than the prior
// FindByID-then-check implementation because it does not share intermediate
// Case state across service calls and touches only the one column that
// matters. However, on its own it is NOT fully TOCTOU-free: a concurrent
// transaction can still toggle the flag between this read and the caller's
// subsequent destructive mutation.
//
// True atomicity requires the caller to run the destructive mutation inside
// the same transaction as this check AND hold a row-level lock
// (SELECT legal_hold FROM cases WHERE id = $1 FOR SHARE) for the remainder
// of the transaction. Until the destruction flow is refactored to expose a
// transaction handle here, callers MUST execute their destructive mutation
// in as short a window as possible after the check returns nil.
func (s *Service) EnsureNotOnHold(ctx context.Context, caseID uuid.UUID) error {
	hold, err := s.repo.CheckLegalHoldStrict(ctx, caseID)
	if err != nil {
		return fmt.Errorf("ensure not on hold: %w", err)
	}
	if hold {
		return ErrLegalHoldActive
	}
	return nil
}

// MemberNotifier is the narrow outbound interface the cases.Service uses to
// notify case members when the legal hold flag is toggled. Production code
// wires this to notifications.Service (which already fans out to all case
// members internally via its EventLegalHoldChanged recipient resolver), so
// the cases package does not need to enumerate members itself.
//
// Defined here (at the consumer) to keep cases.Service decoupled from the
// notifications package. Keep the method set small.
type MemberNotifier interface {
	// NotifyLegalHoldChanged is called once per toggle. Implementations MUST
	// fan out to all case members. A non-nil error is logged by the caller
	// but does not fail the hold toggle — notifications are best-effort.
	NotifyLegalHoldChanged(ctx context.Context, caseID uuid.UUID, newState bool, actor string) error
}
