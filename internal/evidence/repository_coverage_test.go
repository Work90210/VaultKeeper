package evidence

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// MarkNonCurrent
// ---------------------------------------------------------------------------

func TestPGRepository_MarkNonCurrent_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.MarkNonCurrent(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("MarkNonCurrent error: %v", err)
	}
}

func TestPGRepository_MarkNonCurrent_DBError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.MarkNonCurrent(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from MarkNonCurrent")
	}
}

// ---------------------------------------------------------------------------
// MarkNonCurrentWithTx
// ---------------------------------------------------------------------------

func TestPGRepository_MarkNonCurrentWithTx_Success(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.MarkNonCurrentWithTx(context.Background(), tx, uuid.New())
	if err != nil {
		t.Fatalf("MarkNonCurrentWithTx error: %v", err)
	}
}

func TestPGRepository_MarkNonCurrentWithTx_DBError(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("tx exec error")
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.MarkNonCurrentWithTx(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("expected error from MarkNonCurrentWithTx")
	}
}

// ---------------------------------------------------------------------------
// UpdateVersionFieldsWithTx
// ---------------------------------------------------------------------------

func TestPGRepository_UpdateVersionFieldsWithTx_Success(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.UpdateVersionFieldsWithTx(context.Background(), tx, uuid.New(), uuid.New(), 2)
	if err != nil {
		t.Fatalf("UpdateVersionFieldsWithTx error: %v", err)
	}
}

func TestPGRepository_UpdateVersionFieldsWithTx_DBError(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("tx exec error")
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.UpdateVersionFieldsWithTx(context.Background(), tx, uuid.New(), uuid.New(), 2)
	if err == nil {
		t.Fatal("expected error from UpdateVersionFieldsWithTx")
	}
}

// ---------------------------------------------------------------------------
// FindDraftByID — non-ErrNoRows error branch
// ---------------------------------------------------------------------------

func TestPGRepository_FindDraftByID_DBError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("connection reset")}
		},
	}
	repo := &PGRepository{pool: pool}

	_, _, err := repo.FindDraftByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error from FindDraftByID on db error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("db error should not become ErrNotFound")
	}
}

// ---------------------------------------------------------------------------
// ListDrafts — query error branch
// ---------------------------------------------------------------------------

func TestPGRepository_ListDrafts_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListDrafts(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListDrafts on query error")
	}
}

// ListDrafts — scan error branch (rows.Next() returns true but Scan fails)
func TestPGRepository_ListDrafts_ScanError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true},
				scanErr:  errors.New("scan error"),
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListDrafts(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListDrafts on scan error")
	}
}

// ---------------------------------------------------------------------------
// DiscardDraft — exec error branch
// ---------------------------------------------------------------------------

func TestPGRepository_DiscardDraft_ExecError(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := &PGRepository{pool: pool}

	err := repo.DiscardDraft(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error from DiscardDraft on exec error")
	}
}

// ---------------------------------------------------------------------------
// LockDraftForFinalize — error branch (tx.QueryRow.Scan fails)
// ---------------------------------------------------------------------------

func TestPGRepository_LockDraftForFinalize_Error(t *testing.T) {
	tx := &mockTx{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("lock failed")}
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	_, _, err := repo.LockDraftForFinalize(context.Background(), tx, uuid.New())
	if err == nil {
		t.Fatal("expected error from LockDraftForFinalize on scan error")
	}
}

// ---------------------------------------------------------------------------
// MarkDraftApplied — exec error branch
// ---------------------------------------------------------------------------

func TestPGRepository_MarkDraftApplied_ExecError(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.MarkDraftApplied(context.Background(), tx, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error from MarkDraftApplied on exec error")
	}
}

// ---------------------------------------------------------------------------
// ListFinalizedRedactions — scan error branch
// ---------------------------------------------------------------------------

func TestPGRepository_ListFinalizedRedactions_ScanError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				nextVals: []bool{true},
				scanErr:  errors.New("scan error"),
			}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListFinalizedRedactions(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListFinalizedRedactions on scan error")
	}
}

func TestPGRepository_ListFinalizedRedactions_QueryError(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListFinalizedRedactions(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from ListFinalizedRedactions on query error")
	}
}

// ---------------------------------------------------------------------------
// GetManagementView — BeginTx error, individual query errors, commit error
// ---------------------------------------------------------------------------

func TestPGRepository_GetManagementView_BeginTxError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return nil, errors.New("begin tx error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.GetManagementView(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when BeginTx fails")
	}
}

// GetManagementView — ListFinalizedRedactions fails (pool.Query returns error
// for the second pool.Query call but BeginTx succeeds).
func TestPGRepository_GetManagementView_FinalizedQueryError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, errors.New("finalized query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.GetManagementView(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when ListFinalizedRedactions fails inside GetManagementView")
	}
}

// GetManagementView — ListDrafts fails (second Query call fails, first succeeds empty).
func TestPGRepository_GetManagementView_DraftsQueryError(t *testing.T) {
	queryCallCount := 0
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			queryCallCount++
			if queryCallCount == 1 {
				// First call: ListFinalizedRedactions — return empty rows OK
				return &mockRows{nextVals: []bool{}}, nil
			}
			// Second call: ListDrafts — return error
			return nil, errors.New("drafts query error")
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.GetManagementView(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when ListDrafts fails inside GetManagementView")
	}
}

// GetManagementView — commit error path
func TestPGRepository_GetManagementView_CommitError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{commitErr: errors.New("commit error")}, nil
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			// Both ListFinalizedRedactions and ListDrafts return empty OK
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.GetManagementView(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when tx.Commit fails")
	}
}

// GetManagementView — happy path (ensure success path is covered)
func TestPGRepository_GetManagementView_Success(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{nextVals: []bool{}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	view, err := repo.GetManagementView(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetManagementView error: %v", err)
	}
	if view.Finalized == nil {
		t.Error("Finalized should be non-nil empty slice")
	}
	if view.Drafts == nil {
		t.Error("Drafts should be non-nil empty slice")
	}
}

// ---------------------------------------------------------------------------
// SetDerivativeParentWithTx — error branch
// ---------------------------------------------------------------------------

func TestPGRepository_SetDerivativeParentWithTx_ExecError(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec error")
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.SetDerivativeParentWithTx(context.Background(), tx, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error from SetDerivativeParentWithTx on exec error")
	}
}

func TestPGRepository_SetDerivativeParentWithTx_Success(t *testing.T) {
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGRepository{pool: &mockDBPool{}}

	err := repo.SetDerivativeParentWithTx(context.Background(), tx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("SetDerivativeParentWithTx error: %v", err)
	}
}
