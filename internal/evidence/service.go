package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// CustodyRecorder logs custody chain events.
type CustodyRecorder interface {
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// CaseLookup retrieves case information needed by the evidence service.
type CaseLookup interface {
	GetLegalHold(ctx context.Context, caseID uuid.UUID) (bool, error)
	GetReferenceCode(ctx context.Context, caseID uuid.UUID) (string, error)
	GetStatus(ctx context.Context, caseID uuid.UUID) (string, error)
}

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// EXIFExtractor extracts EXIF data from an image.
type EXIFExtractor func(reader io.Reader, mimeType string) ([]byte, error)

// Service orchestrates the evidence upload pipeline.
type Service struct {
	repo        Repository
	storage     ObjectStorage
	tsa         integrity.TimestampAuthority
	indexer     search.SearchIndexer
	custody     CustodyRecorder
	cases       CaseLookup
	thumbGen    ThumbnailGenerator
	exifExtract EXIFExtractor
	logger      *slog.Logger
	maxUpload   int64
}

// NewService creates a new evidence service.
func NewService(
	repo Repository,
	storage ObjectStorage,
	tsa integrity.TimestampAuthority,
	indexer search.SearchIndexer,
	custody CustodyRecorder,
	cases CaseLookup,
	thumbGen ThumbnailGenerator,
	logger *slog.Logger,
	maxUploadSize int64,
) *Service {
	return &Service{
		repo:        repo,
		storage:     storage,
		tsa:         tsa,
		indexer:     indexer,
		custody:     custody,
		cases:       cases,
		thumbGen:    thumbGen,
		exifExtract: ExtractEXIF,
		logger:      logger,
		maxUpload:   maxUploadSize,
	}
}

// UploadInput holds the parameters for uploading evidence.
type UploadInput struct {
	CaseID         uuid.UUID
	File           io.Reader
	Filename       string
	SizeBytes      int64
	Classification string
	Description    string
	Tags           []string
	UploadedBy     string
	UploadedByName string
	Source         string
	SourceDate     *time.Time
}

// Upload processes a new evidence file through the complete upload pipeline.
func (s *Service) Upload(ctx context.Context, input UploadInput) (EvidenceItem, error) {
	if err := s.validateUploadInput(input); err != nil {
		return EvidenceItem{}, err
	}

	// Block uploads to closed or archived cases
	caseStatus, err := s.cases.GetStatus(ctx, input.CaseID)
	if err != nil {
		return EvidenceItem{}, fmt.Errorf("check case status: %w", err)
	}
	if caseStatus != "active" {
		return EvidenceItem{}, &ValidationError{Field: "case", Message: fmt.Sprintf("cannot upload evidence to a %s case", caseStatus)}
	}

	sanitizedName := SanitizeFilename(input.Filename)

	// Read file into memory for hashing and MIME detection
	data, err := io.ReadAll(io.LimitReader(input.File, s.maxUpload+1))
	if err != nil {
		return EvidenceItem{}, fmt.Errorf("read upload data: %w", err)
	}
	if int64(len(data)) > s.maxUpload {
		return EvidenceItem{}, &ValidationError{Field: "file", Message: "file exceeds maximum upload size"}
	}

	// SHA-256 hash
	hashBytes := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hashBytes[:])

	// MIME type detection
	mimeType := http.DetectContentType(data)

	// EXIF extraction
	exifData, err := s.exifExtract(bytes.NewReader(data), mimeType)
	if err != nil {
		s.logger.Warn("EXIF extraction failed", "filename", sanitizedName, "error", err)
	}

	// TSA timestamp
	tsaStatus := TSAStatusPending
	var tsaToken []byte
	var tsaName string
	var tsaTimestamp *time.Time

	token, name, ts, err := s.tsa.IssueTimestamp(ctx, hashBytes[:])
	if err != nil {
		s.logger.Warn("TSA timestamp failed, will retry", "filename", sanitizedName, "error", err)
	} else if token != nil {
		tsaToken = token
		tsaName = name
		tsaTimestamp = &ts
		tsaStatus = TSAStatusStamped
	} else {
		// Noop TSA — disabled
		tsaStatus = TSAStatusDisabled
	}

	// Generate evidence number using case reference code
	caseRef, err := s.cases.GetReferenceCode(ctx, input.CaseID)
	if err != nil {
		return EvidenceItem{}, fmt.Errorf("get case reference code: %w", err)
	}
	counter, err := s.repo.IncrementEvidenceCounter(ctx, input.CaseID)
	if err != nil {
		return EvidenceItem{}, fmt.Errorf("generate evidence number: %w", err)
	}
	evidenceNumber := fmt.Sprintf("%s-%05d", caseRef, counter)

	// Generate a pre-allocated ID for the storage key
	evidenceID := uuid.New()
	storageKey := StorageObjectKey(input.CaseID, evidenceID, 1, sanitizedName)

	// Upload to MinIO
	if err := s.storage.PutObject(ctx, storageKey, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
		return EvidenceItem{}, fmt.Errorf("store evidence file: %w", err)
	}

	// Create DB record
	createInput := CreateEvidenceInput{
		CaseID:         input.CaseID,
		EvidenceNumber: evidenceNumber,
		Filename:       sanitizedName,
		OriginalName:   input.Filename,
		StorageKey:     storageKey,
		MimeType:       mimeType,
		SizeBytes:      int64(len(data)),
		SHA256Hash:     hashHex,
		Classification: input.Classification,
		Description:    html.EscapeString(strings.TrimSpace(input.Description)),
		Tags:           nonNilTags(input.Tags),
		UploadedBy:     input.UploadedBy,
		UploadedByName: input.UploadedByName,
		Source:         input.Source,
		SourceDate:     input.SourceDate,
		TSAToken:       tsaToken,
		TSAName:        tsaName,
		TSATimestamp:   tsaTimestamp,
		TSAStatus:      tsaStatus,
		ExifData:       exifData,
	}

	evidence, err := s.repo.Create(ctx, createInput)
	if err != nil {
		// Cleanup stored file on DB failure
		if delErr := s.storage.DeleteObject(ctx, storageKey); delErr != nil {
			s.logger.Error("failed to cleanup stored file after DB error",
				"key", storageKey, "error", delErr)
		}
		return EvidenceItem{}, fmt.Errorf("create evidence record: %w", err)
	}

	// Generate thumbnail in background
	go s.generateThumbnail(context.Background(), evidence.ID, input.CaseID, evidence.Version, sanitizedName, mimeType, data)

	// Custody log
	s.recordCustodyEvent(ctx, input.CaseID, evidence.ID, "evidence_uploaded", input.UploadedBy, map[string]string{
		"evidence_number": evidenceNumber,
		"filename":        sanitizedName,
		"sha256":          hashHex,
		"size_bytes":      fmt.Sprintf("%d", len(data)),
		"mime_type":       mimeType,
	})

	// Index in Meilisearch
	s.indexEvidence(ctx, evidence)

	return evidence, nil
}

