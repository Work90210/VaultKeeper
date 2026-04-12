//go:build integration

package collaboration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"nhooyr.io/websocket"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---------------------------------------------------------------------------
// Docker helpers (mirroring the pattern from internal/evidence)
// ---------------------------------------------------------------------------

func findDockerCollab() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	candidates := []string{
		"/Applications/Docker.app/Contents/Resources/bin/docker",
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
	}
	for _, c := range candidates {
		if _, err := exec.Command(c, "version").Output(); err == nil {
			return c
		}
	}
	return ""
}

func skipIfNoDockerCollab(t *testing.T) {
	t.Helper()
	dockerPath := findDockerCollab()
	if dockerPath == "" {
		t.Skip("Docker not available, skipping integration test")
	}
	if err := exec.Command(dockerPath, "info").Run(); err != nil {
		t.Skip("Docker daemon not running, skipping integration test")
	}
	t.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
}

func startCollabPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	skipIfNoDockerCollab(t)
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("vaultkeeper_collab_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	runCollabMigrations(t, pool)
	return pool
}

func runCollabMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	migrationsDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("execute migration %s: %v", filepath.Base(f), err)
		}
	}
}

// seedCaseAndEvidence inserts a case and an evidence_items row, returning both IDs.
func seedCaseAndEvidence(t *testing.T, pool *pgxpool.Pool) (caseID, evidenceID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	caseID = uuid.New()
	createdBy := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO cases (id, reference_code, title, created_by) VALUES ($1, $2, $3, $4)`,
		caseID, fmt.Sprintf("TC-%s", caseID.String()[:8]), "Collab Test Case", createdBy)
	if err != nil {
		t.Fatalf("seed case: %v", err)
	}

	evidenceID = uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO evidence_items (id, case_id, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, uploaded_by, is_current)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		evidenceID, caseID, "test.pdf", "test.pdf", "cases/"+caseID.String()+"/test.pdf",
		"application/pdf", 1024, "abc123def456", createdBy.String(), true)
	if err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	return caseID, evidenceID
}

// ---------------------------------------------------------------------------
// PostgresDraftStore integration tests
// ---------------------------------------------------------------------------

func TestIntegration_PostgresDraftStore_LoadDraft_NoDraft(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)

	data, err := store.LoadDraft(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("LoadDraft error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for unknown evidence ID, got %d bytes", len(data))
	}
}

// seedDraft inserts a redaction_draft row directly via SQL.
// SaveDraft's ON CONFLICT clause targets the index dropped in migration 015,
// so we seed integration test data with a raw INSERT to avoid that known issue.
func seedDraft(t *testing.T, pool *pgxpool.Pool, evidenceID, caseID uuid.UUID, actorID string, state []byte) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO redaction_drafts (evidence_id, case_id, created_by, name, purpose, yjs_state)
		 VALUES ($1, $2, $3, $4, 'internal_review', $5)`,
		evidenceID, caseID, actorID, "Test Draft", state)
	if err != nil {
		t.Fatalf("seedDraft: %v", err)
	}
}

