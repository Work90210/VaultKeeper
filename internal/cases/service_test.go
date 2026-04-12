package cases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRepo struct {
	cases      map[uuid.UUID]Case
	createFn   func(ctx context.Context, c Case) (Case, error)
	findByIDFn func(ctx context.Context, id uuid.UUID) (Case, error)
	findAllFn  func(ctx context.Context, filter CaseFilter, page Pagination) ([]Case, int, error)
}

func newMockRepo() *mockRepo {
	return &mockRepo{cases: make(map[uuid.UUID]Case)}
}

func (m *mockRepo) Create(ctx context.Context, c Case) (Case, error) {
	if m.createFn != nil {
		return m.createFn(ctx, c)
	}
	c.ID = uuid.New()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	for _, existing := range m.cases {
		if existing.ReferenceCode == c.ReferenceCode {
			return Case{}, fmt.Errorf("reference code already exists: duplicate key")
		}
	}
	m.cases[c.ID] = c
	return c, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (Case, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	c, ok := m.cases[id]
	if !ok {
		return Case{}, ErrNotFound
	}
	return c, nil
}

func (m *mockRepo) FindAll(ctx context.Context, filter CaseFilter, page Pagination) ([]Case, int, error) {
	if m.findAllFn != nil {
		return m.findAllFn(ctx, filter, page)
	}
	page = ClampPagination(page)
	var result []Case
	for _, c := range m.cases {
		result = append(result, c)
	}
	return result, len(result), nil
}

func (m *mockRepo) Update(_ context.Context, id uuid.UUID, updates UpdateCaseInput) (Case, error) {
	c, ok := m.cases[id]
	if !ok {
		return Case{}, ErrNotFound
	}
	if updates.Title != nil {
		c.Title = *updates.Title
	}
	if updates.Description != nil {
		c.Description = *updates.Description
	}
	if updates.Jurisdiction != nil {
		c.Jurisdiction = *updates.Jurisdiction
	}
	if updates.Status != nil {
		c.Status = *updates.Status
	}
	c.UpdatedAt = time.Now()
	m.cases[id] = c
	return c, nil
}

func (m *mockRepo) Archive(_ context.Context, id uuid.UUID) error {
	c, ok := m.cases[id]
	if !ok {
		return ErrNotFound
	}
	c.Status = StatusArchived
	m.cases[id] = c
	return nil
}

func (m *mockRepo) SetLegalHold(_ context.Context, id uuid.UUID, hold bool) error {
	c, ok := m.cases[id]
	if !ok {
		return ErrNotFound
	}
	c.LegalHold = hold
	m.cases[id] = c
	return nil
}

func (m *mockRepo) CheckLegalHoldStrict(_ context.Context, id uuid.UUID) (bool, error) {
	c, ok := m.cases[id]
	if !ok {
		return false, ErrNotFound
	}
	return c.LegalHold, nil
}

type mockCustody struct {
	events []string
}

func (m *mockCustody) RecordCaseEvent(_ context.Context, caseID uuid.UUID, action string, _ string, _ map[string]string) error {
	m.events = append(m.events, action)
	return nil
}

func newTestService(t *testing.T) (*Service, *mockRepo, *mockCustody) {
	t.Helper()
	repo := newMockRepo()
	custody := &mockCustody{}
	svc, err := NewService(repo, custody, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo, custody
}

func TestService_CreateCase_Valid(t *testing.T) {
	svc, _, custody := newTestService(t)

	c, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-UKR-2024",
		Title:         "Ukraine Investigation",
		Description:   "Test description",
		Jurisdiction:  "ICC",
	}, "user-1", "Test User")

	if err != nil {
		t.Fatalf("CreateCase error: %v", err)
	}
	if c.ReferenceCode != "ICC-UKR-2024" {
		t.Errorf("ReferenceCode = %q", c.ReferenceCode)
	}
	if c.Status != StatusActive {
		t.Errorf("Status = %q, want active", c.Status)
	}
	if len(custody.events) != 1 || custody.events[0] != "case_created" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestService_CreateCase_HTMLEscaped(t *testing.T) {
	svc, _, _ := newTestService(t)

	c, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-XSS-2024",
		Title:         `<script>alert("xss")</script>`,
		Description:   `<img onerror=alert(1)>`,
		Jurisdiction:  "ICC",
	}, "user-1", "Test User")

	if err != nil {
		t.Fatalf("CreateCase error: %v", err)
	}
	if strings.Contains(c.Title, "<script>") {
		t.Errorf("Title should be HTML escaped, got %q", c.Title)
	}
	if strings.Contains(c.Description, "<img") {
		t.Errorf("Description should be HTML escaped, got %q", c.Description)
	}
}

