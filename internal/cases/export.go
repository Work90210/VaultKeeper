package cases

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// CaptureMetadataExporter provides capture metadata for case export.
type CaptureMetadataExporter interface {
	GetByEvidenceIDs(ctx context.Context, evidenceIDs []uuid.UUID) (map[uuid.UUID]*evidence.EvidenceCaptureMetadata, error)
}

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

// InvestigationExporter provides investigation data for case export.
type InvestigationExporter interface {
	ListInquiryLogsByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationInquiryLog, error)
	ListAssessmentsByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationAssessment, error)
	ListVerificationsByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationVerification, error)
	ListCorroborationsByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationCorroboration, error)
	ListAnalysisNotesByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationAnalysisNote, error)
	ListReportsByCase(ctx context.Context, caseID uuid.UUID) ([]InvestigationReport, error)
}

// Lightweight export DTOs — avoids importing the investigation package directly.
type InvestigationInquiryLog struct {
	ID              string `json:"id"`
	Objective       string `json:"objective"`
	SearchStrategy  string `json:"search_strategy"`
	SearchTool      string `json:"search_tool"`
	SearchURL       string `json:"search_url"`
	SearchStartedAt string `json:"search_started_at"`
	SearchEndedAt   string `json:"search_ended_at"`
	ResultsCount    int    `json:"results_count"`
	ResultsRelevant int    `json:"results_relevant"`
	Keywords        string `json:"keywords"`
	Notes           string `json:"notes"`
}

type InvestigationAssessment struct {
	ID                string `json:"id"`
	EvidenceID        string `json:"evidence_id"`
	RelevanceScore    int    `json:"relevance_score"`
	ReliabilityScore  int    `json:"reliability_score"`
	SourceCredibility string `json:"source_credibility"`
	Recommendation    string `json:"recommendation"`
	Methodology       string `json:"methodology"`
	CreatedAt         string `json:"created_at"`
}

type InvestigationVerification struct {
	ID               string `json:"id"`
	EvidenceID       string `json:"evidence_id"`
	VerificationType string `json:"verification_type"`
	Finding          string `json:"finding"`
	ConfidenceLevel  string `json:"confidence_level"`
	Methodology      string `json:"methodology"`
	ToolsUsed        string `json:"tools_used"`
	CreatedAt        string `json:"created_at"`
}

type InvestigationCorroboration struct {
	ID            string `json:"id"`
	ClaimSummary  string `json:"claim_summary"`
	ClaimType     string `json:"claim_type"`
	Strength      string `json:"strength"`
	EvidenceCount int    `json:"evidence_count"`
	CreatedAt     string `json:"created_at"`
}

type InvestigationAnalysisNote struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	AnalysisType string `json:"analysis_type"`
	Status       string `json:"status"`
	Methodology  string `json:"methodology"`
	CreatedAt    string `json:"created_at"`
}

type InvestigationReport struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	ReportType string          `json:"report_type"`
	Status     string          `json:"status"`
	Sections   json.RawMessage `json:"sections"`
	CreatedAt  string          `json:"created_at"`
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
	evidenceRepo    EvidenceExporter
	custodyRepo     CustodyExporter
	caseRepo        CaseExporter
	storage         FileDownloader
	custody         ExportCustodyLogger
	captureMetadata CaptureMetadataExporter // optional — Berkeley Protocol capture metadata
	investigation   InvestigationExporter   // optional — investigation data export
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

// WithCaptureMetadataExporter injects the capture metadata exporter.
func (s *ExportService) WithCaptureMetadataExporter(exporter CaptureMetadataExporter) *ExportService {
	s.captureMetadata = exporter
	return s
}