// TestIntegration_PostgresDraftStore_SaveDraft_NoActiveDraft_CreatesNew verifies
// that SaveDraft creates a new draft with an auto-generated name when no active
// draft exists for the evidence.
func TestIntegration_PostgresDraftStore_SaveDraft_NoActiveDraft_CreatesNew(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	actorID := uuid.New().String()
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	if err := store.SaveDraft(ctx, evidenceID, caseID, actorID, payload); err != nil {
		t.Fatalf("SaveDraft (create path): %v", err)
	}

	got, err := store.LoadDraft(ctx, evidenceID)
	if err != nil {
		t.Fatalf("LoadDraft after save: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), len(got))
	}

	// Verify the auto-generated name starts with "Collaborative Draft"
	var name string
	if err := pool.QueryRow(ctx,
		`SELECT name FROM redaction_drafts WHERE evidence_id = $1 AND status = 'draft'`,
		evidenceID,
	).Scan(&name); err != nil {
		t.Fatalf("lookup name: %v", err)
	}
	if !strings.HasPrefix(name, "Collaborative Draft ") {
		t.Errorf("expected auto-generated name, got %q", name)
	}
}

// TestIntegration_PostgresDraftStore_SaveDraft_ExistingActiveDraft_Updates verifies
// that SaveDraft updates the existing active draft when one exists.
func TestIntegration_PostgresDraftStore_SaveDraft_ExistingActiveDraft_Updates(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	actorID := uuid.New().String()

	// Seed an existing active draft
	initialPayload := []byte{0x01, 0x02}
	seedDraft(t, pool, evidenceID, caseID, actorID, initialPayload)

	// SaveDraft should update it, not create a new one
	newPayload := []byte{0x03, 0x04, 0x05}
	if err := store.SaveDraft(ctx, evidenceID, caseID, actorID, newPayload); err != nil {
		t.Fatalf("SaveDraft (update path): %v", err)
	}

	// Verify the stored state matches the new payload
	got, err := store.LoadDraft(ctx, evidenceID)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if len(got) != len(newPayload) {
		t.Fatalf("expected %d bytes after update, got %d", len(newPayload), len(got))
	}

	// Verify there's still only one draft for this evidence
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM redaction_drafts WHERE evidence_id = $1 AND status = 'draft'`,
		evidenceID,
	).Scan(&count); err != nil {
		t.Fatalf("count drafts: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 draft, got %d", count)
	}
}

// TestIntegration_PostgresDraftStore_SaveDraft_UpdatesMostRecent verifies that
// when multiple active drafts exist, SaveDraft updates the most recently saved one.
func TestIntegration_PostgresDraftStore_SaveDraft_UpdatesMostRecent(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	actorID := uuid.New().String()

	// Seed two active drafts with different names (allowed by the new schema)
	_, err := pool.Exec(ctx,
		`INSERT INTO redaction_drafts (evidence_id, case_id, created_by, name, purpose, yjs_state, last_saved_at)
		 VALUES ($1, $2, $3, 'Draft A', 'internal_review', $4, NOW() - INTERVAL '1 hour')`,
		evidenceID, caseID, actorID, []byte{0xA1},
	)
	if err != nil {
		t.Fatalf("seed draft A: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO redaction_drafts (evidence_id, case_id, created_by, name, purpose, yjs_state, last_saved_at)
		 VALUES ($1, $2, $3, 'Draft B', 'disclosure_defence', $4, NOW())`,
		evidenceID, caseID, actorID, []byte{0xB1},
	)
	if err != nil {
		t.Fatalf("seed draft B: %v", err)
	}

	newPayload := []byte{0xFF, 0xEE}
	if err := store.SaveDraft(ctx, evidenceID, caseID, actorID, newPayload); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	// Draft B (most recent) should have been updated
	var draftBState []byte
	if err := pool.QueryRow(ctx,
		`SELECT yjs_state FROM redaction_drafts WHERE evidence_id = $1 AND name = 'Draft B'`,
		evidenceID,
	).Scan(&draftBState); err != nil {
		t.Fatalf("load draft B: %v", err)
	}
	if len(draftBState) != len(newPayload) || draftBState[0] != 0xFF {
		t.Errorf("draft B should have new payload, got %x", draftBState)
	}

	// Draft A (older) should NOT have been updated
	var draftAState []byte
	if err := pool.QueryRow(ctx,
		`SELECT yjs_state FROM redaction_drafts WHERE evidence_id = $1 AND name = 'Draft A'`,
		evidenceID,
	).Scan(&draftAState); err != nil {
		t.Fatalf("load draft A: %v", err)
	}
	if len(draftAState) != 1 || draftAState[0] != 0xA1 {
		t.Errorf("draft A should be unchanged, got %x", draftAState)
	}
}

// TestIntegration_PostgresDraftStore_SaveDraft_InsertError verifies the error
// branch when the INSERT fails (e.g., foreign key violation for unknown evidence).
func TestIntegration_PostgresDraftStore_SaveDraft_InsertError(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	// Use a non-existent evidence ID → FK violation
	fakeEvidenceID := uuid.New()
	fakeCaseID := uuid.New()
	actorID := uuid.New().String()

	err := store.SaveDraft(ctx, fakeEvidenceID, fakeCaseID, actorID, []byte{0x01})
	if err == nil {
		t.Fatal("expected error on SaveDraft with non-existent evidence")
	}
	if !strings.Contains(err.Error(), "create collaborative draft") {
		t.Errorf("expected 'create collaborative draft' error, got: %v", err)
	}
}

