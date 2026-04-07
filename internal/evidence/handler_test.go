package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

type mockAudit struct{}

func (m *mockAudit) LogAccessDenied(_ context.Context, _, _, _, _, _ string) {}

type mockCustodyReader struct{}

func (m *mockCustodyReader) ListByEvidence(_ context.Context, _ uuid.UUID, _ int, _ string) ([]custody.Event, int, error) {
	return nil, 0, nil
}

func newTestHandler(t *testing.T) (*Handler, *mockRepo) {
	t.Helper()
	repo := newMockRepo()
	storage := newMockStorage()
	custody := &mockCustody{}
	caseLookup := &mockCaseLookup{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, custody, caseLookup,
		&noopThumbGen{}, logger, 100*1024*1024,
	)

	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	return handler, repo
}

func withAuthContext(r *http.Request) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     "test-user",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

func TestHandler_Get(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	repo.items[id] = EvidenceItem{
		ID:       id,
		Filename: "test.pdf",
		Tags:     []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/", handler.Get)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String(), nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
}

func TestHandler_Get_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/", handler.Get)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Get_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/", handler.Get)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.NewString(), nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_Upload(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	// Build multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "evidence.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("test file content"))
	writer.WriteField("classification", "restricted")
	writer.WriteField("description", "Test evidence upload")
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandler_Upload_InvalidCaseID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cases/not-a-uuid/evidence", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Upload_NoAuth(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+uuid.NewString()+"/evidence", nil)
	// No auth context
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_ListByCase(t *testing.T) {
	handler, repo := newTestHandler(t)

	caseID := uuid.New()
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.items[id] = EvidenceItem{
			ID:     id,
			CaseID: caseID,
			Tags:   []string{},
		}
	}

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Get("/", handler.ListByCase)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseID.String()+"/evidence?limit=10", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_UpdateMetadata(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	repo.items[id] = EvidenceItem{
		ID:             id,
		CaseID:         uuid.New(),
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Patch("/", handler.UpdateMetadata)
	})

	body := `{"description":"updated","tags":["new"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+id.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_Destroy(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	caseID := uuid.New()
	testKey := "test/key"
	repo.items[id] = EvidenceItem{
		ID:         id,
		CaseID:     caseID,
		StorageKey: &testKey,
		Tags:       []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Delete("/", handler.Destroy)
	})

	body := `{"reason":"Court ordered destruction"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+id.String(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestSplitTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single", "tag1", 1},
		{"multiple", "tag1,tag2,tag3", 3},
		{"with spaces", " tag1 , tag2 , tag3 ", 3},
		{"with empty parts", "tag1,,tag2", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTags(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitTags(%q) len = %d, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"no limit", "", 0},
		{"valid limit", "limit=25", 25},
		{"invalid limit", "limit=abc", 0},
		{"negative limit", "limit=-5", 0},
		{"zero limit", "limit=0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/test"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			p := parsePagination(req)
			if p.Limit != tt.want {
				t.Errorf("Limit = %d, want %d", p.Limit, tt.want)
			}
		})
	}
}

func TestParsePagination_Cursor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?cursor=abc123", nil)
	p := parsePagination(req)
	if p.Cursor != "abc123" {
		t.Errorf("Cursor = %q, want abc123", p.Cursor)
	}
}

func TestHandler_Download(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	storageKey := "evidence/test/key"

	// Need to also set up storage with the file
	storage := newMockStorage()
	storage.objects[storageKey] = []byte("file contents")

	// Recreate handler with the storage that has the file
	caseLookup := &mockCaseLookup{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, caseLookup,
		&noopThumbGen{}, logger, 100*1024*1024,
	)
	handler = NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)

	repo.items[id] = EvidenceItem{
		ID:         id,
		CaseID:     uuid.New(),
		Filename:   "test.pdf",
		StorageKey: &storageKey,
		Tags:       []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/download", handler.Download)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/download", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if w.Header().Get("Content-Disposition") == "" {
		t.Error("expected Content-Disposition header")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
}

func TestHandler_Download_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/download", handler.Download)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/download", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Download_NoAuth(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/download", handler.Download)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.NewString()+"/download", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_Download_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/download", handler.Download)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.NewString()+"/download", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_GetThumbnail(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	thumbKey := "thumbnails/test/thumb.jpg"

	storage := newMockStorage()
	storage.objects[thumbKey] = []byte("thumb-data")

	caseLookup := &mockCaseLookup{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, caseLookup,
		&noopThumbGen{}, logger, 100*1024*1024,
	)
	handler = NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)

	repo.items[id] = EvidenceItem{
		ID:           id,
		ThumbnailKey: &thumbKey,
		Tags:         []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/thumbnail", handler.GetThumbnail)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/thumbnail", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") == "" {
		t.Error("expected Cache-Control header")
	}
}

