package notifications

import (
	"time"

	"github.com/google/uuid"
)

// Notification represents a stored notification for a user.
type Notification struct {
	ID        uuid.UUID  `json:"id"`
	CaseID    *uuid.UUID `json:"case_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
}

// NotificationEvent describes something that happened and should produce
// notifications for the appropriate recipients.
type NotificationEvent struct {
	Type   string
	CaseID uuid.UUID
	Title  string
	Body   string

	// TargetUserID is the specific user to notify (e.g. for EventUserAddedToCase).
	// This keeps Body free for human-readable notification text.
	TargetUserID string
}

// Well-known event types.
const (
	EventEvidenceUploaded  = "evidence_uploaded"
	EventUserAddedToCase   = "user_added_to_case"
	EventIntegrityWarning  = "integrity_warning"
	EventLegalHoldChanged  = "legal_hold_changed"
	EventRetentionExpiring = "retention_expiring"
	EventBackupFailed      = "backup_failed"
)
