package search

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time interface compliance checks.
var (
	_ UserCaseIDsLoader  = (*PGCaseIDsLoader)(nil)
	_ UserCaseRolesLoader = (*PGCaseIDsLoader)(nil)
)

// --- mock infrastructure ---

// mockCaseDB implements caseLoaderDB for testing.
type mockCaseDB struct {
	queryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockCaseDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFunc(ctx, sql, args...)
}

// mockCaseRows implements pgx.Rows for case_loader tests.
type mockCaseRows struct {
	data    [][]any
	idx     int
	closed  bool
	scanErr error
	iterErr error
}

func (r *mockCaseRows) Close()                                      {}
func (r *mockCaseRows) Err() error                                  { return r.iterErr }
func (r *mockCaseRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("") }
func (r *mockCaseRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockCaseRows) RawValues() [][]byte                          { return nil }
func (r *mockCaseRows) Conn() *pgx.Conn                             { return nil }

func (r *mockCaseRows) Next() bool {
	if r.closed || r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return r.idx <= len(r.data)
}

func (r *mockCaseRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		if ptr, ok := d.(*string); ok {
			*ptr = row[i].(string)
		}
	}
	return nil
}

func (r *mockCaseRows) Values() ([]any, error) {
	if r.idx <= 0 || r.idx > len(r.data) {
		return nil, errors.New("no current row")
	}
	return r.data[r.idx-1], nil
}

// --- constructor tests ---

func TestNewCaseIDsLoader(t *testing.T) {
	loader := NewCaseIDsLoader(nil)
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
}

func TestNewCaseIDsLoader_WithPool(t *testing.T) {
	var pool *pgxpool.Pool
	loader := NewCaseIDsLoader(pool)
	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
}

// --- GetUserCaseIDs tests ---

func TestGetUserCaseIDs_Success(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data: [][]any{
					{"case-1"},
					{"case-2"},
					{"case-3"},
				},
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	ids, err := loader.GetUserCaseIDs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 case IDs, got %d", len(ids))
	}
	expected := []string{"case-1", "case-2", "case-3"}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("expected %q at index %d, got %q", expected[i], i, id)
		}
	}
}

func TestGetUserCaseIDs_Empty(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{data: nil}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	ids, err := loader.GetUserCaseIDs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 case IDs, got %d", len(ids))
	}
}

func TestGetUserCaseIDs_QueryError(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseIDs(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "query user case IDs") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestGetUserCaseIDs_ScanError(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data:    [][]any{{"case-1"}},
				scanErr: errors.New("scan failed"),
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseIDs(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "scan case ID") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestGetUserCaseIDs_RowsErr(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data:    nil,
				iterErr: errors.New("iteration failed"),
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseIDs(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "iterate user case IDs") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- GetUserCaseRoles tests ---

func TestGetUserCaseRoles_Success(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data: [][]any{
					{"case-1", "admin"},
					{"case-2", "viewer"},
				},
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	roles, err := loader.GetUserCaseRoles(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles["case-1"] != "admin" {
		t.Errorf("expected role 'admin' for case-1, got %q", roles["case-1"])
	}
	if roles["case-2"] != "viewer" {
		t.Errorf("expected role 'viewer' for case-2, got %q", roles["case-2"])
	}
}

func TestGetUserCaseRoles_Empty(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{data: nil}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	roles, err := loader.GetUserCaseRoles(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected 0 roles, got %d", len(roles))
	}
}

func TestGetUserCaseRoles_QueryError(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("connection refused")
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseRoles(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "query user case roles") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestGetUserCaseRoles_ScanError(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data:    [][]any{{"case-1", "admin"}},
				scanErr: errors.New("scan failed"),
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseRoles(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "scan case role") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestGetUserCaseRoles_RowsErr(t *testing.T) {
	db := &mockCaseDB{
		queryFunc: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockCaseRows{
				data:    nil,
				iterErr: errors.New("iteration failed"),
			}, nil
		},
	}

	loader := newCaseIDsLoaderFromDB(db)
	_, err := loader.GetUserCaseRoles(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "iterate user case roles") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}
