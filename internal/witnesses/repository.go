package witnesses

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("witness not found")

// Repository defines the witness data access interface.
type Repository interface {
	Create(ctx context.Context, w Witness) (Witness, error)
	FindByID(ctx context.Context, id uuid.UUID) (Witness, error)
	FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Witness, int, error)
	Update(ctx context.Context, id uuid.UUID, w Witness) (Witness, error)
	// FindAll returns all witnesses for administrative key rotation. Must only be called from admin-only paths.
	FindAll(ctx context.Context) ([]Witness, error)
	UpdateEncryptedFields(ctx context.Context, id uuid.UUID, fullName, contactInfo, location []byte) error
}

// scopedWitnessRepo is an optional extension satisfied by PGRepository.
// When the underlying repo implements this interface the service uses
// case-scoped SQL to prevent cross-case IDOR. Test fakes fall back to the
// unscoped Repository methods.
type scopedWitnessRepo interface {
	FindCaseID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	FindByIDScoped(ctx context.Context, caseID, id uuid.UUID) (Witness, error)
}

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PGRepository is the Postgres implementation.
type PGRepository struct {
	pool dbPool
}

// NewRepository creates a new Postgres witness repository.
// Note: this function requires a live *pgxpool.Pool and is exercised by
// integration tests only; unit tests use newTestRepo (test helper) instead.
func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

const witnessColumns = `id, case_id, witness_code, pseudonym, full_name_encrypted,
	contact_info_encrypted, location_encrypted, protection_status, statement_summary,
	related_evidence, judge_identity_visible, created_by, created_at, updated_at`

func scanWitness(row pgx.Row) (Witness, error) {
	var w Witness
	var pseudonym string
	var relatedEvidence []uuid.UUID
	err := row.Scan(
		&w.ID, &w.CaseID, &w.WitnessCode, &pseudonym, &w.FullNameEncrypted,
		&w.ContactInfoEncrypted, &w.LocationEncrypted, &w.ProtectionStatus,
		&w.StatementSummary, &relatedEvidence, &w.JudgeIdentityVisible,
		&w.CreatedBy, &w.CreatedAt, &w.UpdatedAt,
	)
	if relatedEvidence == nil {
		relatedEvidence = []uuid.UUID{}
	}
	w.RelatedEvidence = relatedEvidence
	return w, err
}

func scanWitnessRows(rows pgx.Rows) ([]Witness, error) {
	var items []Witness
	for rows.Next() {
		var w Witness
		var pseudonym string
		var relatedEvidence []uuid.UUID
		err := rows.Scan(
			&w.ID, &w.CaseID, &w.WitnessCode, &pseudonym, &w.FullNameEncrypted,
			&w.ContactInfoEncrypted, &w.LocationEncrypted, &w.ProtectionStatus,
			&w.StatementSummary, &relatedEvidence, &w.JudgeIdentityVisible,
			&w.CreatedBy, &w.CreatedAt, &w.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan witness row: %w", err)
		}
		if relatedEvidence == nil {
			relatedEvidence = []uuid.UUID{}
		}
		w.RelatedEvidence = relatedEvidence
		items = append(items, w)
	}
	return items, rows.Err()
}

func (r *PGRepository) Create(ctx context.Context, w Witness) (Witness, error) {
	query := fmt.Sprintf(`INSERT INTO witnesses
		(id, case_id, witness_code, pseudonym, full_name_encrypted, contact_info_encrypted,
		 location_encrypted, protection_status, statement_summary, related_evidence,
		 judge_identity_visible, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING %s`, witnessColumns)

	row := r.pool.QueryRow(ctx, query,
		w.ID, w.CaseID, w.WitnessCode, w.WitnessCode, w.FullNameEncrypted,
		w.ContactInfoEncrypted, w.LocationEncrypted, w.ProtectionStatus,
		w.StatementSummary, w.RelatedEvidence, w.JudgeIdentityVisible,
		w.CreatedBy,
	)

	return scanWitness(row)
}

func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (Witness, error) {
	query := fmt.Sprintf(`SELECT %s FROM witnesses WHERE id = $1`, witnessColumns)
	w, err := scanWitness(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Witness{}, ErrNotFound
		}
		return Witness{}, fmt.Errorf("find witness by id: %w", err)
	}
	return w, nil
}

