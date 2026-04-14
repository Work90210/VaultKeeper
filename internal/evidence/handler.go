package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// CustodyReader reads custody log entries for evidence items.
type CustodyReader interface {
	ListByEvidence(ctx context.Context, evidenceID uuid.UUID, limit int, cursor string) ([]custody.Event, int, error)
}

// OrgMembershipChecker verifies whether a user belongs to a case's organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
}

// Handler provides HTTP endpoints for evidence operations.
type Handler struct {
	service        *Service
	redaction      *RedactionService
	custody        CustodyReader
	attemptRepo    UploadAttemptRepository
	audit          auth.AuditLogger
	maxUpload      int64
	caseRoleLoader auth.CaseRoleLoader      // optional — Sprint 9 access matrix
	orgChecker     OrgMembershipChecker      // optional — org boundary enforcement
	caseLookupOrg  func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error) // returns org ID for a case
}

// NewHandler creates a new evidence HTTP handler.
func NewHandler(service *Service, custodyReader CustodyReader, audit auth.AuditLogger, maxUploadSize int64, attemptRepos ...UploadAttemptRepository) *Handler {
	var attemptRepo UploadAttemptRepository = noopUploadAttemptRepository{}
	if len(attemptRepos) > 0 && attemptRepos[0] != nil {
		attemptRepo = attemptRepos[0]
	}
	return &Handler{
		service:     service,
		custody:     custodyReader,
		attemptRepo: attemptRepo,
		audit:       audit,
		maxUpload:   maxUploadSize,
	}
}

// SetRedactionService sets the redaction service on the handler.
func (h *Handler) SetRedactionService(rs *RedactionService) {
	h.redaction = rs
}

// SetCaseRoleLoader wires the case-role loader used by Sprint 9 access
// enforcement on list/get/download/thumbnail paths. Production MUST call
// this; if nil, the handler falls back to deny-by-default on case-scoped
// reads (loadCallerCaseRole returns ErrNoCaseRole).
func (h *Handler) SetCaseRoleLoader(loader auth.CaseRoleLoader) {
	h.caseRoleLoader = loader
}

// SetOrgMembershipChecker wires the org membership checker. When set, evidence
// access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// loadCallerCaseRole resolves the caller's case role for access checks.
// Returns the role string (matching evidence.RoleXxx constants) and whether
// the caller may proceed. A false result means the handler should respond
// with 404 (not 403) to avoid leaking existence of classified items.
func (h *Handler) loadCallerCaseRole(ctx context.Context, caseID uuid.UUID) (string, bool) {
	ac, ok := auth.GetAuthContext(ctx)
	if !ok {
		return "", false
	}
	// System admins bypass the case-role matrix — they already have cross-
	// case admin privilege via RequireSystemRole on admin routes, but on
	// evidence reads we let them proceed so support operations still work.
	// This bypass must happen before the loader nil-check so tests and any
	// legitimate admin flow works without a production loader wired.
	if ac.SystemRole == auth.RoleSystemAdmin {
		return RoleJudge, true // use judge as the highest-access effective role
	}

	// Org membership gate: verify the caller is a member of the case's org.
	if h.orgChecker != nil && h.caseLookupOrg != nil {
		orgID, err := h.caseLookupOrg(ctx, caseID)
		if err != nil {
			return "", false
		}
		isMember, err := h.orgChecker.IsActiveMember(ctx, orgID, ac.UserID)
		if err != nil || !isMember {
			return "", false
		}
	}

	if h.caseRoleLoader == nil {
		return "", false
	}
	role, err := h.caseRoleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
	if err != nil {
		return "", false
	}
	return string(role), true
}

