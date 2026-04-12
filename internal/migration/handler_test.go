package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// noopAudit is a minimal auth.AuditLogger used by handler tests.
type noopAudit struct{}

func (noopAudit) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

// fakeCaseLookup satisfies migration.CaseLookup.
type fakeCaseLookup struct {
	info CaseInfo
	err  error
}

func (f fakeCaseLookup) GetCaseInfo(_ context.Context, _ uuid.UUID) (CaseInfo, error) {
	return f.info, f.err
}

// buildHandlerWithStaging returns a wired Handler plus its temp staging
// directory. The directory contains a valid CSV manifest and one file so
// a happy-path run can exercise the full handler → service → ingester path.
func buildHandlerWithStaging(t *testing.T) (*Handler, string, uuid.UUID) {
	t.Helper()
	dir := t.TempDir()
	hashA := writeTestFile(t, dir, "a.txt", "A")
	manifest := filepath.Join(dir, "manifest.csv")
	if err := os.WriteFile(manifest, []byte("filename,sha256_hash\na.txt,"+hashA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := newFakeRepo()
	writer := newFakeWriter()
	svc := NewService(nil, NewIngester(writer, repo), repo, &stubTSA{}, nil)

	signer, err := LoadOrGenerate()
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(
		svc,
		fakeCaseLookup{info: CaseInfo{ReferenceCode: "ICC-TEST", Title: "Test Case"}},
		signer,
		noopAudit{},
		"test",
		dir,
		nil,
	)
	return h, dir, uuid.New()
}

// newAuthedRequest constructs a request with an authenticated context and
// the supplied chi URL params.
func newAuthedRequest(t *testing.T, method, target string, body []byte, params map[string]string) *http.Request {
	t.Helper()
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "tester",
		Username:   "tester",
		SystemRole: auth.RoleCaseAdmin,
	})
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}

func TestValidateStagedPath_HappyPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "manifest.csv")
	if err := os.WriteFile(f, []byte("filename\na.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Handler{stagingRoot: dir}
	got, err := h.validateStagedPath(f)
	if err != nil {
		t.Fatalf("validateStagedPath: %v", err)
	}
	// Compare against the symlink-resolved staging root — on macOS
	// t.TempDir returns paths under /var/folders which resolves to
	// /private/var/folders, and the handler itself calls EvalSymlinks.
	resolvedRoot, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(resolvedRoot, got)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("validated path escaped staging root: got=%q rel=%q", got, rel)
	}
}

func TestValidateStagedPath_Rejections(t *testing.T) {
	dir := t.TempDir()
	h := &Handler{stagingRoot: dir}
	cases := map[string]string{
		"empty":    "",
		"nullByte": "file\x00name",
		"outside":  "/etc/passwd",
		"nonexist": filepath.Join(dir, "does-not-exist"),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := h.validateStagedPath(p); err == nil {
				t.Errorf("want error for %q", p)
			}
		})
	}
}

func TestValidateStagedPath_EmptyStagingDisables(t *testing.T) {
	h := &Handler{stagingRoot: ""}
	if _, err := h.validateStagedPath("/tmp/anything"); err == nil {
		t.Error("want error when staging root is empty")
	}
}

func TestRunMigration_HappyPath(t *testing.T) {
	h, dir, caseID := buildHandlerWithStaging(t)
	body, _ := json.Marshal(runMigrationRequest{
		SourceSystem:   "RelativityOne",
		SourceRoot:     dir,
		ManifestPath:   filepath.Join(dir, "manifest.csv"),
		ManifestFormat: "csv",
		Concurrency:    1,
	})
	r := newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", body, map[string]string{
		"caseID": caseID.String(),
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	data := unwrapData(t, w.Body.Bytes())
	matched, ok := data["matched"].(float64)
	if !ok {
		t.Fatalf("matched field missing; data=%v", data)
	}
	if matched != 1 {
		t.Errorf("matched = %v, want 1", matched)
	}
}