// Get retrieves evidence metadata by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	return s.repo.FindByID(ctx, id)
}

// List retrieves evidence for a case with filtering and pagination.
func (s *Service) List(ctx context.Context, filter EvidenceFilter, page Pagination) (PaginatedResult[EvidenceItem], error) {
	items, total, err := s.repo.FindByCase(ctx, filter, page)
	if err != nil {
		return PaginatedResult[EvidenceItem]{}, err
	}

	page = ClampPagination(page)
	hasMore := len(items) == page.Limit && total > page.Limit

	var nextCursor string
	if hasMore && len(items) > 0 {
		nextCursor = encodeCursor(items[len(items)-1].ID)
	}

	return PaginatedResult[EvidenceItem]{
		Items:      items,
		TotalCount: total,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateMetadata updates evidence description, classification, or tags.
func (s *Service) UpdateMetadata(ctx context.Context, id uuid.UUID, updates EvidenceUpdate, actorID string) (EvidenceItem, error) {
	if err := validateEvidenceUpdate(updates); err != nil {
		return EvidenceItem{}, err
	}

	// Sanitize
	if updates.Description != nil {
		sanitized := html.EscapeString(strings.TrimSpace(*updates.Description))
		updates.Description = &sanitized
	}

	result, err := s.repo.Update(ctx, id, updates)
	if err != nil {
		return EvidenceItem{}, err
	}

	changed := make(map[string]string)
	if updates.Description != nil {
		changed["description"] = "updated"
	}
	if updates.Classification != nil {
		changed["classification"] = *updates.Classification
	}
	if updates.Tags != nil {
		tagsJSON, _ := json.Marshal(updates.Tags)
		changed["tags"] = string(tagsJSON)
	}
	s.recordCustodyEvent(ctx, result.CaseID, result.ID, "metadata_updated", actorID, changed)

	// Re-index
	s.indexEvidence(ctx, result)

	return result, nil
}

// Download streams the evidence file from storage.
func (s *Service) Download(ctx context.Context, id uuid.UUID, actorID string) (io.ReadCloser, int64, string, string, error) {
	evidence, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, 0, "", "", err
	}

	if evidence.DestroyedAt != nil {
		return nil, 0, "", "", &ValidationError{Field: "evidence", Message: "evidence has been destroyed"}
	}

	reader, size, contentType, err := s.storage.GetObject(ctx, derefStr(evidence.StorageKey))
	if err != nil {
		return nil, 0, "", "", fmt.Errorf("download evidence file: %w", err)
	}

	s.recordCustodyEvent(ctx, evidence.CaseID, evidence.ID, "evidence_downloaded", actorID, map[string]string{
		"filename": evidence.Filename,
	})

	return reader, size, contentType, evidence.Filename, nil
}

// GetThumbnail streams the thumbnail from storage.
func (s *Service) GetThumbnail(ctx context.Context, id uuid.UUID) (io.ReadCloser, int64, error) {
	evidence, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, 0, err
	}

	if evidence.ThumbnailKey == nil || *evidence.ThumbnailKey == "" {
		return nil, 0, &ValidationError{Field: "thumbnail", Message: "no thumbnail available"}
	}

	reader, size, _, err := s.storage.GetObject(ctx, *evidence.ThumbnailKey)
	if err != nil {
		return nil, 0, fmt.Errorf("get thumbnail: %w", err)
	}

	return reader, size, nil
}

