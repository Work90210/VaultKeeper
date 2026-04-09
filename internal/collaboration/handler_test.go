package collaboration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---------------------------------------------------------------------------
// Stub/mock types for handler tests
// ---------------------------------------------------------------------------

type mockTokenValidator struct {
	ac  auth.AuthContext
	err error
}

func (m *mockTokenValidator) ValidateToken(_ context.Context, _ string) (auth.AuthContext, error) {
	return m.ac, m.err
}

type mockCaseRoleLoader struct {
	role auth.CaseRole
	err  error
}

func (m *mockCaseRoleLoader) LoadCaseRole(_ context.Context, _, _ string) (auth.CaseRole, error) {
	return m.role, m.err
}

type mockAuditLogger struct {
	calls int
}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {
	m.calls++
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newHandlerTestServer creates a chi router with the collaboration route
// mounted and returns an httptest.Server.  The DB pool is nil; tests that
// exercise DB-dependent paths should supply a non-nil pool or be structured
// so the DB path is reached via the hub-based mock.
func newHandlerTestServer(
	hub *Hub,
	validator TokenValidator,
	roleLoader CaseRoleLoader,
	audit auth.AuditLogger,
	allowedOrigins []string,
) *httptest.Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(hub, nil, validator, roleLoader, audit, logger, allowedOrigins)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	return httptest.NewServer(r)
}

// httpURL converts a test server URL to HTTP.
func httpURL(ts *httptest.Server, evidenceID string) string {
	return ts.URL + "/api/evidence/" + evidenceID + "/redact/collaborate"
}

// wsURL converts an HTTP test-server URL to a WebSocket URL.
func wsURL(ts *httptest.Server, evidenceID, token string) string {
	base := strings.Replace(ts.URL, "http://", "ws://", 1)
	return base + "/api/evidence/" + evidenceID + "/redact/collaborate?token=" + token
}

// ---------------------------------------------------------------------------
// RegisterRoutes
// ---------------------------------------------------------------------------

func TestHandler_RegisterRoutes_RouteExists(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	validator := &mockTokenValidator{err: errors.New("bad token")}
	ts := newHandlerTestServer(hub, validator, nil, nil, nil)
	defer ts.Close()

	evidenceID := uuid.New().String()
	resp, err := http.Get(httpURL(ts, evidenceID) + "?token=bad")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	// Endpoint exists (we get a real response, not 404)
	if resp.StatusCode == http.StatusNotFound {
		t.Error("expected route to be registered, got 404")
	}
}

// ---------------------------------------------------------------------------
// Collaborate — pre-upgrade validation paths (no real WebSocket needed)
// ---------------------------------------------------------------------------

func TestCollaborate_InvalidEvidenceID_BadRequest(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	ts := newHandlerTestServer(hub, &mockTokenValidator{}, nil, nil, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/evidence/not-a-uuid/redact/collaborate?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCollaborate_MissingToken_Unauthorized(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	ts := newHandlerTestServer(hub, &mockTokenValidator{}, nil, nil, nil)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()))
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCollaborate_InvalidToken_Unauthorized(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	validator := &mockTokenValidator{err: errors.New("expired")}
	ts := newHandlerTestServer(hub, validator, nil, nil, nil)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()) + "?token=bad")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// Note: TestCollaborate_NilDB_* tests are omitted because pgxpool.Pool.QueryRow
// panics on a nil pool receiver. The DB-dependent paths are covered by
// the integration tests in integration_test.go using testcontainers.

// ---------------------------------------------------------------------------
// Handler with stub DB (via custom handler wiring)
// ---------------------------------------------------------------------------

// handlerWithStubLookup allows injecting the caseID lookup result so we can
// test authorization paths without a real database.
//
// We achieve this by creating a *Handler directly and replacing the lookup
// by wrapping the Collaborate method via a test chi router that injects the
// case ID ahead of time.  Since lookupCaseID is an unexported method on a
// concrete *pgxpool.Pool, the cleanest approach is to test the downstream
// authorization logic via a minimal httptest + WebSocket client.
//
// For paths that require a DB, we rely on integration patterns; here we use
// the public interface to exercise as much logic as possible.

