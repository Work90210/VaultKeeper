package app

import (
	"context"
	"log/slog"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// LoggingRetentionNotifier is a slog-only stub implementation of
// evidence.RetentionNotifier. It records each expiring item as a warning
// log entry. Production can replace this with an adapter that fans the
// events out via the notifications service; for now it satisfies the
// wiring contract and gives operators a visible trail in the logs.
type LoggingRetentionNotifier struct {
	Logger *slog.Logger
}

// NotifyRetentionExpiring implements evidence.RetentionNotifier. The
// evidence package invokes this per-item from the daily scheduler (see
// Service.NotifyExpiringRetention) and from DestroyEvidence on success.
func (n *LoggingRetentionNotifier) NotifyRetentionExpiring(ctx context.Context, item evidence.ExpiringRetentionItem) error {
	if n == nil || n.Logger == nil {
		return nil
	}
	n.Logger.Warn("retention expiring",
		"evidence_id", item.EvidenceID,
		"case_id", item.CaseID,
		"evidence_number", item.EvidenceNumber,
		"retention_until", item.RetentionUntil,
	)
	return nil
}