// enforceItemAccess applies the classification access matrix to a fetched
// evidence item. Returns true if the caller may see it. On false, the
// handler MUST respond 404, not 403.
//
// Sprint 9 L1: successful reads of confidential or ex_parte items are
// logged with a structured slog entry so the audit pipeline can alert on
// suspicious read patterns (e.g. a prosecution member reading dozens of
// ex_parte items in a short window). Public/restricted reads are not
// logged to keep the signal-to-noise ratio sensible.
func (h *Handler) enforceItemAccess(ctx context.Context, item EvidenceItem) bool {
	if ac, ok := auth.GetAuthContext(ctx); ok && ac.SystemRole == auth.RoleSystemAdmin {
		h.logClassifiedRead(ctx, item, "system_admin", ac.UserID)
		return true
	}
	role, ok := h.loadCallerCaseRole(ctx, item.CaseID)
	if !ok {
		return false
	}
	// Treat empty classification as the default ("restricted") so legacy
	// rows created before Sprint 9 are visible to authorised case roles.
	class := item.Classification
	if class == "" {
		class = ClassificationRestricted
	}
	if !CheckAccess(role, class, item.ExParteSide, UserSideForRole(role)) {
		return false
	}
	// Log classified reads only.
	if ac, ok := auth.GetAuthContext(ctx); ok {
		h.logClassifiedRead(ctx, item, role, ac.UserID)
	}
	return true
}

// logClassifiedRead emits a structured audit entry for successful reads
// of confidential or ex_parte items. Public/restricted reads are not
// logged to keep the signal-to-noise ratio sensible.
func (h *Handler) logClassifiedRead(_ context.Context, item EvidenceItem, role, userID string) {
	if item.Classification != ClassificationConfidential && item.Classification != ClassificationExParte {
		return
	}
	side := ""
	if item.ExParteSide != nil {
		side = *item.ExParteSide
	}
	slog.Info("classified evidence read",
		"evidence_id", item.ID,
		"case_id", item.CaseID,
		"classification", item.Classification,
		"ex_parte_side", side,
		"actor_role", role,
		"actor_user_id", userID,
	)
}

// uploadLimiter allows 10 upload requests per minute per user.
var uploadLimiter = newUserRateLimiter(rate.Every(6*time.Second), 10)

// captureMetadataLimiter allows 6 capture metadata writes per minute per user.
var captureMetadataLimiter = newUserRateLimiter(rate.Every(10*time.Second), 6)

// RegisterRoutes mounts evidence routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Case-scoped evidence routes
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.With(rateLimitMiddleware(uploadLimiter)).Post("/", h.Upload)
		r.Get("/", h.ListByCase)
	})

	// Tag taxonomy routes (Sprint 9 Step 5). Autocomplete is readable by any
	// authenticated user; rename/merge/delete require case_admin.
	r.Get("/api/evidence/tags/autocomplete", h.TagAutocomplete)
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/api/evidence/tags/rename", h.TagRename)
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/api/evidence/tags/merge", h.TagMerge)
	r.With(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit)).Post("/api/evidence/tags/delete", h.TagDelete)

	// Evidence-scoped routes
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Get("/download", h.Download)
		r.Get("/thumbnail", h.GetThumbnail)
		r.Get("/versions", h.GetVersionHistory)
		r.Get("/custody", h.GetCustodyLog)
		r.Patch("/", h.UpdateMetadata)
		r.Delete("/", h.Destroy)
		r.With(rateLimitMiddleware(uploadLimiter)).Post("/version", h.UploadNewVersion)
		r.Post("/redact", h.ApplyRedactions)
		r.Post("/redact/preview", h.PreviewRedactions)

		// Berkeley Protocol capture metadata
		r.With(rateLimitMiddleware(captureMetadataLimiter)).Put("/capture-metadata", h.UpsertCaptureMetadata)
		r.Get("/capture-metadata", h.GetCaptureMetadata)
	})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	form, ok := parseUploadForm(w, r, h.maxUpload)
	if !ok {
		return
	}
	defer form.file.Close()

	userID, err := uuid.Parse(ac.UserID)
	if err != nil {
		slog.Error("invalid user ID in auth context", "raw", ac.UserID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	attemptID, err := h.attemptRepo.Record(r.Context(), UploadAttempt{
		CaseID:     caseID,
		UserID:     userID,
		ClientHash: form.clientHash,
		StartedAt:  time.Now().UTC(),
	})
	if err != nil {
		slog.Error("failed to record upload attempt", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	input := UploadInput{
		CaseID:         caseID,
		File:           form.file,
		Filename:       form.fileHeader,
		SizeBytes:      form.fileSize,
		Classification: form.classification,
		Description:    form.description,
		Tags:           form.tags,
		UploadedBy:     ac.UserID,
		UploadedByName: ac.Username,
		Source:         form.source,
		SourceDate:     form.sourceDate,
		ExpectedSHA256: form.clientHash,
		AttemptID:      attemptID,
	}

	evidence, err := h.service.Upload(r.Context(), input)
	if err != nil {
		slog.Error("evidence upload failed", "error", err, "case_id", caseID, "filename", input.Filename)
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, evidence)
}

func (h *Handler) ListByCase(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	// Sprint 9: resolve the caller's case role so the repository layer can
	// apply the classification access matrix. Missing role → 403.
	role, ok := h.loadCallerCaseRole(r.Context(), caseID)
	if !ok {
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}

	filter := EvidenceFilter{
		CaseID:         caseID,
		Classification: r.URL.Query().Get("classification"),
		MimeType:       r.URL.Query().Get("mime_type"),
		SearchQuery:    r.URL.Query().Get("q"),
		CurrentOnly:    r.URL.Query().Get("current_only") == "true",
		UserRole:       role,
	}

	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		filter.Tags = strings.Split(tagsStr, ",")
	}

	page := parsePagination(r)

	result, err := h.service.List(r.Context(), filter, page)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondPaginated(w, http.StatusOK, result.Items, result.TotalCount, result.NextCursor, result.HasMore)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	evidence, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	// Sprint 9: classification matrix enforced post-fetch (404 on deny to
	// avoid revealing existence of confidential/ex_parte items).
	if !h.enforceItemAccess(r.Context(), evidence) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, evidence)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Sprint 9: gate download with classification matrix. Fetch first so we
	// can read CaseID/Classification for the check, then stream.
	item, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), item) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	reader, size, contentType, filename, err := h.service.Download(r.Context(), id, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, reader); err != nil {
		// Can't change status at this point; just log
		return
	}
}

