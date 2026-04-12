package evidence

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Classification levels for evidence items.
const (
	ClassificationPublic       = "public"
	ClassificationRestricted   = "restricted"
	ClassificationConfidential = "confidential"
	ClassificationExParte      = "ex_parte"
)

// RedactionPurpose defines the legal purpose of a redacted version.
type RedactionPurpose string

const (
	PurposeDisclosureDefence     RedactionPurpose = "disclosure_defence"
	PurposeDisclosureProsecution RedactionPurpose = "disclosure_prosecution"
	PurposePublicRelease         RedactionPurpose = "public_release"
	PurposeCourtSubmission       RedactionPurpose = "court_submission"
	PurposeWitnessProtection     RedactionPurpose = "witness_protection"
	PurposeInternalReview        RedactionPurpose = "internal_review"
)

// ValidPurposes is the set of allowed redaction purpose values.
var ValidPurposes = map[RedactionPurpose]bool{
	PurposeDisclosureDefence:     true,
	PurposeDisclosureProsecution: true,
	PurposePublicRelease:         true,
	PurposeCourtSubmission:       true,
	PurposeWitnessProtection:     true,
	PurposeInternalReview:        true,
}

// PurposeCode maps a redaction purpose to its evidence number suffix code.
var PurposeCode = map[RedactionPurpose]string{
	PurposeDisclosureDefence:     "DEFENCE",
	PurposeDisclosureProsecution: "PROSECUTION",
	PurposePublicRelease:         "PUBLIC",
	PurposeCourtSubmission:       "COURT",
	PurposeWitnessProtection:     "WITNESS",
	PurposeInternalReview:        "INTERNAL",
}

// TSA status values.
const (
	TSAStatusPending  = "pending"
	TSAStatusStamped  = "stamped"
	TSAStatusFailed   = "failed"
	TSAStatusDisabled = "disabled"
)

// Size and validation constants.
const (
	MaxFilenameLength    = 255
	MaxDescriptionLength = 10000
	MaxTagLength         = 100
	MaxTagCount          = 50
	MaxBodySize          = 1 << 20 // 1MB for JSON metadata updates

	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// validClassifications is the set of allowed classification values.
// Unexported so other packages cannot mutate its membership. Use
// IsValidClassification for cross-package lookups.
var validClassifications = map[string]bool{
	ClassificationPublic:       true,
	ClassificationRestricted:   true,
	ClassificationConfidential: true,
	ClassificationExParte:      true,
}

// IsValidClassification reports whether s is a recognised classification
// value (public, restricted, confidential, ex_parte).
func IsValidClassification(s string) bool {
	return validClassifications[s]
}

// safeFilenameRe allows alphanumerics, hyphens, underscores, dots, and spaces.
var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._\- ]`)

// EvidenceItem is the core domain model for an uploaded evidence file.
type EvidenceItem struct {
	ID             uuid.UUID  `json:"id"`
	CaseID         uuid.UUID  `json:"case_id"`
	EvidenceNumber *string    `json:"evidence_number"`
	Filename       string     `json:"filename"`
	OriginalName   string     `json:"original_name"`
	StorageKey     *string    `json:"storage_key"`
	ThumbnailKey   *string    `json:"thumbnail_key,omitempty"`
	MimeType       string     `json:"mime_type"`
	SizeBytes      int64      `json:"size_bytes"`
	SHA256Hash     string     `json:"sha256_hash"`
	Classification string     `json:"classification"`
	Description    string     `json:"description"`
	Tags           []string   `json:"tags"`
	UploadedBy     string     `json:"uploaded_by"`
	UploadedByName string     `json:"uploaded_by_name"`
	IsCurrent      bool       `json:"is_current"`
	Version        int        `json:"version"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	TSAToken       []byte     `json:"-"`
	TSAName        *string    `json:"tsa_name,omitempty"`
	TSATimestamp   *time.Time `json:"tsa_timestamp,omitempty"`
	TSAStatus      string     `json:"tsa_status"`
	TSARetryCount  int        `json:"-"`
	TSALastRetry   *time.Time `json:"-"`
	ExifData       []byte     `json:"exif_data,omitempty"`
	Source         string     `json:"source"`
	SourceDate     *time.Time `json:"source_date,omitempty"`
	ExParteSide    *string    `json:"ex_parte_side,omitempty"`
	DestroyedAt    *time.Time `json:"destroyed_at,omitempty"`
	DestroyedBy    *string    `json:"destroyed_by,omitempty"`
	DestroyReason  *string    `json:"destroy_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`

	// Redaction metadata (populated only for finalized redacted derivatives)
	RedactionName        *string           `json:"redaction_name,omitempty"`
	RedactionPurpose     *RedactionPurpose `json:"redaction_purpose,omitempty"`
	RedactionAreaCount   *int              `json:"redaction_area_count,omitempty"`
	RedactionAuthorID    *uuid.UUID        `json:"redaction_author_id,omitempty"`
	RedactionFinalizedAt *time.Time        `json:"redaction_finalized_at,omitempty"`

	// Retention and destruction metadata (Sprint 9 Step 3/4).
	// RetentionUntil is the soonest date at which this item may be destroyed.
	// DestructionAuthority records the legal authority cited at destruction.
	RetentionUntil       *time.Time `json:"retention_until,omitempty"`
	DestructionAuthority *string    `json:"destruction_authority,omitempty"`
}

// EvidenceFilter specifies query parameters for listing evidence.
type EvidenceFilter struct {
	CaseID           uuid.UUID
	Classification   string
	MimeType         string
	Tags             []string
	CurrentOnly      bool
	IncludeDestroyed bool
	SearchQuery      string
	UserRole         string
}

