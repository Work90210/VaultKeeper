package integrity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockFinder struct {
	items    []PendingTSAItem
	updated  []uuid.UUID
	retried  []uuid.UUID
	failed   []uuid.UUID
	findErr  error
}

func (m *mockFinder) FindPendingTSA(_ context.Context, _ int) ([]PendingTSAItem, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.items, nil
}

func (m *mockFinder) UpdateTSAResult(_ context.Context, id uuid.UUID, _ []byte, _ string, _ time.Time) error {
	m.updated = append(m.updated, id)
	return nil
}

func (m *mockFinder) IncrementTSARetry(_ context.Context, id uuid.UUID) error {
	m.retried = append(m.retried, id)
	return nil
}

func (m *mockFinder) MarkTSAFailed(_ context.Context, id uuid.UUID) error {
	m.failed = append(m.failed, id)
	return nil
}

type mockCustodyRecorder struct {
	events []string
}

func (m *mockCustodyRecorder) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ map[string]string) error {
	m.events = append(m.events, action)
	return nil
}

type mockLocker struct {
	acquired bool
	lockErr  error
}

func (m *mockLocker) TryAdvisoryLock(_ context.Context, _ int64) (bool, error) {
	if m.lockErr != nil {
		return false, m.lockErr
	}
	return m.acquired, nil
}

func (m *mockLocker) ReleaseAdvisoryLock(_ context.Context, _ int64) error {
	return nil
}

type mockTSA struct {
	err error
}

func (m *mockTSA) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	if m.err != nil {
		return nil, "", time.Time{}, m.err
	}
	return []byte("token"), "TestTSA", time.Now(), nil
}

func (m *mockTSA) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error {
	return nil
}

func TestTSARetryJob_RunOnce_NoItems(t *testing.T) {
	finder := &mockFinder{}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	// No items, nothing should happen
	if len(finder.updated) != 0 {
		t.Error("expected no updates")
	}
}

func TestTSARetryJob_RunOnce_Success(t *testing.T) {
	id := uuid.New()
	caseID := uuid.New()
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     caseID,
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	if len(finder.updated) != 1 || finder.updated[0] != id {
		t.Errorf("expected update for %s, got %v", id, finder.updated)
	}
	if len(custody.events) != 1 || custody.events[0] != "tsa_retry_succeeded" {
		t.Errorf("custody events = %v, want [tsa_retry_succeeded]", custody.events)
	}
}

func TestTSARetryJob_RunOnce_TSAFailure(t *testing.T) {
	id := uuid.New()
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{err: errors.New("TSA down")}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	if len(finder.retried) != 1 {
		t.Error("expected retry increment")
	}
}

func TestTSARetryJob_RunOnce_ExpiredItem(t *testing.T) {
	id := uuid.New()
	caseID := uuid.New()
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     caseID,
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 5,
				CreatedAt:  time.Now().Add(-25 * time.Hour), // older than 24h
			},
		},
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	if len(finder.failed) != 1 {
		t.Error("expected item marked as failed")
	}
	if len(custody.events) != 1 || custody.events[0] != "tsa_permanently_failed" {
		t.Errorf("custody events = %v, want [tsa_permanently_failed]", custody.events)
	}
}

func TestTSARetryJob_RunOnce_LockNotAcquired(t *testing.T) {
	finder := &mockFinder{
		items: []PendingTSAItem{{ID: uuid.New(), CaseID: uuid.New()}},
	}
	locker := &mockLocker{acquired: false}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	// When lock not acquired, should not process items
	if len(finder.updated) != 0 {
		t.Error("should not process items when lock not acquired")
	}
}

func TestTSARetryJob_RunOnce_FindError(t *testing.T) {
	finder := &mockFinder{findErr: errors.New("db error")}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	// Should not panic or error out
}

func TestTSARetryJob_RunOnce_LockError(t *testing.T) {
	finder := &mockFinder{}
	locker := &mockLocker{lockErr: errors.New("lock error")}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	if len(finder.updated) != 0 {
		t.Error("should not process items on lock error")
	}
}

func TestTSARetryJob_RunOnce_ReleaseError(t *testing.T) {
	finder := &mockFinder{}
	locker := &mockLockerReleaseErr{acquired: true, releaseErr: errors.New("release error")}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	// Should not panic even when release fails
	job.runOnce(context.Background())
}

type mockLockerReleaseErr struct {
	acquired   bool
	releaseErr error
}

func (m *mockLockerReleaseErr) TryAdvisoryLock(_ context.Context, _ int64) (bool, error) {
	return m.acquired, nil
}

