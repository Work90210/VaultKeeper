package witnesses

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// --- mocks ---

type mockWitnessRepo struct {
	createFn             func(ctx context.Context, w Witness) (Witness, error)
	findByIDFn           func(ctx context.Context, id uuid.UUID) (Witness, error)
	findByCaseFn         func(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Witness, int, error)
	updateFn             func(ctx context.Context, id uuid.UUID, w Witness) (Witness, error)
	findAllFn            func(ctx context.Context) ([]Witness, error)
	updateEncryptedFn    func(ctx context.Context, id uuid.UUID, fn, ci, loc []byte) error
}

func (m *mockWitnessRepo) Create(ctx context.Context, w Witness) (Witness, error) {
	if m.createFn != nil {
		return m.createFn(ctx, w)
	}
	w.ID = uuid.New()
	return w, nil
}

func (m *mockWitnessRepo) FindByID(ctx context.Context, id uuid.UUID) (Witness, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return Witness{}, ErrNotFound
}

func (m *mockWitnessRepo) FindByCase(ctx context.Context, caseID uuid.UUID, page Pagination) ([]Witness, int, error) {
	if m.findByCaseFn != nil {
		return m.findByCaseFn(ctx, caseID, page)
	}
	return nil, 0, nil
}

func (m *mockWitnessRepo) Update(ctx context.Context, id uuid.UUID, w Witness) (Witness, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, w)
	}
	return w, nil
}

func (m *mockWitnessRepo) FindAll(ctx context.Context) ([]Witness, error) {
	if m.findAllFn != nil {
		return m.findAllFn(ctx)
	}
	return nil, nil
}

func (m *mockWitnessRepo) UpdateEncryptedFields(ctx context.Context, id uuid.UUID, fn, ci, loc []byte) error {
	if m.updateEncryptedFn != nil {
		return m.updateEncryptedFn(ctx, id, fn, ci, loc)
	}
	return nil
}

// mockCustody is defined in key_rotation_test.go — reuse it here.
// We just need a helper to check event actions.

func newTestService(repo Repository) (*Service, *mockCustody) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)
	return svc, custody
}

// --- tests ---

func TestService_Create_Valid(t *testing.T) {
	repo := &mockWitnessRepo{
		createFn: func(_ context.Context, w Witness) (Witness, error) {
			w.ID = uuid.New()
			return w, nil
		},
	}
	svc, custody := newTestService(repo)

	view, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-001",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.WitnessCode != "W-001" {
		t.Errorf("WitnessCode: got %q, want W-001", view.WitnessCode)
	}
	if len(custody.events) == 0 || custody.events[0].action != "witness_created" {
		t.Error("expected witness_created custody event")
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	svc, _ := newTestService(&mockWitnessRepo{})

	tests := []struct {
		name  string
		input CreateWitnessInput
	}{
		{"empty case_id", CreateWitnessInput{WitnessCode: "W", ProtectionStatus: "standard", CreatedBy: uuid.New().String()}},
		{"empty witness_code", CreateWitnessInput{CaseID: uuid.New(), ProtectionStatus: "standard", CreatedBy: uuid.New().String()}},
		{"invalid protection", CreateWitnessInput{CaseID: uuid.New(), WitnessCode: "W", ProtectionStatus: "invalid", CreatedBy: uuid.New().String()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tt.input)
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Errorf("expected ValidationError, got %v", err)
			}
		})
	}
}

func TestService_Create_InvalidCreatedBy(t *testing.T) {
	svc, _ := newTestService(&mockWitnessRepo{
		createFn: func(_ context.Context, w Witness) (Witness, error) { return w, nil },
	})
	_, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID: uuid.New(), WitnessCode: "W", ProtectionStatus: "standard", CreatedBy: "not-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid CreatedBy")
	}
}

