package evidence

// Regression tests for the Sprint 9 GDPR handler. In particular,
// TestCreateErasureRequest_IgnoresBodyRequestedBy and
// TestResolveErasureRequest_IgnoresBodyDecidedBy guard against re-
// introducing the bug where the caller-supplied requested_by / decided_by
// field was passed through to the custody log. The custody_log.actor_user_id
// column is a UUID, so any non-UUID body value caused an
// "invalid input syntax for type uuid" insert failure and also created an
// audit-trail impersonation vector.

import (
	"bytes"
	"context"
	"encoding/json"
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

// actorTrackingCustody captures the actorID passed to every custody event
// so tests can assert that the HTTP handler always records the
// authenticated session UUID and never a body-supplied string.
type actorTrackingCustody struct {
	events []string
	actors []string
}

func (m *actorTrackingCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, actor string, _ map[string]string) error {
	m.events = append(m.events, action)
	m.actors = append(m.actors, actor)
	return nil
}

// newGDPRTestHandler wires a Handler backed by in-memory fakes and the
// Sprint 9 erasure repository so the GDPR HTTP routes can be exercised
// without a real database.
func newGDPRTestHandler(t *testing.T) (*Handler, *mockRepo, *fakeErasureRepo, *actorTrackingCustody) {
	t.Helper()
	repo := newMockRepo()
	storage := newMockStorage()
	custody := &actorTrackingCustody{}
	caseLookup := &mockCaseLookup{status: "archived"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	erasureRepo := newFakeErasureRepo()

	svc := NewService(
		repo, storage, &integrity.NoopTimestampAuthority{},
		&search.NoopSearchIndexer{}, custody, caseLookup,
		&noopThumbGen{}, logger, 100*1024*1024,
	)
	svc.WithErasureRepo(erasureRepo)

	handler := NewHandler(svc, &mockCustodyReader{}, &mockAudit{}, 100*1024*1024)
	return handler, repo, erasureRepo, custody
}

// withUUIDAuthContext attaches an auth context whose UserID is the exact
// string the custody log needs. Distinct from handler_test.go's
// withAuthContext which uses "test-user" — the GDPR path requires a UUID.
func withUUIDAuthContext(r *http.Request, userID string) *http.Request {
	ctx := auth.WithAuthContext(r.Context(), auth.AuthContext{
		UserID:     userID,
		Username:   "ignored.username",
		SystemRole: auth.RoleSystemAdmin,
	})
	return r.WithContext(ctx)
}

// TestCreateErasureRequest_IgnoresBodyRequestedBy verifies that a client
// cannot impersonate another actor in the custody audit trail by sending
// a `requested_by` field in the request body. The handler MUST use
// ac.UserID regardless of what the body claims.
func TestCreateErasureRequest_IgnoresBodyRequestedBy(t *testing.T) {
	handler, repo, erasureRepo, custody := newGDPRTestHandler(t)

	// Seed an item.
	itemID := uuid.New()
	caseID := uuid.New()
	repo.items[itemID] = EvidenceItem{
		ID:             itemID,
		CaseID:         caseID,
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}

	sessionUser := uuid.New().String() // valid UUID for the authenticated session
	bodyAttacker := `{"requested_by":"attacker@evil","rationale":"subject rights request"}`

	r := chi.NewRouter()
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Post("/erasure-requests", handler.CreateErasureRequest)
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/evidence/"+itemID.String()+"/erasure-requests",
		strings.NewReader(bodyAttacker),
	)
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, sessionUser)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body: %s", w.Code, w.Body.String())
	}

	// Exactly one erasure request should have been persisted.
	if len(erasureRepo.reqs) != 1 {
		t.Fatalf("erasureRepo.reqs = %d, want 1", len(erasureRepo.reqs))
	}
	var persisted ErasureRequest
	for _, r := range erasureRepo.reqs {
		persisted = r
		break
	}
	if persisted.RequestedBy != sessionUser {
		t.Errorf("persisted.RequestedBy = %q, want %q (session UUID, NOT body.requested_by)", persisted.RequestedBy, sessionUser)
	}
	if persisted.RequestedBy == "attacker@evil" {
		t.Errorf("persisted.RequestedBy leaked attacker body value")
	}

	// Custody event must also be recorded as the session user, not the body string.
	if len(custody.events) == 0 {
		t.Fatal("expected custody event, got none")
	}
	for _, actor := range custody.actors {
		if actor == "attacker@evil" {
			t.Error("custody event recorded attacker body value as actor")
		}
		if actor != sessionUser {
			t.Errorf("custody actor = %q, want %q", actor, sessionUser)
		}
	}
}

