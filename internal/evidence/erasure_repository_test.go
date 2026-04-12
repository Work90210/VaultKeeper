package evidence

// Unit tests for erasure_repository.go. Uses mockDBPool + mockRow so the
// SQL layer is exercised without a live Postgres. Covers happy path,
// pgx.ErrNoRows → ErrNotFound mapping, and generic error wrapping on
// each of the three CRUD methods.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// erasureScan builds a scanFn that fills the 9-column RETURNING payload
// used by all three erasure_repository SQL statements.
func erasureScan(req ErasureRequest) func(dest ...any) error {
	return func(dest ...any) error {
		*dest[0].(*uuid.UUID) = req.ID
		*dest[1].(*uuid.UUID) = req.EvidenceID
		*dest[2].(*string) = req.RequestedBy
		*dest[3].(*string) = req.Rationale
		*dest[4].(*string) = req.Status
		*dest[5].(**string) = req.Decision
		*dest[6].(**string) = req.DecidedBy
		*dest[7].(**time.Time) = req.DecidedAt
		*dest[8].(*time.Time) = req.CreatedAt
		return nil
	}
}

// ---- CreateErasureRequest ----

func TestPGRepo_CreateErasureRequest_Success(t *testing.T) {
	want := ErasureRequest{
		ID:          uuid.New(),
		EvidenceID:  uuid.New(),
		RequestedBy: "admin-uuid",
		Rationale:   "subject access request",
		Status:      ErasureStatusReady,
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: erasureScan(want)}
		},
	}
	repo := &PGRepository{pool: pool}

	got, err := repo.CreateErasureRequest(context.Background(), want)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != want.ID || got.RequestedBy != want.RequestedBy {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, want)
	}
}

func TestPGRepo_CreateErasureRequest_DBError(t *testing.T) {
	boom := errors.New("constraint violation")
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: boom}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.CreateErasureRequest(context.Background(), ErasureRequest{})
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

// ---- FindErasureRequest ----

func TestPGRepo_FindErasureRequest_Success(t *testing.T) {
	want := ErasureRequest{ID: uuid.New(), Status: ErasureStatusConflictPending}
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: erasureScan(want)}
		},
	}
	repo := &PGRepository{pool: pool}

	got, err := repo.FindErasureRequest(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID mismatch")
	}
}

func TestPGRepo_FindErasureRequest_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindErasureRequest(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPGRepo_FindErasureRequest_OtherError(t *testing.T) {
	boom := errors.New("connection lost")
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: boom}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.FindErasureRequest(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("generic errors must not be reported as ErrNotFound")
	}
}

// ---- UpdateErasureDecision ----

func TestPGRepo_UpdateErasureDecision_Success(t *testing.T) {
	want := ErasureRequest{
		ID:     uuid.New(),
		Status: ErasureStatusResolvedPreserve,
	}
	decision := "preserve"
	decidedBy := "admin-uuid"
	decidedAt := time.Now().UTC()
	want.Decision = &decision
	want.DecidedBy = &decidedBy
	want.DecidedAt = &decidedAt

	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: erasureScan(want)}
		},
	}
	repo := &PGRepository{pool: pool}

	got, err := repo.UpdateErasureDecision(context.Background(), want.ID, want.Status, *want.Decision, *want.DecidedBy, *want.DecidedAt)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Status != ErasureStatusResolvedPreserve {
		t.Errorf("status = %q", got.Status)
	}
}

func TestPGRepo_UpdateErasureDecision_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.UpdateErasureDecision(context.Background(), uuid.New(), "preserve", "preserve", "admin", time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPGRepo_UpdateErasureDecision_OtherError(t *testing.T) {
	boom := errors.New("deadlock detected")
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: boom}
		},
	}
	repo := &PGRepository{pool: pool}

	_, err := repo.UpdateErasureDecision(context.Background(), uuid.New(), "preserve", "preserve", "admin", time.Now())
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}
