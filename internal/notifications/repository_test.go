package notifications

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Mock pool for repository tests ---

type repoMockPool struct {
	queryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFunc     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (p *repoMockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFunc != nil {
		return p.queryRowFunc(ctx, sql, args...)
	}
	return &pgxRow{scanFunc: func(_ ...any) error { return nil }}
}

func (p *repoMockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFunc != nil {
		return p.queryFunc(ctx, sql, args...)
	}
	return &pgxRows{}, nil
}

func (p *repoMockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if p.execFunc != nil {
		return p.execFunc(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

// --- NewRepository test ---

func TestNewRepository(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// --- Create tests ---

func TestRepository_Create(t *testing.T) {
	var capturedArgs []any
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			capturedArgs = args
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}
	repo := &Repository{pool: pool}

	n := Notification{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Type:      EventEvidenceUploaded,
		Title:     "Test",
		Body:      "Test body",
		Read:      false,
		CreatedAt: time.Now().UTC(),
	}

	err := repo.Create(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) != 8 {
		t.Errorf("expected 8 args, got %d", len(capturedArgs))
	}
}

func TestRepository_Create_Error(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("insert failed")
		},
	}
	repo := &Repository{pool: pool}

	err := repo.Create(context.Background(), Notification{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- ListByUser tests ---

func TestRepository_ListByUser_DefaultLimit(t *testing.T) {
	now := time.Now().UTC()
	id1 := uuid.New()
	uid := uuid.New()

	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 1
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{
				data: [][]any{
					{id1, (*uuid.UUID)(nil), uid, EventEvidenceUploaded, "Title", "Body", false, now},
				},
			}, nil
		},
	}
	repo := &Repository{pool: pool}

	items, total, err := repo.ListByUser(context.Background(), uid.String(), 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestRepository_ListByUser_WithCursor(t *testing.T) {
	now := time.Now().UTC()
	cursorID := uuid.New()
	cursor := EncodeCursor(now, cursorID)

	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 10
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			// Verify cursor args are passed
			if len(args) < 4 {
				return nil, fmt.Errorf("expected at least 4 args with cursor, got %d", len(args))
			}
			return &pgxRows{}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, total, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, cursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 10 {
		t.Errorf("expected total 10, got %d", total)
	}
}

func TestRepository_ListByUser_InvalidCursor(t *testing.T) {
	pool := &repoMockPool{}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, "invalid-cursor")
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}

func TestRepository_ListByUser_LimitClamping(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 0
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			// Last arg is limit+1; for limit > 100 clamped to 100, so last arg should be 101
			lastArg := args[len(args)-1]
			if v, ok := lastArg.(int); ok && v != 101 {
				// When limit is 200, should be clamped to 100, so limit+1 = 101
				return nil, fmt.Errorf("expected clamped limit+1=101, got %d", v)
			}
			return &pgxRows{}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 200, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_ListByUser_CountError(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(_ ...any) error {
				return errors.New("count failed")
			}}
		},
	}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, "")
	if err == nil {
		t.Fatal("expected error from count query")
	}
}

func TestRepository_ListByUser_QueryError(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 0
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
	}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, "")
	if err == nil {
		t.Fatal("expected error from query")
	}
}

func TestRepository_ListByUser_ScanError(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 1
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{
				data:    [][]any{{}},
				scanErr: errors.New("scan failed"),
			}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, "")
	if err == nil {
		t.Fatal("expected error from scan")
	}
}

func TestRepository_ListByUser_IterError(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 0
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{iterErr: errors.New("iteration failed")}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, _, err := repo.ListByUser(context.Background(), uuid.New().String(), 10, "")
	if err == nil {
		t.Fatal("expected error from iteration")
	}
}

func TestRepository_ListByUser_TrimExtraItem(t *testing.T) {
	now := time.Now().UTC()
	uid := uuid.New()

	// Return 3 items when limit is 2 (limit+1 = 3 requested)
	rows := make([][]any, 3)
	for i := 0; i < 3; i++ {
		rows[i] = []any{uuid.New(), (*uuid.UUID)(nil), uid, "test", "Title", "Body", false, now}
	}

	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 10
				}
				return nil
			}}
		},
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{data: rows}, nil
		},
	}
	repo := &Repository{pool: pool}

	items, _, err := repo.ListByUser(context.Background(), uid.String(), 2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items (trimmed), got %d", len(items))
	}
}

// --- MarkRead tests ---

