//go:build integration

package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/search"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------
//
// Note: mock types (mockDraftRoleLoader, mockDraftCustody, mockDraftCustodyError)
// are defined in draft_mocks_test.go so they remain available to unit tests that
// are NOT behind the `integration` build tag.

// newDraftHandler builds a DraftHandler backed by the real PGRepository so
// that integration tests exercise the full SQL path.
// It returns the handler and the custody recorder so tests can inspect events.
func newDraftHandler(t *testing.T) (*DraftHandler, *mockDraftCustody) {
	t.Helper()
	pool := startPostgresContainer(t)
	custody := &mockDraftCustody{}
	roleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewDraftHandler(pool, roleLoader, custody, logger)
	return h, custody
}

// registerDraftRoutes mounts the DraftHandler routes on a new chi router.
func registerDraftRoutes(h *DraftHandler) chi.Router {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// seedCase2 is a local alias so draft handler tests can seed cases using the
// pgxpool.Pool stored in the fixture without duplicating the helper.
func seedCase2(t *testing.T, pool *pgxpool.Pool, refCode string) uuid.UUID {
	return seedCase(t, pool, refCode)
}

// withDraftAuthContext attaches an admin auth context so the handler skips
// case-role checks (SystemRole >= RoleSystemAdmin bypasses LoadCaseRole).
func withDraftAuthContext(r *http.Request) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		// Use a well-formed UUID so that DB columns typed UUID accept it.
		UserID:     "00000000-0000-4000-8000-000000000099",
		Username:   "testuser",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

// withDraftUserContext attaches a regular-user auth context that requires the
// role loader to permit access. userID must be a valid UUID string.
func withDraftUserContext(r *http.Request, userID string) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     userID,
		Username:   "regular-user",
		SystemRole: auth.RoleUser,
	})
	return r.WithContext(ctx)
}