// unwrapData extracts the `data` field from the envelope that
// httputil.RespondJSON emits ({"data":..., "error":null, "meta":null}).
func unwrapData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v body=%s", err, body)
	}
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("envelope has no data object; env=%v", env)
	}
	return data
}

func TestRunMigration_InvalidCaseID(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	r := newAuthedRequest(t, "POST", "/api/cases/bad/migrations", []byte(`{}`), map[string]string{
		"caseID": "not-a-uuid",
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRunMigration_MissingRequiredFields(t *testing.T) {
	h, _, caseID := buildHandlerWithStaging(t)
	r := newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", []byte(`{}`), map[string]string{
		"caseID": caseID.String(),
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestRunMigration_ManifestPathOutsideStaging(t *testing.T) {
	h, dir, caseID := buildHandlerWithStaging(t)
	body, _ := json.Marshal(runMigrationRequest{
		SourceSystem: "RelativityOne",
		SourceRoot:   dir,
		ManifestPath: "/etc/passwd",
	})
	r := newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", body, map[string]string{
		"caseID": caseID.String(),
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if strings.Contains(w.Body.String(), "/etc/passwd") {
		t.Error("response body leaks the rejected path")
	}
}

func TestListMigrations_HappyPath(t *testing.T) {
	h, dir, caseID := buildHandlerWithStaging(t)
	// Seed one migration row via a real Run.
	body, _ := json.Marshal(runMigrationRequest{
		SourceSystem:   "RelativityOne",
		SourceRoot:     dir,
		ManifestPath:   filepath.Join(dir, "manifest.csv"),
		ManifestFormat: "csv",
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", body,
		map[string]string{"caseID": caseID.String()}))
	if w.Code != http.StatusCreated {
		t.Fatalf("seed failed: %d %s", w.Code, w.Body.String())
	}

	r := newAuthedRequest(t, "GET", "/api/cases/"+caseID.String()+"/migrations", nil,
		map[string]string{"caseID": caseID.String()})
	w2 := httptest.NewRecorder()
	h.ListMigrations(w2, r)
	if w2.Code != http.StatusOK {
		t.Errorf("status = %d, body=%s", w2.Code, w2.Body.String())
	}
	var env map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &env)
	data, _ := env["data"].([]any)
	if len(data) != 1 {
		t.Errorf("want 1 record, got %d", len(data))
	}
}

func TestListMigrations_InvalidCaseID(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	r := newAuthedRequest(t, "GET", "/api/cases/bad/migrations", nil,
		map[string]string{"caseID": "not-a-uuid"})
	w := httptest.NewRecorder()
	h.ListMigrations(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListMigrations_InternalError(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	h.svc = NewService(nil, NewIngester(newFakeWriter(), errorRepo{}), errorRepo{}, &stubTSA{}, nil)
	caseID := uuid.New()
	r := newAuthedRequest(t, "GET", "/api/cases/"+caseID.String()+"/migrations", nil,
		map[string]string{"caseID": caseID.String()})
	w := httptest.NewRecorder()
	h.ListMigrations(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestListMigrations_EmptyCase(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	// A fresh case UUID with no migrations should return an empty array,
	// not null, to keep the client's .length safe.
	freshCase := uuid.New()
	r := newAuthedRequest(t, "GET", "/api/cases/"+freshCase.String()+"/migrations", nil,
		map[string]string{"caseID": freshCase.String()})
	w := httptest.NewRecorder()
	h.ListMigrations(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	data, ok := env["data"].([]any)
	if !ok {
		t.Fatalf("data is not an array: %T", env["data"])
	}
	if len(data) != 0 {
		t.Errorf("len = %d, want 0", len(data))
	}
}

func TestGetMigration_NotFound(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	id := uuid.New()
	r := newAuthedRequest(t, "GET", "/api/migrations/"+id.String(), nil, map[string]string{
		"id": id.String(),
	})
	w := httptest.NewRecorder()
	h.GetMigration(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetMigration_InvalidID(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	r := newAuthedRequest(t, "GET", "/api/migrations/bad", nil, map[string]string{
		"id": "not-a-uuid",
	})
	w := httptest.NewRecorder()
	h.GetMigration(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDownloadCertificate_RendersPDF(t *testing.T) {
	h, dir, caseID := buildHandlerWithStaging(t)
	// First run a migration so we have a record.
	body, _ := json.Marshal(runMigrationRequest{
		SourceSystem:   "RelativityOne",
		SourceRoot:     dir,
		ManifestPath:   filepath.Join(dir, "manifest.csv"),
		ManifestFormat: "csv",
	})
	r := newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", body, map[string]string{
		"caseID": caseID.String(),
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("migration run failed: %d %s", w.Code, w.Body.String())
	}
	data := unwrapData(t, w.Body.Bytes())
	migMap, _ := data["migration"].(map[string]any)
	idStr, _ := migMap["ID"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		t.Fatalf("bad migration id: %v (data=%v)", err, data)
	}

	// Now download the certificate.
	r2 := newAuthedRequest(t, "GET", "/api/migrations/"+id.String()+"/certificate", nil, map[string]string{
		"id": id.String(),
	})
	w2 := httptest.NewRecorder()
	h.DownloadCertificate(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w2.Code, w2.Body.String())
	}
	// The response body is the raw PDF, not JSON.
	if got := w2.Body.Bytes(); len(got) < 5 || string(got[:5]) != "%PDF-" {
		t.Errorf("not a PDF: got first bytes=%q", got[:min(len(got), 20)])
	}
	if ct := w2.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
	if a := w2.Header().Get("X-Certificate-Appendix"); a != "empty" {
		t.Errorf("X-Certificate-Appendix = %q, want empty", a)
	}
}

func TestSigningKeyJWKS_ReturnsPublicKey(t *testing.T) {
	h, _, _ := buildHandlerWithStaging(t)
	r := httptest.NewRequest("GET", "/.well-known/vaultkeeper-signing-key", nil)
	w := httptest.NewRecorder()
	h.signingKeyJWKS(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	data := unwrapData(t, w.Body.Bytes())
	if data["alg"] != "Ed25519" {
		t.Errorf("alg = %v", data["alg"])
	}
	if _, ok := data["publicKey"].(string); !ok {
		t.Errorf("publicKey missing or wrong type: %v", data["publicKey"])
	}
}

func TestSigningKeyJWKS_MissingSigner(t *testing.T) {
	h := &Handler{signer: nil}
	w := httptest.NewRecorder()
	h.signingKeyJWKS(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestFirstAccept_ParsesLocale(t *testing.T) {
	cases := map[string]string{
		"":                     "en",
		"en":                   "en",
		"fr-CA,fr;q=0.9,en;q=0.8": "fr-CA",
		"en-US":                 "en-US",
	}
	for in, want := range cases {
		if got := firstAccept(in); got != want {
			t.Errorf("firstAccept(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunMigration_DryRunRowDeleted(t *testing.T) {
	h, dir, caseID := buildHandlerWithStaging(t)
	body, _ := json.Marshal(runMigrationRequest{
		SourceSystem:   "RelativityOne",
		SourceRoot:     dir,
		ManifestPath:   filepath.Join(dir, "manifest.csv"),
		ManifestFormat: "csv",
		DryRun:         true,
	})
	r := newAuthedRequest(t, "POST", "/api/cases/"+caseID.String()+"/migrations", body, map[string]string{
		"caseID": caseID.String(),
	})
	w := httptest.NewRecorder()
	h.RunMigration(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}
	// The dry-run row must have been deleted by the service.
	data := unwrapData(t, w.Body.Bytes())
	migRaw, _ := data["migration"].(map[string]any)
	idStr, _ := migRaw["ID"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		t.Fatalf("bad migration id: %v", err)
	}
	if _, err := h.svc.Get(context.Background(), id); !errors.Is(err, ErrNotFound) {
		t.Errorf("dry-run row should be deleted; got err=%v", err)
	}
}
