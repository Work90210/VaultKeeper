package cases

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeRow implements pgx.Row with a configurable error.
type fakeRow struct{ err error }

func (r *fakeRow) Scan(_ ...any) error { return r.err }

// fakeRows implements pgx.Rows with configurable errors.
type fakeRows struct {
	called  int
	scanErr error
	errVal  error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.errVal }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeRows) Next() bool {
	if r.called > 0 {
		return false
	}
	r.called++
	return r.scanErr != nil || r.errVal != nil // produce one row if we want to trigger scan/err
}

func (r *fakeRows) Scan(_ ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	return nil
}

func (r *fakeRows) Values() ([]any, error) { return nil, nil }

// fakePool implements dbPool for error injection.
type fakePool struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (p *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &fakeRow{err: fmt.Errorf("not implemented")}
}

func (p *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if p.queryFn != nil {
		return p.queryFn(ctx, sql, args...)
	}
	return nil, fmt.Errorf("not implemented")
}

func (p *fakePool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if p.execFn != nil {
		return p.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, fmt.Errorf("not implemented")
}

// TestFindAll_QueryError covers L162 (pool.Query returns error).
func TestFindAll_QueryError(t *testing.T) {
	callCount := 0
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// Count query succeeds
			return &fakeRow{err: nil}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query failed")
		},
	}
	// Override queryRowFn to actually return a valid count
	pool.queryRowFn = func(_ context.Context, sql string, args ...any) pgx.Row {
		callCount++
		return &countRow{val: 0}
	}

	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from Query")
	}
}

// countRow returns 0 from Scan for COUNT(*) queries.
type countRow struct{ val int }

func (r *countRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*int); ok {
			*p = r.val
		}
	}
	return nil
}

// TestFindAll_ScanError covers L172 (rows.Scan returns error).
func TestFindAll_ScanError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 1}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{scanErr: fmt.Errorf("scan failed")}, nil
		},
	}

	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from Scan")
	}
}

// TestFindAll_RowsErr covers L177 (rows.Err returns error).
func TestFindAll_RowsErr(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{errVal: fmt.Errorf("rows error"), scanErr: nil}, nil
		},
	}

	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from rows.Err")
	}
}

// TestRoleRepo_ListByCaseID_ScanError covers roles.go L80 (scan error).
func TestRoleRepo_ListByCaseID_ScanError(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{scanErr: fmt.Errorf("scan role failed")}, nil
		},
	}

	repo := &RoleRepository{pool: pool}
	_, err := repo.ListByCaseID(context.Background(), [16]byte{1})
	if err == nil {
		t.Fatal("expected error from Scan")
	}
}

// TestFindAll_CountError covers L148 (count query Scan error).
func TestFindAll_CountError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("count failed")}
		},
	}

	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from count query")
	}
}
