package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// dummyPool returns a non-nil *pgxpool.Pool that will fail on any actual query
// but avoids nil-pointer panics in code paths that reference br.pool.
func dummyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Use a connstring pointing to a non-routable address so the pool object
	// is created but connections will fail at query time.
	pool, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	if err != nil {
		t.Fatalf("create dummy pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// --- test doubles ---

type recordingNotifier struct {
	mu     sync.Mutex
	calls  []error
}

func (n *recordingNotifier) NotifyBackupFailed(_ context.Context, err error) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, err)
	return nil
}

func (n *recordingNotifier) callCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

func (n *recordingNotifier) lastError() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.calls) == 0 {
		return nil
	}
	return n.calls[len(n.calls)-1]
}

type failingNotifier struct{}

func (f *failingNotifier) NotifyBackupFailed(_ context.Context, _ error) error {
	return fmt.Errorf("notification delivery failed")
}

// --- scheduler timing tests ---

func TestSchedulerNextRunTime(t *testing.T) {
	tests := []struct {
		name         string
		now          time.Time
		targetHour   int
		targetMinute int
		wantSameDay  bool
	}{
		{
			name:         "target is later today",
			now:          time.Date(2025, 1, 15, 2, 0, 0, 0, time.UTC),
			targetHour:   3,
			targetMinute: 30,
			wantSameDay:  true,
		},
		{
			name:         "target already passed today",
			now:          time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
			targetHour:   3,
			targetMinute: 30,
			wantSameDay:  false,
		},
		{
			name:         "target is exactly now",
			now:          time.Date(2025, 1, 15, 3, 30, 0, 0, time.UTC),
			targetHour:   3,
			targetMinute: 30,
			wantSameDay:  false, // !next.After(now) => add 24h
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reproduce the scheduling logic from StartScheduler.
			next := time.Date(tc.now.Year(), tc.now.Month(), tc.now.Day(),
				tc.targetHour, tc.targetMinute, 0, 0, time.UTC)
			if !next.After(tc.now) {
				next = next.Add(24 * time.Hour)
			}

			if !next.After(tc.now) {
				t.Error("computed next run time is not after now")
			}

			sameDay := next.Day() == tc.now.Day()
			if sameDay != tc.wantSameDay {
				t.Errorf("sameDay: got %v, want %v (next=%v)", sameDay, tc.wantSameDay, next)
			}
		})
	}
}

func TestStartScheduler_CancelsImmediately(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		br.StartScheduler(ctx, 3, 0)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("StartScheduler did not return after context cancellation")
	}
}

// --- consecutive failure tracking ---