func TestService_CreateCase_EmptyTitle(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         "",
	}, "user-1", "Test User")

	if err == nil {
		t.Fatal("expected error for empty title")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if ve.Field != "title" {
		t.Errorf("Field = %q, want title", ve.Field)
	}
}

func TestService_CreateCase_TitleTooLong(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         strings.Repeat("a", MaxTitleLength+1),
	}, "user-1", "Test User")

	if err == nil {
		t.Fatal("expected error for long title")
	}
}

func TestService_CreateCase_DescriptionTooLong(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         "Valid",
		Description:   strings.Repeat("a", MaxDescriptionLength+1),
	}, "user-1", "Test User")

	if err == nil {
		t.Fatal("expected error for long description")
	}
}

func TestService_CreateCase_JurisdictionTooLong(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         "Valid",
		Jurisdiction:  strings.Repeat("a", MaxJurisdictionLen+1),
	}, "user-1", "Test User")

	if err == nil {
		t.Fatal("expected error for long jurisdiction")
	}
}

func TestService_CreateCase_InvalidReferenceCode(t *testing.T) {
	svc, _, _ := newTestService(t)

	invalid := []string{"invalid", "123", "abc-def-1234", "ICC-ukr-2024", ""}
	for _, ref := range invalid {
		_, err := svc.CreateCase(context.Background(), CreateCaseInput{
			ReferenceCode: ref,
			Title:         "Valid",
		}, "user-1", "Test User")
		if err == nil {
			t.Errorf("expected error for reference code %q", ref)
		}
	}
}

func TestService_CreateCase_MissingReferenceCode(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		Title: "Valid",
	}, "user-1", "Test User")
	if err == nil {
		t.Fatal("expected error for missing reference code")
	}
}

func TestService_GetCase(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusActive}

	c, err := svc.GetCase(context.Background(), id)
	if err != nil {
		t.Fatalf("GetCase error: %v", err)
	}
	if c.Title != "Test" {
		t.Errorf("Title = %q", c.Title)
	}
}

func TestService_GetCase_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetCase(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_UpdateCase_Valid(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	newTitle := "New Title"
	c, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{
		Title: &newTitle,
	}, "user-1")

	if err != nil {
		t.Fatalf("UpdateCase error: %v", err)
	}
	if !strings.Contains(c.Title, "New Title") {
		t.Errorf("Title = %q", c.Title)
	}
	if len(custody.events) != 1 || custody.events[0] != "case_updated" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestService_UpdateCase_InvalidStatusTransition(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusArchived}

	active := StatusActive
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{
		Status: &active,
	}, "user-1")

	if err == nil {
		t.Fatal("expected error for invalid transition archived→active")
	}
}

func TestService_ArchiveCase_WithLegalHold(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusClosed, LegalHold: true}

	err := svc.ArchiveCase(context.Background(), id, "user-1")
	if err == nil {
		t.Fatal("expected error for archiving with legal hold")
	}
	if !strings.Contains(err.Error(), "legal hold") {
		t.Errorf("error = %q, expected legal hold message", err.Error())
	}
}

func TestService_ArchiveCase_InvalidTransition(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive}

	err := svc.ArchiveCase(context.Background(), id, "user-1")
	if err == nil {
		t.Fatal("expected error for active→archived (must close first)")
	}
}

func TestService_ArchiveCase_Idempotent(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusArchived}

	err := svc.ArchiveCase(context.Background(), id, "user-1")
	if err != nil {
		t.Fatalf("expected idempotent archive, got: %v", err)
	}
}

func TestService_ArchiveCase_Success(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusClosed}

	err := svc.ArchiveCase(context.Background(), id, "user-1")
	if err != nil {
		t.Fatalf("ArchiveCase error: %v", err)
	}
	if len(custody.events) != 1 || custody.events[0] != "case_archived" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestService_SetLegalHold(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}

	err := svc.SetLegalHold(context.Background(), id, true, "user-1")
	if err != nil {
		t.Fatalf("SetLegalHold error: %v", err)
	}
	if len(custody.events) != 1 || custody.events[0] != "legal_hold_set" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestService_SetLegalHold_Release(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: true}

	err := svc.SetLegalHold(context.Background(), id, false, "user-1")
	if err != nil {
		t.Fatalf("SetLegalHold error: %v", err)
	}
	if custody.events[0] != "legal_hold_released" {
		t.Errorf("custody event = %q", custody.events[0])
	}
}

