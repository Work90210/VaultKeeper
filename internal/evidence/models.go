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

// ValidClassifications is the set of allowed classification values.
var ValidClassifications = map[string]bool{
	ClassificationPublic:       true,
	ClassificationRestricted:   true,
	ClassificationConfidential: true,
	ClassificationExParte:      true,
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
	DestroyedAt    *time.Time `json:"destroyed_at,omitempty"`
	DestroyedBy    *string    `json:"destroyed_by,omitempty"`
	DestroyReason  *string    `json:"destroy_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
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

// EvidenceUpdate holds optional fields for metadata updates.
type EvidenceUpdate struct {
	Description    *string  `json:"description"`
	Classification *string  `json:"classification"`
	Tags           []string `json:"tags"`
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
	Description    string
	Tags           []string
	UploadedBy     string
	TSAToken       []byte
	TSAName        string
	TSATimestamp   *time.Time
	TSAStatus      string
	ExifData       []byte
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