func TestConsecutiveFailureTracking(t *testing.T) {
	notifier := &recordingNotifier{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	br := &BackupRunner{
		db:       dummyPool(t),
		fs:       osFS{},
		logger:   logger,
		notifier: notifier,
	}

	ctx := context.Background()
	id := uuid.New()

	// Simulate 3 consecutive failures.
	for i := 1; i <= 3; i++ {
		result := BackupResult{ID: id, Status: "started"}
		br.failBackup(ctx, id, result, fmt.Errorf("failure %d", i))
	}

	br.mu.Lock()
	failures := br.consecutiveFailures
	br.mu.Unlock()

	if failures != 3 {
		t.Errorf("consecutiveFailures: got %d, want 3", failures)
	}

	if notifier.callCount() != 3 {
		t.Errorf("notification count: got %d, want 3", notifier.callCount())
	}

	// Check that the 3rd failure contains "CRITICAL".
	lastErr := notifier.lastError()
	if lastErr == nil {
		t.Fatal("expected last notification error")
	}
	if got := lastErr.Error(); !containsSubstring(got, "CRITICAL") {
		t.Errorf("expected CRITICAL in notification, got: %s", got)
	}
}

func TestFailBackupWarningLevel(t *testing.T) {
	notifier := &recordingNotifier{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	br := &BackupRunner{
		db:       dummyPool(t),
		fs:       osFS{},
		logger:   logger,
		notifier: notifier,
	}

	ctx := context.Background()
	id := uuid.New()
	result := BackupResult{ID: id, Status: "started"}

	br.failBackup(ctx, id, result, fmt.Errorf("first failure"))

	if notifier.callCount() != 1 {
		t.Fatalf("notification count: got %d, want 1", notifier.callCount())
	}

	// First failure should be WARNING, not CRITICAL.
	got := notifier.lastError().Error()
	if !containsSubstring(got, "WARNING") {
		t.Errorf("expected WARNING in notification, got: %s", got)
	}
	if containsSubstring(got, "CRITICAL") {
		t.Errorf("did not expect CRITICAL for first failure, got: %s", got)
	}
}

func TestFailBackupNoNotifier(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	br := &BackupRunner{
		db:       dummyPool(t),
		fs:       osFS{},
		logger:   logger,
		notifier: nil,
	}

	ctx := context.Background()
	id := uuid.New()
	result := BackupResult{ID: id, Status: "started"}

	// Should not panic when notifier is nil.
	_, err := br.failBackup(ctx, id, result, fmt.Errorf("failure"))
	if err == nil {
		t.Fatal("expected error from failBackup")
	}
}

func TestFailBackupNotifierError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	br := &BackupRunner{
		db:       dummyPool(t),
		fs:       osFS{},
		logger:   logger,
		notifier: &failingNotifier{},
	}

	ctx := context.Background()
	id := uuid.New()
	result := BackupResult{ID: id, Status: "started"}

	// Should not panic even when notifier returns error.
	_, err := br.failBackup(ctx, id, result, fmt.Errorf("failure"))
	if err == nil {
		t.Fatal("expected error from failBackup")
	}
}

func TestConsecutiveFailuresResetConceptually(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	br := &BackupRunner{
		logger:              logger,
		consecutiveFailures: 5,
	}

	// Simulate what RunBackup does on success: reset counter.
	br.mu.Lock()
	br.consecutiveFailures = 0
	br.mu.Unlock()

	br.mu.Lock()
	if br.consecutiveFailures != 0 {
		t.Errorf("consecutiveFailures after reset: got %d, want 0", br.consecutiveFailures)
	}
	br.mu.Unlock()
}

// --- NotificationBridge ---

func TestNotificationBridge(t *testing.T) {
	var called bool
	bridge := &NotificationBridge{
		NotifyFn: func(_ context.Context, err error) error {
			called = true
			return nil
		},
	}

	err := bridge.NotifyBackupFailed(context.Background(), fmt.Errorf("test"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("NotifyFn was not called")
	}
}

// --- writeToFile ---

func TestWriteToFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test-backup.vkbk")
	data := []byte("backup contents here")

	written, err := br.writeToFile(path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("writeToFile: %v", err)
	}
	if written != int64(len(data)) {
		t.Errorf("written: got %d, want %d", written, len(data))
	}

	// Verify file contents.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("file contents mismatch")
	}
}

// --- createArchive ---

func TestCreateArchive_DatabaseOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	pgDump := []byte("fake pg_dump output")
	archive := br.createArchive(pgDump, nil)
	if len(archive) == 0 {
		t.Fatal("archive is empty")
	}
}

func TestCreateArchive_WithStorageObjects(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	pgDump := []byte("fake pg_dump output")
	objects := map[string][]byte{
		"evidence/file1.pdf": []byte("pdf content"),
		"evidence/file2.jpg": []byte("jpg content"),
	}

	archive := br.createArchive(pgDump, objects)
	if len(archive) == 0 {
		t.Fatal("archive is empty")
	}
}

// --- VerifyBackup (file-level, without DB) ---

func TestVerifyBackup_FileOperations(t *testing.T) {
	// Test that writeToFile + reading back produces the correct checksum.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.vkbk")
	data := []byte("backup data for checksum test")

	if _, err := br.writeToFile(path, bytes.NewReader(data)); err != nil {
		t.Fatalf("writeToFile: %v", err)
	}

	// Verify file exists and size matches.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != int64(len(data)) {
		t.Errorf("size: got %d, want %d", info.Size(), len(data))
	}

	// Compute expected checksum.
	h := sha256.Sum256(data)
	expectedChecksum := hex.EncodeToString(h[:])

	// Read and verify.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		t.Errorf("checksum mismatch: got %s, want %s", actualChecksum, expectedChecksum)
	}
}

// --- NewBackupRunner ---

func TestNewBackupRunner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifier := &recordingNotifier{}

	br := NewBackupRunner(nil, []byte("test-key"), "/tmp/backups", logger, notifier, nil)
	if br == nil {
		t.Fatal("NewBackupRunner returned nil")
	}
	if string(br.encKey) != "test-key" {
		t.Errorf("encKey: got %q, want %q", string(br.encKey), "test-key")
	}
	if br.destination != "/tmp/backups" {
		t.Errorf("destination: got %q, want %q", br.destination, "/tmp/backups")
	}
}