func TestService_SetLegalHold_Idempotent(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, LegalHold: true}

	err := svc.SetLegalHold(context.Background(), id, true, "user-1")
	if err != nil {
		t.Fatalf("expected idempotent, got: %v", err)
	}
	if len(custody.events) != 0 {
		t.Error("expected no custody event for idempotent call")
	}
}

func TestService_ListCases(t *testing.T) {
	svc, repo, _ := newTestService(t)
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.cases[id] = Case{ID: id, Title: "Case", Status: StatusActive}
	}

	result, err := svc.ListCases(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("ListCases error: %v", err)
	}
	if result.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", result.TotalCount)
	}
}

func TestNewService_InvalidRegex(t *testing.T) {
	_, err := NewService(newMockRepo(), nil, "[invalid")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "title", Message: "too long"}
	if ve.Error() != "title: too long" {
		t.Errorf("Error() = %q", ve.Error())
	}
}

// ---------------------------------------------------------------------------
// CreateCase — repo.Create error path (L59)
// ---------------------------------------------------------------------------

func TestService_CreateCase_RepoError(t *testing.T) {
	repo := newMockRepo()
	repoErr := fmt.Errorf("db connection lost")
	repo.createFn = func(_ context.Context, _ Case) (Case, error) {
		return Case{}, repoErr
	}
	custody := &mockCustody{}
	svc, _ := NewService(repo, custody, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-UKR-2024",
		Title:         "Test",
	}, "user-1", "Test User")

	if err == nil {
		t.Fatal("expected error from repo.Create, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("error = %v, want %v", err, repoErr)
	}
	if len(custody.events) != 0 {
		t.Error("expected no custody events on create error")
	}
}

// ---------------------------------------------------------------------------
// ListCases — hasMore + nextCursor branch (L87)
// ---------------------------------------------------------------------------

func TestService_ListCases_HasMore(t *testing.T) {
	repo := newMockRepo()
	// Simulate repo returning limit items and a total larger than limit so
	// hasMore is true and nextCursor gets populated.
	limit := 2
	fakeItems := []Case{
		{ID: uuid.New(), Title: "A", Status: StatusActive},
		{ID: uuid.New(), Title: "B", Status: StatusActive},
	}
	repo.findAllFn = func(_ context.Context, _ CaseFilter, _ Pagination) ([]Case, int, error) {
		return fakeItems, 5, nil // 5 total, returning exactly limit items
	}
	custody := &mockCustody{}
	svc, _ := NewService(repo, custody, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	result, err := svc.ListCases(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: limit})
	if err != nil {
		t.Fatalf("ListCases error: %v", err)
	}
	if !result.HasMore {
		t.Error("expected HasMore = true")
	}
	if result.NextCursor == "" {
		t.Error("expected non-empty NextCursor when HasMore is true")
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", result.TotalCount)
	}
}

// ---------------------------------------------------------------------------
// UpdateCase — sanitize description and jurisdiction (L109, L113)
// and custody detail for description, jurisdiction, status changes (L128-L134)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_AllFields(t *testing.T) {
	svc, repo, custody := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	newDesc := "<b>Bold</b>"
	newJur := "  ICC  "
	newStatus := StatusClosed
	c, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{
		Description:  &newDesc,
		Jurisdiction: &newJur,
		Status:       &newStatus,
	}, "user-1")

	if err != nil {
		t.Fatalf("UpdateCase error: %v", err)
	}
	// HTML escaping applied to description
	if strings.Contains(c.Description, "<b>") {
		t.Errorf("Description should be HTML escaped, got %q", c.Description)
	}
	// Whitespace trimmed from jurisdiction
	if strings.Contains(c.Jurisdiction, "  ") {
		t.Errorf("Jurisdiction should be trimmed, got %q", c.Jurisdiction)
	}
	// Custody should record description, jurisdiction, and status changes
	if len(custody.events) == 0 || custody.events[len(custody.events)-1] != "case_updated" {
		t.Errorf("custody events = %v", custody.events)
	}
}

// ---------------------------------------------------------------------------
// ArchiveCase — FindByID error path (L145)
// ---------------------------------------------------------------------------

