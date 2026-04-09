package collaboration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// Hub manages all active collaboration rooms.
type Hub struct {
	store  DraftStore
	logger *slog.Logger

	mu      sync.RWMutex
	rooms   map[uuid.UUID]*Room
	hubCtx  context.Context
	running bool
}

// NewHub creates a new collaboration hub.
func NewHub(store DraftStore, logger *slog.Logger) *Hub {
	return &Hub{
		store:  store,
		logger: logger,
		rooms:  make(map[uuid.UUID]*Room),
	}
}

// Run blocks until ctx is cancelled, then closes all rooms.
func (h *Hub) Run(ctx context.Context) {
	h.mu.Lock()
	h.hubCtx = ctx
	h.running = true
	h.mu.Unlock()

	<-ctx.Done()

	h.mu.Lock()
	defer h.mu.Unlock()
	h.running = false

	for _, room := range h.rooms {
		room.Close()
	}
	h.rooms = make(map[uuid.UUID]*Room)
}

// GetOrCreateRoom returns an existing room or creates a new one for the
// given evidence item.
func (h *Hub) GetOrCreateRoom(ctx context.Context, evidenceID, caseID uuid.UUID, actorID string) (*Room, error) {
	// Fast path: check with read lock
	h.mu.RLock()
	if room, ok := h.rooms[evidenceID]; ok {
		h.mu.RUnlock()
		return room, nil
	}
	h.mu.RUnlock()

	// Slow path: create with write lock (double-check)
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[evidenceID]; ok {
		return room, nil
	}

	room := NewRoom(evidenceID, caseID, actorID, h.store, h.logger, h.removeRoom)
	if err := room.Load(ctx); err != nil {
		return nil, fmt.Errorf("load room: %w", err)
	}

	h.rooms[evidenceID] = room
	go room.Run(h.hubCtx)

	return room, nil
}

func (h *Hub) removeRoom(evidenceID uuid.UUID) {
	h.mu.Lock()
	room, ok := h.rooms[evidenceID]
	if ok {
		delete(h.rooms, evidenceID)
	}
	h.mu.Unlock()

	if ok {
		room.Close()
	}
}