func TestRepository_MarkRead(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &Repository{pool: pool}

	err := repo.MarkRead(context.Background(), uuid.New(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_MarkRead_NotFound(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &Repository{pool: pool}

	err := repo.MarkRead(context.Background(), uuid.New(), uuid.New().String())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRepository_MarkRead_Error(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("update failed")
		},
	}
	repo := &Repository{pool: pool}

	err := repo.MarkRead(context.Background(), uuid.New(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- MarkAllRead tests ---

func TestRepository_MarkAllRead(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 5"), nil
		},
	}
	repo := &Repository{pool: pool}

	err := repo.MarkAllRead(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepository_MarkAllRead_Error(t *testing.T) {
	pool := &repoMockPool{
		execFunc: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(""), errors.New("update failed")
		},
	}
	repo := &Repository{pool: pool}

	err := repo.MarkAllRead(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- GetUnreadCount tests ---

func TestRepository_GetUnreadCount(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(dest ...any) error {
				if ptr, ok := dest[0].(*int); ok {
					*ptr = 42
				}
				return nil
			}}
		},
	}
	repo := &Repository{pool: pool}

	count, err := repo.GetUnreadCount(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestRepository_GetUnreadCount_Error(t *testing.T) {
	pool := &repoMockPool{
		queryRowFunc: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &pgxRow{scanFunc: func(_ ...any) error {
				return errors.New("count failed")
			}}
		},
	}
	repo := &Repository{pool: pool}

	_, err := repo.GetUnreadCount(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- GetCaseUserIDs tests ---

func TestRepository_GetCaseUserIDs(t *testing.T) {
	pool := &repoMockPool{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{
				data: [][]any{
					{"user-1"},
					{"user-2"},
				},
			}, nil
		},
	}
	repo := &Repository{pool: pool}

	ids, err := repo.GetCaseUserIDs(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
}

func TestRepository_GetCaseUserIDs_QueryError(t *testing.T) {
	pool := &repoMockPool{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query failed")
		},
	}
	repo := &Repository{pool: pool}

	_, err := repo.GetCaseUserIDs(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_GetCaseUserIDs_ScanError(t *testing.T) {
	pool := &repoMockPool{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{
				data:    [][]any{{"user-1"}},
				scanErr: errors.New("scan failed"),
			}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, err := repo.GetCaseUserIDs(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from scan")
	}
}

func TestRepository_GetCaseUserIDs_IterError(t *testing.T) {
	pool := &repoMockPool{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &pgxRows{iterErr: errors.New("iteration failed")}, nil
		},
	}
	repo := &Repository{pool: pool}

	_, err := repo.GetCaseUserIDs(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from iteration")
	}
}

// --- Cursor encoding/decoding tests ---

func TestEncodeCursor_DecodeCursor_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	id := uuid.New()

	encoded := EncodeCursor(now, id)
	ts, decodedID, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ts.Equal(now) {
		t.Errorf("timestamp mismatch: expected %v, got %v", now, ts)
	}
	if decodedID != id {
		t.Errorf("ID mismatch: expected %v, got %v", id, decodedID)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, _, err := decodeCursor("!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeCursor_MalformedContent(t *testing.T) {
	// Valid base64 but no pipe separator
	encoded := "bm9waXBl" // "nopipe" in base64
	_, _, err := decodeCursor(encoded)
	if err == nil {
		t.Fatal("expected error for malformed cursor (no pipe)")
	}
}

func TestDecodeCursor_InvalidTimestamp(t *testing.T) {
	// "bad-time|<valid-uuid>"
	raw := "bad-time|" + uuid.New().String()
	encoded := encodeRaw(raw)
	_, _, err := decodeCursor(encoded)
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestDecodeCursor_InvalidUUID(t *testing.T) {
	// "<valid-time>|bad-uuid"
	raw := time.Now().UTC().Format(time.RFC3339Nano) + "|not-a-uuid"
	encoded := encodeRaw(raw)
	_, _, err := decodeCursor(encoded)
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestSplitCursor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"no pipe", "abc", []string{"abc"}},
		{"one pipe", "abc|def", []string{"abc", "def"}},
		{"multiple pipes", "a|b|c", []string{"a|b", "c"}},
		{"pipe at start", "|abc", []string{"", "abc"}},
		{"pipe at end", "abc|", []string{"abc", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitCursor(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d parts, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("part %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func encodeRaw(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
