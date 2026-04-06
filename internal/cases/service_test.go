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
	cases    map[uuid.UUID]Case
	createFn func(ctx context.Context, c Case) (Case, error)
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

func (m *mockRepo) FindByID(_ context.Context, id uuid.UUID) (Case, error) {
	c, ok := m.cases[id]
	if !ok {
		return Case{}, ErrNotFound
	}
	return c, nil
}

func (m *mockRepo) FindAll(_ context.Context, _ CaseFilter, page Pagination) ([]Case, int, error) {
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
	}, "user-1")

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
	}, "user-1")

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
	}, "user-1")

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
	}, "user-1")

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
	}, "user-1")

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
	}, "user-1")

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
		}, "user-1")
		if err == nil {
			t.Errorf("expected error for reference code %q", ref)
		}
	}
}

func TestService_CreateCase_MissingReferenceCode(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateCase(context.Background(), CreateCaseInput{
		Title: "Valid",
	}, "user-1")
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
