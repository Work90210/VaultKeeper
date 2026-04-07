package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// --- Handler test helpers ---

// handlerMockPool supports the exact sequence of pool calls each handler makes.
type handlerMockPool struct {
	// For List handler: QueryRow (count) then Query (items)
	countResult int
	countErr    error
	listRows    [][]any
	listErr     error

	// For UnreadCount handler: QueryRow
	unreadResult int
	unreadErr    error

	// For MarkRead handler: Exec
	markReadTag pgconn.CommandTag
	markReadErr error

	// For MarkAllRead handler: Exec
	markAllReadTag pgconn.CommandTag
	markAllReadErr error

	// Track which handler context we're in
	queryRowCalls int
}

func (p *handlerMockPool) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	p.queryRowCalls++
	return &pgxRow{scanFunc: func(dest ...any) error {
		if p.countErr != nil {
			return p.countErr
		}
		if p.unreadErr != nil {
			return p.unreadErr
		}
		if ptr, ok := dest[0].(*int); ok {
			if p.unreadResult > 0 {
				*ptr = p.unreadResult
			} else {
				*ptr = p.countResult
			}
		}
		return nil
	}}
}

func (p *handlerMockPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	return &pgxRows{data: p.listRows}, nil
}

func (p *handlerMockPool) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	if p.markReadErr != nil {
		return p.markReadTag, p.markReadErr
	}
	if p.markAllReadErr != nil {
		return p.markAllReadTag, p.markAllReadErr
	}
	// Return whichever tag is set
	if p.markReadTag.String() != "" {
		return p.markReadTag, nil
	}
	return p.markAllReadTag, nil
}

func newTestHandler(pool dbPool) *Handler {
	repo := &Repository{pool: pool}
	svc := NewService(repo, noopEmailer(), nil, noopLogger(), nil)
	return NewHandler(svc)
}

func withAuth(ctx context.Context, userID string) context.Context {
	return auth.WithAuthContext(ctx, auth.AuthContext{
		UserID: userID,
	})
}

// --- List tests ---

func TestHandler_List(t *testing.T) {
	now := time.Now().UTC()
	uid := uuid.New()
	nid := uuid.New()

	pool := &handlerMockPool{
		countResult: 1,
		listRows: [][]any{
			{nid, (*uuid.UUID)(nil), uid, EventEvidenceUploaded, "Title", "Body", false, now},
		},
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/?limit=10", nil)
	req = req.WithContext(withAuth(req.Context(), uid.String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []Notification `json:"data"`
		Meta struct {
			Total   int  `json:"total"`
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Meta.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Meta.Total)
	}
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 notification, got %d", len(resp.Data))
	}
}

func TestHandler_List_NoAuth(t *testing.T) {
	pool := &handlerMockPool{}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing auth, got %d", w.Code)
	}
}

func TestHandler_List_ServiceError(t *testing.T) {
	pool := &handlerMockPool{
		countErr: errors.New("db error"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for service error, got %d", w.Code)
	}
}

func TestHandler_List_DefaultLimit(t *testing.T) {
	pool := &handlerMockPool{
		countResult: 0,
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// No limit param - should use default 25
	req := httptest.NewRequest(http.MethodGet, "/api/notifications/", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_List_HasMore(t *testing.T) {
	now := time.Now().UTC()
	uid := uuid.New()

	// Return exactly `limit` items to trigger hasMore=true
	// limit=2, so return 2 items (service requests limit+1=3 from repo, repo returns 2 which equals limit)
	rows := make([][]any, 2)
	for i := 0; i < 2; i++ {
		rows[i] = []any{uuid.New(), (*uuid.UUID)(nil), uid, EventEvidenceUploaded, "Title", "Body", false, now}
	}

	pool := &handlerMockPool{
		countResult: 10,
		listRows:    rows,
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/?limit=2", nil)
	req = req.WithContext(withAuth(req.Context(), uid.String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []Notification `json:"data"`
		Meta struct {
			Total      int    `json:"total"`
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Meta.HasMore {
		t.Error("expected has_more=true")
	}
	if resp.Meta.NextCursor == "" {
		t.Error("expected non-empty next_cursor")
	}
}

func TestHandler_List_InvalidLimit(t *testing.T) {
	pool := &handlerMockPool{
		countResult: 0,
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Invalid limit is ignored, default used
	req := httptest.NewRequest(http.MethodGet, "/api/notifications/?limit=abc", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- UnreadCount tests ---

func TestHandler_UnreadCount(t *testing.T) {
	pool := &handlerMockPool{unreadResult: 5}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			UnreadCount int `json:"unread_count"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Data.UnreadCount != 5 {
		t.Errorf("expected 5 unread, got %d", resp.Data.UnreadCount)
	}
}

func TestHandler_UnreadCount_NoAuth(t *testing.T) {
	pool := &handlerMockPool{}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing auth, got %d", w.Code)
	}
}

func TestHandler_UnreadCount_ServiceError(t *testing.T) {
	pool := &handlerMockPool{unreadErr: errors.New("db error")}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- MarkRead tests ---

func TestHandler_MarkRead(t *testing.T) {
	pool := &handlerMockPool{
		markReadTag: pgconn.NewCommandTag("UPDATE 1"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	notifID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/notifications/"+notifID.String()+"/read", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MarkRead_NotFound_Returns403(t *testing.T) {
	pool := &handlerMockPool{
		markReadTag: pgconn.NewCommandTag("UPDATE 0"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	notifID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/notifications/"+notifID.String()+"/read", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MarkRead_InvalidUUID(t *testing.T) {
	pool := &handlerMockPool{}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPatch, "/api/notifications/not-a-uuid/read", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MarkRead_NoAuth(t *testing.T) {
	pool := &handlerMockPool{}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	notifID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/notifications/"+notifID.String()+"/read", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing auth, got %d", w.Code)
	}
}

func TestHandler_MarkRead_ServiceError(t *testing.T) {
	pool := &handlerMockPool{
		markReadErr: errors.New("db error"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	notifID := uuid.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/notifications/"+notifID.String()+"/read", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- MarkAllRead tests ---

func TestHandler_MarkAllRead(t *testing.T) {
	pool := &handlerMockPool{
		markAllReadTag: pgconn.NewCommandTag("UPDATE 3"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_MarkAllRead_NoAuth(t *testing.T) {
	pool := &handlerMockPool{}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing auth, got %d", w.Code)
	}
}

func TestHandler_MarkAllRead_ServiceError(t *testing.T) {
	pool := &handlerMockPool{
		markAllReadErr: errors.New("db error"),
	}
	h := newTestHandler(pool)

	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	req = req.WithContext(withAuth(req.Context(), uuid.New().String()))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
