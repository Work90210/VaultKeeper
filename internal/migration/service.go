package migration

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// TimestampIssuer is the narrow interface migration needs from an RFC 3161
// client. Matches integrity.TimestampAuthority.IssueTimestamp.
type TimestampIssuer interface {
	IssueTimestamp(ctx context.Context, digest []byte) (token []byte, tsaName string, tsTime time.Time, err error)
}

// Service orchestrates a full migration: parse manifest → pre-check →
// ingest files → compute migration hash → TSA timestamp → persist.
type Service struct {
	parser   *Parser
	ingester *Ingester
	repo     Repository
	tsa      TimestampIssuer
	logger   *slog.Logger
}

// NewService creates a migration service.
func NewService(
	parser *Parser,
	ingester *Ingester,
	repo Repository,
	tsa TimestampIssuer,
	logger *slog.Logger,
) *Service {
	if parser == nil {
		parser = NewParser()
	}
	return &Service{
		parser:   parser,
		ingester: ingester,
		repo:     repo,
		tsa:      tsa,
		logger:   logger,
	}
}

// RunInput is the full input to Run.
type RunInput struct {
	CaseID         uuid.UUID
	SourceSystem   string
	PerformedBy    string
	ManifestSource io.Reader
	ManifestFormat ManifestFormat
	SourceRoot     string
	Options        BatchOptions
	Progress       ProgressFunc
	// ResumeMigrationID, if non-nil, tells Run to continue an existing
	// in-progress migration rather than creating a new row. The manifest
	// and case_id must match the existing record, otherwise Run returns
	// ErrResumeManifestMismatch. Files already recorded in
	// migration_file_progress are skipped. Forces Options.Resume=true.
	ResumeMigrationID *uuid.UUID
}

// ErrResumeManifestMismatch is returned when a resume attempt references a
// migration whose case or manifest hash differs from the supplied input.
// This protects against an operator accidentally resuming the wrong row.
var ErrResumeManifestMismatch = errors.New("migration: resume manifest does not match existing record")

// RunResult is returned from a completed migration.
type RunResult struct {
	Record Record
	Report IngestReport
}

// Run executes the migration end-to-end.
//
// Error handling is deliberately conservative: any failure creates (and
// marks) the migration record so operators can inspect what happened.
func (s *Service) Run(ctx context.Context, in RunInput) (RunResult, error) {
	if in.CaseID == uuid.Nil {
		return RunResult{}, errors.New("migration: case id is required")
	}
	if in.PerformedBy == "" {
		return RunResult{}, errors.New("migration: performed_by is required")
	}
	if in.SourceSystem == "" {
		return RunResult{}, errors.New("migration: source_system is required")
	}

	entries, err := s.parser.Parse(ctx, in.ManifestSource, in.ManifestFormat)
	if err != nil {
		return RunResult{}, fmt.Errorf("parse manifest: %w", err)
	}
	manifestHash := ComputeManifestHash(entries)

	var rec Record
	if in.ResumeMigrationID != nil {
		// Resume path: load the existing row, validate it matches the
		// supplied manifest, and force Resume=true so already-processed
		// files are skipped via the persistent migration_file_progress
		// table.
		existing, err := s.repo.FindByID(ctx, *in.ResumeMigrationID)
		if err != nil {
			return RunResult{}, fmt.Errorf("resume lookup: %w", err)
		}
		if existing.CaseID != in.CaseID || existing.ManifestHash != manifestHash {
			return RunResult{Record: existing}, ErrResumeManifestMismatch
		}
		if existing.Status != StatusInProgress {
			return RunResult{Record: existing}, fmt.Errorf("migration: cannot resume migration in status %q", existing.Status)
		}
		rec = existing
		in.Options.Resume = true
	} else {
		// Fresh migration: create the row up front in "in_progress"
		// state so a crash during ingestion still leaves a trail.
		rec, err = s.repo.Create(ctx, Record{
			CaseID:        in.CaseID,
			SourceSystem:  in.SourceSystem,
			TotalItems:    len(entries),
			ManifestHash:  manifestHash,
			MigrationHash: manifestHash, // placeholder until ingestion computes the real one
			PerformedBy:   in.PerformedBy,
			Status:        StatusInProgress,
			StartedAt:     time.Now().UTC(),
		})
		if err != nil {
			return RunResult{}, err
		}
	}

	report, ingestErr := s.ingester.BatchIngest(ctx, BatchRequest{
		CaseID:     in.CaseID,
		SourceRoot: in.SourceRoot,
		UploadedBy: in.PerformedBy,
		Entries:    entries,
		Options:    in.Options,
	}, rec.ID, in.Progress)
	if ingestErr != nil {
		_ = s.repo.FinalizeFailure(ctx, rec.ID, StatusFailed)
		return RunResult{Record: rec, Report: report}, ingestErr
	}

	if report.Halted {
		_ = s.repo.FinalizeFailure(ctx, rec.ID, StatusHaltedOnMismatch)
		rec.Status = StatusHaltedOnMismatch
		return RunResult{Record: rec, Report: report}, &HashMismatchError{FilePath: report.HaltedFile}
	}

	if in.Options.DryRun {
		// Dry-run: the row exists only so a crash during hashing is
		// traceable. On clean completion we delete it so the
		// evidence_migrations table is not cluttered with phantom
		// in-progress rows. Completed (real) migrations are immutable
		// and never deleted.
		if err := s.repo.Delete(ctx, rec.ID); err != nil && !errors.Is(err, ErrNotFound) {
			s.warn("dry-run cleanup failed", "migration_id", rec.ID, "error", err)
		}
		return RunResult{Record: rec, Report: report}, nil
	}

	// Compute the deterministic migration hash from processed items.
	migrationHash := ComputeMigrationHash(in.SourceSystem, rec.StartedAt, report.Processed)

	// Request an RFC 3161 timestamp for the migration event.
	var (
		token   []byte
		tsaName string
		tsTime  time.Time
	)
	if s.tsa != nil {
		// ComputeMigrationHash is guaranteed to return a 64-char
		// lowercase hex string (SHA-256 via hex.EncodeToString), so
		// DecodeString can never fail here.
		hashBytes, _ := hex.DecodeString(migrationHash)
		var err error
		token, tsaName, tsTime, err = s.tsa.IssueTimestamp(ctx, hashBytes)
		if err != nil {
			s.warn("TSA event timestamp failed", "error", err)
		}
	}

	var tsPtr *time.Time
	if !tsTime.IsZero() {
		tsPtr = &tsTime
	}
	if err := s.repo.FinalizeSuccess(ctx, rec.ID, report.MatchedItems, report.MismatchedItems, migrationHash, token, tsaName, tsPtr); err != nil {
		return RunResult{Record: rec, Report: report}, fmt.Errorf("finalize migration: %w", err)
	}

	rec.Status = StatusCompleted
	rec.MigrationHash = migrationHash
	rec.MatchedItems = report.MatchedItems
	rec.MismatchedItems = report.MismatchedItems
	rec.TSAToken = token
	rec.TSAName = tsaName
	rec.TSATimestamp = tsPtr
	return RunResult{Record: rec, Report: report}, nil
}

// Get fetches a migration record by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Record, error) {
	return s.repo.FindByID(ctx, id)
}

// ListByCase returns all migration records for a case, newest first.
func (s *Service) ListByCase(ctx context.Context, caseID uuid.UUID) ([]Record, error) {
	return s.repo.ListByCase(ctx, caseID)
}

func (s *Service) warn(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Warn(msg, args...)
	}
}

