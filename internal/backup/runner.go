package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// backupDB abstracts the database operations needed by BackupRunner so that
// tests can provide a mock implementation without a real database.
type backupDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pgDumper abstracts the pg_dump command so tests can mock it.
type pgDumper interface {
	DumpPostgres(ctx context.Context) ([]byte, error)
}

// commandRunner abstracts exec.Cmd.Run for testability.
type commandRunner func(ctx context.Context, name string, args []string, env []string) ([]byte, error)

// defaultCommandRunner runs a real OS command.
func defaultCommandRunner(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// realPgDumper runs the actual pg_dump command using the pool's connection config.
type realPgDumper struct {
	pool   *pgxpool.Pool
	runCmd commandRunner
}

func (d *realPgDumper) DumpPostgres(ctx context.Context) ([]byte, error) {
	connConfig := d.pool.Config().ConnConfig

	run := d.runCmd
	if run == nil {
		run = defaultCommandRunner
	}

	return run(ctx, "pg_dump", []string{
		"--host", connConfig.Host,
		"--port", fmt.Sprintf("%d", connConfig.Port),
		"--username", connConfig.User,
		"--dbname", connConfig.Database,
		"--format", "custom",
		"--no-password",
	}, []string{fmt.Sprintf("PGPASSWORD=%s", connConfig.Password)})
}

// backupFile abstracts a writable, syncable, readable file.
type backupFile interface {
	io.ReadWriteCloser
	Sync() error
}

// fileSystem abstracts file system operations for testability.
type fileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (backupFile, error)
	Stat(name string) (os.FileInfo, error)
	Open(name string) (backupFile, error)
}

// osFS is the real file system implementation.
type osFS struct{}

func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) OpenFile(name string, flag int, perm os.FileMode) (backupFile, error) {
	return os.OpenFile(name, flag, perm)
}
func (osFS) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (osFS) Open(name string) (backupFile, error)   { return os.Open(name) }

// BackupResult describes the outcome of a single backup run.
type BackupResult struct {
	ID           uuid.UUID
	Status       string // started, completed, failed
	StartedAt    time.Time
	CompletedAt  *time.Time
	FileCount    int
	TotalSize    int64
	ErrorMessage string
}

