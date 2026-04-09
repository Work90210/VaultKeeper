package witnesses

// coverage_gaps_test.go — targeted tests for the specific branches and paths
// remaining below 100% after the initial test suite, as identified by running:
//
//	go tool cover -func=... | grep -v "100.0%"
//
// For paths that are structurally unreachable with the current implementation
// (e.g., HKDF io.ReadFull failures, AES cipher creation failures with already-
// validated 32-byte keys) the relevant source lines carry an "// unreachable:"
// annotation instead of a contrived test.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ───────────────────────────────────────────────────────────────────────────
// handler.go:Create — getCaseRole returns error (non-admin user, role loader
// fails) → 403 Forbidden
// ───────────────────────────────────────────────────────────────────────────

func TestHandler_Create_RoleError(t *testing.T) {
	wID := uuid.New()
	_ = wID
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&mockWitnessRepo{}, enc, custody, logger)

	// Role loader returns an error — simulates user having no access to the case.
	roleLoader := &mockRoleLoader{err: errors.New("no access")}
	h := NewHandler(svc, roleLoader, &mockAuditLogger{})

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := bytes.NewBufferString(`{"witness_code":"W","protection_status":"standard"}`)
	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/witnesses", body)
	req.Header.Set("Content-Type", "application/json")
	// auth.RoleUser so getCaseRole calls the role loader (not the admin shortcut)
	req = withAuth(req, auth.RoleUser)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// ───────────────────────────────────────────────────────────────────────────
// handler.go:Update — getCaseRole returns error path
// (The existing TestHandler_Update_RoleError covers this case already, but we
// include a complementary test using a nil audit logger to hit the nil-audit
// branch at line 221-224.)
// ───────────────────────────────────────────────────────────────────────────

func TestHandler_Update_WrongRole_NilAudit(t *testing.T) {
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New()}, nil
		},
	}
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, nil, logger)
	roleLoader := &mockRoleLoader{role: auth.CaseRoleObserver}
	// Pass nil audit so the nil-audit guard at handler line 221 is executed.
	h := NewHandler(svc, roleLoader, nil)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("PATCH", "/api/witnesses/"+wID.String(), body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleUser)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// ───────────────────────────────────────────────────────────────────────────
// handler.go:Create — wrong role with nil audit logger (covers the nil-audit
// guard at handler Create line 75)
// ───────────────────────────────────────────────────────────────────────────

func TestHandler_Create_WrongRole_NilAudit(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&mockWitnessRepo{}, enc, nil, logger)
	// Role loader returns Defence — not Investigator/Prosecutor.
	roleLoader := &mockRoleLoader{role: auth.CaseRoleDefence}
	h := NewHandler(svc, roleLoader, nil) // nil audit

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := bytes.NewBufferString(`{"witness_code":"W","protection_status":"standard"}`)
	req := httptest.NewRequest("POST", "/api/cases/"+uuid.New().String()+"/witnesses", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, auth.RoleUser)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403. body: %s", w.Code, w.Body.String())
	}
}

// ───────────────────────────────────────────────────────────────────────────
// service.go:toDecryptedView — contact_info decrypt error after full_name
// succeeds, and location decrypt error after contact_info succeeds.
// ───────────────────────────────────────────────────────────────────────────

// brokenEncryptor wraps a real Encryptor but overrides Decrypt to fail on
// the second or third call, letting the first succeed. This exercises the
// toDecryptedView error branches for contact_info and location.
type brokenAfterNDecryptor struct {
	*Encryptor
	failAfter int
	calls     int
}

func (b *brokenAfterNDecryptor) DecryptField(ciphertext []byte, witnessID, fieldName string) (*string, error) {
	b.calls++
	if b.calls > b.failAfter {
		return nil, errors.New("simulated decrypt failure")
	}
	return b.Encryptor.DecryptField(ciphertext, witnessID, fieldName)
}

// We cannot swap the encryptor inside Service via its public API, so we test
// toDecryptedView indirectly via Service.Get with a tampered witness stored
// in the repo. The tampered ciphertext makes one specific field fail to decrypt
// while the preceding field succeeds.

// makeCorrupt returns a byte slice with a valid version byte but invalid body,
// causing GCM authentication to fail.
func makeCorrupt(version byte) []byte {
	buf := make([]byte, keyVersionSize+nonceSize+gcmTagSize)
	buf[0] = version
	// rest is zeros — GCM Open will fail (authentication error)
	return buf
}

