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

// destructionRepo wraps mockRepo and adds CaseRetentionReader + DestroyerRepository.
type destructionRepo struct {
	*mockRepo
	caseRetention       *time.Time
	caseRetentionErr    error
	destroyAuthorityFn  func(ctx context.Context, id uuid.UUID, authority, actor string) error
	destroyAuthorityLog []destroyAuthorityCall
}

type destroyAuthorityCall struct {
	ID        uuid.UUID
	Authority string
	ActorID   string
}

func (d *destructionRepo) GetCaseRetention(_ context.Context, _ uuid.UUID) (*time.Time, error) {
	if d.caseRetentionErr != nil {
		return nil, d.caseRetentionErr
	}
	return d.caseRetention, nil
}

func (d *destructionRepo) DestroyWithAuthority(ctx context.Context, id uuid.UUID, authority, actor string) error {
	if d.destroyAuthorityFn != nil {
		return d.destroyAuthorityFn(ctx, id, authority, actor)
	}
	d.destroyAuthorityLog = append(d.destroyAuthorityLog, destroyAuthorityCall{ID: id, Authority: authority, ActorID: actor})
	item, ok := d.items[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	item.DestroyedAt = &now
	item.DestroyedBy = &actor
	item.DestructionAuthority = &authority
	item.StorageKey = nil
	d.items[id] = item
	return nil
}

// stubLegalHoldChecker lets tests force a legal-hold block.
type stubLegalHoldChecker struct {
	err error
}

func (s *stubLegalHoldChecker) EnsureNotOnHold(_ context.Context, _ uuid.UUID) error {
	return s.err
}

func newDestructionService(t *testing.T) (*Service, *destructionRepo, *mockStorage, *mockCustody) {
	t.Helper()
	repo := &destructionRepo{mockRepo: newMockRepo()}
	storage := newMockStorage()
	custody := &mockCustody{}
	svc := &Service{
		repo:    repo,
		storage: storage,
		custody: custody,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return svc, repo, storage, custody
}

func seedItem(repo *destructionRepo, storage *mockStorage) EvidenceItem {
	id := uuid.New()
	caseID := uuid.New()
	storageKey := "evidence/" + id.String() + "/file.pdf"
	storage.objects[storageKey] = []byte("payload")
	item := EvidenceItem{
		ID:         id,
		CaseID:     caseID,
		Filename:   "file.pdf",
		SHA256Hash: "abc",
		StorageKey: &storageKey,
		CreatedAt:  time.Now(),
	}
	repo.items[id] = item
	return item
}

func TestDestroyEvidence_AuthorityRequired(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "too short",
	})
	if err == nil {
		t.Fatal("expected validation error for short authority")
	}
	var vErr *ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "authority" {
		t.Errorf("expected ValidationError on authority, got %v", err)
	}
}

func TestDestroyEvidence_EmptyAuthorityRejected(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "   ",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDestroyEvidence_LegalHoldBlocks(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)

	svc.WithLegalHoldChecker(&stubLegalHoldChecker{err: ErrLegalHoldActive})

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "Court order 2030-CV-1",
	})
	if !errors.Is(err, ErrLegalHoldActive) {
		t.Fatalf("expected ErrLegalHoldActive, got %v", err)
	}
	// File must still exist — destruction must not have run.
	if _, ok := storage.objects[*item.StorageKey]; !ok {
		t.Error("expected storage object to remain")
	}
}

func TestDestroyEvidence_RetentionBlocks(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)
	future := time.Now().Add(48 * time.Hour)
	item.RetentionUntil = &future
	repo.items[item.ID] = item

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "Court order 2030-CV-1",
	})
	if !errors.Is(err, ErrRetentionActive) {
		t.Fatalf("expected ErrRetentionActive, got %v", err)
	}
}

func TestDestroyEvidence_CaseRetentionBlocks(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)
	future := time.Now().Add(48 * time.Hour)
	repo.caseRetention = &future

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "Court order 2030-CV-1",
	})
	if !errors.Is(err, ErrRetentionActive) {
		t.Fatalf("expected ErrRetentionActive, got %v", err)
	}
}

func TestDestroyEvidence_Success(t *testing.T) {
	svc, repo, storage, custody := newDestructionService(t)
	item := seedItem(repo, storage)

	svc.WithLegalHoldChecker(&stubLegalHoldChecker{err: nil})

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "Court order 2030-CV-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := storage.objects[*item.StorageKey]; ok {
		t.Error("expected storage object to be deleted")
	}
	if len(repo.destroyAuthorityLog) != 1 {
		t.Fatalf("expected DestroyWithAuthority to be called once, got %d", len(repo.destroyAuthorityLog))
	}
	call := repo.destroyAuthorityLog[0]
	if call.Authority != "Court order 2030-CV-1" {
		t.Errorf("authority = %q", call.Authority)
	}

	// Custody event was emitted.
	found := false
	for _, e := range custody.events {
		if e == "destroyed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected custody event 'destroyed', got %v", custody.events)
	}

	// Metadata preserved (hash remains on the row).
	after := repo.items[item.ID]
	if after.SHA256Hash != "abc" {
		t.Errorf("expected hash preserved, got %q", after.SHA256Hash)
	}
	if after.DestroyedAt == nil {
		t.Error("expected DestroyedAt to be set")
	}
	if after.DestructionAuthority == nil || *after.DestructionAuthority != "Court order 2030-CV-1" {
		t.Errorf("expected authority stored on row")
	}
}

func TestDestroyEvidence_AlreadyDestroyedIdempotent(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)
	now := time.Now()
	item.DestroyedAt = &now
	repo.items[item.ID] = item

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor-1",
		Authority:  "Court order 2030-CV-1",
	})
	if err != nil {
		t.Fatalf("expected idempotent success, got %v", err)
	}
	if len(repo.destroyAuthorityLog) != 0 {
		t.Errorf("expected no destroy call on already-destroyed item")
	}
}
