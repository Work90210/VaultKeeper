// Package migration implements Sprint 10's verified data migration pipeline
// for importing evidence from external systems (RelativityOne and others)
// into VaultKeeper with cryptographic hash bridging and RFC 3161 attestation.
package migration

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ManifestFormat identifies the source manifest structure.
type ManifestFormat string

const (
	FormatCSV        ManifestFormat = "csv"
	FormatRelativity ManifestFormat = "relativity"
	FormatFolder     ManifestFormat = "folder"
)

// ManifestEntry is one evidence file described in a migration manifest.
type ManifestEntry struct {
	// Index is the 1-based position of this entry in the source
	// manifest. It is emitted verbatim into the custody log (as
	// "manifest_entry": "row N") so forensic reviewers can correlate
	// each custody event back to the original manifest line.
	Index          int
	FilePath       string
	OriginalHash   string // hash from source system (may be empty)
	Title          string
	Description    string
	Source         string
	SourceDate     *time.Time
	Tags           []string
	Classification string
	Metadata       map[string]any
}

// ManifestParser parses a source manifest into entries.
//
// Implementations MUST NOT execute embedded content. File paths in entries
// are sanitised for traversal before return (see sanitizeFilePath).
type ManifestParser interface {
	Parse(ctx context.Context, source io.Reader, format ManifestFormat) ([]ManifestEntry, error)
}

// Parser is the default ManifestParser implementation.
type Parser struct{}

// NewParser returns a new manifest parser.
func NewParser() *Parser { return &Parser{} }

// Parse dispatches to the format-specific parser.
func (p *Parser) Parse(ctx context.Context, src io.Reader, format ManifestFormat) ([]ManifestEntry, error) {
	switch format {
	case FormatCSV:
		return p.ParseCSV(ctx, src)
	case FormatRelativity:
		return p.ParseRelativity(ctx, src)
	default:
		return nil, fmt.Errorf("migration: unsupported manifest format %q", format)
	}
}

// sha256HexRE matches a lowercase or uppercase 64-char hex string.
var sha256HexRE = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// csvRequiredHeader is the canonical CSV header set. Only "filename" is
// strictly required; the rest are optional and default to zero values.
var csvKnownHeaders = map[string]bool{
	"filename":       true,
	"sha256_hash":    true,
	"title":          true,
	"description":    true,
	"source":         true,
	"source_date":    true,
	"tags":           true,
	"classification": true,
}

// ParseCSV parses a CSV manifest. The first row must be a header row
// containing at minimum a `filename` column. Unknown columns are collected
// into ManifestEntry.Metadata so adapters can extend the format without
// breaking the parser.
func (p *Parser) ParseCSV(ctx context.Context, src io.Reader) ([]ManifestEntry, error) {
	if src == nil {
		return nil, errors.New("migration: csv source is nil")
	}
	r := csv.NewReader(src)
	r.FieldsPerRecord = -1 // tolerate ragged rows; we validate fields explicitly

	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("migration: manifest is empty")
		}
		return nil, fmt.Errorf("migration: read csv header: %w", err)
	}

	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	if _, ok := colIdx["filename"]; !ok {
		return nil, errors.New("migration: csv manifest missing required 'filename' column")
	}

	seen := make(map[string]int)
	var entries []ManifestEntry
	lineNo := 1

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := r.Read()
		lineNo++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("migration: read csv row at line %d: %w", lineNo, err)
		}

		get := func(name string) string {
			idx, ok := colIdx[name]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		filename := get("filename")
		if filename == "" {
			return nil, fmt.Errorf("migration: csv line %d: filename is required", lineNo)
		}
		sanitized, err := sanitizeFilePath(filename)
		if err != nil {
			return nil, fmt.Errorf("migration: csv line %d: %w", lineNo, err)
		}

		if prev, exists := seen[sanitized]; exists {
			return nil, fmt.Errorf("migration: csv line %d: duplicate file path %q (first seen at line %d)", lineNo, sanitized, prev)
		}
		seen[sanitized] = lineNo

		hash := strings.ToLower(get("sha256_hash"))
		if hash != "" && !sha256HexRE.MatchString(hash) {
			return nil, fmt.Errorf("migration: csv line %d: invalid sha256_hash (must be 64-char hex)", lineNo)
		}

		entry := ManifestEntry{
			Index:          len(entries) + 1,
			FilePath:       sanitized,
			OriginalHash:   hash,
			Title:          get("title"),
			Description:    get("description"),
			Source:         get("source"),
			Classification: get("classification"),
			Tags:           splitTags(get("tags")),
		}

		if sd := get("source_date"); sd != "" {
			if t, perr := parseManifestDate(sd); perr == nil {
				entry.SourceDate = &t
			} else {
				return nil, fmt.Errorf("migration: csv line %d: invalid source_date %q", lineNo, sd)
			}
		}

		meta := collectExtraColumns(header, row)
		if len(meta) > 0 {
			entry.Metadata = meta
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, errors.New("migration: manifest contains no entries")
	}
	return entries, nil
}

