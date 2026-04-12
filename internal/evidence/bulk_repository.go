package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrBulkJobNotFound is returned when a bulk upload job id does not exist.
var ErrBulkJobNotFound = errors.New("bulk upload job not found")

// Bulk upload status constants. Must match the CHECK constraint in
// migration 019.
const (
	BulkStatusExtracting           = "extracting"
	BulkStatusProcessing           = "processing"
	BulkStatusCompleted            = "completed"
	BulkStatusCompletedWithErrors  = "completed_with_errors"
	BulkStatusFailed               = "failed"
)

// BulkJobRepository persists bulk_upload_jobs rows.
type BulkJobRepository interface {
	Create(ctx context.Context, caseID uuid.UUID, performedBy, archiveKey string) (BulkJob, error)
	SetArchiveHash(ctx context.Context, id uuid.UUID, sha256Hex string) error
	UpdateProgress(ctx context.Context, id uuid.UUID, processed, failed, total int, status string) error
	AppendError(ctx context.Context, id uuid.UUID, entry BulkJobError) error
	Finalize(ctx context.Context, id uuid.UUID, status string) error
	// FindByID enforces case scoping in the query so cross-case
	// enumeration (IDOR) is structurally impossible.
	FindByID(ctx context.Context, caseID, id uuid.UUID) (BulkJob, error)
}

// PGBulkJobRepository is the Postgres implementation.
type PGBulkJobRepository struct {
	pool dbPool
}

// NewBulkJobRepository constructs a PGBulkJobRepository.
func NewBulkJobRepository(pool *pgxpool.Pool) *PGBulkJobRepository {
	return &PGBulkJobRepository{pool: pool}
}

func (r *PGBulkJobRepository) Create(ctx context.Context, caseID uuid.UUID, performedBy, archiveKey string) (BulkJob, error) {
	id := uuid.New()
	const q = `
		INSERT INTO bulk_upload_jobs (id, case_id, archive_key, performed_by, status)
		VALUES ($1, $2, $3, $4, 'extracting')
		RETURNING started_at, updated_at`
	var started, updated time.Time
	if err := r.pool.QueryRow(ctx, q, id, caseID, archiveKey, performedBy).Scan(&started, &updated); err != nil {
		return BulkJob{}, fmt.Errorf("create bulk job: %w", err)
	}
	return BulkJob{
		ID:          id,
		CaseID:      caseID,
		Status:      BulkStatusExtracting,
		PerformedBy: performedBy,
		StartedAt:   started,
		UpdatedAt:   updated,
	}, nil
}

func (r *PGBulkJobRepository) SetArchiveHash(ctx context.Context, id uuid.UUID, sha256Hex string) error {
	const q = `UPDATE bulk_upload_jobs SET archive_sha256=$2, updated_at=now() WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, sha256Hex)
	if err != nil {
		return fmt.Errorf("set bulk archive hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBulkJobNotFound
	}
	return nil
}

func (r *PGBulkJobRepository) UpdateProgress(ctx context.Context, id uuid.UUID, processed, failed, total int, status string) error {
	const q = `
		UPDATE bulk_upload_jobs
		   SET processed_files=$2, failed_files=$3, total_files=$4,
		       status=$5, updated_at=now()
		 WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, processed, failed, total, status)
	if err != nil {
		return fmt.Errorf("update bulk job progress: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBulkJobNotFound
	}
	return nil
}

func (r *PGBulkJobRepository) AppendError(ctx context.Context, id uuid.UUID, entry BulkJobError) error {
	// Append the error as a JSON object into the errors array. Using
	// jsonb_build_object + array concatenation keeps the operation atomic
	// without requiring us to read-modify-write the existing array.
	const q = `
		UPDATE bulk_upload_jobs
		   SET errors = errors || jsonb_build_array(jsonb_build_object('filename', $2::text, 'reason', $3::text)),
		       updated_at = now()
		 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, entry.Filename, entry.Reason)
	if err != nil {
		return fmt.Errorf("append bulk job error: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBulkJobNotFound
	}
	return nil
}

func (r *PGBulkJobRepository) Finalize(ctx context.Context, id uuid.UUID, status string) error {
	const q = `
		UPDATE bulk_upload_jobs
		   SET status=$2, completed_at=now(), updated_at=now()
		 WHERE id=$1`
	tag, err := r.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("finalize bulk job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBulkJobNotFound
	}
	return nil
}

func (r *PGBulkJobRepository) FindByID(ctx context.Context, caseID, id uuid.UUID) (BulkJob, error) {
	const q = `
		SELECT id, case_id, total_files, processed_files, failed_files,
		       status, COALESCE(archive_sha256, ''),
		       errors, performed_by, started_at, updated_at, completed_at
		  FROM bulk_upload_jobs
		 WHERE id=$1 AND case_id=$2`
	var job BulkJob
	var errorsJSON []byte
	err := r.pool.QueryRow(ctx, q, id, caseID).Scan(
		&job.ID, &job.CaseID, &job.Total, &job.Processed, &job.Failed,
		&job.Status, &job.ArchiveSHA256,
		&errorsJSON, &job.PerformedBy,
		&job.StartedAt, &job.UpdatedAt, &job.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BulkJob{}, ErrBulkJobNotFound
		}
		return BulkJob{}, fmt.Errorf("find bulk job: %w", err)
	}
	if len(errorsJSON) > 0 {
		if err := json.Unmarshal(errorsJSON, &job.Errors); err != nil {
			return BulkJob{}, fmt.Errorf("decode bulk job errors: %w", err)
		}
	}
	return job, nil
}
