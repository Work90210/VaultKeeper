package disclosures

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockRepo struct {
	disclosures map[uuid.UUID]Disclosure
	createFn    func(ctx context.Context, d Disclosure) (Disclosure, error)
	findByIDFn  func(ctx context.Context, id uuid.UUID) (Disclosure, error)
	findByCaseFn func(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error)
	evidenceBelongsFn func(ctx context.Context, caseID uuid.UUID, evidenceIDs []uuid.UUID) (bool, error)
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		disclosures: make(map[uuid.UUID]Disclosure),
	}
}

func (m *mockRepo) Create(ctx context.Context, d Disclosure) (Disclosure, error) {
	if m.createFn != nil {
		return m.createFn(ctx, d)
	}
	d.ID = uuid.New()
	d.DisclosedAt = time.Now()
	m.disclosures[d.ID] = d
	return d, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (Disclosure, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	d, ok := m.disclosures[id]
	if !ok {
		return Disclosure{}, ErrNotFound
	}
	return d, nil
}

func (m *mockRepo) FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Disclosure, int, error) {
	if m.findByCaseFn != nil {
		return m.findByCaseFn(ctx, caseID, page)
	}
	var result []Disclosure
	for _, d := range m.disclosures {
		if d.CaseID == caseID {
			result = append(result, d)
		}
	}
	return result, len(result), nil
}

func (m *mockRepo) EvidenceBelongsToCase(ctx context.Context, caseID uuid.UUID, evidenceIDs []uuid.UUID) (bool, error) {
	if m.evidenceBelongsFn != nil {
		return m.evidenceBelongsFn(ctx, caseID, evidenceIDs)
	}
	return true, nil
}

type mockCustody struct {
	evidenceEvents []string
	caseEvents     []string
	evidenceErr    error
	caseErr        error
}

func (m *mockCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ map[string]string) error {
	m.evidenceEvents = append(m.evidenceEvents, action)
	return m.evidenceErr
}

func (m *mockCustody) RecordCaseEvent(_ context.Context, _ uuid.UUID, action, _ string, _ map[string]string) error {
	m.caseEvents = append(m.caseEvents, action)
	return m.caseErr
}

type mockNotifier struct {
	called  bool
	lastTo  string
	notifyErr error
}

func (m *mockNotifier) NotifyDisclosure(_ context.Context, _ uuid.UUID, disclosedTo, _, _ string) error {
	m.called = true
	m.lastTo = disclosedTo
	return m.notifyErr
}

func newTestService(repo Repository, custody CustodyRecorder, notify Notifier) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(repo, custody, notify, logger)
}

// ---------------------------------------------------------------------------
// ValidationError tests
// ---------------------------------------------------------------------------

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "field_name", Message: "is required"}
	got := ve.Error()
	want := "field_name: is required"
	if got != want {
		t.Errorf("ValidationError.Error() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Service.Create tests
// ---------------------------------------------------------------------------

func TestService_Create_Valid(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{}
	notify := &mockNotifier{}
	svc := newTestService(repo, custody, notify)

	caseID := uuid.New()
	evidenceID := uuid.New()
	actorID := uuid.New().String()

	input := CreateDisclosureInput{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evidenceID},
		DisclosedTo: "defence@example.com",
		Notes:       "Initial disclosure",
		Redacted:    false,
	}

	d, err := svc.Create(context.Background(), input, actorID)
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if d.ID == uuid.Nil {
		t.Error("expected non-nil disclosure ID")
	}
	if d.CaseID != caseID {
		t.Errorf("CaseID = %s, want %s", d.CaseID, caseID)
	}
	if d.DisclosedTo != "defence@example.com" {
		t.Errorf("DisclosedTo = %q, want %q", d.DisclosedTo, "defence@example.com")
	}
}

func TestService_Create_CustodyLogged(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{}
	notify := &mockNotifier{}
	svc := newTestService(repo, custody, notify)

	caseID := uuid.New()
	evidenceID1 := uuid.New()
	evidenceID2 := uuid.New()
	actorID := uuid.New().String()

	input := CreateDisclosureInput{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evidenceID1, evidenceID2},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, actorID)
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	// 2 evidence events (one per evidenceID) + 1 case event
	if len(custody.evidenceEvents) != 2 {
		t.Errorf("evidence custody events = %d, want 2", len(custody.evidenceEvents))
	}
	if len(custody.caseEvents) != 1 {
		t.Errorf("case custody events = %d, want 1", len(custody.caseEvents))
	}
	for _, ev := range custody.evidenceEvents {
		if ev != "disclosed" {
			t.Errorf("evidence custody action = %q, want %q", ev, "disclosed")
		}
	}
	if custody.caseEvents[0] != "disclosure_created" {
		t.Errorf("case custody action = %q, want %q", custody.caseEvents[0], "disclosure_created")
	}
}

