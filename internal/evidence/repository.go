package evidence

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

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
)

var ErrNotFound = errors.New("evidence not found")

// ErrConflict is returned by Update when an optimistic-concurrency guard
// (ExpectedClassification) fails — i.e. another writer changed the
// classification between the caller's read and the UPDATE. Callers should
// re-fetch the item and retry or abort, depending on their policy.
var ErrConflict = errors.New("evidence update conflict: row changed concurrently")

// Repository defines the evidence data access interface.
type Repository interface {
	Create(ctx context.Context, input CreateEvidenceInput) (EvidenceItem, error)
	FindByID(ctx context.Context, id uuid.UUID) (EvidenceItem, error)
	FindByCase(ctx context.Context, filter EvidenceFilter, page Pagination) ([]EvidenceItem, int, error)
	Update(ctx context.Context, id uuid.UUID, updates EvidenceUpdate) (EvidenceItem, error)
	MarkDestroyed(ctx context.Context, id uuid.UUID, reason, destroyedBy string) error
	FindByHash(ctx context.Context, caseID uuid.UUID, hash string) ([]EvidenceItem, error)
	FindPendingTSA(ctx context.Context, limit int) ([]integrity.PendingTSAItem, error)
	UpdateTSAResult(ctx context.Context, id uuid.UUID, token []byte, tsaName string, tsTime time.Time) error
	IncrementTSARetry(ctx context.Context, id uuid.UUID) error
	MarkTSAFailed(ctx context.Context, id uuid.UUID) error
	IncrementEvidenceCounter(ctx context.Context, caseID uuid.UUID) (int, error)
	UpdateThumbnailKey(ctx context.Context, id uuid.UUID, key string) error
	FindVersionHistory(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceItem, error)
	MarkPreviousVersions(ctx context.Context, parentID uuid.UUID) error
	UpdateVersionFields(ctx context.Context, id uuid.UUID, parentID uuid.UUID, version int) error
	MarkNonCurrent(ctx context.Context, id uuid.UUID) error

	// Tag taxonomy (Sprint 9 Step 5)
	ListDistinctTags(ctx context.Context, caseID uuid.UUID, prefix string, limit int) ([]string, error)
	RenameTagInCase(ctx context.Context, caseID uuid.UUID, oldTag, newTag string) (int64, error)
	MergeTagsInCase(ctx context.Context, caseID uuid.UUID, sources []string, target string) (int64, error)
	DeleteTagFromCase(ctx context.Context, caseID uuid.UUID, tag string) (int64, error)
}

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// PGRepository is the Postgres implementation of Repository.
type PGRepository struct {
	pool dbPool
}

// NewRepository creates a new Postgres evidence repository.
func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

const evidenceColumns = `id, case_id, evidence_number, filename, original_name, storage_key,
	thumbnail_key, mime_type, size_bytes, sha256_hash, classification, description,
	tags, uploaded_by, uploaded_by_name, is_current, version, parent_id, tsa_token, tsa_name, tsa_timestamp,
	tsa_status, tsa_retry_count, tsa_last_retry, exif_data, source, source_date, ex_parte_side,
	destroyed_at, destroyed_by, destroy_reason, created_at,
	redaction_name, redaction_purpose, redaction_area_count, redaction_author_id, redaction_finalized_at,
	retention_until, destruction_authority`

func scanEvidence(row pgx.Row) (EvidenceItem, error) {
	var e EvidenceItem
	err := row.Scan(
		&e.ID, &e.CaseID, &e.EvidenceNumber, &e.Filename, &e.OriginalName, &e.StorageKey,
		&e.ThumbnailKey, &e.MimeType, &e.SizeBytes, &e.SHA256Hash, &e.Classification,
		&e.Description, &e.Tags, &e.UploadedBy, &e.UploadedByName, &e.IsCurrent, &e.Version, &e.ParentID,
		&e.TSAToken, &e.TSAName, &e.TSATimestamp, &e.TSAStatus, &e.TSARetryCount,
		&e.TSALastRetry, &e.ExifData, &e.Source, &e.SourceDate, &e.ExParteSide,
		&e.DestroyedAt, &e.DestroyedBy, &e.DestroyReason,
		&e.CreatedAt,
		&e.RedactionName, &e.RedactionPurpose, &e.RedactionAreaCount, &e.RedactionAuthorID, &e.RedactionFinalizedAt,
		&e.RetentionUntil, &e.DestructionAuthority,
	)
	if e.Tags == nil {
		e.Tags = []string{}
	}
	return e, err
}

