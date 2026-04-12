package evidence

// Handler-level tests for the Sprint 9 tag management endpoints.
// Covers auth/case-membership gating, happy paths, and error mapping.
// Tag service and repository logic is exercised by tags_test.go and
// (under the integration tag) tag_repository_integration_test.go; this
// file fills the gdpr_handler-style gap where the HTTP surface had no
// direct coverage.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// tagTrackingRepo augments the in-memory mockRepo with tag mutation
// tracking so handler tests can assert the service→repo wiring landed.
type tagTrackingRepo struct {
	*mockRepo
	autocompleteCalls []autocompleteCall
	renameCalls       []tagRenameCall
	mergeCalls        []tagMergeCall
	deleteCalls       []tagDeleteCall
}

type autocompleteCall struct {
	caseID uuid.UUID
	prefix string
	limit  int
}
type tagRenameCall struct {
	caseID uuid.UUID
	old    string
	new    string
}
type tagMergeCall struct {
	caseID  uuid.UUID
	sources []string
	target  string
}
type tagDeleteCall struct {
	caseID uuid.UUID
	tag    string
}

func (r *tagTrackingRepo) ListDistinctTags(_ context.Context, caseID uuid.UUID, prefix string, limit int) ([]string, error) {
	r.autocompleteCalls = append(r.autocompleteCalls, autocompleteCall{caseID, prefix, limit})
	if r.tagAutocompleteErr != nil {
		return nil, r.tagAutocompleteErr
	}
	return []string{"photo", "redacted", "witness"}, nil
}
func (r *tagTrackingRepo) RenameTagInCase(_ context.Context, caseID uuid.UUID, oldTag, newTag string) (int64, error) {
	r.renameCalls = append(r.renameCalls, tagRenameCall{caseID, oldTag, newTag})
	return 3, nil
}
func (r *tagTrackingRepo) MergeTagsInCase(_ context.Context, caseID uuid.UUID, sources []string, target string) (int64, error) {
	r.mergeCalls = append(r.mergeCalls, tagMergeCall{caseID, sources, target})
	return 2, nil
}
func (r *tagTrackingRepo) DeleteTagFromCase(_ context.Context, caseID uuid.UUID, tag string) (int64, error) {
	r.deleteCalls = append(r.deleteCalls, tagDeleteCall{caseID, tag})
	return 1, nil
}

// stubCaseRoleLoader implements auth.CaseRoleLoader with a simple allow/deny.
type stubCaseRoleLoader struct {
	role    auth.CaseRole
	err     error
	lastCID string
	lastUID string
}

func (s *stubCaseRoleLoader) LoadCaseRole(_ context.Context, caseID, userID string) (auth.CaseRole, error) {
	s.lastCID = caseID
	s.lastUID = userID
	return s.role, s.err
}

func newTagHandlerTest(t *testing.T, loader auth.CaseRoleLoader) (*Handler, *tagTrackingRepo) {
	t.Helper()
	inner := newMockRepo()
	repo := &tagTrackingRepo{mockRepo: inner}
	storage := newMockStorage()
	custody := &mockCustody{}
	cases := &mockCaseLookup{status: "active"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, custody, cases,
		&noopThumbGen{}, logger, 100*1024*1024,
	)

	h := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	if loader != nil {
		h.SetCaseRoleLoader(loader)
	}
	return h, repo
}

func withCaseMemberAuth(r *http.Request, userID string, role auth.SystemRole) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     userID,
		Username:   "member",
		SystemRole: role,
	})
	return r.WithContext(ctx)
}

// ---- TagAutocomplete ----

func TestTagAutocomplete_SystemAdminBypass(t *testing.T) {
	// System admin bypasses the case-role loader entirely.
	h, repo := newTagHandlerTest(t, nil) // no loader configured

	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id="+caseID.String()+"&q=ph", nil)
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if len(repo.autocompleteCalls) != 1 {
		t.Errorf("autocomplete calls = %d, want 1", len(repo.autocompleteCalls))
	}
	if repo.autocompleteCalls[0].prefix != "ph" {
		t.Errorf("prefix = %q, want %q", repo.autocompleteCalls[0].prefix, "ph")
	}
}

func TestTagAutocomplete_CaseMemberAllowed(t *testing.T) {
	loader := &stubCaseRoleLoader{role: auth.CaseRoleInvestigator}
	h, repo := newTagHandlerTest(t, loader)

	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id="+caseID.String(), nil)
	req = withCaseMemberAuth(req, "member-id", auth.RoleAPIService)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if loader.lastCID != caseID.String() || loader.lastUID != "member-id" {
		t.Errorf("loader args: cID=%q uID=%q", loader.lastCID, loader.lastUID)
	}
	if len(repo.autocompleteCalls) != 1 {
		t.Errorf("autocomplete calls = %d, want 1", len(repo.autocompleteCalls))
	}
}

func TestTagAutocomplete_NonMemberRejected(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	h, repo := newTagHandlerTest(t, loader)

	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id="+uuid.New().String(), nil)
	req = withCaseMemberAuth(req, "stranger-id", auth.RoleAPIService)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if len(repo.autocompleteCalls) != 0 {
		t.Error("autocomplete must not run for non-members")
	}
}

