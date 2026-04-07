package cases

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
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

// ---------------------------------------------------------------------------
// NewRepository — covers L37
// ---------------------------------------------------------------------------

func TestNewRepository_Unit(t *testing.T) {
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

func TestNewRoleRepository_Unit(t *testing.T) {
	repo := NewRoleRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil role repository")
	}
}

// ---------------------------------------------------------------------------
// Create — success + duplicate + general error via fakePool (L41-L58)
// ---------------------------------------------------------------------------

// scanningRow implements pgx.Row and scans predefined values into destinations.
type scanningRow struct {
	vals []any
	err  error
}

func (r *scanningRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i >= len(r.vals) {
			break
		}
		switch p := d.(type) {
		case *uuid.UUID:
			if v, ok := r.vals[i].(uuid.UUID); ok {
				*p = v
			}
		case *string:
			if v, ok := r.vals[i].(string); ok {
				*p = v
			}
		case *bool:
			if v, ok := r.vals[i].(bool); ok {
				*p = v
			}
		case *time.Time:
			if v, ok := r.vals[i].(time.Time); ok {
				*p = v
			}
		}
	}
	return nil
}

func TestPGRepo_Create_Success(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, "REF-001", "Title", "Desc", "Juris", "active", false, "user-1", "User One", now, now,
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	c, err := repo.Create(context.Background(), Case{
		ReferenceCode: "REF-001", Title: "Title", Description: "Desc",
		Jurisdiction: "Juris", Status: "active", CreatedBy: "user-1", CreatedByName: "User One",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID != id {
		t.Errorf("ID = %v, want %v", c.ID, id)
	}
}

func TestPGRepo_Create_DuplicateKey(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("duplicate key value violates unique constraint")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.Create(context.Background(), Case{ReferenceCode: "DUP-001"})
	if err == nil {
		t.Fatal("expected error for duplicate key")
	}
	if !strings.Contains(err.Error(), "reference code already exists") {
		t.Errorf("error = %q, want duplicate key message", err.Error())
	}
}

func TestPGRepo_Create_GeneralError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("connection reset")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.Create(context.Background(), Case{ReferenceCode: "ERR-001"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "insert case") {
		t.Errorf("error = %q, want 'insert case' prefix", err.Error())
	}
}

// ---------------------------------------------------------------------------
// FindByID — success + not found + general error (L60-L76)
// ---------------------------------------------------------------------------

func TestPGRepo_FindByID_Success(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, "REF-001", "Title", "Desc", "Juris", "active", false, "user-1", "User One", now, now,
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	c, err := repo.FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if c.ID != id {
		t.Errorf("ID = %v, want %v", c.ID, id)
	}
}

func TestPGRepo_FindByID_NotFound(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepo_FindByID_GeneralError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("connection refused")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "find case by id") {
		t.Errorf("error = %q, want 'find case by id' prefix", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Update — success, no fields, not found, general error (L196-L246)
// ---------------------------------------------------------------------------

func TestPGRepo_Update_Success(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, "REF-001", "New Title", "Desc", "Juris", "active", false, "user-1", "User One", now, now,
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	newTitle := "New Title"
	c, err := repo.Update(context.Background(), id, UpdateCaseInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if c.ID != id {
		t.Errorf("ID = %v, want %v", c.ID, id)
	}
}

func TestPGRepo_Update_NoFields(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, "REF-001", "Title", "Desc", "Juris", "active", false, "user-1", "User One", now, now,
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	c, err := repo.Update(context.Background(), id, UpdateCaseInput{})
	if err != nil {
		t.Fatalf("Update no fields: %v", err)
	}
	if c.ID != id {
		t.Errorf("ID = %v, want %v", c.ID, id)
	}
}

func TestPGRepo_Update_NotFound(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	newTitle := "Ghost"
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{Title: &newTitle})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepo_Update_GeneralError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("update failed")}
		},
	}
	repo := &PGRepository{pool: pool}
	newTitle := "X"
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{Title: &newTitle})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "update case") {
		t.Errorf("error = %q, want 'update case' prefix", err.Error())
	}
}

