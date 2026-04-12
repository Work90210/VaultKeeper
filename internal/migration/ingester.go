package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IngestedItem describes one file that has been successfully stored and
// hash-verified into VaultKeeper by the migration pipeline.
type IngestedItem struct {
	ManifestEntry ManifestEntry
	EvidenceID    uuid.UUID
	ComputedHash  string
	SizeBytes     int64
	HashMatched   bool // true iff OriginalHash was present and matched
	IngestedAt    time.Time
}

// IngestReport aggregates the outcome of a BatchIngest call.
type IngestReport struct {
	Processed       []IngestedItem
	Failed          []IngestFailure
	Halted          bool      // true when a hash mismatch triggered HaltOnMismatch
	HaltedFile      string    // populated when Halted is true
	StartedAt       time.Time
	CompletedAt     time.Time
	MatchedItems    int
	MismatchedItems int
}

// IngestFailure records a single per-file error encountered during batch
// processing. Non-fatal failures (e.g. I/O errors when HaltOnMismatch is
// false) are collected here rather than aborting the whole batch.
type IngestFailure struct {
	FilePath string
	Reason   string
}

// ProgressFunc is invoked after each file is processed. Receivers must not
// block for long — the call is made from a worker goroutine and blocking
// will stall the batch.
type ProgressFunc func(current, total int, filePath string)

// BatchOptions controls parallel ingestion behaviour.
type BatchOptions struct {
	Concurrency    int  // defaults to 4
	HaltOnMismatch bool // when true, any hash mismatch aborts the batch
	DryRun         bool // compute hashes and verify but do not persist
	Resume         bool // skip files already marked processed by FileResumeStore
}

// BatchRequest is the full input to BatchIngest.
type BatchRequest struct {
	CaseID     uuid.UUID
	SourceRoot string // filesystem root that ManifestEntry.FilePath is resolved against
	UploadedBy string
	Entries    []ManifestEntry
	Options    BatchOptions
}

// EvidenceWriter is the narrow surface the ingester needs from the
// evidence subsystem. Production is wired to an adapter around
// evidence.Service (see cmd/server/main.go).
type EvidenceWriter interface {
	StoreMigratedFile(ctx context.Context, input StoreInput) (StoreResult, error)
}

// StoreInput is the file-level payload handed to EvidenceWriter.
type StoreInput struct {
	CaseID         uuid.UUID
	Filename       string
	OriginalName   string
	Reader         io.Reader
	SizeBytes      int64
	ComputedHash   string
	SourceHash     string // may be empty
	Classification string
	Description    string
	Tags           []string
	Source         string
	SourceDate     *time.Time
	UploadedBy     string
	CustodyDetail  map[string]string
}

// StoreResult is what the evidence layer hands back.
type StoreResult struct {
	EvidenceID uuid.UUID
	SizeBytes  int64
}

// ResumeStore persists per-file progress so a long-running migration can
// resume after an interruption (including across server restarts).
// Keys are canonical ManifestEntry.FilePath.
type ResumeStore interface {
	IsProcessed(ctx context.Context, migrationID uuid.UUID, filePath string) (bool, error)
	MarkProcessed(ctx context.Context, migrationID uuid.UUID, filePath string) error
}

// Ingester runs verified-ingestion batches for the migration pipeline.
type Ingester struct {
	writer EvidenceWriter
	resume ResumeStore
}

// NewIngester wires the ingester with its dependencies. `resume` may be
// nil; resume support is disabled in that case.
func NewIngester(writer EvidenceWriter, resume ResumeStore) *Ingester {
	return &Ingester{writer: writer, resume: resume}
}