func (m *mockLockerReleaseErr) ReleaseAdvisoryLock(_ context.Context, _ int64) error {
	return m.releaseErr
}

func TestTSARetryJob_RunOnce_InvalidHex(t *testing.T) {
	id := uuid.New()
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "zzzz",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	if len(finder.failed) != 1 || finder.failed[0] != id {
		t.Errorf("expected item marked as failed for invalid hex, got %v", finder.failed)
	}
}

func TestTSARetryJob_RunOnce_ContextCancelled(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         id1,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
			{
				ID:         id2,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Use a TSA that cancels context after first call
	ctx, cancel := context.WithCancel(context.Background())
	cancelTSA := &mockTSACancelOnCall{cancel: cancel}

	job := NewTSARetryJob(cancelTSA, finder, locker, custody, logger)
	job.runOnce(ctx)

	// First item should be processed, second should be skipped due to context cancellation
	if len(finder.updated) > 1 {
		t.Errorf("expected at most 1 update after context cancel, got %d", len(finder.updated))
	}
}

type mockTSACancelOnCall struct {
	calls  int
	cancel context.CancelFunc
}

func (m *mockTSACancelOnCall) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	m.calls++
	if m.calls == 1 {
		// Succeed first, then cancel so next iteration exits
		m.cancel()
		return []byte("token"), "TestTSA", time.Now(), nil
	}
	return nil, "", time.Time{}, errors.New("should not reach")
}

func (m *mockTSACancelOnCall) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error {
	return nil
}

func TestTSARetryJob_RunOnce_UpdateTSAResultError(t *testing.T) {
	id := uuid.New()
	finder := &mockFinderUpdateErr{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
		updateErr: errors.New("update failed"),
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	job.runOnce(context.Background())

	// Should not record custody event when update fails
	if len(custody.events) != 0 {
		t.Errorf("expected no custody events on update error, got %v", custody.events)
	}
}

type mockFinderUpdateErr struct {
	items     []PendingTSAItem
	updateErr error
	retried   []uuid.UUID
	failed    []uuid.UUID
}

func (m *mockFinderUpdateErr) FindPendingTSA(_ context.Context, _ int) ([]PendingTSAItem, error) {
	return m.items, nil
}

func (m *mockFinderUpdateErr) UpdateTSAResult(_ context.Context, _ uuid.UUID, _ []byte, _ string, _ time.Time) error {
	return m.updateErr
}

func (m *mockFinderUpdateErr) IncrementTSARetry(_ context.Context, id uuid.UUID) error {
	m.retried = append(m.retried, id)
	return nil
}

func (m *mockFinderUpdateErr) MarkTSAFailed(_ context.Context, id uuid.UUID) error {
	m.failed = append(m.failed, id)
	return nil
}

func TestTSARetryJob_RunOnce_MarkTSAFailedError(t *testing.T) {
	id := uuid.New()
	finder := &mockFinderMarkFailErr{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 5,
				CreatedAt:  time.Now().Add(-25 * time.Hour),
			},
		},
		markFailErr: errors.New("mark fail error"),
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	// Should not panic
	job.runOnce(context.Background())

	// Custody event should still be recorded
	if len(custody.events) != 1 || custody.events[0] != "tsa_permanently_failed" {
		t.Errorf("custody events = %v, want [tsa_permanently_failed]", custody.events)
	}
}

type mockFinderMarkFailErr struct {
	items       []PendingTSAItem
	markFailErr error
}

func (m *mockFinderMarkFailErr) FindPendingTSA(_ context.Context, _ int) ([]PendingTSAItem, error) {
	return m.items, nil
}

func (m *mockFinderMarkFailErr) UpdateTSAResult(_ context.Context, _ uuid.UUID, _ []byte, _ string, _ time.Time) error {
	return nil
}

func (m *mockFinderMarkFailErr) IncrementTSARetry(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockFinderMarkFailErr) MarkTSAFailed(_ context.Context, _ uuid.UUID) error {
	return m.markFailErr
}

func TestTSARetryJob_RunOnce_IncrementRetryError(t *testing.T) {
	id := uuid.New()
	finder := &mockFinderIncrementErr{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
		incrementErr: errors.New("increment failed"),
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{err: errors.New("tsa fail")}, finder, locker, custody, logger)
	// Should not panic when increment fails
	job.runOnce(context.Background())
}

type mockFinderIncrementErr struct {
	items        []PendingTSAItem
	incrementErr error
}