func (h *Handler) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Sprint 9: gate thumbnail on classification matrix.
	item, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), item) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	reader, size, err := h.service.GetThumbnail(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)

	io.Copy(w, reader) //nolint:errcheck
}

func (h *Handler) GetVersionHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Sprint 9: gate version history on classification access. Use the
	// current (newest) item as the access anchor — if the caller can see
	// the current version they can see its history.
	item, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), item) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	versions, err := h.service.GetVersionHistory(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, versions)
}

func (h *Handler) GetCustodyLog(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}
	// Note: custody log is intentionally NOT gated behind enforceItemAccess.
	// The listing paths already hide classified UUIDs from unauthorised
	// roles, so this endpoint is reachable only by callers who legitimately
	// know the ID. Adding an access gate here would also block case
	// auditors (observer/victim_rep) from reviewing the audit trail for
	// items they can see in their listing.

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	cursor := r.URL.Query().Get("cursor")

	events, total, err := h.custody.ListByEvidence(r.Context(), id, limit, cursor)
	if err != nil {
		slog.Error("failed to list custody log", "evidence_id", id, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to load custody log")
		return
	}

	type custodyJSON struct {
		ID           string `json:"id"`
		CaseID       string `json:"case_id"`
		EvidenceID   string `json:"evidence_id"`
		Action       string `json:"action"`
		ActorUserID  string `json:"actor_user_id"`
		Detail       string `json:"detail"`
		HashValue    string `json:"hash_value"`
		PreviousHash string `json:"previous_hash"`
		Timestamp    string `json:"timestamp"`
	}

	items := make([]custodyJSON, 0, len(events))
	for _, e := range events {
		items = append(items, custodyJSON{
			ID:           e.ID.String(),
			CaseID:       e.CaseID.String(),
			EvidenceID:   e.EvidenceID.String(),
			Action:       e.Action,
			ActorUserID:  e.ActorUserID,
			Detail:       e.Detail,
			HashValue:    e.HashValue,
			PreviousHash: e.PreviousHash,
			Timestamp:    e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	httputil.RespondPaginated(w, http.StatusOK, items, total, "", len(events) == limit)
}

func (h *Handler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	var updates EvidenceUpdate
	if err := decodeBody(r, &updates); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Sprint 9: enforce classification access on the current row before
	// allowing a metadata edit (caller must first be able to see the item).
	current, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), current) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	result, err := h.service.UpdateMetadata(r.Context(), id, updates, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

// Destroy is the Sprint 9 audited destruction handler.
//
// DELETE /api/evidence/{id} body: {"authority": "<court order / legal basis>"}
//
// Flow: parse body → enforce classification access (must be able to see
// the item) → call DestroyEvidence (which handles legal hold, retention,
// MinIO-then-DB-safe ordering, custody chain, notifications).
func (h *Handler) Destroy(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	var body struct {
		Authority string `json:"authority"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Gate on access matrix before destruction.
	current, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), current) {
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	if err := h.service.DestroyEvidence(r.Context(), DestroyEvidenceInput{
		EvidenceID: id,
		ActorID:    ac.UserID,
		Authority:  body.Authority,
	}); err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"status": "destroyed"})
}

func decodeBody(r *http.Request, dst any) error {
	limited := io.LimitReader(r.Body, MaxBodySize+1)
	decoder := json.NewDecoder(limited)

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected end") {
			return &ValidationError{Field: "body", Message: "request body too large"}
		}
		return &ValidationError{Field: "body", Message: "invalid JSON"}
	}

	return nil
}

func parsePagination(r *http.Request) Pagination {
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return Pagination{
		Limit:  limit,
		Cursor: r.URL.Query().Get("cursor"),
	}
}

// parseUploadForm extracts and validates the multipart form fields common to
// Upload and UploadNewVersion. It validates the client hash (header + form),
// extracts the file, and parses metadata fields. Returns the validated client
// hash, file handle, file header, and parsed metadata on success.
type uploadFormData struct {
	clientHash     string
	file           io.ReadCloser
	fileHeader     string
	fileSize       int64
	classification string
	description    string
	source         string
	sourceDate     *time.Time
	tags           []string
}

func parseUploadForm(w http.ResponseWriter, r *http.Request, maxUpload int64) (*uploadFormData, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+10<<20)

	headerHash, err := validateClientHashHeader(r)
	if err != nil {
		respondServiceError(w, err)
		return nil, false
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid multipart form")
		return nil, false
	}

	clientHash, err := validateClientHashForm(r, headerHash)
	if err != nil {
		respondServiceError(w, err)
		return nil, false
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "file field is required")
		return nil, false
	}

	classification := r.FormValue("classification")
	if classification == "" {
		classification = ClassificationRestricted
	}

	description := r.FormValue("description")
	source := r.FormValue("source")

	var sourceDate *time.Time
	if sd := r.FormValue("source_date"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			sourceDate = &t
		} else if t, err := time.Parse("2006-01-02", sd); err == nil {
			sourceDate = &t
		}
	}

	var tags []string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			tags = splitTags(tagsStr)
		}
	}

	return &uploadFormData{
		clientHash:     clientHash,
		file:           file,
		fileHeader:     header.Filename,
		fileSize:       header.Size,
		classification: classification,
		description:    description,
		source:         source,
		sourceDate:     sourceDate,
		tags:           tags,
	}, true
}

func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

func (h *Handler) UploadNewVersion(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	parentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	form, ok := parseUploadForm(w, r, h.maxUpload)
	if !ok {
		return
	}
	defer form.file.Close()

	// Look up parent to get case ID for the attempt record.
	parent, err := h.service.Get(r.Context(), parentID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	userID, err := uuid.Parse(ac.UserID)
	if err != nil {
		slog.Error("invalid user ID in auth context", "raw", ac.UserID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	attemptID, err := h.attemptRepo.Record(r.Context(), UploadAttempt{
		CaseID:     parent.CaseID,
		UserID:     userID,
		ClientHash: form.clientHash,
		StartedAt:  time.Now().UTC(),
	})
	if err != nil {
		slog.Error("failed to record upload attempt", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	input := UploadInput{
		File:           form.file,
		Filename:       form.fileHeader,
		SizeBytes:      form.fileSize,
		Classification: form.classification,
		Description:    form.description,
		Tags:           form.tags,
		UploadedBy:     ac.UserID,
		UploadedByName: ac.Username,
		Source:         form.source,
		SourceDate:     form.sourceDate,
		ExpectedSHA256: form.clientHash,
		AttemptID:      attemptID,
	}

	evidence, err := h.service.UploadNewVersion(r.Context(), parentID, input)
	if err != nil {
		slog.Error("evidence version upload failed", "error", err, "parent_id", parentID)
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, evidence)
}

func (h *Handler) ApplyRedactions(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.redaction == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "redaction service not available")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	var body struct {
		Redactions []RedactionArea `json:"redactions"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.redaction.ApplyRedactions(r.Context(), id, body.Redactions, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, result)
}

func (h *Handler) PreviewRedactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if h.redaction == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "redaction service not available")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	var body struct {
		Redactions []RedactionArea `json:"redactions"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	reader, mimeType, err := h.redaction.PreviewRedactions(r.Context(), id, body.Redactions)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader) //nolint:errcheck
}

func (h *Handler) UpsertCaptureMetadata(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Enforce classification access — caller must be able to see the item.
	current, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), current) {
		slog.Warn("capture metadata write denied: classification access failed",
			"evidence_id", id, "case_id", current.CaseID, "actor", ac.UserID)
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	// Write-role enforcement: only investigator/prosecutor/judge may write capture metadata.
	// Observer and victim_representative roles are read-only for provenance data.
	role, roleOK := h.loadCallerCaseRole(r.Context(), current.CaseID)
	if !roleOK {
		slog.Warn("capture metadata write denied: no case role",
			"evidence_id", id, "case_id", current.CaseID, "actor", ac.UserID)
		httputil.RespondError(w, http.StatusForbidden, "no role on this case")
		return
	}
	captureWriteRoles := map[string]bool{"investigator": true, "prosecutor": true, "judge": true}
	if !captureWriteRoles[role] {
		slog.Warn("capture metadata write denied: insufficient role",
			"evidence_id", id, "case_id", current.CaseID, "actor", ac.UserID, "role", role)
		httputil.RespondError(w, http.StatusForbidden, "insufficient role to write capture metadata")
		return
	}

	var input CaptureMetadataInput
	if err := decodeBody(r, &input); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, warnings, err := h.service.UpsertCaptureMetadata(r.Context(), id, input, ac.UserID, role)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	// Redact sensitive fields based on caller's role (already loaded above)
	redacted := saved.RedactForRole(role)

	response := map[string]any{
		"data": redacted,
	}
	if len(warnings) > 0 {
		response["warnings"] = warnings
	}
	httputil.RespondJSON(w, http.StatusOK, response)
}

func (h *Handler) GetCaptureMetadata(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Enforce classification access.
	current, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if !h.enforceItemAccess(r.Context(), current) {
		ac, acOK := auth.GetAuthContext(r.Context())
		actorID := ""
		if acOK {
			actorID = ac.UserID
		}
		slog.Warn("capture metadata read denied: classification access failed",
			"evidence_id", id, "case_id", current.CaseID, "actor", actorID)
		httputil.RespondError(w, http.StatusNotFound, "evidence not found")
		return
	}

	metadata, err := h.service.GetCaptureMetadata(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrCaptureMetadataNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "no capture metadata for this evidence item")
			return
		}
		respondServiceError(w, err)
		return
	}

	role, roleOK := h.loadCallerCaseRole(r.Context(), current.CaseID)
	if !roleOK {
		slog.Warn("could not resolve case role for capture metadata read",
			"evidence_id", id, "case_id", current.CaseID)
	}
	redacted := metadata.RedactForRole(role)
	httputil.RespondJSON(w, http.StatusOK, redacted)
}

func respondServiceError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Sprint 11.5: client hash validation errors → 400.
	if errors.Is(err, ErrMissingClientHash) || errors.Is(err, ErrMalformedClientHash) || errors.Is(err, ErrHashFieldDisagreement) {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		httputil.RespondError(w, http.StatusNotFound, "not found")
		return
	}
	// Sprint 11.5: hash mismatch → 409. Server hash is NOT echoed to the
	// client to prevent use as a hash oracle. The mismatch is logged
	// server-side via custody events for operator forensics.
	if errors.Is(err, ErrHashMismatch) {
		httputil.RespondError(w, http.StatusConflict, "upload_hash_mismatch")
		return
	}
	// Sprint 9: legal hold and retention blocks surface as 409 Conflict.
	if errors.Is(err, ErrLegalHoldActive) || errors.Is(err, ErrRetentionActive) {
		httputil.RespondError(w, http.StatusConflict, err.Error())
		return
	}
	httputil.RespondError(w, http.StatusInternalServerError, "internal error")
}
