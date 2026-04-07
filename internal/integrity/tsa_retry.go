package integrity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// PendingTSAItem represents an evidence item awaiting TSA stamping.
type PendingTSAItem struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	SHA256Hash string
	RetryCount int
	CreatedAt  time.Time
}

// CustodyRecorder logs custody chain events for evidence.
type CustodyRecorder interface {
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// PendingTSAFinder retrieves evidence items needing TSA timestamps.
type PendingTSAFinder interface {
	FindPendingTSA(ctx context.Context, limit int) ([]PendingTSAItem, error)
	UpdateTSAResult(ctx context.Context, id uuid.UUID, token []byte, tsaName string, tsTime time.Time) error
	IncrementTSARetry(ctx context.Context, id uuid.UUID) error
	MarkTSAFailed(ctx context.Context, id uuid.UUID) error
}

// AdvisoryLocker acquires a Postgres advisory lock.
type AdvisoryLocker interface {
	TryAdvisoryLock(ctx context.Context, lockID int64) (bool, error)
	ReleaseAdvisoryLock(ctx context.Context, lockID int64) error
}

const (
	tsaRetryInterval = 5 * time.Minute
	tsaMaxAge        = 24 * time.Hour
	tsaRetryLockID   = 0x5453415F52455452 // "TSA_RETR" as int64
	tsaRetryBatch    = 50
)

// TSARetryJob runs a background job that retries pending TSA timestamps.
type TSARetryJob struct {
	tsa     TimestampAuthority
	finder  PendingTSAFinder
	locker  AdvisoryLocker
	custody CustodyRecorder
	logger  *slog.Logger
}

// NewTSARetryJob creates a new TSA retry background job.
func NewTSARetryJob(tsa TimestampAuthority, finder PendingTSAFinder, locker AdvisoryLocker, custody CustodyRecorder, logger *slog.Logger) *TSARetryJob {
	return &TSARetryJob{
		tsa:     tsa,
		finder:  finder,
		locker:  locker,
		custody: custody,
		logger:  logger,
	}
}

// Start begins the retry loop. It blocks until ctx is cancelled.
func (j *TSARetryJob) Start(ctx context.Context) {
	j.startWithInterval(ctx, tsaRetryInterval)
}

func (j *TSARetryJob) startWithInterval(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	j.logger.Info("TSA retry job started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			j.logger.Info("TSA retry job stopping")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *TSARetryJob) runOnce(ctx context.Context) {
	acquired, err := j.locker.TryAdvisoryLock(ctx, tsaRetryLockID)
	if err != nil {
		j.logger.Error("failed to acquire TSA retry lock", "error", err)
		return
	}
	if !acquired {
		j.logger.Debug("TSA retry lock held by another instance, skipping")
		return
	}
	defer func() {
		if err := j.locker.ReleaseAdvisoryLock(ctx, tsaRetryLockID); err != nil {
			j.logger.Error("failed to release TSA retry lock", "error", err)
		}
	}()

	items, err := j.finder.FindPendingTSA(ctx, tsaRetryBatch)
	if err != nil {
		j.logger.Error("failed to find pending TSA items", "error", err)
		return
	}

	if len(items) == 0 {
		return
	}

	j.logger.Info("processing pending TSA items", "count", len(items))

	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return
		}

		// Skip items older than 24 hours — mark as failed
		if time.Since(item.CreatedAt) > tsaMaxAge {
			if err := j.finder.MarkTSAFailed(ctx, item.ID); err != nil {
				j.logger.Error("failed to mark TSA as failed", "id", item.ID, "error", err)
			}
			j.recordCustodyEvent(ctx, item.CaseID, item.ID, "tsa_permanently_failed", map[string]string{
				"reason": "exceeded 24 hour retry window",
			})
			continue
		}

		digest, err := hexToBytes(item.SHA256Hash)
		if err != nil {
			j.logger.Error("invalid SHA256 hash for TSA retry", "id", item.ID, "error", err)
			if err := j.finder.MarkTSAFailed(ctx, item.ID); err != nil {
				j.logger.Error("failed to mark TSA as failed", "id", item.ID, "error", err)
			}
			continue
		}

		token, tsaName, tsTime, err := j.tsa.IssueTimestamp(ctx, digest)
		if err != nil {
			j.logger.Warn("TSA retry failed", "id", item.ID, "attempt", item.RetryCount+1, "error", err)
			if err := j.finder.IncrementTSARetry(ctx, item.ID); err != nil {
				j.logger.Error("failed to increment TSA retry count", "id", item.ID, "error", err)
			}
			continue
		}

		if err := j.finder.UpdateTSAResult(ctx, item.ID, token, tsaName, tsTime); err != nil {
			j.logger.Error("failed to update TSA result", "id", item.ID, "error", err)
			continue
		}

		j.recordCustodyEvent(ctx, item.CaseID, item.ID, "tsa_retry_succeeded", map[string]string{
			"tsa_name": tsaName,
			"attempt":  fmt.Sprintf("%d", item.RetryCount+1),
		})
		j.logger.Info("TSA retry succeeded", "id", item.ID)
	}
}

func hexToBytes(hex string) ([]byte, error) {
	if len(hex)%2 != 0 {
		return nil, errInvalidHex
	}
	b := make([]byte, len(hex)/2)
	for i := 0; i < len(hex); i += 2 {
		hi, ok1 := hexVal(hex[i])
		lo, ok2 := hexVal(hex[i+1])
		if !ok1 || !ok2 {
			return nil, errInvalidHex
		}
		b[i/2] = hi<<4 | lo
	}
	return b, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

type hexError struct{}

func (hexError) Error() string { return "invalid hex string" }

var errInvalidHex = hexError{}

func (j *TSARetryJob) recordCustodyEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action string, detail map[string]string) {
	if j.custody == nil {
		return
	}
	if err := j.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, "system", detail); err != nil {
		j.logger.Error("failed to record custody event",
			"case_id", caseID, "evidence_id", evidenceID, "action", action, "error", err)
	}
}
