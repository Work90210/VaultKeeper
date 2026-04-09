package disclosures

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("disclosure not found")

// Repository defines the disclosure data access interface.
type Repository interface {
	Create(ctx context.Context, d Disclosure) (Disclosure, error)
	FindByID(ctx context.Context, id uuid.UUID) (Disclosure, error)
	FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error)
	EvidenceBelongsToCase(ctx context.Context, caseID uuid.UUID, evidenceIDs []uuid.UUID) (bool, error)
}

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// PGRepository is the Postgres implementation.
type PGRepository struct {
	pool dbPool
}

// NewRepository creates a new Postgres disclosure repository.
func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// Create inserts one row per evidence item in a single transaction.
// The returned Disclosure has the ID of the first row and all evidence IDs.
func (r *PGRepository) Create(ctx context.Context, d Disclosure) (Disclosure, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Disclosure{}, fmt.Errorf("begin disclosure tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var firstID uuid.UUID
	disclosedAt := time.Now().UTC()

	for i, evidenceID := range d.EvidenceIDs {
		var rowID uuid.UUID
		err := tx.QueryRow(ctx,
			`INSERT INTO disclosures (case_id, evidence_id, disclosed_to, disclosed_by, disclosed_at, notes, redacted)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			d.CaseID, evidenceID, d.DisclosedTo, d.DisclosedBy, disclosedAt, d.Notes, d.Redacted,
		).Scan(&rowID)
		if err != nil {
			return Disclosure{}, fmt.Errorf("insert disclosure row %d: %w", i, err)
		}
		if i == 0 {
			firstID = rowID
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Disclosure{}, fmt.Errorf("commit disclosure tx: %w", err)
	}

	return Disclosure{
		ID:          firstID,
		CaseID:      d.CaseID,
		EvidenceIDs: d.EvidenceIDs,
		DisclosedTo: d.DisclosedTo,
		DisclosedBy: d.DisclosedBy,
		DisclosedAt: disclosedAt,
		Notes:       d.Notes,
		Redacted:    d.Redacted,
	}, nil
}

// FindByID returns a disclosure by ID, aggregating all evidence IDs with the same
// disclosed_at timestamp and case_id from the same disclosure batch.
func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (Disclosure, error) {
	// First get the row to find batch identifiers
	var d Disclosure
	var evidenceID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id, case_id, evidence_id, disclosed_to, disclosed_by, disclosed_at, notes, redacted
		 FROM disclosures WHERE id = $1`, id,
	).Scan(&d.ID, &d.CaseID, &evidenceID, &d.DisclosedTo, &d.DisclosedBy, &d.DisclosedAt, &d.Notes, &d.Redacted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Disclosure{}, ErrNotFound
		}
		return Disclosure{}, fmt.Errorf("find disclosure by id: %w", err)
	}

	// Aggregate all evidence IDs from the same batch
	rows, err := r.pool.Query(ctx,
		`SELECT evidence_id FROM disclosures
		 WHERE case_id = $1 AND disclosed_by = $2 AND disclosed_at = $3
		 ORDER BY id ASC`,
		d.CaseID, d.DisclosedBy, d.DisclosedAt,
	)
	if err != nil {
		return Disclosure{}, fmt.Errorf("aggregate disclosure evidence: %w", err)
	}
	defer rows.Close()

	var evidenceIDs []uuid.UUID
	for rows.Next() {
		var eid uuid.UUID
		if err := rows.Scan(&eid); err != nil {
			return Disclosure{}, fmt.Errorf("scan evidence id: %w", err)
		}
		evidenceIDs = append(evidenceIDs, eid)
	}
	if err := rows.Err(); err != nil {
		return Disclosure{}, fmt.Errorf("iterate evidence ids: %w", err)
	}

	if len(evidenceIDs) == 0 {
		evidenceIDs = []uuid.UUID{evidenceID}
	}
	d.EvidenceIDs = evidenceIDs

	return d, nil
}

// FindByCase returns distinct disclosure batches for a case.
func (r *PGRepository) FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error) {
	page = ClampPagination(page)

	// Count distinct batches
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT (disclosed_by, disclosed_at)) FROM disclosures WHERE case_id = $1`,
		caseID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count disclosures: %w", err)
	}

	// Get distinct batches
	conditions := []string{"case_id = $1"}
	args := []any{caseID}
	argIdx := 2

	if page.Cursor != "" {
		cursorID, err := decodeCursor(page.Cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("id < $%d", argIdx))
		args = append(args, cursorID)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	args = append(args, page.Limit)

	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT DISTINCT ON (disclosed_by, disclosed_at)
			id, case_id, disclosed_to, disclosed_by, disclosed_at, notes, redacted
			FROM disclosures %s
			ORDER BY disclosed_by, disclosed_at DESC, id DESC
			LIMIT $%d`, where, argIdx),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query disclosures: %w", err)
	}
	defer rows.Close()

	var disclosures []Disclosure
	for rows.Next() {
		var d Disclosure
		if err := rows.Scan(&d.ID, &d.CaseID, &d.DisclosedTo, &d.DisclosedBy, &d.DisclosedAt, &d.Notes, &d.Redacted); err != nil {
			return nil, 0, fmt.Errorf("scan disclosure: %w", err)
		}
		d.EvidenceIDs = []uuid.UUID{} // Will be populated if needed
		disclosures = append(disclosures, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate disclosures: %w", err)
	}

	// For each batch, load evidence IDs (no defer in loop; explicit close)
	for i, d := range disclosures {
		eRows, err := r.pool.Query(ctx,
			`SELECT evidence_id FROM disclosures WHERE case_id = $1 AND disclosed_by = $2 AND disclosed_at = $3`,
			d.CaseID, d.DisclosedBy, d.DisclosedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("load disclosure evidence: %w", err)
		}

		var eids []uuid.UUID
		for eRows.Next() {
			var eid uuid.UUID
			if err := eRows.Scan(&eid); err != nil {
				eRows.Close()
				return nil, 0, fmt.Errorf("scan disclosure evidence id: %w", err)
			}
			eids = append(eids, eid)
		}
		eRows.Close()
		if err := eRows.Err(); err != nil {
			return nil, 0, fmt.Errorf("iterate disclosure evidence: %w", err)
		}

		if eids == nil {
			eids = []uuid.UUID{}
		}
		disclosures[i].EvidenceIDs = eids
	}

	return disclosures, total, nil
}

func (r *PGRepository) EvidenceBelongsToCase(ctx context.Context, caseID uuid.UUID, evidenceIDs []uuid.UUID) (bool, error) {
	if len(evidenceIDs) == 0 {
		return false, nil
	}

	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM evidence_items WHERE case_id = $1 AND id = ANY($2) AND destroyed_at IS NULL`,
		caseID, evidenceIDs,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check evidence belongs to case: %w", err)
	}

	return count == len(evidenceIDs), nil
}

func decodeCursor(cursor string) (uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode cursor: %w", err)
	}
	return uuid.Parse(string(decoded))
}

func encodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id.String()))
}
