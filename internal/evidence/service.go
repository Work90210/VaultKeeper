package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
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

// LegalHoldChecker is the narrow interface used by DestroyEvidence to consult
// the legal-hold state of a case without dragging in the full cases.Service
// surface. Production wiring should inject an adapter around cases.Service
// via WithLegalHoldChecker (TODO: wire in cmd/vaultkeeper-api/main.go).
type LegalHoldChecker interface {
	EnsureNotOnHold(ctx context.Context, caseID uuid.UUID) error
}

// Service orchestrates the evidence upload pipeline.
type Service struct {
	repo              Repository
	storage           ObjectStorage
	tsa               integrity.TimestampAuthority
	indexer           search.SearchIndexer
	custody           CustodyRecorder
	cases             CaseLookup
	thumbGen          ThumbnailGenerator
	exifExtract       EXIFExtractor
	logger            *slog.Logger
	maxUpload         int64
	legalHoldChecker  LegalHoldChecker            // optional — set via WithLegalHoldChecker
	retentionNotifier RetentionNotifier            // optional — set via WithRetentionNotifier
	erasureRepo       ErasureRepository            // optional — set via WithErasureRepo
	attempts          UploadAttemptRepository       // optional — Sprint 11.5 upload attempt tracking
	outboxPool        execer                        // optional — for notification_outbox inserts
	captureMetadata   CaptureMetadataRepository     // optional — Berkeley Protocol capture metadata
}

// scopedEvidenceRepo is the narrow interface implemented by PGRepository
// for defense-in-depth IDOR prevention. It is NOT part of the Repository
// interface so existing test mocks (which satisfy Repository) remain
// compilable. Production always uses *PGRepository which implements both.
//
// The three methods serve distinct access patterns:
//   - FindByIDActive: active-only fetch scoped to the caller's evidence ID;
//     used when the caseID is not yet known (e.g. /api/evidence/{id} route).
//     The handler's enforceItemAccess provides the case-membership gate.
//   - FindByIDScoped: full case+id-scoped fetch used when the caseID is
//     already available (e.g. UploadNewVersion, where parent.CaseID is known).
//     Provides DB-level cross-case IDOR prevention.
//   - FindByIDIncludingDestroyed: unfiltered fetch for GDPR / destruction
//     flows that must operate on already-destroyed records.
type scopedEvidenceRepo interface {
	FindByIDActive(ctx context.Context, id uuid.UUID) (EvidenceItem, error)
	FindByIDScoped(ctx context.Context, caseID, id uuid.UUID) (EvidenceItem, error)
	FindByIDIncludingDestroyed(ctx context.Context, id uuid.UUID) (EvidenceItem, error)
}

// getScoped returns the scoped repository implementation when the underlying
// repo supports it (i.e. *PGRepository in production). Returns nil when the
// repo is a test mock that only satisfies the plain Repository interface.
func (s *Service) getScoped() scopedEvidenceRepo {
	sr, _ := s.repo.(scopedEvidenceRepo)
	return sr
}

// findByIDWithCaseScope fetches an active (non-destroyed) evidence item.
// In production it issues a query filtered by `destroyed_at IS NULL`,
// preventing service-layer access to destroyed records without needing
// the caseID. Case-membership enforcement is the handler's responsibility
// via enforceItemAccess. Falls back to the unscoped Repository.FindByID
// for test mocks that do not implement scopedEvidenceRepo.
func (s *Service) findByIDWithCaseScope(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	sr := s.getScoped()
	if sr == nil {
		// Test mock path — return as-is (mocks handle their own scoping).
		return s.repo.FindByID(ctx, id)
	}
	return sr.FindByIDActive(ctx, id)
}

// WithLegalHoldChecker injects a legal-hold checker into the service.
// Returns the service for chaining. Used by DestroyEvidence; if nil, the
// legal-hold check is skipped (production must wire this).
func (s *Service) WithLegalHoldChecker(checker LegalHoldChecker) *Service {
	s.legalHoldChecker = checker
	return s
}

// WithUploadAttemptRepository injects the upload attempt tracker (Sprint 11.5).
func (s *Service) WithUploadAttemptRepository(repo UploadAttemptRepository) *Service {
	s.attempts = repo
	return s
}

// WithOutboxPool injects the DB pool for notification_outbox writes (Sprint 11.5).
func (s *Service) WithOutboxPool(pool execer) *Service {
	s.outboxPool = pool
	return s
}

