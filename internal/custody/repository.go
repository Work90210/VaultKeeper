package custody

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustodyReader interface {
	ListByEvidence(ctx context.Context, evidenceID uuid.UUID, limit int, cursor string) ([]Event, int, error)
	ListByCase(ctx context.Context, caseID uuid.UUID, limit int, cursor string) ([]Event, int, error)
}

type PGRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) Insert(ctx context.Context, e Event) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin custody tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Advisory lock per case to serialize hash chain writes
	lockID := int64(e.CaseID[0])<<56 | int64(e.CaseID[1])<<48 | int64(e.CaseID[2])<<40 | int64(e.CaseID[3])<<32 | int64(e.CaseID[4])<<24 | int64(e.CaseID[5])<<16 | int64(e.CaseID[6])<<8 | int64(e.CaseID[7])
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockID); err != nil {
		return fmt.Errorf("lock custody chain: %w", err)
	}

	// Get last hash for this case
	var previousHash string
	err = tx.QueryRow(ctx,
		`SELECT hash_value FROM custody_log WHERE case_id = $1 ORDER BY timestamp DESC, id DESC LIMIT 1`,
		e.CaseID,
	).Scan(&previousHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get last custody hash: %w", err)
	}

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	e.PreviousHash = previousHash
	e.HashValue = ComputeLogHash(previousHash, e)

	var evidenceID any = e.EvidenceID
	if e.EvidenceID == uuid.Nil {
		evidenceID = nil
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO custody_log (id, case_id, evidence_id, action, actor_user_id, detail, hash_value, previous_hash, timestamp)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.CaseID, evidenceID, e.Action, e.ActorUserID, e.Detail, e.HashValue, e.PreviousHash, e.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert custody log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit custody log: %w", err)
	}
	return nil
}

func (r *PGRepository) ListByCase(ctx context.Context, caseID uuid.UUID, limit int, cursor string) ([]Event, int, error) {
	return r.list(ctx, "case_id", caseID, limit, cursor)
}

func (r *PGRepository) ListByEvidence(ctx context.Context, evidenceID uuid.UUID, limit int, cursor string) ([]Event, int, error) {
	return r.list(ctx, "evidence_id", evidenceID, limit, cursor)
}

func (r *PGRepository) list(ctx context.Context, filterCol string, filterID uuid.UUID, limit int, cursor string) ([]Event, int, error) {
	if filterCol != "case_id" && filterCol != "evidence_id" {
		return nil, 0, fmt.Errorf("invalid filter column: %s", filterCol)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("%s = $%d", filterCol, argIdx))
	args = append(args, filterID)
	argIdx++

	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		cursorID, err := uuid.Parse(string(decoded))
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor UUID: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("id < $%d", argIdx))
		args = append(args, cursorID)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count (without cursor)
	countArgs := args[:1]
	var total int
	err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM custody_log WHERE %s = $1", filterCol),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count custody entries: %w", err)
	}

	args = append(args, limit+1)
	query := fmt.Sprintf(
		`SELECT id, case_id, evidence_id, action, actor_user_id, detail, hash_value, previous_hash, timestamp
		 FROM custody_log WHERE %s ORDER BY timestamp DESC, id DESC LIMIT $%d`,
		where, argIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query custody log: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CaseID, &e.EvidenceID, &e.Action, &e.ActorUserID,
			&e.Detail, &e.HashValue, &e.PreviousHash, &e.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("scan custody event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate custody events: %w", err)
	}

	if len(events) > limit {
		events = events[:limit]
	}

	return events, total, nil
}

func (r *PGRepository) ListAllByCase(ctx context.Context, caseID uuid.UUID) ([]Event, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, case_id, evidence_id, action, actor_user_id, detail, hash_value, previous_hash, timestamp
		 FROM custody_log WHERE case_id = $1 ORDER BY timestamp ASC, id ASC`,
		caseID,
	)
	if err != nil {
		return nil, fmt.Errorf("query all custody events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CaseID, &e.EvidenceID, &e.Action, &e.ActorUserID,
			&e.Detail, &e.HashValue, &e.PreviousHash, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan custody event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
