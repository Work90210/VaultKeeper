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
	tsa_status, tsa_retry_count, tsa_last_retry, exif_data, source, source_date,
	destroyed_at, destroyed_by, destroy_reason, created_at`

func scanEvidence(row pgx.Row) (EvidenceItem, error) {
	var e EvidenceItem
	err := row.Scan(
		&e.ID, &e.CaseID, &e.EvidenceNumber, &e.Filename, &e.OriginalName, &e.StorageKey,
		&e.ThumbnailKey, &e.MimeType, &e.SizeBytes, &e.SHA256Hash, &e.Classification,
		&e.Description, &e.Tags, &e.UploadedBy, &e.UploadedByName, &e.IsCurrent, &e.Version, &e.ParentID,
		&e.TSAToken, &e.TSAName, &e.TSATimestamp, &e.TSAStatus, &e.TSARetryCount,
		&e.TSALastRetry, &e.ExifData, &e.Source, &e.SourceDate,
		&e.DestroyedAt, &e.DestroyedBy, &e.DestroyReason,
		&e.CreatedAt,
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
			&e.TSALastRetry, &e.ExifData, &e.Source, &e.SourceDate,
			&e.DestroyedAt, &e.DestroyedBy, &e.DestroyReason,
			&e.CreatedAt,
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
		 tsa_timestamp, tsa_status, exif_data, source, source_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING %s`, evidenceColumns)

	row := r.pool.QueryRow(ctx, query,
		input.CaseID, input.EvidenceNumber, input.Filename, input.OriginalName,
		input.StorageKey, input.MimeType, input.SizeBytes, input.SHA256Hash,
		input.Classification, input.Description, input.Tags, input.UploadedBy,
		input.UploadedByName, input.TSAToken, input.TSAName, input.TSATimestamp, input.TSAStatus, input.ExifData,
		input.Source, input.SourceDate,
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
		args = append(args, "%"+filter.SearchQuery+"%")
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

	// Defence role: only show disclosed evidence
	joinClause := ""
	if filter.UserRole == "defence" {
		joinClause = " JOIN disclosures d ON d.evidence_id = e.id AND d.case_id = e.case_id"
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
	if updates.Tags != nil {
		sets = append(sets, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, updates.Tags)
		argIdx++
	}

	if len(sets) == 0 {
		return r.FindByID(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(
		`UPDATE evidence_items SET %s WHERE id = $%d AND destroyed_at IS NULL RETURNING %s`,
		strings.Join(sets, ", "), argIdx, evidenceColumns)

	e, err := scanEvidence(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	// Advisory lock per case for gap-free numbering
	lockID := int64(caseID[0])<<56 | int64(caseID[1])<<48 | int64(caseID[2])<<40 |
		int64(caseID[3])<<32 | int64(caseID[4])<<24 | int64(caseID[5])<<16 |
		int64(caseID[6])<<8 | int64(caseID[7])
	// Use a different lock namespace than custody (offset by 1)
	lockID = lockID ^ 0x4556_4944 // "EVID"

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
		`SELECT %s FROM evidence_items WHERE (id = $1 OR parent_id = $1) ORDER BY version ASC`,
		evidenceColumns)
	rows, err := r.pool.Query(ctx, query, rootID)
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

// ListByCaseForExport returns all current, non-destroyed evidence items for case export.
// When userRole is "defence", only disclosed evidence is returned.
func (r *PGRepository) ListByCaseForExport(ctx context.Context, caseID uuid.UUID, userRole string) ([]EvidenceItem, error) {
	joinClause := ""
	if userRole == "defence" {
		joinClause = " INNER JOIN disclosures d ON d.evidence_id = e.id AND d.case_id = e.case_id"
	}

	query := fmt.Sprintf(`SELECT DISTINCT %s
		FROM evidence_items e%s
		WHERE e.case_id = $1 AND e.is_current = true AND e.destroyed_at IS NULL
		ORDER BY e.created_at ASC`, prefixColumns("e", evidenceColumns), joinClause)

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

// prefixColumns adds a table alias prefix to a comma-separated column list.
func prefixColumns(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