// WithCaptureMetadataRepository injects the capture metadata repository (Berkeley Protocol).
func (s *Service) WithCaptureMetadataRepository(repo CaptureMetadataRepository) *Service {
	s.captureMetadata = repo
	return s
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
	ExpectedSHA256 string    // client-declared SHA-256 hex (Sprint 11.5)
	AttemptID      uuid.UUID // upload_attempts_v1 row ID (Sprint 11.5)
}

// Upload processes a new evidence file through the complete upload pipeline.
func (s *Service) Upload(ctx context.Context, input UploadInput) (EvidenceItem, error) {
	// Sprint 9: normalize tags through the full Sprint 9 rules (regex,
	// lowercase, dedupe, max count) before validation. Previously the
	// inline length check was a backdoor that accepted uppercase and
	// whitespace-containing tags.
	if len(input.Tags) > 0 {
		normalized, err := NormalizeTags(input.Tags)
		if err != nil {
			return EvidenceItem{}, err
		}
		input.Tags = normalized
	}

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

	// Sprint 11.5: verify client-declared hash matches server-computed hash.
	// Checked BEFORE MinIO write so no orphan objects on mismatch.
	// Uses constant-time comparison to prevent timing side-channel attacks.
	if input.ExpectedSHA256 != "" && subtle.ConstantTimeCompare([]byte(hashHex), []byte(strings.ToLower(input.ExpectedSHA256))) != 1 {
		s.recordHashMismatch(ctx, input, hashHex, "")
		return EvidenceItem{}, &HashMismatchError{
			ExpectedSHA256: strings.ToLower(input.ExpectedSHA256),
			ActualSHA256:   strings.ToLower(hashHex),
		}
	}

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

	// Generate thumbnail in background with timeout and panic recovery.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("thumbnail generation panicked", "evidence_id", evidence.ID, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		s.generateThumbnail(ctx, evidence.ID, input.CaseID, evidence.Version, sanitizedName, mimeType, data)
	}()

	// Custody log
	s.recordCustodyEvent(ctx, input.CaseID, evidence.ID, "evidence_uploaded", input.UploadedBy, map[string]string{
		"evidence_number": evidenceNumber,
		"filename":        sanitizedName,
		"sha256":          hashHex,
		"size_bytes":      fmt.Sprintf("%d", len(data)),
		"mime_type":       mimeType,
	})

	// Sprint 11.5: record stored event for forensic audit trail.
	if s.attempts != nil && input.AttemptID != uuid.Nil {
		if err := s.attempts.RecordEvent(ctx, input.AttemptID, "stored", map[string]any{
			"evidence_id": evidence.ID.String(),
			"storage_key": storageKey,
		}); err != nil {
			s.logger.Warn("failed to record stored attempt event", "attempt_id", input.AttemptID, "error", err)
		}
	}

	// Index in Meilisearch
	s.indexEvidence(ctx, evidence)

	return evidence, nil
}

// recordHashMismatch records mismatch events and queues cleanup actions.
// No MinIO delete is performed inline — the outbox worker handles it.
func (s *Service) recordHashMismatch(ctx context.Context, input UploadInput, actualHash, storageKey string) {
	expected := strings.ToLower(input.ExpectedSHA256)
	actual := strings.ToLower(actualHash)

	if s.attempts != nil && input.AttemptID != uuid.Nil {
		if err := s.attempts.RecordEvent(ctx, input.AttemptID, "hash_mismatch", map[string]any{
			"expected_sha256": expected,
			"actual_sha256":   actual,
		}); err != nil {
			s.logger.Warn("failed to record hash mismatch attempt event", "attempt_id", input.AttemptID, "error", err)
		}
	}
	s.recordCustodyEvent(ctx, input.CaseID, uuid.Nil, "upload_hash_mismatch", input.UploadedBy, map[string]string{
		"expected_sha256": expected,
		"actual_sha256":   actual,
	})
	// Queue async MinIO object deletion and critical notification.
	if s.outboxPool != nil && storageKey != "" {
		if err := InsertOutboxItem(ctx, s.outboxPool, "minio_delete_object", map[string]any{
			"bucket":     s.storageBucket(),
			"object_key": storageKey,
		}); err != nil {
			s.logger.Error("failed to enqueue minio delete outbox item", "storage_key", storageKey, "error", err)
		}
		if err := InsertOutboxItem(ctx, s.outboxPool, "notification_send", map[string]any{
			"severity": "critical",
			"kind":     "upload_hash_mismatch",
			"case_id":  input.CaseID.String(),
			"user_id":  input.UploadedBy,
		}); err != nil {
			s.logger.Error("failed to enqueue mismatch notification outbox item", "error", err)
		}
	}
}

