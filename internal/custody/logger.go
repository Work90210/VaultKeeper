package custody

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID           uuid.UUID
	CaseID       uuid.UUID
	EvidenceID   uuid.UUID
	Action       string
	ActorUserID  uuid.UUID
	Detail       string
	HashValue    string
	PreviousHash string
	Timestamp    time.Time
}

type CustodyLogger interface {
	Record(ctx context.Context, event Event) error
}