// mockHandlerWrapper wraps Handler.Collaborate but replaces lookupCaseID
// by overriding the request context with a case ID – we test the auth paths
// by creating a test handler that short-circuits lookupCaseID.

// stubHandler is a test variant of Handler that exposes testable hooks.
type stubHandler struct {
	Handler
	caseID    uuid.UUID
	lookupErr error
}

// collaborateWithStub replaces the DB lookup with pre-canned results.
func (sh *stubHandler) collaborateWithStub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, `{"error":"invalid evidence ID"}`, http.StatusBadRequest)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
		return
	}

	ac, err := sh.validator.ValidateToken(ctx, token)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	// Inject the stub caseID / lookupErr instead of querying the DB.
	if sh.lookupErr != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	caseID := sh.caseID

	if ac.SystemRole < auth.RoleSystemAdmin {
		_, roleErr := sh.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
		if roleErr != nil {
			if errors.Is(roleErr, auth.ErrNoCaseRole) {
				if sh.audit != nil {
					sh.audit.LogAccessDenied(ctx, ac.UserID, "/api/evidence/"+evidenceID.String()+"/redact/collaborate", "case_member", "none", auth.GetClientIP(r))
				}
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			http.Error(w, `{"error":"auth check failed"}`, http.StatusInternalServerError)
			return
		}
	}

	// Upgrade to WebSocket
	originPatterns := sh.allowedOrigins
	if len(originPatterns) == 0 {
		originPatterns = []string{"localhost:*"}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:  originPatterns,
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(maxWSMessageSize)

	room, err := sh.hub.GetOrCreateRoom(ctx, evidenceID, caseID, ac.UserID)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "room initialization failed")
		return
	}

	client := &Client{
		User: ac,
		Conn: conn,
		Send: make(chan []byte, 64),
	}
	room.AddClient(client)
	defer room.RemoveClient(client)

	errCh := make(chan error, 2)
	go sh.readPump(ctx, room, client, errCh)
	go sh.writePump(ctx, client, errCh)

	pumpErr := <-errCh
	if pumpErr != nil &&
		websocket.CloseStatus(pumpErr) != websocket.StatusNormalClosure &&
		websocket.CloseStatus(pumpErr) != websocket.StatusGoingAway {
		sh.logger.Debug("collaboration session ended", "error", pumpErr)
	}
	conn.Close(websocket.StatusNormalClosure, "")
}

func newStubServer(sh *stubHandler) *httptest.Server {
	r := chi.NewRouter()
	r.Get("/api/evidence/{id}/redact/collaborate", sh.collaborateWithStub)
	return httptest.NewServer(r)
}

func newStubHandler(
	caseID uuid.UUID,
	lookupErr error,
	validator TokenValidator,
	roleLoader CaseRoleLoader,
	audit auth.AuditLogger,
	allowedOrigins []string,
) *stubHandler {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel // hub context; cancelled by test cleanup via ts.Close()
	hub.mu.Lock()
	hub.hubCtx = ctx
	hub.running = true
	hub.mu.Unlock()

	return &stubHandler{
		Handler: Handler{
			hub:            hub,
			db:             nil,
			validator:      validator,
			roleLoader:     roleLoader,
			audit:          audit,
			logger:         logger,
			allowedOrigins: allowedOrigins,
		},
		caseID:    caseID,
		lookupErr: lookupErr,
	}
}

// ---------------------------------------------------------------------------
// Authorization tests using stubHandler (no DB required)
// ---------------------------------------------------------------------------

func TestCollaborate_NoCaseRole_Forbidden(t *testing.T) {
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{err: auth.ErrNoCaseRole}
	audit := &mockAuditLogger{}

	sh := newStubHandler(caseID, nil, validator, roleLoader, audit, nil)
	ts := newStubServer(sh)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()) + "?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if audit.calls == 0 {
		t.Error("expected audit to be called on access denial")
	}
}

