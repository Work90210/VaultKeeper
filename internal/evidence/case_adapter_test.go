package evidence

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPGCaseLookup_ImplementsCaseLookup(t *testing.T) {
	// Verify interface compliance at compile time
	var _ CaseLookup = (*PGCaseLookup)(nil)
}

func TestNewCaseLookup(t *testing.T) {
	// NewCaseLookup requires *pgxpool.Pool which is nil in unit tests.
	// Verify the constructor returns a non-nil struct.
	lookup := NewCaseLookup(nil)
	if lookup == nil {
		t.Fatal("expected non-nil PGCaseLookup")
	}
}

// mockQueryRowPool satisfies the pool interface used by PGCaseLookup.
type mockQueryRowPool struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (p *mockQueryRowPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.queryRowFn != nil {
		return p.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanErr: pgx.ErrNoRows}
}

func TestPGCaseLookup_GetLegalHold_Success(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*bool) = true
					return nil
				},
			}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	held, err := lookup.GetLegalHold(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetLegalHold error: %v", err)
	}
	if !held {
		t.Error("expected legal hold to be true")
	}
}

func TestPGCaseLookup_GetLegalHold_Error(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	_, err := lookup.GetLegalHold(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGCaseLookup_GetReferenceCode_Success(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "CASE-2024-001"
					return nil
				},
			}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	code, err := lookup.GetReferenceCode(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetReferenceCode error: %v", err)
	}
	if code != "CASE-2024-001" {
		t.Errorf("code = %q, want CASE-2024-001", code)
	}
}

func TestPGCaseLookup_GetReferenceCode_Error(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	_, err := lookup.GetReferenceCode(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPGCaseLookup_GetStatus_Success(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*string) = "active"
					return nil
				},
			}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	status, err := lookup.GetStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetStatus error: %v", err)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}
}

func TestPGCaseLookup_GetStatus_Error(t *testing.T) {
	pool := &mockQueryRowPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db error")}
		},
	}
	lookup := &PGCaseLookup{pool: pool}

	_, err := lookup.GetStatus(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