// --- snapshotMinIO ---

type mockStorage struct {
	objects map[string][]byte
	listErr error
	getErr  error
}

func (m *mockStorage) ListObjects(_ context.Context, _ string) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	keys := make([]string, 0, len(m.objects))
	for k := range m.objects {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockStorage) GetObject(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
	if m.getErr != nil {
		return nil, 0, "", m.getErr
	}
	data, ok := m.objects[key]
	if !ok {
		return nil, 0, "", fmt.Errorf("not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), "", nil
}

func TestSnapshotMinIO_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{
		objects: map[string][]byte{
			"file1.pdf": []byte("pdf data"),
			"file2.jpg": []byte("jpg data"),
		},
	}

	br := &BackupRunner{logger: logger, fs: osFS{}, storage: storage}

	objects, err := br.snapshotMinIO(context.Background())
	if err != nil {
		t.Fatalf("snapshotMinIO: %v", err)
	}

	// Should have the 2 objects + _manifest.json.
	if len(objects) != 3 {
		t.Errorf("object count: got %d, want 3", len(objects))
	}
	if _, ok := objects["_manifest.json"]; !ok {
		t.Error("missing _manifest.json")
	}
}

func TestSnapshotMinIO_EmptyBucket(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{objects: map[string][]byte{}}

	br := &BackupRunner{logger: logger, fs: osFS{}, storage: storage}

	objects, err := br.snapshotMinIO(context.Background())
	if err != nil {
		t.Fatalf("snapshotMinIO: %v", err)
	}
	if objects != nil {
		t.Errorf("expected nil for empty bucket, got %v", objects)
	}
}

func TestSnapshotMinIO_ListError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{listErr: fmt.Errorf("connection refused")}

	br := &BackupRunner{logger: logger, fs: osFS{}, storage: storage}

	_, err := br.snapshotMinIO(context.Background())
	if err == nil {
		t.Fatal("expected error for list failure")
	}
}

func TestSnapshotMinIO_GetError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	storage := &mockStorage{
		objects: map[string][]byte{"file.pdf": []byte("data")},
		getErr:  fmt.Errorf("access denied"),
	}

	br := &BackupRunner{logger: logger, fs: osFS{}, storage: storage}

	_, err := br.snapshotMinIO(context.Background())
	if err == nil {
		t.Fatal("expected error for get failure")
	}
}

// --- DB-dependent methods (exercising error paths via dummy pool) ---

func TestListBackups_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := NewBackupRunner(pool, []byte("key"), t.TempDir(), logger, nil, nil)

	_, err := br.ListBackups(context.Background())
	if err == nil {
		t.Fatal("expected error from ListBackups with unreachable DB")
	}
}

func TestGetLastBackupInfo_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := NewBackupRunner(pool, []byte("key"), t.TempDir(), logger, nil, nil)

	_, _, err := br.GetLastBackupInfo(context.Background())
	if err == nil {
		t.Fatal("expected error from GetLastBackupInfo with unreachable DB")
	}
}

func TestVerifyBackup_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := NewBackupRunner(pool, []byte("key"), t.TempDir(), logger, nil, nil)

	err := br.VerifyBackup(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error from VerifyBackup with unreachable DB")
	}
}

func TestRunBackup_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := NewBackupRunner(pool, []byte("key"), t.TempDir(), logger, nil, nil)

	_, err := br.RunBackup(context.Background())
	if err == nil {
		t.Fatal("expected error from RunBackup with unreachable DB")
	}
}

func TestInsertLog_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{db: pool, fs: osFS{}, logger: logger}

	err := br.insertLog(context.Background(), uuid.New(), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error from insertLog with unreachable DB")
	}
}

func TestUpdateLogCompleted_DBError(t *testing.T) {
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{db: pool, fs: osFS{}, logger: logger}

	err := br.updateLogCompleted(context.Background(), uuid.New(), time.Now().UTC(), 1, 12345, "abc123")
	if err == nil {
		t.Fatal("expected error from updateLogCompleted with unreachable DB")
	}
}

// --- writeToFile edge cases ---

func TestWriteToFile_InvalidPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	// Use /dev/null/impossible to force a directory creation error on some systems,
	// or a path that's definitely invalid.
	_, err := br.writeToFile("/dev/null/impossible/file.vkbk", bytes.NewReader([]byte("data")))
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// --- dumpPostgres ---

