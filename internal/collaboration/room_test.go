package collaboration

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockDraftStore is a controllable in-memory DraftStore for unit tests.
type mockDraftStore struct {
	mu       sync.Mutex
	drafts   map[uuid.UUID][]byte
	loadErr  error
	saveErr  error
	saveCalls int
}

func newMockDraftStore() *mockDraftStore {
	return &mockDraftStore{drafts: make(map[uuid.UUID][]byte)}
}

func (m *mockDraftStore) LoadDraft(_ context.Context, evidenceID uuid.UUID) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.drafts[evidenceID], nil
}

func (m *mockDraftStore) SaveDraft(_ context.Context, evidenceID, _ uuid.UUID, _ string, state []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCalls++
	if m.saveErr != nil {
		return m.saveErr
	}
	cp := make([]byte, len(state))
	copy(cp, state)
	m.drafts[evidenceID] = cp
	return nil
}

func (m *mockDraftStore) getSaveCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveCalls
}

func newTestRoom(store DraftStore) *Room {
	evidenceID := uuid.New()
	caseID := uuid.New()
	return NewRoom(evidenceID, caseID, "user-1", store, newTestLogger(), nil)
}

func newTestClient(bufSize int) *Client {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &Client{
		User: auth.AuthContext{UserID: uuid.New().String()},
		Conn: nil, // not used in unit tests
		Send: make(chan []byte, bufSize),
	}
}

// ---------------------------------------------------------------------------
// Room.Load
// ---------------------------------------------------------------------------

func TestRoom_Load_NoDraft(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	if err := room.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(room.updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(room.updates))
	}
}

func TestRoom_Load_WithDraft(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	store.drafts[room.evidenceID] = []byte{0x01, 0x02, 0x03}

	if err := room.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(room.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(room.updates))
	}
	if len(room.updates[0]) != 3 {
		t.Errorf("expected state length 3, got %d", len(room.updates[0]))
	}
}

func TestRoom_Load_StoreError(t *testing.T) {
	store := newMockDraftStore()
	store.loadErr = errors.New("db down")
	room := newTestRoom(store)

	err := room.Load(context.Background())
	if err == nil {
		t.Fatal("expected error from Load, got nil")
	}
}

// ---------------------------------------------------------------------------
// Room.AddClient / RemoveClient
// ---------------------------------------------------------------------------

func TestRoom_AddClient_ReplaysSyncUpdates(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	// Seed the room with a sync update (msgTypeSync = 0x00)
	syncMsg := []byte{msgTypeSync, 0xAB, 0xCD}
	room.updates = append(room.updates, syncMsg)

	client := newTestClient(64)
	room.AddClient(client)

	select {
	case msg := <-client.Send:
		if len(msg) != 3 || msg[0] != msgTypeSync {
			t.Errorf("unexpected replayed message: %v", msg)
		}
	default:
		t.Fatal("expected message in client.Send after AddClient")
	}
}

func TestRoom_AddClient_EmptyRoom_NoReplay(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	client := newTestClient(64)
	room.AddClient(client)

	select {
	case msg := <-client.Send:
		t.Errorf("unexpected message in send channel: %v", msg)
	default:
		// correct: nothing to replay
	}
}

func TestRoom_AddClient_FullBuffer_Warn(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	// Add many updates so replay overflows a size-1 buffer
	for i := 0; i < 5; i++ {
		room.updates = append(room.updates, []byte{msgTypeSync, byte(i)})
	}

	client := newTestClient(1)
	// Should not block or panic even with a full buffer
	room.AddClient(client)
}

func TestRoom_RemoveClient_TriggersOnEmpty(t *testing.T) {
	store := newMockDraftStore()
	called := make(chan uuid.UUID, 1)
	evidenceID := uuid.New()
	room := NewRoom(evidenceID, uuid.New(), "user-1", store, newTestLogger(), func(id uuid.UUID) {
		called <- id
	})

	client := newTestClient(64)
	room.clients[client] = struct{}{}

	room.RemoveClient(client)

	select {
	case id := <-called:
		if id != evidenceID {
			t.Errorf("onEmpty received %v, want %v", id, evidenceID)
		}
	case <-time.After(time.Second):
		t.Fatal("onEmpty was not called")
	}
}

func TestRoom_RemoveClient_MultipleClients_NoOnEmpty(t *testing.T) {
	store := newMockDraftStore()
	emptyCalled := false
	room := NewRoom(uuid.New(), uuid.New(), "user-1", store, newTestLogger(), func(_ uuid.UUID) {
		emptyCalled = true
	})

	c1 := newTestClient(64)
	c2 := newTestClient(64)
	room.clients[c1] = struct{}{}
	room.clients[c2] = struct{}{}

	room.RemoveClient(c1)

	if emptyCalled {
		t.Error("onEmpty should not be called when clients remain")
	}
}