func TestService_Create_NotificationSent(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{}
	notify := &mockNotifier{}
	svc := newTestService(repo, custody, notify)

	caseID := uuid.New()
	actorID := uuid.New().String()

	input := CreateDisclosureInput{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, actorID)
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if !notify.called {
		t.Error("expected notifier to be called")
	}
	if notify.lastTo != "defence@example.com" {
		t.Errorf("notified to %q, want %q", notify.lastTo, "defence@example.com")
	}
}

func TestService_Create_NotificationError_DoesNotFail(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{}
	notify := &mockNotifier{notifyErr: errors.New("smtp failure")}
	svc := newTestService(repo, custody, notify)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err != nil {
		t.Errorf("Create() should succeed even when notify fails, got: %v", err)
	}
}

func TestService_Create_NilNotifier(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{}
	svc := newTestService(repo, custody, nil) // nil notifier

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err != nil {
		t.Errorf("Create() with nil notifier should not fail: %v", err)
	}
}

func TestService_Create_NilCustody(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, nil, nil) // nil custody and notify

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err != nil {
		t.Errorf("Create() with nil custody should not fail: %v", err)
	}
}

func TestService_Create_EmptyEvidenceIDs(t *testing.T) {
	svc := newTestService(newMockRepo(), &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error for empty evidence_ids")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "evidence_ids" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "evidence_ids")
	}
}

func TestService_Create_MissingDisclosedTo(t *testing.T) {
	svc := newTestService(newMockRepo(), &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error for missing disclosed_to")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "disclosed_to" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "disclosed_to")
	}
}

func TestService_Create_MissingCaseID(t *testing.T) {
	svc := newTestService(newMockRepo(), &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.Nil,
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error for nil case_id")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "case_id" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "case_id")
	}
}

func TestService_Create_EvidenceNotInCase(t *testing.T) {
	repo := newMockRepo()
	repo.evidenceBelongsFn = func(_ context.Context, _ uuid.UUID, _ []uuid.UUID) (bool, error) {
		return false, nil
	}
	svc := newTestService(repo, &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when evidence does not belong to case")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "evidence_ids" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "evidence_ids")
	}
}

func TestService_Create_EvidenceBelongsCheckError(t *testing.T) {
	repo := newMockRepo()
	repo.evidenceBelongsFn = func(_ context.Context, _ uuid.UUID, _ []uuid.UUID) (bool, error) {
		return false, errors.New("db error")
	}
	svc := newTestService(repo, &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when evidence belongs check fails")
	}
}

func TestService_Create_InvalidActorID(t *testing.T) {
	svc := newTestService(newMockRepo(), &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid actor UUID")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "disclosed_by" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "disclosed_by")
	}
}

func TestService_Create_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.createFn = func(_ context.Context, _ Disclosure) (Disclosure, error) {
		return Disclosure{}, errors.New("db constraint")
	}
	svc := newTestService(repo, &mockCustody{}, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err == nil {
		t.Fatal("expected error from repo.Create")
	}
}

