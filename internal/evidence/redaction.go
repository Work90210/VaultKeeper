package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/gen2brain/go-fitz"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
)

// RedactionArea defines a region to redact.
type RedactionArea struct {
	PageNumber int     `json:"page_number"` // 1-based for PDFs, 0 for images
	X          float64 `json:"x"`           // percentage of width (0-100)
	Y          float64 `json:"y"`           // percentage of height (0-100)
	Width      float64 `json:"width"`       // percentage
	Height     float64 `json:"height"`      // percentage
	Reason     string  `json:"reason"`      // why this was redacted
}

// RedactedResult is returned after applying redactions.
type RedactedResult struct {
	NewEvidenceID  uuid.UUID `json:"new_evidence_id"`
	OriginalID     uuid.UUID `json:"original_id"`
	RedactionCount int       `json:"redaction_count"`
	NewHash        string    `json:"new_hash"`
}

// DerivationRecorder records parent–child derivation relationships for
// cross-border federation. Defined here (point of use) so the evidence
// package does not import the federation package.
type DerivationRecorder interface {
	RecordDerivation(ctx context.Context, parentID, childID uuid.UUID, childSHA256 string, parentSHA256 *string, redactionMethod, redactionPurpose string, parameters any) error
}

// RedactionService handles document redaction.
type RedactionService struct {
	evidenceSvc *Service
	storage     ObjectStorage
	tsa         integrity.TimestampAuthority
	custody     CustodyRecorder
	derivation  DerivationRecorder
	logger      *slog.Logger
}

// NewRedactionService creates a new redaction service.
func NewRedactionService(evidenceSvc *Service, storage ObjectStorage, tsa integrity.TimestampAuthority, custody CustodyRecorder, logger *slog.Logger) *RedactionService {
	return &RedactionService{
		evidenceSvc: evidenceSvc,
		storage:     storage,
		tsa:         tsa,
		custody:     custody,
		logger:      logger,
	}
}

// WithDerivationRecorder attaches a DerivationRecorder to the service.
// When set, finalized redactions will create a derivation record for
// federation tracking.
func (rs *RedactionService) WithDerivationRecorder(recorder DerivationRecorder) *RedactionService {
	return &RedactionService{
		evidenceSvc: rs.evidenceSvc,
		storage:     rs.storage,
		tsa:         rs.tsa,
		custody:     rs.custody,
		derivation:  recorder,
		logger:      rs.logger,
	}
}

// maxRedactionSize limits file size for redaction operations (256MB).
const maxRedactionSize = 256 << 20

// Test hooks for otherwise-unreachable error branches. Production code
// reads these directly; unit tests swap them with failing stubs to drive
// 100% line coverage through paths that would otherwise require corrupt
// inputs or filesystem faults.
var (
	redactionReadAll         = io.ReadAll
	redactionImageEncodeJPEG = jpeg.Encode
	redactionImageEncodePNG  = png.Encode
	redactionPDFPageEncode   = jpeg.Encode
	redactionPDFImportImages = pdfcpuapi.ImportImages
	redactionPDFDocImage     = func(doc *fitz.Document, i int) (image.Image, error) {
		return doc.Image(i)
	}
)

