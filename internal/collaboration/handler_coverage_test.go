package collaboration

// Unit coverage for the collaboration Handler's DB lookup + hub room
// allocation. The WebSocket upgrade path (Collaborate, readPump,
// writePump) is exercised by the integration test suite that spins up
// a real pgxpool + websocket client (see handler_test.go).

import (
	"context"
	"errors"
	"log/slog"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// fakeHandlerDB satisfies handlerDB for lookupCaseID tests.
type fakeHandlerDB struct {
	caseID uuid.UUID
	err    error
}

type fakeHandlerRow struct {
	caseID uuid.UUID
	err    error
}

func (r *fakeHandlerRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*uuid.UUID)) = r.caseID
	return nil
}

func (f *fakeHandlerDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &fakeHandlerRow{caseID: f.caseID, err: f.err}
}

func TestLookupCaseID_Success(t *testing.T) {
	want := uuid.New()
	h := &Handler{
		db:     &fakeHandlerDB{caseID: want},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	got, err := h.lookupCaseID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestLookupCaseID_Error(t *testing.T) {
	boom := errors.New("db down")
	h := &Handler{
		db:     &fakeHandlerDB{err: boom},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	_, err := h.lookupCaseID(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, boom) {
		t.Errorf("want wrapped %v, got %v", boom, err)
	}
}

// Note: hub room reuse path is already covered by room_test.go /
// hub_test.go fixtures that set up a real DraftStore.