// Destroy marks evidence as destroyed (soft delete) with audit trail.
func (s *Service) Destroy(ctx context.Context, input DestroyInput) error {
	evidence, err := s.repo.FindByID(ctx, input.EvidenceID)
	if err != nil {
		return err
	}

	if evidence.DestroyedAt != nil {
		return nil // idempotent
	}

	// Block destruction on archived cases
	caseStatus, err := s.cases.GetStatus(ctx, evidence.CaseID)
	if err != nil {
		return fmt.Errorf("check case status: %w", err)
	}
	if caseStatus == "archived" {
		return &ValidationError{Field: "case", Message: "cannot destroy evidence in an archived case"}
	}

	// Check legal hold
	held, err := s.cases.GetLegalHold(ctx, evidence.CaseID)
	if err != nil {
		return fmt.Errorf("check legal hold: %w", err)
	}
	if held {
		return &ValidationError{Field: "legal_hold", Message: "cannot destroy evidence under legal hold"}
	}

	if strings.TrimSpace(input.Reason) == "" {
		return &ValidationError{Field: "reason", Message: "destruction reason is required"}
	}

	// Mark as destroyed in DB first (reversible) before deleting files
	if err := s.repo.MarkDestroyed(ctx, input.EvidenceID, input.Reason, input.ActorID); err != nil {
		return err
	}

	// Delete from storage; if this fails the file is orphaned but the record is correct
	if err := s.storage.DeleteObject(ctx, derefStr(evidence.StorageKey)); err != nil {
		s.logger.Warn("failed to delete evidence file from storage (orphaned)",
			"key", derefStr(evidence.StorageKey), "error", err)
	}

	// Delete thumbnail if exists
	if evidence.ThumbnailKey != nil && *evidence.ThumbnailKey != "" {
		if err := s.storage.DeleteObject(ctx, *evidence.ThumbnailKey); err != nil {
			s.logger.Warn("failed to delete thumbnail", "key", *evidence.ThumbnailKey, "error", err)
		}
	}

	s.recordCustodyEvent(ctx, evidence.CaseID, evidence.ID, "evidence_destroyed", input.ActorID, map[string]string{
		"reason":   input.Reason,
		"filename": evidence.Filename,
		"sha256":   evidence.SHA256Hash,
	})

	return nil
}

// UploadNewVersion creates a new version of existing evidence.
func (s *Service) UploadNewVersion(ctx context.Context, parentID uuid.UUID, input UploadInput) (EvidenceItem, error) {
	parent, err := s.repo.FindByID(ctx, parentID)
	if err != nil {
		return EvidenceItem{}, err
	}

	if parent.DestroyedAt != nil {
		return EvidenceItem{}, &ValidationError{Field: "evidence", Message: "cannot version destroyed evidence"}
	}

	// Upload with the same case
	input.CaseID = parent.CaseID

	evidence, err := s.Upload(ctx, input)
	if err != nil {
		return EvidenceItem{}, err
	}

	// Mark all previous versions as non-current
	rootID := parent.ID
	if parent.ParentID != nil {
		rootID = *parent.ParentID
	}
	if err := s.repo.MarkPreviousVersions(ctx, rootID); err != nil {
		return EvidenceItem{}, fmt.Errorf("mark previous versions: %w", err)
	}

	// Update the new evidence to point to the root parent and correct version
	newVersion := parent.Version + 1
	if err := s.setVersionFields(ctx, evidence.ID, rootID, newVersion); err != nil {
		return EvidenceItem{}, err
	}

	s.recordCustodyEvent(ctx, parent.CaseID, evidence.ID, "new_version_uploaded", input.UploadedBy, map[string]string{
		"parent_id":        parent.ID.String(),
		"previous_version": fmt.Sprintf("%d", parent.Version),
		"new_version":      fmt.Sprintf("%d", newVersion),
	})
	return s.repo.FindByID(ctx, evidence.ID)
}

