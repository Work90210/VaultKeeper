package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ObjectRemover deletes objects from object storage.
type ObjectRemover interface {
	RemoveObject(ctx context.Context, bucket, key string) error
}

// Notifier delivers out-of-band notifications.
type Notifier interface {
	Notify(ctx context.Context, payload map[string]any) error
}

// Worker processes notification_outbox rows on a configurable interval.
type Worker struct {
	db          dbExecer
	minioClient ObjectRemover
	notifier    Notifier
	logger      *slog.Logger
	interval    time.Duration
}

type outboxRow struct {
	ID           uuid.UUID
	Action       string
	Payload      []byte
	AttemptCount int
	MaxAttempts  int
}

// dbExecer is the subset of pgxpool.Pool needed by the worker.
type dbExecer interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewWorker creates a cleanup worker.
func NewWorker(db *pgxpool.Pool, minioClient ObjectRemover, notifier Notifier, logger *slog.Logger, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		db:          db,
		minioClient: minioClient,
		notifier:    notifier,
		logger:      logger,
		interval:    interval,
	}
}

// Run loops ProcessOnce at the configured interval until ctx is cancelled.
// The first iteration runs immediately on start (drain-on-startup) to clear
// any outbox rows that accumulated while the worker was down.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := w.ProcessOnce(ctx); err != nil {
			w.logger.Error("cleanup worker iteration failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessOnce claims and processes all ready outbox rows.
func (w *Worker) ProcessOnce(ctx context.Context) error {
	for {
		row, ok, err := w.claimNext(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := w.processClaimed(ctx, row); err != nil {
			w.logger.Error("cleanup worker processing failed",
				"outbox_id", row.ID, "action", row.Action, "error", err)
		}
	}
}

func (w *Worker) claimNext(ctx context.Context) (outboxRow, bool, error) {
	tx, err := w.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return outboxRow{}, false, fmt.Errorf("begin outbox claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var row outboxRow
	err = tx.QueryRow(ctx, `
		WITH claimed AS (
			SELECT id, action, payload, attempt_count, max_attempts
			FROM notification_outbox
			WHERE next_attempt_at <= now()
			  AND completed_at IS NULL
			  AND dead_letter_at IS NULL
			ORDER BY next_attempt_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE notification_outbox o
		SET next_attempt_at = now() + interval '5 minutes'
		FROM claimed
		WHERE o.id = claimed.id
		RETURNING claimed.id, claimed.action, claimed.payload, claimed.attempt_count, claimed.max_attempts`,
	).Scan(&row.ID, &row.Action, &row.Payload, &row.AttemptCount, &row.MaxAttempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return outboxRow{}, false, nil
		}
		return outboxRow{}, false, fmt.Errorf("claim outbox row: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return outboxRow{}, false, fmt.Errorf("commit outbox claim tx: %w", err)
	}
	return row, true, nil
}

func (w *Worker) processClaimed(ctx context.Context, row outboxRow) error {
	var payload map[string]any
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return w.markDeadLetter(ctx, row, map[string]any{"reason": "invalid_payload"})
	}

	var processErr error
	switch row.Action {
	case "minio_delete_object":
		processErr = w.processMinioDelete(ctx, payload)
	case "notification_send":
		processErr = w.processNotification(ctx, payload)
	default:
		processErr = fmt.Errorf("unknown outbox action %q", row.Action)
	}

	if processErr == nil {
		_, err := w.db.Exec(ctx,
			`UPDATE notification_outbox SET completed_at = now() WHERE id = $1`,
			row.ID)
		if err != nil {
			return fmt.Errorf("complete outbox row: %w", err)
		}
		return nil
	}

	nextAttempt := row.AttemptCount + 1
	if nextAttempt >= row.MaxAttempts {
		return w.markDeadLetter(ctx, row, payload)
	}

	// Exponential backoff: 30s × 2^attempt, capped at 1 hour.
	// Uses integer seconds to avoid fragile Go duration string → Postgres interval casting.
	const maxBackoffSec = 3600 // 1 hour
	backoffSec := 30 * (1 << min(row.AttemptCount, 17)) // cap shift to prevent overflow
	if backoffSec > maxBackoffSec || backoffSec < 0 {
		backoffSec = maxBackoffSec
	}
	_, err := w.db.Exec(ctx,
		`UPDATE notification_outbox
		 SET attempt_count = attempt_count + 1,
		     next_attempt_at = now() + $2 * interval '1 second'
		 WHERE id = $1`,
		row.ID, backoffSec)
	if err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}
	return nil
}

func (w *Worker) processMinioDelete(ctx context.Context, payload map[string]any) error {
	bucket, _ := payload["bucket"].(string)
	objectKey, _ := payload["object_key"].(string)
	if bucket == "" || objectKey == "" {
		return fmt.Errorf("minio_delete_object missing bucket or object_key")
	}
	return w.minioClient.RemoveObject(ctx, bucket, objectKey)
}

func (w *Worker) processNotification(ctx context.Context, payload map[string]any) error {
	if w.notifier == nil {
		return nil
	}
	return w.notifier.Notify(ctx, payload)
}

func (w *Worker) markDeadLetter(ctx context.Context, row outboxRow, payload map[string]any) error {
	_, err := w.db.Exec(ctx,
		`UPDATE notification_outbox
		 SET attempt_count = attempt_count + 1,
		     dead_letter_at = now()
		 WHERE id = $1`,
		row.ID)
	if err != nil {
		return fmt.Errorf("dead-letter outbox row: %w", err)
	}
	// Fire a CRITICAL notification via synchronous fallback.
	if w.notifier != nil {
		if notifyErr := w.notifier.Notify(ctx, map[string]any{
			"severity":  "critical",
			"kind":      "cleanup_dead_letter",
			"outbox_id": row.ID.String(),
			"action":    row.Action,
			"payload":   payload,
		}); notifyErr != nil {
			w.logger.Error("failed to send dead-letter notification", "outbox_id", row.ID, "error", notifyErr)
		}
	}
	return nil
}
