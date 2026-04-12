package evidence

// Unit tests for tag_repository.go that drive PGRepository against the
// mockDBPool / mockTx fakes (no real Postgres required). These close the
// 0%→100% gap that existed because the equivalent integration tests are
// gated behind the //go:build integration tag.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// tagQueryStubRows fakes pgx.Rows for ListDistinctTags by yielding a
// scripted slice of tag strings.
type tagQueryStubRows struct {
	tags []string
	idx  int
	err  error
}

func (r *tagQueryStubRows) Close()                                       {}
func (r *tagQueryStubRows) Err() error                                   { return r.err }
func (r *tagQueryStubRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *tagQueryStubRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *tagQueryStubRows) RawValues() [][]byte                          { return nil }
func (r *tagQueryStubRows) Values() ([]any, error)                       { return nil, nil }
func (r *tagQueryStubRows) Conn() *pgx.Conn                              { return nil }
func (r *tagQueryStubRows) Next() bool {
	if r.idx >= len(r.tags) {
		return false
	}
	r.idx++
	return true
}
func (r *tagQueryStubRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*string)) = r.tags[r.idx-1]
	return nil
}

// ---- ListDistinctTags ----

func TestListDistinctTags_ReturnsRows(t *testing.T) {
	pool := &mockDBPool{
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			return &tagQueryStubRows{tags: []string{"alpha", "beta", "gamma"}}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	tags, err := repo.ListDistinctTags(context.Background(), uuid.New(), "", 10)
	if err != nil {
		t.Fatalf("ListDistinctTags: %v", err)
	}
	if len(tags) != 3 {
		t.Errorf("len = %d, want 3", len(tags))
	}
}

func TestListDistinctTags_DefaultLimit(t *testing.T) {
	var sawArgs []any
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			sawArgs = args
			return &tagQueryStubRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListDistinctTags(context.Background(), uuid.New(), "pre", 0) // zero → default
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// args[2] is the limit.
	if got, ok := sawArgs[2].(int); !ok || got != MaxTagAutocompleteLimit {
		t.Errorf("limit arg = %v, want %d", sawArgs[2], MaxTagAutocompleteLimit)
	}
}

func TestListDistinctTags_EscapesLikeMetachars(t *testing.T) {
	var sawPattern string
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			sawPattern = args[1].(string)
			return &tagQueryStubRows{}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, _ = repo.ListDistinctTags(context.Background(), uuid.New(), "10%_off", 10)
	// Both % and _ must be escaped with a backslash in the LIKE pattern.
	if sawPattern != `10\%\_off%` {
		t.Errorf("LIKE pattern = %q, want %q", sawPattern, `10\%\_off%`)
	}
}

func TestListDistinctTags_QueryError(t *testing.T) {
	boom := errors.New("pg down")
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return nil, boom
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListDistinctTags(context.Background(), uuid.New(), "", 10)
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

func TestListDistinctTags_ScanError(t *testing.T) {
	scanErr := errors.New("scan failed")
	pool := &mockDBPool{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &tagQueryStubRows{tags: []string{"x"}, err: scanErr}, nil
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.ListDistinctTags(context.Background(), uuid.New(), "", 10)
	if err == nil {
		t.Fatal("want scan error")
	}
}

// ---- tagAdvisoryLockID ----

func TestTagAdvisoryLockID_Stable(t *testing.T) {
	id := uuid.MustParse("44b283ca-3ede-4a8b-bdef-86dddb3e9c51")
	a := tagAdvisoryLockID(id)
	b := tagAdvisoryLockID(id)
	if a != b {
		t.Errorf("non-deterministic: %d vs %d", a, b)
	}
}

func TestTagAdvisoryLockID_Distinct(t *testing.T) {
	a := tagAdvisoryLockID(uuid.MustParse("44b283ca-3ede-4a8b-bdef-86dddb3e9c51"))
	b := tagAdvisoryLockID(uuid.MustParse("c3fb8c7c-0775-49bb-8504-57d509cc101e"))
	if a == b {
		t.Errorf("distinct UUIDs produced identical lock ids: %d", a)
	}
}

// ---- withCaseTagLock ----

func TestWithCaseTagLock_BeginTxError(t *testing.T) {
	boom := errors.New("can't begin")
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return nil, boom
		},
	}
	repo := &PGRepository{pool: pool}

	n, err := repo.withCaseTagLock(context.Background(), uuid.New(), func(tx pgx.Tx) (int64, error) {
		return 42, nil
	})
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped begin error, got %v", err)
	}
}

func TestWithCaseTagLock_LockAcquireError(t *testing.T) {
	boom := errors.New("lock failed")
	tx := &mockTx{
		execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			// pg_advisory_xact_lock → return error
			return pgconn.CommandTag{}, boom
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.withCaseTagLock(context.Background(), uuid.New(), func(_ pgx.Tx) (int64, error) {
		return 0, nil
	})
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped lock error, got %v", err)
	}
}

