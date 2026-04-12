package app

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/cases"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// caseRepoStub is a minimal cases.Repository implementation. Only the
// methods reached via cases.Service.EnsureNotOnHold need to be wired
// meaningfully — the rest satisfy the interface contract with zero
// values.
type caseRepoStub struct {
	hold bool
	err  error
}

func (s *caseRepoStub) Create(_ context.Context, _ cases.Case) (cases.Case, error) {
	return cases.Case{}, nil
}
func (s *caseRepoStub) FindByID(_ context.Context, id uuid.UUID) (cases.Case, error) {
	return cases.Case{ID: id, LegalHold: s.hold, Status: cases.StatusActive}, nil
}
func (s *caseRepoStub) FindAll(_ context.Context, _ cases.CaseFilter, _ cases.Pagination) ([]cases.Case, int, error) {
	return nil, 0, nil
}
func (s *caseRepoStub) Update(_ context.Context, _ uuid.UUID, _ cases.UpdateCaseInput) (cases.Case, error) {
	return cases.Case{}, nil
}
func (s *caseRepoStub) Archive(_ context.Context, _ uuid.UUID) error { return nil }
func (s *caseRepoStub) SetLegalHold(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}
func (s *caseRepoStub) CheckLegalHoldStrict(_ context.Context, _ uuid.UUID) (bool, error) {
	return s.hold, s.err
}

func newStubService(t *testing.T, repo cases.Repository) *cases.Service {
	t.Helper()
	svc, err := cases.NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestLegalHoldAdapter_NotOnHold(t *testing.T) {
	svc := newStubService(t, &caseRepoStub{hold: false})
	adapter := &LegalHoldAdapter{Svc: svc}

	if err := adapter.EnsureNotOnHold(context.Background(), uuid.New()); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestLegalHoldAdapter_OnHold_MapsSentinel(t *testing.T) {
	svc := newStubService(t, &caseRepoStub{hold: true})
	adapter := &LegalHoldAdapter{Svc: svc}

	err := adapter.EnsureNotOnHold(context.Background(), uuid.New())
	if !errors.Is(err, evidence.ErrLegalHoldActive) {
		t.Errorf("want evidence.ErrLegalHoldActive, got %v", err)
	}
	// With the shared apperrors package the two sentinels alias each
	// other, so cases.ErrLegalHoldActive should also match.
	if !errors.Is(err, cases.ErrLegalHoldActive) {
		t.Errorf("want cases.ErrLegalHoldActive (aliased), got %v", err)
	}
}

func TestLegalHoldAdapter_PassthroughOtherErrors(t *testing.T) {
	dbErr := errors.New("database exploded")
	svc := newStubService(t, &caseRepoStub{err: dbErr})
	adapter := &LegalHoldAdapter{Svc: svc}

	err := adapter.EnsureNotOnHold(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if errors.Is(err, evidence.ErrLegalHoldActive) {
		t.Error("non-hold error must not be mapped to ErrLegalHoldActive")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("want wrapped %v, got %v", dbErr, err)
	}
}

func TestLegalHoldAdapter_NilAdapter(t *testing.T) {
	var a *LegalHoldAdapter
	err := a.EnsureNotOnHold(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if matched, _ := regexp.MatchString("not configured", err.Error()); !matched {
		t.Errorf("error message = %q, want mention of 'not configured'", err.Error())
	}
}

func TestLegalHoldAdapter_NilSvc(t *testing.T) {
	adapter := &LegalHoldAdapter{Svc: nil}
	err := adapter.EnsureNotOnHold(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