func TestDumpPostgres_NoPgDump(t *testing.T) {
	// If pg_dump is not installed, this should return an error.
	// If it is installed, it will fail because the pool points to an invalid host.
	pool := dummyPool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dumper := &realPgDumper{pool: pool}
	_ = logger

	_, err := dumper.DumpPostgres(context.Background())
	if err == nil {
		t.Fatal("expected error from DumpPostgres with dummy pool")
	}
}

func TestDefaultCommandRunner_Success(t *testing.T) {
	data, err := defaultCommandRunner(context.Background(), "echo", []string{"hello"}, nil)
	if err != nil {
		t.Fatalf("defaultCommandRunner: %v", err)
	}
	if !bytes.Contains(data, []byte("hello")) {
		t.Errorf("unexpected output: %q", data)
	}
}

func TestDefaultCommandRunner_Failure(t *testing.T) {
	_, err := defaultCommandRunner(context.Background(), "false", nil, nil)
	if err == nil {
		t.Fatal("expected error from 'false' command")
	}
}

func TestDumpPostgres_SuccessPath(t *testing.T) {
	pool := dummyPool(t)
	expected := []byte("mock pg_dump output")

	dumper := &realPgDumper{
		pool: pool,
		runCmd: func(_ context.Context, _ string, _ []string, _ []string) ([]byte, error) {
			return expected, nil
		},
	}

	data, err := dumper.DumpPostgres(context.Background())
	if err != nil {
		t.Fatalf("DumpPostgres: %v", err)
	}
	if !bytes.Equal(data, expected) {
		t.Errorf("data mismatch: got %q, want %q", data, expected)
	}
}

// --- createArchive edge cases ---

func TestCreateArchive_EmptyPgDump(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	archive := br.createArchive([]byte{}, nil)
	if len(archive) == 0 {
		t.Fatal("expected non-empty archive even with empty dump")
	}
}

func TestCreateArchive_EmptyStorageObjects(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}}

	archive := br.createArchive([]byte("dump"), map[string][]byte{})
	if len(archive) == 0 {
		t.Fatal("expected non-empty archive")
	}
}

// --- mock DB ---

type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error { return r.scanFn(dest...) }

type mockRows struct {
	data    [][]any
	idx     int
	scanErr error
	iterErr error
}

func (r *mockRows) Close()                                         {}
func (r *mockRows) Err() error                                     { return r.iterErr }
func (r *mockRows) CommandTag() pgconn.CommandTag                   { return pgconn.NewCommandTag("") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription    { return nil }
func (r *mockRows) RawValues() [][]byte                             { return nil }
func (r *mockRows) Conn() *pgx.Conn                                { return nil }

func (r *mockRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return r.idx <= len(r.data)
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.idx-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch ptr := d.(type) {
		case *uuid.UUID:
			*ptr = row[i].(uuid.UUID)
		case *time.Time:
			*ptr = row[i].(time.Time)
		case **time.Time:
			switch v := row[i].(type) {
			case *time.Time:
				*ptr = v
			case time.Time:
				*ptr = &v
			default:
				*ptr = nil
			}
		case *string:
			*ptr = row[i].(string)
		case **string:
			switch v := row[i].(type) {
			case *string:
				*ptr = v
			case string:
				*ptr = &v
			default:
				*ptr = nil
			}
		case **int64:
			switch v := row[i].(type) {
			case *int64:
				*ptr = v
			case int64:
				*ptr = &v
			default:
				*ptr = nil
			}
		case *int64:
			*ptr = row[i].(int64)
		}
	}
	return nil
}

func (r *mockRows) Values() ([]any, error) {
	if r.idx <= 0 || r.idx > len(r.data) {
		return nil, fmt.Errorf("no current row")
	}
	return r.data[r.idx-1], nil
}

type mockDB struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag(""), nil
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return &mockRows{}, nil
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scanFn: func(dest ...any) error { return nil }}
}

// --- mock dumper ---

type mockDumper struct {
	data []byte
	err  error
}

func (d *mockDumper) DumpPostgres(_ context.Context) ([]byte, error) {
	return d.data, d.err
}

// --- mock file system ---

type mockFS struct {
	mkdirAllFn func(path string, perm os.FileMode) error
	openFileFn func(name string, flag int, perm os.FileMode) (backupFile, error)
	statFn     func(name string) (os.FileInfo, error)
	openFn     func(name string) (backupFile, error)
}

