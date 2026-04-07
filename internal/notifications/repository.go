package notifications

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("notification not found")

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository handles persistence for notifications.
type Repository struct {
	pool dbPool
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new notification.
func (r *Repository) Create(ctx context.Context, n Notification) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notifications (id, case_id, user_id, type, title, body, read, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		n.ID, n.CaseID, n.UserID, n.Type, n.Title, n.Body, n.Read, n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// ListByUser returns paginated notifications for a user, ordered by created_at DESC.
// Returns the notifications, total count, and any error.
func (r *Repository) ListByUser(ctx context.Context, userID string, limit int, cursor string) ([]Notification, int, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	args := []any{userID}
	argIdx := 2

	cursorClause := ""
	if cursor != "" {
		ts, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid cursor: %w", err)
		}
		cursorClause = fmt.Sprintf(
			" AND (created_at < $%d OR (created_at = $%d AND id < $%d))",
			argIdx, argIdx, argIdx+1,
		)
		args = append(args, ts, id)
		argIdx += 2
	}

	// Total count (without cursor).
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1`,
		userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	query := fmt.Sprintf(
		`SELECT id, case_id, user_id, type, title, body, read, created_at
		 FROM notifications
		 WHERE user_id = $1%s
		 ORDER BY created_at DESC, id DESC
		 LIMIT $%d`,
		cursorClause, argIdx,
	)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var items []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.CaseID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.Read, &n.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}

	// Trim the extra item used for has-more detection; callers use len vs limit.
	if len(items) > limit {
		items = items[:limit]
	}

	return items, total, nil
}

// MarkRead sets a single notification as read, scoped to the owning user.
func (r *Repository) MarkRead(ctx context.Context, id uuid.UUID, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead sets all unread notifications as read for a user.
func (r *Repository) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = true WHERE user_id = $1 AND read = false`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// GetUnreadCount returns the number of unread notifications for a user.
func (r *Repository) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

// GetCaseUserIDs returns all user IDs assigned to a case via the case_roles table.
func (r *Repository) GetCaseUserIDs(ctx context.Context, caseID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT user_id FROM case_roles WHERE case_id = $1`,
		caseID,
	)
	if err != nil {
		return nil, fmt.Errorf("get case user IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan case user ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case user IDs: %w", err)
	}
	return ids, nil
}

// decodeCursor parses a base64-encoded cursor into a (timestamp, uuid) pair.
func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decode cursor base64: %w", err)
	}

	// Format: RFC3339Nano + "|" + UUID
	parts := splitCursor(string(raw))
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("malformed cursor")
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor time: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse cursor UUID: %w", err)
	}

	return ts, id, nil
}

func splitCursor(s string) []string {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '|' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

// EncodeCursor produces a pagination cursor from a notification's timestamp and ID.
func EncodeCursor(ts time.Time, id uuid.UUID) string {
	raw := ts.Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
