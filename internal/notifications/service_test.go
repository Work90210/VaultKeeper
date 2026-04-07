package notifications

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockEmailResolver implements UserEmailResolver for tests.
type mockEmailResolver struct {
	emails map[string]string
	err    error
}

func (m *mockEmailResolver) GetUserEmail(_ context.Context, userID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	email, ok := m.emails[userID]
	if !ok {
		return "", fmt.Errorf("user %s not found", userID)
	}
	return email, nil
}

// notifyMockPool handles the exact sequence of calls Notify makes:
// 1. GetCaseUserIDs: Query → returns user ID strings
// 2. Create: Exec → inserts notification
type notifyMockPool struct {
	caseUserIDs []string
	caseUserErr error
	createErr   error
	created     []Notification
	queryCalls  int
	execCalls   int
}

func (p *notifyMockPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &pgxRow{scanFunc: func(_ ...any) error { return nil }}
}

func (p *notifyMockPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	p.queryCalls++
	if p.caseUserErr != nil {
		return nil, p.caseUserErr
	}
	data := make([][]any, len(p.caseUserIDs))
	for i, id := range p.caseUserIDs {
		data[i] = []any{id}
	}
	return &pgxRows{data: data}, nil
}

func (p *notifyMockPool) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	p.execCalls++
	if p.createErr != nil {
		return pgconn.NewCommandTag(""), p.createErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// delegationMockPool handles delegation methods (ListByUser, MarkRead, etc.)
type delegationMockPool struct {
	// QueryRow results
	queryRowScanFunc func(dest ...any) error

	// Query results
	queryRows [][]any
	queryErr  error

	// Exec results
	execTag pgconn.CommandTag
	execErr error
}

func (p *delegationMockPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &pgxRow{scanFunc: p.queryRowScanFunc}
}

func (p *delegationMockPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	return &pgxRows{data: p.queryRows}, nil
}

func (p *delegationMockPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return p.execTag, p.execErr
}

// --- Tests ---

func TestNotify_EvidenceUploaded(t *testing.T) {
	user1 := uuid.New().String()
	user2 := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{user1, user2}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	caseID := uuid.New()
	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: caseID,
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 2 {
		t.Errorf("expected 2 create calls, got %d", pool.execCalls)
	}
}

func TestNotify_UserAddedToCase(t *testing.T) {
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	targetUser := uuid.New().String()
	err := svc.Notify(context.Background(), NotificationEvent{
		Type:         EventUserAddedToCase,
		CaseID:       uuid.New(),
		Title:        "Added to case",
		Body:         "You were added",
		TargetUserID: targetUser,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 1 {
		t.Errorf("expected 1 create call, got %d", pool.execCalls)
	}
}

func TestNotify_UserAddedToCase_MissingTarget(t *testing.T) {
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventUserAddedToCase,
		CaseID: uuid.New(),
		Title:  "Added to case",
		Body:   "You were added",
	})
	if err == nil {
		t.Fatal("expected error for missing TargetUserID")
	}
}

func TestNotify_IntegrityWarning_GetCaseUsersError(t *testing.T) {
	admin1 := uuid.New().String()
	pool := &notifyMockPool{caseUserErr: errors.New("db error")}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{admin1})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventIntegrityWarning,
		CaseID: uuid.New(),
		Title:  "Integrity warning",
		Body:   "Hash mismatch",
	})
	if err == nil {
		t.Fatal("expected error from GetCaseUserIDs failure in integrity warning")
	}
}

func TestNotify_IntegrityWarning(t *testing.T) {
	admin1 := uuid.New().String()
	caseUser := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{caseUser}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{admin1})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventIntegrityWarning,
		CaseID: uuid.New(),
		Title:  "Integrity warning",
		Body:   "Hash mismatch",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// admin1 + caseUser = 2 notifications
	if pool.execCalls != 2 {
		t.Errorf("expected 2 create calls, got %d", pool.execCalls)
	}
}

func TestNotify_IntegrityWarning_DeduplicatesAdminAndCaseUser(t *testing.T) {
	sharedUser := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{sharedUser}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{sharedUser})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventIntegrityWarning,
		CaseID: uuid.New(),
		Title:  "Integrity warning",
		Body:   "Hash mismatch",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// sharedUser appears in both admins and case users but should be deduplicated
	if pool.execCalls != 1 {
		t.Errorf("expected 1 create call (deduplicated), got %d", pool.execCalls)
	}
}

func TestNotify_LegalHoldChanged(t *testing.T) {
	user1 := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{user1}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventLegalHoldChanged,
		CaseID: uuid.New(),
		Title:  "Legal hold changed",
		Body:   "Hold status updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 1 {
		t.Errorf("expected 1 create call, got %d", pool.execCalls)
	}
}

func TestNotify_RetentionExpiring(t *testing.T) {
	admin := uuid.New().String()
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{admin})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:  EventRetentionExpiring,
		Title: "Retention expiring",
		Body:  "Data will be deleted",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 1 {
		t.Errorf("expected 1 create call, got %d", pool.execCalls)
	}
}

func TestNotify_BackupFailed(t *testing.T) {
	admin1 := uuid.New().String()
	admin2 := uuid.New().String()
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{admin1, admin2})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:  EventBackupFailed,
		Title: "Backup failed",
		Body:  "Backup system error",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 2 {
		t.Errorf("expected 2 create calls, got %d", pool.execCalls)
	}
}