func (s *Service) storageBucket() string {
	return s.storage.BucketName()
}

// Get retrieves evidence metadata by ID. Uses the case-scoped query to
// prevent cross-case IDOR — the evidence must exist and belong to the same
// case as its stored case_id or the request returns ErrNotFound.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	return s.findByIDWithCaseScope(ctx, id)
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
	// Sprint 9: normalize tags through the full tag rules (lowercase,
	// regex, dedupe, limit). This closes the backdoor where PATCH could
	// bypass ValidateTag and write uppercase or special-character tags.
	if updates.Tags != nil {
		normalized, err := NormalizeTags(updates.Tags)
		if err != nil {
			return EvidenceItem{}, err
		}
		updates.Tags = normalized
	}

	if err := validateEvidenceUpdate(updates); err != nil {
		return EvidenceItem{}, err
	}

	// Always fetch the prior state with case scope to verify the caller has
	// access to this evidence item, regardless of which fields are being
	// updated. This closes the path where non-classification updates
	// (description, tags) could bypass the case-scoping check.
	prior, err := s.findByIDWithCaseScope(ctx, id)
	if err != nil {
		return EvidenceItem{}, err
	}

	// Classification changes must satisfy the ex_parte rules before touching
	// the database. Use the already-fetched prior state to emit a precise
	// custody event and, when the classification moves off ex_parte, clear
	// the side. The prior classification is also passed to the repository as
	// an optimistic-concurrency guard (Sprint 9 M4) so two concurrent writers
	// cannot both "win" a race where one sets classification to ex_parte
	// and the other clears the side.
	if updates.Classification != nil {
		if err := ValidateClassificationChange(*updates.Classification, updates.ExParteSide); err != nil {
			return EvidenceItem{}, err
		}
		if *updates.Classification != ClassificationExParte && updates.ExParteSide == nil {
			updates.ClearExParteSide = true
		}
		priorClass := prior.Classification
		updates.ExpectedClassification = &priorClass
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

	// Classification change gets a dedicated custody event with before/after so
	// the chain records it distinctly from generic metadata edits (Sprint 9).
	if updates.Classification != nil && prior.Classification != *updates.Classification {
		detail := map[string]string{
			"previous_classification": prior.Classification,
			"new_classification":      *updates.Classification,
		}
		if prior.ExParteSide != nil {
			detail["previous_ex_parte_side"] = *prior.ExParteSide
		}
		if updates.ExParteSide != nil {
			detail["new_ex_parte_side"] = *updates.ExParteSide
		}
		s.recordCustodyEvent(ctx, result.CaseID, result.ID, "classification_changed", actorID, detail)
	}

	// Re-index
	s.indexEvidence(ctx, result)

	return result, nil
}

