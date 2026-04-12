package migration

import (
	"time"

	"github.com/google/uuid"
)

// MigrationStatus enumerates the lifecycle states of a migration event.
type MigrationStatus string

const (
	StatusInProgress    MigrationStatus = "in_progress"
	StatusCompleted     MigrationStatus = "completed"
	StatusFailed        MigrationStatus = "failed"
	StatusHaltedOnMismatch MigrationStatus = "halted_mismatch"
)

// Record is a persisted migration event (one row in evidence_migrations).
type Record struct {
	ID              uuid.UUID
	CaseID          uuid.UUID
	SourceSystem    string
	TotalItems      int
	MatchedItems    int
	MismatchedItems int
	MigrationHash   string
	ManifestHash    string
	TSAToken        []byte
	TSAName         string
	TSATimestamp    *time.Time
	PerformedBy     string
	Status          MigrationStatus
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}