func TestService_ArchiveCase_FindByIDError(t *testing.T) {
	repo := newMockRepo()
	findErr := fmt.Errorf("db timeout on find")
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (Case, error) {
		return Case{}, findErr
	}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	err := svc.ArchiveCase(context.Background(), uuid.New(), "user-1")
	if err == nil {
		t.Fatal("expected error from FindByID, got nil")
	}
	if !errors.Is(err, findErr) {
		t.Errorf("error = %v, want %v", err, findErr)
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold — FindByID error path (L174)
// ---------------------------------------------------------------------------

func TestService_SetLegalHold_FindByIDError(t *testing.T) {
	repo := newMockRepo()
	findErr := fmt.Errorf("db timeout on find")
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (Case, error) {
		return Case{}, findErr
	}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	err := svc.SetLegalHold(context.Background(), uuid.New(), true, "user-1")
	if err == nil {
		t.Fatal("expected error from FindByID, got nil")
	}
	if !errors.Is(err, findErr) {
		t.Errorf("error = %v, want %v", err, findErr)
	}
}

// ---------------------------------------------------------------------------
// validateUpdateInput — empty title (L231), title too long (L234),
// description too long (L239), jurisdiction too long (L243)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_EmptyTitle(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	empty := ""
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Title: &empty}, "user-1")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Errorf("expected title ValidationError, got %v", err)
	}
}

func TestService_UpdateCase_TitleTooLong(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	long := strings.Repeat("x", MaxTitleLength+1)
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Title: &long}, "user-1")
	if err == nil {
		t.Fatal("expected error for title too long")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Errorf("expected title ValidationError, got %v", err)
	}
}

func TestService_UpdateCase_DescriptionTooLong(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	long := strings.Repeat("x", MaxDescriptionLength+1)
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Description: &long}, "user-1")
	if err == nil {
		t.Fatal("expected error for description too long")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "description" {
		t.Errorf("expected description ValidationError, got %v", err)
	}
}