// GetVersionHistory returns all versions of an evidence item.
func (s *Service) GetVersionHistory(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceItem, error) {
	return s.repo.FindVersionHistory(ctx, evidenceID)
}

func (s *Service) setVersionFields(ctx context.Context, id, parentID uuid.UUID, version int) error {
	if err := s.repo.UpdateVersionFields(ctx, id, parentID, version); err != nil {
		return fmt.Errorf("set version fields: %w", err)
	}
	return nil
}

func (s *Service) validateUploadInput(input UploadInput) error {
	if input.CaseID == uuid.Nil {
		return &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if strings.TrimSpace(input.Filename) == "" {
		return &ValidationError{Field: "filename", Message: "filename is required"}
	}
	if input.File == nil {
		return &ValidationError{Field: "file", Message: "file is required"}
	}
	if input.Classification == "" {
		input.Classification = ClassificationRestricted
	}
	if !ValidClassifications[input.Classification] {
		return &ValidationError{Field: "classification", Message: "invalid classification"}
	}
	if len(input.Description) > MaxDescriptionLength {
		return &ValidationError{Field: "description", Message: "description too long"}
	}
	if len(input.Tags) > MaxTagCount {
		return &ValidationError{Field: "tags", Message: "too many tags"}
	}
	for _, tag := range input.Tags {
		if len(tag) > MaxTagLength {
			return &ValidationError{Field: "tags", Message: "tag too long"}
		}
	}
	return nil
}

func validateEvidenceUpdate(updates EvidenceUpdate) error {
	if updates.Description != nil && len(*updates.Description) > MaxDescriptionLength {
		return &ValidationError{Field: "description", Message: "description too long"}
	}
	if updates.Classification != nil && !ValidClassifications[*updates.Classification] {
		return &ValidationError{Field: "classification", Message: "invalid classification"}
	}
	if updates.Tags != nil {
		if len(updates.Tags) > MaxTagCount {
			return &ValidationError{Field: "tags", Message: "too many tags"}
		}
		for _, tag := range updates.Tags {
			if len(tag) > MaxTagLength {
				return &ValidationError{Field: "tags", Message: "tag too long"}
			}
		}
	}
	return nil
}

func (s *Service) generateThumbnail(ctx context.Context, evidenceID, caseID uuid.UUID, version int, filename, mimeType string, data []byte) {
	thumbData, err := s.thumbGen.Generate(bytes.NewReader(data), mimeType)
	if err != nil {
		s.logger.Warn("thumbnail generation failed", "evidence_id", evidenceID, "error", err)
		return
	}
	if thumbData == nil {
		return // unsupported format
	}

	thumbKey := fmt.Sprintf("thumbnails/%s/%s/thumb.jpg", caseID, evidenceID)
	if err := s.storage.PutObject(ctx, thumbKey, bytes.NewReader(thumbData), int64(len(thumbData)), "image/jpeg"); err != nil {
		s.logger.Error("failed to store thumbnail", "evidence_id", evidenceID, "error", err)
		return
	}

	if err := s.repo.UpdateThumbnailKey(ctx, evidenceID, thumbKey); err != nil {
		s.logger.Error("failed to update thumbnail key", "evidence_id", evidenceID, "error", err)
	}
}

func (s *Service) recordCustodyEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	if err := s.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record custody event",
			"case_id", caseID, "evidence_id", evidenceID, "action", action, "error", err)
	}
}

func (s *Service) indexEvidence(ctx context.Context, evidence EvidenceItem) {
	if s.indexer == nil {
		return
	}
	doc := search.Document{
		ID:    evidence.ID.String(),
		Index: "evidence",
		Payload: map[string]any{
			"case_id":         evidence.CaseID.String(),
			"evidence_number": derefStr(evidence.EvidenceNumber),
			"filename":        evidence.Filename,
			"original_name":   evidence.OriginalName,
			"mime_type":       evidence.MimeType,
			"classification":  evidence.Classification,
			"description":     evidence.Description,
			"tags":            evidence.Tags,
			"uploaded_by":     evidence.UploadedBy,
			"sha256_hash":     evidence.SHA256Hash,
			"created_at":      evidence.CreatedAt.Format(time.RFC3339),
		},
	}
	if err := s.indexer.IndexDocument(ctx, doc); err != nil {
		s.logger.Error("failed to index evidence", "id", evidence.ID, "error", err)
	}
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

// derefStr safely dereferences a *string, returning "" for nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
