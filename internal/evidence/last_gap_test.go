package evidence

// Final coverage gaps: hooks into the test-injection vars added to
// pages_handler.go, draft_handler.go, and redaction.go to drive paths
// previously labeled "unreachable in tests".

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gen2brain/go-fitz"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// ---- redaction.go remaining hooks ----

// Force the doc.Image rasterize error branch in redactPDF.
func TestRedactPDF_DocImageError(t *testing.T) {
	orig := redactionPDFDocImage
	defer func() { redactionPDFDocImage = orig }()
	redactionPDFDocImage = func(_ *fitz.Document, _ int) (image.Image, error) {
		return nil, errors.New("injected raster failure")
	}
	_, err := redactPDF([]byte(minimalPDF),
		[]RedactionArea{{PageNumber: 1, X: 1, Y: 1, Width: 1, Height: 1, Reason: "x"}})
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- pages_handler.go remaining hooks ----

// runSync replaces pagesGoCache with a synchronous executor so the
// cache-write closure runs inline and its error branch becomes covered.
func runSync(t *testing.T) func() {
	t.Helper()
	orig := pagesGoCache
	pagesGoCache = func(fn func()) { fn() }
	return func() { pagesGoCache = orig }
}

func TestUnit_GetPageCount_CacheWriteError(t *testing.T) {
	defer runSync(t)()
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	// First GetObject returns the PDF, but PutObject fails (cache write).
	storage.putErr = errors.New("cache write failed")
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
}

func TestUnit_GetPage_CacheWriteError(t *testing.T) {
	defer runSync(t)()
	evidenceID := uuid.New()
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	storage.putErr = errors.New("cache write failed")
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
}

// pagesReadAll error injection for GetPageCount and GetPage.
func TestUnit_GetPageCount_ReadAllError(t *testing.T) {
	orig := pagesReadAll
	defer func() { pagesReadAll = orig }()
	pagesReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("injected read failure")
	}
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/page-count", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestUnit_GetPage_ReadAllError(t *testing.T) {
	orig := pagesReadAll
	defer func() { pagesReadAll = orig }()
	pagesReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("injected read failure")
	}
	key := "evidence/x/a.pdf"
	pool := poolForPages(uuid.New(), &key, nil)
	storage := newMockStorage()
	storage.objects[key] = []byte(minimalPDF)
	h := newUnitPagesHandler(t, pool, storage)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/pages/1", nil)
	req = withPagesAdminContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// renderPDFPageAsJPEG: inject ImageDPI error and JPEG-encode error.
