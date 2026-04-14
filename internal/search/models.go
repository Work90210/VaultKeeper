package search

// EvidenceSearchDoc represents an evidence item formatted for search indexing.
//
// Sprint 9: ExParteSide is included in the indexed payload so the search
// handler can enforce the classification access matrix at query time
// without a round-trip to the evidence repository. Without this field a
// defence user could see prosecution ex_parte items in search results.
type EvidenceSearchDoc struct {
	ID             string   `json:"id"`
	CaseID         string   `json:"case_id"`
	OrganizationID string   `json:"organization_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	EvidenceNumber string   `json:"evidence_number"`
	Tags           []string `json:"tags"`
	Source         string   `json:"source"`
	FileName       string   `json:"file_name"`
	MimeType       string   `json:"mime_type"`
	Classification string   `json:"classification"`
	ExParteSide    *string  `json:"ex_parte_side,omitempty"`
	SourceDate     *string  `json:"source_date"`
	UploadedAt     string   `json:"uploaded_at"`
	IsCurrent      bool     `json:"is_current"`
	IsDisclosed    bool     `json:"is_disclosed"`

	// Berkeley Protocol capture metadata (non-sensitive fields only)
	Platform           *string `json:"platform,omitempty"`
	CaptureMethod      *string `json:"capture_method,omitempty"`
	SourceURL          *string `json:"source_url,omitempty"`
	ContentLanguage    *string `json:"content_language,omitempty"`
	VerificationStatus *string `json:"verification_status,omitempty"`
	CaptureTimestamp   *string `json:"capture_timestamp,omitempty"`
}

// SearchQuery describes a full-text search request with filters and pagination.
type SearchQuery struct {
	Query           string
	CaseID          *string
	MimeTypes       []string
	Tags            []string
	Classifications []string
	DateFrom        *string
	DateTo          *string
	Limit           int
	Offset          int
	UserCaseIDs       []string
	OrganizationIDs   []string
	DisclosedOnly     bool
}

// EvidenceSearchResult holds a page of search results with metadata.
type EvidenceSearchResult struct {
	Hits             []EvidenceSearchHit       `json:"hits"`
	TotalHits        int                       `json:"total_hits"`
	Query            string                    `json:"query"`
	ProcessingTimeMs int                       `json:"processing_time_ms"`
	Facets           map[string]map[string]int `json:"facets"`
}

// EvidenceSearchHit represents a single matched evidence item.
type EvidenceSearchHit struct {
	EvidenceID     string              `json:"evidence_id"`
	CaseID         string              `json:"case_id"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	EvidenceNumber string              `json:"evidence_number"`
	FileName       string              `json:"file_name,omitempty"`
	MimeType       string              `json:"mime_type,omitempty"`
	UploadedAt     string              `json:"uploaded_at,omitempty"`
	Highlights     map[string][]string `json:"highlights"`
	Score          float64             `json:"score"`
	// Sprint 9: classification + ex_parte_side drive the post-query
	// access filter in the HTTP handler. Not rendered to the user but
	// required for correctness.
	Classification string  `json:"classification,omitempty"`
	ExParteSide    *string `json:"ex_parte_side,omitempty"`
}