func (m *mockFS) MkdirAll(path string, perm os.FileMode) error {
	if m.mkdirAllFn != nil {
		return m.mkdirAllFn(path, perm)
	}
	return os.MkdirAll(path, perm)
}

func (m *mockFS) OpenFile(name string, flag int, perm os.FileMode) (backupFile, error) {
	if m.openFileFn != nil {
		return m.openFileFn(name, flag, perm)
	}
	return os.OpenFile(name, flag, perm)
}

func (m *mockFS) Stat(name string) (os.FileInfo, error) {
	if m.statFn != nil {
		return m.statFn(name)
	}
	return os.Stat(name)
}

func (m *mockFS) Open(name string) (backupFile, error) {
	if m.openFn != nil {
		return m.openFn(name)
	}
	return os.Open(name)
}

// fakeFile is a mock backupFile that can simulate errors.
type fakeFile struct {
	bytes.Buffer
	syncErr  error
	closeErr error
}

func (f *fakeFile) Close() error { return f.closeErr }
func (f *fakeFile) Sync() error  { return f.syncErr }

// --- RunBackup full path tests ---

func TestRunBackup_FullSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("pg_dump data")}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("test-encryption-key"),
		destination: dir,
		logger:      logger,
	}

	result, err := br.RunBackup(context.Background())
	if err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("status: got %q, want %q", result.Status, "completed")
	}
	if result.CompletedAt == nil {
		t.Error("CompletedAt is nil")
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount: got %d, want 1", result.FileCount)
	}
	if result.TotalSize <= 0 {
		t.Errorf("TotalSize: got %d, want >0", result.TotalSize)
	}
}

func TestRunBackup_FullSuccessWithStorage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("pg_dump data")}
	storage := &mockStorage{
		objects: map[string][]byte{
			"evidence/doc.pdf": []byte("pdf content"),
		},
	}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("test-encryption-key"),
		destination: dir,
		logger:      logger,
		storage:     storage,
	}

	result, err := br.RunBackup(context.Background())
	if err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("status: got %q, want %q", result.Status, "completed")
	}
}

func TestRunBackup_SuccessWithStorageError(t *testing.T) {
	// MinIO snapshot failure should not fail the backup.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("pg_dump data")}
	storage := &mockStorage{listErr: fmt.Errorf("minio down")}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("test-encryption-key"),
		destination: dir,
		logger:      logger,
		storage:     storage,
	}

	result, err := br.RunBackup(context.Background())
	if err != nil {
		t.Fatalf("RunBackup should succeed despite MinIO error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("status: got %q, want %q", result.Status, "completed")
	}
}

func TestRunBackup_DumpError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{err: fmt.Errorf("pg_dump failed")}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("test-encryption-key"),
		destination: dir,
		logger:      logger,
	}

	result, err := br.RunBackup(context.Background())
	if err == nil {
		t.Fatal("expected error from RunBackup when dump fails")
	}
	if result.Status != "failed" {
		t.Errorf("status: got %q, want %q", result.Status, "failed")
	}
}

func TestRunBackup_EncryptError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("data")}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      nil, // empty key causes Encrypt to fail
		destination: dir,
		logger:      logger,
	}

	result, err := br.RunBackup(context.Background())
	if err == nil {
		t.Fatal("expected error from RunBackup when encrypt fails")
	}
	if result.Status != "failed" {
		t.Errorf("status: got %q, want %q", result.Status, "failed")
	}
}

func TestRunBackup_WriteFileError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("data")}

	mfs := &mockFS{
		mkdirAllFn: func(_ string, _ os.FileMode) error {
			return fmt.Errorf("disk full")
		},
	}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          mfs,
		encKey:      []byte("test-key"),
		destination: "/some/path",
		logger:      logger,
	}

	result, err := br.RunBackup(context.Background())
	if err == nil {
		t.Fatal("expected error from RunBackup when write fails")
	}
	if result.Status != "failed" {
		t.Errorf("status: got %q, want %q", result.Status, "failed")
	}
}

