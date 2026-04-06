package custody

import (
	"context"

	"github.com/google/uuid"
)

type CustodyReader interface {
	ListByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]Event, error)
}
