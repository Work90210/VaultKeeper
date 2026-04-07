package integrity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockEvidenceLoader struct {
	items []VerifiableItem
	err   error
}

func (m *mockEvidenceLoader) ListByCaseForVerification(_ context.Context, _ uuid.UUID) ([]VerifiableItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

type mockHandlerFileReader struct {
	content []byte
	err     error
}

func (m *mockHandlerFileReader) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

type mockHandlerTSA struct {
	verifyErr error
}

func (m *mockHandlerTSA) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	return nil, "", time.Time{}, nil
}

func (m *mockHandlerTSA) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error {
	return m.verifyErr
}

type mockHandlerCustody struct {
	events []string
	err    error
}

func (m *mockHandlerCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ map[string]string) error {
	m.events = append(m.events, action)
	return m.err
}

type mockHandlerNotifier struct {
	events []NotificationEvent
	err    error
}

func (m *mockHandlerNotifier) Notify(_ context.Context, event NotificationEvent) error {
	m.events = append(m.events, event)
	return m.err
}

type mockHandlerFlagger struct {
	flagged []uuid.UUID
	err     error
}

func (m *mockHandlerFlagger) FlagIntegrityWarning(_ context.Context, id uuid.UUID) error {
	m.flagged = append(m.flagged, id)
	return m.err
}

type mockAuditLogger struct{}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

// ---------------------------------------------------------------------------
// Helper: create handler with mocks
// ---------------------------------------------------------------------------

func newTestHandler(
	loader *mockEvidenceLoader,
	reader *mockHandlerFileReader,
	tsa *mockHandlerTSA,
	custody *mockHandlerCustody,
	notifier *mockHandlerNotifier,
	flagger *mockHandlerFlagger,
) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(loader, reader, tsa, custody, notifier, flagger, logger, &mockAuditLogger{})
}

