package cases

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

var ErrNotFound = errors.New("case not found")

type Repository interface {
	Create(ctx context.Context, c Case) (Case, error)
	FindByID(ctx context.Context, id uuid.UUID) (Case, error)
	FindAll(ctx context.Context, filter CaseFilter, page Pagination) ([]Case, int, error)
	Update(ctx context.Context, id uuid.UUID, updates UpdateCaseInput) (Case, error)
	Archive(ctx context.Context, id uuid.UUID) error
	SetLegalHold(ctx context.Context, id uuid.UUID, hold bool) error
	// CheckLegalHoldStrict returns the current legal_hold flag for a case via
	// a single targeted read (no full row fetch, no in-memory state shared
	// across calls). It is the minimum viable primitive for guarding
	// destructive mutations without the full TOCTOU window of FindByID → check
	// → act. True atomicity still requires the caller to run the destructive
	// mutation inside the same transaction (or hold a row-level lock); see
	// Service.EnsureNotOnHold for the documented contract.
	CheckLegalHoldStrict(ctx context.Context, id uuid.UUID) (bool, error)
}

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type PGRepository struct {
	pool dbPool
}

func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) Create(ctx context.Context, c Case) (Case, error) {
	var result Case
	err := r.pool.QueryRow(ctx,
		`INSERT INTO cases (organization_id, reference_code, title, description, jurisdiction, status, legal_hold, retention_until, created_by, created_by_name)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, organization_id, reference_code, title, description, jurisdiction, status, legal_hold, retention_until, created_by, created_by_name, created_at, updated_at`,
		c.OrganizationID, c.ReferenceCode, c.Title, c.Description, c.Jurisdiction, c.Status, c.LegalHold, c.RetentionUntil, c.CreatedBy, c.CreatedByName,
	).Scan(&result.ID, &result.OrganizationID, &result.ReferenceCode, &result.Title, &result.Description,
		&result.Jurisdiction, &result.Status, &result.LegalHold, &result.RetentionUntil, &result.CreatedBy, &result.CreatedByName,
		&result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return Case{}, fmt.Errorf("reference code already exists: %w", err)
		}
		return Case{}, fmt.Errorf("insert case: %w", err)
	}
	return result, nil
}

func (r *PGRepository) FindByID(ctx context.Context, id uuid.UUID) (Case, error) {
	var c Case
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, reference_code, title, description, jurisdiction, status, legal_hold, retention_until, created_by, created_by_name, created_at, updated_at
		 FROM cases WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.OrganizationID, &c.ReferenceCode, &c.Title, &c.Description,
		&c.Jurisdiction, &c.Status, &c.LegalHold, &c.RetentionUntil, &c.CreatedBy, &c.CreatedByName,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Case{}, ErrNotFound
		}
		return Case{}, fmt.Errorf("find case by id: %w", err)
	}
	return c, nil
}

