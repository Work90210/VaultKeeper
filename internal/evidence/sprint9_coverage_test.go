package evidence

// Sprint 9 coverage closers. This file drives all remaining uncovered
// branches in the Sprint 9 service/handler surface (destruction.go,
// gdpr.go, gdpr_handler.go, retention.go, tag_handler.go,
// classification.go SQL builder, handler.go access helpers) via
// focused failure-injection tests. It is test-only; no production code
// is added here.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---- classification.go: buildClassificationAccessSQL ----
//
// The public CheckAccess helper is already 100% covered. The SQL-builder
// variant needs to exercise every role branch including the unknown-role
// deny fallback.

func TestBuildClassificationAccessSQL_UnknownRole(t *testing.T) {
	if got := buildClassificationAccessSQL("alien"); got != "1=0" {
		t.Errorf("unknown role must return 1=0, got %q", got)
	}
}

func TestBuildClassificationAccessSQL_AllKnownRoles(t *testing.T) {
	roles := []string{RoleInvestigator, RoleProsecutor, RoleDefence, RoleJudge, RoleObserver, RoleVictimRep}
	for _, r := range roles {
		frag := buildClassificationAccessSQL(r)
		if frag == "" || frag == "1=0" {
			t.Errorf("role %q produced empty/deny fragment: %q", r, frag)
		}
		// Every known role must see public+restricted.
		if !strings.Contains(frag, "classification IN ('public','restricted')") {
			t.Errorf("role %q missing public/restricted clause: %q", r, frag)
		}
	}
}

// ---- retention.go: NotifyExpiringRetention ----

// retentionRepo is a destructionRepo equivalent that also implements
// FindExpiringRetention so NotifyExpiringRetention can be exercised.
type retentionRepo struct {
	*mockRepo
	expiring    []ExpiringRetentionItem
	expiringErr error
}

func (r *retentionRepo) FindExpiringRetention(_ context.Context, _ time.Time) ([]ExpiringRetentionItem, error) {
	return r.expiring, r.expiringErr
}

// scriptedNotifier records each call so tests can assert how many items
// were pushed out.
type scriptedNotifier struct {
	notified []ExpiringRetentionItem
	err      error
}

func (s *scriptedNotifier) NotifyRetentionExpiring(_ context.Context, item ExpiringRetentionItem) error {
	s.notified = append(s.notified, item)
	return s.err
}

func newRetentionService(t *testing.T) (*Service, *retentionRepo, *scriptedNotifier) {
	t.Helper()
	repo := &retentionRepo{mockRepo: newMockRepo()}
	notifier := &scriptedNotifier{}
	svc := &Service{
		repo:              repo,
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		retentionNotifier: notifier,
	}
	return svc, repo, notifier
}

func TestNotifyExpiringRetention_NoItems(t *testing.T) {
	svc, _, notifier := newRetentionService(t)
	n, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Errorf("notified = %d, want 0", n)
	}
	if len(notifier.notified) != 0 {
		t.Errorf("notifier invoked %d times, want 0", len(notifier.notified))
	}
}

func TestNotifyExpiringRetention_RepoError(t *testing.T) {
	svc, repo, _ := newRetentionService(t)
	repo.expiringErr = errors.New("query failed")
	_, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err == nil || !errors.Is(err, repo.expiringErr) {
		t.Errorf("want wrapped repo error, got %v", err)
	}
}

func TestNotifyExpiringRetention_NotifierError_Continues(t *testing.T) {
	svc, repo, notifier := newRetentionService(t)
	repo.expiring = []ExpiringRetentionItem{
		{EvidenceID: uuid.New(), CaseID: uuid.New()},
		{EvidenceID: uuid.New(), CaseID: uuid.New()},
	}
	notifier.err = errors.New("notifier down")
	n, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Errorf("notifier errors must be best-effort, got %v", err)
	}
	// Both items were attempted but the notifier errored on each, so the
	// success counter stayed at 0 (count++ is skipped after `continue`).
	if n != 0 {
		t.Errorf("notified count = %d, want 0 (errors skip count)", n)
	}
	if len(notifier.notified) != 2 {
		t.Errorf("notifier invoked %d times, want 2 (loop must continue)", len(notifier.notified))
	}
}