func scanEvidenceRows(rows pgx.Rows) ([]EvidenceItem, error) {
	var items []EvidenceItem
	for rows.Next() {
		var e EvidenceItem
		err := rows.Scan(
			&e.ID, &e.CaseID, &e.EvidenceNumber, &e.Filename, &e.OriginalName, &e.StorageKey,
			&e.ThumbnailKey, &e.MimeType, &e.SizeBytes, &e.SHA256Hash, &e.Classification,
			&e.Description, &e.Tags, &e.UploadedBy, &e.UploadedByName, &e.IsCurrent, &e.Version, &e.ParentID,
			&e.TSAToken, &e.TSAName, &e.TSATimestamp, &e.TSAStatus, &e.TSARetryCount,
			&e.TSALastRetry, &e.ExifData, &e.Source, &e.SourceDate, &e.ExParteSide,
			&e.DestroyedAt, &e.DestroyedBy, &e.DestroyReason,
			&e.CreatedAt,
			&e.RedactionName, &e.RedactionPurpose, &e.RedactionAreaCount, &e.RedactionAuthorID, &e.RedactionFinalizedAt,
			&e.RetentionUntil, &e.DestructionAuthority,
		)
		if err != nil {
			return nil, fmt.Errorf("scan evidence row: %w", err)
		}
		if e.Tags == nil {
			e.Tags = []string{}
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (r *PGRepository) Create(ctx context.Context, input CreateEvidenceInput) (EvidenceItem, error) {
	query := fmt.Sprintf(`INSERT INTO evidence_items
		(case_id, evidence_number, filename, original_name, storage_key, mime_type, size_bytes,
		 sha256_hash, classification, description, tags, uploaded_by, uploaded_by_name, tsa_token, tsa_name,
		 tsa_timestamp, tsa_status, exif_data, source, source_date, ex_parte_side,
		 redaction_name, redaction_purpose, redaction_area_count, redaction_author_id, redaction_finalized_at,
		 retention_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
		        $21, $22, $23, $24, $25, $26, $27)
		RETURNING %s`, evidenceColumns)

	row := r.pool.QueryRow(ctx, query,
		input.CaseID, input.EvidenceNumber, input.Filename, input.OriginalName,
		input.StorageKey, input.MimeType, input.SizeBytes, input.SHA256Hash,
		input.Classification, input.Description, input.Tags, input.UploadedBy,
		input.UploadedByName, input.TSAToken, input.TSAName, input.TSATimestamp, input.TSAStatus, input.ExifData,
		input.Source, input.SourceDate, input.ExParteSide,
		input.RedactionName, input.RedactionPurpose, input.RedactionAreaCount, input.RedactionAuthorID, input.RedactionFinalizedAt,
		input.RetentionUntil,
	)

	return scanEvidence(row)
}

func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	query := fmt.Sprintf(`SELECT %s FROM evidence_items WHERE id = $1`, evidenceColumns)
	e, err := scanEvidence(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvidenceItem{}, ErrNotFound
		}
		return EvidenceItem{}, fmt.Errorf("find evidence by id: %w", err)
	}
	return e, nil
}

// FindByIDActive fetches a non-destroyed evidence item by ID. It adds a
// `destroyed_at IS NULL` predicate so destroyed records are never surfaced
// through normal service paths. Use FindByIDIncludingDestroyed for GDPR or
// destruction flows that require idempotency on already-destroyed records.
func (r *PGRepository) FindByIDActive(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	query := fmt.Sprintf(`SELECT %s FROM evidence_items WHERE id = $1 AND destroyed_at IS NULL`, evidenceColumns)
	e, err := scanEvidence(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvidenceItem{}, ErrNotFound
		}
		return EvidenceItem{}, fmt.Errorf("find active evidence by id: %w", err)
	}
	return e, nil
}

// FindByIDIncludingDestroyed fetches an evidence item by ID with no
// case_id scope and no destroyed_at filter. Used only by internal GDPR
// and destruction flows that must locate a record regardless of its state.
// Do NOT use this from any path reachable by an unauthenticated or
// cross-case caller.
func (r *PGRepository) FindByIDIncludingDestroyed(ctx context.Context, id uuid.UUID) (EvidenceItem, error) {
	query := fmt.Sprintf(`SELECT %s FROM evidence_items WHERE id = $1`, evidenceColumns)
	e, err := scanEvidence(r.pool.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvidenceItem{}, ErrNotFound
		}
		return EvidenceItem{}, fmt.Errorf("find evidence by id (including destroyed): %w", err)
	}
	return e, nil
}

// FindByIDScoped fetches an active (not destroyed) evidence item and
// enforces that it belongs to the given case. Returns ErrNotFound when
// the ID does not exist, belongs to a different case, or has been
// destroyed. This is the secure variant that prevents cross-case IDOR.
func (r *PGRepository) FindByIDScoped(ctx context.Context, caseID, id uuid.UUID) (EvidenceItem, error) {
	query := fmt.Sprintf(`SELECT %s FROM evidence_items WHERE id = $1 AND case_id = $2 AND destroyed_at IS NULL`, evidenceColumns)
	e, err := scanEvidence(r.pool.QueryRow(ctx, query, id, caseID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvidenceItem{}, ErrNotFound
		}
		return EvidenceItem{}, fmt.Errorf("find evidence by id (scoped): %w", err)
	}
	return e, nil
}

// findCaseIDForEvidence returns only the case_id for an evidence item.
// Used internally to bootstrap case_id when the caller only has an
// evidence ID (e.g. the /api/evidence/{id} route which has no caseID
// in the URL). After obtaining the caseID, callers should validate
// membership before using it to scope further queries.
func (r *PGRepository) findCaseIDForEvidence(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var caseID uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, id).Scan(&caseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("find case_id for evidence %s: %w", id, err)
	}
	return caseID, nil
}