func (r *PGRepository) FindAll(ctx context.Context, filter CaseFilter, page Pagination) ([]Case, int, error) {
	page = ClampPagination(page)

	var conditions []string
	var args []any
	argIdx := 1

	// Organization scope
	if filter.OrganizationID != "" {
		conditions = append(conditions, fmt.Sprintf("c.organization_id = $%d", argIdx))
		args = append(args, filter.OrganizationID)
		argIdx++
	}

	// Filter by user's case roles (non-admin only).
	// Org owners/admins see all org cases (handled by OrganizationID filter above).
	// Regular members only see cases they have a role on.
	if !filter.SystemAdmin && filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf(
			`(c.id IN (SELECT case_id FROM case_roles WHERE user_id = $%d)
			  OR c.organization_id IN (
			    SELECT organization_id FROM organization_memberships
			    WHERE user_id = $%d AND status = 'active' AND role IN ('owner','admin')
			  ))`, argIdx, argIdx))
		args = append(args, filter.UserID)
		argIdx++
	}

	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, s)
			argIdx++
		}
		conditions = append(conditions, fmt.Sprintf("c.status IN (%s)", strings.Join(placeholders, ",")))
	}

	if filter.Jurisdiction != "" {
		conditions = append(conditions, fmt.Sprintf("c.jurisdiction = $%d", argIdx))
		args = append(args, filter.Jurisdiction)
		argIdx++
	}

	if filter.SearchQuery != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(c.title ILIKE $%d OR c.reference_code ILIKE $%d OR c.description ILIKE $%d)",
			argIdx, argIdx, argIdx))
		args = append(args, "%"+filter.SearchQuery+"%")
		argIdx++
	}

	// Cursor-based pagination
	if page.Cursor != "" {
		cursorID, err := decodeCursor(page.Cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		conditions = append(conditions, fmt.Sprintf("c.id < $%d", argIdx))
		args = append(args, cursorID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total (without cursor and limit)
	countWhere := ""
	countConditions := conditions
	// Remove cursor condition from count
	if page.Cursor != "" {
		countConditions = countConditions[:len(countConditions)-1]
	}
	if len(countConditions) > 0 {
		countWhere = "WHERE " + strings.Join(countConditions, " AND ")
	}

	// Count args (without cursor arg)
	countArgs := args
	if page.Cursor != "" {
		countArgs = countArgs[:len(countArgs)-1]
	}

	var total int
	err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM cases c %s", countWhere),
		countArgs...,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count cases: %w", err)
	}

	// Fetch items
	query := fmt.Sprintf(
		`SELECT c.id, c.organization_id, c.reference_code, c.title, c.description, c.jurisdiction, c.status, c.legal_hold, c.retention_until, c.created_by, c.created_by_name, c.created_at, c.updated_at
		 FROM cases c %s
		 ORDER BY c.id DESC
		 LIMIT $%d`,
		where, argIdx)
	args = append(args, page.Limit+1) // fetch one extra to determine has_more

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query cases: %w", err)
	}
	defer rows.Close()

	var cases []Case
	for rows.Next() {
		var c Case
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.ReferenceCode, &c.Title, &c.Description,
			&c.Jurisdiction, &c.Status, &c.LegalHold, &c.RetentionUntil, &c.CreatedBy, &c.CreatedByName,
			&c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan case: %w", err)
		}
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate cases: %w", err)
	}

	hasMore := len(cases) > page.Limit
	if hasMore {
		cases = cases[:page.Limit]
	}

	return cases, total, nil
}

func (r *PGRepository) Update(ctx context.Context, id uuid.UUID, updates UpdateCaseInput) (Case, error) {
	var sets []string
	var args []any
	argIdx := 1

	if updates.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *updates.Title)
		argIdx++
	}
	if updates.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *updates.Description)
		argIdx++
	}
	if updates.Jurisdiction != nil {
		sets = append(sets, fmt.Sprintf("jurisdiction = $%d", argIdx))
		args = append(args, *updates.Jurisdiction)
		argIdx++
	}
	if updates.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *updates.Status)
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

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf(
		`UPDATE cases SET %s WHERE id = $%d
		 RETURNING id, organization_id, reference_code, title, description, jurisdiction, status, legal_hold, retention_until, created_by, created_by_name, created_at, updated_at`,
		strings.Join(sets, ", "), argIdx)

	var c Case
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.OrganizationID, &c.ReferenceCode, &c.Title, &c.Description,
		&c.Jurisdiction, &c.Status, &c.LegalHold, &c.RetentionUntil, &c.CreatedBy, &c.CreatedByName,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Case{}, ErrNotFound
		}
		return Case{}, fmt.Errorf("update case: %w", err)
	}
	return c, nil
}

func (r *PGRepository) Archive(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cases SET status = 'archived', updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("archive case: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CheckLegalHoldStrict performs a single-column read of legal_hold for the
// case. This is strictly stronger than FindByID-then-check because it does
// not share intermediate state with other service calls and touches only the
// one column that matters. Callers that need true atomicity must run this
// inside the same transaction as their destructive mutation and use a
// row-level lock (SELECT ... FOR SHARE) — that refactor is tracked for the
// destruction flow and intentionally out of scope here.
func (r *PGRepository) CheckLegalHoldStrict(ctx context.Context, id uuid.UUID) (bool, error) {
	var hold bool
	err := r.pool.QueryRow(ctx,
		`SELECT legal_hold FROM cases WHERE id = $1`, id,
	).Scan(&hold)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("check legal hold strict: %w", err)
	}
	return hold, nil
}

func (r *PGRepository) SetLegalHold(ctx context.Context, id uuid.UUID, hold bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE cases SET legal_hold = $1, updated_at = now() WHERE id = $2`, hold, id)
	if err != nil {
		return fmt.Errorf("set legal hold: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HasActiveCases checks if an org has any non-archived cases or any case with legal hold.
func (r *PGRepository) HasActiveCases(ctx context.Context, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM cases
			WHERE organization_id = $1
			  AND (status != 'archived' OR legal_hold = true)
		)`, orgID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check active cases: %w", err)
	}
	return exists, nil
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

func EncodeCursor(id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id.String()))
}
