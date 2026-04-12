package evidence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/apperrors"
)

// ErrRetentionActive is returned by CheckRetention when the effective
// retention period has not yet expired. Callers should surface this as a
// 409 Conflict to the operator.
//
// It aliases apperrors.ErrRetentionActive so errors.Is matches across
// package boundaries. Use errors.New (via the alias) rather than
// fmt.Errorf so unwrap semantics are consistent with other sentinels.
var ErrRetentionActive = apperrors.ErrRetentionActive

// EffectiveRetention returns the later of the two retention timestamps.
// A nil input means "no limit"; if either side is nil the other wins;
// if both are nil the result is nil (no retention).
func EffectiveRetention(itemUntil, caseUntil *time.Time) *time.Time {
	switch {
	case itemUntil == nil && caseUntil == nil:
		return nil
	case itemUntil == nil:
		return caseUntil
	case caseUntil == nil:
		return itemUntil
	}
	if itemUntil.After(*caseUntil) {
		return itemUntil
	}
	return caseUntil
}

// CheckRetention reports whether destruction is currently permitted for the
// given item. It returns ErrRetentionActive (wrapped with context) when the
// effective retention date is still in the future. nil means OK to proceed.
func CheckRetention(item EvidenceItem, caseRetention *time.Time, now time.Time) error {
	eff := EffectiveRetention(item.RetentionUntil, caseRetention)
	if eff == nil {
		return nil
	}
	if eff.After(now) {
		return fmt.Errorf("%w: retention expires at %s", ErrRetentionActive, eff.Format(time.RFC3339))
	}
	return nil
}

// ExpiringRetentionItem is a lightweight projection used by the daily
// notification job.
type ExpiringRetentionItem struct {
	EvidenceID     uuid.UUID
	CaseID         uuid.UUID
	EvidenceNumber string
	RetentionUntil time.Time
}

// RetentionRepository is the narrow data-access surface needed by
// NotifyExpiringRetention. Implemented by PGRepository below.
type RetentionRepository interface {
	FindExpiringRetention(ctx context.Context, before time.Time) ([]ExpiringRetentionItem, error)
}

// RetentionNotifier delivers retention-expiry notifications to case admins.
// Production should inject a notifications package adapter. Tests use an
// in-memory fake.
type RetentionNotifier interface {
	NotifyRetentionExpiring(ctx context.Context, item ExpiringRetentionItem) error
}

// WithRetentionNotifier sets the retention notifier on the service.
// Returns the service for chaining. Optional — a nil notifier makes
// NotifyExpiringRetention a no-op that still counts items.
func (s *Service) WithRetentionNotifier(n RetentionNotifier) *Service {
	s.retentionNotifier = n
	return s
}

// NotifyExpiringRetention finds every non-destroyed item whose retention
// expires within `within` and fires a notification per item. Returns the
// count of items processed. Intended to be called from a daily cron.
//
// TODO(cron): wire this from cmd/vaultkeeper-api — e.g. schedule a daily
// ticker that calls svc.NotifyExpiringRetention(ctx, 30*24*time.Hour).
func (s *Service) NotifyExpiringRetention(ctx context.Context, within time.Duration) (int, error) {
	rr, ok := s.repo.(RetentionRepository)
	if !ok {
		return 0, fmt.Errorf("repository does not implement RetentionRepository")
	}
	cutoff := time.Now().Add(within)
	items, err := rr.FindExpiringRetention(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("find expiring retention: %w", err)
	}
	count := 0
	for _, item := range items {
		if s.retentionNotifier != nil {
			if err := s.retentionNotifier.NotifyRetentionExpiring(ctx, item); err != nil {
				s.logger.Error("failed to notify retention expiring",
					"evidence_id", item.EvidenceID, "error", err)
				continue
			}
		}
		count++
	}
	return count, nil
}
