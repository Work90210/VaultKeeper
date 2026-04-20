package disclosures

import (
	"time"

	"github.com/google/uuid"
)

// Disclosure represents an evidence disclosure record.
type Disclosure struct {
	ID          uuid.UUID   `json:"id"`
	BatchID     uuid.UUID   `json:"batch_id"`
	CaseID      uuid.UUID   `json:"case_id"`
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
	DisclosedTo string      `json:"disclosed_to"`
	DisclosedBy uuid.UUID   `json:"disclosed_by"`
	DisclosedAt time.Time   `json:"disclosed_at"`
	Notes       string      `json:"notes"`
	Redacted    bool        `json:"redacted"`
}

// CreateDisclosureInput holds the parameters for creating a disclosure.
type CreateDisclosureInput struct {
	CaseID      uuid.UUID   `json:"case_id"`
	EvidenceIDs []uuid.UUID `json:"evidence_ids"`
	DisclosedTo string      `json:"disclosed_to"`
	Notes       string      `json:"notes"`
	Redacted    bool        `json:"redacted"`
}

// Pagination holds limit and cursor for paginated queries.
type Pagination struct {
	Limit  int
	Cursor string
}

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
	MaxBodySize      = 1 << 20
)

// ClampPagination normalizes pagination parameters.
func ClampPagination(p Pagination) Pagination {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	return p
}
