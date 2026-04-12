package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a migration or job row does not exist.
var ErrNotFound = errors.New("migration not found")

// Repository persists migration records. Resume state is on a separate
// ResumeStore interface (see ingester.go) so consumers that only need
// migration CRUD don't depend on the resume surface.
type Repository interface {
	Create(ctx context.Context, rec Record) (Record, error)
	FinalizeSuccess(ctx context.Context, id uuid.UUID, matched, mismatched int, hash string, token []byte, tsaName string, tsTime *time.Time) error
	FinalizeFailure(ctx context.Context, id uuid.UUID, status MigrationStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (Record, error)
	ListByCase(ctx context.Context, caseID uuid.UUID) ([]Record, error)
}

// PGRepository is the Postgres implementation of Repository + ResumeStore.
// Resume state is persisted in the migration_file_progress table so a
// restart survives without losing per-file progress.
type PGRepository struct {
	pool dbPool
}

// dbPool is the narrow subset of pgxpool.Pool we actually use. Matching
// the existing evidence package pattern keeps unit testing straightforward.
type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NewRepository creates a Postgres-backed migration repository.
func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// Create inserts a new in-progress migration row.
func (r *PGRepository) Create(ctx context.Context, rec Record) (Record, error) {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if rec.Status == "" {
		rec.Status = StatusInProgress
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = time.Now().UTC()
	}

	const q = `
		INSERT INTO evidence_migrations
		    (id, case_id, source_system, total_items, matched_items, mismatched_items,
		     migration_hash, manifest_hash, performed_by, status, started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING created_at`
	if err := r.pool.QueryRow(ctx, q,
		rec.ID, rec.CaseID, rec.SourceSystem, rec.TotalItems,
		rec.MatchedItems, rec.MismatchedItems,
		rec.MigrationHash, rec.ManifestHash, rec.PerformedBy,
		string(rec.Status), rec.StartedAt,
	).Scan(&rec.CreatedAt); err != nil {
		return Record{}, fmt.Errorf("insert migration: %w", err)
	}
	return rec, nil
}

// FinalizeSuccess updates the row after the TSA timestamp has been obtained.
func (r *PGRepository) FinalizeSuccess(
	ctx context.Context,
	id uuid.UUID,
	matched, mismatched int,
	migrationHash string,
	token []byte,
	tsaName string,
	tsTime *time.Time,
) error {
	const q = `
		UPDATE evidence_migrations
		   SET matched_items=$2, mismatched_items=$3, migration_hash=$4,
		       tsa_token=$5, tsa_name=$6, tsa_timestamp=$7,
		       status='completed', completed_at=now()
		 WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, matched, mismatched, migrationHash, token, tsaName, tsTime)
	if err != nil {
		return fmt.Errorf("finalize migration success: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FinalizeFailure marks a migration as failed or halted-on-mismatch.
func (r *PGRepository) FinalizeFailure(ctx context.Context, id uuid.UUID, status MigrationStatus) error {
	if status != StatusFailed && status != StatusHaltedOnMismatch {
		return fmt.Errorf("invalid failure status: %q", status)
	}
	const q = `UPDATE evidence_migrations SET status=$2, completed_at=now() WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, string(status))
	if err != nil {
		return fmt.Errorf("finalize migration failure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a migration row entirely. Used by dry-run to avoid
// leaving orphaned in_progress rows; must not be used for completed
// migrations — those are immutable audit records.
func (r *PGRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM evidence_migrations WHERE id=$1 AND status='in_progress'`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete migration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByID returns one migration record.
func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (Record, error) {
	const q = `
		SELECT id, case_id, source_system, total_items, matched_items, mismatched_items,
		       migration_hash, manifest_hash, tsa_token, COALESCE(tsa_name, ''), tsa_timestamp,
		       performed_by, status, started_at, completed_at, created_at
		  FROM evidence_migrations
		 WHERE id=$1`
	row := r.pool.QueryRow(ctx, q, id)
	var rec Record
	var status string
	if err := row.Scan(
		&rec.ID, &rec.CaseID, &rec.SourceSystem, &rec.TotalItems,
		&rec.MatchedItems, &rec.MismatchedItems,
		&rec.MigrationHash, &rec.ManifestHash,
		&rec.TSAToken, &rec.TSAName, &rec.TSATimestamp,
		&rec.PerformedBy, &status, &rec.StartedAt, &rec.CompletedAt, &rec.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, fmt.Errorf("find migration: %w", err)
	}
	rec.Status = MigrationStatus(status)
	return rec, nil
}

// ListByCase returns all migrations for a case ordered by started_at desc.
func (r *PGRepository) ListByCase(ctx context.Context, caseID uuid.UUID) ([]Record, error) {
	const q = `
		SELECT id, case_id, source_system, total_items, matched_items, mismatched_items,
		       migration_hash, manifest_hash, tsa_token, COALESCE(tsa_name, ''), tsa_timestamp,
		       performed_by, status, started_at, completed_at, created_at
		  FROM evidence_migrations
		 WHERE case_id=$1
		 ORDER BY started_at DESC`
	rows, err := r.pool.Query(ctx, q, caseID)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var status string
		if err := rows.Scan(
			&rec.ID, &rec.CaseID, &rec.SourceSystem, &rec.TotalItems,
			&rec.MatchedItems, &rec.MismatchedItems,
			&rec.MigrationHash, &rec.ManifestHash,
			&rec.TSAToken, &rec.TSAName, &rec.TSATimestamp,
			&rec.PerformedBy, &status, &rec.StartedAt, &rec.CompletedAt, &rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan migration row: %w", err)
		}
		rec.Status = MigrationStatus(status)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// IsProcessed returns true if the given file has already been recorded
// as processed for this migration. Backed by migration_file_progress so
// a server restart preserves the full per-file resume set.
func (r *PGRepository) IsProcessed(ctx context.Context, migrationID uuid.UUID, filePath string) (bool, error) {
	const q = `SELECT 1 FROM migration_file_progress WHERE migration_id=$1 AND file_path=$2`
	var one int
	err := r.pool.QueryRow(ctx, q, migrationID, filePath).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query resume state: %w", err)
	}
	return true, nil
}

// MarkProcessed records a completed file. Idempotent — a retry after a
// crash just upserts the same row.
func (r *PGRepository) MarkProcessed(ctx context.Context, migrationID uuid.UUID, filePath string) error {
	const q = `
		INSERT INTO migration_file_progress (migration_id, file_path)
		VALUES ($1, $2)
		ON CONFLICT (migration_id, file_path) DO NOTHING`
	if _, err := r.pool.Exec(ctx, q, migrationID, filePath); err != nil {
		return fmt.Errorf("record resume state: %w", err)
	}
	return nil
}

