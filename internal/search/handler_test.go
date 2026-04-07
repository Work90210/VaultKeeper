package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// --- mock implementations ---

type mockEvidenceSearcher struct {
	result EvidenceSearchResult
	err    error
	query  SearchQuery // captured from last call
}

func (m *mockEvidenceSearcher) IndexEvidence(_ context.Context, _ EvidenceSearchDoc) error {
	return nil
}

func (m *mockEvidenceSearcher) RemoveEvidence(_ context.Context, _ string) error {
	return nil
}

func (m *mockEvidenceSearcher) SearchEvidence(_ context.Context, q SearchQuery) (EvidenceSearchResult, error) {
	m.query = q
	return m.result, m.err
}

func (m *mockEvidenceSearcher) ConfigureEvidenceIndex(_ context.Context) error {
	return nil
}

func (m *mockEvidenceSearcher) ReindexAll(_ context.Context, _ string, _ []EvidenceSearchDoc) error {
	return nil
}

type mockCaseIDsLoader struct {
	caseIDs []string
	err     error
}

func (m *mockCaseIDsLoader) GetUserCaseIDs(_ context.Context, _ string) ([]string, error) {
	return m.caseIDs, m.err
}

type mockCaseRolesLoader struct {
	roles map[string]string
	err   error
}

func (m *mockCaseRolesLoader) GetUserCaseRoles(_ context.Context, _ string) (map[string]string, error) {
	return m.roles, m.err
}

type mockAuditLogger struct{}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

// --- helpers ---

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *string         `json:"error"`
}

func newTestHandler(searcher *mockEvidenceSearcher, caseIDs *mockCaseIDsLoader, caseRoles *mockCaseRolesLoader) *Handler {
	return NewHandler(searcher, caseIDs, caseRoles, &mockAuditLogger{})
}

func doSearchRequest(t *testing.T, h *Handler, ac *auth.AuthContext, queryParams string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/search?"+queryParams, nil)
	if ac != nil {
		ctx := auth.WithAuthContext(req.Context(), *ac)
		req = req.WithContext(ctx)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	return env
}

func decodeSearchResult(t *testing.T, raw json.RawMessage) EvidenceSearchResult {
	t.Helper()
	var result EvidenceSearchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode search result: %v", err)
	}
	return result
}

// --- tests ---

func TestSearch_MissingAuthContext(t *testing.T) {
	h := newTestHandler(&mockEvidenceSearcher{}, &mockCaseIDsLoader{}, &mockCaseRolesLoader{})
	w := doSearchRequest(t, h, nil, "q=test")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	env := decodeEnvelope(t, w)
	if env.Error == nil || *env.Error != "internal error" {
		t.Fatalf("expected error 'internal error', got %v", env.Error)
	}
}

func TestSearch_WithQueryReturnsResults(t *testing.T) {
	searcher := &mockEvidenceSearcher{
		result: EvidenceSearchResult{
			Hits: []EvidenceSearchHit{
				{EvidenceID: "ev-1", CaseID: "case-1", Title: "Document A"},
			},
			TotalHits:        1,
			Query:            "test",
			ProcessingTimeMs: 5,
		},
	}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"case-1"}}, &mockCaseRolesLoader{roles: map[string]string{"case-1": "investigator"}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	env := decodeEnvelope(t, w)
	result := decodeSearchResult(t, env.Data)
	if len(result.Hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(result.Hits))
	}
	if result.Hits[0].EvidenceID != "ev-1" {
		t.Fatalf("expected evidence ID ev-1, got %s", result.Hits[0].EvidenceID)
	}
}

func TestSearch_AllFilterParams(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"case-1"}}, &mockCaseRolesLoader{roles: map[string]string{"case-1": "investigator"}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}

	w := doSearchRequest(t, h, ac, "q=weapon&case_id=case-1&type=image/jpeg,application/pdf&tag=photo,weapon&classification=confidential&from=2024-01-01&to=2024-12-31&limit=10&offset=5")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	q := searcher.query
	if q.Query != "weapon" {
		t.Errorf("expected query 'weapon', got %q", q.Query)
	}
	if q.CaseID == nil || *q.CaseID != "case-1" {
		t.Errorf("expected case_id 'case-1', got %v", q.CaseID)
	}
	if len(q.MimeTypes) != 2 || q.MimeTypes[0] != "image/jpeg" || q.MimeTypes[1] != "application/pdf" {
		t.Errorf("expected mime types [image/jpeg, application/pdf], got %v", q.MimeTypes)
	}
	if len(q.Tags) != 2 || q.Tags[0] != "photo" || q.Tags[1] != "weapon" {
		t.Errorf("expected tags [photo, weapon], got %v", q.Tags)
	}
	if len(q.Classifications) != 1 || q.Classifications[0] != "confidential" {
		t.Errorf("expected classifications [confidential], got %v", q.Classifications)
	}
	if q.DateFrom == nil || *q.DateFrom != "2024-01-01" {
		t.Errorf("expected date_from '2024-01-01', got %v", q.DateFrom)
	}
	if q.DateTo == nil || *q.DateTo != "2024-12-31" {
		t.Errorf("expected date_to '2024-12-31', got %v", q.DateTo)
	}
	if q.Limit != 10 {
		t.Errorf("expected limit 10, got %d", q.Limit)
	}
	if q.Offset != 5 {
		t.Errorf("expected offset 5, got %d", q.Offset)
	}
}