func (r *PGRepository) FindByCase(ctx context.Context, filter EvidenceFilter, page Pagination) ([]EvidenceItem, int, error) {
	page = ClampPagination(page)

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("e.case_id = $%d", argIdx))
	args = append(args, filter.CaseID)
	argIdx++

	if filter.CurrentOnly {
		conditions = append(conditions, "e.is_current = true")
		// Exclude redacted copies from the main grid — they're accessed through
		// the original's detail page or via disclosure to defence
		conditions = append(conditions, "NOT (e.tags @> '{redacted}')")
	}

	if !filter.IncludeDestroyed {
		conditions = append(conditions, "e.destroyed_at IS NULL")
	}

	if filter.Classification != "" {
		conditions = append(conditions, fmt.Sprintf("e.classification = $%d", argIdx))
		args = append(args, filter.Classification)
		argIdx++
	}

	if filter.MimeType != "" {
		conditions = append(conditions, fmt.Sprintf("e.mime_type ILIKE $%d", argIdx))
		args = append(args, filter.MimeType+"%")
		argIdx++
	}

	if len(filter.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("e.tags @> $%d", argIdx))
		args = append(args, filter.Tags)
		argIdx++
	}

	if filter.SearchQuery != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(e.filename ILIKE $%d OR e.original_name ILIKE $%d OR e.description ILIKE $%d)",
			argIdx, argIdx, argIdx))
		args = append(args, "%"+escapeLikePattern(filter.SearchQuery)+"%")
		argIdx++
	}

	if page.Cursor != "" {
		cursorID, err := decodeCursor(page.Cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("e.id < $%d", argIdx))
		args = append(args, cursorID)
		argIdx++
	}

	// Classification access filter (Sprint 9 Step 1). When UserRole is set,
	// apply the role-aware access matrix as a SQL fragment. Bypassed when
	// UserRole is empty (internal/system queries).
	if filter.ApplyAccessFilter() {
		if frag := buildClassificationAccessSQL(filter.UserRole); frag != "" {
			conditions = append(conditions, frag)
		}
	}

	// Defence role: only show disclosed evidence.
	// When a disclosure has redacted=true, prefer the redacted copy (parent_id = original)
	// over the original. Non-redacted disclosures show the original directly.
	joinClause := ""
	if filter.UserRole == "defence" {
		joinClause = ` JOIN disclosures d ON d.case_id = e.case_id AND (
			(d.redacted = false AND d.evidence_id = e.id) OR
			(d.redacted = true AND e.parent_id = d.evidence_id)
		)`
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count without cursor
	countConditions := conditions
	countArgs := args
	if page.Cursor != "" {
		countConditions = countConditions[:len(countConditions)-1]
		countArgs = countArgs[:len(countArgs)-1]
	}
	countWhere := "WHERE " + strings.Join(countConditions, " AND ")

	var total int
	err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(DISTINCT e.id) FROM evidence_items e%s %s", joinClause, countWhere),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count evidence: %w", err)
	}

	args = append(args, page.Limit+1)
	query := fmt.Sprintf(
		`SELECT DISTINCT %s FROM evidence_items e%s %s ORDER BY e.created_at DESC, e.id DESC LIMIT $%d`,
		prefixColumns("e", evidenceColumns), joinClause, where, argIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query evidence: %w", err)
	}
	defer rows.Close()

	items, err := scanEvidenceRows(rows)
	if err != nil {
		return nil, 0, err
	}

	if len(items) > page.Limit {
		items = items[:page.Limit]
	}

	return items, total, nil
}

func (r *PGRepository) Update(ctx context.Context, id uuid.UUID, updates EvidenceUpdate) (EvidenceItem, error) {
	var sets []string
	var args []any
	argIdx := 1

	if updates.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *updates.Description)
		argIdx++
	}
	if updates.Classification != nil {
		sets = append(sets, fmt.Sprintf("classification = $%d", argIdx))
		args = append(args, *updates.Classification)
		argIdx++
	}
	if updates.ExParteSide != nil {
		sets = append(sets, fmt.Sprintf("ex_parte_side = $%d", argIdx))
		args = append(args, *updates.ExParteSide)
		argIdx++
	} else if updates.ClearExParteSide {
		sets = append(sets, "ex_parte_side = NULL")
	}
	if updates.Tags != nil {
		sets = append(sets, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, updates.Tags)
		argIdx++
	}
	if updates.RetentionUntil != nil {
		sets = append(sets, fmt.Sprintf("retention_until = $%d", argIdx))
		args = append(args, *updates.RetentionUntil)
		argIdx++
	} else if updates.ClearRetentionUntil {
		sets = append(sets, "retention_until = NULL")
	}

	if len(sets) == 0 {
		return r.FindByID(ctx, id)
	}

	args = append(args, id)
	idIdx := argIdx
	argIdx++

	// Sprint 9 optimistic concurrency: when the caller provides
	// ExpectedClassification, require the current row to still carry that
	// classification. If another writer has changed it between the
	// service's prior-fetch and this UPDATE, 0 rows match and we return
	// ErrConflict so the caller can retry with a fresh read.
	where := fmt.Sprintf("id = $%d AND destroyed_at IS NULL", idIdx)
	if updates.ExpectedClassification != nil {
		args = append(args, *updates.ExpectedClassification)
		where += fmt.Sprintf(" AND classification = $%d", argIdx)
		argIdx++
	}

	query := fmt.Sprintf(
		`UPDATE evidence_items SET %s WHERE %s RETURNING %s`,
		strings.Join(sets, ", "), where, evidenceColumns)

	e, err := scanEvidence(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if updates.ExpectedClassification != nil {
				return EvidenceItem{}, ErrConflict
			}
			return EvidenceItem{}, ErrNotFound
		}
		return EvidenceItem{}, fmt.Errorf("update evidence: %w", err)
	}
	return e, nil
}