func TestHandler_GetThumbnail_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/thumbnail", handler.GetThumbnail)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/thumbnail", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_GetThumbnail_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/thumbnail", handler.GetThumbnail)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.NewString()+"/thumbnail", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_GetVersionHistory(t *testing.T) {
	handler, repo := newTestHandler(t)
	id := uuid.New()
	repo.items[id] = EvidenceItem{
		ID:      id,
		Version: 1,
		Tags:    []string{},
	}

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/versions", handler.GetVersionHistory)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/versions", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_GetVersionHistory_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/versions", handler.GetVersionHistory)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/versions", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_GetVersionHistory_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/versions", handler.GetVersionHistory)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+uuid.NewString()+"/versions", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_GetCustodyLog(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_GetCustodyLog_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/not-a-uuid/custody", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_GetCustodyLog_WithLimitAndCursor(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody?limit=10&cursor=abc", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_GetCustodyLog_InvalidLimit(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	// Invalid limit should use default 50
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody?limit=abc", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_GetCustodyLog_OverMaxLimit(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	// Limit > 200 should use default 50
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody?limit=500", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_GetCustodyLog_Error(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, &mockCaseLookup{},
		&noopThumbGen{}, logger, 100*1024*1024,
	)

	failingCustodyReader := &mockFailingCustodyReader{}
	handler := NewHandler(svc, failingCustodyReader, &mockAudit{}, 100*1024*1024)

	id := uuid.New()
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

type mockFailingCustodyReader struct{}

func (m *mockFailingCustodyReader) ListByEvidence(_ context.Context, _ uuid.UUID, _ int, _ string) ([]custody.Event, int, error) {
	return nil, 0, errors.New("custody read error")
}

func TestHandler_UpdateMetadata_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Patch("/", handler.UpdateMetadata)
	})

	body := `{"description":"updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_UpdateMetadata_NoAuth(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Patch("/", handler.UpdateMetadata)
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+uuid.NewString(), strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_UpdateMetadata_InvalidJSON(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Patch("/", handler.UpdateMetadata)
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+uuid.NewString(), strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Destroy_InvalidID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Delete("/", handler.Destroy)
	})

	body := `{"reason":"test"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/not-a-uuid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Destroy_NoAuth(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Delete("/", handler.Destroy)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+uuid.NewString(), strings.NewReader(`{"reason":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_Destroy_InvalidJSON(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Delete("/", handler.Destroy)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+uuid.NewString(), strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Destroy_NotFound(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Delete("/", handler.Destroy)
	})

	body := `{"reason":"test"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/evidence/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandler_ListByCase_ServiceError(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, &mockCaseLookup{},
		&noopThumbGen{}, logger, 100*1024*1024,
	)
	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)

	repo.findByCaseFn = func(_ context.Context, _ EvidenceFilter, _ Pagination) ([]EvidenceItem, int, error) {
		return nil, 0, errors.New("db error")
	}

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Get("/", handler.ListByCase)
	})

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseID.String()+"/evidence", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandler_UpdateMetadata_ServiceValidationError(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Patch("/", handler.UpdateMetadata)
	})

	// Invalid classification should trigger service validation error
	body := `{"classification":"top_secret"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/evidence/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_ListByCase_InvalidCaseID(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Get("/", handler.ListByCase)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cases/not-a-uuid/evidence", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_ListByCase_WithFilters(t *testing.T) {
	handler, _ := newTestHandler(t)
	caseID := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Get("/", handler.ListByCase)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/cases/"+caseID.String()+"/evidence?classification=restricted&mime_type=image/jpeg&q=search&tags=tag1,tag2&current_only=true&limit=10",
		nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_Upload_MissingFile(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	// Multipart form without a file
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("classification", "restricted")
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_Upload_WithTags(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "evidence.pdf")
	part.Write([]byte("test content"))
	writer.WriteField("classification", "restricted")
	writer.WriteField("tags", `["tag1","tag2"]`)
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandler_Upload_CommaSeparatedTags(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "evidence.pdf")
	part.Write([]byte("test content"))
	writer.WriteField("classification", "restricted")
	writer.WriteField("tags", "tag1,tag2,tag3") // comma-separated fallback
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandler_Upload_NoClassification(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "evidence.pdf")
	part.Write([]byte("test content"))
	// No classification field — should default to "restricted"
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestHandler_Upload_InvalidMultipart(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", strings.NewReader("not a multipart form"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=nonexistent")
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	handler, _ := newTestHandler(t)
	r := chi.NewRouter()

	// Should not panic
	handler.RegisterRoutes(r)
}

func TestRespondServiceError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"validation error", &ValidationError{Field: "test", Message: "invalid"}, http.StatusBadRequest},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"internal error", errors.New("unexpected"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondServiceError(w, tt.err)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestDecodeBody_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("not json"))
	var dst map[string]string
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeBody_ValidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"key":"value"}`))
	var dst map[string]string
	err := decodeBody(req, &dst)
	if err != nil {
		t.Fatalf("decodeBody error: %v", err)
	}
	if dst["key"] != "value" {
		t.Errorf("dst[key] = %q, want value", dst["key"])
	}
}

