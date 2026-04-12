package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// userRateLimiter provides per-user rate limiting.
type userRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func newUserRateLimiter(r rate.Limit, burst int) *userRateLimiter {
	return &userRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    burst,
	}
}

func (u *userRateLimiter) allow(userID string) bool {
	u.mu.Lock()
	lim, ok := u.limiters[userID]
	if !ok {
		lim = rate.NewLimiter(u.rate, u.burst)
		u.limiters[userID] = lim
	}
	u.mu.Unlock()
	return lim.Allow()
}

// rateLimitMiddleware rejects requests that exceed the per-user rate limit.
func rateLimitMiddleware(limiter *userRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.GetAuthContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.allow(ac.UserID) {
				httputil.RespondError(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Finalize rate limiter: 2 requests per minute per user.
var finalizeLimiter = newUserRateLimiter(rate.Every(30*time.Second), 2)

// draftHandlerJSONMarshal is the json.Marshal indirection used by SaveDraft
// so unit tests can force the otherwise-unreachable marshal-error branch.
var draftHandlerJSONMarshal = json.Marshal

// DraftHandler provides HTTP endpoints for multi-draft redaction CRUD.
type DraftHandler struct {
	db           dbPool
	repo         *PGRepository
	roleLoader   CaseRoleChecker
	redactionSvc *RedactionService
	custody      CustodyRecorder
	logger       *slog.Logger
}

// NewDraftHandler creates a new redaction draft HTTP handler.
func NewDraftHandler(db *pgxpool.Pool, roleLoader CaseRoleChecker, custody CustodyRecorder, logger *slog.Logger) *DraftHandler {
	return newDraftHandlerFromPool(db, roleLoader, custody, logger)
}

// newDraftHandlerFromPool constructs a DraftHandler from an injected
// dbPool — used by unit tests to wire a mock pool alongside production
// code that passes a real *pgxpool.Pool through NewDraftHandler.
func newDraftHandlerFromPool(db dbPool, roleLoader CaseRoleChecker, custody CustodyRecorder, logger *slog.Logger) *DraftHandler {
	return &DraftHandler{
		db:         db,
		repo:       &PGRepository{pool: db},
		roleLoader: roleLoader,
		custody:    custody,
		logger:     logger,
	}
}

// SetRedactionService sets the redaction service for finalization.
func (h *DraftHandler) SetRedactionService(rs *RedactionService) {
	h.redactionSvc = rs
}

// RegisterRoutes mounts multi-draft redaction routes on the given router.
func (h *DraftHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/evidence/{id}/redact/drafts", func(r chi.Router) {
		r.Post("/", h.CreateDraft)
		r.Get("/", h.ListDrafts)
		r.Get("/{draftId}", h.GetDraft)
		r.Put("/{draftId}", h.SaveDraft)
		r.Delete("/{draftId}", h.DiscardDraft)
	})

	r.With(rateLimitMiddleware(finalizeLimiter)).Post("/api/evidence/{id}/redact/drafts/{draftId}/finalize", h.FinalizeDraft)
	r.Get("/api/evidence/{id}/redactions", h.GetManagementView)
}

// draftArea represents a single redaction area within a draft.
type draftArea struct {
	ID     string  `json:"id"`
	Page   int     `json:"page"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Reason string  `json:"reason"`
	Author string  `json:"author"`
}

// draftState is the JSON structure stored in the yjs_state column.
type draftState struct {
	Areas []draftArea `json:"areas"`
}

// CreateDraft creates a new named draft for an evidence item.
func (h *DraftHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	caseID, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac)
	if !ok2 {
		return
	}

	var body struct {
		Name    string           `json:"name"`
		Purpose RedactionPurpose `json:"purpose"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		httputil.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 255 {
		httputil.RespondError(w, http.StatusBadRequest, "name must be 255 characters or less")
		return
	}
	if !ValidPurposes[body.Purpose] {
		httputil.RespondError(w, http.StatusBadRequest, "invalid purpose")
		return
	}

	draft, err := h.repo.CreateDraft(ctx, evidenceID, caseID, body.Name, body.Purpose, ac.UserID)
	if err != nil {
		if isDuplicateKeyError(err) {
			httputil.RespondError(w, http.StatusConflict, "a draft with this name already exists")
			return
		}
		// unreachable: post-access-check repo failure requires mid-query DB fault;
		// both lookupCaseID and CreateDraft use the same pool, so closing it prevents
		// the access check from passing in the first place.
		h.logger.Error("create draft failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.recordCustody(ctx, caseID, evidenceID, "redaction_draft_created", ac.UserID, map[string]string{
		"draft_id": draft.ID.String(),
		"name":     draft.Name,
		"purpose":  string(draft.Purpose),
	})

	httputil.RespondJSON(w, http.StatusCreated, draft)
}

// ListDrafts returns all non-discarded drafts for an evidence item.
func (h *DraftHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	drafts, err := h.repo.ListDrafts(ctx, evidenceID)
	if err != nil { // unreachable: same pool constraint as CreateDraft — see comment above
		h.logger.Error("list drafts failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, drafts)
}

// GetDraft loads a specific draft with its areas.
func (h *DraftHandler) GetDraft(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	draftID, err := uuid.Parse(chi.URLParam(r, "draftId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid draft ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	draft, yjsState, err := h.repo.FindDraftByID(ctx, draftID, evidenceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "draft not found")
			return
		}
		// unreachable: same pool constraint — post-access-check DB fault
		h.logger.Error("get draft failed", "draft_id", draftID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var state draftState
	if len(yjsState) > 0 {
		if jsonErr := json.Unmarshal(yjsState, &state); jsonErr != nil {
			// unreachable: yjs_state is written by this handler as valid JSON;
			// only a direct-DB UPDATE injecting corrupt bytes would reach here.
			h.logger.Error("unmarshal draft state failed", "draft_id", draftID, "error", jsonErr)
			httputil.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	if state.Areas == nil {
		state.Areas = []draftArea{}
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"draft_id":      draft.ID.String(),
		"name":          draft.Name,
		"purpose":       draft.Purpose,
		"areas":         state.Areas,
		"area_count":    draft.AreaCount,
		"last_saved_at": draft.LastSavedAt,
	})
}

// SaveDraft auto-saves areas and optionally updates name/purpose.
func (h *DraftHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	draftID, err := uuid.Parse(chi.URLParam(r, "draftId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid draft ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	var body struct {
		Areas   []draftArea       `json:"areas"`
		Name    *string           `json:"name,omitempty"`
		Purpose *RedactionPurpose `json:"purpose,omitempty"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			httputil.RespondError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		if len(trimmed) > 255 {
			httputil.RespondError(w, http.StatusBadRequest, "name must be 255 characters or less")
			return
		}
		body.Name = &trimmed
	}
	if body.Purpose != nil && !ValidPurposes[*body.Purpose] {
		httputil.RespondError(w, http.StatusBadRequest, "invalid purpose")
		return
	}

	if len(body.Areas) > 500 {
		httputil.RespondError(w, http.StatusBadRequest, "too many redaction areas (max 500)")
		return
	}

	// Override client-supplied author with the authenticated user to prevent forgery
	for i := range body.Areas {
		body.Areas[i].Author = ac.Username
	}

	state := draftState{Areas: body.Areas}
	if state.Areas == nil {
		state.Areas = []draftArea{}
	}

	stateBytes, err := draftHandlerJSONMarshal(state)
	if err != nil {
		h.logger.Error("marshal draft state failed", "draft_id", draftID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	draft, err := h.repo.UpdateDraft(ctx, draftID, evidenceID, stateBytes, len(body.Areas), body.Name, body.Purpose)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.RespondError(w, http.StatusNotFound, "draft not found or already finalized")
			return
		}
		if isDuplicateKeyError(err) {
			httputil.RespondError(w, http.StatusConflict, "a draft with this name already exists")
			return
		}
		// unreachable: same pool constraint — post-access-check DB fault
		h.logger.Error("save draft failed", "draft_id", draftID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.recordCustody(ctx, draft.CaseID, evidenceID, "redaction_draft_updated", ac.UserID, map[string]string{
		"draft_id":   draftID.String(),
		"area_count": fmt.Sprintf("%d", len(body.Areas)),
	})

	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"draft_id":      draft.ID.String(),
		"last_saved_at": draft.LastSavedAt,
	})
}

// DiscardDraft soft-deletes a draft.
func (h *DraftHandler) DiscardDraft(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	draftID, err := uuid.Parse(chi.URLParam(r, "draftId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid draft ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	caseID, _ := h.lookupCaseID(ctx, evidenceID)

	if err := h.repo.DiscardDraft(ctx, draftID, evidenceID); err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "draft not found")
			return
		}
		// unreachable: same pool constraint — post-access-check DB fault
		h.logger.Error("discard draft failed", "draft_id", draftID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.recordCustody(ctx, caseID, evidenceID, "redaction_draft_discarded", ac.UserID, map[string]string{
		"draft_id": draftID.String(),
	})

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
}

// FinalizeDraft finalizes a draft into a permanent redacted evidence copy.
func (h *DraftHandler) FinalizeDraft(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if h.redactionSvc == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "redaction service not available")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	draftID, err := uuid.Parse(chi.URLParam(r, "draftId"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid draft ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	var body struct {
		Description    string `json:"description"`
		Classification string `json:"classification"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.redactionSvc.FinalizeFromDraft(ctx, FinalizeInput{
		EvidenceID:     evidenceID,
		DraftID:        draftID,
		Description:    body.Description,
		Classification: body.Classification,
		ActorID:        ac.UserID,
		ActorName:      ac.Username,
	})
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("finalize draft failed", "draft_id", draftID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, result)
}

// GetManagementView returns finalized versions and active drafts for an evidence item.
func (h *DraftHandler) GetManagementView(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ctx := r.Context()
	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	if _, ok2 := h.checkCaseAccessHTTP(w, ctx, evidenceID, ac); !ok2 {
		return
	}

	view, err := h.repo.GetManagementView(ctx, evidenceID)
	if err != nil { // unreachable: same pool constraint — post-access-check DB fault
		h.logger.Error("get management view failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, view)
}

// checkCaseAccessHTTP verifies the user has access to the case containing the evidence.
// On success returns the case UUID and true. On failure writes the HTTP error and returns false.
func (h *DraftHandler) checkCaseAccessHTTP(w http.ResponseWriter, ctx context.Context, evidenceID uuid.UUID, ac auth.AuthContext) (uuid.UUID, bool) {
	caseID, err := h.lookupCaseID(ctx, evidenceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.RespondError(w, http.StatusNotFound, "evidence not found")
			return uuid.Nil, false
		}
		h.logger.Error("lookup evidence case_id failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return uuid.Nil, false
	}

	if ac.SystemRole < auth.RoleSystemAdmin {
		_, roleErr := h.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
		if roleErr != nil {
			if errors.Is(roleErr, auth.ErrNoCaseRole) {
				httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
				return uuid.Nil, false
			}
			h.logger.Error("case role check failed", "evidence_id", evidenceID, "error", roleErr)
			httputil.RespondError(w, http.StatusInternalServerError, "authorization check failed")
			return uuid.Nil, false
		}
	}
	return caseID, true
}

// lookupCaseID retrieves the case_id for a given evidence item.
func (h *DraftHandler) lookupCaseID(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error) {
	var caseID uuid.UUID
	err := h.db.QueryRow(ctx,
		`SELECT case_id FROM evidence_items WHERE id = $1`,
		evidenceID,
	).Scan(&caseID)
	return caseID, err
}

func (h *DraftHandler) recordCustody(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorID string, detail map[string]string) {
	if h.custody == nil {
		return
	}
	if err := h.custody.RecordEvidenceEvent(ctx, caseID, evidenceID, action, actorID, detail); err != nil {
		h.logger.Error("failed to record custody event", "evidence_id", evidenceID, "action", action, "error", err)
	}
}

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation (23505).
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