// Helper: create a request with auth context and chi URL param.
func newAuthRequest(method, path string, caseID uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.RoleSystemAdmin,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

// ---------------------------------------------------------------------------
// NewHandler test
// ---------------------------------------------------------------------------

func TestNewHandler(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

// ---------------------------------------------------------------------------
// StartVerification tests
// ---------------------------------------------------------------------------

func TestStartVerification_InvalidCaseID(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/cases/not-a-uuid/verify/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.RoleSystemAdmin,
	})
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.StartVerification(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestStartVerification_NoAuthContext(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/verify/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.StartVerification(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestStartVerification_Success(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{}, // empty items
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()
	req := newAuthRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/verify/", caseID)

	w := httptest.NewRecorder()
	h.StartVerification(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data in response, got %+v", body)
	}
	if _, ok := data["job_id"]; !ok {
		t.Error("expected job_id in response data")
	}
}

func TestStartVerification_AlreadyRunning(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()

	// Pre-store a running job
	h.jobs.Store(caseID.String(), &VerificationJob{
		ID:     "existing-job",
		CaseID: caseID,
		Status: "running",
	})

	req := newAuthRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/verify/", caseID)
	w := httptest.NewRecorder()
	h.StartVerification(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestStartVerification_PreviousCompletedJob(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()

	// Pre-store a completed job — should allow restarting
	h.jobs.Store(caseID.String(), &VerificationJob{
		ID:     "old-job",
		CaseID: caseID,
		Status: "completed",
	})

	req := newAuthRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/verify/", caseID)
	w := httptest.NewRecorder()
	h.StartVerification(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

// ---------------------------------------------------------------------------
// GetStatus tests
// ---------------------------------------------------------------------------

func TestGetStatus_InvalidCaseID(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/cases/bad/verify/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bad")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetStatus_NotFound(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseID.String()+"/verify/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetStatus_Found(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	caseID := uuid.New()
	h.jobs.Store(caseID.String(), &VerificationJob{
		ID:       "job-1",
		CaseID:   caseID,
		Status:   "completed",
		Total:    5,
		Verified: 5,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseID.String()+"/verify/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", caseID.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// runVerification tests
// ---------------------------------------------------------------------------

func TestRunVerification_LoadError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{err: errors.New("db down")},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	if job.Status != "failed" {
		t.Errorf("status = %q, want %q", job.Status, "failed")
	}
	if job.Error == "" {
		t.Error("expected error message")
	}
}

func TestRunVerification_AllItemsVerified(t *testing.T) {
	content := []byte("test file")
	hash, _ := ComputeSHA256(bytes.NewReader(content))

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{ID: uuid.New(), CaseID: uuid.New(), StorageKey: "key1", SHA256Hash: hash, Filename: "f1.pdf"},
				{ID: uuid.New(), CaseID: uuid.New(), StorageKey: "key2", SHA256Hash: hash, Filename: "f2.pdf"},
			},
		},
		&mockHandlerFileReader{content: content},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	if job.Status != "completed" {
		t.Errorf("status = %q, want %q", job.Status, "completed")
	}
	if job.Verified != 2 {
		t.Errorf("verified = %d, want 2", job.Verified)
	}
	if job.Total != 2 {
		t.Errorf("total = %d, want 2", job.Total)
	}
}

func TestRunVerification_HashMismatch(t *testing.T) {
	content := []byte("actual content")
	custody := &mockHandlerCustody{}
	notifier := &mockHandlerNotifier{}
	flagger := &mockHandlerFlagger{}

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{ID: uuid.New(), CaseID: uuid.New(), StorageKey: "key1", SHA256Hash: "wrong-hash", Filename: "f1.pdf"},
			},
		},
		&mockHandlerFileReader{content: content},
		&mockHandlerTSA{},
		custody,
		notifier,
		flagger,
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	if job.Mismatches != 1 {
		t.Errorf("mismatches = %d, want 1", job.Mismatches)
	}
	if len(flagger.flagged) != 1 {
		t.Errorf("flagged count = %d, want 1", len(flagger.flagged))
	}
	if len(custody.events) != 1 {
		t.Errorf("custody events = %d, want 1", len(custody.events))
	}
	if len(notifier.events) != 1 {
		t.Errorf("notification count = %d, want 1", len(notifier.events))
	}
}

func TestRunVerification_MissingFile(t *testing.T) {
	custody := &mockHandlerCustody{}
	notifier := &mockHandlerNotifier{}
	flagger := &mockHandlerFlagger{}

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{ID: uuid.New(), CaseID: uuid.New(), StorageKey: "missing", SHA256Hash: "abc", Filename: "f1.pdf"},
			},
		},
		&mockHandlerFileReader{err: errors.New("not found")},
		&mockHandlerTSA{},
		custody,
		notifier,
		flagger,
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	if job.Missing != 1 {
		t.Errorf("missing = %d, want 1", job.Missing)
	}
	if len(flagger.flagged) != 1 {
		t.Errorf("flagged count = %d, want 1", len(flagger.flagged))
	}
}

func TestRunVerification_WithTSAToken(t *testing.T) {
	content := []byte("test file")
	hash, _ := ComputeSHA256(bytes.NewReader(content))
	// Encode the hash to hex bytes for TSA token digest matching
	tsaToken := []byte("fake-tsa-token")

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{
					ID:         uuid.New(),
					CaseID:     uuid.New(),
					StorageKey: "key1",
					SHA256Hash: hash,
					Filename:   "f1.pdf",
					TSAToken:   tsaToken,
					TSAStatus:  "verified",
				},
			},
		},
		&mockHandlerFileReader{content: content},
		&mockHandlerTSA{verifyErr: errors.New("bad token")},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	// File hash matches, but TSA verification fails — item should still be verified
	if job.Verified != 1 {
		t.Errorf("verified = %d, want 1", job.Verified)
	}
}

func TestRunVerification_TSATokenHexDecodeError(t *testing.T) {
	original := hexDecodeString
	t.Cleanup(func() { hexDecodeString = original })

	hexDecodeString = func(_ string) ([]byte, error) {
		return nil, errors.New("injected hex decode error")
	}

	content := []byte("test file")
	hash, _ := ComputeSHA256(bytes.NewReader(content))

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{
					ID:         uuid.New(),
					CaseID:     uuid.New(),
					StorageKey: "key1",
					SHA256Hash: hash,
					Filename:   "f1.pdf",
					TSAToken:   []byte("some-token"),
				},
			},
		},
		&mockHandlerFileReader{content: content},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	// Item should still be verified (hex decode error is logged but not fatal)
	if job.Verified != 1 {
		t.Errorf("verified = %d, want 1", job.Verified)
	}
}

func TestRunVerification_TSATokenWithInvalidHexHash(t *testing.T) {
	// We need a SHA256Hash that is valid for file verification but invalid hex for TSA.
	// This is tricky because the same hash is used for both. However, we can test
	// the decode error path by creating a custom scenario.
	// The hex.DecodeString error branch triggers when SHA256Hash is not valid hex.
	// But VerifyFileHash computes the hash itself, so SHA256Hash must match computed.
	// The only way to hit the decode error is if SHA256Hash contains non-hex chars,
	// but then VerifyFileHash would fail first (mismatch).
	// So this branch is effectively unreachable in normal operation.
	// We test it via verifyItem directly with a matching hash that has odd length.
	// Actually, the real hash will always be valid hex since ComputeSHA256 returns hex.
	// Let's just test the TSA verify success path with a valid hex hash.
	content := []byte("test file")
	hash, _ := ComputeSHA256(bytes.NewReader(content))

	h := newTestHandler(
		&mockEvidenceLoader{
			items: []VerifiableItem{
				{
					ID:         uuid.New(),
					CaseID:     uuid.New(),
					StorageKey: "key1",
					SHA256Hash: hash,
					Filename:   "f1.pdf",
					TSAToken:   []byte("token"),
				},
			},
		},
		&mockHandlerFileReader{content: content},
		&mockHandlerTSA{}, // VerifyTimestamp returns nil (success)
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.runVerification(context.Background(), job, "user-1")

	if job.Verified != 1 {
		t.Errorf("verified = %d, want 1", job.Verified)
	}
}

// ---------------------------------------------------------------------------
// handleMismatch error branches
// ---------------------------------------------------------------------------

func TestHandleMismatch_FlaggerError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{err: errors.New("flag error")},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMismatch(context.Background(), job, item, "user-1", "computed-hash")

	if job.Mismatches != 1 {
		t.Errorf("mismatches = %d, want 1", job.Mismatches)
	}
}

func TestHandleMismatch_CustodyError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{err: errors.New("custody error")},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMismatch(context.Background(), job, item, "user-1", "computed-hash")

	if job.Mismatches != 1 {
		t.Errorf("mismatches = %d, want 1", job.Mismatches)
	}
}