func (r *PGRepository) MarkDestroyed(ctx context.Context, id uuid.UUID, reason, destroyedBy string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET destroyed_at = now(), destroyed_by = $1, destroy_reason = $2
		 WHERE id = $3 AND destroyed_at IS NULL`,
		destroyedBy, reason, id)
	if err != nil {
		return fmt.Errorf("mark evidence destroyed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PGRepository) FindByHash(ctx context.Context, caseID uuid.UUID, hash string) ([]EvidenceItem, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM evidence_items WHERE case_id = $1 AND sha256_hash = $2 AND destroyed_at IS NULL`,
		evidenceColumns)
	rows, err := r.pool.Query(ctx, query, caseID, hash)
	if err != nil {
		return nil, fmt.Errorf("find evidence by hash: %w", err)
	}
	defer rows.Close()
	return scanEvidenceRows(rows)
}

func (r *PGRepository) FindPendingTSA(ctx context.Context, limit int) ([]integrity.PendingTSAItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, case_id, sha256_hash, tsa_retry_count, created_at
		 FROM evidence_items WHERE tsa_status = 'pending' AND destroyed_at IS NULL
		 AND created_at > now() - INTERVAL '24 hours'
		 ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("find pending TSA items: %w", err)
	}
	defer rows.Close()

	var items []integrity.PendingTSAItem
	for rows.Next() {
		var item integrity.PendingTSAItem
		if err := rows.Scan(&item.ID, &item.CaseID, &item.SHA256Hash, &item.RetryCount, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending TSA item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PGRepository) UpdateTSAResult(ctx context.Context, id uuid.UUID, token []byte, tsaName string, tsTime time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET tsa_token = $1, tsa_name = $2, tsa_timestamp = $3, tsa_status = 'stamped'
		 WHERE id = $4`,
		token, tsaName, tsTime, id)
	if err != nil {
		return fmt.Errorf("update TSA result: %w", err)
	}
	return nil
}

func (r *PGRepository) IncrementTSARetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET tsa_retry_count = tsa_retry_count + 1, tsa_last_retry = now()
		 WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("increment TSA retry: %w", err)
	}
	return nil
}

func (r *PGRepository) MarkTSAFailed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET tsa_status = 'failed' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark TSA failed: %w", err)
	}
	return nil
}

func (r *PGRepository) IncrementEvidenceCounter(ctx context.Context, caseID uuid.UUID) (int, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin evidence counter tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Advisory lock per case for gap-free numbering.
	// XOR-fold the full 16-byte UUID into a 64-bit lock ID to avoid
	// collisions that would occur by truncating to the first 8 bytes.
	lockID := int64(0)
	for i := 0; i < 8; i++ {
		lockID |= int64(caseID[i]^caseID[i+8]) << (56 - i*8)
	}
	// XOR with a namespace constant to keep this lock distinct from the
	// custody advisory lock which uses the same UUID.
	lockID ^= 0x4556_4944 // "EVID"

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockID); err != nil {
		return 0, fmt.Errorf("lock evidence counter: %w", err)
	}

	var counter int
	err = tx.QueryRow(ctx,
		`UPDATE cases SET evidence_counter = evidence_counter + 1 WHERE id = $1 RETURNING evidence_counter`,
		caseID,
	).Scan(&counter)
	if err != nil {
		return 0, fmt.Errorf("increment evidence counter: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit evidence counter: %w", err)
	}

	return counter, nil
}

func (r *PGRepository) UpdateThumbnailKey(ctx context.Context, id uuid.UUID, key string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET thumbnail_key = $1 WHERE id = $2`, key, id)
	if err != nil {
		return fmt.Errorf("update thumbnail key: %w", err)
	}
	return nil
}

func (r *PGRepository) FindVersionHistory(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceItem, error) {
	// Find the root parent
	e, err := r.FindByID(ctx, evidenceID)
	if err != nil {
		return nil, err
	}

	rootID := e.ID
	if e.ParentID != nil {
		rootID = *e.ParentID
	}

	query := fmt.Sprintf(
		`SELECT %s FROM evidence_items WHERE (id = $1 OR parent_id = $1) AND case_id = $2 ORDER BY version ASC`,
		evidenceColumns)
	rows, err := r.pool.Query(ctx, query, rootID, e.CaseID)
	if err != nil {
		return nil, fmt.Errorf("find version history: %w", err)
	}
	defer rows.Close()
	return scanEvidenceRows(rows)
}

func (r *PGRepository) UpdateVersionFields(ctx context.Context, id uuid.UUID, parentID uuid.UUID, version int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET parent_id = $1, version = $2, is_current = true WHERE id = $3`,
		parentID, version, id)
	if err != nil {
		return fmt.Errorf("update version fields: %w", err)
	}
	return nil
}

func (r *PGRepository) MarkPreviousVersions(ctx context.Context, parentID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET is_current = false WHERE id = $1 OR parent_id = $1`,
		parentID)
	if err != nil {
		return fmt.Errorf("mark previous versions: %w", err)
	}
	return nil
}