func TestNotifyExpiringRetention_NilNotifier(t *testing.T) {
	repo := &retentionRepo{mockRepo: newMockRepo()}
	repo.expiring = []ExpiringRetentionItem{{EvidenceID: uuid.New()}}
	svc := &Service{
		repo:   repo,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		// retentionNotifier nil → items are still iterated and counted;
		// the notifier call is simply skipped.
	}
	n, err := svc.NotifyExpiringRetention(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Errorf("nil notifier must not error, got %v", err)
	}
	if n != 1 {
		t.Errorf("nil notifier still counts iterated items, got %d, want 1", n)
	}
}

// ---- destruction.go: checkLegalHold ----

func TestCheckLegalHold_NilCheckerAndCases(t *testing.T) {
	svc := &Service{}
	if err := svc.checkLegalHold(context.Background(), uuid.New()); err != nil {
		t.Errorf("no checker + no cases → no error, got %v", err)
	}
}

func TestCheckLegalHold_CasesGetError(t *testing.T) {
	svc := &Service{cases: &mockCaseLookup{getLegalHoldErr: errors.New("db down")}}
	err := svc.checkLegalHold(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want wrapped error")
	}
	if !strings.Contains(err.Error(), "check legal hold") {
		t.Errorf("want wrap prefix, got %q", err.Error())
	}
}

func TestCheckLegalHold_CheckerNonSentinelError(t *testing.T) {
	dbErr := errors.New("upstream error")
	svc := &Service{legalHoldChecker: &stubLegalHoldChecker{err: dbErr}}
	err := svc.checkLegalHold(context.Background(), uuid.New())
	if !errors.Is(err, dbErr) {
		t.Errorf("want underlying %v, got %v", dbErr, err)
	}
}

// ---- destruction.go: validateDestroyEvidenceInput ----

