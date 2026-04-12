package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// ComputeManifestHash returns a deterministic SHA-256 over the sorted
// canonical file list supplied by the manifest. It is used as a quick
// integrity check on the manifest itself, independent of the subsequent
// hash bridging during ingestion.
func ComputeManifestHash(entries []ManifestEntry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, e.FilePath+"|"+strings.ToLower(e.OriginalHash))
	}
	sort.Strings(lines)
	joined := strings.Join(lines, "\n")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}

// ComputeMigrationHash returns the deterministic migration hash as defined
// by Sprint 10 Step 3. The canonical serialisation is:
//
//	sha256(
//	    "source=<source>\n" +
//	    "started_at=<RFC3339 UTC>\n" +
//	    sorted lines of "<filepath>|<computed_hash>" joined by "\n"
//	)
//
// Each line binds the file path to its SHA-256 so that two manifests
// containing duplicate-content files cannot produce the same migration
// hash — sorting by the composite key keeps the output stable regardless
// of which worker processed which file first during parallel ingestion.
func ComputeMigrationHash(sourceSystem string, startedAt time.Time, items []IngestedItem) string {
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, it.ManifestEntry.FilePath+"|"+strings.ToLower(it.ComputedHash))
	}
	sort.Strings(lines)

	var sb strings.Builder
	sb.WriteString("source=")
	sb.WriteString(sourceSystem)
	sb.WriteString("\nstarted_at=")
	sb.WriteString(startedAt.UTC().Format(time.RFC3339))
	sb.WriteString("\n")
	for _, ln := range lines {
		sb.WriteString(ln)
		sb.WriteString("\n")
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}
