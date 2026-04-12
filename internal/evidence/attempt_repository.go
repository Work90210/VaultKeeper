package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UploadAttempt records an upload attempt before the file is processed.
type UploadAttempt struct {
	CaseID     uuid.UUID
	UserID     uuid.UUID
	ClientHash string
	StartedAt  time.Time
}

// UploadAttemptRepository records upload attempts and events.
type UploadAttemptRepository interface {
	Record(ctx context.Context, attempt UploadAttempt) (uuid.UUID, error)
	RecordEvent(ctx context.Context, attemptID uuid.UUID, eventType string, payload map[string]any) error
	RecordEventTx(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, eventType string, payload map[string]any) error
}

// PgUploadAttemptRepository is the Postgres implementation.
type PgUploadAttemptRepository struct {
	pool dbPool
}

// NewUploadAttemptRepository creates a new repository backed by pgxpool.
func NewUploadAttemptRepository(pool *pgxpool.Pool) *PgUploadAttemptRepository {
	return &PgUploadAttemptRepository{pool: pool}
}

func (r *PgUploadAttemptRepository) Record(ctx context.Context, attempt UploadAttempt) (uuid.UUID, error) {
	if attempt.StartedAt.IsZero() {
		attempt.StartedAt = time.Now().UTC()
	}

	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`INSERT INTO upload_attempts_v1 (case_id, user_id, client_hash, started_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		attempt.CaseID, attempt.UserID, attempt.ClientHash, attempt.StartedAt,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("record upload attempt: %w", err)
	}
	return id, nil
}

func (r *PgUploadAttemptRepository) RecordEvent(ctx context.Context, attemptID uuid.UUID, eventType string, payload map[string]any) error {
	return insertAttemptEvent(ctx, r.pool, attemptID, eventType, payload)
}

func (r *PgUploadAttemptRepository) RecordEventTx(ctx context.Context, tx pgx.Tx, attemptID uuid.UUID, eventType string, payload map[string]any) error {
	return insertAttemptEvent(ctx, tx, attemptID, eventType, payload)
}

// execer is satisfied by both dbPool and pgx.Tx.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertAttemptEvent(ctx context.Context, exec execer, attemptID uuid.UUID, eventType string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal upload attempt event payload: %w", err)
	}
	if _, err := exec.Exec(ctx,
		`INSERT INTO upload_attempt_events (attempt_id, event_type, payload)
		 VALUES ($1, $2, $3::jsonb)`,
		attemptID, eventType, b,
	); err != nil {
		return fmt.Errorf("record upload attempt event: %w", err)
	}
	return nil
}

// InsertOutboxItem writes a notification_outbox row for async compensation.
func InsertOutboxItem(ctx context.Context, exec execer, action string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := exec.Exec(ctx,
		`INSERT INTO notification_outbox (action, payload) VALUES ($1, $2::jsonb)`,
		action, b,
	); err != nil {
		return fmt.Errorf("insert notification outbox: %w", err)
	}
	return nil
}

// noopUploadAttemptRepository is a no-op for backward compatibility.
type noopUploadAttemptRepository struct{}

func (noopUploadAttemptRepository) Record(_ context.Context, _ UploadAttempt) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (noopUploadAttemptRepository) RecordEvent(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
	return nil
}

func (noopUploadAttemptRepository) RecordEventTx(_ context.Context, _ pgx.Tx, _ uuid.UUID, _ string, _ map[string]any) error {
	return nil
}