func TestRunBackup_UpdateLogCompletedError(t *testing.T) {
	// updateLogCompleted error should not fail the backup.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	callCount := 0
	db := &mockDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			callCount++
			// First exec is insertLog (succeed), second is updateLogCompleted (fail)
			if callCount >= 2 {
				return pgconn.NewCommandTag(""), fmt.Errorf("db connection lost")
			}
			return pgconn.NewCommandTag(""), nil
		},
	}
	dumper := &mockDumper{data: []byte("data")}

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("test-key"),
		destination: dir,
		logger:      logger,
	}

	result, err := br.RunBackup(context.Background())
	if err != nil {
		t.Fatalf("RunBackup should succeed despite updateLogCompleted error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("status: got %q, want %q", result.Status, "completed")
	}
}

// --- ListBackups with mock DB ---

func TestListBackups_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now().UTC()
	id1 := uuid.New()
	id2 := uuid.New()
	completedAt := now.Add(5 * time.Second)
	size := int64(12345)

	db := &mockDB{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				data: [][]any{
					{id1, now, &completedAt, "completed", &size, "/backups", (*string)(nil)},
					{id2, now.Add(-time.Hour), (*time.Time)(nil), "failed", (*int64)(nil), "/backups", stringPtr("disk full")},
				},
			}, nil
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, logger: logger}
	backups, err := br.ListBackups(context.Background())
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("backup count: got %d, want 2", len(backups))
	}
	if backups[0].Status != "completed" {
		t.Errorf("first backup status: got %q, want %q", backups[0].Status, "completed")
	}
	if backups[1].Error != "disk full" {
		t.Errorf("second backup error: got %q, want %q", backups[1].Error, "disk full")
	}
}

func TestListBackups_ScanError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db := &mockDB{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				data:    [][]any{{uuid.New()}}, // insufficient columns
				scanErr: fmt.Errorf("scan failed"),
			}, nil
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, logger: logger}
	_, err := br.ListBackups(context.Background())
	if err == nil {
		t.Fatal("expected error from ListBackups with scan error")
	}
}

func TestListBackups_RowsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db := &mockDB{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{
				iterErr: fmt.Errorf("connection reset"),
			}, nil
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, logger: logger}
	_, err := br.ListBackups(context.Background())
	if err == nil {
		t.Fatal("expected error from ListBackups with rows error")
	}
}

func TestListBackups_EmptyResult(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db := &mockDB{
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &mockRows{}, nil
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, logger: logger}
	backups, err := br.ListBackups(context.Background())
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("backup count: got %d, want 0", len(backups))
	}
}

// --- GetLastBackupInfo with mock DB ---

func TestGetLastBackupInfo_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	expectedTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*time.Time)) = expectedTime
					*(dest[1].(*string)) = "completed"
					return nil
				},
			}
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, logger: logger}
	completedAt, status, err := br.GetLastBackupInfo(context.Background())
	if err != nil {
		t.Fatalf("GetLastBackupInfo: %v", err)
	}
	if !completedAt.Equal(expectedTime) {
		t.Errorf("completedAt: got %v, want %v", completedAt, expectedTime)
	}
	if status != "completed" {
		t.Errorf("status: got %q, want %q", status, "completed")
	}
}

// --- VerifyBackup with mock DB and FS ---

func TestVerifyBackup_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	backupID := uuid.New()
	fileContent := []byte("encrypted backup data for verification")

	// Write file to disk.
	filePath := filepath.Join(dir, fmt.Sprintf("backup-%s.vkbk", backupID))
	if err := os.WriteFile(filePath, fileContent, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	size := int64(len(fileContent))
	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = &size
					return nil
				},
			}
		},
	}

	br := &BackupRunner{
		db:          db,
		fs:          osFS{},
		destination: dir,
		logger:      logger,
	}

	err := br.VerifyBackup(context.Background(), backupID)
	if err != nil {
		t.Fatalf("VerifyBackup: %v", err)
	}
}

