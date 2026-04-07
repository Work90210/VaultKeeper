package search

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// caseLoaderDB abstracts the database query method needed by PGCaseIDsLoader.
type caseLoaderDB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// PGCaseIDsLoader loads the case IDs a user has access to from PostgreSQL.
type PGCaseIDsLoader struct {
	pool caseLoaderDB
}

// NewCaseIDsLoader creates a new PGCaseIDsLoader backed by the given connection pool.
func NewCaseIDsLoader(pool *pgxpool.Pool) *PGCaseIDsLoader {
	return &PGCaseIDsLoader{pool: pool}
}

// newCaseIDsLoaderFromDB creates a PGCaseIDsLoader from any caseLoaderDB implementation (for testing).
func newCaseIDsLoaderFromDB(db caseLoaderDB) *PGCaseIDsLoader {
	return &PGCaseIDsLoader{pool: db}
}

// GetUserCaseIDs returns all case IDs the given user has a role assignment for.
func (l *PGCaseIDsLoader) GetUserCaseIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := l.pool.Query(ctx, "SELECT case_id FROM case_roles WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("query user case IDs: %w", err)
	}
	defer rows.Close()

	var caseIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan case ID: %w", err)
		}
		caseIDs = append(caseIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user case IDs: %w", err)
	}

	return caseIDs, nil
}

// GetUserCaseRoles returns a map of case ID to role for the given user.
func (l *PGCaseIDsLoader) GetUserCaseRoles(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := l.pool.Query(ctx, "SELECT case_id::text, role FROM case_roles WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("query user case roles: %w", err)
	}
	defer rows.Close()

	roles := make(map[string]string)
	for rows.Next() {
		var caseID, role string
		if err := rows.Scan(&caseID, &role); err != nil {
			return nil, fmt.Errorf("scan case role: %w", err)
		}
		roles[caseID] = role
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user case roles: %w", err)
	}

	return roles, nil
}
