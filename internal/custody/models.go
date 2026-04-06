package custody

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID           uuid.UUID
	CaseID       uuid.UUID
	EvidenceID   uuid.UUID
	Action       string
	ActorUserID  string
	Detail       string
	HashValue    string
	PreviousHash string
	Timestamp    time.Time
}

type ChainVerification struct {
	Valid        bool
	TotalEntries int
	VerifiedAt   time.Time
	Breaks       []ChainBreak
}

type ChainBreak struct {
	EntryID      uuid.UUID
	Position     int
	ExpectedHash string
	ActualHash   string
	Timestamp    time.Time
}