func TestTagAutocomplete_InvalidCaseID(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id=not-a-uuid", nil)
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTagAutocomplete_LimitClamped(t *testing.T) {
	h, repo := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/tags/autocomplete?case_id="+caseID.String()+"&limit=9999", nil)
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	// Service layer clamps to MaxTagAutocompleteLimit; handler just forwards.
	if repo.autocompleteCalls[0].limit > MaxTagAutocompleteLimit {
		t.Errorf("limit = %d, want <= %d", repo.autocompleteCalls[0].limit, MaxTagAutocompleteLimit)
	}
}

// ---- TagRename ----

func TestTagRename_HappyPath(t *testing.T) {
	h, repo := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)

	caseID := uuid.New()
	body := `{"case_id":"` + caseID.String() + `","old":"witness","new":"key-witness"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if len(repo.renameCalls) != 1 {
		t.Fatalf("rename calls = %d, want 1", len(repo.renameCalls))
	}
	if repo.renameCalls[0].old != "witness" || repo.renameCalls[0].new != "key-witness" {
		t.Errorf("rename args = %+v", repo.renameCalls[0])
	}

	var resp struct {
		Data struct {
			RowsAffected int64  `json:"rows_affected"`
			Old          string `json:"old"`
			New          string `json:"new"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.RowsAffected != 3 {
		t.Errorf("rows_affected = %d, want 3", resp.Data.RowsAffected)
	}
}

func TestTagRename_NonMemberRejected(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	h, repo := newTagHandlerTest(t, loader)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)

	body := `{"case_id":"` + uuid.New().String() + `","old":"a","new":"b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "stranger-id", auth.RoleAPIService)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if len(repo.renameCalls) != 0 {
		t.Error("rename must not run for non-members")
	}
}

func TestTagRename_InvalidBody(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/rename", h.TagRename)

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/rename", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- TagMerge ----

func TestTagMerge_HappyPath(t *testing.T) {
	h, repo := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)

	caseID := uuid.New()
	body := `{"case_id":"` + caseID.String() + `","sources":["a","b"],"target":"merged"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if len(repo.mergeCalls) != 1 {
		t.Fatalf("merge calls = %d, want 1", len(repo.mergeCalls))
	}
	if repo.mergeCalls[0].target != "merged" {
		t.Errorf("target = %q, want merged", repo.mergeCalls[0].target)
	}
	if len(repo.mergeCalls[0].sources) != 2 {
		t.Errorf("sources = %v", repo.mergeCalls[0].sources)
	}
}

func TestTagMerge_NonMemberRejected(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	h, repo := newTagHandlerTest(t, loader)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/merge", h.TagMerge)

	body := `{"case_id":"` + uuid.New().String() + `","sources":["a"],"target":"b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/merge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "stranger", auth.RoleAPIService)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if len(repo.mergeCalls) != 0 {
		t.Error("merge must not run for non-members")
	}
}

// ---- TagDelete ----

func TestTagDelete_HappyPath(t *testing.T) {
	h, repo := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/delete", h.TagDelete)

	caseID := uuid.New()
	body := `{"case_id":"` + caseID.String() + `","tag":"obsolete"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0].tag != "obsolete" {
		t.Errorf("delete calls = %+v", repo.deleteCalls)
	}
}

func TestTagDelete_NonMemberRejected(t *testing.T) {
	loader := &stubCaseRoleLoader{err: errors.New("no role")}
	h, repo := newTagHandlerTest(t, loader)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/delete", h.TagDelete)

	body := `{"case_id":"` + uuid.New().String() + `","tag":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "stranger", auth.RoleAPIService)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	if len(repo.deleteCalls) != 0 {
		t.Error("delete must not run for non-members")
	}
}

func TestTagDelete_InvalidTag(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil)

	r := chi.NewRouter()
	r.Post("/api/evidence/tags/delete", h.TagDelete)

	body := `{"case_id":"` + uuid.New().String() + `","tag":"BAD CHARS!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/tags/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withCaseMemberAuth(req, "admin-id", auth.RoleSystemAdmin)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- requireCaseMembership (negative path: nil loader, non-admin) ----

func TestRequireCaseMembership_NilLoaderNonAdmin(t *testing.T) {
	h, _ := newTagHandlerTest(t, nil) // loader nil
	caseID := uuid.New()
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     "user-id",
		SystemRole: auth.RoleAPIService,
	})
	if h.requireCaseMembership(ctx, caseID) {
		t.Error("nil loader + non-admin must deny")
	}
}

// ---- IsValidExParteSide + IsValidClassification exported helpers ----

func TestIsValidClassification(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{ClassificationPublic, true},
		{ClassificationRestricted, true},
		{ClassificationConfidential, true},
		{ClassificationExParte, true},
		{"", false},
		{"top_secret", false},
	}
	for _, tc := range cases {
		if got := IsValidClassification(tc.in); got != tc.want {
			t.Errorf("IsValidClassification(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsValidExParteSide(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"prosecution", true},
		{"defence", true},
		{"", false},
		{"judge", false},
		{"PROSECUTION", false}, // case-sensitive
	}
	for _, tc := range cases {
		if got := IsValidExParteSide(tc.in); got != tc.want {
			t.Errorf("IsValidExParteSide(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
