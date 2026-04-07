package backup

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// --- test doubles ---

type stubAuditLogger struct{}

func (s *stubAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

type mockBackupRunner struct {
	runResult BackupResult
	runErr    error

	listResult []BackupInfo
	listErr    error

	verifyErr error
}

func (m *mockBackupRunner) doRunBackup(_ context.Context) (BackupResult, error) {
	return m.runResult, m.runErr
}

func (m *mockBackupRunner) doListBackups(_ context.Context) ([]BackupInfo, error) {
	return m.listResult, m.listErr
}

func (m *mockBackupRunner) doVerifyBackup(_ context.Context, _ uuid.UUID) error {
	return m.verifyErr
}

// testHandler creates a Handler whose runner methods delegate to the mock.
// Since BackupRunner is a concrete struct with database dependencies, we
// wrap the handler methods in a chi router and inject the auth context manually.
func newTestRouter(mock *mockBackupRunner) http.Handler {
	logger := slog.Default()
	audit := &stubAuditLogger{}

	r := chi.NewRouter()

	r.Post("/api/admin/backups/run", func(w http.ResponseWriter, r *http.Request) {
		ac, ok := auth.GetAuthContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		_ = ac

		result, err := mock.doRunBackup(r.Context())
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "backup failed"})
			return
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"id":           result.ID,
			"status":       result.Status,
			"started_at":   result.StartedAt,
			"completed_at": result.CompletedAt,
			"file_count":   result.FileCount,
			"total_size":   result.TotalSize,
		})
	})

	r.Get("/api/admin/backups", func(w http.ResponseWriter, r *http.Request) {
		backups, err := mock.doListBackups(r.Context())
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list backups"})
			return
		}
		respondJSON(w, http.StatusOK, backups)
	})

	r.Get("/api/admin/backups/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		backupID, err := uuid.Parse(idStr)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid backup ID"})
			return
		}

		if err := mock.doVerifyBackup(r.Context(), backupID); err != nil {
			respondJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "verification failed"})
			return
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"id":       backupID,
			"verified": true,
		})
	})

	_ = logger
	_ = audit

	return r
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func withAuthContext(r *http.Request) *http.Request {
	ac := auth.AuthContext{
		UserID:     "test-user-id",
		Email:      "admin@example.com",
		Username:   "admin",
		SystemRole: auth.RoleSystemAdmin,
	}
	ctx := auth.WithAuthContext(r.Context(), ac)
	return r.WithContext(ctx)
}

// --- tests ---

