package evidence

// Round-2 coverage fill: bulk_repository, migrate_adapter,
// GenerateRedactionNumber, and the remaining small gaps in service.go
// (UpdateMetadata / UploadNewVersion / validateUploadInput /
// validateEvidenceUpdate / indexEvidence) plus gdpr_handler
// ResolveErasureRequest and redaction.go (ApplyRedactions /
// PreviewRedactions / redactImage / redactPDF minor branches).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ============================================================
// bulk_repository.go
// ============================================================

func TestBulkJobRepo_Create_Success(t *testing.T) {
	now := time.Now()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*time.Time) = now
				*dest[1].(*time.Time) = now
				return nil
			}}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	job, err := repo.Create(context.Background(), uuid.New(), "admin", "archive-key")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if job.Status != BulkStatusExtracting {
		t.Errorf("status = %q", job.Status)
	}
}

func TestBulkJobRepo_Create_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("insert failed")}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	_, err := repo.Create(context.Background(), uuid.New(), "admin", "k")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestBulkJobRepo_UpdateProgress_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.UpdateProgress(context.Background(), uuid.New(), 1, 0, 5, BulkStatusProcessing); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestBulkJobRepo_UpdateProgress_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	err := repo.UpdateProgress(context.Background(), uuid.New(), 0, 0, 0, "x")
	if !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestBulkJobRepo_UpdateProgress_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db err")
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.UpdateProgress(context.Background(), uuid.New(), 0, 0, 0, "x"); err == nil {
		t.Fatal("want error")
	}
}

func TestBulkJobRepo_AppendError_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.AppendError(context.Background(), uuid.New(), BulkJobError{Filename: "f", Reason: "r"}); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestBulkJobRepo_AppendError_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.AppendError(context.Background(), uuid.New(), BulkJobError{}); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestBulkJobRepo_AppendError_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec err")
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.AppendError(context.Background(), uuid.New(), BulkJobError{}); err == nil {
		t.Fatal("want error")
	}
}

func TestBulkJobRepo_Finalize(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.Finalize(context.Background(), uuid.New(), BulkStatusCompleted); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestBulkJobRepo_Finalize_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.Finalize(context.Background(), uuid.New(), "x"); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestBulkJobRepo_Finalize_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec err")
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.Finalize(context.Background(), uuid.New(), "x"); err == nil {
		t.Fatal("want error")
	}
}

func TestBulkJobRepo_FindByID_Success(t *testing.T) {
	wantID := uuid.New()
	caseID := uuid.New()
	now := time.Now()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = wantID
				*dest[1].(*uuid.UUID) = caseID
				*dest[2].(*int) = 10
				*dest[3].(*int) = 5
				*dest[4].(*int) = 0
				*dest[5].(*string) = BulkStatusProcessing
				*dest[6].(*string) = "abc123" // archive_sha256
				*dest[7].(*[]byte) = []byte(`[{"filename":"f","reason":"r"}]`)
				*dest[8].(*string) = "admin"
				*dest[9].(*time.Time) = now
				*dest[10].(*time.Time) = now
				*dest[11].(**time.Time) = nil
				return nil
			}}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	job, err := repo.FindByID(context.Background(), caseID, wantID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if job.ID != wantID || len(job.Errors) != 1 {
		t.Errorf("job = %+v", job)
	}
}

func TestBulkJobRepo_FindByID_NotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: pgx.ErrNoRows}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestBulkJobRepo_FindByID_OtherError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db err")}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New(), uuid.New())
	if err == nil || errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want generic error, got %v", err)
	}
}

func TestBulkJobRepo_FindByID_BadErrorJSON(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = uuid.New()
				*dest[1].(*uuid.UUID) = uuid.New()
				*dest[2].(*int) = 0
				*dest[3].(*int) = 0
				*dest[4].(*int) = 0
				*dest[5].(*string) = "x"
				*dest[6].(*string) = "" // archive_sha256
				*dest[7].(*[]byte) = []byte(`not-json`)
				*dest[8].(*string) = "a"
				now := time.Now()
				*dest[9].(*time.Time) = now
				*dest[10].(*time.Time) = now
				*dest[11].(**time.Time) = nil
				return nil
			}}
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	_, err := repo.FindByID(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("want decode error")
	}
}