func TestValidateDestroyEvidenceInput_EmptyActor(t *testing.T) {
	err := validateDestroyEvidenceInput(DestroyEvidenceInput{
		EvidenceID: uuid.New(),
		ActorID:    "   ",
		Authority:  "Court Order 2026-001",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "actor_id" {
		t.Errorf("want actor_id validation error, got %v", err)
	}
}

func TestValidateDestroyEvidenceInput_NilEvidenceID(t *testing.T) {
	err := validateDestroyEvidenceInput(DestroyEvidenceInput{
		EvidenceID: uuid.Nil,
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "evidence_id" {
		t.Errorf("want evidence_id validation error, got %v", err)
	}
}

// ---- destruction.go: DestroyEvidence uncovered branches ----

func TestDestroyEvidence_FindByIDError(t *testing.T) {
	svc, repo, _, _ := newDestructionService(t)
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("db explode")
	}
	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: uuid.New(),
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDestroyEvidence_CaseRetentionError(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)
	repo.caseRetentionErr = errors.New("case retention read failed")
	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	if err == nil {
		t.Fatal("want wrapped case retention error")
	}
}

func TestDestroyEvidence_DestroyWithAuthorityError(t *testing.T) {
	svc, repo, storage, _ := newDestructionService(t)
	item := seedItem(repo, storage)
	repo.destroyAuthorityFn = func(_ context.Context, _ uuid.UUID, _, _ string) error {
		return errors.New("update failed")
	}
	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	if err == nil {
		t.Fatal("want wrapped mark-destroyed error")
	}
}

func TestDestroyEvidence_StorageDeleteError_SucceedsSilently(t *testing.T) {
	svc, repo, storage, custody := newDestructionService(t)
	item := seedItem(repo, storage)
	// Attach a thumbnail so both cleanup branches run.
	thumbKey := "thumb/" + item.ID.String() + ".jpg"
	storage.objects[thumbKey] = []byte("thumb-bytes")
	updated := repo.items[item.ID]
	updated.ThumbnailKey = &thumbKey
	repo.items[item.ID] = updated
	storage.deleteErr = errors.New("s3 unreachable")

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: item.ID,
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	if err != nil {
		t.Errorf("storage errors after DB commit must be silent, got %v", err)
	}
	// Custody event must still fire.
	found := false
	for _, e := range custody.events {
		if e == "destroyed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custody 'destroyed' event must fire before storage cleanup")
	}
}

// repositoryOnly is a Repository implementation that does NOT promote
// DestroyWithAuthority. We embed the Repository interface (not the
// concrete mockRepo struct) so only the 20 methods in the interface
// are reachable on the wrapper — the type assertion to
// DestroyerRepository will correctly fail.
type repositoryOnly struct {
	Repository
}

func (r *repositoryOnly) GetCaseRetention(_ context.Context, _ uuid.UUID) (*time.Time, error) {
	return nil, nil
}

// newRepoWithoutDestroyer builds a repositoryOnly wrapper around a
// mockRepo that has pre-seeded items. The wrapper exposes Repository +
// CaseRetentionReader but not DestroyerRepository, driving the
// type-assertion error branch in DestroyEvidence / destroyEvidenceOverride.
func newRepoWithoutDestroyer(items map[uuid.UUID]EvidenceItem) *repositoryOnly {
	base := newMockRepo()
	for k, v := range items {
		base.items[k] = v
	}
	return &repositoryOnly{Repository: base}
}

func TestDestroyEvidence_RepoNotDestroyer(t *testing.T) {
	// Build a repo that satisfies Repository but NOT DestroyerRepository.
	id := uuid.New()
	key := "k"
	repo := newRepoWithoutDestroyer(map[uuid.UUID]EvidenceItem{
		id: {ID: id, CaseID: uuid.New(), StorageKey: &key},
	})
	svc := &Service{
		repo:    repo,
		storage: newMockStorage(),
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	err := svc.DestroyEvidence(context.Background(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    "actor",
		Authority:  "Court Order 2026-001",
	})
	if err == nil || !strings.Contains(err.Error(), "DestroyerRepository") {
		t.Errorf("want DestroyerRepository type assertion error, got %v", err)
	}
}

// ---- gdpr.go: remaining error branches ----

func TestCreateErasureRequest_EmptyRequestedBy(t *testing.T) {
	svc, _, _, _, _ := newGDPRService(t)
	_, _, err := svc.CreateErasureRequest(context.Background(), uuid.New(), "   ", "rationale")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "requested_by" {
		t.Errorf("want requested_by validation error, got %v", err)
	}
}

func TestCreateErasureRequest_NoRepoConfigured(t *testing.T) {
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	_, _, err := svc.CreateErasureRequest(context.Background(), uuid.New(), "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "erasure repository not configured") {
		t.Errorf("want not-configured error, got %v", err)
	}
}

func TestCreateErasureRequest_FindByIDError(t *testing.T) {
	svc, repo, _, _, _ := newGDPRService(t)
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("not found")
	}
	_, _, err := svc.CreateErasureRequest(context.Background(), uuid.New(), "admin", "reason")
	if err == nil {
		t.Fatal("want wrapped find error")
	}
}

func TestCreateErasureRequest_PersistError(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	erasureRepo.createErr = errors.New("insert failed")
	_, _, err := svc.CreateErasureRequest(context.Background(), item.ID, "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "persist erasure request") {
		t.Errorf("want persist error, got %v", err)
	}
}

// ---- gdpr.go: ResolveErasureConflict + destroyEvidenceOverride ----

func TestResolveErasureConflict_NoRepoConfigured(t *testing.T) {
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := svc.ResolveErasureConflict(context.Background(), uuid.New(), ErasureDecisionPreserve, "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "erasure repository not configured") {
		t.Errorf("want not-configured error, got %v", err)
	}
}

func TestResolveErasureConflict_FindError(t *testing.T) {
	svc, _, erasureRepo, _, _ := newGDPRService(t)
	erasureRepo.findErr = errors.New("find failed")
	err := svc.ResolveErasureConflict(context.Background(), uuid.New(), ErasureDecisionPreserve, "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "find erasure request") {
		t.Errorf("want wrapped find error, got %v", err)
	}
}

func TestResolveErasureConflict_AlreadyResolved(t *testing.T) {
	svc, _, erasureRepo, _, _ := newGDPRService(t)
	id := uuid.New()
	erasureRepo.reqs[id] = ErasureRequest{ID: id, Status: ErasureStatusResolvedPreserve}
	err := svc.ResolveErasureConflict(context.Background(), id, ErasureDecisionPreserve, "admin", "reason")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("want validation error on already-resolved, got %v", err)
	}
}

func TestResolveErasureConflict_LoadEvidenceError(t *testing.T) {
	svc, repo, erasureRepo, _, _ := newGDPRService(t)
	reqID := uuid.New()
	evID := uuid.New()
	erasureRepo.reqs[reqID] = ErasureRequest{
		ID:         reqID,
		EvidenceID: evID,
		Status:     ErasureStatusConflictPending,
	}
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("ev not found")
	}
	err := svc.ResolveErasureConflict(context.Background(), reqID, ErasureDecisionPreserve, "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "load evidence for erasure resolution") {
		t.Errorf("want wrapped load error, got %v", err)
	}
}