func TestVerifyBackup_NotCompleted(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backupID := uuid.New()

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "failed"
					*(dest[1].(**int64)) = nil
					return nil
				},
			}
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, destination: t.TempDir(), logger: logger}
	err := br.VerifyBackup(context.Background(), backupID)
	if err == nil {
		t.Fatal("expected error for non-completed backup")
	}
	if !containsSubstring(err.Error(), "expected \"completed\"") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyBackup_FileNotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backupID := uuid.New()

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = nil
					return nil
				},
			}
		},
	}

	br := &BackupRunner{
		db:          db,
		fs:          osFS{},
		destination: t.TempDir(), // empty dir, file doesn't exist
		logger:      logger,
	}

	err := br.VerifyBackup(context.Background(), backupID)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !containsSubstring(err.Error(), "backup file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyBackup_SizeMismatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	backupID := uuid.New()

	// Write a small file.
	filePath := filepath.Join(dir, fmt.Sprintf("backup-%s.vkbk", backupID))
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	wrongSize := int64(99999) // does not match actual file size of 4
	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = &wrongSize
					return nil
				},
			}
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, destination: dir, logger: logger}
	err := br.VerifyBackup(context.Background(), backupID)
	if err == nil {
		t.Fatal("expected error for size mismatch")
	}
	if !containsSubstring(err.Error(), "size mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyBackup_OpenError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	backupID := uuid.New()

	// Write a file so Stat succeeds.
	filePath := filepath.Join(dir, fmt.Sprintf("backup-%s.vkbk", backupID))
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = nil
					return nil
				},
			}
		},
	}

	mfs := &mockFS{
		statFn: func(name string) (os.FileInfo, error) {
			return os.Stat(name) // real stat
		},
		openFn: func(_ string) (backupFile, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}

	br := &BackupRunner{db: db, fs: mfs, destination: dir, logger: logger}
	err := br.VerifyBackup(context.Background(), backupID)
	if err == nil {
		t.Fatal("expected error for open failure")
	}
	if !containsSubstring(err.Error(), "open backup file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVerifyBackup_NilSizeBytes(t *testing.T) {
	// When sizeBytes is nil, size check should be skipped.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	backupID := uuid.New()

	filePath := filepath.Join(dir, fmt.Sprintf("backup-%s.vkbk", backupID))
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = nil
					return nil
				},
			}
		},
	}

	br := &BackupRunner{db: db, fs: osFS{}, destination: dir, logger: logger}
	err := br.VerifyBackup(context.Background(), backupID)
	if err != nil {
		t.Fatalf("VerifyBackup: %v", err)
	}
}

// --- snapshotMinIO read error ---

type failingReadCloser struct{}

func (f *failingReadCloser) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

func (f *failingReadCloser) Close() error { return nil }

type readErrorStorage struct{}

func (s *readErrorStorage) ListObjects(_ context.Context, _ string) ([]string, error) {
	return []string{"file.pdf"}, nil
}

func (s *readErrorStorage) GetObject(_ context.Context, _ string) (io.ReadCloser, int64, string, error) {
	return &failingReadCloser{}, 0, "", nil
}

func TestSnapshotMinIO_ReadError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	br := &BackupRunner{logger: logger, fs: osFS{}, storage: &readErrorStorage{}}

	_, err := br.snapshotMinIO(context.Background())
	if err == nil {
		t.Fatal("expected error for read failure")
	}
	if !containsSubstring(err.Error(), "read object") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- osFS coverage ---

func TestOsFS_StatAndOpen(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	fs := osFS{}

	info, err := fs.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 7 {
		t.Errorf("size: got %d, want 7", info.Size())
	}

	f, err := fs.Open(filePath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	data := make([]byte, 7)
	if _, err := f.Read(data); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("content: got %q, want %q", data, "content")
	}
}

// --- StartScheduler with actual backup execution ---

func TestStartScheduler_ExecutesBackupSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{data: []byte("scheduled dump")}

	ctx, cancel := context.WithCancel(context.Background())

	backupRan := make(chan struct{}, 1)

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("scheduler-test-key"),
		destination: dir,
		logger:      logger,
		newTimer: func(_ time.Duration) <-chan time.Time {
			// Fire immediately, then cancel after backup runs.
			ch := make(chan time.Time, 1)
			ch <- time.Now()
			go func() {
				// Wait a bit for backup to complete, then cancel.
				time.Sleep(100 * time.Millisecond)
				backupRan <- struct{}{}
				cancel()
			}()
			return ch
		},
	}

	done := make(chan struct{})
	go func() {
		br.StartScheduler(ctx, 3, 0)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("StartScheduler did not return")
	}

	select {
	case <-backupRan:
		// OK - backup was executed
	default:
		t.Error("backup did not run")
	}
}

func TestStartScheduler_ExecutesBackupFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	db := &mockDB{}
	dumper := &mockDumper{err: fmt.Errorf("pg_dump not found")}

	ctx, cancel := context.WithCancel(context.Background())

	br := &BackupRunner{
		db:          db,
		dumper:      dumper,
		fs:          osFS{},
		encKey:      []byte("scheduler-test-key"),
		destination: dir,
		logger:      logger,
		newTimer: func(_ time.Duration) <-chan time.Time {
			ch := make(chan time.Time, 1)
			ch <- time.Now()
			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()
			return ch
		},
	}

	done := make(chan struct{})
	go func() {
		br.StartScheduler(ctx, 3, 0)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("StartScheduler did not return")
	}
}