func TestService_Get_Investigator(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()
	name := "Jane Doe"
	fullNameEnc, _ := enc.EncryptField(&name, wID.String(), "full_name")

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, id uuid.UUID) (Witness, error) {
			return Witness{
				ID: wID, CaseID: uuid.New(), WitnessCode: "W-001",
				FullNameEncrypted: fullNameEnc, ProtectionStatus: "standard",
			}, nil
		},
	}

	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{UserID: "test"})
	view, err := svc.Get(ctx, wID, auth.CaseRoleInvestigator)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !view.IdentityVisible {
		t.Error("investigator should see identity")
	}
	if view.FullName == nil || *view.FullName != "Jane Doe" {
		t.Errorf("FullName: got %v, want Jane Doe", view.FullName)
	}
}

func TestService_Get_Defence(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()
	name := "Jane Doe"
	fullNameEnc, _ := enc.EncryptField(&name, wID.String(), "full_name")

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{
				ID: wID, CaseID: uuid.New(), WitnessCode: "W-001",
				FullNameEncrypted: fullNameEnc, ProtectionStatus: "standard",
			}, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{UserID: "test"})
	view, err := svc.Get(ctx, wID, auth.CaseRoleDefence)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.IdentityVisible {
		t.Error("defence should NOT see identity")
	}
	if view.FullName != nil {
		t.Errorf("FullName should be nil for defence, got %v", view.FullName)
	}
}

func TestService_Get_JudgeWithFlag(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{
				ID: wID, CaseID: uuid.New(), WitnessCode: "W-001",
				ProtectionStatus: "standard", JudgeIdentityVisible: true,
			}, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{UserID: "test"})
	view, err := svc.Get(ctx, wID, auth.CaseRoleJudge)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !view.IdentityVisible {
		t.Error("judge with flag should see identity")
	}
}

func TestService_Get_JudgeWithoutFlag(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{
				ID: wID, CaseID: uuid.New(), WitnessCode: "W-001",
				ProtectionStatus: "standard", JudgeIdentityVisible: false,
			}, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{UserID: "test"})
	view, err := svc.Get(ctx, wID, auth.CaseRoleJudge)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.IdentityVisible {
		t.Error("judge without flag should NOT see identity")
	}
}

func TestService_Get_NotFound(t *testing.T) {
	svc, _ := newTestService(&mockWitnessRepo{})
	_, err := svc.Get(context.Background(), uuid.New(), auth.CaseRoleInvestigator)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_GetCaseID(t *testing.T) {
	caseID := uuid.New()
	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{CaseID: caseID}, nil
		},
	}
	svc, _ := newTestService(repo)
	got, err := svc.GetCaseID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetCaseID: %v", err)
	}
	if got != caseID {
		t.Errorf("got %v, want %v", got, caseID)
	}
}

func TestService_List(t *testing.T) {
	repo := &mockWitnessRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
			return []Witness{{WitnessCode: "W-001", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}}, 1, nil
		},
	}
	svc, _ := newTestService(repo)

	views, total, err := svc.List(context.Background(), uuid.New(), auth.CaseRoleDefence, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(views) != 1 {
		t.Errorf("expected 1 result, got %d (total %d)", len(views), total)
	}
	if views[0].IdentityVisible {
		t.Error("defence role should not see identity in list")
	}
}

func TestService_Update(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), WitnessCode: "W-001", ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, w Witness) (Witness, error) {
			return w, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	newStatus := "protected"
	summary := "Updated statement"
	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{
		ProtectionStatus: &newStatus,
		StatementSummary: &summary,
	}, uuid.New().String())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(custody.events) == 0 || custody.events[0].action != "witness_updated" {
		t.Error("expected witness_updated custody event")
	}
}

func TestService_Update_InvalidProtectionStatus(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, nil, logger)

	bad := "invalid"
	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{ProtectionStatus: &bad}, "actor")
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestService_RecordCustodyEvent_NilCustody(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&mockWitnessRepo{}, enc, nil, logger)
	// Should not panic
	svc.recordCustodyEvent(context.Background(), uuid.New(), "test", "actor", nil)
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "name", Message: "required"}
	if err.Error() != "name: required" {
		t.Errorf("got %q, want %q", err.Error(), "name: required")
	}
}

