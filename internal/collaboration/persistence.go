package collaboration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// autosaveInterval controls how often dirty rooms are persisted.
// Declared as a var (not const) so tests can override it.
var autosaveInterval = 5 * time.Second

// DraftStore persists Yjs document state for redaction drafts.
type DraftStore interface {
	LoadDraft(ctx context.Context, evidenceID uuid.UUID) ([]byte, error)
	SaveDraft(ctx context.Context, evidenceID, caseID uuid.UUID, actorID string, state []byte) error
}

// PostgresDraftStore implements DraftStore using Postgres.
type PostgresDraftStore struct {
	db *pgxpool.Pool
}

// NewPostgresDraftStore creates a DraftStore backed by Postgres.
func NewPostgresDraftStore(db *pgxpool.Pool) *PostgresDraftStore {
	return &PostgresDraftStore{db: db}
}

// LoadDraft returns the most recent draft Yjs state for an evidence item.
// Returns nil, nil when no draft exists.
func (s *PostgresDraftStore) LoadDraft(ctx context.Context, evidenceID uuid.UUID) ([]byte, error) {
	var state []byte
	err := s.db.QueryRow(ctx,
		`SELECT yjs_state FROM redaction_drafts
		 WHERE evidence_id = $1 AND status = 'draft'
		 LIMIT 1`,
		evidenceID,
	).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load redaction draft: %w", err)
	}
	return state, nil
}

// SaveDraft persists the Yjs binary state for the most recently active draft
// on the evidence item. If no active draft exists for the evidence, creates a
// new one with an auto-generated name.
//
// Migration 015 replaced the single-draft-per-evidence model with multi-draft,
// so this now targets whichever active draft was most recently saved. Callers
// of the WebSocket collaboration path that want draft-specific persistence
// should use the REST API at /api/evidence/{id}/redact/drafts/{draftId} instead.
func (s *PostgresDraftStore) SaveDraft(ctx context.Context, evidenceID, caseID uuid.UUID, actorID string, state []byte) error {
	// Try to update the most recently saved active draft for this evidence.
	tag, err := s.db.Exec(ctx,
		`UPDATE redaction_drafts
		 SET yjs_state = $1, last_saved_at = now()
		 WHERE id = (
		     SELECT id FROM redaction_drafts
		     WHERE evidence_id = $2 AND status = 'draft'
		     ORDER BY last_saved_at DESC
		     LIMIT 1
		 )`,
		state, evidenceID,
	)
	if err != nil {
		return fmt.Errorf("save redaction draft: %w", err)
	}

	if tag.RowsAffected() > 0 {
		return nil
	}

	// No active draft — create a new one with a generated name.
	name := "Collaborative Draft " + time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = s.db.Exec(ctx,
		`INSERT INTO redaction_drafts (evidence_id, case_id, created_by, yjs_state, name, purpose, area_count)
		 VALUES ($1, $2, $3, $4, $5, 'internal_review', 0)`,
		evidenceID, caseID, actorID, state, name,
	)
	if err != nil {
		return fmt.Errorf("create collaborative draft: %w", err)
	}
	return nil
}