// ApplyAccessFilter reports whether the caller's role is set, meaning the
// repository should restrict results using the classification access matrix.
func (f EvidenceFilter) ApplyAccessFilter() bool {
	return f.UserRole != ""
}

// EvidenceUpdate holds optional fields for metadata updates.
type EvidenceUpdate struct {
	Description    *string    `json:"description"`
	Classification *string    `json:"classification"`
	ExParteSide    *string    `json:"ex_parte_side"`
	Tags           []string   `json:"tags"`
	RetentionUntil *time.Time `json:"retention_until"`
	// ClearExParteSide instructs the repository to set ex_parte_side back to
	// NULL (used when a classification is changed away from ex_parte).
	ClearExParteSide bool `json:"-"`
	// ClearRetentionUntil instructs the repository to set retention_until to NULL.
	ClearRetentionUntil bool `json:"-"`
	// ExpectedClassification is an optimistic-concurrency guard: when
	// non-nil, the repository UPDATE adds `AND classification = $expected`
	// to the WHERE clause. If another writer changed the classification
	// between the service's prior-fetch and the UPDATE, the repository
	// returns ErrConflict and the caller must retry with a fresh read.
	// Not serialised over JSON (internal-only).
	ExpectedClassification *string `json:"-"`
}

// CreateEvidenceInput is the validated input for creating a new evidence record.
type CreateEvidenceInput struct {
	CaseID         uuid.UUID
	EvidenceNumber string
	Filename       string
	OriginalName   string
	StorageKey     string
	MimeType       string
	SizeBytes      int64
	SHA256Hash     string
	Classification string
	ExParteSide    *string
	Description    string
	Tags           []string
	UploadedBy     string
	UploadedByName string
	Source         string
	SourceDate     *time.Time
	TSAToken       []byte
	TSAName        string
	TSATimestamp   *time.Time
	TSAStatus      string
	ExifData       []byte

	// Redaction metadata (set during finalize-from-draft)
	RedactionName        *string
	RedactionPurpose     *RedactionPurpose
	RedactionAreaCount   *int
	RedactionAuthorID    *uuid.UUID
	RedactionFinalizedAt *time.Time

	// Retention metadata (Sprint 9 Step 3).
	RetentionUntil *time.Time
}

// DestroyInput holds the parameters for destroying evidence.
type DestroyInput struct {
	EvidenceID uuid.UUID
	Reason     string
	ActorID    string
}

// Pagination holds limit and cursor for paginated queries.
type Pagination struct {
	Limit  int
	Cursor string
}

// PaginatedResult wraps a page of results with metadata.
type PaginatedResult[T any] struct {
	Items      []T    `json:"items"`
	TotalCount int    `json:"total_count"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// SanitizeFilename removes path traversal, unsafe characters, and enforces length.
func SanitizeFilename(name string) string {
	// Strip directory components
	name = filepath.Base(name)

	// Remove null bytes
	name = strings.ReplaceAll(name, "\x00", "")

	// Replace unsafe characters
	name = safeFilenameRe.ReplaceAllString(name, "_")

	// Collapse multiple underscores/spaces
	name = regexp.MustCompile(`[_ ]{2,}`).ReplaceAllString(name, "_")

	// Trim leading/trailing dots and spaces
	name = strings.Trim(name, ". ")

	// Enforce max length
	if len(name) > MaxFilenameLength {
		ext := filepath.Ext(name)
		base := name[:MaxFilenameLength-len(ext)]
		name = base + ext
	}

	if name == "" || name == "." || name == ".." {
		name = "unnamed"
	}

	return name
}

// ClampPagination normalizes pagination parameters to valid ranges.
func ClampPagination(p Pagination) Pagination {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	return p
}

// StorageObjectKey builds the MinIO key for an evidence file.
func StorageObjectKey(caseID, evidenceID uuid.UUID, version int, filename string) string {
	return fmt.Sprintf("evidence/%s/%s/%d/%s", caseID, evidenceID, version, filename)
}

// RedactionDraft represents a named redaction draft in progress.
type RedactionDraft struct {
	ID          uuid.UUID        `json:"id"`
	EvidenceID  uuid.UUID        `json:"evidence_id"`
	CaseID      uuid.UUID        `json:"case_id"`
	Name        string           `json:"name"`
	Purpose     RedactionPurpose `json:"purpose"`
	AreaCount   int              `json:"area_count"`
	CreatedBy   string           `json:"created_by"`
	Status      string           `json:"status"`
	LastSavedAt time.Time        `json:"last_saved_at"`
	CreatedAt   time.Time        `json:"created_at"`
}

// FinalizedRedaction represents a finalized redacted evidence derivative.
type FinalizedRedaction struct {
	ID             uuid.UUID        `json:"id"`
	EvidenceNumber string           `json:"evidence_number"`
	Name           string           `json:"name"`
	Purpose        RedactionPurpose `json:"purpose"`
	AreaCount      int              `json:"area_count"`
	Author         string           `json:"author"`
	FinalizedAt    time.Time        `json:"finalized_at"`
}

// RedactionManagementView combines finalized versions and active drafts.
type RedactionManagementView struct {
	Finalized []FinalizedRedaction `json:"finalized"`
	Drafts    []RedactionDraft     `json:"drafts"`
}

// FinalizeInput holds the parameters for finalizing a draft into a permanent redacted copy.
type FinalizeInput struct {
	EvidenceID     uuid.UUID
	DraftID        uuid.UUID
	Description    string
	Classification string
	ActorID        string
	ActorName      string
}