func (r *PGRepository) MarkNonCurrent(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET is_current = false WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark non-current: %w", err)
	}
	return nil
}

// ListByCaseForExport returns all current, non-destroyed evidence items for case export.
// When userRole is "defence", only disclosed evidence is returned.
// Sprint 9: the classification access matrix is applied identically to FindByCase.
func (r *PGRepository) ListByCaseForExport(ctx context.Context, caseID uuid.UUID, userRole string) ([]EvidenceItem, error) {
	joinClause := ""
	if userRole == "defence" {
		joinClause = " INNER JOIN disclosures d ON d.evidence_id = e.id AND d.case_id = e.case_id"
	}

	where := "WHERE e.case_id = $1 AND e.is_current = true AND e.destroyed_at IS NULL"
	if userRole != "" {
		if frag := buildClassificationAccessSQL(userRole); frag != "" {
			where += " AND " + frag
		}
	}

	query := fmt.Sprintf(`SELECT DISTINCT %s
		FROM evidence_items e%s
		%s
		ORDER BY e.created_at ASC`, prefixColumns("e", evidenceColumns), joinClause, where)

	rows, err := r.pool.Query(ctx, query, caseID)
	if err != nil {
		return nil, fmt.Errorf("list evidence for export: %w", err)
	}
	defer rows.Close()

	return scanEvidenceRows(rows)
}

// ListForVerification returns all current, non-destroyed evidence items in a case
// with only the fields needed for integrity verification.
func (r *PGRepository) ListForVerification(ctx context.Context, caseID uuid.UUID) ([]VerifiableItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, case_id, storage_key, sha256_hash, tsa_token, tsa_status, filename
		 FROM evidence_items
		 WHERE case_id = $1 AND is_current = true AND destroyed_at IS NULL
		 ORDER BY created_at ASC`,
		caseID)
	if err != nil {
		return nil, fmt.Errorf("list evidence for verification: %w", err)
	}
	defer rows.Close()

	var items []VerifiableItem
	for rows.Next() {
		var item VerifiableItem
		var storageKey *string
		if err := rows.Scan(&item.ID, &item.CaseID, &storageKey, &item.SHA256Hash, &item.TSAToken, &item.TSAStatus, &item.Filename); err != nil {
			return nil, fmt.Errorf("scan evidence for verification: %w", err)
		}
		if storageKey != nil {
			item.StorageKey = *storageKey
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// VerifiableItem is a minimal struct for integrity verification.
type VerifiableItem struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	StorageKey string
	SHA256Hash string
	TSAToken   []byte
	TSAStatus  string
	Filename   string
}

// FlagIntegrityWarning sets the integrity_warning flag to true for the given evidence item.
func (r *PGRepository) FlagIntegrityWarning(ctx context.Context, evidenceID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE evidence_items SET integrity_warning = true WHERE id = $1`, evidenceID)
	if err != nil {
		return fmt.Errorf("flag integrity warning: %w", err)
	}
	return nil
}

// TryAdvisoryLock implements integrity.AdvisoryLocker.
func (r *PGRepository) TryAdvisoryLock(ctx context.Context, lockID int64) (bool, error) {
	var acquired bool
	err := r.pool.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("try advisory lock: %w", err)
	}
	return acquired, nil
}

// ReleaseAdvisoryLock implements integrity.AdvisoryLocker.
func (r *PGRepository) ReleaseAdvisoryLock(ctx context.Context, lockID int64) error {
	_, err := r.pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, lockID)
	if err != nil {
		return fmt.Errorf("release advisory lock: %w", err)
	}
	return nil
}

func decodeCursor(cursor string) (uuid.UUID, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode cursor base64: %w", err)
	}
	id, err := uuid.Parse(string(decoded))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse cursor UUID: %w", err)
	}
	return id, nil
}

func encodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id.String()))
}

// --- Multi-draft CRUD ---

// CreateDraft creates a new named redaction draft for an evidence item.
func (r *PGRepository) CreateDraft(ctx context.Context, evidenceID, caseID uuid.UUID, name string, purpose RedactionPurpose, createdBy string) (RedactionDraft, error) {
	var d RedactionDraft
	err := r.pool.QueryRow(ctx,
		`INSERT INTO redaction_drafts (id, evidence_id, case_id, name, purpose, created_by, yjs_state, status, last_saved_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NULL, 'draft', NOW(), NOW())
		 RETURNING id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at`,
		uuid.New(), evidenceID, caseID, name, purpose, createdBy,
	).Scan(&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt)
	if err != nil {
		return RedactionDraft{}, fmt.Errorf("create redaction draft: %w", err)
	}
	return d, nil
}

