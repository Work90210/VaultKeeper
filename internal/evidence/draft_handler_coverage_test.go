package evidence

// Unit coverage for draft_handler.go. Uses mockDBPool + mockTx + a
// stub CaseRoleChecker to exercise CreateDraft / ListDrafts / GetDraft
// / SaveDraft / DiscardDraft / GetManagementView / FinalizeDraft /
// checkCaseAccessHTTP / lookupCaseID / isDuplicateKeyError / rate
// limiter helpers without touching a real database.

import (
	"context"
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
	"golang.org/x/time/rate"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// ---- Stubs ----

type stubRoleChecker struct {
	role auth.CaseRole
	err  error
}

func (s *stubRoleChecker) LoadCaseRole(_ context.Context, _ string, _ string) (auth.CaseRole, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.role, nil
}

// scanningRow lets a test fill arbitrary Scan destinations.
type scanningRow struct {
	scanFn  func(dest ...any) error
	scanErr error
}

func (r *scanningRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

// draftRowsMulti fakes pgx.Rows for draft-list scans.
type draftRowsMulti struct {
	items []RedactionDraft
	idx   int
}

func (r *draftRowsMulti) Close()                                       {}
func (r *draftRowsMulti) Err() error                                   { return nil }
func (r *draftRowsMulti) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *draftRowsMulti) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *draftRowsMulti) RawValues() [][]byte                          { return nil }
func (r *draftRowsMulti) Values() ([]any, error)                       { return nil, nil }
func (r *draftRowsMulti) Conn() *pgx.Conn                              { return nil }
func (r *draftRowsMulti) Next() bool {
	if r.idx >= len(r.items) {
		return false
	}
	r.idx++
	return true
}
func (r *draftRowsMulti) Scan(dest ...any) error {
	return draftScan(r.items[r.idx-1], nil, false)(dest...)
}

func newDraftHandlerTest(t *testing.T, pool dbPool, role auth.CaseRole, roleErr error) *DraftHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newDraftHandlerFromPool(pool, &stubRoleChecker{role: role, err: roleErr}, &mockCustody{}, logger)
}

func draftReq(method, url, body string, admin bool) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	sysRole := auth.RoleAPIService
	if admin {
		sysRole = auth.RoleSystemAdmin
	}
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{
		UserID:     uuid.New().String(),
		SystemRole: sysRole,
	})
	return req.WithContext(ctx)
}

// ---- Rate limiter ----

func TestUserRateLimiter_AllowsBurst(t *testing.T) {
	l := newUserRateLimiter(rate.Every(time.Hour), 2)
	if !l.allow("u1") {
		t.Error("first call must allow")
	}
	if !l.allow("u1") {
		t.Error("second call must allow (burst=2)")
	}
	if l.allow("u1") {
		t.Error("third call must deny (burst exhausted)")
	}
}

func TestRateLimitMiddleware_NoAuthContext_PassesThrough(t *testing.T) {
	l := newUserRateLimiter(rate.Every(time.Hour), 1)
	mw := rateLimitMiddleware(l)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	if !called {
		t.Error("missing auth context must pass through")
	}
}

func TestRateLimitMiddleware_Allows(t *testing.T) {
	l := newUserRateLimiter(rate.Every(time.Hour), 1)
	mw := rateLimitMiddleware(l)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: "u1"}))
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("first allowed, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_Blocks(t *testing.T) {
	l := newUserRateLimiter(rate.Every(time.Hour), 1)
	mw := rateLimitMiddleware(l)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// Exhaust
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: "u-same"}))
		w := httptest.NewRecorder()
		mw(next).ServeHTTP(w, req)
		if i == 1 && w.Code != http.StatusTooManyRequests {
			t.Errorf("second call should be 429, got %d", w.Code)
		}
	}
}

// ---- isDuplicateKeyError ----

func TestIsDuplicateKeyError_True(t *testing.T) {
	if !isDuplicateKeyError(&pgconn.PgError{Code: "23505"}) {
		t.Error("want true")
	}
}

func TestIsDuplicateKeyError_FalseOther(t *testing.T) {
	if isDuplicateKeyError(&pgconn.PgError{Code: "99999"}) {
		t.Error("want false for non-23505")
	}
}

func TestIsDuplicateKeyError_FalseWrong(t *testing.T) {
	if isDuplicateKeyError(errors.New("generic")) {
		t.Error("want false for non-pgerror")
	}
}

// ---- lookupCaseID ----