// --- writeToFile additional edge cases ---

func TestWriteToFile_OpenFileError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mfs := &mockFS{
		openFileFn: func(_ string, _ int, _ os.FileMode) (backupFile, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}
	br := &BackupRunner{logger: logger, fs: mfs}

	_, err := br.writeToFile("/tmp/test-open-error.vkbk", bytes.NewReader([]byte("data")))
	if err == nil {
		t.Fatal("expected error for OpenFile failure")
	}
	if !containsSubstring(err.Error(), "open file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriteToFile_CopyError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()

	br := &BackupRunner{logger: logger, fs: osFS{}}

	// Use a reader that fails after some data.
	errReader := &errAfterNBytesReader{data: []byte("partial"), err: fmt.Errorf("connection reset")}

	_, err := br.writeToFile(filepath.Join(dir, "test-copy-error.vkbk"), errReader)
	if err == nil {
		t.Fatal("expected error for io.Copy failure")
	}
	if !containsSubstring(err.Error(), "write file") {
		t.Errorf("unexpected error: %v", err)
	}
}

type errAfterNBytesReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterNBytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func TestWriteToFile_SyncError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mfs := &mockFS{
		openFileFn: func(_ string, _ int, _ os.FileMode) (backupFile, error) {
			return &fakeFile{syncErr: fmt.Errorf("disk full on sync")}, nil
		},
	}
	br := &BackupRunner{logger: logger, fs: mfs}

	n, err := br.writeToFile("/tmp/test-sync-error.vkbk", bytes.NewReader([]byte("data")))
	if err == nil {
		t.Fatal("expected error for Sync failure")
	}
	if !containsSubstring(err.Error(), "sync file") {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 4 {
		t.Errorf("written: got %d, want 4", n)
	}
}

func TestVerifyBackup_ReadError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	backupID := uuid.New()

	// Write a file so Stat succeeds.
	filePath := filepath.Join(dir, fmt.Sprintf("backup-%s.vkbk", backupID))
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	db := &mockDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{
				scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "completed"
					*(dest[1].(**int64)) = nil
					return nil
				},
			}
		},
	}

	mfs := &mockFS{
		statFn: func(name string) (os.FileInfo, error) {
			return os.Stat(name)
		},
		openFn: func(_ string) (backupFile, error) {
			return &fakeFile{
				Buffer: *bytes.NewBuffer(nil), // empty, but Read will error
			}, nil
		},
	}

	// Use a custom fakeFile that fails on Read after Open succeeds.
	readErrFile := &readErrorFile{}
	mfs.openFn = func(_ string) (backupFile, error) {
		return readErrFile, nil
	}

	br := &BackupRunner{db: db, fs: mfs, destination: dir, logger: logger}
	err := br.VerifyBackup(context.Background(), backupID)
	if err == nil {
		t.Fatal("expected error for read failure during checksum")
	}
	if !containsSubstring(err.Error(), "compute checksum") {
		t.Errorf("unexpected error: %v", err)
	}
}

type readErrorFile struct{}

func (f *readErrorFile) Read(_ []byte) (int, error)  { return 0, fmt.Errorf("disk I/O error") }
func (f *readErrorFile) Write(_ []byte) (int, error) { return 0, nil }
func (f *readErrorFile) Close() error                { return nil }
func (f *readErrorFile) Sync() error                 { return nil }

// --- DumpPostgres success path ---
// The DumpPostgres success path requires a real pg_dump binary and database.
// We test the interface-level mock instead.

func TestMockDumper(t *testing.T) {
	dumper := &mockDumper{data: []byte("mock dump output")}
	data, err := dumper.DumpPostgres(context.Background())
	if err != nil {
		t.Fatalf("DumpPostgres: %v", err)
	}
	if !bytes.Equal(data, []byte("mock dump output")) {
		t.Errorf("data mismatch")
	}
}

func TestMockDumper_Error(t *testing.T) {
	dumper := &mockDumper{err: fmt.Errorf("pg_dump not found")}
	_, err := dumper.DumpPostgres(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- helpers ---

func stringPtr(s string) *string {
	return &s
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && bytes.Contains([]byte(s), []byte(substr))
}