func TestCanViewIdentity(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(&mockWitnessRepo{}, enc, nil, logger)

	w := Witness{JudgeIdentityVisible: true}
	tests := []struct {
		role auth.CaseRole
		want bool
	}{
		{auth.CaseRoleInvestigator, true},
		{auth.CaseRoleProsecutor, true},
		{auth.CaseRoleDefence, false},
		{auth.CaseRoleObserver, false},
		{auth.CaseRoleJudge, true}, // flag is true
		{auth.CaseRoleVictimRepresentative, false},
	}
	for _, tt := range tests {
		got := svc.canViewIdentity(w, tt.role)
		if got != tt.want {
			t.Errorf("canViewIdentity(%s): got %v, want %v", tt.role, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Service.List — decryption error path (investigator role with corrupt ciphertext)
// ---------------------------------------------------------------------------

func TestService_List_DecryptionError(t *testing.T) {
	// Produce a witness encrypted with key version 1 then swap in garbage bytes
	// so that decryption fails while the version byte still matches.
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	// Valid encryption for contact_info and location; corrupt full_name.
	ci, _ := enc.EncryptField(nil, wID.String(), "contact_info")
	loc, _ := enc.EncryptField(nil, wID.String(), "location")

	// Corrupt: keep version byte 1 but fill rest with zeros (invalid ciphertext).
	corruptFullName := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	repo := &mockWitnessRepo{
		findByCaseFn: func(_ context.Context, _ uuid.UUID, _ Pagination) ([]Witness, int, error) {
			return []Witness{
				{
					ID:                   wID,
					CaseID:               uuid.New(),
					WitnessCode:          "W-CORRUPT",
					ProtectionStatus:     "standard",
					FullNameEncrypted:    corruptFullName,
					ContactInfoEncrypted: ci,
					LocationEncrypted:    loc,
					RelatedEvidence:      []uuid.UUID{},
				},
			}, 1, nil
		},
	}

	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, enc, custody, logger)

	// Investigator role → tries to decrypt → hits the error path → falls back to
	// redacted view without panicking.
	views, total, err := svc.List(context.Background(), uuid.New(), auth.CaseRoleInvestigator, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("List should not return error on decryption failure, got: %v", err)
	}
	if total != 1 || len(views) != 1 {
		t.Errorf("expected 1 result, got %d (total %d)", len(views), total)
	}
	// The fallback view should have IdentityVisible=false.
	if views[0].IdentityVisible {
		t.Error("decryption-failed view should have IdentityVisible=false")
	}
}

// ---------------------------------------------------------------------------
// Service.Update — repository error
// ---------------------------------------------------------------------------

func TestService_Update_RepoError(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()
	repoErr := errors.New("database error")

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ Witness) (Witness, error) {
			return Witness{}, repoErr
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, nil, logger)

	_, err := svc.Update(context.Background(), wID, UpdateWitnessInput{}, uuid.New().String())
	if err == nil {
		t.Fatal("expected error from repo.Update, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repoErr, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Service.Update — encrypts identity fields when supplied
// ---------------------------------------------------------------------------

func TestService_Update_WithIdentityFields(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, w Witness) (Witness, error) {
			return w, nil
		},
	}
	custody := &mockCustody{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, custody, logger)

	name := "Alice"
	contact := "alice@example.com"
	loc := "safe house B"
	summary := "Witness to the event"
	judgeVisible := true

	view, err := svc.Update(context.Background(), wID, UpdateWitnessInput{
		FullName:             &name,
		ContactInfo:          &contact,
		Location:             &loc,
		StatementSummary:     &summary,
		JudgeIdentityVisible: &judgeVisible,
	}, uuid.New().String())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !view.IdentityVisible {
		t.Error("view should have IdentityVisible=true after update with identity fields")
	}
	if view.FullName == nil || *view.FullName != name {
		t.Errorf("FullName: got %v, want %q", view.FullName, name)
	}
}

// ---------------------------------------------------------------------------
// Service.Update — FindByID returns ErrNotFound
// ---------------------------------------------------------------------------

func TestService_Update_NotFound(t *testing.T) {
	svc, _ := newTestService(&mockWitnessRepo{})
	_, err := svc.Update(context.Background(), uuid.New(), UpdateWitnessInput{}, uuid.New().String())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Service.Update — toDecryptedView fallback when result has corrupt ciphertext
// ---------------------------------------------------------------------------

func TestService_Update_ToDecryptedViewFallback(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()

	// The repo returns a witness with a corrupt full_name ciphertext so
	// toDecryptedView fails; the service should fall back to toView(result, false).
	corruptFullName := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, _ Witness) (Witness, error) {
			// Return a witness with corrupt encrypted data.
			return Witness{
				ID:                wID,
				CaseID:            uuid.New(),
				WitnessCode:       "W-CORRUPT",
				ProtectionStatus:  "standard",
				FullNameEncrypted: corruptFullName,
				RelatedEvidence:   []uuid.UUID{},
			}, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(repo, enc, nil, logger)

	view, err := svc.Update(context.Background(), wID, UpdateWitnessInput{}, uuid.New().String())
	if err != nil {
		t.Fatalf("Update should not return error on toDecryptedView fallback, got: %v", err)
	}
	if view.IdentityVisible {
		t.Error("fallback view should have IdentityVisible=false")
	}
}

// ---------------------------------------------------------------------------
// Service.Update — with RelatedEvidence field set
// ---------------------------------------------------------------------------

func TestService_Update_WithRelatedEvidence(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	wID := uuid.New()
	evidenceID := uuid.New()

	repo := &mockWitnessRepo{
		findByIDFn: func(_ context.Context, _ uuid.UUID) (Witness, error) {
			return Witness{ID: wID, CaseID: uuid.New(), ProtectionStatus: "standard", RelatedEvidence: []uuid.UUID{}}, nil
		},
		updateFn: func(_ context.Context, _ uuid.UUID, w Witness) (Witness, error) {
			return w, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewService(repo, enc, nil, logger)

	relEvidence := []uuid.UUID{evidenceID}
	view, err := svc.Update(context.Background(), wID, UpdateWitnessInput{
		RelatedEvidence: relEvidence,
	}, uuid.New().String())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(view.RelatedEvidence) != 1 || view.RelatedEvidence[0] != evidenceID {
		t.Errorf("RelatedEvidence: got %v, want [%v]", view.RelatedEvidence, evidenceID)
	}
}

// ---------------------------------------------------------------------------
// Service.recordCustodyEvent — custody recorder returns error (logged, not propagated)
// ---------------------------------------------------------------------------

type errorCustody struct{}

func (e *errorCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return errors.New("custody error")
}
func (e *errorCustody) RecordCaseEvent(_ context.Context, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return errors.New("custody error")
}

func TestService_RecordCustodyEvent_Error(t *testing.T) {
	enc, _ := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
	svc := NewService(&mockWitnessRepo{}, enc, &errorCustody{}, logger)
	// Should not panic or propagate the custody error.
	svc.recordCustodyEvent(context.Background(), uuid.New(), "test_action", "actor", map[string]string{"key": "val"})
}

// ---------------------------------------------------------------------------
// Service.Create — repository error
// ---------------------------------------------------------------------------

func TestService_Create_RepoError(t *testing.T) {
	repoErr := errors.New("insert failed")

	repo := &mockWitnessRepo{
		createFn: func(_ context.Context, _ Witness) (Witness, error) {
			return Witness{}, repoErr
		},
	}
	svc, _ := newTestService(repo)

	_, err := svc.Create(context.Background(), CreateWitnessInput{
		CaseID:           uuid.New(),
		WitnessCode:      "W-001",
		ProtectionStatus: "standard",
		CreatedBy:        uuid.New().String(),
	})
	if err == nil {
		t.Fatal("expected error from repo.Create, got nil")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repoErr, got: %v", err)
	}
}
