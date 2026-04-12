package evidence

// Happy-path coverage tests for draft_handler.go CRUD flows:
// CreateDraft (access-fail branch), ListDrafts (error + access-fail
// branches), GetDraft, SaveDraft, DiscardDraft, FinalizeDraft,
// GetManagementView. Uses the existing draftPoolWithRow helper pattern
// augmented with an "ignore access check" variant and a repository-error
// injector.

import (
	"context"
	"errors"
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

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---- Shared builder ----

// flowPool returns a mockDBPool whose first QueryRow (lookupCaseID) fills
// the case UUID, and all subsequent QueryRow/Query/Exec calls are routed to
// the provided hooks. This lets a single test orchestrate the entire
// draft-CRUD flow through one pool.
type flowPoolOpts struct {
	caseID      uuid.UUID
	draftRow    func() pgx.Row                                      // CreateDraft / GetDraft / SaveDraft scan results
	listRows    func() (pgx.Rows, error)                            // ListDrafts / GetManagementView list
	execTag     func() (pgconn.CommandTag, error)                   // DiscardDraft / MarkDraftApplied exec
	beginTxFn   func(context.Context, pgx.TxOptions) (pgx.Tx, error) // GetManagementView BeginTx
	rowErr      error                                               // forced error from second+ QueryRow
}

func flowPool(o flowPoolOpts) *mockDBPool {
	calls := 0
	return &mockDBPool{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			calls++
			if calls == 1 && strings.Contains(sql, "SELECT case_id") {
				return &scanningRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = o.caseID
					return nil
				}}
			}
			if o.rowErr != nil {
				return &scanningRow{scanErr: o.rowErr}
			}
			if o.draftRow != nil {
				return o.draftRow()
			}
			return &scanningRow{}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			if o.listRows != nil {
				return o.listRows()
			}
			return &draftRowsMulti{}, nil
		},
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			if o.execTag != nil {
				return o.execTag()
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		beginTxFn: o.beginTxFn,
	}
}

func newFlowHandler(t *testing.T, pool dbPool) *DraftHandler {
	t.Helper()
	return newDraftHandlerFromPool(pool,
		&stubRoleChecker{role: auth.CaseRoleInvestigator},
		&mockCustody{},
		slog.New(slog.NewTextHandler(testingLogWriter{t}, nil)),
	)
}

// testingLogWriter redirects slog output to t.Log so failing tests don't
// spam the terminal with DB-error noise.
type testingLogWriter struct{ t *testing.T }