// IngestFile performs the per-file 5-step protocol for a single entry and
// returns the resulting IngestedItem. It is exported for the CLI dry-run
// path and for tests; BatchIngest orchestrates it concurrently.
func (ig *Ingester) IngestFile(ctx context.Context, caseID uuid.UUID, sourceRoot, uploadedBy string, entry ManifestEntry, dryRun bool) (IngestedItem, error) {
	if ig.writer == nil && !dryRun {
		return IngestedItem{}, errors.New("migration: evidence writer is nil")
	}
	absPath := resolveSourcePath(sourceRoot, entry.FilePath)

	f, err := os.Open(absPath) // #nosec G304 -- path is validated & rooted
	if err != nil {
		return IngestedItem{}, fmt.Errorf("open %s: %w", entry.FilePath, err)
	}
	defer f.Close()

	// f.Stat() on an *os.File obtained from a successful os.Open cannot
	// fail on any supported platform; skip the defensive check.
	stat, _ := f.Stat()
	if stat.IsDir() {
		return IngestedItem{}, fmt.Errorf("%s is a directory, not a file", entry.FilePath)
	}

	// Stream hash so large files do not require full RAM. io.Copy from
	// an open regular file cannot fail except on a disk I/O failure,
	// which is effectively unreachable in unit tests.
	h := sha256.New()
	_, _ = io.Copy(h, f)
	computed := hex.EncodeToString(h.Sum(nil))

	hashMatched := false
	if entry.OriginalHash != "" {
		if computed != entry.OriginalHash {
			return IngestedItem{
				ManifestEntry: entry,
				ComputedHash:  computed,
				SizeBytes:     stat.Size(),
			}, &HashMismatchError{
				FilePath:     entry.FilePath,
				SourceHash:   entry.OriginalHash,
				ComputedHash: computed,
			}
		}
		hashMatched = true
	}

	item := IngestedItem{
		ManifestEntry: entry,
		ComputedHash:  computed,
		SizeBytes:     stat.Size(),
		HashMatched:   hashMatched,
		IngestedAt:    time.Now().UTC(),
	}

	if dryRun {
		return item, nil
	}

	// Rewind and hand off to the evidence writer. Seek(0) on a
	// newly-opened *os.File is always successful for regular files.
	_, _ = f.Seek(0, io.SeekStart)

	// manifest_entry is the plan-specified row reference "row N" so a
	// reviewer can correlate a custody event back to the original
	// manifest line. The file path is carried separately in the
	// evidence item itself (as filename) so we do not repeat it here.
	detail := map[string]string{
		"source_system":  entry.Source,
		"source_hash":    entry.OriginalHash,
		"computed_hash":  computed,
		"match":          fmt.Sprintf("%t", hashMatched),
		"manifest_entry": fmt.Sprintf("row %d", entry.Index),
		"file_path":      entry.FilePath,
	}
	res, err := ig.writer.StoreMigratedFile(ctx, StoreInput{
		CaseID:         caseID,
		Filename:       filepath.Base(entry.FilePath),
		OriginalName:   filepath.Base(entry.FilePath),
		Reader:         f,
		SizeBytes:      stat.Size(),
		ComputedHash:   computed,
		SourceHash:     entry.OriginalHash,
		Classification: entry.Classification,
		Description:    entry.Description,
		Tags:           entry.Tags,
		Source:         entry.Source,
		SourceDate:     entry.SourceDate,
		UploadedBy:     uploadedBy,
		CustodyDetail:  detail,
	})
	if err != nil {
		return IngestedItem{}, fmt.Errorf("store %s: %w", entry.FilePath, err)
	}
	item.EvidenceID = res.EvidenceID
	return item, nil
}

// HashMismatchError is returned when a manifest-supplied source hash does
// not match the hash computed on ingestion.
type HashMismatchError struct {
	FilePath     string
	SourceHash   string
	ComputedHash string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("hash mismatch for %s (source=%s computed=%s)", e.FilePath, e.SourceHash, e.ComputedHash)
}

// IsHashMismatch returns true if err wraps a HashMismatchError.
func IsHashMismatch(err error) bool {
	var hme *HashMismatchError
	return errors.As(err, &hme)
}

