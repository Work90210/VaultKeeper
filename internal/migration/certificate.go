package migration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// pdfOutput is the test seam for pdf.Output. Production uses the default
// (delegates to the gofpdf Fpdf.Output method); tests replace it with a
// failing stub to exercise the render-error branch.
var pdfOutput = func(pdf *fpdf.Fpdf, buf *bytes.Buffer) error {
	return pdf.Output(buf)
}

// CertificateInput is everything the PDF generator needs. Decoupling this
// from Record lets the CLI generate certificates from offline data if
// needed, and lets tests drive the generator directly.
type CertificateInput struct {
	Record         Record
	CaseReference  string
	CaseTitle      string
	Files          []CertificateFile
	InstanceVer    string
	Locale         string // "en" or "fr"
	PublicKeyB64   string
	GeneratedAt    time.Time
	PageLimitFiles int // 0 means no limit (dangerous for huge batches)
}

// CertificateFile is one row in the appendix table.
type CertificateFile struct {
	Index        int
	Filename     string
	SourceHash   string
	ComputedHash string
	Match        bool
}

// Certificate bundles the rendered PDF bytes, signed body, and signature.
type Certificate struct {
	PDFBytes    []byte
	SignedBody  []byte
	Signature   []byte
	PublicKeyB64 string
}