// FindDraftByID loads a draft by ID, scoped to the given evidence item.
func (r *PGRepository) FindDraftByID(ctx context.Context, draftID, evidenceID uuid.UUID) (RedactionDraft, []byte, error) {
	var d RedactionDraft
	var yjsState []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at, yjs_state
		 FROM redaction_drafts WHERE id = $1 AND evidence_id = $2`,
		draftID, evidenceID,
	).Scan(&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt, &yjsState)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RedactionDraft{}, nil, ErrNotFound
		}
		return RedactionDraft{}, nil, fmt.Errorf("find draft by id: %w", err)
	}
	return d, yjsState, nil
}

// ListDrafts returns all non-discarded drafts for an evidence item.
func (r *PGRepository) ListDrafts(ctx context.Context, evidenceID uuid.UUID) ([]RedactionDraft, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at
		 FROM redaction_drafts WHERE evidence_id = $1 AND status != 'discarded'
		 ORDER BY last_saved_at DESC`,
		evidenceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()

	var drafts []RedactionDraft
	for rows.Next() {
		var d RedactionDraft
		if err := rows.Scan(&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan draft row: %w", err)
		}
		drafts = append(drafts, d)
	}
	if drafts == nil {
		drafts = []RedactionDraft{}
	}
	return drafts, rows.Err()
}

// UpdateDraft saves areas and optionally updates name/purpose on a draft.
// The evidenceID parameter prevents cross-evidence IDOR attacks.
func (r *PGRepository) UpdateDraft(ctx context.Context, draftID, evidenceID uuid.UUID, yjsState []byte, areaCount int, name *string, purpose *RedactionPurpose) (RedactionDraft, error) {
	var sets []string
	var args []any
	argIdx := 1

	sets = append(sets, fmt.Sprintf("yjs_state = $%d", argIdx))
	args = append(args, yjsState)
	argIdx++

	sets = append(sets, fmt.Sprintf("area_count = $%d", argIdx))
	args = append(args, areaCount)
	argIdx++

	sets = append(sets, "last_saved_at = NOW()")

	if name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *name)
		argIdx++
	}
	if purpose != nil {
		sets = append(sets, fmt.Sprintf("purpose = $%d", argIdx))
		args = append(args, *purpose)
		argIdx++
	}

	args = append(args, draftID)
	argIdx++
	args = append(args, evidenceID)
	query := fmt.Sprintf(
		`UPDATE redaction_drafts SET %s WHERE id = $%d AND evidence_id = $%d AND status = 'draft'
		 RETURNING id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at`,
		strings.Join(sets, ", "), argIdx-1, argIdx,
	)

	var d RedactionDraft
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt,
	)
	if err != nil {
		return RedactionDraft{}, fmt.Errorf("update draft: %w", err)
	}
	return d, nil
}

// DiscardDraft soft-deletes a draft (status → discarded).
// The evidenceID parameter prevents cross-evidence IDOR attacks.
func (r *PGRepository) DiscardDraft(ctx context.Context, draftID, evidenceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE redaction_drafts SET status = 'discarded' WHERE id = $1 AND evidence_id = $2 AND status = 'draft'`,
		draftID, evidenceID,
	)
	if err != nil {
		return fmt.Errorf("discard draft: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LockDraftForFinalize acquires a row-level lock on the draft within a transaction.
func (r *PGRepository) LockDraftForFinalize(ctx context.Context, tx pgx.Tx, draftID uuid.UUID) (RedactionDraft, []byte, error) {
	var d RedactionDraft
	var yjsState []byte
	err := tx.QueryRow(ctx,
		`SELECT id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at, yjs_state
		 FROM redaction_drafts WHERE id = $1 FOR UPDATE`,
		draftID,
	).Scan(&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt, &yjsState)
	if err != nil {
		return RedactionDraft{}, nil, fmt.Errorf("lock draft for finalize: %w", err)
	}
	return d, yjsState, nil
}

// MarkDraftApplied sets draft status to 'applied' within a transaction.
func (r *PGRepository) MarkDraftApplied(ctx context.Context, tx pgx.Tx, draftID, evidenceID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE redaction_drafts SET status = 'applied' WHERE id = $1 AND evidence_id = $2`,
		draftID, evidenceID,
	)
	if err != nil {
		return fmt.Errorf("mark draft applied: %w", err)
	}
	return nil
}