func TestService_ToDecryptedView_ContactInfoDecryptError(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	// full_name: valid encryption; contact_info: corrupt; location: valid.
	name := "Jane"
	fullNameEnc, _ := enc.EncryptField(&name, wID.String(), "full_name")
	corruptContactInfo := makeCorrupt(1)
	loc := "safe house"
	locationEnc, _ := enc.EncryptField(&loc, wID.String(), "location")

	w := Witness{
		ID:                   wID,
		CaseID:               uuid.New(),
		WitnessCode:          "W-CI-ERR",
		ProtectionStatus:     "standard",
		FullNameEncrypted:    fullNameEnc,
		ContactInfoEncrypted: corruptContactInfo,
		LocationEncrypted:    locationEnc,
		RelatedEvidence:      []uuid.UUID{},
	}

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return w, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, enc, custody, logger)

	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{UserID: "test"})
	// toDecryptedView is called; it fails on contact_info after full_name succeeds.
	_, err := svc.toDecryptedView(w)
	if err == nil {
		t.Fatal("expected error when contact_info decrypt fails, got nil")
	}
	_ = ctx
}

func TestService_ToDecryptedView_LocationDecryptError(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	name := "Jane"
	fullNameEnc, _ := enc.EncryptField(&name, wID.String(), "full_name")
	contact := "jane@example.com"
	contactInfoEnc, _ := enc.EncryptField(&contact, wID.String(), "contact_info")
	corruptLocation := makeCorrupt(1)

	w := Witness{
		ID:                   wID,
		CaseID:               uuid.New(),
		WitnessCode:          "W-LOC-ERR",
		ProtectionStatus:     "standard",
		FullNameEncrypted:    fullNameEnc,
		ContactInfoEncrypted: contactInfoEnc,
		LocationEncrypted:    corruptLocation,
		RelatedEvidence:      []uuid.UUID{},
	}

	_, err := (&Service{
		encryptor: enc,
	}).toDecryptedView(w)
	if err == nil {
		t.Fatal("expected error when location decrypt fails, got nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// key_rotation.go:rotateWitness — contact_info and location decrypt error
// paths after the preceding field succeeds.
// ───────────────────────────────────────────────────────────────────────────

func TestRotateWitness_ContactInfoDecryptError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	wID := uuid.New()
	idStr := wID.String()

	// full_name: valid; contact_info: corrupt (version byte matches old key but
	// body is garbage so Decrypt fails); location: valid.
	fullNameEnc := encryptedWith(t, oldEnc, idStr, "full_name", "Jane Doe")
	corruptContactInfo := makeCorrupt(oldEnc.CurrentVersion())
	locationEnc := encryptedWith(t, oldEnc, idStr, "location", "Safe house A")

	w := Witness{
		ID:                   wID,
		CaseID:               uuid.New(),
		FullNameEncrypted:    fullNameEnc,
		ContactInfoEncrypted: corruptContactInfo,
		LocationEncrypted:    locationEnc,
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness error: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1 for contact_info decrypt error, got %d", prog.Failed)
	}
}

func TestRotateWitness_LocationDecryptError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	newEnc := makeEncryptor(t, 2)

	wID := uuid.New()
	idStr := wID.String()

	fullNameEnc := encryptedWith(t, oldEnc, idStr, "full_name", "Jane Doe")
	contactInfoEnc := encryptedWith(t, oldEnc, idStr, "contact_info", "jane@example.com")
	corruptLocation := makeCorrupt(oldEnc.CurrentVersion())

	w := Witness{
		ID:                   wID,
		CaseID:               uuid.New(),
		FullNameEncrypted:    fullNameEnc,
		ContactInfoEncrypted: contactInfoEnc,
		LocationEncrypted:    corruptLocation,
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, newEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness error: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1 for location decrypt error, got %d", prog.Failed)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// service.go:Create — nil RelatedEvidence normalisation (ensures the nil→
// empty-slice path is covered)
// ───────────────────────────────────────────────────────────────────────────

func TestService_Create_NilRelatedEvidence_Normalised(t *testing.T) {
	repo := &mockWitnessRepo{
		createFn: func(_ context.Context, w Witness) (Witness, error) {
			w.ID = uuid.New()
			w.RelatedEvidence = []uuid.UUID{}
			return w, nil
		},
	}
	svc, _ := newTestService(repo)

	view, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-NIL-RE",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
		RelatedEvidence:  nil, // explicitly nil — must be normalised to []uuid.UUID{}
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.RelatedEvidence == nil {
		t.Error("RelatedEvidence should be non-nil empty slice, not nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// service.go:Update — encrypt identity fields when update repo succeeds then
// toDecryptedView encounters corrupt result → falls back to toView(false).
// (This specific path through Update where toDecryptedView fails on
// contact_info/location after full_name is covered by the fallback test, but
// we add an explicit test for the contact_info-decrypt-in-result path.)
// ───────────────────────────────────────────────────────────────────────────

func TestService_Update_ToDecryptedView_ContactInfoError(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	name := "Alice"
	fullNameEnc, _ := enc.EncryptField(&name, wID.String(), "full_name")
	corruptContactInfo := makeCorrupt(1)

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ Witness) (Witness, error) {
			return Witness{
				ID:                   wID,
				CaseID:               uuid.New(),
				WitnessCode:          "W-CI-ERR",
				ProtectionStatus:     "standard",
				FullNameEncrypted:    fullNameEnc,
				ContactInfoEncrypted: corruptContactInfo,
				RelatedEvidence:      []uuid.UUID{},
			}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, enc, nil, logger)

	view, err := svc.Update(context.Background(), wID, UpdateWitnessInput{}, uuid.New().String())
	if err != nil {
		t.Fatalf("Update should not return error on toDecryptedView fallback: %v", err)
	}
	// Fallback: identity not visible.
	if view.IdentityVisible {
		t.Error("fallback view should have IdentityVisible=false")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// handler.go:Update — ErrNotFound from GetCaseID (witness does not exist)
// ───────────────────────────────────────────────────────────────────────────

func TestHandler_Update_WitnessNotFound(t *testing.T) {
	// mockWitnessRepo default FindByID returns ErrNotFound.
	h := newTestHandler(&mockWitnessRepo{}, auth.CaseRoleInvestigator)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest("PATCH", "/api/witnesses/"+uuid.New().String(), nil)
	req = withAuth(req, auth.RoleSystemAdmin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404. body: %s", w.Code, w.Body.String())
	}
}

// ───────────────────────────────────────────────────────────────────────────
// repository.go:NewRepository — the constructor stores the pool pointer
// without dereferencing it, so passing nil is safe for a unit-level smoke test.
// ───────────────────────────────────────────────────────────────────────────

func TestNewRepository_CalledWithNilPool(t *testing.T) {
	// NewRepository only stores the pool; it never dereferences it, so passing
	// nil is safe here and exercises the constructor path in coverage.
	repo := NewRepository(nil)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// encryption.go:Encrypt — "current key version not found" branch (line 87-91).
//
// NewEncryptor guarantees current is always present in keys, so this branch
// cannot be reached through the public API. However, because tests live in the
// same package (package witnesses) they may construct a broken Encryptor
// directly via the struct literal to exercise the defensive branch without
// changing production code.
// ───────────────────────────────────────────────────────────────────────────

func TestEncrypt_CurrentKeyVersionNotFound(t *testing.T) {
	// Construct an Encryptor whose keys map is empty but current is non-zero.
	// This triggers the defensive "current key version not found" error return
	// that cannot be reached through NewEncryptor.
	enc := &Encryptor{
		keys:    map[byte]EncryptionKey{}, // empty — current version absent
		current: 1,
	}
	_, err := enc.Encrypt([]byte("hello"), "witness-1", "full_name")
	if err == nil {
		t.Fatal("expected error when current key version is missing from keys map, got nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// service.go:Create — encrypt-error branches for full_name, contact_info,
// and location (lines 66-82). These are annotated unreachable for a correctly
// initialised Encryptor, but are exercised here via a broken Encryptor that
// has no keys loaded, causing EncryptField → Encrypt to fail on the first
// non-nil field.
// ───────────────────────────────────────────────────────────────────────────

func TestService_Create_EncryptFullNameError(t *testing.T) {
	// Broken encryptor: empty keys map, current=1. Encrypt will fail immediately.
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(&mockWitnessRepo{}, brokenEnc, nil, logger)

	name := "Jane Doe"
	_, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-ENC-ERR",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
		FullName:         &name,
	})
	if err == nil {
		t.Fatal("expected error when full_name encryption fails, got nil")
	}
}

func TestService_Create_EncryptContactInfoError(t *testing.T) {
	// Encryptor with a valid key for version 1 so full_name (which is nil here)
	// is skipped — EncryptField returns nil for nil input — then contact_info
	// fails because the broken encryptor has no keys.
	//
	// Wait: EncryptField returns nil,nil for nil input without calling Encrypt.
	// So to reach the contact_info error path, full_name must be nil and
	// contact_info must be non-nil. Use broken encryptor for encryption.
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(&mockWitnessRepo{}, brokenEnc, nil, logger)

	contact := "jane@example.com"
	_, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-CI-ENC-ERR",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
		FullName:         nil,     // nil → EncryptField returns nil, nil — no Encrypt call
		ContactInfo:      &contact, // non-nil → triggers Encrypt → fails
	})
	if err == nil {
		t.Fatal("expected error when contact_info encryption fails, got nil")
	}
}

func TestService_Create_EncryptLocationError(t *testing.T) {
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(&mockWitnessRepo{}, brokenEnc, nil, logger)

	loc := "safe house"
	_, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-LOC-ENC-ERR",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
		FullName:         nil, // nil → skips Encrypt
		ContactInfo:      nil, // nil → skips Encrypt
		Location:         &loc, // non-nil → triggers Encrypt → fails
	})
	if err == nil {
		t.Fatal("expected error when location encryption fails, got nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// service.go:Update — encrypt-error branches for full_name, contact_info,
// and location (lines 179-200).
// ───────────────────────────────────────────────────────────────────────────

func TestService_Update_EncryptFullNameError(t *testing.T) {
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, brokenEnc, nil, logger)

	name := "Alice"
	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{FullName: &name}, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when Update full_name encryption fails, got nil")
	}
}

func TestService_Update_EncryptContactInfoError(t *testing.T) {
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, brokenEnc, nil, logger)

	contact := "contact@example.com"
	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{
		FullName:    nil, // nil → not updated, skips Encrypt
		ContactInfo: &contact,
	}, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when Update contact_info encryption fails, got nil")
	}
}

func TestService_Update_EncryptLocationError(t *testing.T) {
	brokenEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 1}
	wID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, brokenEnc, nil, logger)

	loc := "safe house"
	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{
		FullName:    nil, // nil → skips Encrypt
		ContactInfo: nil, // nil → skips Encrypt
		Location:    &loc,
	}, uuid.New().String())
	if err == nil {
		t.Fatal("expected error when Update location encryption fails, got nil")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// key_rotation.go:rotateWitness — re-encrypt error branches for full_name,
// contact_info, and location (lines 109-114, 122-128, 134-140).
//
// These paths fire when newEnc.Encrypt fails after oldEnc.Decrypt succeeds.
// A broken newEnc (empty keys map) triggers the error while oldEnc remains
// functional so decryption works.
// ───────────────────────────────────────────────────────────────────────────

func TestRotateWitness_ReEncryptFullNameError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	// newEnc has an empty keys map, so Encrypt will fail immediately.
	brokenNewEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 2}

	wID := uuid.New()
	idStr := wID.String()
	w := Witness{
		ID:                wID,
		CaseID:            uuid.New(),
		FullNameEncrypted: encryptedWith(t, oldEnc, idStr, "full_name", "Jane Doe"),
		// contact_info and location are nil so only full_name rotation is attempted
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, brokenNewEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness error: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1 for full_name re-encrypt error, got %d", prog.Failed)
	}
}

func TestRotateWitness_ReEncryptContactInfoError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	brokenNewEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 2}

	wID := uuid.New()
	idStr := wID.String()
	w := Witness{
		ID:                   wID,
		CaseID:               uuid.New(),
		FullNameEncrypted:    nil, // nil → skipped in rotateWitness
		ContactInfoEncrypted: encryptedWith(t, oldEnc, idStr, "contact_info", "jane@example.com"),
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, brokenNewEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness error: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1 for contact_info re-encrypt error, got %d", prog.Failed)
	}
}

