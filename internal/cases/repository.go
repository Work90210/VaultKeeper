package cases

import (
	"context"

	"github.com/google/uuid"
)

type CaseWriter interface {
	Create(ctx context.Context, input CreateCaseInput) (Case, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateCaseInput) (Case, error)
}

type Repository interface {
	CaseReader
	CaseWriter
}