// ApplyRedactions creates a redacted copy of an evidence item.
func (rs *RedactionService) ApplyRedactions(ctx context.Context, evidenceID uuid.UUID, redactions []RedactionArea, actorID string) (RedactedResult, error) {
	if err := validateRedactionAreas(redactions); err != nil {
		return RedactedResult{}, err
	}

	// Fetch original evidence
	original, err := rs.evidenceSvc.Get(ctx, evidenceID)
	if err != nil {
		return RedactedResult{}, fmt.Errorf("get original evidence: %w", err)
	}

	if original.DestroyedAt != nil {
		return RedactedResult{}, &ValidationError{Field: "evidence", Message: "cannot redact destroyed evidence"}
	}

	// Download the original file with size limit
	if original.SizeBytes > maxRedactionSize {
		return RedactedResult{}, &ValidationError{Field: "file", Message: "file too large for redaction (max 256MB)"}
	}

	reader, _, _, err := rs.storage.GetObject(ctx, derefStr(original.StorageKey))
	if err != nil {
		return RedactedResult{}, fmt.Errorf("download original file: %w", err)
	}

	data, err := redactionReadAll(io.LimitReader(reader, maxRedactionSize+1))
	reader.Close()
	if err != nil {
		return RedactedResult{}, fmt.Errorf("read original file: %w", err)
	}

	// Apply redactions based on MIME type
	var redactedData []byte
	mimeType := original.MimeType

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		redactedData, err = redactImage(data, mimeType, redactions)
	case mimeType == "application/pdf":
		redactedData, err = redactPDF(data, redactions)
	default:
		return RedactedResult{}, &ValidationError{Field: "mime_type", Message: "redaction only supported for images and PDFs"}
	}

	if err != nil {
		return RedactedResult{}, fmt.Errorf("apply redactions: %w", err)
	}

	// Hash the redacted file
	hashBytes := sha256.Sum256(redactedData)
	hashHex := hex.EncodeToString(hashBytes[:])

	// TSA timestamp
	var tsaToken []byte
	var tsaName string
	tsaStatus := TSAStatusPending

	token, name, _, tsaErr := rs.tsa.IssueTimestamp(ctx, hashBytes[:])
	if tsaErr != nil {
		rs.logger.Warn("TSA timestamp failed for redacted file", "error", tsaErr)
	} else if token != nil {
		tsaToken = token
		tsaName = name
		tsaStatus = TSAStatusStamped
	} else {
		tsaStatus = TSAStatusDisabled
	}

	// Store redacted file
	newID := uuid.New()
	storageKey := StorageObjectKey(original.CaseID, newID, 1, "redacted_"+original.Filename)

	if err := rs.storage.PutObject(ctx, storageKey, bytes.NewReader(redactedData), int64(len(redactedData)), mimeType); err != nil {
		return RedactedResult{}, fmt.Errorf("store redacted file: %w", err)
	}
	storageCleanup := func() { _ = rs.storage.DeleteObject(ctx, storageKey) }

	// Create new evidence record with parent_id pointing to original.
	// Steps 2-4 (Create, UpdateVersionFields, MarkNonCurrent) run inside a
	// transaction so that a partial failure cannot leave an orphaned record
	// without parent linkage.
	repo, ok := rs.evidenceSvc.repo.(*PGRepository)
	if !ok {
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("apply redactions requires PGRepository")
	}

	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("begin redaction tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	evidenceNumber := derefStr(original.EvidenceNumber) + "-R"
	createInput := CreateEvidenceInput{
		CaseID:         original.CaseID,
		EvidenceNumber: evidenceNumber,
		Filename:       "redacted_" + original.Filename,
		OriginalName:   original.OriginalName,
		StorageKey:     storageKey,
		MimeType:       mimeType,
		SizeBytes:      int64(len(redactedData)),
		SHA256Hash:     hashHex,
		Classification: original.Classification,
		Description:    fmt.Sprintf("Redacted copy of %s (%d areas redacted)", derefStr(original.EvidenceNumber), len(redactions)),
		Tags:           append(original.Tags, "redacted"),
		UploadedBy:     actorID,
		UploadedByName: actorID,
		TSAToken:       tsaToken,
		TSAName:        tsaName,
		TSAStatus:      tsaStatus,
	}

	newEvidence, err := repo.CreateWithTx(ctx, tx, createInput)
	if err != nil {
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("create redacted evidence record: %w", err)
	}

	// Set parent_id pointing to original and mark the derivative as non-current.
	// The original remains current (source of truth for legal proceedings).
	// Redacted copies are accessed via disclosures or the version/derivatives list.
	if err := repo.SetDerivativeParentWithTx(ctx, tx, newEvidence.ID, original.ID); err != nil {
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("set derivative parent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("commit redaction tx: %w", err)
	}

	// Custody log with redaction details
	for i, area := range redactions {
		rs.recordCustody(ctx, original.CaseID, newEvidence.ID, "redacted", actorID, map[string]string{
			"original_id":     original.ID.String(),
			"area_index":      fmt.Sprintf("%d", i),
			"page":            fmt.Sprintf("%d", area.PageNumber),
			"reason":          area.Reason,
			"new_hash":        hashHex,
			"redaction_count": fmt.Sprintf("%d", len(redactions)),
		})
	}

	// Record derivation for federation tracking
	rs.recordDerivation(ctx, original.ID, newEvidence.ID, hashHex, original.SHA256Hash, mimeType, "", redactions)

	return RedactedResult{
		NewEvidenceID:  newEvidence.ID,
		OriginalID:     original.ID,
		RedactionCount: len(redactions),
		NewHash:        hashHex,
	}, nil
}

// PreviewRedactions returns a redacted preview without creating a permanent copy.
func (rs *RedactionService) PreviewRedactions(ctx context.Context, evidenceID uuid.UUID, redactions []RedactionArea) (io.ReadCloser, string, error) {
	if err := validateRedactionAreas(redactions); err != nil {
		return nil, "", err
	}

	original, err := rs.evidenceSvc.Get(ctx, evidenceID)
	if err != nil {
		return nil, "", fmt.Errorf("get original evidence: %w", err)
	}

	if original.SizeBytes > maxRedactionSize {
		return nil, "", &ValidationError{Field: "file", Message: "file too large for redaction (max 256MB)"}
	}

	reader, _, _, err := rs.storage.GetObject(ctx, derefStr(original.StorageKey))
	if err != nil {
		return nil, "", fmt.Errorf("download original file: %w", err)
	}

	data, err := redactionReadAll(io.LimitReader(reader, maxRedactionSize+1))
	reader.Close()
	if err != nil {
		return nil, "", fmt.Errorf("read original file: %w", err)
	}

	mimeType := original.MimeType
	var redactedData []byte

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		redactedData, err = redactImage(data, mimeType, redactions)
	case mimeType == "application/pdf":
		redactedData, err = redactPDF(data, redactions)
	default:
		return nil, "", &ValidationError{Field: "mime_type", Message: "redaction only supported for images and PDFs"}
	}

	if err != nil {
		return nil, "", fmt.Errorf("apply redactions for preview: %w", err)
	}

	return io.NopCloser(bytes.NewReader(redactedData)), mimeType, nil
}

