package search

// EvidenceSearchDoc represents an evidence item formatted for search indexing.
type EvidenceSearchDoc struct {
	ID             string   `json:"id"`
	CaseID         string   `json:"case_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	EvidenceNumber string   `json:"evidence_number"`
	Tags           []string `json:"tags"`
	Source         string   `json:"source"`
	FileName       string   `json:"file_name"`
	MimeType       string   `json:"mime_type"`
	Classification string   `json:"classification"`
	SourceDate     *string  `json:"source_date"`
	UploadedAt     string   `json:"uploaded_at"`
	IsCurrent      bool     `json:"is_current"`
	IsDisclosed    bool     `json:"is_disclosed"`
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
	UserCaseIDs     []string
	DisclosedOnly   bool
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
	Highlights     map[string][]string `json:"highlights"`
	Score          float64             `json:"score"`
}