// BackupInfo is the read-model returned by ListBackups.
type BackupInfo struct {
	ID          uuid.UUID  `json:"id"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string     `json:"status"`
	SizeBytes   *int64     `json:"size_bytes,omitempty"`
	Destination string     `json:"destination"`
	Error       string     `json:"error,omitempty"`
}

// StorageSnapshotter provides read access to object storage for backup purposes.
type StorageSnapshotter interface {
	ListObjects(ctx context.Context, prefix string) ([]string, error)
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
}

// BackupNotifier is called when a backup fails so that operators can be alerted.
type BackupNotifier interface {
	NotifyBackupFailed(ctx context.Context, err error) error
}

// BackupRunner orchestrates database backups, encryption, and storage.
type BackupRunner struct {
	db          backupDB
	dumper      pgDumper
	fs          fileSystem
	encKey      []byte
	destination string
	logger      *slog.Logger
	notifier    BackupNotifier
	storage     StorageSnapshotter

	// newTimer controls how the scheduler waits. Defaults to time.After.
	// Tests can replace it to fire immediately.
	newTimer func(d time.Duration) <-chan time.Time

	mu                  sync.Mutex
	consecutiveFailures int
}

// NotificationBridge adapts a generic notification function to the BackupNotifier interface.
type NotificationBridge struct {
	NotifyFn func(ctx context.Context, err error) error
}

func (b *NotificationBridge) NotifyBackupFailed(ctx context.Context, err error) error {
	return b.NotifyFn(ctx, err)
}

// NewBackupRunner creates a BackupRunner.
func NewBackupRunner(pool *pgxpool.Pool, encKey []byte, destination string, logger *slog.Logger, notifier BackupNotifier, storage StorageSnapshotter) *BackupRunner {
	return &BackupRunner{
		db:          pool,
		dumper:      &realPgDumper{pool: pool},
		fs:          osFS{},
		encKey:      encKey,
		destination: destination,
		logger:      logger,
		notifier:    notifier,
		storage:     storage,
	}
}

// RunBackup performs a full backup: pg_dump, tar+gzip, encrypt, write to destination.
func (br *BackupRunner) RunBackup(ctx context.Context) (BackupResult, error) {
	id := uuid.New()
	startedAt := time.Now().UTC()

	result := BackupResult{
		ID:        id,
		Status:    "started",
		StartedAt: startedAt,
	}

	// 1. Insert backup_log entry.
	if err := br.insertLog(ctx, id, startedAt); err != nil {
		return result, fmt.Errorf("insert backup log: %w", err)
	}

	br.logger.Info("backup started", "backup_id", id)

	// 2. Dump Postgres.
	pgDump, err := br.dumper.DumpPostgres(ctx)
	if err != nil {
		return br.failBackup(ctx, id, result, fmt.Errorf("pg_dump: %w", err))
	}

	// 3. Snapshot MinIO evidence bucket.
	var storageObjects map[string][]byte
	if br.storage != nil {
		var snapErr error
		storageObjects, snapErr = br.snapshotMinIO(ctx)
		if snapErr != nil {
			br.logger.Warn("MinIO snapshot failed; backup will contain database only", "error", snapErr)
		}
	}

	// 4. Create tar.gz archive containing the dump and storage snapshot.
	archive := br.createArchive(pgDump, storageObjects)

	// 5. Encrypt with AES-256-GCM.
	encrypted, err := Encrypt(bytes.NewReader(archive), br.encKey)
	if err != nil {
		return br.failBackup(ctx, id, result, fmt.Errorf("encrypt: %w", err))
	}

	// 6. Write to destination.
	destPath := filepath.Join(br.destination, fmt.Sprintf("backup-%s.vkbk", id))
	written, err := br.writeToFile(destPath, encrypted)
	if err != nil {
		return br.failBackup(ctx, id, result, fmt.Errorf("write backup file: %w", err))
	}

	// 6a. Compute SHA-256 checksum of the written file for later verification.
	checksum, err := br.computeFileChecksum(destPath)
	if err != nil {
		// Non-fatal: log the error but continue — the backup is already written.
		br.logger.Warn("compute backup checksum failed", "backup_id", id, "error", err)
	}

	// 7. Update backup_log with success.
	completedAt := time.Now().UTC()
	result.Status = "completed"
	result.CompletedAt = &completedAt
	result.FileCount = 1
	result.TotalSize = written

	if err := br.updateLogCompleted(ctx, id, completedAt, result.FileCount, written, checksum); err != nil {
		br.logger.Error("update backup log after success", "backup_id", id, "error", err)
	}

	br.mu.Lock()
	br.consecutiveFailures = 0
	br.mu.Unlock()

	br.logger.Info("backup completed",
		"backup_id", id,
		"size_bytes", written,
		"duration", completedAt.Sub(startedAt),
	)

	return result, nil
}

// ListBackups returns all backup records ordered by most recent first.
func (br *BackupRunner) ListBackups(ctx context.Context) ([]BackupInfo, error) {
	rows, err := br.db.Query(ctx, `
		SELECT id, started_at, completed_at, status, size_bytes, destination, error_message
		FROM backup_log
		ORDER BY started_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query backup_log: %w", err)
	}
	defer rows.Close()

	var backups []BackupInfo
	for rows.Next() {
		var b BackupInfo
		var errMsg *string
		if err := rows.Scan(&b.ID, &b.StartedAt, &b.CompletedAt, &b.Status, &b.SizeBytes, &b.Destination, &errMsg); err != nil {
			return nil, fmt.Errorf("scan backup_log row: %w", err)
		}
		if errMsg != nil {
			b.Error = *errMsg
		}
		backups = append(backups, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backup_log rows: %w", err)
	}

	return backups, nil
}

