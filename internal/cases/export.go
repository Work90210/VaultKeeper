package cases

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// EvidenceExporter provides evidence data for case export.
type EvidenceExporter interface {
	ListByCaseForExport(ctx context.Context, caseID uuid.UUID, userRole string) ([]evidence.EvidenceItem, error)
}

// CustodyExporter provides custody log data for case export.
type CustodyExporter interface {
	ListAllByCase(ctx context.Context, caseID uuid.UUID) ([]custody.Event, error)
}

// CaseExporter provides case data for export.
type CaseExporter interface {
	FindByID(ctx context.Context, id uuid.UUID) (Case, error)
}

// FileDownloader retrieves evidence files from object storage.
type FileDownloader interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
}

// ExportCustodyLogger records export events in the custody chain.
type ExportCustodyLogger interface {
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error
}

// zipCreator is a minimal interface for the subset of *zip.Writer that write
// methods need. Allows error-path testing since Go's zip.Writer buffers
// everything internally and never returns errors from Create/Write.
type zipCreator interface {
	Create(name string) (io.Writer, error)
}

// zipArchive extends zipCreator with Close for the full export pipeline.
type zipArchive interface {
	zipCreator
	Close() error
}

const (
	exportVersion = "1.0"
)

// ExportService creates ZIP archives of case data for legal export.
type ExportService struct {
	evidenceRepo EvidenceExporter
	custodyRepo  CustodyExporter
	caseRepo     CaseExporter
	storage      FileDownloader
	custody      ExportCustodyLogger
}

// NewExportService creates a new export service with the required dependencies.
func NewExportService(
	evidenceRepo EvidenceExporter,
	custodyRepo CustodyExporter,
	caseRepo CaseExporter,
	storage FileDownloader,
	custodyLogger ExportCustodyLogger,
) *ExportService {
	return &ExportService{
		evidenceRepo: evidenceRepo,
		custodyRepo:  custodyRepo,
		caseRepo:     caseRepo,
		storage:      storage,
		custody:      custodyLogger,
	}
}

type exportManifest struct {
	ExportDate   time.Time `json:"export_date"`
	Version      string    `json:"version"`
	CaseID       string    `json:"case_id"`
	Reference    string    `json:"reference_code"`
	Title        string    `json:"title"`
	Jurisdiction string    `json:"jurisdiction"`
	Status       string    `json:"status"`
	TotalItems   int       `json:"total_items"`
	ExportedBy   string    `json:"exported_by"`
}

// ExportCase streams a ZIP archive of all case data to the provided writer.
// For defence users, only disclosed evidence is included.
func (s *ExportService) ExportCase(ctx context.Context, caseID uuid.UUID, userRole string, actorUserID string, w io.Writer) error {
	caseData, err := s.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return fmt.Errorf("load case for export: %w", err)
	}

	items, err := s.evidenceRepo.ListByCaseForExport(ctx, caseID, userRole)
	if err != nil {
		return fmt.Errorf("list evidence for export: %w", err)
	}

	events, err := s.custodyRepo.ListAllByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("list custody events for export: %w", err)
	}

	prefix := caseData.ReferenceCode + "-export/"

	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := s.buildArchive(ctx, zw, prefix, caseData, items, events, actorUserID); err != nil {
		return err
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}

	// Log the export as a custody event (best-effort, don't fail the export)
	_ = s.custody.RecordCaseEvent(ctx, caseID, "case_exported", actorUserID, map[string]string{
		"evidence_count": fmt.Sprintf("%d", len(items)),
		"user_role":      userRole,
	})

	return nil
}

func (s *ExportService) buildArchive(ctx context.Context, zw zipCreator, prefix string, caseData Case, items []evidence.EvidenceItem, events []custody.Event, actorUserID string) error {
	if err := s.writeManifest(zw, prefix, caseData, items, actorUserID); err != nil {
		return err
	}
	if err := s.writeCaseJSON(zw, prefix, caseData); err != nil {
		return err
	}
	if err := s.writeEvidenceFiles(ctx, zw, prefix, items); err != nil {
		return err
	}
	if err := s.writeMetadataCSV(zw, prefix, items); err != nil {
		return err
	}
	if err := s.writeCustodyCSV(zw, prefix, events); err != nil {
		return err
	}
	if err := s.writeHashesCSV(zw, prefix, items); err != nil {
		return err
	}
	if err := s.writeREADME(zw, prefix, caseData); err != nil {
		return err
	}
	return nil
}

// GetReferenceCode returns the reference code for a case, used by the handler.
func (s *ExportService) GetReferenceCode(ctx context.Context, caseID uuid.UUID) (string, error) {
	c, err := s.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return "", fmt.Errorf("get reference code for export: %w", err)
	}
	return c.ReferenceCode, nil
}

func (s *ExportService) writeManifest(zw zipCreator, prefix string, c Case, items []evidence.EvidenceItem, actorUserID string) error {
	manifest := exportManifest{
		ExportDate:   time.Now().UTC(),
		Version:      exportVersion,
		CaseID:       c.ID.String(),
		Reference:    c.ReferenceCode,
		Title:        c.Title,
		Jurisdiction: c.Jurisdiction,
		Status:       c.Status,
		TotalItems:   len(items),
		ExportedBy:   actorUserID,
	}

	fw, err := zw.Create(prefix + "manifest.json")
	if err != nil {
		return fmt.Errorf("create manifest entry: %w", err)
	}

	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return nil
}

func (s *ExportService) writeCaseJSON(zw zipCreator, prefix string, c Case) error {
	fw, err := zw.Create(prefix + "case.json")
	if err != nil {
		return fmt.Errorf("create case.json entry: %w", err)
	}

	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode case.json: %w", err)
	}
	return nil
}

