package integrity

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// StorageFileReader tests
// ---------------------------------------------------------------------------

func TestStorageFileReader_GetObject_Success(t *testing.T) {
	content := "file content"
	sfr := &StorageFileReader{
		GetFn: func(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
			if key != "test-key" {
				t.Errorf("key = %q, want %q", key, "test-key")
			}
			return io.NopCloser(strings.NewReader(content)), int64(len(content)), "text/plain", nil
		},
	}

	rc, err := sfr.GetObject(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != content {
		t.Errorf("data = %q, want %q", string(data), content)
	}
}

func TestStorageFileReader_GetObject_Error(t *testing.T) {
	injected := errors.New("storage error")
	sfr := &StorageFileReader{
		GetFn: func(_ context.Context, _ string) (io.ReadCloser, int64, string, error) {
			return nil, 0, "", injected
		},
	}

	_, err := sfr.GetObject(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want %v", err, injected)
	}
}

// ---------------------------------------------------------------------------
// VerifiableItemAdapter tests
// ---------------------------------------------------------------------------

type testItem struct {
	Name string
	ID   uuid.UUID
}

func TestVerifiableItemAdapter_ListByCaseForVerification_Success(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	caseID := uuid.New()

	adapter := &VerifiableItemAdapter[testItem]{
		ListFn: func(_ context.Context, gotCaseID uuid.UUID) ([]testItem, error) {
			if gotCaseID != caseID {
				t.Errorf("caseID = %v, want %v", gotCaseID, caseID)
			}
			return []testItem{
				{Name: "file1.pdf", ID: id1},
				{Name: "file2.pdf", ID: id2},
			}, nil
		},
		ConvertFn: func(item testItem) VerifiableItem {
			return VerifiableItem{
				ID:       item.ID,
				Filename: item.Name,
			}
		},
	}

	items, err := adapter.ListByCaseForVerification(context.Background(), caseID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].Filename != "file1.pdf" {
		t.Errorf("items[0].Filename = %q", items[0].Filename)
	}
	if items[1].ID != id2 {
		t.Errorf("items[1].ID = %v, want %v", items[1].ID, id2)
	}
}

func TestVerifiableItemAdapter_ListByCaseForVerification_Error(t *testing.T) {
	injected := errors.New("db error")
	adapter := &VerifiableItemAdapter[testItem]{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]testItem, error) {
			return nil, injected
		},
		ConvertFn: func(item testItem) VerifiableItem {
			return VerifiableItem{}
		},
	}

	_, err := adapter.ListByCaseForVerification(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want %v", err, injected)
	}
}

func TestVerifiableItemAdapter_ListByCaseForVerification_Empty(t *testing.T) {
	adapter := &VerifiableItemAdapter[testItem]{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]testItem, error) {
			return []testItem{}, nil
		},
		ConvertFn: func(item testItem) VerifiableItem {
			return VerifiableItem{}
		},
	}

	items, err := adapter.ListByCaseForVerification(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len = %d, want 0", len(items))
	}
}

// ---------------------------------------------------------------------------
// NotificationAdapter tests
// ---------------------------------------------------------------------------

func TestNotificationAdapter_Notify_Success(t *testing.T) {
	var captured NotificationEvent
	adapter := &NotificationAdapter{
		NotifyFn: func(_ context.Context, event NotificationEvent) error {
			captured = event
			return nil
		},
	}

	event := NotificationEvent{
		Type:   "integrity_warning",
		CaseID: uuid.New(),
		Title:  "Alert",
		Body:   "Test body",
	}

	err := adapter.Notify(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Type != "integrity_warning" {
		t.Errorf("captured.Type = %q", captured.Type)
	}
	if captured.Title != "Alert" {
		t.Errorf("captured.Title = %q", captured.Title)
	}
}

func TestNotificationAdapter_Notify_Error(t *testing.T) {
	injected := errors.New("notify error")
	adapter := &NotificationAdapter{
		NotifyFn: func(_ context.Context, _ NotificationEvent) error {
			return injected
		},
	}

	err := adapter.Notify(context.Background(), NotificationEvent{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error = %v, want %v", err, injected)
	}
}