func TestRenderPDFPageAsJPEG_ImageDPIError(t *testing.T) {
	orig := pagesDocImageDPI
	defer func() { pagesDocImageDPI = orig }()
	pagesDocImageDPI = func(_ *fitz.Document, _ int, _ float64) (image.Image, error) {
		return nil, errors.New("injected ImageDPI failure")
	}
	_, err := renderPDFPageAsJPEG([]byte(minimalPDF), 1, defaultPageDPI)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestRenderPDFPageAsJPEG_JPEGEncodeError(t *testing.T) {
	orig := pagesJPEGEncode
	defer func() { pagesJPEGEncode = orig }()
	pagesJPEGEncode = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected jpeg encode failure")
	}
	_, err := renderPDFPageAsJPEG([]byte(minimalPDF), 1, defaultPageDPI)
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- draft_handler.go SaveDraft marshal error ----

func TestSaveDraft_MarshalError(t *testing.T) {
	orig := draftHandlerJSONMarshal
	defer func() { draftHandlerJSONMarshal = orig }()
	draftHandlerJSONMarshal = func(_ any) ([]byte, error) {
		return nil, errors.New("injected marshal failure")
	}

	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"areas":[]}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- draft_handler.go FinalizeDraft handler success + error branches ----

// finalizeDraftHandlerSetup builds a DraftHandler whose db pool routes
// the lookupCaseID call AND seeds the underlying RedactionService's
// PGRepository transaction so FinalizeDraft can be exercised end-to-end.
//
// Two pools are needed because DraftHandler.db (used for the access check)
// and RedactionService.evidenceSvc.repo.pool (used by FinalizeFromDraft)
// are separate fields. We share a single mockDBPool whose queryRowFn
// branches by SQL prefix.
func finalizeDraftHandlerSetup(t *testing.T, finalizeInputErr bool) (*DraftHandler, uuid.UUID, uuid.UUID, *mockCustody) {
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
	draftStatus := "draft"
	if finalizeInputErr {
		// Cause a ValidationError later by passing wrong evidence id.
		draftStatus = "applied"
	}
	draft := RedactionDraft{
		ID:         draftID,
		EvidenceID: evidenceID,
		CaseID:     caseID,
		Name:       "review",
		Purpose:    "internal_review",
		Status:     draftStatus,
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	newEv := EvidenceItem{
		ID:             uuid.New(),
		CaseID:         caseID,
		Filename:       "redacted_file.pdf",
		Classification: ClassificationRestricted,
		Tags:           []string{"redacted"},
	}
	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs, newEvidence: newEv}

	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return tx, nil
		},
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT case_id FROM evidence_items") {
				// DraftHandler.lookupCaseID
				return &scanningRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = caseID
					return nil
				}}
			}
			if strings.Contains(sql, "SELECT EXISTS") {
				return &mockRow{scanFn: func(dest ...any) error {
					*(dest[0].(*bool)) = false
					return nil
				}}
			}
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}

	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)

	custody := &mockCustody{}
	svc := &Service{
		repo:    repo,
		storage: storage,
		tsa:     &integrity.NoopTimestampAuthority{},
		indexer: &search.NoopSearchIndexer{},
		custody: custody,
		cases:   &mockCaseLookup{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rs := NewRedactionService(svc, storage, &integrity.NoopTimestampAuthority{},
		custody, slog.New(slog.NewTextHandler(io.Discard, nil)))

	h := newDraftHandlerFromPool(pool,
		&stubRoleChecker{role: auth.CaseRoleInvestigator},
		custody,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	h.SetRedactionService(rs)

	return h, evidenceID, draftID, custody
}

func TestFinalizeDraft_Handler_Success(t *testing.T) {
	h, evidenceID, draftID, _ := finalizeDraftHandlerSetup(t, false)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft)
	})
	body := `{"description":"final","classification":"restricted"}`
	req := draftReq(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+draftID.String()+"/finalize",
		body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("code = %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestFinalizeDraft_Handler_ValidationError(t *testing.T) {
	h, evidenceID, draftID, _ := finalizeDraftHandlerSetup(t, true) // draft already applied → ValidationError
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft)
	})
	req := draftReq(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+draftID.String()+"/finalize",
		`{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFinalizeDraft_Handler_GenericError(t *testing.T) {
	// Build a setup where FinalizeFromDraft returns a non-ValidationError
	// (e.g., BeginTx failure inside the redaction service).
	caseID := uuid.New()
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return nil, errors.New("begin failed")
		},
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = caseID
				return nil
			}}
		},
	}
	repo := &PGRepository{pool: pool}
	svc := &Service{repo: repo, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rs := NewRedactionService(svc, newMockStorage(),
		&integrity.NoopTimestampAuthority{}, &mockCustody{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := newDraftHandlerFromPool(pool,
		&stubRoleChecker{role: auth.CaseRoleInvestigator},
		&mockCustody{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	h.SetRedactionService(rs)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft)
	})
	req := draftReq(http.MethodPost,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize",
		`{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- FinalizeFromDraft remaining branches ----

// finalizeHarnessForBranch builds a harness whose pool serves a Get
// (FindByID) call that can be tuned to fail.
type harnessOpts struct {
	getEvidenceErr  error
	storageGetErr   error
	storageDelErr   error
	storagePutErr   error
	tsaErr          error
	tsaReturnsToken bool
	useImageMime    bool
	imageData       []byte
}

func buildFinalizeHarnessOpts(t *testing.T, opts harnessOpts) (*RedactionService, uuid.UUID, uuid.UUID) {
	t.Helper()
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	mime := "application/pdf"
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"
	if opts.useImageMime {
		mime = "image/jpeg"
		storageKey = "evidence/" + evidenceID.String() + "/v1/original.jpg"
	}
	origNum := "CASE-1"
	original := EvidenceItem{
		ID:             evidenceID,
		CaseID:         caseID,
		EvidenceNumber: &origNum,
		Filename:       "file",
		OriginalName:   "file",
		StorageKey:     &storageKey,
		MimeType:       mime,
		SizeBytes:      int64(len(minimalPDF)),
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}
	draft := RedactionDraft{
		ID:         draftID,
		EvidenceID: evidenceID,
		CaseID:     caseID,
		Name:       "review",
		Purpose:    "internal_review",
		Status:     "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	newEv := EvidenceItem{ID: uuid.New(), CaseID: caseID, Tags: []string{"redacted"}}

	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs, newEvidence: newEv}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT EXISTS") {
				return &mockRow{scanFn: func(dest ...any) error {
					*(dest[0].(*bool)) = false
					return nil
				}}
			}
			if opts.getEvidenceErr != nil {
				return &mockRow{scanErr: opts.getEvidenceErr}
			}
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	if opts.useImageMime {
		storage.objects[storageKey] = opts.imageData
	} else {
		storage.objects[storageKey] = []byte(minimalPDF)
	}
	storage.getErr = opts.storageGetErr
	storage.deleteErr = opts.storageDelErr
	storage.putErr = opts.storagePutErr

	tsa := &flakyTSA{err: opts.tsaErr, returnToken: opts.tsaReturnsToken}

	svc := &Service{
		repo: repo, storage: storage,
		tsa: tsa, indexer: &search.NoopSearchIndexer{},
		custody: &mockCustody{},
		cases:   &mockCaseLookup{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	rs := NewRedactionService(svc, storage, tsa, &mockCustody{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return rs, evidenceID, draftID
}

// flakyTSA implements integrity.TimestampAuthority with controllable
// behaviour for FinalizeFromDraft TSA branches.
type flakyTSA struct {
	err         error
	returnToken bool
}

func (f *flakyTSA) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	if f.err != nil {
		return nil, "", time.Time{}, f.err
	}
	if f.returnToken {
		return []byte("token"), "tsa-name", time.Now(), nil
	}
	return nil, "", time.Time{}, nil
}

func (f *flakyTSA) VerifyTimestamp(_ context.Context, _ []byte, _ []byte) error {
	return nil
}

func TestFinalizeFromDraft_GetObjectError(t *testing.T) {
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{
		storageGetErr: errors.New("storage get failed"),
	})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_ReadAllError(t *testing.T) {
	orig := redactionReadAll
	defer func() { redactionReadAll = orig }()
	redactionReadAll = func(_ io.Reader) ([]byte, error) {
		return nil, errors.New("injected read failure")
	}
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_ImageBranch(t *testing.T) {
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{
		useImageMime: true,
		imageData:    createTestJPEG(50, 50),
	})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeFromDraft_RedactError(t *testing.T) {
	orig := redactionImageEncodeJPEG
	defer func() { redactionImageEncodeJPEG = orig }()
	redactionImageEncodeJPEG = func(_ io.Writer, _ image.Image, _ *jpeg.Options) error {
		return errors.New("injected jpeg failure")
	}
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{
		useImageMime: true,
		imageData:    createTestJPEG(50, 50),
	})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_TSAError(t *testing.T) {
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{
		tsaErr: errors.New("tsa down"),
	})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("TSA error should be soft-failed, got %v", err)
	}
}

func TestFinalizeFromDraft_TSAReturnsToken(t *testing.T) {
	rs, evID, draftID := buildFinalizeHarnessOpts(t, harnessOpts{
		tsaReturnsToken: true,
	})
	_, err := rs.FinalizeFromDraft(context.Background(), FinalizeInput{
		EvidenceID: evID, DraftID: draftID, ActorID: uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeFromDraft_GetEvidenceError(t *testing.T) {
	// Make the second QueryRow (rs.evidenceSvc.Get → FindByID) fail.
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			// Always fail FindByID — the lock used the tx so this is the
			// service.Get call.
			return &mockRow{scanErr: errors.New("evidence vanished")}
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
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_NumberCollisionExhausted(t *testing.T) {
	// CheckEvidenceNumberExists always returns true → GenerateRedactionNumber
	// loops past 100 attempts and returns the "too many collisions" error.
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"
	origNum := "CASE-1"
	original := EvidenceItem{
		ID: evidenceID, CaseID: caseID, EvidenceNumber: &origNum,
		Filename: "file.pdf", OriginalName: "file.pdf",
		StorageKey: &storageKey, MimeType: "application/pdf",
		SizeBytes: int64(len(minimalPDF)), Classification: ClassificationRestricted,
		Tags: []string{},
	}
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	newEv := EvidenceItem{ID: uuid.New(), CaseID: caseID, Tags: []string{"redacted"}}
	tx := &finalizeTx{t: t, draft: draft, yjsState: yjs, newEvidence: newEv}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT EXISTS") {
				return &mockRow{scanFn: func(dest ...any) error {
					*(dest[0].(*bool)) = true // collision every time
					return nil
				}}
			}
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)
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
	if err == nil {
		t.Fatal("want error")
	}
}

func TestFinalizeFromDraft_StorageDeleteError(t *testing.T) {
	// Trigger SetDerivativeParentWithTx failure → storageCleanup runs →
	// DeleteObject returns an error, exercising the cleanup-error log line.
	caseID := uuid.New()
	evidenceID := uuid.New()
	draftID := uuid.New()
	storageKey := "evidence/" + evidenceID.String() + "/v1/original.pdf"
	origNum := "CASE-1"
	original := EvidenceItem{
		ID: evidenceID, CaseID: caseID, EvidenceNumber: &origNum,
		Filename: "file.pdf", OriginalName: "file.pdf",
		StorageKey: &storageKey, MimeType: "application/pdf",
		SizeBytes: int64(len(minimalPDF)), Classification: ClassificationRestricted,
		Tags: []string{},
	}
	draft := RedactionDraft{
		ID: draftID, EvidenceID: evidenceID, CaseID: caseID,
		Name: "r", Purpose: "internal_review", Status: "draft",
	}
	yjs := []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":1,"w":2,"h":2,"reason":"r"}]}`)
	newEv := EvidenceItem{ID: uuid.New(), CaseID: caseID, Tags: []string{"redacted"}}
	tx := &finalizeTx{
		t: t, draft: draft, yjsState: yjs, newEvidence: newEv,
		setParentErr: errors.New("set parent failed"),
	}
	pool := &mockDBPool{
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return tx, nil },
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			if strings.Contains(sql, "SELECT EXISTS") {
				return &mockRow{scanFn: func(dest ...any) error {
					*(dest[0].(*bool)) = false
					return nil
				}}
			}
			return &mockRow{scanFn: evidenceScan(original)}
		},
	}
	repo := &PGRepository{pool: pool}
	storage := newMockStorage()
	storage.objects[storageKey] = []byte(minimalPDF)
	storage.deleteErr = errors.New("delete failed") // exercises cleanup error log
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
	if err == nil {
		t.Fatal("want error")
	}
}

// silence unused imports — these stay for potential future hooks.
var (
	_ = json.Marshal
	_ = sync.Once{}
	_ = time.Now
)