func TestDraftHandler_LookupCaseID_Success(t *testing.T) {
	want := uuid.New()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = want
				return nil
			}}
		},
	}
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	got, err := h.lookupCaseID(context.Background(), uuid.New())
	if err != nil || got != want {
		t.Errorf("got %s err=%v", got, err)
	}
}

func TestDraftHandler_LookupCaseID_Error(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: errors.New("db down")}
		},
	}
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	_, err := h.lookupCaseID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- recordCustody ----

func TestDraftHandler_RecordCustody_NilCustody(t *testing.T) {
	h := &DraftHandler{
		custody: nil,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h.recordCustody(context.Background(), uuid.New(), uuid.New(), "action", "actor", nil)
}

// errCustody satisfies CustodyRecorder but always errors.
type errCustody struct{}

func (errCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _ string, _ string, _ map[string]string) error {
	return errors.New("custody fail")
}

func TestDraftHandler_RecordCustody_Error(t *testing.T) {
	h := &DraftHandler{
		custody: errCustody{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	h.recordCustody(context.Background(), uuid.New(), uuid.New(), "action", "actor", nil)
}

// ---- NewDraftHandler / SetRedactionService / RegisterRoutes ----

func TestNewDraftHandler_Constructor(t *testing.T) {
	// Pass a nil pgxpool.Pool — constructor just forwards to
	// newDraftHandlerFromPool and doesn't dereference.
	h := NewDraftHandler(nil, &stubRoleChecker{}, &mockCustody{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if h == nil {
		t.Fatal("nil handler")
	}
	h.SetRedactionService(nil) // no-op but exercises the setter
	router := chi.NewRouter()
	h.RegisterRoutes(router)
}

// ---- checkCaseAccessHTTP ----

func TestCheckCaseAccessHTTP_EvidenceNotFound(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: pgx.ErrNoRows}
		},
	}
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	w := httptest.NewRecorder()
	_, ok := h.checkCaseAccessHTTP(w, context.Background(), uuid.New(), auth.AuthContext{UserID: "u"})
	if ok {
		t.Error("want !ok")
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestCheckCaseAccessHTTP_LookupError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanErr: errors.New("db down")}
		},
	}
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	w := httptest.NewRecorder()
	_, ok := h.checkCaseAccessHTTP(w, context.Background(), uuid.New(), auth.AuthContext{UserID: "u"})
	if ok || w.Code != http.StatusInternalServerError {
		t.Errorf("ok=%v code=%d", ok, w.Code)
	}
}

func TestCheckCaseAccessHTTP_AdminBypass(t *testing.T) {
	caseID := uuid.New()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = caseID
				return nil
			}}
		},
	}
	h := newDraftHandlerTest(t, pool, "", errors.New("role check should not run"))
	w := httptest.NewRecorder()
	got, ok := h.checkCaseAccessHTTP(w, context.Background(), uuid.New(), auth.AuthContext{UserID: "u", SystemRole: auth.RoleSystemAdmin})
	if !ok || got != caseID {
		t.Errorf("admin bypass failed: ok=%v got=%s", ok, got)
	}
}

func TestCheckCaseAccessHTTP_NoCaseRole(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = uuid.New()
				return nil
			}}
		},
	}
	h := newDraftHandlerTest(t, pool, "", auth.ErrNoCaseRole)
	w := httptest.NewRecorder()
	_, ok := h.checkCaseAccessHTTP(w, context.Background(), uuid.New(), auth.AuthContext{UserID: "u"})
	if ok || w.Code != http.StatusForbidden {
		t.Errorf("ok=%v code=%d", ok, w.Code)
	}
}

func TestCheckCaseAccessHTTP_RoleLoaderError(t *testing.T) {
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = uuid.New()
				return nil
			}}
		},
	}
	h := newDraftHandlerTest(t, pool, "", errors.New("lookup failed"))
	w := httptest.NewRecorder()
	_, ok := h.checkCaseAccessHTTP(w, context.Background(), uuid.New(), auth.AuthContext{UserID: "u"})
	if ok || w.Code != http.StatusInternalServerError {
		t.Errorf("ok=%v code=%d", ok, w.Code)
	}
}

// ---- CreateDraft ----