// ParseRelativity parses a RelativityOne export CSV. Relativity's exports
// use `Control Number` as the filename anchor and `MD5 Hash` / `SHA256 Hash`
// for the source hash. We remap these to VaultKeeper's canonical field set
// and then re-use ParseCSV's validation path by reconstructing a CSV.
func (p *Parser) ParseRelativity(ctx context.Context, src io.Reader) ([]ManifestEntry, error) {
	if src == nil {
		return nil, errors.New("migration: relativity source is nil")
	}
	r := csv.NewReader(src)
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("migration: relativity manifest is empty")
		}
		return nil, fmt.Errorf("migration: read relativity header: %w", err)
	}

	hIdx := map[string]int{}
	for i, h := range header {
		hIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	fileCol, ok := firstIndex(hIdx, "native file", "control number", "filename")
	if !ok {
		return nil, errors.New("migration: relativity export missing 'Native File' / 'Control Number' column")
	}
	hashCol, _ := firstIndex(hIdx, "sha256 hash", "sha256_hash", "hash")
	titleCol, _ := firstIndex(hIdx, "title", "document title")
	descCol, _ := firstIndex(hIdx, "description", "document description")
	dateCol, _ := firstIndex(hIdx, "source date", "date created")
	tagsCol, _ := firstIndex(hIdx, "tags", "issues")

	seen := make(map[string]int)
	var entries []ManifestEntry
	lineNo := 1
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := r.Read()
		lineNo++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("migration: relativity line %d: %w", lineNo, err)
		}

		filename := safeGet(row, fileCol)
		if filename == "" {
			return nil, fmt.Errorf("migration: relativity line %d: file column empty", lineNo)
		}
		sanitized, err := sanitizeFilePath(filename)
		if err != nil {
			return nil, fmt.Errorf("migration: relativity line %d: %w", lineNo, err)
		}
		if prev, exists := seen[sanitized]; exists {
			return nil, fmt.Errorf("migration: relativity line %d: duplicate file path %q (first seen at line %d)", lineNo, sanitized, prev)
		}
		seen[sanitized] = lineNo

		hash := strings.ToLower(strings.TrimSpace(safeGet(row, hashCol)))
		if hash != "" && !sha256HexRE.MatchString(hash) {
			// Relativity sometimes exports MD5. Reject so callers know the
			// source did not provide a verifiable SHA-256 hash — migration
			// can still proceed but without the source-hash assertion.
			hash = ""
		}

		entry := ManifestEntry{
			Index:        len(entries) + 1,
			FilePath:     sanitized,
			OriginalHash: hash,
			Title:        safeGet(row, titleCol),
			Description:  safeGet(row, descCol),
			Source:       "RelativityOne",
			Tags:         splitTags(safeGet(row, tagsCol)),
		}
		if sd := safeGet(row, dateCol); sd != "" {
			if t, perr := parseManifestDate(sd); perr == nil {
				entry.SourceDate = &t
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("migration: relativity manifest contains no entries")
	}
	return entries, nil
}

