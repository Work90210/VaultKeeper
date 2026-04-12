package collaboration

// Unit coverage for PostgresDraftStore using a fake draftStoreDB.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---- fake db ----

type fakePersistRow struct {
	state []byte
	err   error
}

func (r *fakePersistRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if p, ok := dest[0].(*[]byte); ok {
			*p = r.state
		}
	}
	return nil
}

type fakePersistDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (f *fakePersistDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRowFn(ctx, sql, args...)
}
func (f *fakePersistDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return f.execFn(ctx, sql, args...)
}

// ---- LoadDraft ----

func TestLoadDraft_Found(t *testing.T) {
	store := &PostgresDraftStore{db: &fakePersistDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakePersistRow{state: []byte("yjs-bytes")}
		},
	}}
	got, err := store.LoadDraft(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(got) != "yjs-bytes" {
		t.Errorf("got %q, want yjs-bytes", got)
	}
}

func TestLoadDraft_NotFound(t *testing.T) {
	store := &PostgresDraftStore{db: &fakePersistDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakePersistRow{err: pgx.ErrNoRows}
		},
	}}
	got, err := store.LoadDraft(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("ErrNoRows should become (nil, nil), got %v", err)
	}
	if got != nil {
		t.Errorf("want nil state, got %v", got)
	}
}

func TestLoadDraft_GenericError(t *testing.T) {
	boom := errors.New("db down")
	store := &PostgresDraftStore{db: &fakePersistDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakePersistRow{err: boom}
		},
	}}
	_, err := store.LoadDraft(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

// ---- SaveDraft ----

func TestSaveDraft_UpdateExisting(t *testing.T) {
	store := &PostgresDraftStore{db: &fakePersistDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}}
	if err := store.SaveDraft(context.Background(), uuid.New(), uuid.New(), "actor-uuid", []byte("x")); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestSaveDraft_UpdateError(t *testing.T) {
	boom := errors.New("pg update failed")
	store := &PostgresDraftStore{db: &fakePersistDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, boom
		},
	}}
	err := store.SaveDraft(context.Background(), uuid.New(), uuid.New(), "actor", []byte("x"))
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

func TestSaveDraft_InsertWhenNoActive(t *testing.T) {
	var calls int
	store := &PostgresDraftStore{db: &fakePersistDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			calls++
			if calls == 1 {
				return pgconn.NewCommandTag("UPDATE 0"), nil // no active draft
			}
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}}
	if err := store.SaveDraft(context.Background(), uuid.New(), uuid.New(), "actor", []byte("x")); err != nil {
		t.Fatalf("err: %v", err)
	}
	if calls != 2 {
		t.Errorf("exec calls = %d, want 2 (update then insert)", calls)
	}
}

func TestSaveDraft_InsertError(t *testing.T) {
	boom := errors.New("insert failed")
	var calls int
	store := &PostgresDraftStore{db: &fakePersistDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			calls++
			if calls == 1 {
				return pgconn.NewCommandTag("UPDATE 0"), nil
			}
			return pgconn.CommandTag{}, boom
		},
	}}
	err := store.SaveDraft(context.Background(), uuid.New(), uuid.New(), "actor", []byte("x"))
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}