func TestRoom_RemoveClient_ClosesClientSend(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	client := newTestClient(64)
	room.clients[client] = struct{}{}

	room.RemoveClient(client)

	// Channel should be closed; receiving from a closed channel returns zero value
	select {
	case _, ok := <-client.Send:
		if ok {
			t.Error("expected Send channel to be closed")
		}
	default:
		t.Error("expected Send channel to be readable (closed)")
	}
}

func TestRoom_RemoveClient_IdempotentStopCh(t *testing.T) {
	// Calling RemoveClient twice (even from different goroutines) must not panic.
	store := newMockDraftStore()
	room := newTestRoom(store)

	c1 := newTestClient(64)
	room.clients[c1] = struct{}{}

	room.RemoveClient(c1)
	// Second call with empty room — stopCh already closed; must not panic
	room.RemoveClient(c1)
}

// ---------------------------------------------------------------------------
// Room.HandleMessage
// ---------------------------------------------------------------------------

func TestRoom_HandleMessage_EmptyPayload(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	client := newTestClient(64)

	err := room.HandleMessage(context.Background(), client, []byte{})
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestRoom_HandleMessage_SyncMessage_AppendAndBroadcast(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	sender := newTestClient(64)
	receiver := newTestClient(64)
	room.clients[sender] = struct{}{}
	room.clients[receiver] = struct{}{}

	msg := []byte{msgTypeSync, 0x01, 0x02}
	if err := room.HandleMessage(context.Background(), sender, msg); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}

	// Sync update should be appended to room.updates
	if len(room.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(room.updates))
	}
	// Broadcast should have sent to receiver, not sender
	select {
	case got := <-receiver.Send:
		if got[0] != msgTypeSync {
			t.Errorf("unexpected message type in receiver: %d", got[0])
		}
	default:
		t.Fatal("receiver did not get broadcast message")
	}
	select {
	case <-sender.Send:
		t.Error("sender should not receive its own broadcast")
	default:
		// correct
	}
}

func TestRoom_HandleMessage_AwarenessMessage_BroadcastOnly(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	sender := newTestClient(64)
	receiver := newTestClient(64)
	room.clients[sender] = struct{}{}
	room.clients[receiver] = struct{}{}

	msg := []byte{msgTypeAwareness, 0x01}
	if err := room.HandleMessage(context.Background(), sender, msg); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}

	// Awareness should NOT be appended to updates
	if len(room.updates) != 0 {
		t.Errorf("awareness message must not be stored in updates, got %d", len(room.updates))
	}
	// But it should be broadcast
	select {
	case <-receiver.Send:
		// correct
	default:
		t.Fatal("receiver did not get awareness broadcast")
	}
}

func TestRoom_HandleMessage_UnknownType_Ignored(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	client := newTestClient(64)
	room.clients[client] = struct{}{}

	msg := []byte{0xFF, 0x01, 0x02}
	if err := room.HandleMessage(context.Background(), client, msg); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	// No updates appended
	if len(room.updates) != 0 {
		t.Errorf("unknown message type must not modify updates")
	}
	// No broadcast to self
	select {
	case <-client.Send:
		t.Error("client should not receive unknown-type message")
	default:
		// correct
	}
}

func TestRoom_HandleMessage_SlowReceiver_Dropped(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	sender := newTestClient(64)
	// Size-0 buffer: any send will block; channel is full
	slowReceiver := &Client{
		User: auth.AuthContext{UserID: "slow"},
		Send: make(chan []byte, 0),
	}
	room.clients[sender] = struct{}{}
	room.clients[slowReceiver] = struct{}{}

	msg := []byte{msgTypeSync, 0x01}
	// Should not block even though receiver buffer is full
	if err := room.HandleMessage(context.Background(), sender, msg); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
}

func TestRoom_HandleMessage_ImmutableCopy(t *testing.T) {
	// Mutating the original payload after HandleMessage must not affect stored update
	store := newMockDraftStore()
	room := newTestRoom(store)

	sender := newTestClient(64)
	room.clients[sender] = struct{}{}

	original := []byte{msgTypeSync, 0xAA, 0xBB}
	if err := room.HandleMessage(context.Background(), sender, original); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}

	// Mutate original
	original[1] = 0xFF

	if room.updates[0][1] != 0xAA {
		t.Error("stored update was mutated by modifying original payload")
	}
}

// ---------------------------------------------------------------------------
// Room.persist
// ---------------------------------------------------------------------------

func TestRoom_Persist_SkipsWhenNotDirty(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.dirty = false

	room.persist(context.Background())

	if store.getSaveCalls() != 0 {
		t.Error("persist should not save when not dirty")
	}
}

func TestRoom_Persist_SkipsWhenClosed(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.dirty = true
	room.closed = true

	room.persist(context.Background())

	if store.getSaveCalls() != 0 {
		t.Error("persist should not save when room is closed")
	}
}

func TestRoom_Persist_SavesWhenDirty(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.updates = append(room.updates, []byte{msgTypeSync, 0x01})
	room.dirty = true

	room.persist(context.Background())

	if store.getSaveCalls() != 1 {
		t.Errorf("expected 1 save call, got %d", store.getSaveCalls())
	}
	if room.dirty {
		t.Error("room should not be dirty after successful persist")
	}
}