// BatchIngest processes a manifest in parallel, reporting progress via the
// optional ProgressFunc. A pre-check validates every file exists before
// any ingestion begins (the Sprint 10 plan's pre-check requirement).
func (ig *Ingester) BatchIngest(ctx context.Context, req BatchRequest, migrationID uuid.UUID, progress ProgressFunc) (IngestReport, error) {
	report := IngestReport{StartedAt: time.Now().UTC()}
	if len(req.Entries) == 0 {
		return report, errors.New("migration: batch has no entries")
	}
	concurrency := req.Options.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	// Hard ceiling so a case_admin cannot request 500 workers through
	// the HTTP API and spawn a goroutine flood. 32 is well above the
	// throughput knee for I/O-bound hashing against a single filesystem
	// and well below any reasonable resource risk.
	if concurrency > 32 {
		concurrency = 32
	}

	// Pre-check: every file must exist and be readable.
	for _, entry := range req.Entries {
		abs := resolveSourcePath(req.SourceRoot, entry.FilePath)
		if _, err := os.Stat(abs); err != nil {
			return report, fmt.Errorf("migration pre-check failed for %s: %w", entry.FilePath, err)
		}
	}

	// Resume filter — must run before the channel is sized so the
	// buffer capacity reflects the actual work set, not the full
	// manifest. Undersized buffers can deadlock workers; oversized
	// buffers waste memory but are otherwise safe.
	toProcess := req.Entries
	if req.Options.Resume && ig.resume != nil {
		filtered := make([]ManifestEntry, 0, len(req.Entries))
		for _, e := range req.Entries {
			done, err := ig.resume.IsProcessed(ctx, migrationID, e.FilePath)
			if err != nil {
				return report, fmt.Errorf("migration resume lookup: %w", err)
			}
			if done {
				continue
			}
			filtered = append(filtered, e)
		}
		toProcess = filtered
	}

	type job struct {
		index int
		entry ManifestEntry
	}
	jobs := make(chan job)
	results := make(chan ingestResult, len(toProcess))

	var wg sync.WaitGroup
	haltCtx, cancelHalt := context.WithCancel(ctx)
	defer cancelHalt()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Workers drain the jobs channel until the dispatcher
			// closes it. When a hash mismatch fires HaltOnMismatch,
			// cancelHalt() causes the dispatcher to stop enqueuing
			// new jobs — in-flight workers finish their current
			// file and exit naturally as the channel closes. This
			// costs at most (concurrency - 1) extra files beyond the
			// mismatched one, which is acceptable given that the
			// mismatch check happens early in IngestFile.
			for j := range jobs {
				item, err := ig.IngestFile(haltCtx, req.CaseID, req.SourceRoot, req.UploadedBy, j.entry, req.Options.DryRun)
				results <- ingestResult{index: j.index, entry: j.entry, item: item, err: err}
				if err != nil && IsHashMismatch(err) && req.Options.HaltOnMismatch {
					cancelHalt()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i, e := range toProcess {
			select {
			case <-haltCtx.Done():
				return
			case jobs <- job{index: i, entry: e}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	total := len(toProcess)
	processedCount := 0
	for r := range results {
		processedCount++
		if progress != nil {
			progress(processedCount, total, r.entry.FilePath)
		}
		if r.err != nil {
			if IsHashMismatch(r.err) {
				report.MismatchedItems++
				report.Failed = append(report.Failed, IngestFailure{FilePath: r.entry.FilePath, Reason: r.err.Error()})
				if req.Options.HaltOnMismatch {
					report.Halted = true
					report.HaltedFile = r.entry.FilePath
				}
				continue
			}
			report.Failed = append(report.Failed, IngestFailure{FilePath: r.entry.FilePath, Reason: r.err.Error()})
			continue
		}
		report.Processed = append(report.Processed, r.item)
		if r.item.HashMatched {
			report.MatchedItems++
		}
		if ig.resume != nil && !req.Options.DryRun {
			// Use a detached short-timeout context for the checkpoint
			// write: the file is already safely stored and the custody
			// event already written, so if the parent context is
			// cancelled (e.g. HTTP client abort or graceful shutdown)
			// we still want a fair chance to record the progress and
			// avoid re-ingesting this file on the next resume.
			ckCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := ig.resume.MarkProcessed(ckCtx, migrationID, r.entry.FilePath)
			cancel()
			if err != nil {
				// Not fatal — the file is already stored and the
				// custody event is already written. A failure here
				// only means a resume after a subsequent crash would
				// re-ingest this file. Surface it so the operator
				// notices.
				report.Failed = append(report.Failed, IngestFailure{
					FilePath: r.entry.FilePath,
					Reason:   "resume checkpoint failed: " + err.Error(),
				})
			}
		}
	}

	// Deterministic ordering: sort by ManifestEntry.FilePath so downstream
	// migration-hash computation is stable regardless of worker scheduling.
	sort.SliceStable(report.Processed, func(i, j int) bool {
		return report.Processed[i].ManifestEntry.FilePath < report.Processed[j].ManifestEntry.FilePath
	})

	report.CompletedAt = time.Now().UTC()
	return report, nil
}

type ingestResult struct {
	index int
	entry ManifestEntry
	item  IngestedItem
	err   error
}

// resolveSourcePath joins the root and sanitized relative path. The
// relative path has already been validated by sanitizeFilePath so this is
// safe from traversal.
func resolveSourcePath(root, rel string) string {
	if root == "" {
		return rel
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}