func TestRotateWitness_ReEncryptLocationError(t *testing.T) {
	oldEnc := makeEncryptor(t, 1)
	brokenNewEnc := &Encryptor{keys: map[byte]EncryptionKey{}, current: 2}

	wID := uuid.New()
	idStr := wID.String()
	w := Witness{
		ID:                wID,
		CaseID:            uuid.New(),
		FullNameEncrypted: nil, // nil → skipped
		LocationEncrypted: encryptedWith(t, oldEnc, idStr, "location", "Safe house"),
	}

	repo := &mockRepo{witnesses: []Witness{w}}
	job := NewKeyRotationJob(repo, oldEnc, brokenNewEnc, nil, silentLogger())

	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run should not propagate individual witness error: %v", err)
	}

	prog := job.Progress()
	if prog.Failed != 1 {
		t.Errorf("want Failed=1 for location re-encrypt error, got %d", prog.Failed)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// encryption.go remaining unreachable paths — documented:
//
// deriveKey line 75: io.ReadFull on hkdf.New's reader never errors (HMAC
//                    SHA-256 produces an infinite byte stream).
//                    unreachable: covered by // unreachable: comment in source.
//
// Encrypt/Decrypt: aes.NewCipher and cipher.NewGCM errors are structurally
//                  unreachable: HKDF always produces 32 bytes (valid AES-256
//                  key length) and AES always uses a 16-byte block size.
//                  unreachable: covered by // unreachable: comments in source.
//
// Encrypt line 115: io.ReadFull(rand.Reader, nonce) — theoretically possible
//                   but requires mocking crypto/rand.Reader, which demands
//                   production code changes (injecting an io.Reader). The
//                   defensive error path is preserved for safety; the
//                   // unreachable: comment is omitted from source because
//                   the OS could theoretically fail to supply entropy.
// ───────────────────────────────────────────────────────────────────────────
