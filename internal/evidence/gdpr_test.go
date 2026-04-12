package evidence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeErasureRepo is an in-memory ErasureRepository.
type fakeErasureRepo struct {
	reqs map[uuid.UUID]ErasureRequest
	// Error-injection knobs for coverage tests.
	createErr error
	findErr   error
	updateErr error
}

func newFakeErasureRepo() *fakeErasureRepo {
	return &fakeErasureRepo{reqs: map[uuid.UUID]ErasureRequest{}}
}

func (f *fakeErasureRepo) CreateErasureRequest(_ context.Context, req ErasureRequest) (ErasureRequest, error) {
	if f.createErr != nil {
		return ErasureRequest{}, f.createErr
	}
	f.reqs[req.ID] = req
	return req, nil
}

func (f *fakeErasureRepo) FindErasureRequest(_ context.Context, id uuid.UUID) (ErasureRequest, error) {
	if f.findErr != nil {
		return ErasureRequest{}, f.findErr
	}
	r, ok := f.reqs[id]
	if !ok {
		return ErasureRequest{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeErasureRepo) UpdateErasureDecision(_ context.Context, id uuid.UUID, status, decision, decidedBy string, decidedAt time.Time) (ErasureRequest, error) {
	if f.updateErr != nil {
		return ErasureRequest{}, f.updateErr
	}
	r, ok := f.reqs[id]
	if !ok {
		return ErasureRequest{}, ErrNotFound
	}
	r.Status = status
	r.Decision = &decision
	r.DecidedBy = &decidedBy
	r.DecidedAt = &decidedAt
	f.reqs[id] = r
	return r, nil
}

func newGDPRService(t *testing.T) (*Service, *destructionRepo, *fakeErasureRepo, *mockCaseLookup, *mockStorage) {
	t.Helper()
	repo := &destructionRepo{mockRepo: newMockRepo()}
	erasureRepo := newFakeErasureRepo()
	cases := &mockCaseLookup{status: "archived"}
	storage := newMockStorage()
	svc := &Service{
		repo:    repo,
		storage: storage,
		custody: &mockCustody{},
		cases:   cases,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc.WithErasureRepo(erasureRepo)
	return svc, repo, erasureRepo, cases, storage
}

func TestCreateErasureRequest_ConflictFree(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)

	req, report, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "subject request #42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.HasConflict {
		t.Errorf("expected no conflict, got %+v", report)
	}
	if req.Status != ErasureStatusReady {
		t.Errorf("status = %q, want ready", req.Status)
	}
	if len(erasureRepo.reqs) != 1 {
		t.Errorf("expected 1 persisted request, got %d", len(erasureRepo.reqs))
	}
}

func TestCreateErasureRequest_LegalHoldConflict(t *testing.T) {
	svc, repo, _, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)

	svc.WithLegalHoldChecker(&stubLegalHoldChecker{err: ErrLegalHoldActive})

	req, report, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "subject request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.HasConflict || !report.LegalHold {
		t.Errorf("expected legal-hold conflict, got %+v", report)
	}
	if req.Status != ErasureStatusConflictPending {
		t.Errorf("status = %q, want conflict_pending", req.Status)
	}
}

func TestCreateErasureRequest_RetentionConflict(t *testing.T) {
	svc, repo, _, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	future := time.Now().Add(72 * time.Hour)
	item.RetentionUntil = &future
	repo.items[item.ID] = item

	_, report, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "subject request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.HasConflict || report.RetentionUntil == nil {
		t.Errorf("expected retention conflict, got %+v", report)
	}
}

func TestCreateErasureRequest_CaseActiveConflict(t *testing.T) {
	svc, repo, _, cases, storage := newGDPRService(t)
	cases.status = "active"
	item := seedItem(repo, storage)

	_, report, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "subject request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.HasConflict || !report.CaseActive {
		t.Errorf("expected case-active conflict, got %+v", report)
	}
}

func TestCreateErasureRequest_RationaleRequired(t *testing.T) {
	svc, repo, _, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	_, _, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "   ")
	if err == nil {
		t.Fatal("expected rationale error")
	}
}

func TestResolveErasureConflict_Preserve(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	svc.WithLegalHoldChecker(&stubLegalHoldChecker{err: ErrLegalHoldActive})

	req, _, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "sub")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Clear the legal hold for the resolution flow (still blocked from destruction if we tried),
	// but preserve decision doesn't destroy.
	if err := svc.ResolveErasureConflict(context.Background(), req.ID, ErasureDecisionPreserve, "admin", "not erasable under legal hold"); err != nil {
		t.Fatalf("resolve preserve: %v", err)
	}

	stored := erasureRepo.reqs[req.ID]
	if stored.Status != ErasureStatusResolvedPreserve {
		t.Errorf("status = %q, want resolved_preserve", stored.Status)
	}
	if stored.Decision == nil || *stored.Decision != ErasureDecisionPreserve {
		t.Errorf("decision not recorded")
	}
	// Item should still exist.
	if _, ok := storage.objects[*item.StorageKey]; !ok {
		t.Error("expected item to remain after preserve decision")
	}
}

func TestResolveErasureConflict_Erase(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)

	// Conflict-free request.
	req, _, err := svc.CreateErasureRequest(context.Background(), item.ID, "dpo@x", "sub")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.ResolveErasureConflict(context.Background(), req.ID, ErasureDecisionErase, "admin", "confirmed lawful"); err != nil {
		t.Fatalf("resolve erase: %v", err)
	}

	stored := erasureRepo.reqs[req.ID]
	if stored.Status != ErasureStatusResolvedErase {
		t.Errorf("status = %q, want resolved_erase", stored.Status)
	}
	if _, ok := storage.objects[*item.StorageKey]; ok {
		t.Error("expected item to be destroyed after erase decision")
	}
	after := repo.items[item.ID]
	if after.DestroyedAt == nil {
		t.Error("expected DestroyedAt to be set")
	}
	if after.DestructionAuthority == nil || !contains(*after.DestructionAuthority, "GDPR erasure") {
		t.Errorf("expected authority to cite GDPR, got %v", after.DestructionAuthority)
	}
}

func TestResolveErasureConflict_InvalidDecision(t *testing.T) {
	svc, _, _, _, _ := newGDPRService(t)
	err := svc.ResolveErasureConflict(context.Background(), uuid.New(), "yeet", "admin", "x")
	if err == nil {
		t.Fatal("expected validation error")
	}
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}
