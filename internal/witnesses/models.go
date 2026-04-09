package witnesses

import (
	"time"

	"github.com/google/uuid"
)

// Protection status values.
const (
	ProtectionStandard = "standard"
	ProtectionProtected = "protected"
	ProtectionHighRisk  = "high_risk"
)

// ValidProtectionStatuses is the set of allowed protection status values.
var ValidProtectionStatuses = map[string]bool{
	ProtectionStandard:  true,
	ProtectionProtected: true,
	ProtectionHighRisk:  true,
}

// Witness is the internal representation with encrypted identity fields.
type Witness struct {
	ID                    uuid.UUID  `json:"id"`
	CaseID                uuid.UUID  `json:"case_id"`
	WitnessCode           string     `json:"witness_code"`
	FullNameEncrypted     []byte     `json:"-"`
	ContactInfoEncrypted  []byte     `json:"-"`
	LocationEncrypted     []byte     `json:"-"`
	ProtectionStatus      string     `json:"protection_status"`
	StatementSummary      string     `json:"statement_summary"`
	RelatedEvidence       []uuid.UUID `json:"related_evidence"`
	JudgeIdentityVisible  bool       `json:"judge_identity_visible"`
	CreatedBy             uuid.UUID  `json:"created_by"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// WitnessView is the API response with identity fields conditionally present.
type WitnessView struct {
	ID               uuid.UUID   `json:"id"`
	CaseID           uuid.UUID   `json:"case_id"`
	WitnessCode      string      `json:"witness_code"`
	FullName         *string     `json:"full_name"`
	ContactInfo      *string     `json:"contact_info"`
	Location         *string     `json:"location"`
	ProtectionStatus string      `json:"protection_status"`
	StatementSummary string      `json:"statement_summary"`
	RelatedEvidence  []uuid.UUID `json:"related_evidence"`
	IdentityVisible  bool        `json:"identity_visible"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// CreateWitnessInput holds the parameters for creating a new witness.
type CreateWitnessInput struct {
	CaseID           uuid.UUID
	WitnessCode      string
	FullName         *string
	ContactInfo      *string
	Location         *string
	ProtectionStatus string
	StatementSummary string
	RelatedEvidence  []uuid.UUID
	CreatedBy        string
}

// UpdateWitnessInput holds optional fields for updating a witness.
type UpdateWitnessInput struct {
	FullName             *string     `json:"full_name"`
	ContactInfo          *string     `json:"contact_info"`
	Location             *string     `json:"location"`
	ProtectionStatus     *string     `json:"protection_status"`
	StatementSummary     *string     `json:"statement_summary"`
	RelatedEvidence      []uuid.UUID `json:"related_evidence,omitempty"`
	JudgeIdentityVisible *bool       `json:"judge_identity_visible"`
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