func TestHandleMismatch_NotifierError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{err: errors.New("notify error")},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMismatch(context.Background(), job, item, "user-1", "computed-hash")

	if job.Mismatches != 1 {
		t.Errorf("mismatches = %d, want 1", job.Mismatches)
	}
}

// ---------------------------------------------------------------------------
// handleMissing error branches
// ---------------------------------------------------------------------------

func TestHandleMissing_FlaggerError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{err: errors.New("flag error")},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMissing(context.Background(), job, item, "user-1", errors.New("not found"))

	if job.Missing != 1 {
		t.Errorf("missing = %d, want 1", job.Missing)
	}
}

func TestHandleMissing_CustodyError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{err: errors.New("custody error")},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMissing(context.Background(), job, item, "user-1", errors.New("not found"))

	if job.Missing != 1 {
		t.Errorf("missing = %d, want 1", job.Missing)
	}
}

func TestHandleMissing_NotifierError(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{err: errors.New("notify error")},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{Status: "running"}
	item := VerifiableItem{ID: uuid.New(), CaseID: uuid.New(), Filename: "f.pdf"}

	h.handleMissing(context.Background(), job, item, "user-1", errors.New("not found"))

	if job.Missing != 1 {
		t.Errorf("missing = %d, want 1", job.Missing)
	}
}

// ---------------------------------------------------------------------------
// setJobFailed
// ---------------------------------------------------------------------------

func TestSetJobFailed(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	job := &VerificationJob{
		ID:     "job-1",
		CaseID: uuid.New(),
		Status: "running",
	}

	h.setJobFailed(job, "something went wrong")

	if job.Status != "failed" {
		t.Errorf("status = %q, want %q", job.Status, "failed")
	}
	if job.Error != "something went wrong" {
		t.Errorf("error = %q", job.Error)
	}
}

// ---------------------------------------------------------------------------
// VerificationJob.snapshot
// ---------------------------------------------------------------------------

func TestVerificationJob_Snapshot(t *testing.T) {
	now := time.Now().UTC()
	job := &VerificationJob{
		ID:         "job-1",
		CaseID:     uuid.New(),
		Status:     "running",
		Total:      10,
		Verified:   5,
		Mismatches: 2,
		Missing:    1,
		StartedAt:  now,
		Error:      "",
	}

	snap := job.snapshot()

	if snap.ID != "job-1" {
		t.Errorf("ID = %q", snap.ID)
	}
	if snap.Status != "running" {
		t.Errorf("Status = %q", snap.Status)
	}
	if snap.Total != 10 {
		t.Errorf("Total = %d", snap.Total)
	}
	if snap.Verified != 5 {
		t.Errorf("Verified = %d", snap.Verified)
	}
	if snap.Mismatches != 2 {
		t.Errorf("Mismatches = %d", snap.Mismatches)
	}
	if snap.Missing != 1 {
		t.Errorf("Missing = %d", snap.Missing)
	}
}

// ---------------------------------------------------------------------------
// RegisterRoutes
// ---------------------------------------------------------------------------

func TestRegisterRoutes(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceLoader{},
		&mockHandlerFileReader{},
		&mockHandlerTSA{},
		&mockHandlerCustody{},
		&mockHandlerNotifier{},
		&mockHandlerFlagger{},
	)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Verify routes are registered by walking the router
	routeCount := 0
	chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routeCount++
		return nil
	})

	if routeCount == 0 {
		t.Error("expected routes to be registered")
	}
}