func (s *ExportService) writeEvidenceFiles(ctx context.Context, zw zipCreator, prefix string, items []evidence.EvidenceItem) error {
	for _, item := range items {
		if item.StorageKey == nil {
			continue
		}

		evidenceNum := ""
		if item.EvidenceNumber != nil {
			evidenceNum = *item.EvidenceNumber
		}

		filename := evidenceNum + "_" + item.Filename
		fw, err := zw.Create(prefix + "evidence/" + filename)
		if err != nil {
			return fmt.Errorf("create evidence zip entry %s: %w", filename, err)
		}

		rc, _, _, err := s.storage.GetObject(ctx, *item.StorageKey)
		if err != nil {
			return fmt.Errorf("download evidence file %s: %w", *item.StorageKey, err)
		}

		if _, err := io.Copy(fw, rc); err != nil {
			rc.Close()
			return fmt.Errorf("copy evidence file %s to zip: %w", filename, err)
		}
		rc.Close()
	}
	return nil
}

func (s *ExportService) writeMetadataCSV(zw zipCreator, prefix string, items []evidence.EvidenceItem) error {
	fw, err := zw.Create(prefix + "metadata.csv")
	if err != nil {
		return fmt.Errorf("create metadata.csv entry: %w", err)
	}

	cw := csv.NewWriter(fw)
	header := []string{
		"evidence_number", "filename", "original_name", "mime_type",
		"size_bytes", "sha256_hash", "classification", "description",
		"uploaded_by", "tsa_status", "tsa_timestamp", "created_at",
	}
	cw.Write(header)

	for _, item := range items {
		evidenceNum := ""
		if item.EvidenceNumber != nil {
			evidenceNum = *item.EvidenceNumber
		}
		tsaTimestamp := ""
		if item.TSATimestamp != nil {
			tsaTimestamp = item.TSATimestamp.Format(time.RFC3339)
		}

		row := []string{
			evidenceNum,
			item.Filename,
			item.OriginalName,
			item.MimeType,
			fmt.Sprintf("%d", item.SizeBytes),
			item.SHA256Hash,
			item.Classification,
			item.Description,
			item.UploadedBy,
			item.TSAStatus,
			tsaTimestamp,
			item.CreatedAt.Format(time.RFC3339),
		}
		cw.Write(row)
	}

	cw.Flush()
	return cw.Error()
}

func (s *ExportService) writeCustodyCSV(zw zipCreator, prefix string, events []custody.Event) error {
	fw, err := zw.Create(prefix + "custody_log.csv")
	if err != nil {
		return fmt.Errorf("create custody_log.csv entry: %w", err)
	}

	cw := csv.NewWriter(fw)
	header := []string{
		"id", "case_id", "evidence_id", "action", "actor_user_id",
		"detail", "hash_value", "previous_hash", "timestamp",
	}
	cw.Write(header)

	for _, e := range events {
		evidenceID := ""
		if e.EvidenceID != uuid.Nil {
			evidenceID = e.EvidenceID.String()
		}

		row := []string{
			e.ID.String(),
			e.CaseID.String(),
			evidenceID,
			e.Action,
			e.ActorUserID,
			e.Detail,
			e.HashValue,
			e.PreviousHash,
			e.Timestamp.Format(time.RFC3339),
		}
		cw.Write(row)
	}

	cw.Flush()
	return cw.Error()
}

func (s *ExportService) writeHashesCSV(zw zipCreator, prefix string, items []evidence.EvidenceItem) error {
	fw, err := zw.Create(prefix + "hashes.csv")
	if err != nil {
		return fmt.Errorf("create hashes.csv entry: %w", err)
	}

	cw := csv.NewWriter(fw)
	header := []string{"evidence_number", "filename", "sha256_hash", "tsa_status", "tsa_timestamp"}
	cw.Write(header)

	for _, item := range items {
		evidenceNum := ""
		if item.EvidenceNumber != nil {
			evidenceNum = *item.EvidenceNumber
		}
		tsaTimestamp := ""
		if item.TSATimestamp != nil {
			tsaTimestamp = item.TSATimestamp.Format(time.RFC3339)
		}

		row := []string{
			evidenceNum,
			item.Filename,
			item.SHA256Hash,
			item.TSAStatus,
			tsaTimestamp,
		}
		cw.Write(row)
	}

	cw.Flush()
	return cw.Error()
}

func (s *ExportService) writeREADME(zw zipCreator, prefix string, c Case) error {
	fw, err := zw.Create(prefix + "README.txt")
	if err != nil {
		return fmt.Errorf("create README.txt entry: %w", err)
	}

	content := fmt.Sprintf(`VaultKeeper Case Export
======================

Case Reference: %s
Case Title:     %s
Jurisdiction:   %s
Export Date:     %s
Format Version: %s

Contents
--------
manifest.json    - Export metadata (date, version, case info, total items)
case.json        - Full case record
evidence/        - Evidence files named as {evidence_number}_{filename}
metadata.csv     - One row per evidence item with all metadata fields
custody_log.csv  - Complete chain-of-custody log for the case
hashes.csv       - SHA-256 hash and TSA timestamp status per evidence item
README.txt       - This file

Integrity Verification
----------------------
Each evidence file's SHA-256 hash is recorded in hashes.csv.
Recompute the hash of any file and compare against the recorded value
to verify integrity. TSA timestamps provide independent proof that the
file existed at the recorded time.

The custody log includes a hash chain where each entry's hash depends
on the previous entry, ensuring the log has not been tampered with.
`, c.ReferenceCode, c.Title, c.Jurisdiction,
		time.Now().UTC().Format(time.RFC3339), exportVersion)

	if _, err := io.WriteString(fw, content); err != nil {
		return fmt.Errorf("write README.txt: %w", err)
	}
	return nil
}