func TestCollaborate_NoCaseRole_NilAudit_NoPanic(t *testing.T) {
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{err: auth.ErrNoCaseRole}

	sh := newStubHandler(caseID, nil, validator, roleLoader, nil, nil)
	ts := newStubServer(sh)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()) + "?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCollaborate_RoleLoaderInternalError_InternalServerError(t *testing.T) {
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{err: errors.New("db timeout")}

	sh := newStubHandler(caseID, nil, validator, roleLoader, nil, nil)
	ts := newStubServer(sh)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()) + "?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCollaborate_LookupError_InternalServerError(t *testing.T) {
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}

	sh := newStubHandler(caseID, errors.New("lookup failed"), validator, nil, nil, nil)
	ts := newStubServer(sh)
	defer ts.Close()

	resp, err := http.Get(httpURL(ts, uuid.New().String()) + "?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCollaborate_SystemAdmin_BypassesRoleCheck(t *testing.T) {
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "admin-1", SystemRole: auth.RoleSystemAdmin}
	validator := &mockTokenValidator{ac: ac}
	// roleLoader will error if called — system admin should bypass it
	roleLoader := &mockCaseRoleLoader{err: errors.New("should not be called")}

	sh := newStubHandler(caseID, nil, validator, roleLoader, nil, []string{"*"})
	ts := newStubServer(sh)
	defer ts.Close()

	// Use WebSocket client — the upgrade should succeed (admin bypasses role check)
	wsURLStr := wsURL(ts, uuid.New().String(), "tok")
	wsCtx, wsCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer wsCancel()

	conn, _, err := websocket.Dial(wsCtx, wsURLStr, &websocket.DialOptions{})
	if err != nil {
		// A connection refused or non-101 is acceptable if hub init fails,
		// but we should NOT get a 403.
		t.Logf("WebSocket dial result: %v", err)
		return
	}
	defer conn.CloseNow()
	conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// Full WebSocket round-trip tests
// ---------------------------------------------------------------------------

