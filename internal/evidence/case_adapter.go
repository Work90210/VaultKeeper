package evidence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGCaseLookup implements CaseLookup using direct Postgres queries.
type PGCaseLookup struct {
	pool interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	}
}

// NewCaseLookup creates a new case lookup backed by Postgres.
func NewCaseLookup(pool *pgxpool.Pool) *PGCaseLookup {
	return &PGCaseLookup{pool: pool}
}

func (l *PGCaseLookup) GetLegalHold(ctx context.Context, caseID uuid.UUID) (bool, error) {
	var held bool
	err := l.pool.QueryRow(ctx, `SELECT legal_hold FROM cases WHERE id = $1`, caseID).Scan(&held)
	if err != nil {
		return false, fmt.Errorf("get legal hold status: %w", err)
	}
	return held, nil
}

func (l *PGCaseLookup) GetReferenceCode(ctx context.Context, caseID uuid.UUID) (string, error) {
	var code string
	err := l.pool.QueryRow(ctx, `SELECT reference_code FROM cases WHERE id = $1`, caseID).Scan(&code)
	if err != nil {
		return "", fmt.Errorf("get case reference code: %w", err)
	}
	return code, nil
}