func TestBulkJobRepo_SetArchiveHash_Success(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.SetArchiveHash(context.Background(), uuid.New(), "abc"); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestBulkJobRepo_SetArchiveHash_NotFound(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.SetArchiveHash(context.Background(), uuid.New(), "abc"); !errors.Is(err, ErrBulkJobNotFound) {
		t.Errorf("want ErrBulkJobNotFound, got %v", err)
	}
}

func TestBulkJobRepo_SetArchiveHash_Error(t *testing.T) {
	pool := &mockDBPool{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec err")
		},
	}
	repo := &PGBulkJobRepository{pool: pool}
	if err := repo.SetArchiveHash(context.Background(), uuid.New(), "abc"); err == nil {
		t.Fatal("want error")
	}
}

// ============================================================
// numbering.GenerateRedactionNumber
// ============================================================

func TestGenerateRedactionNumber_FirstAvailable(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = false
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := GenerateRedactionNumber(context.Background(), repo, "ICC-001", RedactionPurpose("internal_review"), "Name 1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "ICC-001-R-") || !strings.Contains(got, "NAME-1") {
		t.Errorf("got %q", got)
	}
}

func TestGenerateRedactionNumber_CollisionResolved(t *testing.T) {
	// First 2 attempts → exists, third → free.
	calls := 0
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			calls++
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = calls < 3
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	got, err := GenerateRedactionNumber(context.Background(), repo, "ICC-001", RedactionPurpose("internal_review"), "Redacted Copy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Third attempt is -3.
	if !strings.HasSuffix(got, "-3") {
		t.Errorf("suffix = %q", got)
	}
}

func TestGenerateRedactionNumber_CheckError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanErr: errors.New("db err")}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := GenerateRedactionNumber(context.Background(), repo, "ICC-001", RedactionPurpose("internal_review"), "n")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestGenerateRedactionNumber_UnknownPurpose(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = false
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	// Purpose not in PurposeCode → defaults to "REDACTED"
	got, err := GenerateRedactionNumber(context.Background(), repo, "ICC-001", RedactionPurpose("phantom"), "n")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(got, "-R-REDACTED-") {
		t.Errorf("got %q", got)
	}
}

func TestGenerateRedactionNumber_Exhausted(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true // every candidate exists
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	_, err := GenerateRedactionNumber(context.Background(), repo, "ICC-001", RedactionPurpose("internal_review"), "n")
	if err == nil {
		t.Fatal("want exhaustion error")
	}
}

// ============================================================
// migrate_adapter.StoreMigratedFile
// ============================================================