func TestResolveErasureConflict_UpdateDecisionError(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	reqID := uuid.New()
	erasureRepo.reqs[reqID] = ErasureRequest{
		ID:         reqID,
		EvidenceID: item.ID,
		Status:     ErasureStatusConflictPending,
	}
	erasureRepo.updateErr = errors.New("update failed")
	err := svc.ResolveErasureConflict(context.Background(), reqID, ErasureDecisionPreserve, "admin", "reason")
	if err == nil || !strings.Contains(err.Error(), "record erasure decision") {
		t.Errorf("want wrapped update error, got %v", err)
	}
}

func TestResolveErasureConflict_EraseSuccess(t *testing.T) {
	svc, repo, erasureRepo, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	reqID := uuid.New()
	erasureRepo.reqs[reqID] = ErasureRequest{
		ID:         reqID,
		EvidenceID: item.ID,
		Status:     ErasureStatusConflictPending,
	}
	err := svc.ResolveErasureConflict(context.Background(), reqID, ErasureDecisionErase, "admin-uuid", "court order applies")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Item should be marked destroyed via destroyEvidenceOverride.
	destroyed := repo.items[item.ID]
	if destroyed.DestroyedAt == nil {
		t.Error("item not marked destroyed")
	}
	if destroyed.DestructionAuthority == nil || !strings.Contains(*destroyed.DestructionAuthority, "GDPR erasure") {
		t.Errorf("authority = %v", destroyed.DestructionAuthority)
	}
}

func TestDestroyEvidenceOverride_ValidationError(t *testing.T) {
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: uuid.Nil, // invalid
		ActorID:    "admin",
		Authority:  "GDPR erasure — x",
	})
	if err == nil {
		t.Fatal("want validation error")
	}
}

func TestDestroyEvidenceOverride_FindError(t *testing.T) {
	repo := &destructionRepo{mockRepo: newMockRepo()}
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("not found")
	}
	svc := &Service{
		repo:    repo,
		storage: newMockStorage(),
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: uuid.New(),
		ActorID:    "admin",
		Authority:  "GDPR erasure — test rationale",
	})
	if err == nil {
		t.Fatal("want find error")
	}
}

func TestDestroyEvidenceOverride_AlreadyDestroyed(t *testing.T) {
	repo := &destructionRepo{mockRepo: newMockRepo()}
	id := uuid.New()
	now := time.Now()
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), DestroyedAt: &now}
	svc := &Service{
		repo:    repo,
		storage: newMockStorage(),
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    "admin",
		Authority:  "GDPR erasure — rationale",
	})
	if err != nil {
		t.Errorf("idempotent on already-destroyed, got %v", err)
	}
}

func TestDestroyEvidenceOverride_RepoNotDestroyer(t *testing.T) {
	id := uuid.New()
	repo := newRepoWithoutDestroyer(map[uuid.UUID]EvidenceItem{
		id: {ID: id, CaseID: uuid.New()},
	})
	svc := &Service{
		repo:    repo,
		storage: newMockStorage(),
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    "admin",
		Authority:  "GDPR erasure — rationale",
	})
	if err == nil || !strings.Contains(err.Error(), "DestroyerRepository") {
		t.Errorf("want DestroyerRepository error, got %v", err)
	}
}

func TestDestroyEvidenceOverride_StorageErrorsLogged(t *testing.T) {
	repo := &destructionRepo{mockRepo: newMockRepo()}
	storage := newMockStorage()
	id := uuid.New()
	key := "evidence/" + id.String() + "/file"
	thumbKey := "thumb/" + id.String() + ".jpg"
	storage.objects[key] = []byte("x")
	storage.objects[thumbKey] = []byte("t")
	storage.deleteErr = errors.New("delete failed")
	repo.items[id] = EvidenceItem{
		ID:           id,
		CaseID:       uuid.New(),
		StorageKey:   &key,
		ThumbnailKey: &thumbKey,
	}
	svc := &Service{
		repo:    repo,
		storage: storage,
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    "admin",
		Authority:  "GDPR erasure — rationale",
	})
	if err != nil {
		t.Errorf("storage errors must be silent, got %v", err)
	}
}