func TestWithCaseTagLock_InnerError(t *testing.T) {
	innerErr := errors.New("inner")
	tx := &mockTx{}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.withCaseTagLock(context.Background(), uuid.New(), func(_ pgx.Tx) (int64, error) {
		return 0, innerErr
	})
	if !errors.Is(err, innerErr) {
		t.Errorf("want inner error, got %v", err)
	}
}

func TestWithCaseTagLock_CommitError(t *testing.T) {
	tx := &mockTx{commitErr: errors.New("commit failed")}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.withCaseTagLock(context.Background(), uuid.New(), func(_ pgx.Tx) (int64, error) {
		return 5, nil
	})
	if err == nil {
		t.Fatal("want commit error")
	}
}

func TestWithCaseTagLock_Success(t *testing.T) {
	tx := &mockTx{}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	n, err := repo.withCaseTagLock(context.Background(), uuid.New(), func(_ pgx.Tx) (int64, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 42 {
		t.Errorf("n = %d, want 42", n)
	}
}

// ---- RenameTagInCase / MergeTagsInCase / DeleteTagFromCase ----

func execTagWith(rows int64) *mockTx {
	return &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE " + itoa(rows)), nil
		},
	}
}

// itoa without strconv dependency noise.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestRenameTagInCase_Success(t *testing.T) {
	var execCalls int
	tx := &mockTx{
		execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
			execCalls++
			// First exec is pg_advisory_xact_lock, second is the UPDATE.
			if execCalls == 2 {
				return pgconn.NewCommandTag("UPDATE 5"), nil
			}
			return pgconn.CommandTag{}, nil
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	n, err := repo.RenameTagInCase(context.Background(), uuid.New(), "old", "new")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if execCalls != 2 {
		t.Errorf("exec calls = %d, want 2 (lock + update)", execCalls)
	}
}

func TestRenameTagInCase_UpdateError(t *testing.T) {
	boom := errors.New("update failed")
	var n int
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			n++
			if n == 1 {
				return pgconn.CommandTag{}, nil // lock ok
			}
			return pgconn.CommandTag{}, boom
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.RenameTagInCase(context.Background(), uuid.New(), "x", "y")
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped update error, got %v", err)
	}
}

func TestMergeTagsInCase_EmptySources(t *testing.T) {
	pool := &mockDBPool{}
	repo := &PGRepository{pool: pool}

	n, err := repo.MergeTagsInCase(context.Background(), uuid.New(), nil, "target")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
}

func TestMergeTagsInCase_Success(t *testing.T) {
	var n int
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			n++
			if n == 2 {
				return pgconn.NewCommandTag("UPDATE 3"), nil
			}
			return pgconn.CommandTag{}, nil
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	rows, err := repo.MergeTagsInCase(context.Background(), uuid.New(), []string{"a", "b"}, "z")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows != 3 {
		t.Errorf("rows = %d, want 3", rows)
	}
}

func TestMergeTagsInCase_UpdateError(t *testing.T) {
	boom := errors.New("merge failed")
	var n int
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			n++
			if n == 1 {
				return pgconn.CommandTag{}, nil
			}
			return pgconn.CommandTag{}, boom
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.MergeTagsInCase(context.Background(), uuid.New(), []string{"a"}, "z")
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped merge error, got %v", err)
	}
}

func TestDeleteTagFromCase_Success(t *testing.T) {
	var n int
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			n++
			if n == 2 {
				return pgconn.NewCommandTag("UPDATE 2"), nil
			}
			return pgconn.CommandTag{}, nil
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	rows, err := repo.DeleteTagFromCase(context.Background(), uuid.New(), "stale")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows != 2 {
		t.Errorf("rows = %d, want 2", rows)
	}
}

func TestDeleteTagFromCase_UpdateError(t *testing.T) {
	boom := errors.New("delete failed")
	var n int
	tx := &mockTx{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			n++
			if n == 1 {
				return pgconn.CommandTag{}, nil
			}
			return pgconn.CommandTag{}, boom
		},
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.DeleteTagFromCase(context.Background(), uuid.New(), "stale")
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped delete error, got %v", err)
	}
}

// ---- escapeLikePattern ----

func TestEscapeLikePattern_Cases(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"with%pct", `with\%pct`},
		{"with_under", `with\_under`},
		{`with\slash`, `with\\slash`},
		{`all %_\`, `all \%\_\\`},
		{"", ""},
	}
	for _, tc := range cases {
		if got := escapeLikePattern(tc.in); got != tc.want {
			t.Errorf("escapeLikePattern(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