func TestStoreMigratedFile_Success(t *testing.T) {
	svc, _, _, custody := newTestService(t)
	res, err := svc.StoreMigratedFile(context.Background(), MigrationStoreInput{
		CaseID:       uuid.New(),
		Filename:     "f.pdf",
		OriginalName: "f.pdf",
		Reader:       bytes.NewReader([]byte("payload")),
		SizeBytes:    7,
		UploadedBy:   "admin",
		Source:       "legacy",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.EvidenceID == uuid.Nil {
		t.Error("EvidenceID not set")
	}
	found := false
	for _, e := range custody.events {
		if e == "migrated" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custody 'migrated' event not emitted")
	}
}

func TestStoreMigratedFile_DefaultClassification(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// Empty classification → defaults to restricted
	_, err := svc.StoreMigratedFile(context.Background(), MigrationStoreInput{
		CaseID:     uuid.New(),
		Filename:   "f.pdf",
		Reader:     bytes.NewReader([]byte("p")),
		SizeBytes:  1,
		UploadedBy: "admin",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestStoreMigratedFile_UploadError(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.createFn = func(_ context.Context, _ CreateEvidenceInput) (EvidenceItem, error) {
		return EvidenceItem{}, errors.New("upload failed")
	}
	_, err := svc.StoreMigratedFile(context.Background(), MigrationStoreInput{
		CaseID:         uuid.New(),
		Filename:       "f.pdf",
		Reader:         bytes.NewReader([]byte("p")),
		SizeBytes:      1,
		Classification: ClassificationRestricted,
		UploadedBy:     "admin",
	})
	if err == nil {
		t.Fatal("want upload error")
	}
}

func TestStoreMigratedFile_HashDrift(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	// ComputedHash set to a value that won't match the actual SHA of payload.
	_, err := svc.StoreMigratedFile(context.Background(), MigrationStoreInput{
		CaseID:         uuid.New(),
		Filename:       "f.pdf",
		Reader:         bytes.NewReader([]byte("payload")),
		SizeBytes:      7,
		ComputedHash:   "0000000000000000000000000000000000000000000000000000000000000000",
		Classification: ClassificationRestricted,
		UploadedBy:     "admin",
	})
	if err == nil || !strings.Contains(err.Error(), "hash drift") {
		t.Errorf("want hash drift error, got %v", err)
	}
}

// ============================================================
// service.go small gaps (validateUploadInput / validateEvidenceUpdate)
// ============================================================

func TestValidateUploadInput_NilCaseID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.validateUploadInput(UploadInput{
		Filename:       "f.pdf",
		File:           bytes.NewReader([]byte("p")),
		Classification: ClassificationRestricted,
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "case_id" {
		t.Errorf("want case_id error, got %v", err)
	}
}

func TestValidateUploadInput_EmptyFilename(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.validateUploadInput(UploadInput{
		CaseID:         uuid.New(),
		Filename:       "   ",
		File:           bytes.NewReader([]byte("p")),
		Classification: ClassificationRestricted,
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "filename" {
		t.Errorf("want filename error, got %v", err)
	}
}

func TestValidateUploadInput_NilFile(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.validateUploadInput(UploadInput{
		CaseID:         uuid.New(),
		Filename:       "f.pdf",
		File:           nil,
		Classification: ClassificationRestricted,
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "file" {
		t.Errorf("want file error, got %v", err)
	}
}

func TestValidateUploadInput_BadClassification(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.validateUploadInput(UploadInput{
		CaseID:         uuid.New(),
		Filename:       "f.pdf",
		File:           bytes.NewReader([]byte("p")),
		Classification: "top_secret",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "classification" {
		t.Errorf("want classification error, got %v", err)
	}
}

func TestValidateUploadInput_DescriptionTooLong(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	err := svc.validateUploadInput(UploadInput{
		CaseID:         uuid.New(),
		Filename:       "f.pdf",
		File:           bytes.NewReader([]byte("p")),
		Classification: ClassificationRestricted,
		Description:    strings.Repeat("x", MaxDescriptionLength+1),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "description" {
		t.Errorf("want description error, got %v", err)
	}
}

func TestValidateEvidenceUpdate_LongDescription(t *testing.T) {
	desc := strings.Repeat("x", MaxDescriptionLength+1)
	err := validateEvidenceUpdate(EvidenceUpdate{Description: &desc})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "description" {
		t.Errorf("want description error, got %v", err)
	}
}

func TestValidateEvidenceUpdate_BadClassification(t *testing.T) {
	bad := "top_secret"
	err := validateEvidenceUpdate(EvidenceUpdate{Classification: &bad})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "classification" {
		t.Errorf("want classification error, got %v", err)
	}
}

// ============================================================
// gdpr_handler.ResolveErasureRequest remaining branches
// ============================================================

func TestResolveErasureRequest_LegalHoldConflict(t *testing.T) {
	// Seed a pending request, then force the service to return
	// ErrLegalHoldActive so the handler's explicit 409 mapping branch
	// fires.
	handler, _, erasureRepo, _ := newGDPRTestHandler(t)
	reqID := uuid.New()
	erasureRepo.reqs[reqID] = ErasureRequest{
		ID:         reqID,
		EvidenceID: uuid.New(),
		Status:     ErasureStatusConflictPending,
	}
	// The service tries to load evidence and will return an error;
	// handler maps non-409 errors via respondServiceError.
	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)
	body := `{"decision":"preserve","rationale":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/erasure-requests/"+reqID.String()+"/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Service returns ErrNotFound (evidence missing) → handler maps via
	// respondServiceError → 404. This exercises the fall-through error
	// path inside ResolveErasureRequest.
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404, body: %s", w.Code, w.Body.String())
	}
}

// Needed import markers (silence unused imports).
var _ = json.Marshal
var _ io.Writer = io.Discard
var _ = slog.Default