// redactImage applies black rectangles to an image.
func redactImage(data []byte, mimeType string, areas []RedactionArea) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	// Create a mutable copy
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	black := color.RGBA{0, 0, 0, 255}

	for _, area := range areas {
		x := int(area.X / 100.0 * imgWidth)
		y := int(area.Y / 100.0 * imgHeight)
		w := int(area.Width / 100.0 * imgWidth)
		h := int(area.Height / 100.0 * imgHeight)

		rect := image.Rect(x+bounds.Min.X, y+bounds.Min.Y, x+w+bounds.Min.X, y+h+bounds.Min.Y)
		draw.Draw(dst, rect, &image.Uniform{black}, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	switch {
	case strings.Contains(mimeType, "png"):
		err = redactionImageEncodePNG(&buf, dst)
	default:
		err = redactionImageEncodeJPEG(&buf, dst, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		return nil, fmt.Errorf("encode redacted image: %w", err)
	}

	return buf.Bytes(), nil
}

// redactPDF performs DESTRUCTIVE redaction on PDF pages.
//
// Approach: rasterize each page to a high-res image, paint black rectangles over
// the redaction areas on the raster, then reconstruct the PDF from the rasterized images.
// This completely destroys the text layer — no copy-paste, no text search, no extraction.
// The output is a pixel-based PDF with no recoverable text content in redacted areas.
func redactPDF(data []byte, areas []RedactionArea) ([]byte, error) {
	// Open PDF with MuPDF via go-fitz for rasterization
	doc, err := fitz.NewFromMemory(data)
	if err != nil {
		return nil, fmt.Errorf("open PDF for rasterization: %w", err)
	}
	defer doc.Close()

	pageCount := doc.NumPage()

	// Group redaction areas by page (1-based)
	areasByPage := make(map[int][]RedactionArea)
	for _, area := range areas {
		page := area.PageNumber
		if page < 1 {
			page = 1
		}
		if page > pageCount {
			continue
		}
		areasByPage[page] = append(areasByPage[page], area)
	}

	// Rasterize each page at 300 DPI, apply redactions, collect as JPEG images
	var pageImages [][]byte
	for i := 0; i < pageCount; i++ {
		pageNum := i + 1

		// Rasterize page to image (300 DPI for quality)
		img, err := redactionPDFDocImage(doc, i)
		if err != nil {
			return nil, fmt.Errorf("rasterize page %d: %w", pageNum, err)
		}

		// Convert to mutable RGBA
		bounds := img.Bounds()
		dst := image.NewRGBA(bounds)
		draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

		// Apply redactions for this page
		if pageAreas, ok := areasByPage[pageNum]; ok {
			imgWidth := float64(bounds.Dx())
			imgHeight := float64(bounds.Dy())
			black := color.RGBA{0, 0, 0, 255}

			for _, area := range pageAreas {
				x := int(area.X / 100.0 * imgWidth)
				y := int(area.Y / 100.0 * imgHeight)
				w := int(area.Width / 100.0 * imgWidth)
				h := int(area.Height / 100.0 * imgHeight)

				rect := image.Rect(
					x+bounds.Min.X, y+bounds.Min.Y,
					x+w+bounds.Min.X, y+h+bounds.Min.Y,
				)
				draw.Draw(dst, rect, &image.Uniform{black}, image.Point{}, draw.Src)
			}
		}

		// Encode page as JPEG
		var pageBuf bytes.Buffer
		if err := redactionPDFPageEncode(&pageBuf, dst, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("encode page %d: %w", pageNum, err)
		}
		pageImages = append(pageImages, pageBuf.Bytes())
	}

	// Reconstruct PDF from rasterized page images using pdfcpu ImportImages
	conf := pdfcpumodel.NewDefaultConfiguration()
	var outBuf bytes.Buffer

	// ImportImages: first reader is a seed PDF (nil = create new), then image readers
	imgReaders := make([]io.Reader, len(pageImages))
	for i, img := range pageImages {
		imgReaders[i] = bytes.NewReader(img)
	}

	if err := redactionPDFImportImages(nil, &outBuf, imgReaders, nil, conf); err != nil {
		return nil, fmt.Errorf("reconstruct PDF from redacted images: %w", err)
	}

	return outBuf.Bytes(), nil
}

const maxRedactionAreas = 500

func validateRedactionAreas(areas []RedactionArea) error {
	if len(areas) == 0 {
		return &ValidationError{Field: "redactions", Message: "at least one redaction area is required"}
	}
	if len(areas) > maxRedactionAreas {
		return &ValidationError{Field: "redactions", Message: fmt.Sprintf("too many redaction areas (max %d)", maxRedactionAreas)}
	}

	for i, area := range areas {
		if area.X < 0 || area.X > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: X must be between 0 and 100", i)}
		}
		if area.Y < 0 || area.Y > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: Y must be between 0 and 100", i)}
		}
		if area.Width <= 0 || area.Width > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: width must be between 0 and 100", i)}
		}
		if area.Height <= 0 || area.Height > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: height must be between 0 and 100", i)}
		}
		if area.X+area.Width > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: X + Width exceeds 100%%", i)}
		}
		if area.Y+area.Height > 100 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: Y + Height exceeds 100%%", i)}
		}
		if strings.TrimSpace(area.Reason) == "" {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: reason is required", i)}
		}
		if len(strings.TrimSpace(area.Reason)) > 2000 {
			return &ValidationError{Field: "redactions", Message: fmt.Sprintf("area %d: reason exceeds maximum length of 2000 characters", i)}
		}
	}

	return nil
}