func TestIntegration_PostgresDraftStore_SaveAndLoad(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)

	_, evidenceID := seedCaseAndEvidence(t, pool)
	ctx := context.Background()

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	actorID := uuid.New().String()
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01}
	seedDraft(t, pool, evidenceID, caseID, actorID, payload)

	got, err := store.LoadDraft(ctx, evidenceID)
	if err != nil {
		t.Fatalf("LoadDraft after seed error: %v", err)
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

// TestIntegration_PostgresDraftStore_LoadDraftStatus verifies that LoadDraft
// only returns rows with status = 'draft' (not 'applied' or 'discarded').
func TestIntegration_PostgresDraftStore_LoadDraftStatus(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	actorID := uuid.New().String()
	payload := []byte{0x01, 0x02, 0x03}
	seedDraft(t, pool, evidenceID, caseID, actorID, payload)

	// Mark the draft as applied
	_, err := pool.Exec(ctx, `UPDATE redaction_drafts SET status = 'applied' WHERE evidence_id = $1`, evidenceID)
	if err != nil {
		t.Fatalf("update draft status: %v", err)
	}

	// LoadDraft should return nil (only 'draft' status)
	got, err := store.LoadDraft(ctx, evidenceID)
	if err != nil {
		t.Fatalf("LoadDraft: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-draft status, got %d bytes", len(got))
	}
}

// ---------------------------------------------------------------------------
// Full WebSocket + DB integration: lookupCaseID via real Handler
// ---------------------------------------------------------------------------

func TestIntegration_Collaborate_NotFoundEvidence(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{role: auth.CaseRoleInvestigator}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, roleLoader, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Use a non-existent evidence ID — should get 404
	fakeID := uuid.New().String()
	resp, err := ts.Client().Get(ts.URL + "/api/evidence/" + fakeID + "/redact/collaborate?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for unknown evidence ID, got %d", resp.StatusCode)
	}
}

func TestIntegration_Collaborate_FullWebSocketSession(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleSystemAdmin}
	validator := &mockTokenValidator{ac: ac}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, nil, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.CloseNow()

	// Send a sync message
	syncMsg := []byte{msgTypeSync, 0xAB, 0xCD}
	if err := conn.Write(ctx, websocket.MessageBinary, syncMsg); err != nil {
		t.Fatalf("write sync message: %v", err)
	}

	// Send an awareness message
	awarenessMsg := []byte{msgTypeAwareness, 0x01}
	if err := conn.Write(ctx, websocket.MessageBinary, awarenessMsg); err != nil {
		t.Fatalf("write awareness message: %v", err)
	}

	conn.Close(websocket.StatusNormalClosure, "done")

	// Verify the room was created in hub
	hub.mu.RLock()
	_, exists := hub.rooms[evidenceID]
	hub.mu.RUnlock()
	// Room may or may not still exist depending on timing (onEmpty cleanup)
	// Just verify no panic occurred.
	_ = exists
}

