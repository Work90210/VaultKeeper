package evidence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateErasureRequest inserts a new GDPR erasure request row.
func (r *PGRepository) CreateErasureRequest(ctx context.Context, req ErasureRequest) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`INSERT INTO erasure_requests (id, evidence_id, requested_by, rationale, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, evidence_id, requested_by, rationale, status, decision, decided_by, decided_at, created_at`,
		req.ID, req.EvidenceID, req.RequestedBy, req.Rationale, req.Status, req.CreatedAt,
	).Scan(&out.ID, &out.EvidenceID, &out.RequestedBy, &out.Rationale, &out.Status,
		&out.Decision, &out.DecidedBy, &out.DecidedAt, &out.CreatedAt)
	if err != nil {
		return ErasureRequest{}, fmt.Errorf("insert erasure request: %w", err)
	}
	return out, nil
}

// FindErasureRequest loads an erasure request by ID.
func (r *PGRepository) FindErasureRequest(ctx context.Context, id uuid.UUID) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`SELECT id, evidence_id, requested_by, rationale, status, decision, decided_by, decided_at, created_at
		 FROM erasure_requests WHERE id = $1`,
		id,
	).Scan(&out.ID, &out.EvidenceID, &out.RequestedBy, &out.Rationale, &out.Status,
		&out.Decision, &out.DecidedBy, &out.DecidedAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErasureRequest{}, ErrNotFound
		}
		return ErasureRequest{}, fmt.Errorf("find erasure request: %w", err)
	}
	return out, nil
}

// UpdateErasureDecision transitions an erasure request to a terminal state.
func (r *PGRepository) UpdateErasureDecision(ctx context.Context, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`UPDATE erasure_requests
		 SET status = $1, decision = $2, decided_by = $3, decided_at = $4
		 WHERE id = $5
		 RETURNING id, evidence_id, requested_by, rationale, status, decision, decided_by, decided_at, created_at`,
		status, decision, decidedBy, decidedAt, id,
	).Scan(&out.ID, &out.EvidenceID, &out.RequestedBy, &out.Rationale, &out.Status,
		&out.Decision, &out.DecidedBy, &out.DecidedAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErasureRequest{}, ErrNotFound
		}
		return ErasureRequest{}, fmt.Errorf("update erasure decision: %w", err)
	}
	return out, nil
}
