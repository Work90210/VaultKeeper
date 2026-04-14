package cases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// reqNoAuth builds a request with no auth context in the context at all.
func reqNoAuth(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// reqWithChiParam attaches chi URL params directly so we can call handler
// methods without going through the router.
func reqWithChiParam(method, path string, body any, params map[string]string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

// reqWithChiParamAndAuth is like reqWithChiParam but also sets auth context.
func reqWithChiParamAndAuth(method, path string, body any, params map[string]string) *http.Request {
	r := reqWithChiParam(method, path, body, params)
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "user-1",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

func setupHandlerTest(t *testing.T) (*Handler, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	custody := &mockCustody{}
	svc, err := NewService(repo, custody, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(svc, nil, nil, nil)
	return h, repo
}

func reqWithAuth(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "user-1",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

func TestHandler_Create(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         "Test Case",
		Jurisdiction:  "ICC",
	}

	req := reqWithAuth(http.MethodPost, "/api/cases", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatal("expected data in response")
	}
	if data["reference_code"] != "ICC-TST-2024" {
		t.Errorf("reference_code = %q", data["reference_code"])
	}
}

func TestHandler_Create_InvalidJSON(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/cases", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{
		UserID: "user-1", SystemRole: auth.RoleSystemAdmin,
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_Create_ValidationError(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	body := CreateCaseInput{
		ReferenceCode: "invalid",
		Title:         "Test",
	}

	req := reqWithAuth(http.MethodPost, "/api/cases", body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_Get(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusActive}

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodGet, "/api/cases/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodGet, "/api/cases/"+uuid.New().String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandler_Get_InvalidID(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodGet, "/api/cases/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_List(t *testing.T) {
	h, repo := setupHandlerTest(t)
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.cases[id] = Case{ID: id, Title: "Case", Status: StatusActive}
	}

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodGet, "/api/cases", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["meta"] == nil {
		t.Error("expected meta in paginated response")
	}
}

func TestHandler_Update(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	newTitle := "New Title"
	body := UpdateCaseInput{Title: &newTitle}
	req := reqWithAuth(http.MethodPatch, "/api/cases/"+id.String(), body)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Archive(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusClosed}

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodPost, "/api/cases/"+id.String()+"/archive", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_SetLegalHold(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodPost, "/api/cases/"+id.String()+"/legal-hold", map[string]bool{"hold": true})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Request without auth context
	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		query     string
		wantLimit int
	}{
		{"", 0},
		{"limit=25", 25},
		{"limit=abc", DefaultPageLimit},
		{"limit=300", 300}, // ClampPagination handles capping
	}

	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, "/api/cases?"+tt.query, nil)
		p := parsePagination(r)
		if p.Limit != tt.wantLimit {
			t.Errorf("query=%q: Limit = %d, want %d", tt.query, p.Limit, tt.wantLimit)
		}
	}
}

func TestRespondServiceError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"validation", &ValidationError{Field: "f", Message: "m"}, http.StatusBadRequest},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"duplicate", fmt.Errorf("reference code already exists: dup"), http.StatusConflict},
		{"internal", fmt.Errorf("db timeout"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			respondServiceError(rr, tt.err)
			if rr.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantCode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Auth-context guard (500) — call handler methods directly without auth ctx
// ---------------------------------------------------------------------------

func TestHandler_Create_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqNoAuth(http.MethodPost, "/api/cases", CreateCaseInput{
		ReferenceCode: "ICC-TST-2024",
		Title:         "Test",
	})
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_List_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqNoAuth(http.MethodGet, "/api/cases", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_Update_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqWithChiParam(http.MethodPatch, "/api/cases/"+uuid.New().String(), nil,
		map[string]string{"id": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_Archive_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqWithChiParam(http.MethodPost, "/api/cases/"+uuid.New().String()+"/archive", nil,
		map[string]string{"id": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.Archive(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_SetLegalHold_NoAuthContext(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqWithChiParam(http.MethodPost, "/api/cases/"+uuid.New().String()+"/legal-hold",
		map[string]bool{"hold": true},
		map[string]string{"id": uuid.New().String()})
	rr := httptest.NewRecorder()
	h.SetLegalHold(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Invalid UUID path param — Update / Archive / SetLegalHold
// ---------------------------------------------------------------------------

func TestHandler_Update_InvalidUUID(t *testing.T) {
	h, _ := setupHandlerTest(t)
	newTitle := "x"
	req := reqWithChiParamAndAuth(http.MethodPatch, "/api/cases/not-a-uuid",
		UpdateCaseInput{Title: &newTitle},
		map[string]string{"id": "not-a-uuid"})
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_Archive_InvalidUUID(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqWithChiParamAndAuth(http.MethodPost, "/api/cases/not-a-uuid/archive", nil,
		map[string]string{"id": "not-a-uuid"})
	rr := httptest.NewRecorder()
	h.Archive(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_SetLegalHold_InvalidUUID(t *testing.T) {
	h, _ := setupHandlerTest(t)
	req := reqWithChiParamAndAuth(http.MethodPost, "/api/cases/not-a-uuid/legal-hold",
		map[string]bool{"hold": true},
		map[string]string{"id": "not-a-uuid"})
	rr := httptest.NewRecorder()
	h.SetLegalHold(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Invalid JSON body — Update / SetLegalHold
// ---------------------------------------------------------------------------

func TestHandler_Update_InvalidJSON(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}

	req := reqWithChiParamAndAuth(http.MethodPatch, "/api/cases/"+id.String(), nil,
		map[string]string{"id": id.String()})
	// Overwrite body with garbage JSON after constructing the request.
	req.Body = io.NopCloser(strings.NewReader("not json"))

	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestHandler_SetLegalHold_InvalidJSON(t *testing.T) {
	h, repo := setupHandlerTest(t)
	id := uuid.New()
	repo.cases[id] = Case{ID: id, Status: StatusActive}

	req := reqWithChiParamAndAuth(http.MethodPost, "/api/cases/"+id.String()+"/legal-hold", nil,
		map[string]string{"id": id.String()})
	req.Body = io.NopCloser(strings.NewReader("not json"))

	rr := httptest.NewRecorder()
	h.SetLegalHold(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// decodeBody: body too large (exceeds MaxBodySize)
// ---------------------------------------------------------------------------

func TestDecodeBody_TooLarge(t *testing.T) {
	// Construct a JSON object whose value is larger than MaxBodySize.
	// We embed the oversized payload as the value of a valid JSON field so
	// that io.LimitReader truncates mid-stream, triggering the "too large"
	// branch via io.ErrUnexpectedEOF.
	oversize := `{"title":"` + strings.Repeat("a", MaxBodySize+10) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversize))
	req.Header.Set("Content-Type", "application/json")

	var dst UpdateCaseInput
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want 'too large'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// List: query parameter combinations
// ---------------------------------------------------------------------------

func TestHandler_List_QueryParams(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantCode int
	}{
		{
			name:     "status filter",
			query:    "status=active,closed",
			wantCode: http.StatusOK,
		},
		{
			name:     "search query",
			query:    "q=ukraine",
			wantCode: http.StatusOK,
		},
		{
			name:     "jurisdiction filter",
			query:    "jurisdiction=ICC",
			wantCode: http.StatusOK,
		},
		{
			name:     "cursor pagination",
			query:    "cursor=abc123&limit=10",
			wantCode: http.StatusOK,
		},
		{
			name:     "limit only",
			query:    "limit=5",
			wantCode: http.StatusOK,
		},
		{
			name:     "all params combined",
			query:    "status=active&q=test&jurisdiction=ICC&cursor=&limit=25",
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupHandlerTest(t)
			id := uuid.New()
			repo.cases[id] = Case{ID: id, Title: "Test", Status: StatusActive, Jurisdiction: "ICC"}

			r := chi.NewRouter()
			h.RegisterRoutes(r)

			req := reqWithAuth(http.MethodGet, "/api/cases?"+tt.query, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("query=%q: status = %d, want %d. Body: %s",
					tt.query, rr.Code, tt.wantCode, rr.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// decodeBody: More() path — two separate JSON values within the size limit
// ---------------------------------------------------------------------------

func TestDecodeBody_MoreData(t *testing.T) {
	// Send two valid JSON objects back-to-back. The first decodes into dst,
	// then decoder.More() is true and the second decode succeeds, which
	// triggers the "request body too large" path.
	body := `{}{}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst UpdateCaseInput
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for body with trailing data, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want 'too large'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Service-error paths — use an error-injecting repo wrapper
// ---------------------------------------------------------------------------

// errRepo wraps mockRepo and allows overriding individual methods with errors.
type errRepo struct {
	*mockRepo
	findAllErr      error
	updateErr       error
	archiveErr      error
	setLegalHoldErr error
}

func (e *errRepo) FindAll(ctx context.Context, f CaseFilter, p Pagination) ([]Case, int, error) {
	if e.findAllErr != nil {
		return nil, 0, e.findAllErr
	}
	return e.mockRepo.FindAll(ctx, f, p)
}

func (e *errRepo) Update(ctx context.Context, id uuid.UUID, updates UpdateCaseInput) (Case, error) {
	if e.updateErr != nil {
		return Case{}, e.updateErr
	}
	return e.mockRepo.Update(ctx, id, updates)
}

func (e *errRepo) Archive(ctx context.Context, id uuid.UUID) error {
	if e.archiveErr != nil {
		return e.archiveErr
	}
	return e.mockRepo.Archive(ctx, id)
}

func (e *errRepo) SetLegalHold(ctx context.Context, id uuid.UUID, hold bool) error {
	if e.setLegalHoldErr != nil {
		return e.setLegalHoldErr
	}
	return e.mockRepo.SetLegalHold(ctx, id, hold)
}

func setupHandlerWithErrRepo(t *testing.T, repo *errRepo) *Handler {
	t.Helper()
	custody := &mockCustody{}
	svc, err := NewService(repo, custody, `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(svc, nil, nil, nil)
}

func TestHandler_List_ServiceError(t *testing.T) {
	inner := newMockRepo()
	repo := &errRepo{mockRepo: inner, findAllErr: fmt.Errorf("db timeout")}
	h := setupHandlerWithErrRepo(t, repo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodGet, "/api/cases", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_Update_ServiceError(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	inner.cases[id] = Case{ID: id, Title: "Old", Status: StatusActive}
	repo := &errRepo{mockRepo: inner, updateErr: fmt.Errorf("db timeout")}
	h := setupHandlerWithErrRepo(t, repo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	newTitle := "New"
	req := reqWithAuth(http.MethodPatch, "/api/cases/"+id.String(), UpdateCaseInput{Title: &newTitle})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_Archive_ServiceError(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	inner.cases[id] = Case{ID: id, Status: StatusClosed}
	repo := &errRepo{mockRepo: inner, archiveErr: fmt.Errorf("db timeout")}
	h := setupHandlerWithErrRepo(t, repo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodPost, "/api/cases/"+id.String()+"/archive", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestHandler_SetLegalHold_ServiceError(t *testing.T) {
	inner := newMockRepo()
	id := uuid.New()
	// LegalHold=false so the service doesn't short-circuit on idempotent check.
	inner.cases[id] = Case{ID: id, Status: StatusActive, LegalHold: false}
	repo := &errRepo{mockRepo: inner, setLegalHoldErr: fmt.Errorf("db timeout")}
	h := setupHandlerWithErrRepo(t, repo)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := reqWithAuth(http.MethodPost, "/api/cases/"+id.String()+"/legal-hold",
		map[string]bool{"hold": true})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