func TestPGRepo_Update_AllFields(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, "REF-001", "T", "D", "J", "closed", false, "user-1", "User One", now, now,
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	t2 := "T"
	d2 := "D"
	j2 := "J"
	s2 := "closed"
	_, err := repo.Update(context.Background(), id, UpdateCaseInput{
		Title: &t2, Description: &d2, Jurisdiction: &j2, Status: &s2,
	})
	if err != nil {
		t.Fatalf("Update all fields: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Archive — success, not found, general error (L248-L258)
// ---------------------------------------------------------------------------

// fakeCommandTag wraps pgconn.CommandTag creation.
func makeCommandTag(affected int64) pgconn.CommandTag {
	if affected > 0 {
		return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", affected))
	}
	return pgconn.NewCommandTag("UPDATE 0")
}

func TestPGRepo_Archive_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(1), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.Archive(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
}

func TestPGRepo_Archive_NotFound(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(0), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.Archive(context.Background(), uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepo_Archive_Error(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec failed")
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.Archive(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "archive case") {
		t.Errorf("error = %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold — success, not found, general error (L260-L270)
// ---------------------------------------------------------------------------

func TestPGRepo_SetLegalHold_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(1), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.SetLegalHold(context.Background(), uuid.New(), true)
	if err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}
}

func TestPGRepo_SetLegalHold_NotFound(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(0), nil
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.SetLegalHold(context.Background(), uuid.New(), true)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPGRepo_SetLegalHold_Error(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec failed")
		},
	}
	repo := &PGRepository{pool: pool}
	err := repo.SetLegalHold(context.Background(), uuid.New(), true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "set legal hold") {
		t.Errorf("error = %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// FindAll — additional filter paths (L78-L194)
// ---------------------------------------------------------------------------

func TestPGRepo_FindAll_UserFilter(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin: false, UserID: "user-1",
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll user filter: %v", err)
	}
}

func TestPGRepo_FindAll_StatusFilter(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin: true, Status: []string{"active", "closed"},
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll status filter: %v", err)
	}
}

func TestPGRepo_FindAll_JurisdictionFilter(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin: true, Jurisdiction: "ICC",
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll jurisdiction filter: %v", err)
	}
}

func TestPGRepo_FindAll_SearchFilter(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin: true, SearchQuery: "test",
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll search filter: %v", err)
	}
}

func TestPGRepo_FindAll_CursorFilter(t *testing.T) {
	cursorID := uuid.New()
	cursor := EncodeCursor(cursorID)
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin: true,
	}, Pagination{Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindAll cursor filter: %v", err)
	}
}

func TestPGRepo_FindAll_InvalidCursor(t *testing.T) {
	pool := &fakePool{}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10, Cursor: "!!!"})
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}

func TestPGRepo_FindAll_AllFilters(t *testing.T) {
	cursorID := uuid.New()
	cursor := EncodeCursor(cursorID)
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 0}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	_, _, err := repo.FindAll(context.Background(), CaseFilter{
		SystemAdmin:  false,
		UserID:       "user-1",
		Status:       []string{"active"},
		Jurisdiction: "ICC",
		SearchQuery:  "test",
	}, Pagination{Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindAll all filters: %v", err)
	}
}

// ---------------------------------------------------------------------------
// RoleRepository unit tests via fakePool (roles.go L38-L112)
// ---------------------------------------------------------------------------

func TestRoleRepo_Assign_Success(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	caseID := uuid.New()
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{
				id, caseID, "user-1", "investigator", "admin-1", now,
			}}
		},
	}
	repo := &RoleRepository{pool: pool}
	cr, err := repo.Assign(context.Background(), caseID, "user-1", "investigator", "admin-1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if cr.ID != id {
		t.Errorf("ID = %v, want %v", cr.ID, id)
	}
}

func TestRoleRepo_Assign_Duplicate(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("duplicate key value violates unique constraint")}
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.Assign(context.Background(), uuid.New(), "user-1", "investigator", "admin-1")
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
	if !strings.Contains(err.Error(), "role already assigned") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRoleRepo_Assign_GeneralError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("connection refused")}
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.Assign(context.Background(), uuid.New(), "user-1", "investigator", "admin-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "assign case role") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRoleRepo_Revoke_Success(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(1), nil
		},
	}
	repo := &RoleRepository{pool: pool}
	err := repo.Revoke(context.Background(), uuid.New(), "user-1")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
}

