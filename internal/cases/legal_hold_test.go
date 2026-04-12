package cases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubHoldReader struct {
	active bool
	err    error
	calls  int
}

func (s *stubHoldReader) IsLegalHoldActive(ctx context.Context, caseID uuid.UUID) (bool, error) {
	s.calls++
	return s.active, s.err
}

func TestEnsureNotOnHold_NotActive(t *testing.T) {
	r := &stubHoldReader{active: false}
	if err := EnsureNotOnHold(context.Background(), r, uuid.New()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if r.calls != 1 {
		t.Errorf("calls = %d, want 1", r.calls)
	}
}

func TestEnsureNotOnHold_Active(t *testing.T) {
	r := &stubHoldReader{active: true}
	err := EnsureNotOnHold(context.Background(), r, uuid.New())
	if !errors.Is(err, ErrLegalHoldActive) {
		t.Fatalf("want ErrLegalHoldActive, got %v", err)
	}
}

func TestEnsureNotOnHold_NilReader(t *testing.T) {
	if err := EnsureNotOnHold(context.Background(), nil, uuid.New()); err == nil {
		t.Fatal("want error for nil reader")
	}
}

func TestEnsureNotOnHold_ReaderError(t *testing.T) {
	boom := errors.New("db down")
	r := &stubHoldReader{err: boom}
	err := EnsureNotOnHold(context.Background(), r, uuid.New())
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("want wrapped %v, got %v", boom, err)
	}
}

// ---------------------------------------------------------------------------
// Service.EnsureNotOnHold — uses the strict single-column read path
// ---------------------------------------------------------------------------

// strictCountingRepo wraps mockRepo to verify that Service.EnsureNotOnHold
// uses CheckLegalHoldStrict (and NOT FindByID) so future regressions that
// accidentally re-introduce the full row fetch are caught.
type strictCountingRepo struct {
	*mockRepo
	strictCalls   int
	findByIDCalls int
}

func (s *strictCountingRepo) CheckLegalHoldStrict(ctx context.Context, id uuid.UUID) (bool, error) {
	s.strictCalls++
	return s.mockRepo.CheckLegalHoldStrict(ctx, id)
}

func (s *strictCountingRepo) FindByID(ctx context.Context, id uuid.UUID) (Case, error) {
	s.findByIDCalls++
	return s.mockRepo.FindByID(ctx, id)
}

func TestServiceEnsureNotOnHold_UsesStrictRead_NotOnHold(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	inner.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}
	repo := &strictCountingRepo{mockRepo: inner}

	svc, err := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := svc.EnsureNotOnHold(context.Background(), id); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if repo.strictCalls != 1 {
		t.Errorf("strict calls = %d, want 1", repo.strictCalls)
	}
	if repo.findByIDCalls != 0 {
		t.Errorf("FindByID must not be used by EnsureNotOnHold, calls = %d", repo.findByIDCalls)
	}
}

func TestServiceEnsureNotOnHold_UsesStrictRead_OnHold(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	inner.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: true}
	repo := &strictCountingRepo{mockRepo: inner}

	svc, err := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = svc.EnsureNotOnHold(context.Background(), id)
	if !errors.Is(err, ErrLegalHoldActive) {
		t.Fatalf("want ErrLegalHoldActive, got %v", err)
	}
	if repo.strictCalls != 1 {
		t.Errorf("strict calls = %d, want 1", repo.strictCalls)
	}
	if repo.findByIDCalls != 0 {
		t.Errorf("FindByID must not be used, calls = %d", repo.findByIDCalls)
	}
}

func TestServiceEnsureNotOnHold_StrictReadNotFound(t *testing.T) {
	inner := newMockRepo()
	repo := &strictCountingRepo{mockRepo: inner}
	svc, err := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = svc.EnsureNotOnHold(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound wrapped, got %v", err)
	}
}
