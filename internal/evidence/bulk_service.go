package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"
)

// BulkService processes uploaded ZIP archives: validate, extract, and
// ingest each file through the standard evidence upload pipeline.
//
// Each file inside the archive is processed independently so a single
// failure (e.g. a corrupt image) does not block the rest of the batch.
// Failures are captured on the job row so operators can inspect them via
// the status endpoint.
type BulkService struct {
	jobs      BulkJobRepository
	evidence  *Service
	logger    *slog.Logger
	maxUpload int64
}

// NewBulkService constructs a BulkService.
func NewBulkService(jobs BulkJobRepository, evidenceSvc *Service, logger *slog.Logger, maxUpload int64) *BulkService {
	return &BulkService{
		jobs:      jobs,
		evidence:  evidenceSvc,
		logger:    logger,
		maxUpload: maxUpload,
	}
}

// BulkSubmitInput is the input to Submit.
type BulkSubmitInput struct {
	CaseID uuid.UUID
	// ArchiveBytes holds the full compressed archive. We require the
	// raw bytes (rather than an io.ReaderAt + size) so the service can
	// compute the archive SHA-256 before extraction and write it to the
	// job row — this gives operators a tamper-evident record of what
	// was actually uploaded.
	ArchiveBytes   []byte
	ArchiveName    string
	UploadedBy     string
	UploadedByName string
	Classification string // applied to every file unless overridden by _metadata.csv
}

// Submit creates a bulk job row, computes the archive-level SHA-256 for
// a tamper-evident record of what was uploaded, extracts and validates
// the archive, then processes every entry **synchronously**. The call
// blocks for the duration of the entire batch; the plan's 10k-entry
// ceiling keeps wall time bounded. A future enhancement can push the
// processing phase onto a worker pool and return the job ID immediately
// — the repository interface and status transitions are already shaped
// for that.
func (s *BulkService) Submit(ctx context.Context, in BulkSubmitInput) (BulkJob, error) {
	if in.CaseID == uuid.Nil {
		return BulkJob{}, &ValidationError{Field: "case_id", Message: "case id is required"}
	}
	if len(in.ArchiveBytes) == 0 {
		return BulkJob{}, &ValidationError{Field: "archive", Message: "archive is required"}
	}

	job, err := s.jobs.Create(ctx, in.CaseID, in.UploadedBy, in.ArchiveName)
	if err != nil {
		return BulkJob{}, err
	}

	// Archive integrity: hash the raw bytes before extraction so we have
	// a forensic record of exactly what was uploaded. Soft-fail on write
	// error — the upload should not abort because a progress column
	// update failed.
	archiveHashBytes := sha256.Sum256(in.ArchiveBytes)
	archiveHash := hex.EncodeToString(archiveHashBytes[:])
	job.ArchiveSHA256 = archiveHash
	if err := s.jobs.SetArchiveHash(ctx, job.ID, archiveHash); err != nil {
		s.logger.Warn("bulk archive hash persist failed", "job_id", job.ID, "error", err)
	}

	// Extract phase.
	bulk, err := ExtractBulkZIP(ctx, bytes.NewReader(in.ArchiveBytes), int64(len(in.ArchiveBytes)), s.maxUpload)
	if err != nil {
		// Extraction failures are terminal — no per-file errors to record.
		_ = s.jobs.Finalize(ctx, job.ID, BulkStatusFailed)
		s.logger.Warn("bulk extraction failed", "job_id", job.ID, "error", err)
		return job, err
	}

	total := len(bulk.Files)
	if err := s.jobs.UpdateProgress(ctx, job.ID, 0, 0, total, BulkStatusProcessing); err != nil {
		s.logger.Error("bulk progress update failed", "job_id", job.ID, "error", err)
	}

	// Processing phase. Sequential ingestion keeps the code simple; the
	// evidence.Service.Upload path is already concurrency-safe so a future
	// goroutine pool is a drop-in change.
	var (
		processed int
		failed    int
	)
	for i, f := range bulk.Files {
		// Context cancellation is handled naturally by the underlying
		// evidence.Service.Upload call below — a cancelled context
		// propagates through its hash/TSA/store steps and surfaces as
		// a per-file failure on this iteration.

		// Metadata override: if _metadata.csv inside the archive has a
		// row keyed by this file's base name, apply its fields; otherwise
		// fall back to the submit-level defaults.
		meta, hasMeta := bulk.Metadata[filepath.Base(f.Name)]
		classification := in.Classification
		if hasMeta && meta.Classification != "" {
			classification = meta.Classification
		}
		if classification == "" {
			classification = ClassificationRestricted
		}

		upload := UploadInput{
			CaseID:         in.CaseID,
			File:           bytes.NewReader(f.Content),
			Filename:       filepath.Base(f.Name),
			SizeBytes:      f.Size,
			Classification: classification,
			UploadedBy:     in.UploadedBy,
			UploadedByName: in.UploadedByName,
		}
		if hasMeta {
			upload.Description = meta.Description
			upload.Tags = meta.Tags
			upload.Source = meta.Source
			upload.SourceDate = meta.SourceDate
		}

		if _, err := s.evidence.Upload(ctx, upload); err != nil {
			failed++
			if errAppend := s.jobs.AppendError(ctx, job.ID, BulkJobError{
				Filename: f.Name,
				Reason:   err.Error(),
			}); errAppend != nil {
				s.logger.Error("bulk error append failed", "job_id", job.ID, "error", errAppend)
			}
			s.logger.Warn("bulk file ingest failed", "job_id", job.ID, "file", f.Name, "error", err)
		} else {
			processed++
		}

		// Update progress after every file so the status endpoint reflects
		// live state. For very large batches this adds N Postgres writes —
		// acceptable at the 10k cap.
		if err := s.jobs.UpdateProgress(ctx, job.ID, processed, failed, total, BulkStatusProcessing); err != nil {
			s.logger.Error("bulk progress update failed", "job_id", job.ID, "index", i, "error", err)
		}
	}

	finalStatus := BulkStatusCompleted
	if failed > 0 {
		finalStatus = BulkStatusCompletedWithErrors
	}
	if err := s.jobs.Finalize(ctx, job.ID, finalStatus); err != nil {
		s.logger.Error("bulk finalize failed", "job_id", job.ID, "error", err)
	}
	// Reflect final values on the returned job.
	job.Total = total
	job.Processed = processed
	job.Failed = failed
	job.Status = finalStatus
	return job, nil
}

// Get fetches a bulk job by id, scoped to a case. The case scope is
// enforced at the repository layer so an IDOR attack cannot leak a job
// across cases by guessing job IDs.
func (s *BulkService) Get(ctx context.Context, caseID, id uuid.UUID) (BulkJob, error) {
	return s.jobs.FindByID(ctx, caseID, id)
}

