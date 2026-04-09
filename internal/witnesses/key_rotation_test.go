package witnesses

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

type updatedFields struct {
	fullName    []byte
	contactInfo []byte
	location    []byte
}

type mockRepo struct {
	witnesses    []Witness
	updatedMap   map[uuid.UUID]updatedFields
	findAllErr   error
	updateErr    error
	updateCalled atomic.Int64
}

func (m *mockRepo) FindAll(_ context.Context) ([]Witness, error) {
	if m.findAllErr != nil {
		return nil, m.findAllErr
	}
	return m.witnesses, nil
}

func (m *mockRepo) UpdateEncryptedFields(_ context.Context, id uuid.UUID, fn, ci, loc []byte) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updateCalled.Add(1)
	if m.updatedMap == nil {
		m.updatedMap = make(map[uuid.UUID]updatedFields)
	}
	m.updatedMap[id] = updatedFields{fullName: fn, contactInfo: ci, location: loc}
	return nil
}

func (m *mockRepo) Create(_ context.Context, w Witness) (Witness, error) {
	return Witness{}, ErrNotFound
}

func (m *mockRepo) FindByID(_ context.Context, _ uuid.UUID) (Witness, error) {
	return Witness{}, ErrNotFound
}

func (m *mockRepo) FindByCase(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
	return nil, 0, ErrNotFound
}

func (m *mockRepo) Update(_ context.Context, _ uuid.UUID, _ Witness) (Witness, error) {
	return Witness{}, ErrNotFound
}

// ---------------------------------------------------------------------------
// Mock custody recorder
// ---------------------------------------------------------------------------

type custodyEvent struct {
	caseID      uuid.UUID
	action      string
	actorUserID string
	detail      map[string]string
}

type mockCustody struct {
	events []custodyEvent
}

func (mc *mockCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return nil
}