func TestService_UpdateCase_JurisdictionTooLong(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	long := strings.Repeat("x", MaxJurisdictionLen+1)
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Jurisdiction: &long}, "user-1")
	if err == nil {
		t.Fatal("expected error for jurisdiction too long")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "jurisdiction" {
		t.Errorf("expected jurisdiction ValidationError, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateUpdateInput — FindByID error during status transition check (L249)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_StatusTransition_FindByIDError(t *testing.T) {
	repo := newMockRepo()
	findErr := fmt.Errorf("db timeout during status check")
	repo.findByIDFn = func(_ context.Context, _ uuid.UUID) (Case, error) {
		return Case{}, findErr
	}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	newStatus := StatusClosed
	_, err := svc.UpdateCase(context.Background(), uuid.New(), UpdateCaseInput{Status: &newStatus}, "user-1")
	if err == nil {
		t.Fatal("expected error from FindByID during status check, got nil")
	}
	if !errors.Is(err, findErr) {
		t.Errorf("error = %v, want %v", err, findErr)
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold — archived case (L189)
// ---------------------------------------------------------------------------

func TestService_SetLegalHold_ArchivedCase(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusArchived, LegalHold: false}

	err := svc.SetLegalHold(context.Background(), id, true, "user-1")
	if err == nil {
		t.Fatal("expected error for archived case")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold — repo.SetLegalHold error (L196)
// ---------------------------------------------------------------------------

func TestService_SetLegalHold_RepoError(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	inner.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}
	repo := &errRepo{mockRepo: inner, setLegalHoldErr: fmt.Errorf("db error on set")}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	err := svc.SetLegalHold(context.Background(), id, true, "user-1")
	if err == nil {
		t.Fatal("expected error from repo.SetLegalHold")
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold — nil custody logger (L205)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SetLegalHold — notifies all case members via MemberNotifier
// ---------------------------------------------------------------------------

type fakeMemberNotifier struct {
	calls []fakeNotifyCall
	err   error
}

type fakeNotifyCall struct {
	caseID   uuid.UUID
	newState bool
	actor    string
}

func (f *fakeMemberNotifier) NotifyLegalHoldChanged(_ context.Context, caseID uuid.UUID, newState bool, actor string) error {
	f.calls = append(f.calls, fakeNotifyCall{caseID: caseID, newState: newState, actor: actor})
	return f.err
}

func TestService_SetLegalHold_NotifiesMembers(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}

	notifier := &fakeMemberNotifier{}
	svc.SetMemberNotifier(notifier)

	if err := svc.SetLegalHold(context.Background(), id, true, "user-1"); err != nil {
		t.Fatalf("SetLegalHold error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("notifier calls = %d, want 1", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.caseID != id || call.newState != true || call.actor != "user-1" {
		t.Errorf("notify call = %+v", call)
	}
}

func TestService_SetLegalHold_NotifiesOnRelease(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: true}

	notifier := &fakeMemberNotifier{}
	svc.SetMemberNotifier(notifier)

	if err := svc.SetLegalHold(context.Background(), id, false, "user-2"); err != nil {
		t.Fatalf("SetLegalHold error: %v", err)
	}
	if len(notifier.calls) != 1 || notifier.calls[0].newState != false {
		t.Errorf("expected one release notification, got %+v", notifier.calls)
	}
}

func TestService_SetLegalHold_NotifierErrorIsBestEffort(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}

	notifier := &fakeMemberNotifier{err: fmt.Errorf("notification backend down")}
	svc.SetMemberNotifier(notifier)

	if err := svc.SetLegalHold(context.Background(), id, true, "user-1"); err != nil {
		t.Fatalf("expected notifier error to be swallowed, got: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Errorf("notifier should still have been invoked once, got %d", len(notifier.calls))
	}
}

func TestService_SetLegalHold_IdempotentSkipsNotification(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: true}

	notifier := &fakeMemberNotifier{}
	svc.SetMemberNotifier(notifier)

	if err := svc.SetLegalHold(context.Background(), id, true, "user-1"); err != nil {
		t.Fatalf("SetLegalHold error: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Errorf("expected no notifications for idempotent no-op, got %d", len(notifier.calls))
	}
}

func TestService_SetLegalHold_NilNotifierIsSafe(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}
	// No notifier configured.
	if err := svc.SetLegalHold(context.Background(), id, true, "user-1"); err != nil {
		t.Fatalf("nil notifier must be safe, got: %v", err)
	}
}

func TestService_SetLegalHold_NilCustody(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	err := svc.SetLegalHold(context.Background(), id, true, "user-1")
	if err != nil {
		t.Fatalf("expected nil custody to be handled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateCase — nil custody logger (L64)
// ---------------------------------------------------------------------------

func TestService_CreateCase_NilCustody(t *testing.T) {
	repo := newMockRepo()
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		ReferenceCode: "ICC-NCC-2024",
		Title:         "No Custody",
	}, "user-1", "Test User")

	if err != nil {
		t.Fatalf("expected nil custody to be handled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateCase — nil custody logger (L133)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_NilCustody(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	newTitle := "New"
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Title: &newTitle}, "user-1")
	if err != nil {
		t.Fatalf("expected nil custody to be handled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ArchiveCase — nil custody logger (L175)
// ---------------------------------------------------------------------------

func TestService_ArchiveCase_NilCustody(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusClosed}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	err := svc.ArchiveCase(context.Background(), id, "user-1")
	if err != nil {
		t.Fatalf("expected nil custody to be handled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateUpdateInput — invalid status transition (non-archived case) (L266)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_InvalidStatusTransition_ActiveToArchived(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusActive}

	archived := StatusArchived
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{
		Status: &archived,
	}, "user-1")

	if err == nil {
		t.Fatal("expected error for invalid transition active→archived")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// UpdateCase — update an archived case (L107)
// ---------------------------------------------------------------------------

func TestService_UpdateCase_ArchivedCase(t *testing.T) {
	svc, repo, _ := newTestService(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Archived Case", Status: StatusArchived}

	newTitle := "New Title"
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Title: &newTitle}, "user-1")
	if err == nil {
		t.Fatal("expected error when updating archived case")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("error = %q, want archived message", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ListCases — repo error (L79)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// validateUpdateInput — FindByID error on second call (L262-L264)
// The first FindByID at L102 succeeds (returns active case), but the second
// FindByID inside validateUpdateInput at L262 fails.
// ---------------------------------------------------------------------------

func TestService_UpdateCase_ValidateStatusTransition_SecondFindByIDError(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusActive}

	callCount := 0
	findErr := fmt.Errorf("db timeout on second find")
	repo.findByIDFn = func(_ context.Context, findID uuid.UUID) (Case, error) {
		callCount++
		if callCount <= 1 {
			// First call succeeds (from UpdateCase L102)
			return repo.cases[findID], nil
		}
		// Second call fails (from validateUpdateInput L262)
		return Case{}, findErr
	}

	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	newStatus := StatusClosed
	_, err := svc.UpdateCase(context.Background(), id, UpdateCaseInput{Status: &newStatus}, "user-1")
	if err == nil {
		t.Fatal("expected error from second FindByID, got nil")
	}
	if !errors.Is(err, findErr) {
		t.Errorf("error = %v, want %v", err, findErr)
	}
}

func TestService_ListCases_Error(t *testing.T) {
	repo := newMockRepo()
	repo.findAllFn = func(_ context.Context, _ CaseFilter, _ Pagination) ([]Case, int, error) {
		return nil, 0, fmt.Errorf("db error")
	}
	svc, _ := NewService(repo, nil, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

	_, err := svc.ListCases(context.Background(), CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err == nil {
		t.Fatal("expected error from repo.FindAll")
	}
}
