package cases

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Case struct {
	ID            uuid.UUID
	ReferenceCode string
	Title         string
	Description   string
	Jurisdiction  string
	Status        string
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CaseFilter struct {
	Status       string
	Jurisdiction string
}

type CreateCaseInput struct {
	ReferenceCode string
	Title         string
	Description   string
	Jurisdiction  string
}

type UpdateCaseInput struct {
	Title       *string
	Description *string
	Status      *string
}

type CaseReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (Case, error)
	List(ctx context.Context, filter CaseFilter) ([]Case, error)
}
