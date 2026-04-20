package collaboration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type wsTicket struct {
	token     string // the real JWT
	createdAt time.Time
}

// TicketStore issues and redeems short-lived WebSocket tickets so that JWT
// tokens are never transmitted as URL query parameters (where they would be
// captured by access logs).
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]wsTicket
}

// NewTicketStore returns an initialised TicketStore.
func NewTicketStore() *TicketStore {
	return &TicketStore{tickets: make(map[string]wsTicket)}
}

const maxTickets = 10000

// Issue creates a short-lived ticket that can be exchanged for a WebSocket
// connection. The ticket is a 32-byte cryptographically random hex string and
// expires after 60 seconds.
func (ts *TicketStore) Issue(jwtToken string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)

	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Prune expired tickets to prevent unbounded memory growth.
	now := time.Now()
	for k, v := range ts.tickets {
		if now.Sub(v.createdAt) > 60*time.Second {
			delete(ts.tickets, k)
		}
	}

	if len(ts.tickets) >= maxTickets {
		return "", fmt.Errorf("ticket store full")
	}

	ts.tickets[ticket] = wsTicket{token: jwtToken, createdAt: now}
	return ticket, nil
}

// Redeem exchanges a ticket for the JWT, consuming it (single-use). Returns
// false when the ticket is unknown or has expired.
func (ts *TicketStore) Redeem(ticket string) (string, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	t, ok := ts.tickets[ticket]
	if !ok {
		return "", false
	}
	delete(ts.tickets, ticket)

	if time.Since(t.createdAt) > 60*time.Second {
		return "", false
	}
	return t.token, true
}