// FindCaseID returns the case_id for a witness without scope filtering.
// Used to bootstrap the caseID before a scoped FindByID call.
func (r *PGRepository) FindCaseID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var caseID uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT case_id FROM witnesses WHERE id = $1`, id).Scan(&caseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("find case_id for witness %s: %w", id, err)
	}
	return caseID, nil
}

// FindByIDScoped returns a witness by ID scoped to the given case.
// Satisfies the scopedWitnessRepo interface for IDOR prevention.
func (r *PGRepository) FindByIDScoped(ctx context.Context, caseID, id uuid.UUID) (Witness, error) {
	query := fmt.Sprintf(`SELECT %s FROM witnesses WHERE id = $1 AND case_id = $2`, witnessColumns)
	w, err := scanWitness(r.pool.QueryRow(ctx, query, id, caseID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Witness{}, ErrNotFound
		}
		return Witness{}, fmt.Errorf("find witness by id scoped: %w", err)
	}
	return w, nil
}

func (r *PGRepository) FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Witness, int, error) {
	page = ClampPagination(page)

	var args []any
	args = append(args, caseID)
	conditions := []string{"case_id = $1"}
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

	// Count (without cursor)
	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM witnesses WHERE case_id = $1", caseID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count witnesses: %w", err)
	}

	args = append(args, page.Limit+1)
	query := fmt.Sprintf(
		`SELECT %s FROM witnesses %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		witnessColumns, where, argIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query witnesses: %w", err)
	}
	defer rows.Close()

	items, err := scanWitnessRows(rows)
	if err != nil {
		return nil, 0, err
	}

	if len(items) > page.Limit {
		items = items[:page.Limit]
	}

	return items, total, nil
}

func (r *PGRepository) Update(ctx context.Context, id uuid.UUID, w Witness) (Witness, error) {
	query := fmt.Sprintf(`UPDATE witnesses SET
		full_name_encrypted = $1, contact_info_encrypted = $2, location_encrypted = $3,
		protection_status = $4, statement_summary = $5, related_evidence = $6,
		judge_identity_visible = $7, updated_at = now()
		WHERE id = $8 AND case_id = $9
		RETURNING %s`, witnessColumns)

	result, err := scanWitness(r.pool.QueryRow(ctx, query,
		w.FullNameEncrypted, w.ContactInfoEncrypted, w.LocationEncrypted,
		w.ProtectionStatus, w.StatementSummary, w.RelatedEvidence,
		w.JudgeIdentityVisible, id, w.CaseID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Witness{}, ErrNotFound
		}
		return Witness{}, fmt.Errorf("update witness: %w", err)
	}
	return result, nil
}

func (r *PGRepository) FindAll(ctx context.Context) ([]Witness, error) {
	query := fmt.Sprintf(`SELECT %s FROM witnesses ORDER BY id ASC`, witnessColumns)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find all witnesses: %w", err)
	}
	defer rows.Close()
	return scanWitnessRows(rows)
}

// FindAllPaginated returns a batch of witnesses ordered by ID for use in
// administrative key rotation. Callers should iterate with increasing offsets
// until an empty slice is returned.
func (r *PGRepository) FindAllPaginated(ctx context.Context, limit, offset int) ([]Witness, error) {
	query := fmt.Sprintf(`SELECT %s FROM witnesses ORDER BY id ASC LIMIT $1 OFFSET $2`, witnessColumns)
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("find all witnesses paginated: %w", err)
	}
	defer rows.Close()
	return scanWitnessRows(rows)
}

// UpdateEncryptedFields updates the encrypted identity fields for a witness.
// Called exclusively from the admin-only key-rotation job (system admin required).
func (r *PGRepository) UpdateEncryptedFields(ctx context.Context, id uuid.UUID, fullName, contactInfo, location []byte) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE witnesses SET full_name_encrypted = $1, contact_info_encrypted = $2,
		 location_encrypted = $3, updated_at = now() WHERE id = $4`,
		fullName, contactInfo, location, id)
	if err != nil {
		return fmt.Errorf("update encrypted fields: %w", err)
	}
	return nil
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