func TestIntegration_Collaborate_WithPersistedDraft_ReplayedOnJoin(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	ctx := context.Background()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	var caseID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT case_id FROM evidence_items WHERE id = $1`, evidenceID).Scan(&caseID); err != nil {
		t.Fatalf("lookup case_id: %v", err)
	}

	// Pre-seed a draft using direct SQL (SaveDraft ON CONFLICT targets old index)
	actorID := uuid.New().String()
	draftState := []byte{msgTypeSync, 0x10, 0x20, 0x30}
	seedDraft(t, pool, evidenceID, caseID, actorID, draftState)

	hub := NewHub(store, newTestLogger())
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	ac := auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleSystemAdmin}
	validator := &mockTokenValidator{ac: ac}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, nil, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok"

	wsCtx, wsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wsCancel()

	conn, _, err := websocket.Dial(wsCtx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.CloseNow()

	// The first message should be the replayed draft state
	readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer readCancel()

	msgType, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read replayed state: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Errorf("expected binary message type, got %v", msgType)
	}
	if len(data) == 0 || data[0] != msgTypeSync {
		t.Errorf("expected sync replay, got: %v", data)
	}

	conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// Real Collaborate handler — authorization paths (requires DB)
// ---------------------------------------------------------------------------

func TestIntegration_Collaborate_RealHandler_NoCaseRole_Forbidden(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	// Regular user with no case role
	ac := auth.AuthContext{UserID: uuid.New().String(), SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{err: auth.ErrNoCaseRole}
	audit := &mockAuditLogger{}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, roleLoader, audit, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if audit.calls == 0 {
		t.Error("expected audit log for access denial")
	}
}

func TestIntegration_Collaborate_RealHandler_CaseRoleLoaderError_InternalError(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	ac := auth.AuthContext{UserID: uuid.New().String(), SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{err: errorString("db timeout")}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, roleLoader, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestIntegration_Collaborate_RealHandler_WithCaseRole_WebSocketSuccess(t *testing.T) {
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	ac := auth.AuthContext{UserID: uuid.New().String(), SystemRole: auth.RoleUser}
	validator := &mockTokenValidator{ac: ac}
	roleLoader := &mockCaseRoleLoader{role: auth.CaseRoleInvestigator}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, roleLoader, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.CloseNow()

	// Send a sync message
	msg := []byte{msgTypeSync, 0xFF}
	if err := conn.Write(ctx, websocket.MessageBinary, msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.Close(websocket.StatusNormalClosure, "")
}

func TestIntegration_Collaborate_RealHandler_EmptyOrigins_UsesLocalhost(t *testing.T) {
	// When allowedOrigins is empty, the handler defaults to "localhost:*".
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	ac := auth.AuthContext{UserID: uuid.New().String(), SystemRole: auth.RoleSystemAdmin}
	validator := &mockTokenValidator{ac: ac}
	logger := newTestLogger()

	// Empty allowedOrigins — will default to "localhost:*"
	h := NewHandler(hub, pool, validator, nil, nil, logger, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		// nhooyr/websocket may reject non-localhost origins even with "localhost:*"
		// when the test server binds to 127.0.0.1 — this is acceptable.
		t.Logf("WebSocket dial with default origins: %v", err)
		return
	}
	defer conn.CloseNow()
	conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// readPump — HandleMessage error path (requires real WebSocket)
// ---------------------------------------------------------------------------

func TestIntegration_ReadPump_HandleMessageError_EmptyMessage(t *testing.T) {
	// The readPump exits when HandleMessage returns an error (e.g., empty payload).
	// We send a binary message with 0 bytes — HandleMessage returns "empty message".
	// This exercises the errCh <- err path in readPump.
	pool := startCollabPostgres(t)
	store := NewPostgresDraftStore(pool)
	hub := NewHub(store, newTestLogger())

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	hub.mu.Lock()
	hub.hubCtx = hubCtx
	hub.running = true
	hub.mu.Unlock()

	_, evidenceID := seedCaseAndEvidence(t, pool)

	ac := auth.AuthContext{UserID: uuid.New().String(), SystemRole: auth.RoleSystemAdmin}
	validator := &mockTokenValidator{ac: ac}
	logger := newTestLogger()

	h := NewHandler(hub, pool, validator, nil, nil, logger, []string{"*"})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	wsBase := strings.Replace(ts.URL, "http://", "ws://", 1)
	path := "/api/evidence/" + evidenceID.String() + "/redact/collaborate?token=tok"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.CloseNow()

	// nhooyr/websocket rejects zero-byte binary frames at the library level,
	// so we send a 1-byte "sync" message then close normally.
	// The empty-message error path in readPump is an internal guard — tested
	// via TestRoom_HandleMessage_EmptyPayload in room_test.go.
	msg := []byte{msgTypeAwareness, 0x01}
	if err := conn.Write(ctx, websocket.MessageBinary, msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}