func (w testingLogWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

// ---- CreateDraft: access fail ----

func TestFlow_CreateDraft_AccessCheckFails(t *testing.T) {
	// lookupCaseID returns ErrNoRows → checkCaseAccessHTTP responds 404 and
	// returns !ok. The remaining return in CreateDraft was previously
	// uncovered.
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts",
		`{"name":"n","purpose":"internal_review"}`, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

// ---- ListDrafts: access fail + error branch ----

func TestFlow_ListDrafts_AccessCheckFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts", h.ListDrafts) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts", ``, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_ListDrafts_RepoError(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		listRows: func() (pgx.Rows, error) {
			return nil, errors.New("list failed")
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts", h.ListDrafts) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- GetDraft: full coverage ----

func TestFlow_GetDraft_InvalidEvidenceID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet, "/api/evidence/bad-uuid/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_GetDraft_InvalidDraftID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/not-a-uuid", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_GetDraft_AccessFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_GetDraft_NotFound(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		rowErr: pgx.ErrNoRows,
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_GetDraft_RepoError(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		rowErr: errors.New("lookup failed"),
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestFlow_GetDraft_Success(t *testing.T) {
	caseID := uuid.New()
	want := RedactionDraft{
		ID:         uuid.New(),
		EvidenceID: uuid.New(),
		CaseID:     caseID,
		Name:       "draft",
		Purpose:    "internal_review",
	}
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		draftRow: func() pgx.Row {
			return &scanningRow{scanFn: draftScan(want, []byte(`{"areas":[{"id":"a","page":1,"x":1,"y":2,"w":3,"h":4,"reason":"r","author":"u"}]}`), true)}
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+want.EvidenceID.String()+"/redact/drafts/"+want.ID.String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestFlow_GetDraft_SuccessEmptyState(t *testing.T) {
	// yjs_state empty → state.Areas stays nil → handler replaces with
	// []draftArea{}. Covers the `if state.Areas == nil` branch.
	caseID := uuid.New()
	want := RedactionDraft{ID: uuid.New(), EvidenceID: uuid.New(), CaseID: caseID}
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		draftRow: func() pgx.Row {
			return &scanningRow{scanFn: draftScan(want, nil, true)}
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+want.EvidenceID.String()+"/redact/drafts/"+want.ID.String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
}

func TestFlow_GetDraft_CorruptYjsState(t *testing.T) {
	caseID := uuid.New()
	want := RedactionDraft{ID: uuid.New(), EvidenceID: uuid.New(), CaseID: caseID}
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		draftRow: func() pgx.Row {
			return &scanningRow{scanFn: draftScan(want, []byte("not-json"), true)}
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet,
		"/api/evidence/"+want.EvidenceID.String()+"/redact/drafts/"+want.ID.String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- SaveDraft: full coverage ----

func TestFlow_SaveDraft_InvalidEvidenceID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/bad-uuid/redact/drafts/"+uuid.New().String(), `{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_InvalidDraftID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/bad-uuid", `{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_AccessFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), `{}`, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_SaveDraft_InvalidJSON(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), `{not-json`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_EmptyName(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"name":"  "}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_NameTooLong(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	body := `{"name":"` + strings.Repeat("x", 256) + `"}`
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_InvalidPurpose(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"purpose":"bogus"}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_TooManyAreas(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	// Build an areas array with 501 entries.
	var sb strings.Builder
	sb.WriteString(`{"areas":[`)
	for i := 0; i < 501; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"id":"x","page":1,"x":1,"y":1,"w":1,"h":1,"reason":"r","author":"u"}`)
	}
	sb.WriteString(`]}`)
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), sb.String(), true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_SaveDraft_Success(t *testing.T) {
	caseID := uuid.New()
	want := RedactionDraft{
		ID:          uuid.New(),
		EvidenceID:  uuid.New(),
		CaseID:      caseID,
		Name:        "updated",
		Purpose:     "internal_review",
		LastSavedAt: time.Now(),
	}
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		draftRow: func() pgx.Row {
			return &scanningRow{scanFn: draftScan(want, nil, false)}
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	body := `{"name":"updated","purpose":"internal_review","areas":[{"id":"a","page":1,"x":1,"y":2,"w":3,"h":4,"reason":"r","author":"u"}]}`
	req := draftReq(http.MethodPut,
		"/api/evidence/"+want.EvidenceID.String()+"/redact/drafts/"+want.ID.String(), body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestFlow_SaveDraft_AlreadyFinalized(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		rowErr: pgx.ErrNoRows,
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"areas":[]}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_SaveDraft_DuplicateKey(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		rowErr: &pgconn.PgError{Code: "23505"},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"name":"dup"}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("code = %d, want 409", w.Code)
	}
}

func TestFlow_SaveDraft_GenericDBError(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		rowErr: errors.New("db down"),
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := draftReq(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		`{"name":"x"}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- DiscardDraft: full coverage ----

func TestFlow_DiscardDraft_InvalidEvidenceID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/bad/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_DiscardDraft_InvalidDraftID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/bad", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_DiscardDraft_AccessFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_DiscardDraft_Success(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		execTag: func() (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestFlow_DiscardDraft_NotFound(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		execTag: func() (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_DiscardDraft_ExecError(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		execTag: func() (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec failed")
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := draftReq(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

// ---- FinalizeDraft: access-fail, bad IDs, decode error, validation error ----

func TestFlow_FinalizeDraft_InvalidEvidenceID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	h.SetRedactionService(&RedactionService{}) // non-nil so 503 branch doesn't fire
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := draftReq(http.MethodPost,
		"/api/evidence/bad/redact/drafts/"+uuid.New().String()+"/finalize", `{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_FinalizeDraft_InvalidDraftID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	h.SetRedactionService(&RedactionService{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := draftReq(http.MethodPost,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/bad/finalize", `{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_FinalizeDraft_AccessFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	h.SetRedactionService(&RedactionService{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := draftReq(http.MethodPost,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize", `{}`, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestFlow_FinalizeDraft_BadJSON(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{caseID: caseID})
	h := newFlowHandler(t, pool)
	h.SetRedactionService(&RedactionService{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := draftReq(http.MethodPost,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize",
		`{not-json`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// ---- GetManagementView: access-fail, repo error, success ----

func TestFlow_GetManagementView_InvalidEvidenceID(t *testing.T) {
	h := newFlowHandler(t, &mockDBPool{})
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redactions", h.GetManagementView) })
	req := draftReq(http.MethodGet, "/api/evidence/bad-uuid/redactions", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestFlow_GetManagementView_AccessFails(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redactions", h.GetManagementView) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redactions", ``, false)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

// managementTx is a minimal mockTx that supports GetManagementView's
// BeginTx → ListFinalizedRedactions → ListDrafts → Commit sequence.
type managementTx struct {
	mockTx
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (t *managementTx) Commit(_ context.Context) error   { return nil }
func (t *managementTx) Rollback(_ context.Context) error { return nil }

func TestFlow_GetManagementView_Success(t *testing.T) {
	caseID := uuid.New()
	// BeginTx returns a mockTx. Both ListFinalizedRedactions and ListDrafts
	// run against the pool (not the tx), so we route them through queryFn.
	callCount := 0
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = caseID
				return nil
			}}
		},
		queryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			callCount++
			if strings.Contains(sql, "redaction_name IS NOT NULL") {
				return &finalizedRows{items: nil}, nil
			}
			return &draftRowsMulti{}, nil
		},
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
	}
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redactions", h.GetManagementView) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redactions", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if callCount != 2 {
		t.Errorf("queryFn calls = %d, want 2", callCount)
	}
}

func TestFlow_GetManagementView_RepoError(t *testing.T) {
	caseID := uuid.New()
	pool := flowPool(flowPoolOpts{
		caseID: caseID,
		listRows: func() (pgx.Rows, error) {
			return nil, errors.New("list failed")
		},
		beginTxFn: func(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
			return &mockTx{}, nil
		},
	})
	h := newFlowHandler(t, pool)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redactions", h.GetManagementView) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redactions", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}