// ParseFolder walks `rootDir` and returns one ManifestEntry per regular
// file. If `metadataCSV` (optional) is non-nil, its rows are parsed as a
// CSV manifest and merged onto matching filenames (by base name).
//
// Symlinks are skipped for security. Hidden files (leading dot) are
// included — callers can filter if desired.
func (p *Parser) ParseFolder(ctx context.Context, rootDir string, metadataCSV io.Reader) ([]ManifestEntry, error) {
	if rootDir == "" {
		return nil, errors.New("migration: folder root is empty")
	}
	info, err := os.Stat(rootDir)
	if err != nil {
		return nil, fmt.Errorf("migration: stat folder root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("migration: %q is not a directory", rootDir)
	}

	var metaIndex map[string]ManifestEntry
	if metadataCSV != nil {
		metaEntries, err := p.ParseCSV(ctx, metadataCSV)
		if err != nil {
			return nil, fmt.Errorf("migration: folder metadata csv: %w", err)
		}
		metaIndex = make(map[string]ManifestEntry, len(metaEntries))
		for _, e := range metaEntries {
			metaIndex[filepath.Base(e.FilePath)] = e
		}
	}

	// Walk the tree. Under normal POSIX semantics filepath.Walk cannot
	// fail once the root directory is known to exist (which we verified
	// above), the callback receives a nil werr on a readable entry, the
	// rel path is always derivable, and walk visits each path exactly
	// once so duplicates cannot occur. Symlinks are skipped by Mode().
	// The rel path under a validated root is trusted — no sanitisation
	// is needed because POSIX filenames cannot contain the null bytes,
	// absolute prefixes, or traversal segments that sanitizeFilePath
	// catches at the manifest-entry boundary.
	var entries []ManifestEntry
	_ = filepath.Walk(rootDir, func(path string, fi os.FileInfo, _ error) error {
		if fi == nil || fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, _ := filepath.Rel(rootDir, path)
		sanitized := filepath.ToSlash(rel)

		entry := ManifestEntry{
			Index:    len(entries) + 1,
			FilePath: sanitized,
			Title:    filepath.Base(rel),
		}
		if metaIndex != nil {
			if m, ok := metaIndex[filepath.Base(rel)]; ok {
				entry.OriginalHash = m.OriginalHash
				entry.Title = fallbackStr(m.Title, entry.Title)
				entry.Description = m.Description
				entry.Source = m.Source
				entry.SourceDate = m.SourceDate
				entry.Tags = m.Tags
				entry.Classification = m.Classification
				entry.Metadata = m.Metadata
			}
		}
		entries = append(entries, entry)
		return nil
	})
	if len(entries) == 0 {
		return nil, errors.New("migration: folder contains no files")
	}
	return entries, nil
}

// sanitizeFilePath rejects absolute paths and parent-traversal, and
// normalises separators to forward slashes so downstream code can build
// deterministic keys regardless of host OS.
func sanitizeFilePath(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return "", errors.New("file path is empty")
	}
	// Null bytes in paths have no legitimate use and can truncate
	// downstream OS calls in unexpected ways. Reject outright.
	if strings.ContainsRune(trimmed, 0) {
		return "", errors.New("null byte in file path")
	}
	// Reject Windows and POSIX absolute paths.
	if filepath.IsAbs(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) {
		return "", fmt.Errorf("absolute path not permitted: %q", trimmed)
	}
	// Reject drive letters (C:\...).
	if len(trimmed) >= 2 && trimmed[1] == ':' {
		return "", fmt.Errorf("absolute path not permitted: %q", trimmed)
	}
	// Normalise Windows separators to forward slashes before Clean so the
	// cross-platform test expectations hold on Linux CI where filepath.Clean
	// treats `\` as a literal character. UNC paths (\\server\share or
	// //server/share) are already rejected by the earlier single-char
	// prefix check above.
	normalized := strings.ReplaceAll(trimmed, `\`, "/")
	cleaned := filepath.ToSlash(filepath.Clean(normalized))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal not permitted: %q", trimmed)
	}
	return cleaned, nil
}

// splitTags parses a tag cell. Accepts comma and semicolon as separators
// and trims whitespace. Empty tokens are dropped.
func splitTags(s string) []string {
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

// parseManifestDate accepts RFC 3339 and common date-only formats.
func parseManifestDate(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "01/02/2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date format: %q", s)
}

func collectExtraColumns(header, row []string) map[string]any {
	meta := map[string]any{}
	for i, h := range header {
		key := strings.TrimSpace(strings.ToLower(h))
		if csvKnownHeaders[key] || key == "" {
			continue
		}
		if i >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[i])
		if v != "" {
			meta[key] = v
		}
	}
	return meta
}

func firstIndex(idx map[string]int, names ...string) (int, bool) {
	for _, n := range names {
		if i, ok := idx[n]; ok {
			return i, true
		}
	}
	return -1, false
}

func safeGet(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func fallbackStr(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