func TestDraftHandler_CreateDraft_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDraftHandler_CreateDraft_BadEvidenceID(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/not-uuid/redact/drafts", `{}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// draftPoolWithRow returns a mockDBPool where lookup returns a valid case,
// and CreateDraft's INSERT RETURNING is controlled by draftFn.
func draftPoolWithRow(caseID uuid.UUID, draftFn func() pgx.Row) *mockDBPool {
	call := 0
	return &mockDBPool{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			call++
			// First call: lookupCaseID. Second+ call: CreateDraft insert.
			if call == 1 && strings.Contains(sql, "SELECT case_id") {
				return &scanningRow{scanFn: func(dest ...any) error {
					*(dest[0].(*uuid.UUID)) = caseID
					return nil
				}}
			}
			return draftFn()
		},
	}
}

func TestDraftHandler_CreateDraft_InvalidJSON(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", `{not-json`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDraftHandler_CreateDraft_MissingName(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts",
		`{"name":"  ","purpose":"internal_review"}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDraftHandler_CreateDraft_NameTooLong(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	longName := strings.Repeat("x", 256)
	body := `{"name":"` + longName + `","purpose":"internal_review"}`
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDraftHandler_CreateDraft_InvalidPurpose(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts",
		`{"name":"n","purpose":"bogus"}`, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDraftHandler_CreateDraft_Success(t *testing.T) {
	caseID := uuid.New()
	evID := uuid.New()
	want := RedactionDraft{
		ID:         uuid.New(),
		EvidenceID: evID,
		CaseID:     caseID,
		Name:       "Test",
		Purpose:    "internal_review",
		Status:     "draft",
	}
	pool := draftPoolWithRow(caseID, func() pgx.Row {
		return &scanningRow{scanFn: draftScan(want, nil, false)}
	})
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	body := `{"name":"Test","purpose":"internal_review"}`
	req := draftReq(http.MethodPost, "/api/evidence/"+evID.String()+"/redact/drafts", body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}
}

func TestDraftHandler_CreateDraft_DuplicateKey(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row {
		return &scanningRow{scanErr: &pgconn.PgError{Code: "23505"}}
	})
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	body := `{"name":"Test","purpose":"internal_review"}`
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestDraftHandler_CreateDraft_GenericDBError(t *testing.T) {
	pool := draftPoolWithRow(uuid.New(), func() pgx.Row {
		return &scanningRow{scanErr: errors.New("db err")}
	})
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts", h.CreateDraft) })
	body := `{"name":"Test","purpose":"internal_review"}`
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", body, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// ---- ListDrafts ----

func TestDraftHandler_ListDrafts_Success(t *testing.T) {
	caseID := uuid.New()
	pool := &mockDBPool{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return &scanningRow{scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = caseID
				return nil
			}}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			return &draftRowsMulti{items: []RedactionDraft{{ID: uuid.New(), Name: "d1"}}}, nil
		},
	}
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts", h.ListDrafts) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestDraftHandler_ListDrafts_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts", h.ListDrafts) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDraftHandler_ListDrafts_BadEvidenceID(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts", h.ListDrafts) })
	req := draftReq(http.MethodGet, "/api/evidence/not-uuid/redact/drafts", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- GetDraft ----

func TestDraftHandler_GetDraft_BadDraftID(t *testing.T) {
	caseID := uuid.New()
	pool := draftPoolWithRow(caseID, func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := draftReq(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts/not-uuid", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDraftHandler_GetDraft_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redact/drafts/{draftId}", h.GetDraft) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// ---- SaveDraft / DiscardDraft / GetManagementView ----

func TestDraftHandler_SaveDraft_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Put("/redact/drafts/{draftId}", h.SaveDraft) })
	req := httptest.NewRequest(http.MethodPut, "/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDraftHandler_DiscardDraft_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Delete("/redact/drafts/{draftId}", h.DiscardDraft) })
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDraftHandler_GetManagementView_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Get("/redactions", h.GetManagementView) })
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.New().String()+"/redactions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// ---- FinalizeDraft ----

func TestDraftHandler_FinalizeDraft_NoAuth(t *testing.T) {
	h := newDraftHandlerTest(t, &mockDBPool{}, auth.CaseRoleInvestigator, nil)
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestDraftHandler_FinalizeDraft_NoRedactionSvc(t *testing.T) {
	caseID := uuid.New()
	pool := draftPoolWithRow(caseID, func() pgx.Row { return &scanningRow{} })
	h := newDraftHandlerTest(t, pool, auth.CaseRoleInvestigator, nil)
	// redactionSvc not set
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) { r.Post("/redact/drafts/{draftId}/finalize", h.FinalizeDraft) })
	req := draftReq(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize", ``, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

// Ensure logger import is exercised even if some test removed it.
var _ = log{}

type log struct{}