func TestRoleRepo_Revoke_NotFound(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return makeCommandTag(0), nil
		},
	}
	repo := &RoleRepository{pool: pool}
	err := repo.Revoke(context.Background(), uuid.New(), "user-1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRoleRepo_Revoke_Error(t *testing.T) {
	pool := &fakePool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, fmt.Errorf("exec failed")
		},
	}
	repo := &RoleRepository{pool: pool}
	err := repo.Revoke(context.Background(), uuid.New(), "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "revoke case role") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRoleRepo_ListByCaseID_Success(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{}, nil // no rows = empty list
		},
	}
	repo := &RoleRepository{pool: pool}
	roles, err := repo.ListByCaseID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListByCaseID: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("expected empty list, got %d", len(roles))
	}
}

func TestRoleRepo_ListByCaseID_QueryError(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("query failed")
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.ListByCaseID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list case roles") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRoleRepo_ListByCaseID_RowsErr(t *testing.T) {
	pool := &fakePool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &fakeRows{errVal: fmt.Errorf("rows iteration error")}, nil
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.ListByCaseID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from rows.Err")
	}
}

func TestRoleRepo_LoadCaseRole_Success(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{vals: []any{"investigator"}}
		},
	}
	repo := &RoleRepository{pool: pool}
	role, err := repo.LoadCaseRole(context.Background(), uuid.New().String(), "user-1")
	if err != nil {
		t.Fatalf("LoadCaseRole: %v", err)
	}
	if role != auth.CaseRoleInvestigator {
		t.Errorf("role = %q, want investigator", role)
	}
}

func TestRoleRepo_LoadCaseRole_NoRows(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.LoadCaseRole(context.Background(), uuid.New().String(), "user-1")
	if err != auth.ErrNoCaseRole {
		t.Errorf("expected ErrNoCaseRole, got %v", err)
	}
}

func TestRoleRepo_LoadCaseRole_GeneralError(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: fmt.Errorf("connection timeout")}
		},
	}
	repo := &RoleRepository{pool: pool}
	_, err := repo.LoadCaseRole(context.Background(), uuid.New().String(), "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "load case role") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRoleRepo_LoadCaseRole_InvalidCaseID(t *testing.T) {
	repo := &RoleRepository{pool: &fakePool{}}
	_, err := repo.LoadCaseRole(context.Background(), "not-a-uuid", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid case ID")
	}
	if !strings.Contains(err.Error(), "parse case ID") {
		t.Errorf("error = %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// FindAll — hasMore branch (L188-L191)
// Requires fakeRows that return more rows than the page limit.
// ---------------------------------------------------------------------------

// multiCaseRows returns a configurable number of case rows from Next/Scan.
type multiCaseRows struct {
	total   int
	current int
}

func (r *multiCaseRows) Close()                                       {}
func (r *multiCaseRows) Err() error                                   { return nil }
func (r *multiCaseRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *multiCaseRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *multiCaseRows) RawValues() [][]byte                          { return nil }
func (r *multiCaseRows) Conn() *pgx.Conn                              { return nil }
func (r *multiCaseRows) Values() ([]any, error)                       { return nil, nil }

func (r *multiCaseRows) Next() bool {
	if r.current < r.total {
		r.current++
		return true
	}
	return false
}

func (r *multiCaseRows) Scan(dest ...any) error {
	now := time.Now()
	vals := []any{
		uuid.New(), "REF-" + fmt.Sprintf("%03d", r.current), "Title", "Desc",
		"Juris", "active", false, "user-1", "User", now, now,
	}
	for i, d := range dest {
		if i >= len(vals) {
			break
		}
		switch p := d.(type) {
		case *uuid.UUID:
			if v, ok := vals[i].(uuid.UUID); ok {
				*p = v
			}
		case *string:
			if v, ok := vals[i].(string); ok {
				*p = v
			}
		case *bool:
			if v, ok := vals[i].(bool); ok {
				*p = v
			}
		case *time.Time:
			if v, ok := vals[i].(time.Time); ok {
				*p = v
			}
		}
	}
	return nil
}

func TestPGRepo_FindAll_HasMore(t *testing.T) {
	limit := 2
	// Return limit+1 rows to trigger the hasMore branch
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &countRow{val: 5}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &multiCaseRows{total: limit + 1}, nil
		},
	}
	repo := &PGRepository{pool: pool}
	cases, total, err := repo.FindAll(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: limit})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	// Should be trimmed to limit
	if len(cases) != limit {
		t.Errorf("len(cases) = %d, want %d (trimmed from %d)", len(cases), limit, limit+1)
	}
}
