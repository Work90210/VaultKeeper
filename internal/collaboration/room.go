package collaboration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// y-websocket message types.
const (
	msgTypeSync      byte = 0
	msgTypeAwareness byte = 1
)

// Client represents a single WebSocket connection in a room.
type Client struct {
	User      auth.AuthContext
	Conn      *websocket.Conn
	Send      chan []byte
	closeOnce sync.Once
}

// CloseSend safely closes the Send channel exactly once.
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() { close(c.Send) })
}

// Room manages a collaborative editing session for one evidence item.
type Room struct {
	evidenceID uuid.UUID
	caseID     uuid.UUID
	createdBy  string
	store      DraftStore
	logger     *slog.Logger
	onEmpty    func(uuid.UUID)

	mu      sync.RWMutex
	clients map[*Client]struct{}
	updates [][]byte
	dirty   bool
	closed  bool
	stopCh  chan struct{}
}

// NewRoom creates a new collaboration room for an evidence item.
func NewRoom(evidenceID, caseID uuid.UUID, createdBy string, store DraftStore, logger *slog.Logger, onEmpty func(uuid.UUID)) *Room {
	return &Room{
		evidenceID: evidenceID,
		caseID:     caseID,
		createdBy:  createdBy,
		store:      store,
		logger:     logger,
		onEmpty:    onEmpty,
		clients:    make(map[*Client]struct{}),
		updates:    make([][]byte, 0, 64),
		stopCh:     make(chan struct{}),
	}
}

// Load restores persisted Yjs state from the draft store.
func (r *Room) Load(ctx context.Context) error {
	state, err := r.store.LoadDraft(ctx, r.evidenceID)
	if err != nil {
		return fmt.Errorf("load room state: %w", err)
	}
	if state == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates[:0], state)
	return nil
}

// Run is the room's background loop that auto-saves dirty state.
func (r *Room) Run(ctx context.Context) {
	ticker := time.NewTicker(autosaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			r.persist(shutdownCtx)
			cancel()
			return
		case <-r.stopCh:
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			r.persist(shutdownCtx)
			cancel()
			return
		case <-ticker.C:
			r.persist(ctx)
		}
	}
}

// AddClient registers a client and replays the current state.
func (r *Room) AddClient(client *Client) {
	r.mu.Lock()
	r.clients[client] = struct{}{}
	snapshot := cloneSlices(r.updates)
	r.mu.Unlock()

	for _, frame := range snapshot {
		select {
		case client.Send <- frame:
		default:
			r.logger.Warn("client send buffer full during replay",
				"evidence_id", r.evidenceID, "user_id", client.User.UserID)
		}
	}
}

// RemoveClient unregisters a client and closes the room when empty.
func (r *Room) RemoveClient(client *Client) {
	r.mu.Lock()
	delete(r.clients, client)
	remaining := len(r.clients)
	r.mu.Unlock()

	client.CloseSend()

	if remaining == 0 {
		select {
		case <-r.stopCh:
		default:
			close(r.stopCh)
		}
		if r.onEmpty != nil {
			r.onEmpty(r.evidenceID)
		}
	}
}

// HandleMessage processes a y-websocket binary message from a client.
func (r *Room) HandleMessage(_ context.Context, sender *Client, payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty message")
	}

	// Copy payload immediately — the WebSocket library may reuse the buffer
	msg := make([]byte, len(payload))
	copy(msg, payload)

	switch msg[0] {
	case msgTypeSync:
		r.appendUpdate(msg)
		r.broadcast(sender, msg)
	case msgTypeAwareness:
		r.broadcast(sender, msg)
	default:
		r.logger.Debug("ignoring unknown message type", "type", msg[0])
	}

	return nil
}

func (r *Room) appendUpdate(msg []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, msg)
	r.dirty = true
}

func (r *Room) broadcast(sender *Client, msg []byte) {
	r.mu.RLock()
	recipients := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		if c != sender {
			recipients = append(recipients, c)
		}
	}
	r.mu.RUnlock()

	for _, c := range recipients {
		select {
		case c.Send <- msg:
		default:
			r.logger.Warn("dropping message for slow client",
				"evidence_id", r.evidenceID, "user_id", c.User.UserID)
		}
	}
}

func (r *Room) persist(ctx context.Context) {
	r.mu.Lock()
	if !r.dirty || r.closed {
		r.mu.Unlock()
		return
	}
	totalSize := 0
	for _, u := range r.updates {
		totalSize += len(u)
	}
	state := make([]byte, 0, totalSize)
	for _, u := range r.updates {
		state = append(state, u...)
	}
	caseID := r.caseID
	createdBy := r.createdBy
	r.dirty = false
	r.mu.Unlock()

	if err := r.store.SaveDraft(ctx, r.evidenceID, caseID, createdBy, state); err != nil {
		r.mu.Lock()
		r.dirty = true
		r.mu.Unlock()
		r.logger.Error("persist room draft failed",
			"evidence_id", r.evidenceID, "error", err)
	}
}

// Close marks the room as closed and stops the background loop.
func (r *Room) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	r.mu.Unlock()

	select {
	case <-r.stopCh:
	default:
		close(r.stopCh)
	}
}

func cloneSlices(slices [][]byte) [][]byte {
	out := make([][]byte, len(slices))
	for i, s := range slices {
		cp := make([]byte, len(s))
		copy(cp, s)
		out[i] = cp
	}
	return out
}