// WithInvestigationExporter injects the investigation data exporter.
func (s *ExportService) WithInvestigationExporter(exporter InvestigationExporter) *ExportService {
	s.investigation = exporter
	return s
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
	if err := s.writeCaptureMetadataCSV(ctx, zw, prefix, items); err != nil {
		return err
	}
	if err := s.writeCustodyCSV(zw, prefix, events); err != nil {
		return err
	}
	if err := s.writeHashesCSV(zw, prefix, items); err != nil {
		return err
	}
	if s.investigation != nil {
		if err := s.writeInvestigationData(ctx, zw, prefix, caseData.ID); err != nil {
			return err
		}
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

		filename := evidenceNum + "_" + filepath.Base(item.Filename)
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

func (s *ExportService) writeCaptureMetadataCSV(ctx context.Context, zw zipCreator, prefix string, items []evidence.EvidenceItem) error {
	if s.captureMetadata == nil {
		return nil // no capture metadata exporter configured — skip
	}

	evidenceIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		evidenceIDs = append(evidenceIDs, item.ID)
	}

	captureMap, err := s.captureMetadata.GetByEvidenceIDs(ctx, evidenceIDs)
	if err != nil {
		return fmt.Errorf("batch fetch capture metadata for export: %w", err)
	}

	if len(captureMap) == 0 {
		return nil // no capture metadata to export
	}

	fw, err := zw.Create(prefix + "capture_metadata.csv")
	if err != nil {
		return fmt.Errorf("create capture_metadata.csv entry: %w", err)
	}

	cw := csv.NewWriter(fw)
	header := []string{
		"evidence_number", "source_url", "platform", "capture_method",
		"capture_timestamp", "publication_timestamp", "content_language",
		"availability_status", "verification_status",
	}
	cw.Write(header)

	for _, item := range items {
		cm, ok := captureMap[item.ID]
		if !ok {
			continue
		}

		evidenceNum := ""
		if item.EvidenceNumber != nil {
			evidenceNum = *item.EvidenceNumber
		}

		pubTS := ""
		if cm.PublicationTimestamp != nil {
			pubTS = cm.PublicationTimestamp.Format(time.RFC3339)
		}

		row := []string{
			evidenceNum,
			derefStrPtr(cm.SourceURL),
			derefStrPtr(cm.Platform),
			cm.CaptureMethod,
			cm.CaptureTimestamp.Format(time.RFC3339),
			pubTS,
			derefStrPtr(cm.ContentLanguage),
			derefStrPtr(cm.AvailabilityStatus),
			cm.VerificationStatus,
		}
		cw.Write(row)
	}

	cw.Flush()
	return cw.Error()
}

func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
manifest.json         - Export metadata (date, version, case info, total items)
case.json             - Full case record
evidence/             - Evidence files named as {evidence_number}_{filename}
metadata.csv          - One row per evidence item with all metadata fields
capture_metadata.csv  - Berkeley Protocol capture provenance (online captures only)
custody_log.csv       - Complete chain-of-custody log for the case
hashes.csv            - SHA-256 hash and TSA timestamp status per evidence item
README.txt            - This file

Note: capture_metadata.csv excludes sensitive fields (collector identity,
network context, browser fingerprint) for security. These fields are
available through the VaultKeeper API with appropriate role permissions.

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

// --- Investigation data export ---

func (s *ExportService) writeInvestigationData(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	if err := s.writeInquiryLogsCSV(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	if err := s.writeAssessmentsCSV(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	if err := s.writeVerificationsCSV(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	if err := s.writeCorroborationsCSV(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	if err := s.writeAnalysisNotesCSV(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	if err := s.writeReportsJSON(ctx, zw, prefix, caseID); err != nil {
		return err
	}
	return nil
}

func (s *ExportService) writeInquiryLogsCSV(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	logs, err := s.investigation.ListInquiryLogsByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export inquiry logs: %w", err)
	}
	fw, err := zw.Create(prefix + "investigation/inquiry_logs.csv")
	if err != nil {
		return fmt.Errorf("create inquiry_logs.csv: %w", err)
	}
	w := csv.NewWriter(fw)
	_ = w.Write([]string{"id", "objective", "search_strategy", "search_tool", "search_url", "search_started_at", "search_ended_at", "results_count", "results_relevant", "keywords", "notes"})
	for _, l := range logs {
		_ = w.Write([]string{l.ID, l.Objective, l.SearchStrategy, l.SearchTool, l.SearchURL, l.SearchStartedAt, l.SearchEndedAt, fmt.Sprintf("%d", l.ResultsCount), fmt.Sprintf("%d", l.ResultsRelevant), l.Keywords, l.Notes})
	}
	w.Flush()
	return w.Error()
}

func (s *ExportService) writeAssessmentsCSV(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	items, err := s.investigation.ListAssessmentsByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export assessments: %w", err)
	}
	fw, err := zw.Create(prefix + "investigation/assessments.csv")
	if err != nil {
		return fmt.Errorf("create assessments.csv: %w", err)
	}
	w := csv.NewWriter(fw)
	_ = w.Write([]string{"id", "evidence_id", "relevance_score", "reliability_score", "source_credibility", "recommendation", "methodology", "created_at"})
	for _, a := range items {
		_ = w.Write([]string{a.ID, a.EvidenceID, fmt.Sprintf("%d", a.RelevanceScore), fmt.Sprintf("%d", a.ReliabilityScore), a.SourceCredibility, a.Recommendation, a.Methodology, a.CreatedAt})
	}
	w.Flush()
	return w.Error()
}

func (s *ExportService) writeVerificationsCSV(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	items, err := s.investigation.ListVerificationsByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export verifications: %w", err)
	}
	fw, err := zw.Create(prefix + "investigation/verifications.csv")
	if err != nil {
		return fmt.Errorf("create verifications.csv: %w", err)
	}
	w := csv.NewWriter(fw)
	_ = w.Write([]string{"id", "evidence_id", "verification_type", "finding", "confidence_level", "methodology", "tools_used", "created_at"})
	for _, v := range items {
		_ = w.Write([]string{v.ID, v.EvidenceID, v.VerificationType, v.Finding, v.ConfidenceLevel, v.Methodology, v.ToolsUsed, v.CreatedAt})
	}
	w.Flush()
	return w.Error()
}

func (s *ExportService) writeCorroborationsCSV(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	items, err := s.investigation.ListCorroborationsByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export corroborations: %w", err)
	}
	fw, err := zw.Create(prefix + "investigation/corroborations.csv")
	if err != nil {
		return fmt.Errorf("create corroborations.csv: %w", err)
	}
	w := csv.NewWriter(fw)
	_ = w.Write([]string{"id", "claim_summary", "claim_type", "strength", "evidence_count", "created_at"})
	for _, c := range items {
		_ = w.Write([]string{c.ID, c.ClaimSummary, c.ClaimType, c.Strength, fmt.Sprintf("%d", c.EvidenceCount), c.CreatedAt})
	}
	w.Flush()
	return w.Error()
}

func (s *ExportService) writeAnalysisNotesCSV(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	items, err := s.investigation.ListAnalysisNotesByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export analysis notes: %w", err)
	}
	fw, err := zw.Create(prefix + "investigation/analysis_notes.csv")
	if err != nil {
		return fmt.Errorf("create analysis_notes.csv: %w", err)
	}
	w := csv.NewWriter(fw)
	_ = w.Write([]string{"id", "title", "analysis_type", "status", "methodology", "created_at"})
	for _, n := range items {
		_ = w.Write([]string{n.ID, n.Title, n.AnalysisType, n.Status, n.Methodology, n.CreatedAt})
	}
	w.Flush()
	return w.Error()
}

func (s *ExportService) writeReportsJSON(ctx context.Context, zw zipCreator, prefix string, caseID uuid.UUID) error {
	reports, err := s.investigation.ListReportsByCase(ctx, caseID)
	if err != nil {
		return fmt.Errorf("export reports: %w", err)
	}
	for _, r := range reports {
		fw, err := zw.Create(prefix + "investigation/reports/" + filepath.Base(r.ID) + ".json")
		if err != nil {
			return fmt.Errorf("create report json: %w", err)
		}
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		if _, err := fw.Write(data); err != nil {
			return fmt.Errorf("write report json: %w", err)
		}
	}
	return nil
}