func TestNotify_UnknownEvent_NoRecipients(t *testing.T) {
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:  "unknown_event",
		Title: "Unknown",
		Body:  "Unknown event",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 0 {
		t.Errorf("expected 0 create calls for unknown event, got %d", pool.execCalls)
	}
}

func TestNotify_NoRecipients(t *testing.T) {
	pool := &notifyMockPool{caseUserIDs: []string{}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "No users on case",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool.execCalls != 0 {
		t.Errorf("expected 0 create calls, got %d", pool.execCalls)
	}
}

func TestNotify_InvalidRecipientID_Skipped(t *testing.T) {
	pool := &notifyMockPool{caseUserIDs: []string{"not-a-uuid"}}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid UUID should be skipped, no create call
	if pool.execCalls != 0 {
		t.Errorf("expected 0 create calls for invalid UUID, got %d", pool.execCalls)
	}
}

func TestNotify_CreateError_ContinuesOtherRecipients(t *testing.T) {
	user1 := uuid.New().String()
	user2 := uuid.New().String()
	pool := &notifyMockPool{
		caseUserIDs: []string{user1, user2},
		createErr:   errors.New("db error"),
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	// Notify does not return an error for individual create failures
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both users attempted despite errors
	if pool.execCalls != 2 {
		t.Errorf("expected 2 exec calls (both attempted), got %d", pool.execCalls)
	}
}

func TestNotify_GetCaseUserIDsError(t *testing.T) {
	pool := &notifyMockPool{caseUserErr: errors.New("db error")}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	if err == nil {
		t.Fatal("expected error from GetCaseUserIDs failure")
	}
}

func TestNotify_WithEmailResolver(t *testing.T) {
	userID := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{userID}}
	repo := &Repository{pool: pool}

	resolver := &mockEmailResolver{
		emails: map[string]string{userID: "user@example.com"},
	}
	// Use noop emailer (empty host) so Send returns nil immediately
	emailer := noopEmailer()
	svc := NewService(repo, emailer, resolver, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotify_WithEmailResolver_ResolveError(t *testing.T) {
	userID := uuid.New().String()
	pool := &notifyMockPool{caseUserIDs: []string{userID}}
	repo := &Repository{pool: pool}

	resolver := &mockEmailResolver{err: errors.New("resolve error")}
	svc := NewService(repo, noopEmailer(), resolver, noopLogger(), nil)

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:   EventEvidenceUploaded,
		CaseID: uuid.New(),
		Title:  "Evidence uploaded",
		Body:   "File uploaded",
	})
	// Should not error; email failure is logged but not returned
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNotify_NilCaseID_NotSet(t *testing.T) {
	userID := uuid.New().String()
	pool := &notifyMockPool{}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), []string{userID})

	err := svc.Notify(context.Background(), NotificationEvent{
		Type:  EventBackupFailed,
		Title: "Backup failed",
		Body:  "Error",
		// CaseID is zero value (uuid.Nil)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetUserNotifications(t *testing.T) {
	now := time.Now().UTC()
	pool := &delegationMockPool{
		queryRowScanFunc: func(dest ...any) error {
			if ptr, ok := dest[0].(*int); ok {
				*ptr = 5
			}
			return nil
		},
		queryRows: [][]any{
			{uuid.New(), (*uuid.UUID)(nil), uuid.New(), "test", "Title", "Body", false, now},
		},
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	items, total, err := svc.GetUserNotifications(context.Background(), uuid.New().String(), 25, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestGetUserNotifications_Error(t *testing.T) {
	pool := &delegationMockPool{
		queryRowScanFunc: func(dest ...any) error {
			return errors.New("db error")
		},
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	_, _, err := svc.GetUserNotifications(context.Background(), uuid.New().String(), 25, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkRead(t *testing.T) {
	pool := &delegationMockPool{
		execTag: pgconn.NewCommandTag("UPDATE 1"),
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.MarkRead(context.Background(), uuid.New(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkRead_Error(t *testing.T) {
	pool := &delegationMockPool{
		execErr: errors.New("db error"),
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.MarkRead(context.Background(), uuid.New(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkAllRead(t *testing.T) {
	pool := &delegationMockPool{
		execTag: pgconn.NewCommandTag("UPDATE 3"),
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.MarkAllRead(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkAllRead_Error(t *testing.T) {
	pool := &delegationMockPool{
		execErr: errors.New("db error"),
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	err := svc.MarkAllRead(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetUnreadCount(t *testing.T) {
	pool := &delegationMockPool{
		queryRowScanFunc: func(dest ...any) error {
			if ptr, ok := dest[0].(*int); ok {
				*ptr = 7
			}
			return nil
		},
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	count, err := svc.GetUnreadCount(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("expected 7, got %d", count)
	}
}

func TestGetUnreadCount_Error(t *testing.T) {
	pool := &delegationMockPool{
		queryRowScanFunc: func(dest ...any) error {
			return errors.New("db error")
		},
	}
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)

	_, err := svc.GetUnreadCount(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"empty", nil, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all same", []string{"x", "x", "x"}, []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestNewService_CopiesAdminIDs(t *testing.T) {
	admins := []string{"a", "b"}
	svc := NewService(&Repository{pool: &notifyMockPool{}}, noopEmailer(), nil, noopLogger(), admins)

	// Mutate original slice - should not affect service
	admins[0] = "modified"
	if svc.adminUserIDs[0] == "modified" {
		t.Error("NewService should copy adminUserIDs, not reference the original slice")
	}
}