// Download streams the evidence file from storage.
func (s *Service) Download(ctx context.Context, id uuid.UUID, actorID string) (io.ReadCloser, int64, string, string, error) {
	evidence, err := s.findByIDWithCaseScope(ctx, id)
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
	evidence, err := s.findByIDWithCaseScope(ctx, id)
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

// Destroy is the pre-Sprint 9 soft-delete path. It is DEPRECATED and is no
// longer reachable via HTTP — the DELETE /api/evidence/{id} handler routes
// to DestroyEvidence (destruction.go), which enforces the audited
// authority/retention/DB-first flow required by the Sprint 9 spec.
//
// This method is retained solely for backwards compatibility with existing
// unit tests. New callers MUST use DestroyEvidence. Do not wire this to any
// HTTP, CLI, or scheduled job path.
//
// Deprecated: use DestroyEvidence instead.
func (s *Service) Destroy(ctx context.Context, input DestroyInput) error {
	evidence, err := s.findByIDWithCaseScope(ctx, input.EvidenceID)
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
	parent, err := s.findByIDWithCaseScope(ctx, parentID)
	if err != nil {
		return EvidenceItem{}, err
	}

	if parent.DestroyedAt != nil {
		return EvidenceItem{}, &ValidationError{Field: "evidence", Message: "cannot version destroyed evidence"}
	}

	// Sprint 9: file replacement is a destructive mutation — block when the
	// case is under legal hold.
	if err := s.checkLegalHold(ctx, parent.CaseID); err != nil {
		return EvidenceItem{}, err
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
	// Re-fetch via scoped query — we know the caseID from the parent.
	sr := s.getScoped()
	if sr != nil {
		return sr.FindByIDScoped(ctx, parent.CaseID, evidence.ID)
	}
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
	if !validClassifications[input.Classification] {
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
	if updates.Classification != nil && !validClassifications[*updates.Classification] {
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
	// Sprint 9: include ex_parte_side in the indexed payload so the search
	// handler can apply the classification access matrix at query time.
	// Without this field a defence user could see prosecution ex_parte
	// items in search results.
	payload := map[string]any{
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
	}
	if evidence.ExParteSide != nil {
		payload["ex_parte_side"] = *evidence.ExParteSide
	}
	doc := search.Document{
		ID:      evidence.ID.String(),
		Index:   "evidence",
		Payload: payload,
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

// UpsertCaptureMetadata validates and saves capture metadata for an evidence item.
// actorRole is the caller's case role (investigator/prosecutor/judge) — used to
// enforce role-elevation for verification status changes.
func (s *Service) UpsertCaptureMetadata(ctx context.Context, evidenceID uuid.UUID, input CaptureMetadataInput, actorID, actorRole string) (*EvidenceCaptureMetadata, []CaptureMetadataWarning, error) {
	if s.captureMetadata == nil {
		return nil, nil, fmt.Errorf("capture metadata repository not configured")
	}

	warnings, err := ValidateCaptureMetadataInput(input)
	if err != nil {
		return nil, nil, err
	}

	// Verify evidence exists and get case ID for custody event.
	// Use the scoped variant to prevent cross-case IDOR.
	evidence, err := s.findByIDWithCaseScope(ctx, evidenceID)
	if err != nil {
		return nil, nil, err
	}

	captureTS, _ := time.Parse(time.RFC3339, input.CaptureTimestamp)

	var pubTS *time.Time
	if input.PublicationTimestamp != nil && *input.PublicationTimestamp != "" {
		t, _ := time.Parse(time.RFC3339, *input.PublicationTimestamp)
		pubTS = &t
	}

	// Pin collector to the authenticated actor — never accept client-supplied collector_user_id
	// to prevent chain-of-custody spoofing.
	actorUUID, err := uuid.Parse(actorID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid actor ID: %w", err)
	}
	collectorUID := &actorUUID

	// Check if existing metadata has a different verification status
	var previousVerification string
	existing, existErr := s.captureMetadata.GetByEvidenceID(ctx, evidenceID)
	if existErr == nil && existing != nil {
		previousVerification = existing.VerificationStatus
	}

	verificationStatus := VerificationUnverified
	if input.VerificationStatus != nil && *input.VerificationStatus != "" {
		verificationStatus = *input.VerificationStatus
	}

	// Role-elevation: setting verified/disputed requires prosecutor or judge.
	// Investigators can capture metadata but cannot self-certify verification.
	elevatedStatuses := map[string]bool{VerificationVerified: true, VerificationDisputed: true}
	if elevatedStatuses[verificationStatus] {
		elevatedRoles := map[string]bool{"prosecutor": true, "judge": true}
		if !elevatedRoles[actorRole] {
			return nil, nil, &ValidationError{
				Field:   "verification_status",
				Message: "setting verified or disputed status requires prosecutor or judge role",
			}
		}
	}

	metadata := &EvidenceCaptureMetadata{
		EvidenceID:                    evidenceID,
		SourceURL:                     input.SourceURL,
		CanonicalURL:                  input.CanonicalURL,
		Platform:                      input.Platform,
		PlatformContentType:           input.PlatformContentType,
		CaptureMethod:                 input.CaptureMethod,
		CaptureTimestamp:              captureTS,
		PublicationTimestamp:           pubTS,
		CollectorUserID:               collectorUID,
		// CollectorDisplayName is intentionally NOT persisted from user input.
		// The _encrypted DB column requires application-layer encryption (same pattern
		// as witness PII). Until the encryption service is wired, we store only
		// collector_user_id and resolve the display name at read time via Keycloak/SSO.
		// This prevents storing plaintext PII in a column named _encrypted.
		CreatorAccountHandle:          input.CreatorAccountHandle,
		CreatorAccountDisplayName:     input.CreatorAccountDisplayName,
		CreatorAccountURL:             input.CreatorAccountURL,
		CreatorAccountID:              input.CreatorAccountID,
		ContentDescription:            input.ContentDescription,
		ContentLanguage:               input.ContentLanguage,
		GeoLatitude:                   input.GeoLatitude,
		GeoLongitude:                  input.GeoLongitude,
		GeoPlaceName:                  input.GeoPlaceName,
		GeoSource:                     input.GeoSource,
		AvailabilityStatus:            input.AvailabilityStatus,
		WasLive:                       input.WasLive,
		WasDeleted:                    input.WasDeleted,
		CaptureToolName:               input.CaptureToolName,
		CaptureToolVersion:            input.CaptureToolVersion,
		BrowserName:                   input.BrowserName,
		BrowserVersion:                input.BrowserVersion,
		BrowserUserAgent:              input.BrowserUserAgent,
		NetworkContext:                 input.NetworkContext,
		PreservationNotes:             input.PreservationNotes,
		VerificationStatus:            verificationStatus,
		VerificationNotes:             input.VerificationNotes,
		MetadataSchemaVersion:         1,
	}

	if err := s.captureMetadata.UpsertByEvidenceID(ctx, evidenceID, metadata); err != nil {
		return nil, nil, fmt.Errorf("upsert capture metadata: %w", err)
	}

	// Set fields that the DB populates
	metadata.EvidenceID = evidenceID
	now := time.Now().UTC()
	metadata.CreatedAt = now
	metadata.UpdatedAt = now

	// Custody event: capture_metadata_upserted
	detail := map[string]string{
		"capture_method":      metadata.CaptureMethod,
		"verification_status": metadata.VerificationStatus,
	}
	if metadata.Platform != nil {
		detail["platform"] = *metadata.Platform
	}
	if metadata.SourceURL != nil && *metadata.SourceURL != "" {
		detail["source_url_present"] = "true"
	}
	s.recordCustodyEvent(ctx, evidence.CaseID, evidenceID, "capture_metadata_upserted", actorID, detail)

	// Dedicated custody event if verification status changed
	if previousVerification != "" && previousVerification != verificationStatus {
		s.recordCustodyEvent(ctx, evidence.CaseID, evidenceID, "capture_metadata_verification_changed", actorID, map[string]string{
			"previous_status": previousVerification,
			"new_status":      verificationStatus,
		})
	}

	// Re-index with capture metadata fields
	s.indexEvidenceWithCapture(ctx, evidence, metadata)

	return metadata, warnings, nil
}

// GetCaptureMetadata retrieves capture metadata for an evidence item.
func (s *Service) GetCaptureMetadata(ctx context.Context, evidenceID uuid.UUID) (*EvidenceCaptureMetadata, error) {
	if s.captureMetadata == nil {
		return nil, ErrCaptureMetadataNotFound
	}
	return s.captureMetadata.GetByEvidenceID(ctx, evidenceID)
}

// indexEvidenceWithCapture extends the search index entry with capture metadata fields.
func (s *Service) indexEvidenceWithCapture(ctx context.Context, ev EvidenceItem, cm *EvidenceCaptureMetadata) {
	if s.indexer == nil {
		return
	}
	payload := map[string]any{
		"case_id":         ev.CaseID.String(),
		"evidence_number": derefStr(ev.EvidenceNumber),
		"filename":        ev.Filename,
		"original_name":   ev.OriginalName,
		"mime_type":       ev.MimeType,
		"classification":  ev.Classification,
		"description":     ev.Description,
		"tags":            ev.Tags,
		"uploaded_by":     ev.UploadedBy,
		"sha256_hash":     ev.SHA256Hash,
		"created_at":      ev.CreatedAt.Format(time.RFC3339),
	}
	if ev.ExParteSide != nil {
		payload["ex_parte_side"] = *ev.ExParteSide
	}
	// Add searchable capture metadata fields (exclude sensitive ones)
	if cm != nil {
		if cm.Platform != nil {
			payload["platform"] = *cm.Platform
		}
		if cm.SourceURL != nil {
			payload["source_url"] = *cm.SourceURL
		}
		if cm.ContentLanguage != nil {
			payload["content_language"] = *cm.ContentLanguage
		}
		payload["capture_method"] = cm.CaptureMethod
		payload["verification_status"] = cm.VerificationStatus
		payload["capture_timestamp"] = cm.CaptureTimestamp.Format(time.RFC3339)
	}
	doc := search.Document{
		ID:      ev.ID.String(),
		Index:   "evidence",
		Payload: payload,
	}
	if err := s.indexer.IndexDocument(ctx, doc); err != nil {
		s.logger.Error("failed to index evidence with capture metadata", "id", ev.ID, "error", err)
	}
}
