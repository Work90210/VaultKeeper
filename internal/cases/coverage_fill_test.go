package cases

// Coverage-fill tests to bring internal/cases to 100%. Targets the
// remaining gaps: Update (missing pgx.ErrNoRows + generic error paths),
// CheckLegalHoldStrict (0%), SetLogger (0%), and the SetLegalHold
// notifier error branch.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/apperrors"
)

// ---- CheckLegalHoldStrict ----

type checkHoldRow struct {
	hold bool
	err  error
}

func (r *checkHoldRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*bool)) = r.hold
	return nil
}

func TestCheckLegalHoldStrict_True(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &checkHoldRow{hold: true}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.CheckLegalHoldStrict(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got {
		t.Error("want true")
	}
}

func TestCheckLegalHoldStrict_False(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &checkHoldRow{hold: false}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := repo.CheckLegalHoldStrict(context.Background(), uuid.New())
	if err != nil || got {
		t.Errorf("got %v err=%v", got, err)
	}
}

func TestCheckLegalHoldStrict_NotFound(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &checkHoldRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.CheckLegalHoldStrict(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestCheckLegalHoldStrict_OtherError(t *testing.T) {
	boom := errors.New("pool closed")
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &checkHoldRow{err: boom}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := repo.CheckLegalHoldStrict(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("must not be ErrNotFound")
	}
}

// ---- Service.SetLogger ----

func TestServiceSetLogger(t *testing.T) {
	svc, err := NewService(newMockRepo(), nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc.SetLogger(logger)
	if svc.logger != logger {
		t.Error("logger not set")
	}
}

// ---- Service.SetLegalHold notifier error branch ----

type errorMemberNotifier struct{}

func (e *errorMemberNotifier) NotifyLegalHoldChanged(_ context.Context, _ uuid.UUID, _ bool, _ string) error {
	return errors.New("notifier down")
}

func TestSetLegalHold_NotifierError_BestEffort(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}

	svc, err := NewService(repo, &captureCustody{}, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetMemberNotifier(&errorMemberNotifier{})
	svc.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Notifier error must not fail the toggle itself.
	if err := svc.SetLegalHold(context.Background(), id, true, "actor"); err != nil {
		t.Errorf("notifier errors must be best-effort, got %v", err)
	}
	// State must still be updated in the repo.
	if !repo.cases[id].LegalHold {
		t.Error("hold not persisted")
	}
}

// captureCustody is a minimal CustodyRecorder for the above test.
type captureCustody struct{}

func (c *captureCustody) RecordCaseEvent(_ context.Context, _ uuid.UUID, _ string, _ string, _ map[string]string) error {
	return nil
}

// Compile-time check that our apperrors alias is wired.
var _ = apperrors.ErrLegalHoldActive

// ---- Update() remaining branches: ErrNoRows → ErrNotFound, generic error ----

type updateCaseRow struct{ err error }

func (r *updateCaseRow) Scan(_ ...any) error { return r.err }

func TestUpdate_NotFound(t *testing.T) {
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &updateCaseRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	title := "new title"
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{Title: &title})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUpdate_OtherError(t *testing.T) {
	boom := errors.New("update failed")
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &updateCaseRow{err: boom}
		},
	}
	repo := &PGRepository{pool: pool}
	desc := "new desc"
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{Description: &desc})
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

func TestUpdate_AllFieldsSet(t *testing.T) {
	// Drives every branch of the dynamic SET-clause builder, including
	// RetentionUntil and the ClearRetentionUntil path.
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows} // short-circuit after SQL is assembled
		},
	}
	repo := &PGRepository{pool: pool}
	title := "t"
	desc := "d"
	jur := "j"
	status := StatusActive
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{
		Title:               &title,
		Description:         &desc,
		Jurisdiction:        &jur,
		Status:              &status,
		ClearRetentionUntil: true,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound from stubbed row, got %v", err)
	}
}

func TestUpdate_RetentionUntilSet(t *testing.T) {
	// Covers the `updates.RetentionUntil != nil` branch separately — the
	// ClearRetentionUntil=true path and the SET-value path are mutually
	// exclusive in the builder.
	pool := &fakePool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &fakeRow{err: pgx.ErrNoRows}
		},
	}
	repo := &PGRepository{pool: pool}
	until := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := repo.Update(context.Background(), uuid.New(), UpdateCaseInput{
		RetentionUntil: &until,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}