func TestDestroyEvidenceOverride_DestroyWithAuthorityError(t *testing.T) {
	repo := &destructionRepo{mockRepo: newMockRepo()}
	id := uuid.New()
	key := "k"
	repo.items[id] = EvidenceItem{ID: id, CaseID: uuid.New(), StorageKey: &key}
	repo.destroyAuthorityFn = func(_ context.Context, _ uuid.UUID, _, _ string) error {
		return errors.New("db update failed")
	}
	svc := &Service{
		repo:    repo,
		storage: newMockStorage(),
		custody: &mockCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := svc.destroyEvidenceOverride(context.Background(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    "admin",
		Authority:  "GDPR erasure — rationale",
	})
	if err == nil || !strings.Contains(err.Error(), "mark destroyed (override)") {
		t.Errorf("want wrapped error, got %v", err)
	}
}

// ---- gdpr.go: buildConflictReport ----

func TestBuildConflictReport_CaseLookupErrors(t *testing.T) {
	svc, repo, _, cases, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	cases.status = "active" // not archived → case_active blocker
	cases.legalHold = false
	report := svc.buildConflictReport(context.Background(), item)
	if !report.CaseActive {
		t.Error("expected CaseActive=true")
	}
}

func TestBuildConflictReport_LegalHoldViaCheckerNonSentinel(t *testing.T) {
	svc, repo, _, _, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	svc.WithLegalHoldChecker(&stubLegalHoldChecker{err: errors.New("upstream down")})
	// Non-sentinel errors should not set LegalHold=true.
	report := svc.buildConflictReport(context.Background(), item)
	if report.LegalHold {
		t.Error("non-sentinel errors must not flag LegalHold")
	}
}

func TestBuildConflictReport_LegalHoldViaCases(t *testing.T) {
	svc, repo, _, cases, storage := newGDPRService(t)
	item := seedItem(repo, storage)
	cases.legalHold = true
	svc.legalHoldChecker = nil // force cases fallback
	report := svc.buildConflictReport(context.Background(), item)
	if !report.LegalHold {
		t.Error("cases fallback must detect legal hold")
	}
}

// ---- gdpr_handler.go: error mapping branches ----

func TestGDPRHandler_Create_MissingAuthContext(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/erasure-requests", strings.NewReader(`{"rationale":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	// no auth context
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/erasure-requests", handler.CreateErasureRequest)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGDPRHandler_Create_InvalidEvidenceID(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/not-uuid/erasure-requests", strings.NewReader(`{"rationale":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/erasure-requests", handler.CreateErasureRequest)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGDPRHandler_Create_InvalidJSONBody(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/erasure-requests", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/erasure-requests", handler.CreateErasureRequest)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGDPRHandler_Create_ServiceValidationError(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	// empty rationale → service returns ValidationError → 400
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/erasure-requests", strings.NewReader(`{"rationale":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/erasure-requests", handler.CreateErasureRequest)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

func TestGDPRHandler_Resolve_MissingAuthContext(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/erasure-requests/"+uuid.New().String()+"/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGDPRHandler_Resolve_InvalidRequestID(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/erasure-requests/not-uuid/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGDPRHandler_Resolve_InvalidJSONBody(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/erasure-requests/"+uuid.New().String()+"/resolve", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGDPRHandler_Resolve_ServiceValidationError(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/erasure-requests/"+uuid.New().String()+"/resolve", strings.NewReader(`{"decision":"","rationale":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400, body: %s", w.Code, w.Body.String())
	}
}

// ---- tag_handler.go: remaining TagMerge/TagRename/TagDelete edge cases ----

func TestTagHandler_Rename_InvalidTagInput(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)
	body := `{"case_id":"` + uuid.New().String() + `","old":"BAD CHARS!","new":"new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Rename_InvalidJSON(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Merge_InvalidJSON(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Merge_EmptySources(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)
	body := `{"case_id":"` + uuid.New().String() + `","sources":[],"target":"target"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Merge_InvalidTarget(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)
	body := `{"case_id":"` + uuid.New().String() + `","sources":["a"],"target":"BAD TARGET!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Delete_InvalidJSON(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/delete", h.TagDelete)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/delete", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagHandler_Autocomplete_MissingAuthContext(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id="+uuid.New().String(), nil)
	// no auth context
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestTagHandler_Rename_MissingAuthContext(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	// no auth context
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestTagHandler_Merge_MissingAuthContext(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestTagHandler_Delete_MissingAuthContext(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)
	r := chi.NewRouter()
	r.Post("/api/evidence/tags/delete", h.TagDelete)
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/delete", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// Small helper so tests don't need json import repeatedly.
var _ = json.Marshal
var _ io.Writer = &bytes.Buffer{}