func TestHandler_Download_IOCopyError(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, &mockCaseLookup{},
		&noopThumbGen{}, logger, 100*1024*1024,
	)
	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)

	id := uuid.New()
	storageKey := "evidence/test/copy-fail"
	repo.items[id] = EvidenceItem{
		ID:         id,
		CaseID:     uuid.New(),
		Filename:   "test.pdf",
		StorageKey: &storageKey,
		Tags:       []string{},
	}
	storage.objects[storageKey] = []byte("file contents")

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/download", handler.Download)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/download", nil)
	req = withAuthContext(req)
	// Use a failWriter that fails on Write to trigger the io.Copy error path
	w := &failWriter{ResponseWriter: httptest.NewRecorder()}

	r.ServeHTTP(w, req)
	// The handler should not panic; it just returns early on io.Copy error
}

// failWriter wraps http.ResponseWriter and fails on Write.
type failWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (f *failWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func (f *failWriter) WriteHeader(statusCode int) {
	f.ResponseWriter.WriteHeader(statusCode)
}

func (f *failWriter) Header() http.Header {
	return f.ResponseWriter.Header()
}

func TestDecodeBody_TooLarge(t *testing.T) {
	// Create body with valid JSON start but larger than MaxBodySize (1MB).
	// The LimitReader will cut it off, causing ErrUnexpectedEOF.
	largeBody := `{"key":"` + strings.Repeat("x", MaxBodySize+100) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))
	var dst map[string]string
	err := decodeBody(req, &dst)
	if err == nil {
		t.Fatal("expected error for body too large")
	}
}

func TestHandler_GetCustodyLog_ZeroLimit(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	// limit=0 should use default
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody?limit=0", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_GetCustodyLog_NegativeLimit(t *testing.T) {
	handler, _ := newTestHandler(t)
	id := uuid.New()

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody?limit=-5", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_GetCustodyLog_WithEvents(t *testing.T) {
	repo := newMockRepo()
	storage := newMockStorage()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, &mockCustody{}, &mockCaseLookup{},
		&noopThumbGen{}, logger, 100*1024*1024,
	)

	custodyReader := &mockCustodyReaderWithEvents{}
	handler := NewHandler(svc, custodyReader, &mockAudit{}, 100*1024*1024)

	id := uuid.New()
	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/custody", handler.GetCustodyLog)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+id.String()+"/custody", nil)
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

type mockCustodyReaderWithEvents struct{}

func (m *mockCustodyReaderWithEvents) ListByEvidence(_ context.Context, _ uuid.UUID, _ int, _ string) ([]custody.Event, int, error) {
	return []custody.Event{
		{
			ID:           uuid.New(),
			CaseID:       uuid.New(),
			EvidenceID:   uuid.New(),
			Action:       "evidence_uploaded",
			ActorUserID:  "user-1",
			Detail:       `{"filename":"test.pdf"}`,
			HashValue:    "abc123",
			PreviousHash: "",
			Timestamp:    time.Now(),
		},
	}, 1, nil
}

func TestHandler_Upload_ServiceValidationError(t *testing.T) {
	handler, _ := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", handler.Upload)
	})

	// Upload with invalid classification
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "evidence.pdf")
	part.Write([]byte("test content"))
	writer.WriteField("classification", "top_secret_invalid")
	writer.Close()

	caseID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID.String()+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