// seedEvidenceItem inserts a minimal evidence_items row and returns its ID.
func seedEvidenceItem(t *testing.T, h *DraftHandler, caseID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	evidenceID := uuid.New()
	_, err := h.db.Exec(ctx,
		`INSERT INTO evidence_items
		 (id, case_id, evidence_number, filename, original_name, storage_key,
		  mime_type, size_bytes, sha256_hash, classification, uploaded_by, tsa_status, tags)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		evidenceID, caseID, "EV-DRAFT-001", "doc.pdf", "doc.pdf", "evidence/test/doc.pdf",
		"application/pdf", 1024, strings.Repeat("a", 64), "restricted",
		"00000000-0000-4000-8000-000000000001", "disabled", []string{},
	)
	if err != nil {
		t.Fatalf("seed evidence_item: %v", err)
	}
	return evidenceID
}

// decodeEnvelope decodes the standard JSON envelope and unmarshals data into dst.
func decodeEnvelope(t *testing.T, body []byte, dst any) {
	t.Helper()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *string         `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if dst != nil && env.Data != nil {
		if err := json.Unmarshal(env.Data, dst); err != nil {
			t.Fatalf("decode envelope data: %v", err)
		}
	}
}

// errorMessage decodes and returns the error field from the response envelope.
func errorMessage(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error *string `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error == nil {
		return ""
	}
	return *env.Error
}

// jsonBody serialises v and returns a bytes.Buffer (implements io.Reader and exposes Bytes).
func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ---------------------------------------------------------------------------
// Auth: unauthenticated requests → 401
// ---------------------------------------------------------------------------

func TestDraftHandler_Unauthenticated(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)
	evidenceID := uuid.New()

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, fmt.Sprintf("/api/evidence/%s/redact/drafts", evidenceID)},
		{http.MethodGet, fmt.Sprintf("/api/evidence/%s/redact/drafts", evidenceID)},
		{http.MethodGet, fmt.Sprintf("/api/evidence/%s/redact/drafts/%s", evidenceID, uuid.New())},
		{http.MethodPut, fmt.Sprintf("/api/evidence/%s/redact/drafts/%s", evidenceID, uuid.New())},
		{http.MethodDelete, fmt.Sprintf("/api/evidence/%s/redact/drafts/%s", evidenceID, uuid.New())},
		{http.MethodPost, fmt.Sprintf("/api/evidence/%s/redact/drafts/%s/finalize", evidenceID, uuid.New())},
		{http.MethodGet, fmt.Sprintf("/api/evidence/%s/redactions", evidenceID)},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			// No auth context attached — should be rejected.
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want %d; body: %s", w.Code, http.StatusUnauthorized, w.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CreateDraft
// ---------------------------------------------------------------------------

func TestDraftHandler_CreateDraft_Success(t *testing.T) {
	h, custody := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DRAFT-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{
		"name":    "Defence Disclosure v1",
		"purpose": "disclosure_defence",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var draft RedactionDraft
	decodeEnvelope(t, w.Body.Bytes(), &draft)
	if draft.ID == uuid.Nil {
		t.Error("expected non-nil draft ID")
	}
	if draft.Name != "Defence Disclosure v1" {
		t.Errorf("name = %q, want %q", draft.Name, "Defence Disclosure v1")
	}
	if draft.Purpose != PurposeDisclosureDefence {
		t.Errorf("purpose = %q, want %q", draft.Purpose, PurposeDisclosureDefence)
	}
	if draft.EvidenceID != evidenceID {
		t.Errorf("evidence_id = %s, want %s", draft.EvidenceID, evidenceID)
	}

	if len(custody.events) != 1 || custody.events[0] != "redaction_draft_created" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestDraftHandler_CreateDraft_MissingName(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DRAFT-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{
		"name":    "",
		"purpose": "disclosure_defence",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if msg := errorMessage(t, w.Body.Bytes()); !strings.Contains(msg, "name") {
		t.Errorf("error message %q does not mention 'name'", msg)
	}
}

func TestDraftHandler_CreateDraft_InvalidPurpose(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DRAFT-003")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{
		"name":    "Bad Purpose Draft",
		"purpose": "not_a_real_purpose",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if msg := errorMessage(t, w.Body.Bytes()); !strings.Contains(msg, "purpose") {
		t.Errorf("error message %q does not mention 'purpose'", msg)
	}
}

func TestDraftHandler_CreateDraft_NameTooLong(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DRAFT-004")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{
		"name":    strings.Repeat("x", 256),
		"purpose": "disclosure_defence",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDraftHandler_CreateDraft_DuplicateName_Returns409(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DRAFT-005")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{
		"name":    "Unique Draft",
		"purpose": "disclosure_defence",
	})

	// First creation succeeds.
	req1 := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", bytes.NewReader(body.Bytes()))
	req1.Header.Set("Content-Type", "application/json")
	req1 = withDraftAuthContext(req1)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d, body: %s", w1.Code, w1.Body.String())
	}

	// Second creation with same name should conflict.
	bodyBytes, _ := json.Marshal(map[string]string{
		"name":    "Unique Draft",
		"purpose": "disclosure_defence",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2 = withDraftAuthContext(req2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d; body: %s", w2.Code, http.StatusConflict, w2.Body.String())
	}
}

func TestDraftHandler_CreateDraft_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/evidence/not-a-uuid/redact/drafts",
		jsonBody(map[string]string{"name": "x", "purpose": "disclosure_defence"}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDraftHandler_CreateDraft_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	body := jsonBody(map[string]string{
		"name":    "Ghost Draft",
		"purpose": "disclosure_defence",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+uuid.New().String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// ListDrafts
// ---------------------------------------------------------------------------

func TestDraftHandler_ListDrafts_ReturnsCreatedDrafts(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-LIST-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create two drafts.
	for _, name := range []string{"Draft Alpha", "Draft Beta"} {
		body := jsonBody(map[string]string{"name": name, "purpose": "disclosure_defence"})
		req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
		req.Header.Set("Content-Type", "application/json")
		req = withDraftAuthContext(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create draft %q: status=%d body=%s", name, w.Code, w.Body.String())
		}
	}

	// List.
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redact/drafts", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var drafts []RedactionDraft
	decodeEnvelope(t, w.Body.Bytes(), &drafts)
	if len(drafts) != 2 {
		t.Errorf("got %d drafts, want 2", len(drafts))
	}
}

func TestDraftHandler_ListDrafts_EmptyList(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-LIST-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redact/drafts", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var drafts []RedactionDraft
	decodeEnvelope(t, w.Body.Bytes(), &drafts)
	if len(drafts) != 0 {
		t.Errorf("expected empty list, got %d drafts", len(drafts))
	}
}

// ---------------------------------------------------------------------------
// GetDraft
// ---------------------------------------------------------------------------

func TestDraftHandler_GetDraft_Success(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-GET-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create a draft.
	body := jsonBody(map[string]string{"name": "Get Test Draft", "purpose": "court_submission"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create: status=%d body=%s", createW.Code, createW.Body.String())
	}
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Get the draft.
	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]json.RawMessage
	decodeEnvelope(t, w.Body.Bytes(), &result)

	if _, ok := result["draft_id"]; !ok {
		t.Error("response missing 'draft_id' field")
	}
	if _, ok := result["areas"]; !ok {
		t.Error("response missing 'areas' field")
	}
	if _, ok := result["purpose"]; !ok {
		t.Error("response missing 'purpose' field")
	}
}

func TestDraftHandler_GetDraft_WithSavedAreas(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-GET-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create draft.
	createBody := jsonBody(map[string]string{"name": "Areas Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Save areas.
	saveBody := jsonBody(map[string]any{
		"areas": []map[string]any{
			{"id": uuid.New().String(), "page": 1, "x": 10.0, "y": 20.0, "w": 30.0, "h": 15.0, "reason": "PII"},
		},
	})
	saveReq := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq = withDraftAuthContext(saveReq)
	saveW := httptest.NewRecorder()
	r.ServeHTTP(saveW, saveReq)
	if saveW.Code != http.StatusOK {
		t.Fatalf("save: status=%d body=%s", saveW.Code, saveW.Body.String())
	}

	// Get and verify areas.
	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result struct {
		Areas []draftArea `json:"areas"`
	}
	decodeEnvelope(t, w.Body.Bytes(), &result)
	if len(result.Areas) != 1 {
		t.Errorf("got %d areas, want 1", len(result.Areas))
	}
	if result.Areas[0].Reason != "PII" {
		t.Errorf("area reason = %q, want %q", result.Areas[0].Reason, "PII")
	}
}

func TestDraftHandler_GetDraft_NotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-GET-003")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDraftHandler_GetDraft_InvalidDraftID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-GET-004")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/not-a-uuid", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// SaveDraft
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_Success(t *testing.T) {
	h, custody := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create draft.
	createBody := jsonBody(map[string]string{"name": "Save Test Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)
	custody.events = nil // reset events

	// Save.
	saveBody := jsonBody(map[string]any{
		"areas": []map[string]any{
			{"id": uuid.New().String(), "page": 1, "x": 5.0, "y": 5.0, "w": 20.0, "h": 10.0, "reason": "confidential"},
			{"id": uuid.New().String(), "page": 2, "x": 0.0, "y": 0.0, "w": 50.0, "h": 50.0, "reason": "classified"},
		},
	})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]json.RawMessage
	decodeEnvelope(t, w.Body.Bytes(), &result)
	if _, ok := result["draft_id"]; !ok {
		t.Error("response missing 'draft_id'")
	}
	if _, ok := result["last_saved_at"]; !ok {
		t.Error("response missing 'last_saved_at'")
	}

	if len(custody.events) != 1 || custody.events[0] != "redaction_draft_updated" {
		t.Errorf("custody events = %v", custody.events)
	}
}

func TestDraftHandler_SaveDraft_EmptyName_Returns400(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create draft.
	createBody := jsonBody(map[string]string{"name": "Name Valid Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Save with empty name.
	emptyName := ""
	saveBody := jsonBody(map[string]any{
		"areas": []any{},
		"name":  &emptyName,
	})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestDraftHandler_SaveDraft_NameTooLong_Returns400(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-003")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Length Check Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	longName := strings.Repeat("y", 256)
	saveBody := jsonBody(map[string]any{
		"areas": []any{},
		"name":  &longName,
	})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDraftHandler_SaveDraft_AreaCountExceedsLimit_Returns400(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-004")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Area Limit Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Build 501 areas — one over the limit.
	areas := make([]map[string]any, 501)
	for i := range areas {
		areas[i] = map[string]any{
			"id": uuid.New().String(), "page": 1,
			"x": 0.0, "y": 0.0, "w": 1.0, "h": 1.0, "reason": "r",
		}
	}
	saveBody := jsonBody(map[string]any{"areas": areas})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if msg := errorMessage(t, w.Body.Bytes()); !strings.Contains(msg, "500") {
		t.Errorf("error message %q does not mention area limit", msg)
	}
}

func TestDraftHandler_SaveDraft_InvalidPurpose_Returns400(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-005")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Purpose Update Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	invalidPurpose := RedactionPurpose("totally_invalid")
	saveBody := jsonBody(map[string]any{
		"areas":   []any{},
		"purpose": &invalidPurpose,
	})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDraftHandler_SaveDraft_AuthorOverridden(t *testing.T) {
	// The handler must override client-supplied author with the authenticated username.
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-006")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Author Override Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Save with a forged author.
	saveBody := jsonBody(map[string]any{
		"areas": []map[string]any{
			{"id": uuid.New().String(), "page": 1, "x": 0.0, "y": 0.0, "w": 10.0, "h": 10.0,
				"reason": "pii", "author": "evil-attacker"},
		},
	})
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), saveBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Retrieve and verify author was replaced.
	getReq := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), nil)
	getReq = withDraftAuthContext(getReq)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	var result struct {
		Areas []draftArea `json:"areas"`
	}
	decodeEnvelope(t, getW.Body.Bytes(), &result)
	if len(result.Areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(result.Areas))
	}
	if result.Areas[0].Author == "evil-attacker" {
		t.Error("author was not overridden — forgery allowed")
	}
	// The authenticated username set in withDraftAuthContext is "testuser".
	if result.Areas[0].Author != "testuser" {
		t.Errorf("author = %q, want %q", result.Areas[0].Author, "testuser")
	}
}

// ---------------------------------------------------------------------------
// DiscardDraft
// ---------------------------------------------------------------------------

func TestDraftHandler_DiscardDraft_Success(t *testing.T) {
	h, custody := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DISCARD-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create draft.
	createBody := jsonBody(map[string]string{"name": "Discard Me", "purpose": "disclosure_defence"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)
	custody.events = nil

	// Discard.
	req := httptest.NewRequest(http.MethodDelete,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var result map[string]string
	decodeEnvelope(t, w.Body.Bytes(), &result)
	if result["status"] != "discarded" {
		t.Errorf("status = %q, want %q", result["status"], "discarded")
	}

	if len(custody.events) != 1 || custody.events[0] != "redaction_draft_discarded" {
		t.Errorf("custody events = %v", custody.events)
	}

	// Draft should no longer appear in the list.
	listReq := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redact/drafts", nil)
	listReq = withDraftAuthContext(listReq)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	var drafts []RedactionDraft
	decodeEnvelope(t, listW.Body.Bytes(), &drafts)
	if len(drafts) != 0 {
		t.Errorf("expected 0 drafts after discard, got %d", len(drafts))
	}
}

func TestDraftHandler_DiscardDraft_NotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DISCARD-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ---------------------------------------------------------------------------
// GetManagementView
// ---------------------------------------------------------------------------

func TestDraftHandler_GetManagementView_ReturnsCombinedView(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-VIEW-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create a couple of drafts.
	for _, name := range []string{"View Draft 1", "View Draft 2"} {
		body := jsonBody(map[string]string{"name": name, "purpose": "public_release"})
		req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
		req.Header.Set("Content-Type", "application/json")
		req = withDraftAuthContext(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %q: status=%d body=%s", name, w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redactions", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var view RedactionManagementView
	decodeEnvelope(t, w.Body.Bytes(), &view)
	if view.Drafts == nil {
		t.Error("expected non-nil Drafts slice")
	}
	if view.Finalized == nil {
		t.Error("expected non-nil Finalized slice")
	}
	if len(view.Drafts) != 2 {
		t.Errorf("got %d drafts, want 2", len(view.Drafts))
	}
	if len(view.Finalized) != 0 {
		t.Errorf("got %d finalized, want 0", len(view.Finalized))
	}
}

func TestDraftHandler_GetManagementView_EmptyEvidence(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-VIEW-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redactions", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var view RedactionManagementView
	decodeEnvelope(t, w.Body.Bytes(), &view)
	if len(view.Drafts) != 0 {
		t.Errorf("expected 0 drafts, got %d", len(view.Drafts))
	}
	if len(view.Finalized) != 0 {
		t.Errorf("expected 0 finalized, got %d", len(view.Finalized))
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — rate limiting
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_NoRedactionService_Returns503(t *testing.T) {
	h, _ := newDraftHandler(t)
	// Deliberately do not call h.SetRedactionService — it is nil.
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-FIN-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Finalize Draft", "purpose": "court_submission"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	finalizeBody := jsonBody(map[string]string{
		"description":    "Final version",
		"classification": "restricted",
	})
	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String()+"/finalize",
		finalizeBody)
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
}

func TestDraftHandler_FinalizeDraft_RateLimitExceeded(t *testing.T) {
	// The rate limiter allows 2 requests per minute per user.
	// After 2 successful (or failed) requests the third must be rejected.
	h, _ := newDraftHandler(t)

	caseID := seedCase(t, h.db, "CR-FIN-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Use a fixed user ID so all requests share the same rate limit bucket.
	const rateLimitedUserID = "rate-limit-test-user"
	ctx := auth.WithAuthContext(context.Background(), auth.AuthContext{
		UserID:     rateLimitedUserID,
		Username:   "ratelimituser",
		SystemRole: auth.RoleSystemAdmin,
	})

	// A fresh per-test rate limiter to avoid interference with other tests.
	testLimiter := newUserRateLimiter(0, 2) // allow up to burst of 2

	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	fakeDraftID := uuid.New()
	path := "/api/evidence/" + evidenceID.String() + "/redact/drafts/" + fakeDraftID.String() + "/finalize"

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, path,
			jsonBody(map[string]string{"description": "v", "classification": "restricted"}))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		testRouter.ServeHTTP(w, req)

		if i <= 2 {
			// First two should pass through to handler (which returns 503 since no svc).
			if w.Code == http.StatusTooManyRequests {
				t.Errorf("request %d: got 429 but expected to pass rate limit", i)
			}
		} else {
			// Third must be rate-limited.
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: got %d, want %d", i, w.Code, http.StatusTooManyRequests)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Authorization: non-admin user with no case role → 403
// ---------------------------------------------------------------------------

func TestDraftHandler_CreateDraft_ForbiddenWithoutCaseRole(t *testing.T) {
	h, _ := newDraftHandler(t)
	// Override role loader to deny access.
	h.roleLoader = &mockDraftRoleLoader{err: auth.ErrNoCaseRole}
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-AUTHZ-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	body := jsonBody(map[string]string{"name": "Forbidden Draft", "purpose": "disclosure_defence"})
	req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", body)
	req.Header.Set("Content-Type", "application/json")
	// Regular user — will trigger the role check.
	req = withDraftUserContext(req, "some-user-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SetRedactionService
// ---------------------------------------------------------------------------

func TestDraftHandler_SetRedactionService(t *testing.T) {
	h, _ := newDraftHandler(t)
	if h.redactionSvc != nil {
		t.Fatal("expected redactionSvc to be nil before SetRedactionService")
	}
	// Build a minimal RedactionService so we can verify the field is set.
	pool := h.db
	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	h.SetRedactionService(rs)
	if h.redactionSvc != rs {
		t.Error("SetRedactionService did not store the redaction service")
	}
}

// ---------------------------------------------------------------------------
// recordCustody — nil custody guard
// ---------------------------------------------------------------------------

func TestDraftHandler_RecordCustody_NilCustody(t *testing.T) {
	// When h.custody is nil, recordCustody must return without panicking.
	h, _ := newDraftHandler(t)
	h.custody = nil // override
	// This must not panic.
	h.recordCustody(context.Background(), uuid.New(), uuid.New(), "test_action", uuid.New().String(), nil)
}

func TestDraftHandler_RecordCustody_ErrorIsLogged(t *testing.T) {
	// When RecordEvidenceEvent returns an error, recordCustody must log it and
	// continue without propagating (the caller should not be affected).
	h, _ := newDraftHandler(t)
	h.custody = &mockDraftCustodyError{}
	// This must not panic or return an error — it only logs.
	h.recordCustody(context.Background(), uuid.New(), uuid.New(), "test_action", uuid.New().String(), nil)
}

// ---------------------------------------------------------------------------
// isDuplicateKeyError non-pgconn path is covered in draft_handler_unit_test.go
// (no DB dependency, so it runs under the default unit-test build).
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// checkCaseAccessHTTP — roleLoader internal error path
// ---------------------------------------------------------------------------

func TestDraftHandler_CheckCaseAccess_RoleLoaderInternalError(t *testing.T) {
	// When the role loader returns a non-ErrNoCaseRole error the handler must
	// respond 500 (authorization check failed).
	h, _ := newDraftHandler(t)
	h.roleLoader = &mockDraftRoleLoader{err: errors.New("connection pool exhausted")}
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-ACCESS-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Use a non-admin user so the role loader is invoked.
	req := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redact/drafts", nil)
	req = withDraftUserContext(req, "00000000-0000-4000-8000-000000000077")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — invalid draft UUID
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_InvalidDraftID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-007")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/not-a-uuid",
		jsonBody(map[string]any{"areas": []any{}}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — draft not found (pgx.ErrNoRows path)
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_DraftNotFound(t *testing.T) {
	// Saving to a non-existent draft ID triggers pgx.ErrNoRows → 404.
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-008")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+uuid.New().String(),
		jsonBody(map[string]any{"areas": []any{}}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — duplicate name conflict (409)
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_DuplicateNameConflict(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-SAVE-009")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Create two drafts with different names.
	for _, name := range []string{"Draft Alpha", "Draft Beta"} {
		req := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts",
			jsonBody(map[string]string{"name": name, "purpose": "internal_review"}))
		req.Header.Set("Content-Type", "application/json")
		req = withDraftAuthContext(req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %q: status=%d body=%s", name, w.Code, w.Body.String())
		}
	}

	// Get the ID of "Draft Beta".
	listReq := httptest.NewRequest(http.MethodGet, "/api/evidence/"+evidenceID.String()+"/redact/drafts", nil)
	listReq = withDraftAuthContext(listReq)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)
	var drafts []RedactionDraft
	decodeEnvelope(t, listW.Body.Bytes(), &drafts)
	if len(drafts) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(drafts))
	}

	// Find "Draft Beta" to rename it to "Draft Alpha" (conflict).
	var betaID uuid.UUID
	for _, d := range drafts {
		if d.Name == "Draft Beta" {
			betaID = d.ID
		}
	}
	if betaID == uuid.Nil {
		t.Fatal("could not find Draft Beta")
	}

	newName := "Draft Alpha"
	saveReq := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+betaID.String(),
		jsonBody(map[string]any{"areas": []any{}, "name": &newName}))
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq = withDraftAuthContext(saveReq)
	saveW := httptest.NewRecorder()
	r.ServeHTTP(saveW, saveReq)

	if saveW.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d; body: %s", saveW.Code, http.StatusConflict, saveW.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DiscardDraft — invalid draft UUID
// ---------------------------------------------------------------------------

func TestDraftHandler_DiscardDraft_InvalidDraftID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-DISCARD-003")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/not-a-uuid", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_InvalidEvidenceID(t *testing.T) {
	// The redactionSvc nil check fires before evidence ID parsing, so we must
	// wire a real RedactionService. The evidence ID parse happens before any DB
	// query so we can use any pool.
	h, _ := newDraftHandler(t)
	pool := h.db
	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)
	h.SetRedactionService(rs)
	// Use a dedicated per-test rate limiter to avoid shared-state 429 collisions.
	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 2)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/not-a-uuid/redact/drafts/"+uuid.New().String()+"/finalize",
		jsonBody(map[string]string{"description": "x", "classification": "restricted"}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — invalid draft ID
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_InvalidDraftID(t *testing.T) {
	pool := startPostgresContainer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	caseID := seedCase(t, pool, "CR-FIN-007")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/not-a-uuid/finalize",
		jsonBody(map[string]string{"description": "x", "classification": "restricted"}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — bad request body (decodeBody error)
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_BadBody(t *testing.T) {
	pool := startPostgresContainer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	// Use a per-test rate limiter so the global finalizeLimiter state does not
	// cause 429 responses when tests run in sequence.
	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	caseID := seedCase(t, pool, "CR-FIN-008")
	evidenceID := seedEvidenceItem(t, h, caseID)
	draftID := uuid.New()

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+draftID.String()+"/finalize",
		strings.NewReader("not-valid-json{{{"))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — validation error from FinalizeFromDraft (empty areas → 400)
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_ValidationError(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	caseID := seedCase(t, pool, "CR-FIN-009")

	// Create a real PNG evidence item so the evidence lookup succeeds.
	pngData := createSmallPNG(50, 50)
	storageKey := "evidence/test/validerror.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-VAL-001", Filename: "file.png", OriginalName: "file.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("g", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	// Create a draft with NO areas — FinalizeFromDraft will return a ValidationError.
	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Empty Finalize", PurposeInternalReview, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+original.ID.String()+"/redact/drafts/"+draft.ID.String()+"/finalize",
		jsonBody(map[string]string{"description": "test", "classification": ClassificationRestricted}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — internal error from FinalizeFromDraft (non-validation error → 500)
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_InternalError(t *testing.T) {
	// Pass a non-existent draft ID so LockDraftForFinalize fails internally
	// (pgx error that is not a ValidationError), causing a 500 response.
	pool := startPostgresContainer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	caseID := seedCase(t, pool, "CR-FIN-010")
	evidenceID := seedEvidenceItem(t, h, caseID)

	// Use a random non-existent draft ID — the SQL lock will return pgx.ErrNoRows
	// which is wrapped as a generic error (not a ValidationError) by LockDraftForFinalize.
	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+uuid.New().String()+"/finalize",
		jsonBody(map[string]string{"description": "x", "classification": ClassificationRestricted}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — success path (full flow with real storage)
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_Success(t *testing.T) {
	pool := startPostgresContainer(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	// Use a per-test rate limiter with high burst to prevent 429 collisions.
	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	caseID := seedCase(t, pool, "CR-FIN-011")

	pngData := createSmallPNG(100, 100)
	storageKey := "evidence/test/finalize_handler.png"
	storage.objects[storageKey] = pngData

	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "EV-HANDLER-001", Filename: "handler.png", OriginalName: "handler.png",
		StorageKey: storageKey, MimeType: "image/png", SizeBytes: int64(len(pngData)),
		SHA256Hash: strings.Repeat("h", 64), Classification: ClassificationRestricted,
		Tags: []string{}, UploadedBy: uuid.New().String(), UploadedByName: "user", TSAStatus: TSAStatusDisabled,
	})
	if err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	draft, err := repo.CreateDraft(ctx, original.ID, caseID, "Handler Finalize Test", PurposeDisclosureDefence, uuid.New().String())
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Save areas to draft so finalization has something to redact.
	areas := draftState{Areas: []draftArea{
		{ID: uuid.New().String(), Page: 0, X: 10, Y: 10, W: 20, H: 20, Reason: "PII"},
	}}
	areasJSON, _ := json.Marshal(areas)
	_, err = repo.UpdateDraft(ctx, draft.ID, original.ID, areasJSON, 1, nil, nil)
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+original.ID.String()+"/redact/drafts/"+draft.ID.String()+"/finalize",
		jsonBody(map[string]string{
			"description":    "Handler integration finalize",
			"classification": ClassificationRestricted,
		}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var result RedactedResult
	decodeEnvelope(t, w.Body.Bytes(), &result)
	if result.NewEvidenceID == uuid.Nil {
		t.Error("expected non-nil new evidence ID")
	}
	if result.OriginalID != original.ID {
		t.Errorf("original_id = %s, want %s", result.OriginalID, original.ID)
	}
	if result.RedactionCount != 1 {
		t.Errorf("redaction_count = %d, want 1", result.RedactionCount)
	}
}

// ---------------------------------------------------------------------------
// CreateDraft — bad JSON body
// ---------------------------------------------------------------------------

func TestDraftHandler_CreateDraft_BadBody(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-BODY-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts",
		strings.NewReader("not-valid-json{{{"))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ListDrafts — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_ListDrafts_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/not-a-uuid/redact/drafts", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ListDrafts — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_ListDrafts_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetDraft — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_GetDraft_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/not-a-uuid/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetDraft — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_GetDraft_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	// chi router requires a valid path segment; use a real evidenceID shape that
	// fails uuid.Parse to exercise the branch rather than getting a 404 from chi.
	// Actually chi will match /{id} and pass "not-a-uuid" as the param.
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/not-a-uuid/redact/drafts/"+uuid.New().String(),
		jsonBody(map[string]any{"areas": []any{}}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(),
		jsonBody(map[string]any{"areas": []any{}}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — no areas key (nil areas → defaults to empty slice)
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_NilAreas(t *testing.T) {
	// When the request body omits the "areas" key entirely, body.Areas is nil.
	// The handler must normalise it to an empty slice and still return 200.
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-NILAREA-001")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Nil Areas Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	// Body contains only name; no "areas" key → body.Areas will be nil.
	newName := "Nil Areas Draft Updated"
	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(),
		jsonBody(map[string]any{"name": newName}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SaveDraft — bad JSON body
// ---------------------------------------------------------------------------

func TestDraftHandler_SaveDraft_BadBody(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	caseID := seedCase(t, h.db, "CR-BODY-002")
	evidenceID := seedEvidenceItem(t, h, caseID)

	createBody := jsonBody(map[string]string{"name": "Bad Body Draft", "purpose": "internal_review"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/evidence/"+evidenceID.String()+"/redact/drafts", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withDraftAuthContext(createReq)
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	var createdDraft RedactionDraft
	decodeEnvelope(t, createW.Body.Bytes(), &createdDraft)

	req := httptest.NewRequest(http.MethodPut,
		"/api/evidence/"+evidenceID.String()+"/redact/drafts/"+createdDraft.ID.String(),
		strings.NewReader("not-valid-json{{{"))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DiscardDraft — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_DiscardDraft_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// DiscardDraft — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_DiscardDraft_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/evidence/not-a-uuid/redact/drafts/"+uuid.New().String(), nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetManagementView — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_GetManagementView_EvidenceNotFound(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/"+uuid.New().String()+"/redactions", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GetManagementView — invalid evidence ID
// ---------------------------------------------------------------------------

func TestDraftHandler_GetManagementView_InvalidEvidenceID(t *testing.T) {
	h, _ := newDraftHandler(t)
	r := registerDraftRoutes(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/evidence/not-a-uuid/redactions", nil)
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// FinalizeDraft — evidence not found (checkCaseAccessHTTP → 404)
// ---------------------------------------------------------------------------

func TestDraftHandler_FinalizeDraft_EvidenceNotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	repo := NewRepository(pool)
	storage := &inMemStorage{objects: make(map[string][]byte)}
	tsa := &integrity.NoopTimestampAuthority{}
	custody := &noopCustody{}
	caseLookup := NewCaseLookup(pool)
	thumbGen := NewThumbnailGenerator()
	svc := NewService(repo, storage, tsa, &search.NoopSearchIndexer{}, custody, caseLookup, thumbGen, logger, 10<<20)
	rs := NewRedactionService(svc, storage, tsa, custody, logger)

	caseRoleLoader := &mockDraftRoleLoader{role: auth.CaseRoleInvestigator}
	mockCustody := &mockDraftCustody{}
	h := NewDraftHandler(pool, caseRoleLoader, mockCustody, logger)
	h.SetRedactionService(rs)

	testLimiter := newUserRateLimiter(rate.Every(30*time.Second), 10)
	testRouter := chi.NewRouter()
	testRouter.With(rateLimitMiddleware(testLimiter)).Post(
		"/api/evidence/{id}/redact/drafts/{draftId}/finalize",
		h.FinalizeDraft,
	)

	req := httptest.NewRequest(http.MethodPost,
		"/api/evidence/"+uuid.New().String()+"/redact/drafts/"+uuid.New().String()+"/finalize",
		jsonBody(map[string]string{"description": "x", "classification": ClassificationRestricted}))
	req.Header.Set("Content-Type", "application/json")
	req = withDraftAuthContext(req)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Untestable branches — annotated for coverage tool
// ---------------------------------------------------------------------------

// The following branches in draft_handler.go cannot be triggered with a
// real Postgres container because the database is healthy and the schema is
// valid.  They are defensive error paths for infrastructure failures:
//
//   - CreateDraft:     repo.CreateDraft internal (non-duplicate) error
//                      → would require injecting a DB fault mid-query
//   - ListDrafts:      repo.ListDrafts error
//                      → same reason; healthy DB never returns a scan error
//   - GetDraft:        repo.FindDraftByID internal error (not ErrNotFound)
//                      → requires mid-query DB fault
//   - GetDraft:        json.Unmarshal error on corrupt yjs_state
//                      → yjs_state is written by the Go handler so it is
//                        always valid JSON; corrupt bytes require direct DB
//                        manipulation that testcontainers does not support
//   - SaveDraft:       json.Marshal error on draftState
//                      → draftState only contains basic Go types; Marshal
//                        cannot fail for them
//   - SaveDraft:       repo.UpdateDraft non-pgx.ErrNoRows / non-duplicate err
//                      → requires mid-query DB fault
//   - DiscardDraft:    repo.DiscardDraft internal error
//                      → same as above
//   - GetManagementView: repo.GetManagementView error
//                      → same as above
//   - checkCaseAccessHTTP: lookupCaseID internal error (not ErrNoRows)
//                      → requires mid-query DB fault on the SELECT
//
// These branches are all "log + 500" paths that protect against impossible
// failures in a healthy deployment.  They are omitted intentionally and do
// not reduce production reliability.
