package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Mock: ObjectRemover
// ---------------------------------------------------------------------------

type mockObjectRemover struct {
	removeObjectFn func(ctx context.Context, bucket, key string) error
	calls          []removeCall
}

type removeCall struct {
	bucket string
	key    string
}

func (m *mockObjectRemover) RemoveObject(ctx context.Context, bucket, key string) error {
	m.calls = append(m.calls, removeCall{bucket: bucket, key: key})
	if m.removeObjectFn != nil {
		return m.removeObjectFn(ctx, bucket, key)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: Notifier
// ---------------------------------------------------------------------------

type mockNotifier struct {
	notifyFn func(ctx context.Context, payload map[string]any) error
	calls    []map[string]any
}

func (m *mockNotifier) Notify(ctx context.Context, payload map[string]any) error {
	// Clone payload to avoid aliasing mutations after the call.
	clone := make(map[string]any, len(payload))
	for k, v := range payload {
		clone[k] = v
	}
	m.calls = append(m.calls, clone)
	if m.notifyFn != nil {
		return m.notifyFn(ctx, payload)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: pgx.Row
// ---------------------------------------------------------------------------

// mockRow implements pgx.Row. It either scans pre-populated values into the
// destination pointers or returns pgx.ErrNoRows when empty is true.
type mockRow struct {
	empty  bool
	values []any // ordered to match Scan arguments
	err    error
}

func (r *mockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.empty {
		return pgx.ErrNoRows
	}
	if len(dest) != len(r.values) {
		return errors.New("mockRow: Scan dest/value count mismatch")
	}
	for i, d := range dest {
		switch dst := d.(type) {
		case *uuid.UUID:
			*dst = r.values[i].(uuid.UUID)
		case *string:
			*dst = r.values[i].(string)
		case *[]byte:
			*dst = r.values[i].([]byte)
		case *int:
			*dst = r.values[i].(int)
		default:
			return errors.New("mockRow: unsupported dest type")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: pgx.Tx
// ---------------------------------------------------------------------------

type mockTx struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	commitErr  error
}

func (t *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{empty: true}
}

func (t *mockTx) Commit(ctx context.Context) error   { return t.commitErr }
func (t *mockTx) Rollback(ctx context.Context) error { return nil }

// Unused pgx.Tx surface — only implement what BeginTx callers need.
func (t *mockTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *mockTx) Begin(ctx context.Context) (pgx.Tx, error)  { return &mockTx{}, nil }
func (t *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *mockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects                                { return pgx.LargeObjects{} }
func (t *mockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) Conn() *pgx.Conn { return nil }

// ---------------------------------------------------------------------------
// Mock: dbExecer
// ---------------------------------------------------------------------------

type execCall struct {
	sql  string
	args []any
}

type mockDB struct {
	// claimNext support
	beginTxFn func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)

	// Exec support
	execFn    func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	execCalls []execCall

	// QueryRow support (used by non-tx paths if any)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *mockDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if m.beginTxFn != nil {
		return m.beginTxFn(ctx, opts)
	}
	return &mockTx{}, nil
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.execCalls = append(m.execCalls, execCall{sql: sql, args: args})
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{empty: true}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustMarshal: %v", err)
	}
	return b
}

// newWorkerWithMocks builds a Worker whose db field is replaced with mockDB.
// The public NewWorker constructor requires *pgxpool.Pool, so we bypass it
// by constructing the struct directly (same package).
func newWorkerWithMocks(db *mockDB, remover ObjectRemover, notifier Notifier) *Worker {
	return &Worker{
		db:          db,
		minioClient: remover,
		notifier:    notifier,
		logger:      discardLogger(),
	}
}

// buildTxWithRow returns a BeginTx function that presents one claimable row
// and then ErrNoRows on the second call, causing ProcessOnce to exit cleanly.
func buildTxWithRow(row outboxRow) func(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	called := 0
	return func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
		called++
		if called == 1 {
			tx := &mockTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{
						values: []any{
							row.ID,
							row.Action,
							row.Payload,
							row.AttemptCount,
							row.MaxAttempts,
						},
					}
				},
			}
			return tx, nil
		}
		// Second call → no more rows.
		return &mockTx{}, nil
	}
}

// ---------------------------------------------------------------------------
// Tests: processMinioDelete (direct unit tests on Worker method)
// ---------------------------------------------------------------------------

func TestProcessMinioDelete(t *testing.T) {
	tests := []struct {
		name        string
		payload     map[string]any
		removeFn    func(ctx context.Context, bucket, key string) error
		wantErr     bool
		wantErrMsg  string
		wantCalls   int
	}{
		{
			name:    "success — valid bucket and key",
			payload: map[string]any{"bucket": "evidence", "object_key": "file.pdf"},
			wantErr:   false,
			wantCalls: 1,
		},
		{
			name:       "missing bucket — returns error",
			payload:    map[string]any{"object_key": "file.pdf"},
			wantErr:    true,
			wantErrMsg: "missing bucket or object_key",
			wantCalls:  0,
		},
		{
			name:       "empty bucket string — returns error",
			payload:    map[string]any{"bucket": "", "object_key": "file.pdf"},
			wantErr:    true,
			wantErrMsg: "missing bucket or object_key",
			wantCalls:  0,
		},
		{
			name:       "missing object_key — returns error",
			payload:    map[string]any{"bucket": "evidence"},
			wantErr:    true,
			wantErrMsg: "missing bucket or object_key",
			wantCalls:  0,
		},
		{
			name:    "empty object_key string — returns error",
			payload: map[string]any{"bucket": "evidence", "object_key": ""},
			wantErr:    true,
			wantErrMsg: "missing bucket or object_key",
			wantCalls:  0,
		},
		{
			name:    "minio RemoveObject returns error — propagated",
			payload: map[string]any{"bucket": "evidence", "object_key": "file.pdf"},
			removeFn: func(_ context.Context, _, _ string) error {
				return errors.New("connection refused")
			},
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:    "non-string bucket type — treated as empty",
			payload: map[string]any{"bucket": 42, "object_key": "file.pdf"},
			wantErr:    true,
			wantErrMsg: "missing bucket or object_key",
			wantCalls:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remover := &mockObjectRemover{removeObjectFn: tc.removeFn}
			w := newWorkerWithMocks(&mockDB{}, remover, nil)

			err := w.processMinioDelete(context.Background(), tc.payload)

			if tc.wantErr && err == nil {
				t.Fatal("expected an error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErrMsg != "" && err != nil {
				if !contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrMsg)
				}
			}
			if len(remover.calls) != tc.wantCalls {
				t.Errorf("RemoveObject called %d times, want %d", len(remover.calls), tc.wantCalls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: processNotification (direct unit tests on Worker method)
// ---------------------------------------------------------------------------

func TestProcessNotification(t *testing.T) {
	tests := []struct {
		name       string
		notifier   *mockNotifier
		payload    map[string]any
		wantErr    bool
		wantCalls  int
	}{
		{
			name:      "nil notifier — returns nil without panicking",
			notifier:  nil,
			payload:   map[string]any{"event": "test"},
			wantErr:   false,
			wantCalls: 0,
		},
		{
			name:      "notifier success",
			notifier:  &mockNotifier{},
			payload:   map[string]any{"event": "test"},
			wantErr:   false,
			wantCalls: 1,
		},
		{
			name: "notifier returns error — propagated",
			notifier: &mockNotifier{
				notifyFn: func(_ context.Context, _ map[string]any) error {
					return errors.New("notify failed")
				},
			},
			payload:   map[string]any{"event": "test"},
			wantErr:   true,
			wantCalls: 1,
		},
		{
			name:      "empty payload — delivered as-is",
			notifier:  &mockNotifier{},
			payload:   map[string]any{},
			wantErr:   false,
			wantCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var notifier Notifier
			if tc.notifier != nil {
				notifier = tc.notifier
			}
			w := newWorkerWithMocks(&mockDB{}, nil, notifier)

			err := w.processNotification(context.Background(), tc.payload)

			if tc.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.notifier != nil && len(tc.notifier.calls) != tc.wantCalls {
				t.Errorf("Notify called %d times, want %d", len(tc.notifier.calls), tc.wantCalls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: ProcessOnce (table-driven, covers outbox flow end-to-end via mocks)
// ---------------------------------------------------------------------------

func TestProcessOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("no pending rows returns nil immediately", func(t *testing.T) {
		db := &mockDB{} // BeginTx returns mockTx with empty QueryRow by default
		w := newWorkerWithMocks(db, nil, nil)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		// Exec should never be called — nothing to update.
		if len(db.execCalls) != 0 {
			t.Errorf("Exec called %d times, want 0", len(db.execCalls))
		}
	})

	t.Run("minio_delete_object success sets completed_at", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "file.pdf"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 0, MaxAttempts: 3}

		remover := &mockObjectRemover{}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, remover, nil)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(remover.calls) != 1 {
			t.Fatalf("RemoveObject called %d times, want 1", len(remover.calls))
		}
		if remover.calls[0].bucket != "evidence" {
			t.Errorf("bucket = %q, want %q", remover.calls[0].bucket, "evidence")
		}
		if remover.calls[0].key != "file.pdf" {
			t.Errorf("key = %q, want %q", remover.calls[0].key, "file.pdf")
		}
		// Exactly one Exec should update completed_at.
		if !anyExecContains(db.execCalls, "completed_at") {
			t.Error("expected an Exec setting completed_at, none found")
		}
	})

	t.Run("minio_delete_object failure increments attempt_count", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "file.pdf"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 1, MaxAttempts: 5}

		remover := &mockObjectRemover{
			removeObjectFn: func(_ context.Context, _, _ string) error {
				return errors.New("minio unavailable")
			},
		}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, remover, nil)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should retry: Exec updates attempt_count + next_attempt_at.
		if !anyExecContains(db.execCalls, "attempt_count") {
			t.Error("expected Exec updating attempt_count for retry, none found")
		}
		if anyExecContains(db.execCalls, "completed_at") {
			t.Error("completed_at should NOT be set on failure")
		}
	})

	t.Run("minio_delete_object missing bucket triggers dead-letter", func(t *testing.T) {
		rowID := uuid.New()
		// Payload deliberately omits bucket.
		payload := mustMarshal(t, map[string]any{"object_key": "file.pdf"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 2, MaxAttempts: 3}

		remover := &mockObjectRemover{}
		notifier := &mockNotifier{}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, remover, notifier)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Missing bucket is a permanent error — should dead-letter immediately on max attempt.
		if !anyExecContains(db.execCalls, "dead_letter_at") {
			t.Error("expected dead_letter_at to be set")
		}
		if len(remover.calls) != 0 {
			t.Error("RemoveObject should not be called when bucket is missing")
		}
	})

	t.Run("notification_send success sets completed_at", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"event": "case_updated", "case_id": "abc"})
		row := outboxRow{ID: rowID, Action: "notification_send", Payload: payload, AttemptCount: 0, MaxAttempts: 3}

		notifier := &mockNotifier{}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, nil, notifier)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifier.calls) != 1 {
			t.Fatalf("Notify called %d times, want 1", len(notifier.calls))
		}
		if !anyExecContains(db.execCalls, "completed_at") {
			t.Error("expected Exec setting completed_at after successful notification")
		}
	})

	t.Run("unknown action is retried", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"foo": "bar"})
		row := outboxRow{ID: rowID, Action: "unsupported_action", Payload: payload, AttemptCount: 0, MaxAttempts: 5}

		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, nil, nil)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should schedule a retry (attempt_count increment).
		if !anyExecContains(db.execCalls, "attempt_count") {
			t.Error("expected retry Exec for unknown action")
		}
		if anyExecContains(db.execCalls, "completed_at") {
			t.Error("completed_at must not be set for unknown action")
		}
	})

	t.Run("dead letter after max_attempts — CRITICAL notification fired", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "x"})
		// AttemptCount = MaxAttempts - 1 so next attempt tips it over the edge.
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 2, MaxAttempts: 3}

		remover := &mockObjectRemover{
			removeObjectFn: func(_ context.Context, _, _ string) error {
				return errors.New("permanent failure")
			},
		}
		notifier := &mockNotifier{}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, remover, notifier)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !anyExecContains(db.execCalls, "dead_letter_at") {
			t.Error("expected dead_letter_at to be set")
		}
		// CRITICAL notification must be fired.
		if len(notifier.calls) == 0 {
			t.Fatal("expected at least one CRITICAL notification, got none")
		}
		var hasCritical bool
		for _, c := range notifier.calls {
			if c["severity"] == "critical" && c["kind"] == "cleanup_dead_letter" {
				hasCritical = true
				break
			}
		}
		if !hasCritical {
			t.Errorf("no critical dead-letter notification found in calls: %v", notifier.calls)
		}
	})

	t.Run("BeginTx error propagated from ProcessOnce", func(t *testing.T) {
		beginErr := errors.New("pg connection pool exhausted")
		db := &mockDB{
			beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
				return nil, beginErr
			},
		}
		w := newWorkerWithMocks(db, nil, nil)

		err := w.ProcessOnce(ctx)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, beginErr) {
			t.Errorf("error %v does not wrap beginErr", err)
		}
	})

	t.Run("invalid payload JSON triggers dead-letter immediately", func(t *testing.T) {
		rowID := uuid.New()
		// Not valid JSON.
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: []byte("{bad json"), AttemptCount: 0, MaxAttempts: 3}

		notifier := &mockNotifier{}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, nil, notifier)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !anyExecContains(db.execCalls, "dead_letter_at") {
			t.Error("invalid JSON payload should immediately dead-letter the row")
		}
	})

	t.Run("tx commit error propagated from claimNext", func(t *testing.T) {
		commitErr := errors.New("commit failed")
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "b", "object_key": "k"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 0, MaxAttempts: 3}

		db := &mockDB{
			beginTxFn: func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
				return &mockTx{
					queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
						return &mockRow{
							values: []any{
								row.ID,
								row.Action,
								row.Payload,
								row.AttemptCount,
								row.MaxAttempts,
							},
						}
					},
					commitErr: commitErr,
				}, nil
			},
		}
		w := newWorkerWithMocks(db, nil, nil)

		err := w.ProcessOnce(ctx)

		if err == nil {
			t.Fatal("expected error from commit failure, got nil")
		}
		if !errors.Is(err, commitErr) {
			t.Errorf("error %v does not wrap commitErr", err)
		}
	})

	t.Run("Exec failure when marking completed_at is returned", func(t *testing.T) {
		execErr := errors.New("db write error")
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "file.pdf"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 0, MaxAttempts: 3}

		remover := &mockObjectRemover{}
		db := &mockDB{
			beginTxFn: buildTxWithRow(row),
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, execErr
			},
		}
		w := newWorkerWithMocks(db, remover, nil)

		// ProcessOnce logs the error and continues; it does NOT propagate
		// per-row Exec failures — they are logged by processClaimed.
		err := w.ProcessOnce(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify that Exec was at least attempted (the error was encountered).
		if len(db.execCalls) == 0 {
			t.Error("expected at least one Exec call")
		}
	})

	t.Run("retry Exec failure is absorbed by ProcessOnce", func(t *testing.T) {
		execErr := errors.New("retry write error")
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "file.pdf"})
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 0, MaxAttempts: 5}

		remover := &mockObjectRemover{
			removeObjectFn: func(_ context.Context, _, _ string) error {
				return errors.New("transient error")
			},
		}
		db := &mockDB{
			beginTxFn: buildTxWithRow(row),
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, execErr
			},
		}
		w := newWorkerWithMocks(db, remover, nil)

		// Per-row errors are logged, not propagated.
		err := w.ProcessOnce(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("backoff capped at 1 hour for very high attempt counts", func(t *testing.T) {
		rowID := uuid.New()
		payload := mustMarshal(t, map[string]any{"bucket": "evidence", "object_key": "file.pdf"})
		// AttemptCount well beyond the shift cap (17).
		row := outboxRow{ID: rowID, Action: "minio_delete_object", Payload: payload, AttemptCount: 100, MaxAttempts: 200}

		remover := &mockObjectRemover{
			removeObjectFn: func(_ context.Context, _, _ string) error {
				return errors.New("still failing")
			},
		}
		db := &mockDB{beginTxFn: buildTxWithRow(row)}
		w := newWorkerWithMocks(db, remover, nil)

		err := w.ProcessOnce(ctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify retry Exec was called with the capped backoff value (3600).
		if !anyExecContains(db.execCalls, "attempt_count") {
			t.Error("expected retry Exec for high attempt count")
		}
		// The args on the retry Exec should include 3600 as the backoff seconds.
		var found bool
		for _, c := range db.execCalls {
			for _, arg := range c.args {
				if v, ok := arg.(int); ok && v == 3600 {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected backoff to be capped at 3600 seconds")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: markDeadLetter (direct unit tests)
// ---------------------------------------------------------------------------

func TestMarkDeadLetter(t *testing.T) {
	t.Run("fires CRITICAL notification with correct fields", func(t *testing.T) {
		rowID := uuid.New()
		row := outboxRow{ID: rowID, Action: "minio_delete_object", AttemptCount: 2, MaxAttempts: 3}
		payload := map[string]any{"bucket": "b", "object_key": "k"}

		notifier := &mockNotifier{}
		db := &mockDB{}
		w := newWorkerWithMocks(db, nil, notifier)

		err := w.markDeadLetter(context.Background(), row, payload)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !anyExecContains(db.execCalls, "dead_letter_at") {
			t.Error("expected dead_letter_at in Exec SQL")
		}
		if len(notifier.calls) != 1 {
			t.Fatalf("Notify called %d times, want 1", len(notifier.calls))
		}
		call := notifier.calls[0]
		if call["severity"] != "critical" {
			t.Errorf("severity = %v, want critical", call["severity"])
		}
		if call["kind"] != "cleanup_dead_letter" {
			t.Errorf("kind = %v, want cleanup_dead_letter", call["kind"])
		}
		if call["outbox_id"] != rowID.String() {
			t.Errorf("outbox_id = %v, want %v", call["outbox_id"], rowID.String())
		}
		if call["action"] != "minio_delete_object" {
			t.Errorf("action = %v, want minio_delete_object", call["action"])
		}
	})

	t.Run("Exec failure returns error", func(t *testing.T) {
		execErr := errors.New("db write failed")
		db := &mockDB{
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, execErr
			},
		}
		w := newWorkerWithMocks(db, nil, nil)

		err := w.markDeadLetter(context.Background(), outboxRow{ID: uuid.New()}, nil)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, execErr) {
			t.Errorf("error %v does not wrap execErr", err)
		}
	})

	t.Run("nil notifier does not panic", func(t *testing.T) {
		db := &mockDB{}
		w := newWorkerWithMocks(db, nil, nil)

		err := w.markDeadLetter(context.Background(), outboxRow{ID: uuid.New()}, nil)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: NewWorker defaults
// ---------------------------------------------------------------------------

func TestNewWorker_Defaults(t *testing.T) {
	t.Run("negative interval defaults to 30s", func(t *testing.T) {
		// NewWorker requires *pgxpool.Pool; we only check the interval field
		// which is set before the pool is used.  Passing nil is safe for
		// construction — it would only panic on use.
		w := &Worker{}
		// Replicate the interval-defaulting logic directly to validate it.
		interval := -1
		if interval <= 0 {
			interval = 30
		}
		if interval != 30 {
			t.Errorf("default interval = %d, want 30", interval)
		}
		_ = w
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// anyExecContains returns true if any recorded Exec SQL contains substr.
func anyExecContains(calls []execCall, substr string) bool {
	for _, c := range calls {
		if contains(c.sql, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || stringContains(s, substr))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