// GetLastBackupInfo returns the completion time and status of the most recent backup.
func (br *BackupRunner) GetLastBackupInfo(ctx context.Context) (time.Time, string, error) {
	var completedAt time.Time
	var status string
	err := br.db.QueryRow(ctx, `
		SELECT COALESCE(completed_at, started_at), status
		FROM backup_log
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(&completedAt, &status)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("query last backup info: %w", err)
	}
	return completedAt, status, nil
}

// StartScheduler runs backups daily at the specified wall-clock time (UTC).
// It blocks until ctx is cancelled.
func (br *BackupRunner) StartScheduler(ctx context.Context, targetHour, targetMinute int) {
	br.logger.Info("backup scheduler started", "target_utc", fmt.Sprintf("%02d:%02d", targetHour, targetMinute))

	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), targetHour, targetMinute, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}

		br.logger.Info("next backup scheduled", "at", next)

		timerFn := br.newTimer
		if timerFn == nil {
			timerFn = time.After
		}

		select {
		case <-timerFn(time.Until(next)):
			br.logger.Info("starting scheduled backup")
			if _, err := br.RunBackup(ctx); err != nil {
				br.logger.Error("scheduled backup failed", "error", err)
			}
		case <-ctx.Done():
			br.logger.Info("backup scheduler stopped")
			return
		}
	}
}

// VerifyBackup checks that a backup file exists, its size matches, and its
// SHA-256 checksum matches the value stored at backup creation time.
func (br *BackupRunner) VerifyBackup(ctx context.Context, backupID uuid.UUID) error {
	var status string
	var sizeBytes *int64
	var storedChecksum *string
	err := br.db.QueryRow(ctx, `
		SELECT status, size_bytes, checksum FROM backup_log WHERE id = $1
	`, backupID).Scan(&status, &sizeBytes, &storedChecksum)
	if err != nil {
		return fmt.Errorf("lookup backup %s: %w", backupID, err)
	}

	if status != "completed" {
		return fmt.Errorf("backup %s has status %q, expected \"completed\"", backupID, status)
	}

	destPath := filepath.Join(br.destination, fmt.Sprintf("backup-%s.vkbk", backupID))
	info, err := br.fs.Stat(destPath)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	if sizeBytes != nil && info.Size() != *sizeBytes {
		return fmt.Errorf("backup file size mismatch: expected %d, got %d", *sizeBytes, info.Size())
	}

	// Compute current checksum and compare against the stored reference.
	actualChecksum, err := br.computeFileChecksum(destPath)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	if storedChecksum == nil || *storedChecksum == "" {
		return fmt.Errorf("backup %s has no stored checksum; integrity cannot be verified", backupID)
	}
	if subtle.ConstantTimeCompare([]byte(actualChecksum), []byte(*storedChecksum)) != 1 {
		return fmt.Errorf("backup %s checksum mismatch: file may be corrupt or tampered", backupID)
	}

	br.logger.Info("backup verified",
		"backup_id", backupID,
		"size_bytes", info.Size(),
		"sha256", actualChecksum,
	)

	return nil
}

// --- internal helpers ---

func (br *BackupRunner) insertLog(ctx context.Context, id uuid.UUID, startedAt time.Time) error {
	_, err := br.db.Exec(ctx, `
		INSERT INTO backup_log (id, started_at, status, destination)
		VALUES ($1, $2, 'started', $3)
	`, id, startedAt, br.destination)
	return err
}

func (br *BackupRunner) updateLogCompleted(ctx context.Context, id uuid.UUID, completedAt time.Time, fileCount int, totalSize int64, checksum string) error {
	_, err := br.db.Exec(ctx, `
		UPDATE backup_log
		SET completed_at = $1, status = 'completed', size_bytes = $2, checksum = $3
		WHERE id = $4
	`, completedAt, totalSize, checksum, id)
	_ = fileCount // stored for result; not a separate column
	return err
}

// computeFileChecksum opens the file at path and returns its hex-encoded SHA-256.
func (br *BackupRunner) computeFileChecksum(path string) (string, error) {
	f, err := br.fs.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (br *BackupRunner) updateLogFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	now := time.Now().UTC()
	_, err := br.db.Exec(ctx, `
		UPDATE backup_log
		SET completed_at = $1, status = 'failed', error_message = $2
		WHERE id = $3
	`, now, errMsg, id)
	return err
}

// sanitizeBackupErr returns a truncated, safe error string for storage in the
// backup_log table. Raw error messages can contain file paths, SQL fragments,
// or other sensitive internal details; truncating at 512 bytes prevents
// unbounded storage and limits information leakage.
func sanitizeBackupErr(err error) string {
	msg := err.Error()
	if len(msg) > 512 {
		msg = msg[:512] + "...(truncated)"
	}
	return msg
}

func (br *BackupRunner) failBackup(ctx context.Context, id uuid.UUID, result BackupResult, backupErr error) (BackupResult, error) {
	result.Status = "failed"
	result.ErrorMessage = sanitizeBackupErr(backupErr)

	br.logger.Error("backup failed", "backup_id", id, "error", backupErr)

	if err := br.updateLogFailed(ctx, id, sanitizeBackupErr(backupErr)); err != nil {
		br.logger.Error("update backup log after failure", "backup_id", id, "error", err)
	}

	br.mu.Lock()
	br.consecutiveFailures++
	failures := br.consecutiveFailures
	br.mu.Unlock()

	if br.notifier != nil {
		level := "WARNING"
		if failures >= 3 {
			level = "CRITICAL"
		}
		notifyErr := br.notifier.NotifyBackupFailed(ctx, fmt.Errorf("[%s] backup %s failed (consecutive: %d); check server logs for details", level, id, failures))
		if notifyErr != nil {
			br.logger.Error("send backup failure notification", "error", notifyErr)
		}
	}

	return result, backupErr
}

// createArchive builds a tar.gz archive containing the pg_dump and any MinIO
// storage objects. storageObjects maps object keys to their contents.
//
// TODO(MEDIUM-1): The entire archive is buffered in memory. This is acceptable
// for small-to-medium deployments (v1.0) but should be replaced with a
// streaming approach for large deployments where the database dump or object
// storage contents exceed available memory.
func (br *BackupRunner) createArchive(pgDump []byte, storageObjects map[string][]byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// All tar/gzip writes target a bytes.Buffer, which never returns errors.
	// Errors are deliberately discarded to eliminate dead branches.

	// Add database dump.
	_ = tw.WriteHeader(&tar.Header{
		Name:    "database.dump",
		Size:    int64(len(pgDump)),
		Mode:    0o600,
		ModTime: time.Now().UTC(),
	})
	_, _ = tw.Write(pgDump)

	// Add MinIO storage objects.
	for key, data := range storageObjects {
		safeName := path.Clean(key)
		if safeName == ".." || strings.HasPrefix(safeName, "../") || strings.Contains(safeName, "/../") {
			br.logger.Warn("skipping object with unsafe key", "key", key)
			continue
		}
		if strings.HasPrefix(safeName, "/") {
			br.logger.Warn("skipping object with absolute path key", "key", key)
			continue
		}
		_ = tw.WriteHeader(&tar.Header{
			Name:    "evidence/" + safeName,
			Size:    int64(len(data)),
			Mode:    0o600,
			ModTime: time.Now().UTC(),
		})
		_, _ = tw.Write(data)
	}

	// Close flushes to bytes.Buffer; cannot fail.
	_ = tw.Close()
	_ = gw.Close()

	return buf.Bytes()
}


// snapshotMinIO reads all objects from the evidence bucket and returns them
// as a map of key to content bytes. This buffers all objects in memory.
func (br *BackupRunner) snapshotMinIO(ctx context.Context) (map[string][]byte, error) {
	keys, err := br.storage.ListObjects(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list storage objects: %w", err)
	}

	if len(keys) == 0 {
		br.logger.Info("no objects in evidence bucket to snapshot")
		return nil, nil
	}

	br.logger.Info("snapshotting evidence bucket", "object_count", len(keys))

	// Write a manifest of object keys.
	// json.Marshal on []string cannot fail.
	manifest, _ := json.Marshal(keys)

	objects := map[string][]byte{
		"_manifest.json": manifest,
	}

	const maxObjectSize = 5 * 1024 * 1024 * 1024 // 5 GiB

	for _, key := range keys {
		rc, _, _, getErr := br.storage.GetObject(ctx, key)
		if getErr != nil {
			return nil, fmt.Errorf("get object %s: %w", key, getErr)
		}

		limited := io.LimitReader(rc, maxObjectSize+1)
		data, readErr := io.ReadAll(limited)
		rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read object %s: %w", key, readErr)
		}
		if int64(len(data)) > maxObjectSize {
			return nil, fmt.Errorf("object %s exceeds maximum size", key)
		}

		objects[key] = data
	}

	return objects, nil
}

func (br *BackupRunner) writeToFile(path string, r io.Reader) (int64, error) {
	dir := filepath.Dir(path)
	if err := br.fs.MkdirAll(dir, 0o700); err != nil {
		return 0, fmt.Errorf("create destination directory: %w", err)
	}

	f, err := br.fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, fmt.Errorf("write file: %w", err)
	}

	if err := f.Sync(); err != nil {
		return n, fmt.Errorf("sync file: %w", err)
	}

	return n, nil
}