// FinalizeFromDraft locks a draft, applies redactions, creates a permanent
// derivative evidence item with full redaction metadata, and marks the draft as applied.
func (rs *RedactionService) FinalizeFromDraft(ctx context.Context, input FinalizeInput) (RedactedResult, error) {
	repo, ok := rs.evidenceSvc.repo.(*PGRepository)
	if !ok {
		return RedactedResult{}, fmt.Errorf("finalize requires PGRepository")
	}

	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { // unreachable in tests: requires postgres pool to reject BeginTx
		return RedactedResult{}, fmt.Errorf("begin finalize tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Lock draft row
	draft, yjsState, err := repo.LockDraftForFinalize(ctx, tx, input.DraftID)
	if err != nil {
		return RedactedResult{}, fmt.Errorf("lock draft: %w", err)
	}

	if draft.Status != "draft" {
		return RedactedResult{}, &ValidationError{Field: "draft", Message: "draft has already been " + draft.Status}
	}
	if draft.EvidenceID != input.EvidenceID {
		return RedactedResult{}, &ValidationError{Field: "draft", Message: "draft does not belong to this evidence"}
	}

	// 2. Parse yjs_state → areas
	var state struct {
		Areas []struct {
			ID     string  `json:"id"`
			Page   int     `json:"page"`
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			W      float64 `json:"w"`
			H      float64 `json:"h"`
			Reason string  `json:"reason"`
		} `json:"areas"`
	}
	if len(yjsState) > 0 {
		if err := json.Unmarshal(yjsState, &state); err != nil {
			return RedactedResult{}, fmt.Errorf("unmarshal draft state: %w", err)
		}
	}

	// Convert to RedactionArea
	redactions := make([]RedactionArea, 0, len(state.Areas))
	for _, a := range state.Areas {
		redactions = append(redactions, RedactionArea{
			PageNumber: a.Page,
			X:          a.X,
			Y:          a.Y,
			Width:      a.W,
			Height:     a.H,
			Reason:     a.Reason,
		})
	}

	if err := validateRedactionAreas(redactions); err != nil {
		return RedactedResult{}, err
	}

	// 3. Fetch original evidence
	original, err := rs.evidenceSvc.Get(ctx, input.EvidenceID)
	if err != nil { // unreachable: draft creation requires evidence to exist; concurrent deletion is not supported
		return RedactedResult{}, fmt.Errorf("get original evidence: %w", err)
	}

	if original.DestroyedAt != nil {
		return RedactedResult{}, &ValidationError{Field: "evidence", Message: "cannot redact destroyed evidence"}
	}

	// 4. Download and redact
	if original.SizeBytes > maxRedactionSize {
		return RedactedResult{}, &ValidationError{Field: "file", Message: "file too large for redaction (max 256MB)"}
	}

	reader, _, _, err := rs.storage.GetObject(ctx, derefStr(original.StorageKey))
	if err != nil {
		return RedactedResult{}, fmt.Errorf("download original file: %w", err)
	}

	data, err := redactionReadAll(io.LimitReader(reader, maxRedactionSize+1))
	reader.Close()
	if err != nil {
		return RedactedResult{}, fmt.Errorf("read original file: %w", err)
	}

	var redactedData []byte
	mimeType := original.MimeType

	switch {
	case strings.HasPrefix(mimeType, "image/"):
		redactedData, err = redactImage(data, mimeType, redactions)
	case mimeType == "application/pdf":
		redactedData, err = redactPDF(data, redactions)
	default:
		return RedactedResult{}, &ValidationError{Field: "mime_type", Message: "redaction only supported for images and PDFs"}
	}

	if err != nil {
		return RedactedResult{}, fmt.Errorf("apply redactions: %w", err)
	}

	// 5. Hash + TSA
	hashBytes := sha256.Sum256(redactedData)
	hashHex := hex.EncodeToString(hashBytes[:])

	var tsaToken []byte
	var tsaName string
	tsaStatus := TSAStatusPending

	token, name, _, tsaErr := rs.tsa.IssueTimestamp(ctx, hashBytes[:])
	if tsaErr != nil {
		rs.logger.Warn("TSA timestamp failed for redacted file", "error", tsaErr)
	} else if token != nil {
		tsaToken = token
		tsaName = name
		tsaStatus = TSAStatusStamped
	} else {
		tsaStatus = TSAStatusDisabled
	}

	// 6. Store redacted file
	newID := uuid.New()
	storageKey := StorageObjectKey(original.CaseID, newID, 1, "redacted_"+original.Filename)

	if err := rs.storage.PutObject(ctx, storageKey, bytes.NewReader(redactedData), int64(len(redactedData)), mimeType); err != nil {
		return RedactedResult{}, fmt.Errorf("store redacted file: %w", err)
	}
	// Cleanup on any subsequent failure
	storageCleanup := func() {
		if delErr := rs.storage.DeleteObject(ctx, storageKey); delErr != nil {
			rs.logger.Warn("failed to cleanup redacted file after error", "key", storageKey, "error", delErr)
		}
	}

	// 7. Generate evidence number
	evidenceNumber, err := GenerateRedactionNumber(ctx, repo, derefStr(original.EvidenceNumber), draft.Purpose, draft.Name)
	if err != nil { // unreachable in tests: requires >100 collisions or DB failure inside CheckEvidenceNumberExists
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("generate evidence number: %w", err)
	}

	// 8. Classification — use original's classification unless overridden
	classification := original.Classification
	if input.Classification != "" && validClassifications[input.Classification] {
		classification = input.Classification
	}

	// 9. Description
	description := input.Description
	if description == "" {
		description = fmt.Sprintf("Redacted copy: %s (%d areas)", draft.Name, len(redactions))
	}

	// 10. Prepare redaction metadata
	areaCount := len(redactions)
	actorUUID, err := uuid.Parse(input.ActorID)
	if err != nil {
		storageCleanup()
		return RedactedResult{}, &ValidationError{Field: "actor_id", Message: "invalid actor ID"}
	}
	now := time.Now()
	purpose := draft.Purpose

	// Fallback actor name to actor ID if empty (some SSO tokens omit preferred_username)
	actorName := input.ActorName
	if actorName == "" {
		actorName = input.ActorID
	}

	createInput := CreateEvidenceInput{
		CaseID:         original.CaseID,
		EvidenceNumber: evidenceNumber,
		Filename:       "redacted_" + original.Filename,
		OriginalName:   original.OriginalName,
		StorageKey:     storageKey,
		MimeType:       mimeType,
		SizeBytes:      int64(len(redactedData)),
		SHA256Hash:     hashHex,
		Classification: classification,
		Description:    description,
		Tags:           append(original.Tags, "redacted"),
		UploadedBy:     input.ActorID,
		UploadedByName: actorName,
		TSAToken:       tsaToken,
		TSAName:        tsaName,
		TSAStatus:      tsaStatus,

		RedactionName:        &draft.Name,
		RedactionPurpose:     &purpose,
		RedactionAreaCount:   &areaCount,
		RedactionAuthorID:    &actorUUID,
		RedactionFinalizedAt: &now,
	}

	newEvidence, err := repo.CreateWithTx(ctx, tx, createInput)
	if err != nil { // unreachable in tests: requires mid-transaction DB failure
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("create redacted evidence record: %w", err)
	}

	// Set parent_id — derivative is always non-current
	if err := repo.SetDerivativeParentWithTx(ctx, tx, newEvidence.ID, original.ID); err != nil { // unreachable in tests: mid-transaction DB failure
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("set derivative parent: %w", err)
	}

	// 11. Mark draft as applied
	if err := repo.MarkDraftApplied(ctx, tx, input.DraftID, input.EvidenceID); err != nil { // unreachable in tests: mid-transaction DB failure
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("mark draft applied: %w", err)
	}

	// 12. Commit
	if err := tx.Commit(ctx); err != nil { // unreachable in tests: requires postgres to reject commit
		storageCleanup()
		return RedactedResult{}, fmt.Errorf("commit finalize tx: %w", err)
	}

	// 13. Custody events — use detached context so cancellation of the HTTP request
	// after commit does not prevent audit trail recording.
	custodyCtx := context.WithoutCancel(ctx)
	rs.recordCustody(custodyCtx, original.CaseID, newEvidence.ID, "redaction_finalized", input.ActorID, map[string]string{
		"draft_id":        input.DraftID.String(),
		"original_id":     original.ID.String(),
		"name":            draft.Name,
		"purpose":         string(draft.Purpose),
		"area_count":      fmt.Sprintf("%d", len(redactions)),
		"evidence_number": evidenceNumber,
		"new_hash":        hashHex,
	})

	for i, area := range redactions {
		rs.recordCustody(custodyCtx, original.CaseID, newEvidence.ID, "redaction_area", input.ActorID, map[string]string{
			"area_index": fmt.Sprintf("%d", i),
			"page":       fmt.Sprintf("%d", area.PageNumber),
			"reason":     area.Reason,
		})
	}

	// Record derivation for federation tracking
	rs.recordDerivation(custodyCtx, original.ID, newEvidence.ID, hashHex, original.SHA256Hash, mimeType, string(draft.Purpose), redactions)

	return RedactedResult{
		NewEvidenceID:  newEvidence.ID,
		OriginalID:     original.ID,
		RedactionCount: len(redactions),
		NewHash:        hashHex,
	}, nil
}

func (rs *RedactionService) recordCustody(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorID string, detail map[string]string) {
	if rs.custody == nil {
		return
	}
	if err := rs.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, actorID, detail); err != nil {
		rs.logger.Error("failed to record custody event", "evidence_id", evidenceID, "action", action, "error", err)
	}
}

// redactionMethodForMIME returns the derivation method identifier based on the
// content type of the redacted file.
func redactionMethodForMIME(mimeType string) string {
	if mimeType == "application/pdf" {
		return "pdf-redact-v1"
	}
	return "image-blur-v1"
}

func (rs *RedactionService) recordDerivation(ctx context.Context, parentID, childID uuid.UUID, childSHA256, parentSHA256, mimeType, purpose string, areas []RedactionArea) {
	if rs.derivation == nil {
		return
	}
	var parentHash *string
	if parentSHA256 != "" {
		parentHash = &parentSHA256
	}
	method := redactionMethodForMIME(mimeType)
	params := map[string]any{
		"areas":      areas,
		"area_count": len(areas),
	}
	if err := rs.derivation.RecordDerivation(ctx, parentID, childID, childSHA256, parentHash, method, purpose, params); err != nil {
		rs.logger.Error("failed to record derivation", "parent_id", parentID, "child_id", childID, "error", err)
	}
}
