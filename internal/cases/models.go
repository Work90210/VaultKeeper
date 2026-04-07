package cases

import (
	"time"

	"github.com/google/uuid"
)

type Case struct {
	ID            uuid.UUID `json:"id"`
	ReferenceCode string    `json:"reference_code"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Jurisdiction  string    `json:"jurisdiction"`
	Status        string    `json:"status"`
	LegalHold     bool      `json:"legal_hold"`
	CreatedBy     string    `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CaseFilter struct {
	UserID       string
	SystemAdmin  bool
	Status       []string
	Jurisdiction string
	SearchQuery  string
}

type CreateCaseInput struct {
	ReferenceCode string `json:"reference_code"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Jurisdiction  string `json:"jurisdiction"`
}

type UpdateCaseInput struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	Jurisdiction *string `json:"jurisdiction"`
	Status       *string `json:"status"`
}

type Pagination struct {
	Limit  int
	Cursor string
}

type PaginatedResult[T any] struct {
	Items      []T    `json:"items"`
	TotalCount int    `json:"total_count"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type CaseRole struct {
	ID        uuid.UUID `json:"id"`
	CaseID    uuid.UUID `json:"case_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	GrantedBy string    `json:"granted_by"`
	GrantedAt time.Time `json:"granted_at"`
}

type AssignRoleInput struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

const (
	StatusActive   = "active"
	StatusClosed   = "closed"
	StatusArchived = "archived"

	MaxTitleLength       = 500
	MaxDescriptionLength = 10000
	MaxJurisdictionLen   = 200
	MaxBodySize          = 1 << 20 // 1MB

	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

var ValidCaseRoles = map[string]bool{
	"investigator":          true,
	"prosecutor":            true,
	"defence":               true,
	"judge":                 true,
	"observer":              true,
	"victim_representative": true,
}

var validStatusTransitions = map[string]string{
	StatusActive: StatusClosed,
	StatusClosed: StatusArchived,
}

func IsValidStatusTransition(from, to string) bool {
	allowed, ok := validStatusTransitions[from]
	return ok && allowed == to
}

func ClampPagination(p Pagination) Pagination {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	return p
}