// ListFinalizedRedactions returns finalized redacted derivatives of an evidence item.
func (r *PGRepository) ListFinalizedRedactions(ctx context.Context, evidenceID uuid.UUID) ([]FinalizedRedaction, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, COALESCE(e.evidence_number, ''), COALESCE(e.redaction_name, ''),
		        COALESCE(e.redaction_purpose, 'internal_review'), COALESCE(e.redaction_area_count, 0),
		        COALESCE(e.uploaded_by_name, e.uploaded_by::text), COALESCE(e.redaction_finalized_at, e.created_at)
		 FROM evidence_items e
		 WHERE e.parent_id = $1 AND e.redaction_name IS NOT NULL
		 ORDER BY e.created_at DESC`,
		evidenceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list finalized redactions: %w", err)
	}
	defer rows.Close()

	var items []FinalizedRedaction
	for rows.Next() {
		var f FinalizedRedaction
		if err := rows.Scan(&f.ID, &f.EvidenceNumber, &f.Name, &f.Purpose, &f.AreaCount, &f.Author, &f.FinalizedAt); err != nil {
			return nil, fmt.Errorf("scan finalized redaction: %w", err)
		}
		items = append(items, f)
	}
	if items == nil {
		items = []FinalizedRedaction{}
	}
	return items, rows.Err()
}

// GetManagementView returns both finalized versions and active drafts for an evidence item.
// Uses a REPEATABLE READ transaction to guarantee a consistent point-in-time snapshot.
func (r *PGRepository) GetManagementView(ctx context.Context, evidenceID uuid.UUID) (RedactionManagementView, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return RedactionManagementView{}, fmt.Errorf("begin management view tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Run both inner queries on the transaction to honour REPEATABLE READ isolation.
	finalizedRows, err := tx.Query(ctx,
		`SELECT e.id, COALESCE(e.evidence_number, ''), COALESCE(e.redaction_name, ''),
		        COALESCE(e.redaction_purpose, 'internal_review'), COALESCE(e.redaction_area_count, 0),
		        COALESCE(e.uploaded_by_name, e.uploaded_by::text), COALESCE(e.redaction_finalized_at, e.created_at)
		 FROM evidence_items e
		 WHERE e.parent_id = $1 AND e.redaction_name IS NOT NULL
		 ORDER BY e.created_at DESC`,
		evidenceID,
	)
	if err != nil {
		return RedactionManagementView{}, fmt.Errorf("list finalized redactions: %w", err)
	}
	defer finalizedRows.Close()

	var finalized []FinalizedRedaction
	for finalizedRows.Next() {
		var f FinalizedRedaction
		if err := finalizedRows.Scan(&f.ID, &f.EvidenceNumber, &f.Name, &f.Purpose, &f.AreaCount, &f.Author, &f.FinalizedAt); err != nil {
			return RedactionManagementView{}, fmt.Errorf("scan finalized redaction: %w", err)
		}
		finalized = append(finalized, f)
	}
	if err := finalizedRows.Err(); err != nil {
		return RedactionManagementView{}, fmt.Errorf("iterate finalized redactions: %w", err)
	}
	finalizedRows.Close()
	if finalized == nil {
		finalized = []FinalizedRedaction{}
	}

	draftRows, err := tx.Query(ctx,
		`SELECT id, evidence_id, case_id, name, purpose, area_count, created_by, status, last_saved_at, created_at
		 FROM redaction_drafts WHERE evidence_id = $1 AND status != 'discarded'
		 ORDER BY last_saved_at DESC`,
		evidenceID,
	)
	if err != nil {
		return RedactionManagementView{}, fmt.Errorf("list drafts: %w", err)
	}
	defer draftRows.Close()

	var drafts []RedactionDraft
	for draftRows.Next() {
		var d RedactionDraft
		if err := draftRows.Scan(&d.ID, &d.EvidenceID, &d.CaseID, &d.Name, &d.Purpose, &d.AreaCount, &d.CreatedBy, &d.Status, &d.LastSavedAt, &d.CreatedAt); err != nil {
			return RedactionManagementView{}, fmt.Errorf("scan draft row: %w", err)
		}
		drafts = append(drafts, d)
	}
	if err := draftRows.Err(); err != nil {
		return RedactionManagementView{}, fmt.Errorf("iterate drafts: %w", err)
	}
	draftRows.Close()
	if drafts == nil {
		drafts = []RedactionDraft{}
	}

	if err := tx.Commit(ctx); err != nil {
		return RedactionManagementView{}, fmt.Errorf("commit management view tx: %w", err)
	}

	return RedactionManagementView{
		Finalized: finalized,
		Drafts:    drafts,
	}, nil
}

// CheckEvidenceNumberExists checks if an evidence number is already in use.
func (r *PGRepository) CheckEvidenceNumberExists(ctx context.Context, number string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM evidence_items WHERE evidence_number = $1)`,
		number,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check evidence number exists: %w", err)
	}
	return exists, nil
}

// CreateWithTx creates an evidence item within an existing transaction.
func (r *PGRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, input CreateEvidenceInput) (EvidenceItem, error) {
	query := fmt.Sprintf(`INSERT INTO evidence_items
		(case_id, evidence_number, filename, original_name, storage_key, mime_type, size_bytes,
		 sha256_hash, classification, description, tags, uploaded_by, uploaded_by_name, tsa_token, tsa_name,
		 tsa_timestamp, tsa_status, exif_data, source, source_date, ex_parte_side,
		 redaction_name, redaction_purpose, redaction_area_count, redaction_author_id, redaction_finalized_at,
		 retention_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
		        $21, $22, $23, $24, $25, $26, $27)
		RETURNING %s`, evidenceColumns)

	row := tx.QueryRow(ctx, query,
		input.CaseID, input.EvidenceNumber, input.Filename, input.OriginalName,
		input.StorageKey, input.MimeType, input.SizeBytes, input.SHA256Hash,
		input.Classification, input.Description, input.Tags, input.UploadedBy,
		input.UploadedByName, input.TSAToken, input.TSAName, input.TSATimestamp, input.TSAStatus, input.ExifData,
		input.Source, input.SourceDate, input.ExParteSide,
		input.RedactionName, input.RedactionPurpose, input.RedactionAreaCount, input.RedactionAuthorID, input.RedactionFinalizedAt,
		input.RetentionUntil,
	)

	return scanEvidence(row)
}

// MarkNonCurrentWithTx marks an evidence item as non-current within a transaction.
func (r *PGRepository) MarkNonCurrentWithTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE evidence_items SET is_current = false WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark non-current (tx): %w", err)
	}
	return nil
}

