package reports

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// CustodyReportSource provides custody events for report generation.
type CustodyReportSource interface {
	ListAllByCase(ctx context.Context, caseID uuid.UUID) ([]custody.Event, error)
	ListByEvidence(ctx context.Context, evidenceID uuid.UUID, limit int, cursor string) ([]custody.Event, int, error)
}

// EvidenceReportSource provides evidence data for report generation.
type EvidenceReportSource interface {
	FindByCase(ctx context.Context, filter evidence.EvidenceFilter, page evidence.Pagination) ([]evidence.EvidenceItem, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (evidence.EvidenceItem, error)
}

// CaseRecord is a minimal case representation used by reports.
// It avoids importing the cases package to prevent circular dependencies.
type CaseRecord struct {
	ID            uuid.UUID
	ReferenceCode string
	Title         string
	Jurisdiction  string
	Status        string
}

// CaseReportSource provides case data for report generation.
type CaseReportSource interface {
	FindByID(ctx context.Context, id uuid.UUID) (CaseRecord, error)
}

// CustodyReportGenerator creates PDF reports for custody chains.
type CustodyReportGenerator struct {
	custodyRepo  CustodyReportSource
	evidenceRepo EvidenceReportSource
	caseRepo     CaseReportSource
}

// NewCustodyReportGenerator creates a new report generator.
func NewCustodyReportGenerator(
	custodyRepo CustodyReportSource,
	evidenceRepo EvidenceReportSource,
	caseRepo CaseReportSource,
) *CustodyReportGenerator {
	return &CustodyReportGenerator{
		custodyRepo:  custodyRepo,
		evidenceRepo: evidenceRepo,
		caseRepo:     caseRepo,
	}
}

// pdfOutput writes the generated PDF to the buffer. It is a package-level
// var so tests can inject failures for the otherwise-infallible pdf.Output.
var pdfOutput = func(pdf *fpdf.Fpdf, buf *bytes.Buffer) error {
	return pdf.Output(buf)
}

const (
	pageMargin    = 15.0
	headerHeight  = 8.0
	rowHeight     = 6.0
	fontSizeTitle = 14.0
	fontSizeH2    = 11.0
	fontSizeBody  = 9.0
	fontSizeSmall = 7.5
)

// GenerateCustodyPDF creates a full custody chain report for a case.
func (g *CustodyReportGenerator) GenerateCustodyPDF(ctx context.Context, caseID uuid.UUID) ([]byte, error) {
	caseData, err := g.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("load case for custody report: %w", err)
	}

	items, _, err := g.evidenceRepo.FindByCase(ctx, evidence.EvidenceFilter{
		CaseID:      caseID,
		CurrentOnly: true,
	}, evidence.Pagination{Limit: evidence.MaxPageLimit})
	if err != nil {
		return nil, fmt.Errorf("load evidence for custody report: %w", err)
	}

	events, err := g.custodyRepo.ListAllByCase(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("load custody events for report: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, pageMargin)

	pdf.AddPage()
	writeReportHeader(pdf, caseData)
	writeEvidenceSummary(pdf, items)
	writeCustodyTable(pdf, events)
	writeHashChainStatus(pdf, events)
	writeFooter(pdf)

	var buf bytes.Buffer
	if err := pdfOutput(pdf, &buf); err != nil {
		return nil, fmt.Errorf("render custody PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// GetReferenceCode returns the reference code for the given case.
func (g *CustodyReportGenerator) GetReferenceCode(ctx context.Context, caseID uuid.UUID) (string, error) {
	c, err := g.caseRepo.FindByID(ctx, caseID)
	if err != nil {
		return "", fmt.Errorf("load case for reference code: %w", err)
	}
	return c.ReferenceCode, nil
}

// GenerateEvidenceCustodyPDF creates a custody report for a single evidence item.
func (g *CustodyReportGenerator) GenerateEvidenceCustodyPDF(ctx context.Context, evidenceID uuid.UUID) ([]byte, error) {
	item, err := g.evidenceRepo.FindByID(ctx, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("load evidence item for report: %w", err)
	}

	caseData, err := g.caseRepo.FindByID(ctx, item.CaseID)
	if err != nil {
		return nil, fmt.Errorf("load case for evidence report: %w", err)
	}

	events, _, err := g.custodyRepo.ListByEvidence(ctx, evidenceID, 10000, "")
	if err != nil {
		return nil, fmt.Errorf("load custody events for evidence report: %w", err)
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, pageMargin)

	pdf.AddPage()
	writeReportHeader(pdf, caseData)
	writeSingleEvidenceDetail(pdf, item)
	writeCustodyTable(pdf, events)
	writeFooter(pdf)

	var buf bytes.Buffer
	if err := pdfOutput(pdf, &buf); err != nil {
		return nil, fmt.Errorf("render evidence custody PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func writeReportHeader(pdf *fpdf.Fpdf, c CaseRecord) {
	pdf.SetFont("Helvetica", "B", fontSizeTitle)
	pdf.CellFormat(0, 10, "Chain of Custody Report", "", 1, "C", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", fontSizeBody)
	pdf.CellFormat(45, rowHeight, "Case Reference:", "", 0, "", false, 0, "")
	pdf.SetFont("Helvetica", "B", fontSizeBody)
	pdf.CellFormat(0, rowHeight, c.ReferenceCode, "", 1, "", false, 0, "")

	pdf.SetFont("Helvetica", "", fontSizeBody)
	pdf.CellFormat(45, rowHeight, "Case Title:", "", 0, "", false, 0, "")
	pdf.CellFormat(0, rowHeight, c.Title, "", 1, "", false, 0, "")

	pdf.CellFormat(45, rowHeight, "Jurisdiction:", "", 0, "", false, 0, "")
	pdf.CellFormat(0, rowHeight, c.Jurisdiction, "", 1, "", false, 0, "")

	pdf.CellFormat(45, rowHeight, "Report Generated:", "", 0, "", false, 0, "")
	pdf.CellFormat(0, rowHeight, time.Now().UTC().Format(time.RFC3339), "", 1, "", false, 0, "")

	pdf.Ln(4)
	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(pageMargin, pdf.GetY(), 210-pageMargin, pdf.GetY())
	pdf.Ln(4)
}

func writeEvidenceSummary(pdf *fpdf.Fpdf, items []evidence.EvidenceItem) {
	pdf.SetFont("Helvetica", "B", fontSizeH2)
	pdf.CellFormat(0, headerHeight, "Evidence Items", "", 1, "", false, 0, "")
	pdf.Ln(2)

	colWidths := []float64{25, 50, 25, 35, 45}
	headers := []string{"Number", "Filename", "TSA Status", "Upload Date", "SHA-256 (first 16 chars)"}

	pdf.SetFont("Helvetica", "B", fontSizeSmall)
	pdf.SetFillColor(230, 230, 230)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], headerHeight, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", fontSizeSmall)
	pdf.SetFillColor(245, 245, 245)
	for idx, item := range items {
		fill := idx%2 == 1
		evidenceNum := ""
		if item.EvidenceNumber != nil {
			evidenceNum = *item.EvidenceNumber
		}
		tsaTimestamp := ""
		if item.TSATimestamp != nil {
			tsaTimestamp = item.TSATimestamp.Format("2006-01-02 15:04")
		}
		hashPrefix := item.SHA256Hash
		if len(hashPrefix) > 16 {
			hashPrefix = hashPrefix[:16]
		}

		pdf.CellFormat(colWidths[0], rowHeight, evidenceNum, "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[1], rowHeight, truncate(item.Filename, 30), "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[2], rowHeight, item.TSAStatus, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[3], rowHeight, tsaTimestamp, "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[4], rowHeight, hashPrefix, "1", 0, "", fill, 0, "")
		pdf.Ln(-1)
	}

	pdf.Ln(4)
}

func writeSingleEvidenceDetail(pdf *fpdf.Fpdf, item evidence.EvidenceItem) {
	pdf.SetFont("Helvetica", "B", fontSizeH2)
	pdf.CellFormat(0, headerHeight, "Evidence Item Detail", "", 1, "", false, 0, "")
	pdf.Ln(2)

	evidenceNum := ""
	if item.EvidenceNumber != nil {
		evidenceNum = *item.EvidenceNumber
	}
	tsaTimestamp := ""
	if item.TSATimestamp != nil {
		tsaTimestamp = item.TSATimestamp.Format(time.RFC3339)
	}

	fields := []struct {
		label, value string
	}{
		{"Evidence Number", evidenceNum},
		{"Filename", item.Filename},
		{"Original Name", item.OriginalName},
		{"MIME Type", item.MimeType},
		{"Size (bytes)", fmt.Sprintf("%d", item.SizeBytes)},
		{"Classification", item.Classification},
		{"SHA-256 Hash", item.SHA256Hash},
		{"TSA Status", item.TSAStatus},
		{"TSA Timestamp", tsaTimestamp},
		{"Uploaded By", item.UploadedBy},
		{"Created At", item.CreatedAt.Format(time.RFC3339)},
	}

	pdf.SetFont("Helvetica", "", fontSizeBody)
	for _, f := range fields {
		pdf.SetFont("Helvetica", "B", fontSizeBody)
		pdf.CellFormat(45, rowHeight, f.label+":", "", 0, "", false, 0, "")
		pdf.SetFont("Helvetica", "", fontSizeBody)
		pdf.CellFormat(0, rowHeight, f.value, "", 1, "", false, 0, "")
	}

	pdf.Ln(4)
	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(pageMargin, pdf.GetY(), 210-pageMargin, pdf.GetY())
	pdf.Ln(4)
}

func writeCustodyTable(pdf *fpdf.Fpdf, events []custody.Event) {
	pdf.SetFont("Helvetica", "B", fontSizeH2)
	pdf.CellFormat(0, headerHeight, "Custody Chain", "", 1, "", false, 0, "")
	pdf.Ln(2)

	colWidths := []float64{35, 25, 30, 30, 60}
	headers := []string{"Timestamp", "Action", "Actor", "Hash (first 12)", "Detail"}

	pdf.SetFont("Helvetica", "B", fontSizeSmall)
	pdf.SetFillColor(230, 230, 230)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], headerHeight, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", fontSizeSmall)
	pdf.SetFillColor(245, 245, 245)
	for idx, e := range events {
		fill := idx%2 == 1
		hashPrefix := e.HashValue
		if len(hashPrefix) > 12 {
			hashPrefix = hashPrefix[:12]
		}

		pdf.CellFormat(colWidths[0], rowHeight, e.Timestamp.Format("2006-01-02 15:04:05"), "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[1], rowHeight, truncate(e.Action, 15), "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[2], rowHeight, truncate(e.ActorUserID, 18), "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[3], rowHeight, hashPrefix, "1", 0, "", fill, 0, "")
		pdf.CellFormat(colWidths[4], rowHeight, truncate(e.Detail, 38), "1", 0, "", fill, 0, "")
		pdf.Ln(-1)
	}

	pdf.Ln(4)
}

func writeHashChainStatus(pdf *fpdf.Fpdf, events []custody.Event) {
	pdf.SetFont("Helvetica", "B", fontSizeH2)
	pdf.CellFormat(0, headerHeight, "Hash Chain Verification", "", 1, "", false, 0, "")
	pdf.Ln(2)

	verified := true
	breakCount := 0
	for i := 1; i < len(events); i++ {
		expected := custody.ComputeLogHash(events[i-1].HashValue, events[i])
		if expected != events[i].HashValue {
			verified = false
			breakCount++
		}
	}

	pdf.SetFont("Helvetica", "", fontSizeBody)
	pdf.CellFormat(45, rowHeight, "Total Entries:", "", 0, "", false, 0, "")
	pdf.CellFormat(0, rowHeight, fmt.Sprintf("%d", len(events)), "", 1, "", false, 0, "")

	pdf.CellFormat(45, rowHeight, "Chain Status:", "", 0, "", false, 0, "")
	if verified {
		pdf.SetTextColor(0, 128, 0)
		pdf.CellFormat(0, rowHeight, "VERIFIED - Chain is intact", "", 1, "", false, 0, "")
	} else {
		pdf.SetTextColor(200, 0, 0)
		pdf.CellFormat(0, rowHeight, fmt.Sprintf("BROKEN - %d break(s) detected", breakCount), "", 1, "", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)

	pdf.Ln(4)
}

func writeFooter(pdf *fpdf.Fpdf) {
	pdf.SetDrawColor(180, 180, 180)
	pdf.Line(pageMargin, pdf.GetY(), 210-pageMargin, pdf.GetY())
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "I", fontSizeSmall)
	pdf.SetTextColor(128, 128, 128)
	pdf.CellFormat(0, rowHeight,
		fmt.Sprintf("Generated by VaultKeeper at %s", time.Now().UTC().Format(time.RFC3339)),
		"", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
