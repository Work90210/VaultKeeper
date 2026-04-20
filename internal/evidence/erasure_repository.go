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

// FindErasureRequest loads an erasure request by ID (unscoped).
// Satisfies the ErasureRepository interface — used as the fallback when the
// caller does not have a case ID. Production service code uses the
// scopedErasureRepository methods (FindCaseIDForErasureRequest +
// FindErasureRequestScoped) to prevent cross-case IDOR.
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

// FindCaseIDForErasureRequest returns the case_id that owns the evidence
// item referenced by the given erasure request. Used to bootstrap
// case-scoped operations when only the erasure request ID is known.
// Implements the scopedErasureRepository extension interface.
func (r *PGRepository) FindCaseIDForErasureRequest(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var caseID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT ei.case_id
		 FROM erasure_requests er
		 JOIN evidence_items ei ON ei.id = er.evidence_id
		 WHERE er.id = $1`,
		id,
	).Scan(&caseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, ErrNotFound
		}
		return uuid.UUID{}, fmt.Errorf("find case id for erasure request: %w", err)
	}
	return caseID, nil
}

// FindErasureRequestScoped loads an erasure request by ID, restricted to
// the given case. The JOIN on evidence_items prevents cross-case IDOR: a
// request that belongs to a different case returns ErrNotFound rather than
// leaking data. Implements the scopedErasureRepository extension interface.
func (r *PGRepository) FindErasureRequestScoped(ctx context.Context, caseID, id uuid.UUID) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`SELECT er.id, er.evidence_id, er.requested_by, er.rationale, er.status,
		        er.decision, er.decided_by, er.decided_at, er.created_at
		 FROM erasure_requests er
		 JOIN evidence_items ei ON ei.id = er.evidence_id
		 WHERE er.id = $1 AND ei.case_id = $2`,
		id, caseID,
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

// UpdateErasureDecision transitions an erasure request to a terminal state
// (unscoped). Satisfies the ErasureRepository interface. Production service
// code uses UpdateErasureDecisionScoped to prevent cross-case IDOR.
func (r *PGRepository) UpdateErasureDecision(ctx context.Context, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`UPDATE erasure_requests
		 SET status = $1, decision = $2, decided_by = $3, decided_at = $4
		 WHERE id = $5
		   AND status = 'conflict_pending'
		 RETURNING id, evidence_id, requested_by, rationale, status, decision, decided_by, decided_at, created_at`,
		status, decision, decidedBy, decidedAt, id,
	).Scan(&out.ID, &out.EvidenceID, &out.RequestedBy, &out.Rationale, &out.Status,
		&out.Decision, &out.DecidedBy, &out.DecidedAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErasureRequest{}, fmt.Errorf("erasure request not found or already resolved: %w", ErrNotFound)
		}
		return ErasureRequest{}, fmt.Errorf("update erasure decision: %w", err)
	}
	return out, nil
}

// UpdateErasureDecisionScoped transitions an erasure request to a terminal
// state, restricted to the given case. The subquery on evidence_items
// prevents cross-case IDOR: an update that targets a request owned by a
// different case matches no rows and returns ErrNotFound.
// Implements the scopedErasureRepository extension interface.
func (r *PGRepository) UpdateErasureDecisionScoped(ctx context.Context, caseID, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error) {
	var out ErasureRequest
	err := r.pool.QueryRow(ctx,
		`UPDATE erasure_requests
		 SET status = $1, decision = $2, decided_by = $3, decided_at = $4
		 WHERE id = $5
		   AND status = 'conflict_pending'
		   AND evidence_id IN (SELECT id FROM evidence_items WHERE case_id = $6)
		 RETURNING id, evidence_id, requested_by, rationale, status, decision, decided_by, decided_at, created_at`,
		status, decision, decidedBy, decidedAt, id, caseID,
	).Scan(&out.ID, &out.EvidenceID, &out.RequestedBy, &out.Rationale, &out.Status,
		&out.Decision, &out.DecidedBy, &out.DecidedAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErasureRequest{}, fmt.Errorf("erasure request not found or already resolved: %w", ErrNotFound)
		}
		return ErasureRequest{}, fmt.Errorf("update erasure decision: %w", err)
	}
	return out, nil
}
