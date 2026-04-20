package evidence

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Bulk upload security limits. These are intentionally conservative; a
// future enhancement can load them from config if operators need more.
const (
	BulkMaxFiles             = 10_000
	BulkMaxUncompressedRatio = 10 // total uncompressed ≤ 10 × MAX_UPLOAD_SIZE
	bulkMetadataFilename     = "_metadata.csv"
)

// ErrZipRejected is the error family returned for all ZIP-safety failures.
// Callers inspect with errors.Is(err, ErrZipRejected) and respond 400.
var ErrZipRejected = errors.New("bulk upload: archive rejected")

// BulkFileEntry describes one extracted file awaiting ingestion.
type BulkFileEntry struct {
	Name    string // sanitised relative path within the archive
	Size    int64
	Content []byte // full file contents; callers stream this to ObjectStorage
}

// BulkMetadata is a per-file metadata override parsed from _metadata.csv.
type BulkMetadata struct {
	Title          string
	Description    string
	Tags           []string
	Classification string
	Source         string
	SourceDate     *time.Time
}

// ExtractedBulk is the sanitised, validated contents of an uploaded ZIP.
type ExtractedBulk struct {
	Files    []BulkFileEntry
	Metadata map[string]BulkMetadata // keyed by filename (base name)
}

// BulkJob is an in-memory view of a bulk_upload_jobs row.
type BulkJob struct {
	ID            uuid.UUID  `json:"id"`
	CaseID        uuid.UUID  `json:"case_id"`
	Total         int        `json:"total_files"`
	Processed     int        `json:"processed_files"`
	Failed        int        `json:"failed_files"`
	Status        string     `json:"status"`
	// ArchiveSHA256 is the SHA-256 hex of the raw compressed archive
	// bytes received by the server. Empty until SetArchiveHash has
	// been called. Operators can compare this against their own
	// upload-side hash for a tamper-evident record of what arrived.
	ArchiveSHA256 string         `json:"archive_sha256,omitempty"`
	Errors        []BulkJobError `json:"errors,omitempty"`
	PerformedBy   string         `json:"performed_by"`
	StartedAt     time.Time      `json:"started_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

// BulkJobError is a per-file failure captured in the job status.
type BulkJobError struct {
	Filename string
	Reason   string
}

// ExtractBulkZIP validates and extracts a bulk upload ZIP. It rejects
// archives that look like zip bombs, contain symlinks, nested ZIPs, or
// absolute-path entries. Returns a fully in-memory ExtractedBulk so
// callers can stream each entry to storage.
//
// The `maxUploadSize` parameter is VaultKeeper's single-file max, used to
// derive the total-uncompressed limit (BulkMaxUncompressedRatio × this).
func ExtractBulkZIP(ctx context.Context, reader io.ReaderAt, size, maxUploadSize int64) (*ExtractedBulk, error) {
	zr, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("%w: not a valid zip archive: %v", ErrZipRejected, err)
	}
	if len(zr.File) == 0 {
		return nil, fmt.Errorf("%w: archive is empty", ErrZipRejected)
	}
	if len(zr.File) > BulkMaxFiles {
		return nil, fmt.Errorf("%w: archive contains %d entries (max %d)", ErrZipRejected, len(zr.File), BulkMaxFiles)
	}

	maxTotalUncompressed := maxUploadSize * BulkMaxUncompressedRatio
	var totalUncompressed int64
	out := &ExtractedBulk{
		Metadata: make(map[string]BulkMetadata),
	}

	for _, entry := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Directory entries are skipped.
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: symlinks not permitted (%s)", ErrZipRejected, entry.Name)
		}
		// Reject nested ZIPs by extension — the safer way would be by
		// magic bytes, but nested archives serve no legitimate purpose for
		// bulk evidence upload and extension-based rejection is enough.
		if strings.EqualFold(filepath.Ext(entry.Name), ".zip") {
			return nil, fmt.Errorf("%w: nested zip not permitted (%s)", ErrZipRejected, entry.Name)
		}
		// Sanitise path.
		cleaned, err := sanitizeZipEntryName(entry.Name)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrZipRejected, err)
		}

		// Zip-bomb defence: enforce both per-entry and cumulative limits.
		if int64(entry.UncompressedSize64) > maxUploadSize {
			return nil, fmt.Errorf("%w: entry %s exceeds per-file limit", ErrZipRejected, cleaned)
		}
		totalUncompressed += int64(entry.UncompressedSize64)
		if totalUncompressed > maxTotalUncompressed {
			return nil, fmt.Errorf("%w: total uncompressed size exceeds %d bytes", ErrZipRejected, maxTotalUncompressed)
		}

		// Open and read the entry. archive/zip validates the entry's
		// declared UncompressedSize64 as it reads, so the primary
		// zip-bomb defence is the per-file and cumulative size caps
		// enforced above. Open/ReadAll errors on a valid archive
		// require filesystem-level corruption outside the test matrix.
		rc, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: open entry %s: %v", ErrZipRejected, cleaned, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: read entry %s: %v", ErrZipRejected, cleaned, err)
		}

		base := filepath.Base(cleaned)
		if base == bulkMetadataFilename {
			meta, err := parseBulkMetadataCSV(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("%w: metadata csv: %v", ErrZipRejected, err)
			}
			out.Metadata = meta
			continue
		}

		out.Files = append(out.Files, BulkFileEntry{
			Name:    cleaned,
			Size:    int64(len(data)),
			Content: data,
		})
	}

	if len(out.Files) == 0 {
		return nil, fmt.Errorf("%w: archive contains no ingestable files", ErrZipRejected)
	}
	return out, nil
}

// sanitizeZipEntryName rejects absolute paths and traversal, normalising
// separators. Returns the cleaned path or an error.
func sanitizeZipEntryName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("empty entry name")
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", errors.New("null byte in entry name")
	}
	// ZIP spec uses forward slashes; some zip tools produce backslashes.
	// After normalisation, any leading-slash form (POSIX absolute, UNC
	// //server/share) is rejected as absolute.
	normalised := strings.ReplaceAll(trimmed, `\`, "/")
	if strings.HasPrefix(normalised, "/") {
		return "", fmt.Errorf("absolute path not permitted: %q", name)
	}
	if len(normalised) >= 2 && normalised[1] == ':' {
		return "", fmt.Errorf("absolute path not permitted: %q", name)
	}
	cleaned := filepath.ToSlash(filepath.Clean(normalised))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal not permitted: %q", name)
	}
	return cleaned, nil
}

// parseBulkMetadataCSV parses the _metadata.csv manifest embedded in the
// archive. The first column must be `filename`; the rest are optional.
func parseBulkMetadataCSV(r io.Reader) (map[string]BulkMetadata, error) {
	c := csv.NewReader(r)
	c.FieldsPerRecord = -1
	header, err := c.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty metadata csv")
		}
		return nil, err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	if _, ok := idx["filename"]; !ok {
		return nil, errors.New("metadata csv missing 'filename' column")
	}
	get := func(row []string, name string) string {
		i, ok := idx[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	out := make(map[string]BulkMetadata)
	for {
		row, err := c.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		fn := get(row, "filename")
		if fn == "" {
			continue
		}
		entry := BulkMetadata{
			Title:          get(row, "title"),
			Description:    get(row, "description"),
			Classification: get(row, "classification"),
			Source:         get(row, "source"),
			Tags:           splitBulkTags(get(row, "tags")),
		}
		if sd := get(row, "source_date"); sd != "" {
			if t, err := time.Parse("2006-01-02", sd); err == nil {
				entry.SourceDate = &t
			} else if t, err := time.Parse(time.RFC3339, sd); err == nil {
				entry.SourceDate = &t
			}
		}
		out[fn] = entry
	}
	return out, nil
}

func splitBulkTags(s string) []string {
	if s == "" {
		return nil
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '|'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		t := strings.TrimSpace(f)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