func TestSearch_SystemAdminBypassesCaseFiltering(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	// Case IDs loader should NOT be called for system admin
	caseLoader := &mockCaseIDsLoader{err: errors.New("should not be called")}
	rolesLoader := &mockCaseRolesLoader{err: errors.New("should not be called")}

	h := newTestHandler(searcher, caseLoader, rolesLoader)
	ac := &auth.AuthContext{UserID: "admin-1", SystemRole: auth.RoleSystemAdmin}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if len(searcher.query.UserCaseIDs) != 0 {
		t.Errorf("expected no UserCaseIDs for system admin, got %v", searcher.query.UserCaseIDs)
	}
}

func TestSearch_NonAdminWithZeroCaseIDsReturnsEmpty(t *testing.T) {
	searcher := &mockEvidenceSearcher{
		result: EvidenceSearchResult{Hits: []EvidenceSearchHit{{EvidenceID: "should-not-appear"}}},
	}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: nil}, &mockCaseRolesLoader{})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	env := decodeEnvelope(t, w)
	result := decodeSearchResult(t, env.Data)
	if len(result.Hits) != 0 {
		t.Errorf("expected 0 hits for user with no cases, got %d", len(result.Hits))
	}
}

func TestSearch_DefenceRoleSetsDisclosedOnly(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(
		searcher,
		&mockCaseIDsLoader{caseIDs: []string{"case-1", "case-2"}},
		&mockCaseRolesLoader{roles: map[string]string{"case-1": "investigator", "case-2": "defence"}},
	)

	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if !searcher.query.DisclosedOnly {
		t.Error("expected DisclosedOnly=true for defence role user")
	}
}

func TestSearch_NonDefenceRoleDoesNotSetDisclosedOnly(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(
		searcher,
		&mockCaseIDsLoader{caseIDs: []string{"case-1"}},
		&mockCaseRolesLoader{roles: map[string]string{"case-1": "investigator"}},
	)

	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if searcher.query.DisclosedOnly {
		t.Error("expected DisclosedOnly=false for non-defence role user")
	}
}

func TestSearch_InvalidLimitUsesDefault(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test&limit=abc&offset=xyz")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if searcher.query.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", searcher.query.Limit)
	}
	if searcher.query.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", searcher.query.Offset)
	}
}

func TestSearch_LimitCappedAt200(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test&limit=999")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if searcher.query.Limit != 200 {
		t.Errorf("expected capped limit 200, got %d", searcher.query.Limit)
	}
}

func TestSearch_NegativeLimitUsesDefault(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test&limit=-5")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Negative limit is not > 0, so stays at default 50
	if searcher.query.Limit != 50 {
		t.Errorf("expected default limit 50 for negative input, got %d", searcher.query.Limit)
	}
}

func TestSearch_NegativeOffsetUsesDefault(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test&offset=-3")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	if searcher.query.Offset != 0 {
		t.Errorf("expected default offset 0 for negative input, got %d", searcher.query.Offset)
	}
}

func TestSearch_MeilisearchErrorReturns503(t *testing.T) {
	searcher := &mockEvidenceSearcher{err: errors.New("meilisearch connection failed")}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	env := decodeEnvelope(t, w)
	if env.Error == nil || *env.Error != "search service unavailable" {
		t.Fatalf("expected error 'search service unavailable', got %v", env.Error)
	}
}

func TestSearch_CaseIDLoaderErrorReturns500(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceSearcher{},
		&mockCaseIDsLoader{err: errors.New("db error")},
		&mockCaseRolesLoader{},
	)
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestSearch_CaseRolesLoaderErrorReturns500(t *testing.T) {
	h := newTestHandler(
		&mockEvidenceSearcher{},
		&mockCaseIDsLoader{caseIDs: []string{"c1"}},
		&mockCaseRolesLoader{err: errors.New("db error")},
	)
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "q=test")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestSearch_EmptyQueryAllowed(t *testing.T) {
	searcher := &mockEvidenceSearcher{result: EvidenceSearchResult{}}

	h := newTestHandler(searcher, &mockCaseIDsLoader{caseIDs: []string{"c1"}}, &mockCaseRolesLoader{roles: map[string]string{}})
	ac := &auth.AuthContext{UserID: "user-1", SystemRole: auth.RoleUser}
	w := doSearchRequest(t, h, ac, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// --- parseSearchParams tests ---

func TestParseSearchParams_NoParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	q := parseSearchParams(req)

	if q.Query != "" {
		t.Errorf("expected empty query, got %q", q.Query)
	}
	if q.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", q.Limit)
	}
	if q.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", q.Offset)
	}
	if q.CaseID != nil {
		t.Errorf("expected nil CaseID, got %v", q.CaseID)
	}
	if q.DateFrom != nil {
		t.Errorf("expected nil DateFrom, got %v", q.DateFrom)
	}
	if q.DateTo != nil {
		t.Errorf("expected nil DateTo, got %v", q.DateTo)
	}
}

// --- splitAndTrim tests ---

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "single value", input: "photo", expected: []string{"photo"}},
		{name: "multiple values", input: "photo,weapon,blood", expected: []string{"photo", "weapon", "blood"}},
		{name: "with spaces", input: " photo , weapon , blood ", expected: []string{"photo", "weapon", "blood"}},
		{name: "empty parts filtered", input: "photo,,weapon,", expected: []string{"photo", "weapon"}},
		{name: "all empty", input: ",,,", expected: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d: %v", len(tt.expected), len(got), got)
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("item %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	r := chi.NewRouter()
	h := newTestHandler(&mockEvidenceSearcher{}, &mockCaseIDsLoader{}, &mockCaseRolesLoader{})
	h.RegisterRoutes(r)

	// Verify the route is registered by sending a request with auth context
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", nil)
	ctx := auth.WithAuthContext(req.Context(), auth.AuthContext{UserID: "u1", SystemRole: auth.RoleSystemAdmin})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected route to be registered and return %d, got %d", http.StatusOK, w.Code)
	}
}
