package evidence

// Unit coverage for RedactionService.FinalizeFromDraft and the
// previously-unreachable error branches in redactImage / redactPDF /
// ApplyRedactions / PreviewRedactions. Uses test-hook variables added to
// redaction.go + a scripted mockDBPool/mockTx to drive the transactional
// happy path without a real Postgres instance.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// ---- Test hook coverage for the "unreachable" branches ----

func TestRedactImage_EncodeJPEGError(t *testing.T) {
	orig := redactionImageEncodeJPEG
	defer func() { redactionImageEncodeJPEG = orig }()
	redactionImageEncodeJPEG = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected jpeg encode failure")
	}
	_, err := redactImage(createTestJPEG(10, 10), "image/jpeg",
		[]RedactionArea{{X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestRedactImage_EncodePNGError(t *testing.T) {
	orig := redactionImageEncodePNG
	defer func() { redactionImageEncodePNG = orig }()
	redactionImageEncodePNG = func(_ io.Writer, _ image.Image) error {
		return errors.New("injected png encode failure")
	}
	_, err := redactImage(createTestPNG(10, 10), "image/png",
		[]RedactionArea{{X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestRedactPDF_PageEncodeError(t *testing.T) {
	orig := redactionPDFPageEncode
	defer func() { redactionPDFPageEncode = orig }()
	redactionPDFPageEncode = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected page encode failure")
	}
	_, err := redactPDF([]byte(minimalPDF),
		[]RedactionArea{{PageNumber: 1, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestRedactPDF_ImportImagesError(t *testing.T) {
	orig := redactionPDFImportImages
	defer func() { redactionPDFImportImages = orig }()
	redactionPDFImportImages = func(_ io.ReadSeeker, _ io.Writer, _ []io.Reader, _ *pdfcpu.Import, _ *pdfcpumodel.Configuration) error {
		return errors.New("injected import failure")
	}
	_, err := redactPDF([]byte(minimalPDF),
		[]RedactionArea{{PageNumber: 1, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- ReadAll error injection ----

func TestApplyRedactions_ReadAllError(t *testing.T) {
	orig := redactionReadAll
	defer func() { redactionReadAll = orig }()
	redactionReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("injected read failure")
	}

	svc, repo, storage, custody := newTestService(t)
	_ = custody
	caseID := uuid.New()
	itemID := uuid.New()
	storageKey := "e/" + itemID.String() + "/a.pdf"
	repo.items[itemID] = EvidenceItem{
		ID: itemID, CaseID: caseID, StorageKey: &storageKey,
		MimeType: "application/pdf", Classification: ClassificationRestricted,
	}
	storage.objects[storageKey] = []byte(minimalPDF)

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := rs.ApplyRedactions(context.Background(), itemID,
		[]RedactionArea{{PageNumber: 1, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}}, "actor")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestPreviewRedactions_ReadAllError(t *testing.T) {
	orig := redactionReadAll
	defer func() { redactionReadAll = orig }()
	redactionReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("injected read failure")
	}

	svc, repo, storage, _ := newTestService(t)
	caseID := uuid.New()
	itemID := uuid.New()
	storageKey := "e/" + itemID.String() + "/a.pdf"
	repo.items[itemID] = EvidenceItem{
		ID: itemID, CaseID: caseID, StorageKey: &storageKey,
		MimeType: "application/pdf", Classification: ClassificationRestricted,
	}
	storage.objects[storageKey] = []byte(minimalPDF)

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, _, err := rs.PreviewRedactions(context.Background(), itemID,
		[]RedactionArea{{PageNumber: 1, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

// redactImage error via ApplyRedactions (encode failure surfaces as apply error)
func TestApplyRedactions_EncodeError(t *testing.T) {
	orig := redactionImageEncodeJPEG
	defer func() { redactionImageEncodeJPEG = orig }()
	redactionImageEncodeJPEG = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected jpeg failure")
	}

	svc, repo, storage, _ := newTestService(t)
	caseID := uuid.New()
	itemID := uuid.New()
	storageKey := "e/" + itemID.String() + "/a.jpg"
	repo.items[itemID] = EvidenceItem{
		ID: itemID, CaseID: caseID, StorageKey: &storageKey,
		MimeType: "image/jpeg", Classification: ClassificationRestricted,
	}
	storage.objects[storageKey] = createTestJPEG(50, 50)

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := rs.ApplyRedactions(context.Background(), itemID,
		[]RedactionArea{{PageNumber: 0, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}}, "actor")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestPreviewRedactions_EncodeError(t *testing.T) {
	orig := redactionImageEncodeJPEG
	defer func() { redactionImageEncodeJPEG = orig }()
	redactionImageEncodeJPEG = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected jpeg failure")
	}

	svc, repo, storage, _ := newTestService(t)
	caseID := uuid.New()
	itemID := uuid.New()
	storageKey := "e/" + itemID.String() + "/a.jpg"
	repo.items[itemID] = EvidenceItem{
		ID: itemID, CaseID: caseID, StorageKey: &storageKey,
		MimeType: "image/jpeg", Classification: ClassificationRestricted,
	}
	storage.objects[storageKey] = createTestJPEG(50, 50)

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, _, err := rs.PreviewRedactions(context.Background(), itemID,
		[]RedactionArea{{PageNumber: 0, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- FinalizeFromDraft happy path + failure modes ----

// finalizeTx is a scripted mockTx that tracks call count so each
// QueryRow/Exec invocation can return a different result.
type finalizeTx struct {
	t              *testing.T
	draft          RedactionDraft
	yjsState       []byte
	newEvidence    EvidenceItem
	queryRowCount  int
	execCount      int
	commitErr      error
	lockErr        error
	createErr      error
	setParentErr   error
	markAppliedErr error
	rollbackCount  int
}

func (t *finalizeTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (t *finalizeTx) Commit(_ context.Context) error          { return t.commitErr }
func (t *finalizeTx) Rollback(_ context.Context) error        { t.rollbackCount++; return nil }
func (t *finalizeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *finalizeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *finalizeTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (t *finalizeTx) Prepare(_ context.Context, _ string, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *finalizeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *finalizeTx) Conn() *pgx.Conn { return nil }

func (t *finalizeTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	t.queryRowCount++
	if strings.Contains(sql, "redaction_drafts WHERE id") {
		// LockDraftForFinalize
		if t.lockErr != nil {
			return &mockRow{scanErr: t.lockErr}
		}
		return &mockRow{scanFn: draftScan(t.draft, t.yjsState, true)}
	}
	if strings.Contains(sql, "INSERT INTO evidence_items") {
		// CreateWithTx
		if t.createErr != nil {
			return &mockRow{scanErr: t.createErr}
		}
		return &mockRow{scanFn: evidenceScan(t.newEvidence)}
	}
	t.t.Errorf("unexpected tx.QueryRow sql: %s", sql)
	return &mockRow{scanErr: errors.New("unexpected")}
}

func (t *finalizeTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execCount++
	if strings.Contains(sql, "parent_id") {
		if t.setParentErr != nil {
			return pgconn.CommandTag{}, t.setParentErr
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	if strings.Contains(sql, "status = 'applied'") {
		if t.markAppliedErr != nil {
			return pgconn.CommandTag{}, t.markAppliedErr
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag(""), nil
}

// evidenceScan fills the full set of evidence columns scanned by
// scanEvidence in repository.go. The destination order matches the
// scanEvidence call exactly.
func evidenceScan(e EvidenceItem) func(dest ...any) error {
	return func(dest ...any) error {
		*(dest[0].(*uuid.UUID)) = e.ID
		*(dest[1].(*uuid.UUID)) = e.CaseID
		*(dest[2].(**string)) = e.EvidenceNumber
		*(dest[3].(*string)) = e.Filename
		*(dest[4].(*string)) = e.OriginalName
		*(dest[5].(**string)) = e.StorageKey
		*(dest[6].(**string)) = e.ThumbnailKey
		*(dest[7].(*string)) = e.MimeType
		*(dest[8].(*int64)) = e.SizeBytes
		*(dest[9].(*string)) = e.SHA256Hash
		*(dest[10].(*string)) = e.Classification
		*(dest[11].(*string)) = e.Description
		*(dest[12].(*[]string)) = e.Tags
		*(dest[13].(*string)) = e.UploadedBy
		*(dest[14].(*string)) = e.UploadedByName
		*(dest[15].(*bool)) = e.IsCurrent
		*(dest[16].(*int)) = e.Version
		*(dest[17].(**uuid.UUID)) = e.ParentID
		*(dest[18].(*[]byte)) = e.TSAToken
		*(dest[19].(**string)) = e.TSAName
		*(dest[20].(**time.Time)) = e.TSATimestamp
		*(dest[21].(*string)) = e.TSAStatus
		*(dest[22].(*int)) = e.TSARetryCount
		*(dest[23].(**time.Time)) = e.TSALastRetry
		*(dest[24].(*[]byte)) = e.ExifData
		*(dest[25].(*string)) = e.Source
		*(dest[26].(**time.Time)) = e.SourceDate
		*(dest[27].(**string)) = e.ExParteSide
		*(dest[28].(**time.Time)) = e.DestroyedAt
		*(dest[29].(**string)) = e.DestroyedBy
		*(dest[30].(**string)) = e.DestroyReason
		*(dest[31].(*time.Time)) = e.CreatedAt
		*(dest[32].(**string)) = e.RedactionName
		*(dest[33].(**RedactionPurpose)) = e.RedactionPurpose
		*(dest[34].(**int)) = e.RedactionAreaCount
		*(dest[35].(**uuid.UUID)) = e.RedactionAuthorID
		*(dest[36].(**time.Time)) = e.RedactionFinalizedAt
		*(dest[37].(**time.Time)) = e.RetentionUntil
		*(dest[38].(**string)) = e.DestructionAuthority
		return nil
	}
}

func buildFinalizeHarness(t *testing.T) (*RedactionService, *finalizeTx, *mockStorage, uuid.UUID, uuid.UUID) {
	t.Helper()
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"

	origNum := "CASE-1"
	original := EvidenceItem{
		ID:             evidenceID,
		CaseID:         caseID,
		EvidenceNumber: &origNum,
		Filename:       "file.pdf",
		OriginalName:   "file.pdf",
		StorageKey:     &storageKey,
		MimeType:       "application/pdf",
		SizeBytes:      int64(len(minimalPDF)),
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}
	newEv := EvidenceItem{
		ID:             uuid.New(),
		CaseID:         caseID,
		Filename:       "redacted_file.pdf",
		OriginalName:   "file.pdf",
		Classification: ClassificationRestricted,
		Tags:           []string{"redacted"},
	}
	draft := RedactionDraft{
		ID:         draftID,
		EvidenceID: evidenceID,
		CaseID:     caseID,
		Name:       "review",
		Purpose:    "internal_review",
		Status:     "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"pii"}]}`)

	tx := &finalizeTx{
		t:           t,
		draft:       draft,
		yjsState:    yjs,
		newEvidence: newEv,
	}

	// The pool serves:
	// - service.Get → FindByID: scan original evidence
	// - GenerateRedactionNumber → CheckEvidenceNumberExists: returns false
	queryRowCount := 0
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return tx, nil
		},
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			queryRowCount++
			if strings.Contains(sql, "SELECT EXISTS") {
				// CheckEvidenceNumberExists
				return &mockRow{scanFn: func(dest ...any) error {
					*(dest[0].(*bool)) = false
					return nil
				}}
			}
			// FindByID
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}

	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)

	svc := &Service{
		repo:    repo,
		storage: storage,
		tsa:     &integrity.NoopTimestampAuthority{},
		indexer: &search.NoopSearchIndexer{},
		custody: &mockCustody{},
		cases:   &mockCaseLookup{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	return rs, tx, storage, evidenceID, draftID
}

func TestFinalizeFromDraft_HappyPath(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	actorID := uuid.New().String()

	result, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID:  evidenceID,
		DraftID:     draftID,
		Description: "finalized",
		ActorID:     actorID,
		ActorName:   "Test Actor",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RedactionCount != 1 {
		t.Errorf("redaction count = %d, want 1", result.RedactionCount)
	}
	if tx.execCount < 2 {
		t.Errorf("exec count = %d, want >= 2", tx.execCount)
	}
}

func TestFinalizeFromDraft_NonPGRepository_Gap(t *testing.T) {
	// rs.evidenceSvc.repo is a mockRepo, not *PGRepository.
	svc, _, storage, _ := newTestService(t)
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: uuid.New(), DraftID: uuid.New(), ActorID: uuid.New().String(),
	})
	if err == nil || !strings.Contains(err.Error(), "PGRepository") {
		t.Fatalf("want PGRepository error, got %v", err)
	}
}

func TestFinalizeFromDraft_BeginTxError(t *testing.T) {
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return nil, errors.New("begin failed")
		},
	}
	repo := &PGRepository{pool: pool}
	svc := &Service{repo: repo, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rs := NewRedactionService(svc, newMockStorage(),
		&integrity.NoopTimestampAuthority{}, &mockCustody{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: uuid.New(), DraftID: uuid.New(), ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_LockError(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.lockErr = errors.New("lock failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_AlreadyApplied(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.draft.Status = "applied"
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestFinalizeFromDraft_WrongEvidence(t *testing.T) {
	rs, _, _, _, draftID := buildFinalizeHarness(t)
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: uuid.New(), // does not match tx.draft.EvidenceID
		DraftID:    draftID,
		ActorID:    uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestFinalizeFromDraft_CorruptYjsState(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.yjsState = []byte("not-json")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("want unmarshal error, got %v", err)
	}
}

func TestFinalizeFromDraft_EmptyAreas(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.yjsState = []byte(`{"areas":[]}`)
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestFinalizeFromDraft_InvalidActorID(t *testing.T) {
	rs, _, _, evidenceID, draftID := buildFinalizeHarness(t)
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: "not-a-uuid",
	})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "actor_id" {
		t.Fatalf("want actor_id error, got %v", err)
	}
}

func TestFinalizeFromDraft_SetParentError(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.setParentErr = errors.New("set parent failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_MarkAppliedError(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.markAppliedErr = errors.New("mark applied failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_CommitError(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.commitErr = errors.New("commit failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_CreateWithTxError(t *testing.T) {
	rs, tx, _, evidenceID, draftID := buildFinalizeHarness(t)
	tx.createErr = errors.New("create failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_PutObjectError(t *testing.T) {
	rs, _, storage, evidenceID, draftID := buildFinalizeHarness(t)
	storage.putErr = errors.New("put failed")
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_OriginalDestroyed(t *testing.T) {
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"
	destroyedAt := time.Now()
	origNum := "CASE-1"
	original := EvidenceItem{
		ID: evidenceID, CaseID: caseID, EvidenceNumber: &origNum,
		Filename: "file.pdf", OriginalName: "file.pdf",
		StorageKey: &storageKey, MimeType: "application/pdf",
		SizeBytes: 10, Classification: ClassificationRestricted,
		Tags: []string{}, DestroyedAt: &destroyedAt,
	}
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)

	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return tx, nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	svc := &Service{
		repo: repo, storage: storage,
		tsa:    &integrity.NoopTimestampAuthority{},
		custody: &mockCustody{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError for destroyed evidence, got %v", err)
	}
}

func TestFinalizeFromDraft_FileTooLarge(t *testing.T) {
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"
	origNum := "CASE-1"
	original := EvidenceItem{
		ID: evidenceID, CaseID: caseID, EvidenceNumber: &origNum,
		Filename: "file.pdf", OriginalName: "file.pdf",
		StorageKey: &storageKey, MimeType: "application/pdf",
		SizeBytes: maxRedactionSize + 1, Classification: ClassificationRestricted,
		Tags: []string{},
	}
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return tx, nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	svc := &Service{
		repo: repo, storage: newMockStorage(),
		tsa: &integrity.NoopTimestampAuthority{}, custody: &mockCustody{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rs := NewRedactionService(svc, newMockStorage(), &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestFinalizeFromDraft_UnsupportedMime(t *testing.T) {
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.bin"
	origNum := "CASE-1"
	original := EvidenceItem{
		ID: evidenceID, CaseID: caseID, EvidenceNumber: &origNum,
		Filename: "file.bin", OriginalName: "file.bin",
		StorageKey: &storageKey, MimeType: "application/octet-stream",
		SizeBytes: 10, Classification: ClassificationRestricted,
		Tags: []string{},
	}
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs}
	storage := newMockStorage()
	storage.objects[storageKey] = []byte("random bytes")
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return tx, nil
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	svc := &Service{
		repo: repo, storage: storage,
		tsa: &integrity.NoopTimestampAuthority{}, custody: &mockCustody{},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		&mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evidenceID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

// Silence unused imports.
var _ = bytes.NewReader