func (m *mockFinderIncrementErr) FindPendingTSA(_ context.Context, _ int) ([]PendingTSAItem, error) {
	return m.items, nil
}

func (m *mockFinderIncrementErr) UpdateTSAResult(_ context.Context, _ uuid.UUID, _ []byte, _ string, _ time.Time) error {
	return nil
}

func (m *mockFinderIncrementErr) IncrementTSARetry(_ context.Context, _ uuid.UUID) error {
	return m.incrementErr
}

func (m *mockFinderIncrementErr) MarkTSAFailed(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestTSARetryJob_Start_ContextCancel(t *testing.T) {
	finder := &mockFinder{}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		job.Start(ctx)
		close(done)
	}()

	// Cancel immediately to test the ctx.Done() select branch
	cancel()

	select {
	case <-done:
		// Start returned as expected
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestTSARetryJob_Start_TickerFires(t *testing.T) {
	callCount := 0
	finder := &mockFinder{
		items: []PendingTSAItem{
			{
				ID:         uuid.New(),
				CaseID:     uuid.New(),
				SHA256Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
	}
	locker := &mockLockerCountCalls{acquired: true, callCount: &callCount}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)

	// Use startWithInterval with a very short interval so the ticker fires quickly
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	job.startWithInterval(ctx, 10*time.Millisecond)

	// After the context times out, the ticker should have fired at least once
	if callCount == 0 {
		t.Error("expected at least one lock call from ticker firing")
	}
	if len(finder.updated) == 0 {
		t.Error("expected at least one update from ticker-triggered runOnce")
	}
}

type mockLockerCountCalls struct {
	acquired  bool
	callCount *int
}

func (m *mockLockerCountCalls) TryAdvisoryLock(_ context.Context, _ int64) (bool, error) {
	*m.callCount++
	return m.acquired, nil
}

func (m *mockLockerCountCalls) ReleaseAdvisoryLock(_ context.Context, _ int64) error {
	return nil
}

func TestTSARetryJob_RecordCustodyEvent_NilCustody(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := &TSARetryJob{
		logger: logger,
		// custody is nil
	}

	// Should not panic
	job.recordCustodyEvent(context.Background(), uuid.New(), uuid.New(), "test", nil)
}

func TestTSARetryJob_RecordCustodyEvent_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	custody := &mockCustodyRecorderErr{err: errors.New("custody error")}

	job := &TSARetryJob{
		custody: custody,
		logger:  logger,
	}

	// Should not panic when custody recorder returns error
	job.recordCustodyEvent(context.Background(), uuid.New(), uuid.New(), "test", nil)
}

type mockCustodyRecorderErr struct {
	err error
}

func (m *mockCustodyRecorderErr) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return m.err
}

func TestHexError_Error(t *testing.T) {
	var e hexError
	if e.Error() != "invalid hex string" {
		t.Errorf("hexError.Error() = %q, want %q", e.Error(), "invalid hex string")
	}
}

func TestHexToBytes_UpperCase(t *testing.T) {
	got, err := hexToBytes("E3B0C442")
	if err != nil {
		t.Fatalf("hexToBytes uppercase error: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
	if got[0] != 0xe3 || got[1] != 0xb0 || got[2] != 0xc4 || got[3] != 0x42 {
		t.Errorf("got %x", got)
	}
}

func TestHexToBytes_MixedCase(t *testing.T) {
	got, err := hexToBytes("aAbBcCdD")
	if err != nil {
		t.Fatalf("hexToBytes mixed case error: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("len = %d, want 4", len(got))
	}
}

func TestTSARetryJob_RunOnce_InvalidHexMarkFailErr(t *testing.T) {
	id := uuid.New()
	finder := &mockFinderMarkFailErr{
		items: []PendingTSAItem{
			{
				ID:         id,
				CaseID:     uuid.New(),
				SHA256Hash: "gg",
				RetryCount: 0,
				CreatedAt:  time.Now(),
			},
		},
		markFailErr: errors.New("mark fail error"),
	}
	locker := &mockLocker{acquired: true}
	custody := &mockCustodyRecorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	job := NewTSARetryJob(&mockTSA{}, finder, locker, custody, logger)
	// Should not panic when mark fail errors on invalid hex path
	job.runOnce(context.Background())
}

func TestHexToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{"valid hex", "e3b0c44298fc1c14", 8, false},
		{"odd length", "e3b", 0, true},
		{"invalid chars", "xyz123", 0, true},
		{"empty", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("hexToBytes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("hexToBytes(%q) len = %d, want %d", tt.input, len(got), tt.wantLen)
			}
		})
	}
}
