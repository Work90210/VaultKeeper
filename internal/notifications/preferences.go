package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationPreferences holds a user's notification preference flags.
// All fields default to their zero value (false) when not explicitly set.
type NotificationPreferences struct {
	EmailEnabled      bool `json:"email_enabled"`
	EvidenceUploaded  bool `json:"evidence_uploaded"`
	EvidenceDestroyed bool `json:"evidence_destroyed"`
	CaseStatusChanged bool `json:"case_status_changed"`
	LegalHoldChanged  bool `json:"legal_hold_changed"`
	MemberJoined      bool `json:"member_joined"`
	MemberRemoved     bool `json:"member_removed"`
	CustodyChainEvent bool `json:"custody_chain_event"`
	BackupFailed      bool `json:"backup_failed"`
	RetentionExpiring bool `json:"retention_expiring"`
}

// DefaultPreferences returns the default notification preferences for a new user.
func DefaultPreferences() NotificationPreferences {
	return NotificationPreferences{
		EmailEnabled:      false,
		EvidenceUploaded:  true,
		EvidenceDestroyed: true,
		CaseStatusChanged: true,
		LegalHoldChanged:  true,
		MemberJoined:      true,
		MemberRemoved:     false,
		CustodyChainEvent: true,
		BackupFailed:      true,
		RetentionExpiring: true,
	}
}

// PreferencesRepository defines persistence for notification preferences.
type PreferencesRepository interface {
	Get(ctx context.Context, userID string) (NotificationPreferences, error)
	Upsert(ctx context.Context, userID string, prefs NotificationPreferences) error
}

// PGPreferencesRepository implements PreferencesRepository with PostgreSQL.
type PGPreferencesRepository struct {
	pool *pgxpool.Pool
}

// NewPGPreferencesRepository creates a PGPreferencesRepository.
func NewPGPreferencesRepository(pool *pgxpool.Pool) *PGPreferencesRepository {
	return &PGPreferencesRepository{pool: pool}
}

// Get retrieves notification preferences for a user. If no row exists,
// it returns the default preferences without error.
func (r *PGPreferencesRepository) Get(ctx context.Context, userID string) (NotificationPreferences, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT preferences FROM notification_preferences WHERE user_id = $1`,
		userID,
	).Scan(&raw)
	if err != nil {
		// No row — return defaults.
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultPreferences(), nil
		}
		return NotificationPreferences{}, fmt.Errorf("get notification preferences: %w", err)
	}

	// Start from defaults, then overlay whatever the user has stored.
	prefs := DefaultPreferences()
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return NotificationPreferences{}, fmt.Errorf("unmarshal notification preferences: %w", err)
	}
	return prefs, nil
}

// Upsert inserts or replaces notification preferences for a user.
func (r *PGPreferencesRepository) Upsert(ctx context.Context, userID string, prefs NotificationPreferences) error {
	raw, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("marshal notification preferences: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO notification_preferences (user_id, preferences, updated_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET preferences = $2, updated_at = $3`,
		userID, raw, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert notification preferences: %w", err)
	}
	return nil
}