func TestRunBackupHandler_Success(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(5 * time.Second)
	id := uuid.New()

	mock := &mockBackupRunner{
		runResult: BackupResult{
			ID:          id,
			Status:      "completed",
			StartedAt:   now,
			CompletedAt: &completedAt,
			FileCount:   1,
			TotalSize:   12345,
		},
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	req = withAuthContext(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "completed" {
		t.Errorf("status: got %q, want %q", body["status"], "completed")
	}
}

func TestRunBackupHandler_Failure(t *testing.T) {
	mock := &mockBackupRunner{
		runResult: BackupResult{ErrorMessage: "pg_dump failed"},
		runErr:    errors.New("pg_dump failed"),
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	req = withAuthContext(req)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRunBackupHandler_MissingAuthContext(t *testing.T) {
	mock := &mockBackupRunner{}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	// Deliberately do NOT set auth context.
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestListBackupsHandler_ReturnsList(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now().UTC()

	mock := &mockBackupRunner{
		listResult: []BackupInfo{
			{ID: id1, StartedAt: now, Status: "completed", Destination: "/backups"},
			{ID: id2, StartedAt: now.Add(-time.Hour), Status: "failed", Destination: "/backups", Error: "disk full"},
		},
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var backups []BackupInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &backups); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(backups) != 2 {
		t.Errorf("backup count: got %d, want 2", len(backups))
	}
}

func TestListBackupsHandler_EmptyList(t *testing.T) {
	mock := &mockBackupRunner{
		listResult: []BackupInfo{},
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestListBackupsHandler_Error(t *testing.T) {
	mock := &mockBackupRunner{
		listErr: errors.New("database error"),
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestVerifyBackupHandler_Success(t *testing.T) {
	backupID := uuid.New()
	mock := &mockBackupRunner{}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/"+backupID.String()+"/verify", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["verified"] != true {
		t.Errorf("verified: got %v, want true", body["verified"])
	}
}

func TestVerifyBackupHandler_InvalidID(t *testing.T) {
	mock := &mockBackupRunner{}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/not-a-uuid/verify", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestVerifyBackupHandler_VerificationFailed(t *testing.T) {
	backupID := uuid.New()
	mock := &mockBackupRunner{
		verifyErr: errors.New("file not found"),
	}

	router := newTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/"+backupID.String()+"/verify", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestNewHandler(t *testing.T) {
	h := NewHandler(nil, slog.Default(), &stubAuditLogger{})
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestRegisterRoutes(t *testing.T) {
	h := NewHandler(nil, slog.Default(), &stubAuditLogger{})
	r := chi.NewRouter()
	// Should not panic.
	h.RegisterRoutes(r)
}

// --- tests exercising real Handler methods (with dummy pool for error paths) ---

func dummyPoolForHandler(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	if err != nil {
		t.Fatalf("create dummy pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestHandlerRunBackup_NoAuthContext(t *testing.T) {
	pool := dummyPoolForHandler(t)
	runner := NewBackupRunner(pool, "key", t.TempDir(), slog.Default(), nil, nil)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	// No auth context set.
	rec := httptest.NewRecorder()

	h.RunBackup(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlerRunBackup_WithAuthContext_FailsOnDB(t *testing.T) {
	pool := dummyPoolForHandler(t)
	runner := NewBackupRunner(pool, "key", t.TempDir(), slog.Default(), nil, nil)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	req = withAuthContext(req)
	rec := httptest.NewRecorder()

	h.RunBackup(rec, req)

	// RunBackup will fail because pool can't connect; expect 500.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlerListBackups_FailsOnDB(t *testing.T) {
	pool := dummyPoolForHandler(t)
	runner := NewBackupRunner(pool, "key", t.TempDir(), slog.Default(), nil, nil)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups", nil)
	rec := httptest.NewRecorder()

	h.ListBackups(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlerVerifyBackup_InvalidID(t *testing.T) {
	pool := dummyPoolForHandler(t)
	runner := NewBackupRunner(pool, "key", t.TempDir(), slog.Default(), nil, nil)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	// Use chi context to inject URL param.
	r := chi.NewRouter()
	r.Get("/api/admin/backups/{id}/verify", h.VerifyBackup)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/not-a-uuid/verify", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerVerifyBackup_DBFails(t *testing.T) {
	pool := dummyPoolForHandler(t)
	runner := NewBackupRunner(pool, "key", t.TempDir(), slog.Default(), nil, nil)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	r := chi.NewRouter()
	r.Get("/api/admin/backups/{id}/verify", h.VerifyBackup)

	backupID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/"+backupID.String()+"/verify", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// VerifyBackup will fail because pool can't connect; expect 422.
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

// --- Handler tests with mocked BackupRunner internals for success paths ---

func newMockedRunner(t *testing.T) *BackupRunner {
	t.Helper()
	dir := t.TempDir()

	db := &mockHandlerDB{}
	dumper := &mockHandlerDumper{data: []byte("pg_dump data")}

	return &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      "handler-test-key",
		destination: dir,
		logger:      slog.Default(),
	}
}

type mockHandlerDB struct{}

func (m *mockHandlerDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (m *mockHandlerDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockHandlerRows{}, nil
}

func (m *mockHandlerDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockHandlerRow{}
}

type mockHandlerRow struct{}

func (r *mockHandlerRow) Scan(_ ...any) error { return nil }

type mockHandlerRows struct{ called bool }

func (r *mockHandlerRows) Close()                                         {}
func (r *mockHandlerRows) Err() error                                     { return nil }
func (r *mockHandlerRows) CommandTag() pgconn.CommandTag                   { return pgconn.NewCommandTag("") }
func (r *mockHandlerRows) FieldDescriptions() []pgconn.FieldDescription    { return nil }
func (r *mockHandlerRows) RawValues() [][]byte                             { return nil }
func (r *mockHandlerRows) Conn() *pgx.Conn                                { return nil }
func (r *mockHandlerRows) Next() bool                                      { return false }
func (r *mockHandlerRows) Scan(_ ...any) error                             { return nil }
func (r *mockHandlerRows) Values() ([]any, error)                          { return nil, nil }

type mockHandlerDumper struct {
	data []byte
}

func (d *mockHandlerDumper) DumpPostgres(_ context.Context) ([]byte, error) {
	return d.data, nil
}

func TestHandlerRunBackup_SuccessPath(t *testing.T) {
	runner := newMockedRunner(t)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/backups/run", nil)
	req = withAuthContext(req)
	rec := httptest.NewRecorder()

	h.RunBackup(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data envelope, got: %v", body)
	}
	if data["status"] != "completed" {
		t.Errorf("status: got %q, want %q", data["status"], "completed")
	}
}

func TestHandlerListBackups_SuccessPath(t *testing.T) {
	runner := newMockedRunner(t)
	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups", nil)
	rec := httptest.NewRecorder()

	h.ListBackups(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlerVerifyBackup_SuccessPath(t *testing.T) {
	dir := t.TempDir()
	backupID := uuid.New()

	// Write a file so verification passes.
	filePath := dir + "/backup-" + backupID.String() + ".vkbk"
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	db := &mockVerifyDB{backupID: backupID}
	runner := &BackupRunner{
		db:          db,
		fs:          osFS{},
		destination: dir,
		logger:      slog.Default(),
	}

	h := NewHandler(runner, slog.Default(), &stubAuditLogger{})

	r := chi.NewRouter()
	r.Get("/api/admin/backups/{id}/verify", h.VerifyBackup)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/backups/"+backupID.String()+"/verify", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

type mockVerifyDB struct {
	backupID uuid.UUID
}

func (m *mockVerifyDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (m *mockVerifyDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &mockHandlerRows{}, nil
}

func (m *mockVerifyDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &mockHandlerRow2{}
}

type mockHandlerRow2 struct{}

func (r *mockHandlerRow2) Scan(dest ...any) error {
	*(dest[0].(*string)) = "completed"
	*(dest[1].(**int64)) = nil
	return nil
}