func TestRoom_Persist_RestoresDirtyOnSaveError(t *testing.T) {
	store := newMockDraftStore()
	store.saveErr = errors.New("disk full")
	room := newTestRoom(store)
	room.updates = append(room.updates, []byte{msgTypeSync, 0x01})
	room.dirty = true

	room.persist(context.Background())

	// dirty flag should be restored on error
	room.mu.Lock()
	dirty := room.dirty
	room.mu.Unlock()
	if !dirty {
		t.Error("room should be dirty again after failed persist")
	}
}

func TestRoom_Persist_ConcatenatesMultipleUpdates(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.updates = append(room.updates,
		[]byte{msgTypeSync, 0x01},
		[]byte{msgTypeSync, 0x02},
	)
	room.dirty = true

	room.persist(context.Background())

	saved := store.drafts[room.evidenceID]
	if len(saved) != 4 { // 2 bytes + 2 bytes
		t.Errorf("expected 4 bytes saved, got %d", len(saved))
	}
}

// ---------------------------------------------------------------------------
// Room.Close
// ---------------------------------------------------------------------------

func TestRoom_Close_IdempotentDoesNotPanic(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	room.Close()
	room.Close() // must not panic (double-close)
}

func TestRoom_Close_SetsClosed(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	room.Close()

	room.mu.Lock()
	closed := room.closed
	room.mu.Unlock()

	if !closed {
		t.Error("room.closed should be true after Close()")
	}
}

// ---------------------------------------------------------------------------
// Room.Run (autosave loop)
// ---------------------------------------------------------------------------

func TestRoom_Run_PersistsOnContextCancel(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.updates = append(room.updates, []byte{msgTypeSync, 0x01})
	room.dirty = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		room.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	if store.getSaveCalls() == 0 {
		t.Error("expected persist on context cancel, got none")
	}
}

func TestRoom_Run_AutosaveOnTicker(t *testing.T) {
	// Override autosaveInterval to a short duration so we don't wait 5 seconds.
	orig := autosaveInterval
	autosaveInterval = 10 * time.Millisecond
	defer func() { autosaveInterval = orig }()

	store := newMockDraftStore()
	room := newTestRoom(store)
	room.updates = append(room.updates, []byte{msgTypeSync, 0x01})
	room.dirty = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		room.Run(ctx)
		close(done)
	}()

	// Wait for the ticker to fire and persist at least once
	deadline := time.After(2 * time.Second)
	for {
		if store.getSaveCalls() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("autosave ticker did not fire within deadline")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	<-done
}

func TestRoom_Run_PersistsOnStopCh(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)
	room.updates = append(room.updates, []byte{msgTypeSync, 0x42})
	room.dirty = true

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		room.Run(ctx)
		close(done)
	}()

	close(room.stopCh)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after stopCh closed")
	}

	if store.getSaveCalls() == 0 {
		t.Error("expected persist on stop, got none")
	}
}

// ---------------------------------------------------------------------------
// cloneSlices
// ---------------------------------------------------------------------------

func TestCloneSlices_EmptyInput(t *testing.T) {
	result := cloneSlices(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestCloneSlices_DeepCopy(t *testing.T) {
	original := [][]byte{{0x01, 0x02}, {0x03, 0x04}}
	cloned := cloneSlices(original)

	if len(cloned) != 2 {
		t.Fatalf("expected 2 items, got %d", len(cloned))
	}

	// Mutate clone — original should be unchanged
	cloned[0][0] = 0xFF
	if original[0][0] != 0x01 {
		t.Error("cloneSlices did not produce a deep copy")
	}
}

// ---------------------------------------------------------------------------
// Client.CloseSend
// ---------------------------------------------------------------------------

func TestClient_CloseSend_Idempotent(t *testing.T) {
	client := newTestClient(4)
	client.CloseSend()
	client.CloseSend() // must not panic
}

func TestClient_CloseSend_ChannelClosed(t *testing.T) {
	client := newTestClient(4)
	client.CloseSend()

	_, ok := <-client.Send
	if ok {
		t.Error("expected Send channel to be closed")
	}
}

// ---------------------------------------------------------------------------
// Concurrency stress test
// ---------------------------------------------------------------------------

func TestRoom_HandleMessage_Concurrent(t *testing.T) {
	store := newMockDraftStore()
	room := newTestRoom(store)

	const goroutines = 10
	clients := make([]*Client, goroutines)
	for i := range clients {
		c := newTestClient(128)
		clients[i] = c
		room.clients[c] = struct{}{}
	}

	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(sender *Client) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				msg := []byte{msgTypeSync, byte(i)}
				_ = room.HandleMessage(context.Background(), sender, msg)
			}
		}(c)
	}
	wg.Wait()

	// No panic = success; verify some updates were recorded
	room.mu.RLock()
	n := len(room.updates)
	room.mu.RUnlock()
	if n == 0 {
		t.Error("expected some updates to be recorded")
	}
}