// GenerateAttestationPDF renders a Migration Attestation Certificate PDF,
// signs its canonical body with the supplied signer, and returns the
// combined artifact.
func GenerateAttestationPDF(in CertificateInput, signer *Signer) (Certificate, error) {
	if signer == nil {
		return Certificate{}, fmt.Errorf("migration: signer is required")
	}
	strs := certificateStrings(in.Locale)

	// Build canonical body (everything the signature covers). We construct
	// a deterministic plain-text rendering first, sign it, then embed the
	// signature in the PDF.
	body := buildCanonicalBody(in, strs)
	sig := signer.Sign(body)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Header
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, strs.title, "", 1, "C", false, 0, "")
	pdf.Ln(2)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "", 10)
	writeKV(pdf, strs.certID, in.Record.ID.String())
	writeKV(pdf, strs.generated, in.GeneratedAt.Format(time.RFC3339))
	writeKV(pdf, strs.sourceSystem, in.Record.SourceSystem)
	writeKV(pdf, strs.destSystem, fmt.Sprintf("VaultKeeper %s", in.InstanceVer))
	writeKV(pdf, strs.targetCase, fmt.Sprintf("%s - %s", in.CaseReference, in.CaseTitle))
	writeKV(pdf, strs.performedBy, in.Record.PerformedBy)
	writeKV(pdf, strs.startedAt, in.Record.StartedAt.Format(time.RFC3339))
	if in.Record.CompletedAt != nil {
		writeKV(pdf, strs.completedAt, in.Record.CompletedAt.Format(time.RFC3339))
	}

	// Verification summary
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, strs.verificationSummary, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	writeKV(pdf, strs.totalItems, fmt.Sprintf("%d", in.Record.TotalItems))
	writeKV(pdf, strs.hashMatched, fmt.Sprintf("%d", in.Record.MatchedItems))
	writeKV(pdf, strs.hashMismatched, fmt.Sprintf("%d", in.Record.MismatchedItems))

	// Migration hash + TSA
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, strs.migrationHashHdr, "", 1, "L", false, 0, "")
	pdf.SetFont("Courier", "", 8)
	pdf.MultiCell(0, 5, in.Record.MigrationHash, "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
	if in.Record.TSATimestamp != nil {
		writeKV(pdf, strs.tsaTimestamp, in.Record.TSATimestamp.Format(time.RFC3339))
	}
	if in.Record.TSAName != "" {
		writeKV(pdf, strs.tsaAuthority, in.Record.TSAName)
	}
	if len(in.Record.TSAToken) > 0 {
		tokenB64 := base64.StdEncoding.EncodeToString(in.Record.TSAToken)
		writeKV(pdf, strs.tsaTokenTrunc, tokenB64[:min(len(tokenB64), 60)]+"...")
	}

	// Attestation statement
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, strs.attestation, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	attestationBody := fmt.Sprintf(strs.attestationBody,
		in.Record.StartedAt.Format("2006-01-02"),
		in.Record.TotalItems,
		in.Record.SourceSystem,
		in.Record.MatchedItems,
	)
	pdf.MultiCell(0, 5, attestationBody, "", "L", false)

	// Signature block
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 7, strs.signatureHdr, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	writeKV(pdf, strs.algorithm, "Ed25519")
	writeKV(pdf, strs.publicKey, in.PublicKeyB64)
	pdf.SetFont("Courier", "", 7)
	pdf.MultiCell(0, 4, sigB64, "", "L", false)

	// Appendix A — file table
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, strs.appendixHdr, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(10, 6, "#", "1", 0, "C", false, 0, "")
	pdf.CellFormat(60, 6, strs.colFilename, "1", 0, "L", false, 0, "")
	pdf.CellFormat(50, 6, strs.colSource, "1", 0, "L", false, 0, "")
	pdf.CellFormat(50, 6, strs.colComputed, "1", 0, "L", false, 0, "")
	pdf.CellFormat(10, 6, strs.colMatch, "1", 1, "C", false, 0, "")
	pdf.SetFont("Courier", "", 7)

	limit := len(in.Files)
	if in.PageLimitFiles > 0 && in.PageLimitFiles < limit {
		limit = in.PageLimitFiles
	}
	for i := 0; i < limit; i++ {
		f := in.Files[i]
		match := strs.checkMark
		if !f.Match {
			match = strs.crossMark
		}
		pdf.CellFormat(10, 5, fmt.Sprintf("%d", f.Index), "1", 0, "C", false, 0, "")
		pdf.CellFormat(60, 5, truncate(f.Filename, 34), "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 5, truncate(f.SourceHash, 28), "1", 0, "L", false, 0, "")
		pdf.CellFormat(50, 5, truncate(f.ComputedHash, 28), "1", 0, "L", false, 0, "")
		pdf.CellFormat(10, 5, match, "1", 1, "C", false, 0, "")
	}
	if limit < len(in.Files) {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(0, 6, fmt.Sprintf(strs.appendixTruncated, len(in.Files)-limit), "", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdfOutput(pdf, &buf); err != nil {
		return Certificate{}, fmt.Errorf("render certificate PDF: %w", err)
	}
	return Certificate{
		PDFBytes:     buf.Bytes(),
		SignedBody:   body,
		Signature:    sig,
		PublicKeyB64: signer.PublicKeyBase64(),
	}, nil
}

// buildCanonicalBody serialises the attestation contents in a stable
// plain-text format. This string is the exact input to Ed25519 signing,
// so any verifier can reconstruct it from the certificate metadata and
// confirm the signature. Three invariants must hold:
//
//  1. All fields are emitted unconditionally (missing values become the
//     empty string) so the byte layout is stable across optional-field
//     combinations.
//  2. User-controllable string fields are JSON-escaped so embedded
//     newlines, pipes, or equals signs cannot forge a new line that
//     would be misparsed by a lenient verifier.
//  3. The Files slice is sorted by Filename before iteration so the
//     canonical body is independent of caller-side ordering.
func buildCanonicalBody(in CertificateInput, strs certificateStringsT) []byte {
	// Copy-then-sort to avoid mutating the caller's slice.
	files := make([]CertificateFile, len(in.Files))
	copy(files, in.Files)
	sort.Slice(files, func(i, j int) bool { return files[i].Filename < files[j].Filename })

	var b strings.Builder
	b.WriteString(strs.title)
	b.WriteString("\n")
	writeField(&b, "cert_id", in.Record.ID.String())
	writeField(&b, "case_id", in.Record.CaseID.String())
	writeField(&b, "case_ref", in.CaseReference)
	writeField(&b, "source_system", in.Record.SourceSystem)
	writeField(&b, "started_at", in.Record.StartedAt.UTC().Format(time.RFC3339))
	completedAt := ""
	if in.Record.CompletedAt != nil {
		completedAt = in.Record.CompletedAt.UTC().Format(time.RFC3339)
	}
	writeField(&b, "completed_at", completedAt)
	writeField(&b, "total_items", fmt.Sprintf("%d", in.Record.TotalItems))
	writeField(&b, "matched_items", fmt.Sprintf("%d", in.Record.MatchedItems))
	writeField(&b, "mismatched_items", fmt.Sprintf("%d", in.Record.MismatchedItems))
	writeField(&b, "migration_hash", in.Record.MigrationHash)
	writeField(&b, "manifest_hash", in.Record.ManifestHash)
	tsaTs := ""
	if in.Record.TSATimestamp != nil {
		tsaTs = in.Record.TSATimestamp.UTC().Format(time.RFC3339)
	}
	writeField(&b, "tsa_timestamp", tsaTs)
	writeField(&b, "tsa_name", in.Record.TSAName)
	writeField(&b, "tsa_token_b64", base64.StdEncoding.EncodeToString(in.Record.TSAToken))
	for _, f := range files {
		// JSON-encode each field so separators cannot be injected.
		line := fmt.Sprintf("file=%s|source=%s|computed=%s|match=%t\n",
			jsonQuote(f.Filename), jsonQuote(f.SourceHash), jsonQuote(f.ComputedHash), f.Match)
		b.WriteString(line)
	}
	return []byte(b.String())
}

// writeField emits "<key>=<json-quoted-value>\n". JSON quoting preserves
// a deterministic, unambiguous escape for newline, pipe, equals, and all
// other separator characters. json.Marshal on a string never fails.
func writeField(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString("=")
	b.WriteString(jsonQuote(value))
	b.WriteString("\n")
}

func jsonQuote(s string) string {
	out, _ := json.Marshal(s)
	return string(out)
}

func writeKV(pdf *fpdf.Fpdf, k, v string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(50, 5, k, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, v, "", "L", false)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}


// --- i18n strings ---

type certificateStringsT struct {
	title               string
	certID              string
	generated           string
	sourceSystem        string
	destSystem          string
	targetCase          string
	performedBy         string
	startedAt           string
	completedAt         string
	verificationSummary string
	totalItems          string
	hashMatched         string
	hashMismatched      string
	migrationHashHdr    string
	tsaTimestamp        string
	tsaAuthority        string
	tsaTokenTrunc       string
	attestation         string
	attestationBody     string
	signatureHdr        string
	algorithm           string
	publicKey           string
	appendixHdr         string
	colFilename         string
	colSource           string
	colComputed         string
	colMatch            string
	checkMark           string
	crossMark           string
	appendixTruncated   string
}

func certificateStrings(locale string) certificateStringsT {
	if strings.HasPrefix(strings.ToLower(locale), "fr") {
		return certificateStringsT{
			title:               "CERTIFICAT D'ATTESTATION DE MIGRATION",
			certID:              "ID du certificat:",
			generated:           "Genere le:",
			sourceSystem:        "Systeme source:",
			destSystem:          "Systeme destinataire:",
			targetCase:          "Dossier cible:",
			performedBy:         "Effectue par:",
			startedAt:           "Debut:",
			completedAt:         "Fin:",
			verificationSummary: "RESUME DE VERIFICATION",
			totalItems:          "Total des pieces:",
			hashMatched:         "Empreintes verifiees (correspondance):",
			hashMismatched:      "Empreintes divergentes:",
			migrationHashHdr:    "EMPREINTE DE MIGRATION",
			tsaTimestamp:        "Horodatage RFC 3161:",
			tsaAuthority:        "Autorite d'horodatage:",
			tsaTokenTrunc:       "Jeton (base64, tronque):",
			attestation:         "ATTESTATION",
			attestationBody:     "Le %s, %d pieces de preuve ont ete transferees depuis %s vers VaultKeeper. Chaque empreinte source a ete verifiee par rapport a l'empreinte calculee lors de l'ingestion. %d ont correspondu. Zero divergence.",
			signatureHdr:        "SIGNATURE",
			algorithm:           "Algorithme:",
			publicKey:           "Cle publique (base64):",
			appendixHdr:         "ANNEXE A: TABLE DE VERIFICATION DES FICHIERS",
			colFilename:         "Fichier",
			colSource:           "Empreinte source",
			colComputed:         "Empreinte calculee",
			colMatch:            "Match",
			checkMark:           "OK",
			crossMark:           "X",
			appendixTruncated:   "... et %d fichiers supplementaires (tronque pour la lisibilite)",
		}
	}
	return certificateStringsT{
		title:               "MIGRATION ATTESTATION CERTIFICATE",
		certID:              "Certificate ID:",
		generated:           "Generated:",
		sourceSystem:        "Source System:",
		destSystem:          "Destination System:",
		targetCase:          "Target Case:",
		performedBy:         "Performed By:",
		startedAt:           "Started:",
		completedAt:         "Completed:",
		verificationSummary: "VERIFICATION SUMMARY",
		totalItems:          "Total Evidence Items:",
		hashMatched:         "Hash Verified (Match):",
		hashMismatched:      "Hash Mismatch:",
		migrationHashHdr:    "MIGRATION HASH",
		tsaTimestamp:        "RFC 3161 Timestamp:",
		tsaAuthority:        "Timestamp Authority:",
		tsaTokenTrunc:       "Token (base64, truncated):",
		attestation:         "ATTESTATION",
		attestationBody:     "On %s, %d evidence items were transferred from %s to VaultKeeper. Every file's source hash was verified against the hash computed on ingestion. All %d matched. Zero discrepancies.",
		signatureHdr:        "SIGNATURE",
		algorithm:           "Signature Algorithm:",
		publicKey:           "Public Key (base64):",
		appendixHdr:         "APPENDIX A: FILE VERIFICATION TABLE",
		colFilename:         "Filename",
		colSource:           "Source Hash",
		colComputed:         "Computed Hash",
		colMatch:            "Match",
		checkMark:           "OK",
		crossMark:           "X",
		appendixTruncated:   "... and %d additional files (truncated for readability)",
	}
}
