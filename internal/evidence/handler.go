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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// CustodyReader reads custody log entries for evidence items.
type CustodyReader interface {
	ListByEvidence(ctx context.Context, evidenceID uuid.UUID, limit int, cursor string) ([]custody.Event, int, error)
}

// Handler provides HTTP endpoints for evidence operations.
type Handler struct {
	service   *Service
	custody   CustodyReader
	audit     auth.AuditLogger
	maxUpload int64
}

// NewHandler creates a new evidence HTTP handler.
func NewHandler(service *Service, custodyReader CustodyReader, audit auth.AuditLogger, maxUploadSize int64) *Handler {
	return &Handler{
		service:   service,
		custody:   custodyReader,
		audit:     audit,
		maxUpload: maxUploadSize,
	}
}

// RegisterRoutes mounts evidence routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Case-scoped evidence routes
	r.Route("/api/cases/{caseID}/evidence", func(r chi.Router) {
		r.Post("/", h.Upload)
		r.Get("/", h.ListByCase)
	})

	// Evidence-scoped routes
	r.Route("/api/evidence/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Get("/download", h.Download)
		r.Get("/thumbnail", h.GetThumbnail)
		r.Get("/versions", h.GetVersionHistory)
		r.Get("/custody", h.GetCustodyLog)
		r.Patch("/", h.UpdateMetadata)
		r.Delete("/", h.Destroy)
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

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload+10<<20) // extra room for multipart overhead

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	classification := r.FormValue("classification")
	if classification == "" {
		classification = ClassificationRestricted
	}

	description := r.FormValue("description")

	var tags []string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			// Try comma-separated fallback
			tags = splitTags(tagsStr)
		}
	}

	input := UploadInput{
		CaseID:         caseID,
		File:           file,
		Filename:       header.Filename,
		SizeBytes:      header.Size,
		Classification: classification,
		Description:    description,
		Tags:           tags,
		UploadedBy:     ac.UserID,
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

	filter := EvidenceFilter{
		CaseID:         caseID,
		Classification: r.URL.Query().Get("classification"),
		MimeType:       r.URL.Query().Get("mime_type"),
		SearchQuery:    r.URL.Query().Get("q"),
		CurrentOnly:    r.URL.Query().Get("current_only") == "true",
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

	result, err := h.service.UpdateMetadata(r.Context(), id, updates, ac.UserID)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

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
		Reason string `json:"reason"`
	}
	if err := decodeBody(r, &body); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := DestroyInput{
		EvidenceID: id,
		Reason:     body.Reason,
		ActorID:    ac.UserID,
	}

	if err := h.service.Destroy(r.Context(), input); err != nil {
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

func respondServiceError(w http.ResponseWriter, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		httputil.RespondError(w, http.StatusNotFound, "not found")
		return
	}
	httputil.RespondError(w, http.StatusInternalServerError, "internal error")
}
