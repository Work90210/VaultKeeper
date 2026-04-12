package evidence

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// MigrationStoreInput mirrors migration.StoreInput without creating a
// package dependency cycle (migration imports evidence types; evidence
// must not import migration). Fields are a subset of UploadInput plus the
// dual-hash custody detail needed for the "migrated" custody event.
type MigrationStoreInput struct {
	CaseID         uuid.UUID
	Filename       string
	OriginalName   string
	Reader         io.Reader
	SizeBytes      int64
	ComputedHash   string
	SourceHash     string
	Classification string
	Description    string
	Tags           []string
	Source         string
	SourceDate     *time.Time
	UploadedBy     string
	CustodyDetail  map[string]string
}

// MigrationStoreResult mirrors migration.StoreResult.
type MigrationStoreResult struct {
	EvidenceID uuid.UUID
	SizeBytes  int64
}

// StoreMigratedFile ingests a file produced by the migration pipeline. It
// reuses the standard Upload path for hashing/storage/TSA stamping but
// then emits a `migrated` custody event carrying the source/computed hash
// pair instead of the default `evidence_uploaded` event.
//
// Callers must supply the pre-computed SHA-256 (from the ingester) as
// ComputedHash. This function trusts that value — any tamper would have
// already been caught by the ingester's hash bridging step.
func (s *Service) StoreMigratedFile(ctx context.Context, in MigrationStoreInput) (MigrationStoreResult, error) {
	up := UploadInput{
		CaseID:         in.CaseID,
		File:           in.Reader,
		Filename:       in.Filename,
		SizeBytes:      in.SizeBytes,
		Classification: in.Classification,
		Description:    in.Description,
		Tags:           in.Tags,
		UploadedBy:     in.UploadedBy,
		UploadedByName: in.UploadedBy,
		Source:         in.Source,
		SourceDate:     in.SourceDate,
	}
	if up.Classification == "" {
		up.Classification = ClassificationRestricted
	}
	item, err := s.Upload(ctx, up)
	if err != nil {
		return MigrationStoreResult{}, fmt.Errorf("migrated upload: %w", err)
	}
	// Hash-bridging assertion: the ingester computes SHA-256 before
	// handing the reader to Upload. Upload re-hashes the same bytes and
	// stores its own computed hash on the row. The two MUST agree —
	// otherwise the file's content changed between the ingester's hash
	// check and the Upload read, which would invalidate the whole
	// migration integrity guarantee. Fail loudly instead of silently
	// discarding the ingester's hash.
	if in.ComputedHash != "" && item.SHA256Hash != in.ComputedHash {
		return MigrationStoreResult{}, fmt.Errorf(
			"migrated upload: hash drift detected for %s (ingester=%s upload=%s)",
			in.Filename, in.ComputedHash, item.SHA256Hash,
		)
	}
	// Emit the migration-specific custody event in addition to the
	// default `evidence_uploaded` event from Upload. The caller-supplied
	// CustodyDetail is authoritative — it already carries the
	// manifest_entry "row N" reference set by the migration ingester —
	// so we seed the detail map with only the fields the adapter owns
	// and let the caller's map overwrite/extend them.
	detail := map[string]string{
		"source_system": in.Source,
		"source_hash":   in.SourceHash,
		"computed_hash": in.ComputedHash,
	}
	for k, v := range in.CustodyDetail {
		detail[k] = v
	}
	s.recordCustodyEvent(ctx, in.CaseID, item.ID, "migrated", in.UploadedBy, detail)
	return MigrationStoreResult{EvidenceID: item.ID, SizeBytes: item.SizeBytes}, nil
}
