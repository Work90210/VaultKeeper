package collaboration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Hub.NewHub
// ---------------------------------------------------------------------------

func TestNewHub_ReturnsNonNil(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	if hub == nil {
		t.Fatal("NewHub returned nil")
	}
	if hub.rooms == nil {
		t.Error("hub.rooms map is nil")
	}
}

// ---------------------------------------------------------------------------
// Hub.GetOrCreateRoom
// ---------------------------------------------------------------------------

func TestHub_GetOrCreateRoom_CreatesNew(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.mu.Lock()
	hub.hubCtx = ctx
	hub.running = true
	hub.mu.Unlock()

	evidenceID := uuid.New()
	caseID := uuid.New()
	room, err := hub.GetOrCreateRoom(ctx, evidenceID, caseID, "user-1")
	if err != nil {
		t.Fatalf("GetOrCreateRoom error: %v", err)
	}
	if room == nil {
		t.Fatal("expected non-nil room")
	}
}

func TestHub_GetOrCreateRoom_ReturnsSameRoomOnSecondCall(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.mu.Lock()
	hub.hubCtx = ctx
	hub.running = true
	hub.mu.Unlock()

	evidenceID := uuid.New()
	caseID := uuid.New()

	r1, err := hub.GetOrCreateRoom(ctx, evidenceID, caseID, "user-1")
	if err != nil {
		t.Fatalf("first GetOrCreateRoom error: %v", err)
	}

	r2, err := hub.GetOrCreateRoom(ctx, evidenceID, caseID, "user-1")
	if err != nil {
		t.Fatalf("second GetOrCreateRoom error: %v", err)
	}

	if r1 != r2 {
		t.Error("second call should return the same room pointer")
	}
}

func TestHub_GetOrCreateRoom_LoadError_ReturnsError(t *testing.T) {
	store := newMockDraftStore()
	store.loadErr = errors.New("db failure")
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.mu.Lock()
	hub.hubCtx = ctx
	hub.running = true
	hub.mu.Unlock()

	_, err := hub.GetOrCreateRoom(ctx, uuid.New(), uuid.New(), "user-1")
	if err == nil {
		t.Fatal("expected error when LoadDraft fails, got nil")
	}
}

func TestHub_GetOrCreateRoom_Concurrent_SameRoom(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.mu.Lock()
	hub.hubCtx = ctx
	hub.running = true
	hub.mu.Unlock()

	evidenceID := uuid.New()
	caseID := uuid.New()

	const goroutines = 20
	results := make([]*Room, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			room, err := hub.GetOrCreateRoom(ctx, evidenceID, caseID, "user-1")
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
				return
			}
			results[idx] = room
		}(i)
	}
	wg.Wait()

	first := results[0]
	for i, r := range results {
		if r != first {
			t.Errorf("goroutine %d returned different room pointer", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Hub.Run
// ---------------------------------------------------------------------------

func TestHub_Run_SetsRunningAndStopsOnCancel(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	// Give Run a moment to set running = true
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	running := hub.running
	hub.mu.RUnlock()
	if !running {
		t.Error("expected hub.running to be true while context is live")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	hub.mu.RLock()
	running = hub.running
	hub.mu.RUnlock()
	if running {
		t.Error("expected hub.running to be false after context cancel")
	}
}

func TestHub_Run_ClosesAllRooms(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())

	// Pre-populate rooms before Run is called
	evidenceID := uuid.New()
	room := NewRoom(evidenceID, uuid.New(), "user-1", store, newTestLogger(), nil)
	hub.rooms[evidenceID] = room

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-done

	// After Run ends, rooms map should be empty
	hub.mu.RLock()
	n := len(hub.rooms)
	hub.mu.RUnlock()
	if n != 0 {
		t.Errorf("expected 0 rooms after Run, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// Hub.removeRoom
// ---------------------------------------------------------------------------

func TestHub_RemoveRoom_DeletesFromMap(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	evidenceID := uuid.New()
	room := NewRoom(evidenceID, uuid.New(), "user-1", store, newTestLogger(), nil)
	hub.rooms[evidenceID] = room

	hub.removeRoom(evidenceID)

	hub.mu.RLock()
	_, ok := hub.rooms[evidenceID]
	hub.mu.RUnlock()
	if ok {
		t.Error("expected room to be removed from map")
	}
}

func TestHub_RemoveRoom_NonExistentID_NoOp(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())

	// Should not panic
	hub.removeRoom(uuid.New())
}
