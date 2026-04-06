package custody

import (
	"context"

	"github.com/google/uuid"
)

type VerificationResult struct {
	Valid  bool
	Reason string
}

type ChainVerifier interface {
	Verify(ctx context.Context, evidenceID uuid.UUID) (VerificationResult, error)
}