func TestService_Create_CustodyErrorsDoNotFail(t *testing.T) {
	repo := newMockRepo()
	custody := &mockCustody{
		evidenceErr: errors.New("custody write fail"),
		caseErr:     errors.New("custody write fail"),
	}
	svc := newTestService(repo, custody, nil)

	input := CreateDisclosureInput{
		CaseID:      uuid.New(),
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	_, err := svc.Create(context.Background(), input, uuid.New().String())
	if err != nil {
		t.Errorf("Create() should succeed despite custody errors, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Service.Get tests
// ---------------------------------------------------------------------------

func TestService_Get_Found(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, nil, nil)

	caseID := uuid.New()
	id := uuid.New()
	repo.disclosures[id] = Disclosure{
		ID:          id,
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{uuid.New()},
		DisclosedTo: "defence@example.com",
	}

	got, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if got.ID != id {
		t.Errorf("Get() ID = %s, want %s", got.ID, id)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	svc := newTestService(newMockRepo(), nil, nil)

	_, err := svc.Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Get_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (Disclosure, error) {
		return Disclosure{}, errors.New("db timeout")
	}
	svc := newTestService(repo, nil, nil)

	_, err := svc.Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from repo.FindByID")
	}
}

// ---------------------------------------------------------------------------
// Service.List tests
// ---------------------------------------------------------------------------

func TestService_List_PaginatedResults(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo, nil, nil)

	caseID := uuid.New()
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.disclosures[id] = Disclosure{
			ID:     id,
			CaseID: caseID,
		}
	}

	results, total, err := svc.List(context.Background(), caseID, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("List() len = %d, want 3", len(results))
	}
	if total != 3 {
		t.Errorf("List() total = %d, want 3", total)
	}
}

func TestService_List_EmptyCase(t *testing.T) {
	svc := newTestService(newMockRepo(), nil, nil)

	results, total, err := svc.List(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("List() len = %d, want 0", len(results))
	}
	if total != 0 {
		t.Errorf("List() total = %d, want 0", total)
	}
}

func TestService_List_RepoError(t *testing.T) {
	repo := newMockRepo()
	repo.findByCaseFn = func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Disclosure, int, error) {
		return nil, 0, errors.New("db timeout")
	}
	svc := newTestService(repo, nil, nil)

	_, _, err := svc.List(context.Background(), uuid.New(), Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from repo.FindByCase")
	}
}

// ---------------------------------------------------------------------------
// validateCreateInput edge-case table tests
// ---------------------------------------------------------------------------

func TestValidateCreateInput(t *testing.T) {
	validEvidenceIDs := []uuid.UUID{uuid.New()}

	tests := []struct {
		name      string
		input     CreateDisclosureInput
		wantField string
	}{
		{
			name: "nil case_id",
			input: CreateDisclosureInput{
				CaseID:      uuid.Nil,
				EvidenceIDs: validEvidenceIDs,
				DisclosedTo: "x@example.com",
			},
			wantField: "case_id",
		},
		{
			name: "empty evidence_ids",
			input: CreateDisclosureInput{
				CaseID:      uuid.New(),
				EvidenceIDs: nil,
				DisclosedTo: "x@example.com",
			},
			wantField: "evidence_ids",
		},
		{
			name: "empty disclosed_to",
			input: CreateDisclosureInput{
				CaseID:      uuid.New(),
				EvidenceIDs: validEvidenceIDs,
				DisclosedTo: "",
			},
			wantField: "disclosed_to",
		},
		{
			name: "valid input",
			input: CreateDisclosureInput{
				CaseID:      uuid.New(),
				EvidenceIDs: validEvidenceIDs,
				DisclosedTo: "x@example.com",
			},
			wantField: "",
		},
	}

	svc := newTestService(newMockRepo(), nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateCreateInput(tt.input)
			if tt.wantField == "" {
				if err != nil {
					t.Errorf("validateCreateInput() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validateCreateInput() expected error, got nil")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.wantField)
			}
		})
	}
}
