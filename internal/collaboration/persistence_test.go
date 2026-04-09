package collaboration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// PostgresDraftStore — unit-tested via the mock interface.
// The interface contract is validated by mockDraftStore in room_test.go.
// Here we verify the concrete type satisfies the interface at compile time
// and test the store constructor.
// ---------------------------------------------------------------------------

// Compile-time check: PostgresDraftStore satisfies DraftStore.
var _ DraftStore = (*PostgresDraftStore)(nil)

func TestNewPostgresDraftStore_NotNil(t *testing.T) {
	// db = nil is intentional for unit tests (we cannot connect to Postgres here).
	// We only verify the constructor does not panic.
	store := NewPostgresDraftStore(nil)
	if store == nil {
		t.Fatal("NewPostgresDraftStore returned nil")
	}
}

// ---------------------------------------------------------------------------
// mockDraftStore — verify mock behaves correctly (used by other tests)
// ---------------------------------------------------------------------------

func TestMockDraftStore_LoadDraft_NoDraft(t *testing.T) {
	store := newMockDraftStore()
	data, err := store.LoadDraft(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data for unknown ID, got %v", data)
	}
}

func TestMockDraftStore_SaveAndLoad(t *testing.T) {
	store := newMockDraftStore()
	evidenceID := uuid.New()
	caseID := uuid.New()

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if err := store.SaveDraft(context.Background(), evidenceID, caseID, "user-1", payload); err != nil {
		t.Fatalf("SaveDraft error: %v", err)
	}

	got, err := store.LoadDraft(context.Background(), evidenceID)
	if err != nil {
		t.Fatalf("LoadDraft error: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), len(got))
	}
	for i, b := range payload {
		if got[i] != b {
			t.Errorf("byte %d: expected %02x, got %02x", i, b, got[i])
		}
	}
}

func TestMockDraftStore_SaveDraft_IsolatedCopy(t *testing.T) {
	// Mutating the saved slice after SaveDraft must not affect what is stored.
	store := newMockDraftStore()
	evidenceID := uuid.New()

	payload := []byte{0x01, 0x02}
	if err := store.SaveDraft(context.Background(), evidenceID, uuid.New(), "user-1", payload); err != nil {
		t.Fatalf("SaveDraft error: %v", err)
	}

	payload[0] = 0xFF // mutate original

	got, _ := store.LoadDraft(context.Background(), evidenceID)
	if got[0] != 0x01 {
		t.Error("SaveDraft did not store an independent copy of the state")
	}
}

func TestMockDraftStore_LoadErr_Propagated(t *testing.T) {
	store := newMockDraftStore()
	store.loadErr = errTestSentinel

	_, err := store.LoadDraft(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from LoadDraft, got nil")
	}
}

func TestMockDraftStore_SaveErr_Propagated(t *testing.T) {
	store := newMockDraftStore()
	store.saveErr = errTestSentinel

	err := store.SaveDraft(context.Background(), uuid.New(), uuid.New(), "user-1", []byte{0x01})
	if err == nil {
		t.Fatal("expected error from SaveDraft, got nil")
	}
}

// errTestSentinel is a shared sentinel error for persistence tests.
var errTestSentinel = errorString("test sentinel error")

type errorString string

func (e errorString) Error() string { return string(e) }