// UpdateVersionFieldsWithTx sets parent_id and version within a transaction.
func (r *PGRepository) UpdateVersionFieldsWithTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, parentID uuid.UUID, version int) error {
	_, err := tx.Exec(ctx,
		`UPDATE evidence_items SET parent_id = $1, version = $2, is_current = true WHERE id = $3`,
		parentID, version, id)
	if err != nil {
		return fmt.Errorf("update version fields (tx): %w", err)
	}
	return nil
}

// SetDerivativeParentWithTx sets parent_id and marks a derivative as non-current in a single statement.
func (r *PGRepository) SetDerivativeParentWithTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, parentID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE evidence_items SET parent_id = $1, version = 1, is_current = false WHERE id = $2`,
		parentID, id)
	if err != nil {
		return fmt.Errorf("set derivative parent: %w", err)
	}
	return nil
}

// Tag repository methods (ListDistinctTags, RenameTagInCase,
// MergeTagsInCase, DeleteTagFromCase, withCaseTagLock, tagAdvisoryLockID,
// escapeLikePattern) moved to tag_repository.go as part of Sprint 9
// cleanup. Same package, same methods on *PGRepository — no import or
// interface changes required.

// FindExpiringRetention returns non-destroyed items whose retention_until
// is still in the future AND lies on or before `before` (the upcoming
// expiry window). Items already past their retention date are excluded so
// the daily notification job does not re-alert about them every run —
// they are actionable now and belong in a separate "ready-to-destroy"
// query rather than an "expiring soon" notification.
func (r *PGRepository) FindExpiringRetention(ctx context.Context, before time.Time) ([]ExpiringRetentionItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, case_id, COALESCE(evidence_number, ''), retention_until
		 FROM evidence_items
		 WHERE destroyed_at IS NULL
		   AND retention_until IS NOT NULL
		   AND retention_until > now()
		   AND retention_until <= $1
		 ORDER BY retention_until ASC`,
		before,
	)
	if err != nil {
		return nil, fmt.Errorf("find expiring retention: %w", err)
	}
	defer rows.Close()

	var items []ExpiringRetentionItem
	for rows.Next() {
		var it ExpiringRetentionItem
		if err := rows.Scan(&it.EvidenceID, &it.CaseID, &it.EvidenceNumber, &it.RetentionUntil); err != nil {
			return nil, fmt.Errorf("scan expiring retention row: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetCaseRetention reads the case-level retention_until floor.
func (r *PGRepository) GetCaseRetention(ctx context.Context, caseID uuid.UUID) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT retention_until FROM cases WHERE id = $1`,
		caseID,
	).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get case retention: %w", err)
	}
	return t, nil
}

// DestroyWithAuthority clears the storage key, records destroyed_at,
// destroyed_by, and destruction_authority in a single UPDATE. Returns
// ErrNotFound if the row is absent or already destroyed.
func (r *PGRepository) DestroyWithAuthority(ctx context.Context, id uuid.UUID, authority, actorID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE evidence_items
		 SET storage_key = NULL,
		     destroyed_at = now(),
		     destroyed_by = $1,
		     destruction_authority = $2
		 WHERE id = $3 AND destroyed_at IS NULL`,
		actorID, authority, id,
	)
	if err != nil {
		return fmt.Errorf("destroy with authority: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DestroyWithLegalHoldCheck atomically verifies that the owning case is NOT
// under legal hold and then marks the evidence item as destroyed — all inside
// a single serializable transaction. The SELECT ... FOR SHARE on the cases row
// means a concurrent SetLegalHold(true) must wait until this transaction
// commits or rolls back, closing the TOCTOU window that exists when the check
// and the destruction are two separate statements.
//
// Returns ErrLegalHoldActive if the case is under hold (transaction is rolled
// back). Returns ErrNotFound if the evidence row is absent or already
// destroyed.
func (r *PGRepository) DestroyWithLegalHoldCheck(ctx context.Context, id uuid.UUID, caseID uuid.UUID, authority, actorID string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Acquire a shared lock on the cases row. This prevents a concurrent
	// UPDATE cases SET legal_hold = true from committing until we're done.
	var legalHold bool
	err = tx.QueryRow(ctx,
		`SELECT legal_hold FROM cases WHERE id = $1 FOR SHARE`, caseID,
	).Scan(&legalHold)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("destroy with legal hold check: case not found")
		}
		return fmt.Errorf("destroy with legal hold check: read legal hold: %w", err)
	}
	if legalHold {
		err = ErrLegalHoldActive
		return err
	}

	// Legal hold is clear — destroy the evidence item in the same transaction.
	var tag pgconn.CommandTag
	tag, err = tx.Exec(ctx,
		`UPDATE evidence_items
		 SET storage_key = NULL,
		     destroyed_at = now(),
		     destroyed_by = $1,
		     destruction_authority = $2
		 WHERE id = $3 AND destroyed_at IS NULL`,
		actorID, authority, id,
	)
	if err != nil {
		return fmt.Errorf("destroy with legal hold check: update evidence: %w", err)
	}
	if tag.RowsAffected() == 0 {
		err = ErrNotFound
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("destroy with legal hold check: commit: %w", err)
	}
	return nil
}

// prefixColumns adds a table alias prefix to a comma-separated column list.
func prefixColumns(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