func (mc *mockCustody) RecordCaseEvent(_ context.Context, caseID uuid.UUID, action, actorUserID string, detail map[string]string) error {
	mc.events = append(mc.events, custodyEvent{
		caseID:      caseID,
		action:      action,
		actorUserID: actorUserID,
		detail:      detail,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeEncryptor(t *testing.T, version byte) *Encryptor {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = version + byte(i)
	}
	enc, err := NewEncryptor(EncryptionKey{Version: version, Key: key})
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	return enc
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

// encryptedWith encrypts a plaintext string using the given Encryptor,
// bound to the witness and field names. Fails the test on error.
func encryptedWith(t *testing.T, enc *Encryptor, witnessID, field, plaintext string) []byte {
	t.Helper()
	b, err := enc.Encrypt([]byte(plaintext), witnessID, field)
	if err != nil {
		t.Fatalf("encrypt %s: %v", field, err)
	}
	return b
}

func newWitnessWithEnc(t *testing.T, enc *Encryptor) Witness {
	t.Helper()
	id := uuid.New()
	idStr := id.String()
	w := Witness{
		ID:                   id,
		CaseID:               uuid.New(),
		WitnessCode:          "W-001",
		FullNameEncrypted:    encryptedWith(t, enc, idStr, "full_name", "Jane Doe"),
		ContactInfoEncrypted: encryptedWith(t, enc, idStr, "contact_info", "jane@example.com"),
		LocationEncrypted:    encryptedWith(t, enc, idStr, "location", "Safe house A"),
	}
	return w
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_RotatesAllWitnesses
// ---------------------------------------------------------------------------

func TestKeyRotationRun_RotatesAllWitnesses(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	w1 := newWitnessWithEnc(t, oldEnc)
	w2 := newWitnessWithEnc(t, oldEnc)

	repo := &mockRepo{witnesses: []Witness{w1, w2}}
	custody := &mockCustody{}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, custody, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	prog := job.Progress()
	if prog.Total != 2 {
		t.Errorf("want Total=2, got %d", prog.Total)
	}
	if prog.Processed != 2 {
		t.Errorf("want Processed=2, got %d", prog.Processed)
	}
	if prog.Failed != 0 {
		t.Errorf("want Failed=0, got %d", prog.Failed)
	}
	if prog.Running {
		t.Error("want Running=false after completion")
	}

	// Verify that both witnesses were re-encrypted with the new key version.
	for _, id := range []uuid.UUID{w1.ID, w2.ID} {
		fields, ok := repo.updatedMap[id]
		if !ok {
			t.Errorf("witness %s was not updated in repository", id)
			continue
		}
		if len(fields.fullName) == 0 || fields.fullName[0] != newEnc.CurrentVersion() {
			t.Errorf("witness %s: full_name not re-encrypted with new version", id)
		}
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_SkipsAlreadyRotatedWitnesses
// ---------------------------------------------------------------------------

func TestKeyRotationRun_SkipsAlreadyRotatedWitnesses(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	// w1 already uses new key version, w2 still uses old.
	w1 := newWitnessWithEnc(t, newEnc) // already on new key
	w2 := newWitnessWithEnc(t, oldEnc) // needs rotation

	repo := &mockRepo{witnesses: []Witness{w1, w2}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	prog := job.Progress()
	if prog.Processed != 2 {
		t.Errorf("want Processed=2 (both counted), got %d", prog.Processed)
	}
	if prog.Failed != 0 {
		t.Errorf("want Failed=0, got %d", prog.Failed)
	}

	// Only w2 should have been written to the repository.
	if _, skipped := repo.updatedMap[w1.ID]; skipped {
		t.Error("already-rotated witness w1 should not have been updated")
	}
	if _, updated := repo.updatedMap[w2.ID]; !updated {
		t.Error("old-key witness w2 should have been updated")
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_ContextCancellationStopsMidway
// ---------------------------------------------------------------------------

func TestKeyRotationRun_ContextCancellationStopsMidway(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	// Build 5 witnesses encrypted with old key.
	witnesses := make([]Witness, 5)
	for i := range witnesses {
		witnesses[i] = newWitnessWithEnc(t, oldEnc)
	}

	// Use a blockingRepo that cancels the context after the first update.
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	type blockingRepo struct {
		mockRepo
	}
	repo := &mockRepo{witnesses: witnesses}

	// Wrap UpdateEncryptedFields to cancel after first call.
	cancellingRepo := struct {
		*mockRepo
		updateFn func(context.Context, uuid.UUID, []byte, []byte, []byte) error
	}{
		mockRepo: repo,
	}
	_ = cancellingRepo

	// We need a custom approach: use a repo that cancels the context after first update.
	cancelRepo := &cancelOnNthUpdateRepo{
		inner:     repo,
		cancelAt:  1,
		cancelFn:  cancel,
		callCount: &callCount,
	}

	job := NewKeyRotationJob(cancelRepo, oldEnc, newEnc, nil, silentLogger())
	err := job.Run(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}

	prog := job.Progress()
	if prog.Running {
		t.Error("want Running=false after cancellation")
	}
	// At least one witness was processed and the rest were not.
	if prog.Processed == 0 {
		t.Error("want at least one processed witness before cancellation")
	}
	if prog.Processed >= 5 {
		t.Error("want cancellation to stop before all 5 witnesses are processed")
	}
}

// cancelOnNthUpdateRepo wraps a mockRepo and cancels a context after N updates.
type cancelOnNthUpdateRepo struct {
	inner     *mockRepo
	cancelAt  int
	cancelFn  context.CancelFunc
	callCount *int
}

func (r *cancelOnNthUpdateRepo) FindAll(ctx context.Context) ([]Witness, error) {
	return r.inner.FindAll(ctx)
}

func (r *cancelOnNthUpdateRepo) UpdateEncryptedFields(ctx context.Context, id uuid.UUID, fn, ci, loc []byte) error {
	err := r.inner.UpdateEncryptedFields(ctx, id, fn, ci, loc)
	*r.callCount++
	if *r.callCount >= r.cancelAt {
		r.cancelFn()
	}
	return err
}

func (r *cancelOnNthUpdateRepo) Create(ctx context.Context, w Witness) (Witness, error) {
	return r.inner.Create(ctx, w)
}

func (r *cancelOnNthUpdateRepo) FindByID(ctx context.Context, id uuid.UUID) (Witness, error) {
	return r.inner.FindByID(ctx, id)
}

func (r *cancelOnNthUpdateRepo) FindByCase(ctx context.Context, caseID uuid.UUID, p Pagination) ([]Witness, int, error) {
	return r.inner.FindByCase(ctx, caseID, p)
}

func (r *cancelOnNthUpdateRepo) Update(ctx context.Context, id uuid.UUID, w Witness) (Witness, error) {
	return r.inner.Update(ctx, id, w)
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_IndividualWitnessFailureContinues
// ---------------------------------------------------------------------------

func TestKeyRotationRun_IndividualWitnessFailureContinues(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	good1 := newWitnessWithEnc(t, oldEnc)
	bad := newWitnessWithEnc(t, oldEnc)
	good2 := newWitnessWithEnc(t, oldEnc)

	updateErr := errors.New("simulated DB error")
	callCount := 0
	failingRepo := &failOnWitnessRepo{
		inner:    &mockRepo{witnesses: []Witness{good1, bad, good2}},
		failID:   bad.ID,
		failErr:  updateErr,
		calls:    &callCount,
	}

	job := NewKeyRotationJob(failingRepo, oldEnc, newEnc, nil, silentLogger())
	err := job.Run(context.Background())

	if err != nil {
		t.Errorf("Run should not return error when individual witness fails, got: %v", err)
	}

	prog := job.Progress()
	if prog.Total != 3 {
		t.Errorf("want Total=3, got %d", prog.Total)
	}
	if prog.Processed != 2 {
		t.Errorf("want Processed=2 (good witnesses), got %d", prog.Processed)
	}
	if prog.Failed != 1 {
		t.Errorf("want Failed=1, got %d", prog.Failed)
	}

	// Good witnesses must have been updated.
	if _, ok := failingRepo.inner.updatedMap[good1.ID]; !ok {
		t.Error("good1 should have been updated")
	}
	if _, ok := failingRepo.inner.updatedMap[good2.ID]; !ok {
		t.Error("good2 should have been updated")
	}
	if _, ok := failingRepo.inner.updatedMap[bad.ID]; ok {
		t.Error("failing witness should not appear in updated map")
	}
}

// failOnWitnessRepo fails UpdateEncryptedFields for a specific witness ID.
type failOnWitnessRepo struct {
	inner   *mockRepo
	failID  uuid.UUID
	failErr error
	calls   *int
}

func (r *failOnWitnessRepo) FindAll(ctx context.Context) ([]Witness, error) {
	return r.inner.FindAll(ctx)
}

func (r *failOnWitnessRepo) UpdateEncryptedFields(ctx context.Context, id uuid.UUID, fn, ci, loc []byte) error {
	*r.calls++
	if id == r.failID {
		return r.failErr
	}
	return r.inner.UpdateEncryptedFields(ctx, id, fn, ci, loc)
}

func (r *failOnWitnessRepo) Create(ctx context.Context, w Witness) (Witness, error) {
	return r.inner.Create(ctx, w)
}

func (r *failOnWitnessRepo) FindByID(ctx context.Context, id uuid.UUID) (Witness, error) {
	return r.inner.FindByID(ctx, id)
}

func (r *failOnWitnessRepo) FindByCase(ctx context.Context, caseID uuid.UUID, p Pagination) ([]Witness, int, error) {
	return r.inner.FindByCase(ctx, caseID, p)
}

func (r *failOnWitnessRepo) Update(ctx context.Context, id uuid.UUID, w Witness) (Witness, error) {
	return r.inner.Update(ctx, id, w)
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_EmptyWitnessList
// ---------------------------------------------------------------------------

func TestKeyRotationRun_EmptyWitnessList(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	repo := &mockRepo{witnesses: []Witness{}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run with empty list returned error: %v", err)
	}

	prog := job.Progress()
	if prog.Total != 0 {
		t.Errorf("want Total=0, got %d", prog.Total)
	}
	if prog.Processed != 0 {
		t.Errorf("want Processed=0, got %d", prog.Processed)
	}
	if prog.Failed != 0 {
		t.Errorf("want Failed=0, got %d", prog.Failed)
	}
	if prog.Running {
		t.Error("want Running=false")
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_FindAllError
// ---------------------------------------------------------------------------

func TestKeyRotationRun_FindAllError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	findErr := errors.New("database unavailable")
	repo := &mockRepo{findAllErr: findErr}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	err := job.Run(context.Background())
	if err == nil {
		t.Fatal("want error when FindAll fails, got nil")
	}
	if !errors.Is(err, findErr) {
		t.Errorf("want wrapped findErr, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_CustodyEventsLogged
// ---------------------------------------------------------------------------

func TestKeyRotationRun_CustodyEventsLogged(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	w1 := newWitnessWithEnc(t, oldEnc)
	w2 := newWitnessWithEnc(t, oldEnc)

	repo := &mockRepo{witnesses: []Witness{w1, w2}}
	custody := &mockCustody{}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, custody, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(custody.events) != 2 {
		t.Fatalf("want 2 custody events, got %d", len(custody.events))
	}

	witnessIDs := map[string]bool{
		w1.ID.String(): false,
		w2.ID.String(): false,
	}

	for _, ev := range custody.events {
		if ev.action != "witness_key_rotated" {
			t.Errorf("want action=witness_key_rotated, got %q", ev.action)
		}
		if ev.actorUserID != "system" {
			t.Errorf("want actorUserID=system, got %q", ev.actorUserID)
		}
		wid, ok := ev.detail["witness_id"]
		if !ok {
			t.Error("custody event missing witness_id in detail")
			continue
		}
		// Verify the detail does NOT contain identity content.
		for k, v := range ev.detail {
			if k == "full_name" || k == "contact_info" || k == "location" {
				t.Errorf("custody event must not contain identity field %q=%q", k, v)
			}
		}
		witnessIDs[wid] = true
	}

	for id, seen := range witnessIDs {
		if !seen {
			t.Errorf("no custody event recorded for witness %s", id)
		}
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_NilCustody
// ---------------------------------------------------------------------------

func TestKeyRotationRun_NilCustody(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	w := newWitnessWithEnc(t, oldEnc)
	repo := &mockRepo{witnesses: []Witness{w}}
	// Pass nil custody — should not panic.
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run with nil custody returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationProgress_Tracking
// ---------------------------------------------------------------------------

func TestKeyRotationProgress_Tracking(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	witnesses := make([]Witness, 3)
	for i := range witnesses {
		witnesses[i] = newWitnessWithEnc(t, oldEnc)
	}

	repo := &mockRepo{witnesses: witnesses}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	// Before Run: zero progress.
	before := job.Progress()
	if before.Total != 0 || before.Processed != 0 || before.Failed != 0 {
		t.Errorf("want zeroed progress before Run, got %+v", before)
	}

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	after := job.Progress()
	if after.Total != 3 {
		t.Errorf("want Total=3, got %d", after.Total)
	}
	if after.Processed != 3 {
		t.Errorf("want Processed=3, got %d", after.Processed)
	}
	if after.Running {
		t.Error("want Running=false after completion")
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_Resumable
// ---------------------------------------------------------------------------

func TestKeyRotationRun_Resumable(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	// w1 already rotated to new key; w2 still on old key.
	w1 := newWitnessWithEnc(t, newEnc)
	w2 := newWitnessWithEnc(t, oldEnc)

	repo := &mockRepo{witnesses: []Witness{w1, w2}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	// First run.
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("first Run error: %v", err)
	}

	// Simulate a second run: w2 is now updated to new key in the "DB".
	// Build fresh witness list where both are on new key.
	w2Updated := w2
	w2Updated.FullNameEncrypted = repo.updatedMap[w2.ID].fullName
	w2Updated.ContactInfoEncrypted = repo.updatedMap[w2.ID].contactInfo
	w2Updated.LocationEncrypted = repo.updatedMap[w2.ID].location

	repo2 := &mockRepo{witnesses: []Witness{w1, w2Updated}}
	job2 := NewKeyRotationJob(repo2, oldEnc, newEnc, nil, silentLogger())

	if err := job2.Run(context.Background()); err != nil {
		t.Fatalf("second Run error: %v", err)
	}

	// Neither witness should have been written on the second run.
	if len(repo2.updatedMap) != 0 {
		t.Errorf("second run should skip all already-rotated witnesses, got updates: %v", repo2.updatedMap)
	}

	prog := job2.Progress()
	if prog.Failed != 0 {
		t.Errorf("want Failed=0 on second run, got %d", prog.Failed)
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_WitnessWithNilFields
// ---------------------------------------------------------------------------

func TestKeyRotationRun_WitnessWithNilFields(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	// A witness with no encrypted fields at all (nil slices).
	w := Witness{
		ID:     uuid.New(),
		CaseID: uuid.New(),
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run with nil-field witness returned error: %v", err)
	}

	// The witness has no version byte; it will attempt rotation (not skip).
	// UpdateEncryptedFields should be called with nil slices.
	prog := job.Progress()
	if prog.Failed != 0 {
		t.Errorf("want Failed=0 for nil-field witness, got %d", prog.Failed)
	}
	if prog.Processed != 1 {
		t.Errorf("want Processed=1, got %d", prog.Processed)
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_DecryptionError
// ---------------------------------------------------------------------------

func TestKeyRotationRun_DecryptionError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	// Craft a witness whose FullNameEncrypted starts with old version byte but
	// has garbage content so Decrypt fails.
	w := Witness{
		ID:     uuid.New(),
		CaseID: uuid.New(),
		// starts with version byte 1 (old), followed by garbage
		FullNameEncrypted: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness errors: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1, got %d", prog.Failed)
	}
}

// ---------------------------------------------------------------------------
// TestKeyRotationRun_ConcurrentProgressSafety
// ---------------------------------------------------------------------------

func TestKeyRotationRun_ConcurrentProgressSafety(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	witnesses := make([]Witness, 20)
	for i := range witnesses {
		witnesses[i] = newWitnessWithEnc(t, oldEnc)
	}

	repo := &mockRepo{witnesses: witnesses}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	done := make(chan struct{})
	go func() {
		// Continuously read progress while Run executes.
		for {
			select {
			case <-done:
				return
			default:
				_ = job.Progress()
				time.Sleep(time.Microsecond)
			}
		}
	}()

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	close(done)
}