// TestResolveErasureRequest_IgnoresBodyDecidedBy is the mirror regression
// test for the resolve endpoint.
func TestResolveErasureRequest_IgnoresBodyDecidedBy(t *testing.T) {
	handler, repo, erasureRepo, custody := newGDPRTestHandler(t)

	itemID := uuid.New()
	caseID := uuid.New()
	repo.items[itemID] = EvidenceItem{
		ID:             itemID,
		CaseID:         caseID,
		Classification: ClassificationRestricted,
		Tags:           []string{},
	}

	// Seed a pending erasure request directly on the fake so resolve has
	// something to close.
	reqID := uuid.New()
	seed := ErasureRequest{
		ID:          reqID,
		EvidenceID:  itemID,
		RequestedBy: "original-requester-uuid",
		Rationale:   "data subject request",
		Status:      ErasureStatusConflictPending,
	}
	if _, err := erasureRepo.CreateErasureRequest(context.Background(), seed); err != nil {
		t.Fatalf("seed erasure request: %v", err)
	}

	sessionUser := uuid.New().String()
	bodyAttacker := `{"decision":"preserve","decided_by":"attacker@evil","rationale":"retaining per legal"}`

	r := chi.NewRouter()
	r.Post("/api/erasure-requests/{id}/resolve", handler.ResolveErasureRequest)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/erasure-requests/"+reqID.String()+"/resolve",
		bytes.NewBufferString(bodyAttacker),
	)
	req.Header.Set("Content-Type", "application/json")
	req = withUUIDAuthContext(req, sessionUser)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body: %s", w.Code, w.Body.String())
	}

	// The persisted decision must carry the session UUID, not the body field.
	resolved, err := erasureRepo.FindErasureRequest(context.Background(), reqID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if resolved.DecidedBy == nil {
		t.Fatal("DecidedBy not set on resolved request")
	}
	if *resolved.DecidedBy != sessionUser {
		t.Errorf("DecidedBy = %q, want %q", *resolved.DecidedBy, sessionUser)
	}
	if *resolved.DecidedBy == "attacker@evil" {
		t.Error("DecidedBy leaked attacker body value")
	}

	for _, actor := range custody.actors {
		if actor == "attacker@evil" {
			t.Error("custody event recorded attacker body value as actor")
		}
	}
}

// Compile-time guard — ensure we don't accidentally drop the body struct
// fields (they stay for backwards compat but are ignored).
var _ = createErasureRequestBody{RequestedBy: "", Rationale: ""}
var _ = resolveErasureRequestBody{Decision: "", DecidedBy: "", Rationale: ""}

// Ensure io is not accidentally removed by future edits.
var _ io.Writer = io.Discard

// json imported for test payload consistency even if not used directly.
var _ = json.Marshal

// TestGDPRRouteRegistrar_MountsBothRoutes verifies that RegisterRoutes
// actually mounts the two erasure endpoints on the chi router. Without
// this the wiring could silently break and the handlers would become
// unreachable from HTTP clients.
func TestGDPRRouteRegistrar_MountsBothRoutes(t *testing.T) {
	handler, _, _, _ := newGDPRTestHandler(t)
	registrar := &GDPRRouteRegistrar{Handler: handler, Audit: &mockAudit{}}

	r := chi.NewRouter()
	registrar.RegisterRoutes(r)

	// Check both routes resolve to something (no 404 in the routing layer).
	// We use a non-UUID id so the handlers short-circuit with 400 — the key
	// assertion is that the status is NOT 404 (route not found).
	checks := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/evidence/not-a-uuid/erasure-requests"},
		{http.MethodPost, "/api/erasure-requests/not-a-uuid/resolve"},
	}
	for _, c := range checks {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req = withUUIDAuthContext(req, uuid.New().String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s: got 404, route not mounted", c.method, c.path)
		}
	}
}
