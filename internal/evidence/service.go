package evidence

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EvidenceItem struct {
	ID             uuid.UUID
	CaseID         uuid.UUID
	Filename       string
	OriginalName   string
	MimeType       string
	SizeBytes      int64
	SHA256Hash     string
	Classification string
	UploadedBy     uuid.UUID
	IsCurrent      bool
	Version        int
	TSAToken       []byte
	CreatedAt      time.Time
}

type EvidenceFilter struct {
	CaseID         uuid.UUID
	Classification string
	CurrentOnly    bool
}

type CreateEvidenceInput struct {
	CaseID         uuid.UUID
	Filename       string
	OriginalName   string
	MimeType       string
	SizeBytes      int64
	SHA256Hash     string
	Classification string
}

type EvidenceReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (EvidenceItem, error)
	ListByCase(ctx context.Context, filter EvidenceFilter) ([]EvidenceItem, error)
}

type EvidenceWriter interface {
	Create(ctx context.Context, input CreateEvidenceInput) (EvidenceItem, error)
}