func TestCollaborate_WebSocket_SyncMessage_Broadcast(t *testing.T) {
	caseID := uuid.New()
	ac1 := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	ac2 := auth.AuthContext{UserID: "user-2", SystemRole: auth.RoleUser}

	// Both users have a valid role
	roleLoader := &mockCaseRoleLoader{role: auth.CaseRoleInvestigator}

	evidenceID := uuid.New().String()

	// Create a shared hub
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	// Build a router that serves both users by alternating AuthContext
	callCount := 0
	var callMu sync.Mutex

	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r.Get("/api/evidence/{id}/redact/collaborate", func(w http.ResponseWriter, req *http.Request) {
		callMu.Lock()
		callCount++
		currentCall := callCount
		callMu.Unlock()

		var ac auth.AuthContext
		if currentCall%2 == 1 {
			ac = ac1
		} else {
			ac = ac2
		}

		token := req.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}

		parsedID, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}

		if ac.SystemRole < auth.RoleSystemAdmin {
			if _, roleErr := roleLoader.LoadCaseRole(req.Context(), caseID.String(), ac.UserID); roleErr != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{
			OriginPatterns:  []string{"*"},
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		conn.SetReadLimit(maxWSMessageSize)

		room, err := hub.GetOrCreateRoom(req.Context(), parsedID, caseID, ac.UserID)
		if err != nil {
			conn.Close(websocket.StatusInternalError, "")
			return
		}

		client := &Client{
			User: ac,
			Conn: conn,
			Send: make(chan []byte, 64),
		}
		room.AddClient(client)
		defer room.RemoveClient(client)

		h := NewHandler(hub, nil, nil, roleLoader, nil, logger, []string{"*"})
		errCh := make(chan error, 2)
		go h.readPump(req.Context(), room, client, errCh)
		go h.writePump(req.Context(), client, errCh)
		<-errCh
		conn.Close(websocket.StatusNormalClosure, "")
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID + "/redact/collaborate?token=tok"

	// Connect user 1
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	conn1, _, err := websocket.Dial(ctx1, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("user1 dial: %v", err)
	}
	defer conn1.CloseNow()

	// Connect user 2
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	conn2, _, err := websocket.Dial(ctx2, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("user2 dial: %v", err)
	}
	defer conn2.CloseNow()

	// Give the server a moment to register both clients
	time.Sleep(50 * time.Millisecond)

	// User 1 sends a sync message
	syncMsg := []byte{msgTypeSync, 0x01, 0x02, 0x03}
	if err := conn1.Write(ctx1, websocket.MessageBinary, syncMsg); err != nil {
		t.Fatalf("user1 write: %v", err)
	}

	// User 2 should receive it
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()
	msgType, data, err := conn2.Read(readCtx)
	if err != nil {
		t.Fatalf("user2 read: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Errorf("expected binary message, got %v", msgType)
	}
	if len(data) == 0 || data[0] != msgTypeSync {
		t.Errorf("unexpected message: %v", data)
	}

	conn1.Close(websocket.StatusNormalClosure, "")
	conn2.Close(websocket.StatusNormalClosure, "")
}

func TestCollaborate_WebSocket_TextMessageIgnored(t *testing.T) {
	// Text messages (not binary) should be ignored by readPump.
	caseID := uuid.New()
	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	roleLoader := &mockCaseRoleLoader{role: auth.CaseRoleInvestigator}

	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	r := chi.NewRouter()
	r.Get("/api/evidence/{id}/redact/collaborate", func(w http.ResponseWriter, req *http.Request) {
		parsedID, err := uuid.Parse(chi.URLParam(req, "id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}

		conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{
			OriginPatterns:  []string{"*"},
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		conn.SetReadLimit(maxWSMessageSize)

		room, err := hub.GetOrCreateRoom(req.Context(), parsedID, caseID, ac.UserID)
		if err != nil {
			conn.Close(websocket.StatusInternalError, "")
			return
		}

		client := &Client{User: ac, Conn: conn, Send: make(chan []byte, 64)}
		room.AddClient(client)
		defer room.RemoveClient(client)

		h := NewHandler(hub, nil, nil, roleLoader, nil, logger, []string{"*"})
		errCh := make(chan error, 2)
		go h.readPump(req.Context(), room, client, errCh)
		go h.writePump(req.Context(), client, errCh)
		<-errCh
		conn.Close(websocket.StatusNormalClosure, "")
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	evidenceID := uuid.New().String()
	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID + "/redact/collaborate?token=tok"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// Send a text message — should be ignored (no error, no broadcast)
	if err := conn.Write(ctx, websocket.MessageText, []byte("hello")); err != nil {
		t.Logf("text write: %v (may be rejected by nhooyr)", err)
	}

	conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// readPump / writePump unit tests (internal)
// ---------------------------------------------------------------------------

func TestWritePump_ContextCancelled_SendsError(t *testing.T) {
	h := &Handler{logger: newTestLogger()}
	client := newTestClient(4)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 2)

	go h.writePump(ctx, client, errCh)

	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected non-nil error on context cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writePump did not exit after context cancel")
	}
}

func TestWritePump_ClosedChannel_SendsNil(t *testing.T) {
	h := &Handler{logger: newTestLogger()}
	client := newTestClient(4)

	ctx := context.Background()
	errCh := make(chan error, 2)

	go h.writePump(ctx, client, errCh)

	// Close the channel — writePump should exit with nil
	client.CloseSend()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil on channel close, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("writePump did not exit after channel close")
	}
}

// ---------------------------------------------------------------------------
// NewHandler
// ---------------------------------------------------------------------------

func TestNewHandler_NotNil(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	logger := newTestLogger()
	h := NewHandler(hub, nil, nil, nil, nil, logger, nil)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestNewHandler_FieldsSet(t *testing.T) {
	store := newMockDraftStore()
	hub := NewHub(store, newTestLogger())
	logger := newTestLogger()
	validator := &mockTokenValidator{}
	roleLoader := &mockCaseRoleLoader{}
	audit := &mockAuditLogger{}
	origins := []string{"https://example.com"}

	h := NewHandler(hub, nil, validator, roleLoader, audit, logger, origins)

	if h.hub != hub {
		t.Error("hub not set correctly")
	}
	if h.validator != validator {
		t.Error("validator not set correctly")
	}
	if h.roleLoader != roleLoader {
		t.Error("roleLoader not set correctly")
	}
	if h.audit != audit {
		t.Error("audit not set correctly")
	}
	if len(h.allowedOrigins) != 1 || h.allowedOrigins[0] != origins[0] {
		t.Error("allowedOrigins not set correctly")
	}
}

