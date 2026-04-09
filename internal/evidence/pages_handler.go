package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gen2brain/go-fitz"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

const (
	defaultPageDPI  = 150
	minPageDPI      = 72
	maxPageDPI      = 300
	maxPDFReadSize  = 512 << 20 // 512 MB limit for PDF rendering
)

// CaseRoleChecker checks case membership for authorization.
type CaseRoleChecker interface {
	LoadCaseRole(ctx context.Context, caseID, userID string) (auth.CaseRole, error)
}

// PagesHandler serves rendered PDF pages as JPEG images with MinIO caching.
type PagesHandler struct {
	db         *pgxpool.Pool
	storage    ObjectStorage
	roleLoader CaseRoleChecker
	logger     *slog.Logger
}

// NewPagesHandler creates a handler for the PDF page renderer API.
func NewPagesHandler(db *pgxpool.Pool, storage ObjectStorage, roleLoader CaseRoleChecker, logger *slog.Logger) *PagesHandler {
	return &PagesHandler{db: db, storage: storage, roleLoader: roleLoader, logger: logger}
}

// RegisterRoutes mounts the page renderer routes.
func (h *PagesHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/evidence/{id}/page-count", h.GetPageCount)
	r.Get("/api/evidence/{id}/pages/{pageNum}", h.GetPage)
}

// GetPageCount returns the total number of pages in a PDF evidence item.
func (h *PagesHandler) GetPageCount(w http.ResponseWriter, r *http.Request) {
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

	caseID, storageKey, err := h.lookupEvidenceInfo(ctx, evidenceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.RespondError(w, http.StatusNotFound, "evidence not found")
			return
		}
		h.logger.Error("lookup evidence info failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if ac.SystemRole < auth.RoleSystemAdmin {
		_, roleErr := h.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
		if roleErr != nil {
			if errors.Is(roleErr, auth.ErrNoCaseRole) {
				httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			h.logger.Error("case role check failed", "evidence_id", evidenceID, "error", roleErr)
			httputil.RespondError(w, http.StatusInternalServerError, "authorization check failed")
			return
		}
	}

	// Try serving from cache
	cacheKey := fmt.Sprintf("page-cache/%s/page_count.json", evidenceID)
	if cached, count := h.readCachedPageCount(ctx, cacheKey); cached {
		httputil.RespondJSON(w, http.StatusOK, map[string]int{"page_count": count})
		return
	}

	// Download PDF from storage with size limit
	pdfReader, _, _, err := h.storage.GetObject(ctx, storageKey)
	if err != nil {
		h.logger.Error("read evidence file failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to load evidence file")
		return
	}
	pdfData, err := io.ReadAll(io.LimitReader(pdfReader, maxPDFReadSize))
	pdfReader.Close()
	if err != nil { // unreachable: in-memory and MinIO readers over a local network rarely return read errors
		h.logger.Error("read evidence file failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to load evidence file")
		return
	}

	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		h.logger.Error("open PDF failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to read PDF")
		return
	}
	pageCount := doc.NumPage()
	doc.Close()

	// Cache asynchronously
	go func() {
		payload, _ := json.Marshal(map[string]int{"page_count": pageCount})
		if putErr := h.storage.PutObject(context.Background(), cacheKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); putErr != nil {
			// unreachable: goroutine runs after response is sent; assertion impossible
			h.logger.Warn("cache page count failed", "cache_key", cacheKey, "error", putErr)
		}
	}()

	httputil.RespondJSON(w, http.StatusOK, map[string]int{"page_count": pageCount})
}

// readCachedPageCount attempts to read a cached page count from storage.
func (h *PagesHandler) readCachedPageCount(ctx context.Context, cacheKey string) (bool, int) {
	rc, _, _, err := h.storage.GetObject(ctx, cacheKey)
	if err != nil {
		return false, 0
	}
	defer rc.Close()

	var result struct {
		PageCount int `json:"page_count"`
	}
	if err := json.NewDecoder(rc).Decode(&result); err != nil {
		return false, 0
	}
	return true, result.PageCount
}

// GetPage renders a single PDF page as JPEG.
func (h *PagesHandler) GetPage(w http.ResponseWriter, r *http.Request) {
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

	pageNum, err := strconv.Atoi(chi.URLParam(r, "pageNum"))
	if err != nil || pageNum < 1 {
		httputil.RespondError(w, http.StatusBadRequest, "invalid page number")
		return
	}

	dpi, err := parsePageDPI(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Authorization: look up case_id and verify membership
	caseID, storageKey, err := h.lookupEvidenceInfo(ctx, evidenceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.RespondError(w, http.StatusNotFound, "evidence not found")
			return
		}
		h.logger.Error("lookup evidence info failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if ac.SystemRole < auth.RoleSystemAdmin {
		_, roleErr := h.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
		if roleErr != nil {
			if errors.Is(roleErr, auth.ErrNoCaseRole) {
				httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			h.logger.Error("case role check failed", "evidence_id", evidenceID, "error", roleErr)
			httputil.RespondError(w, http.StatusInternalServerError, "authorization check failed")
			return
		}
	}

	cacheKey := fmt.Sprintf("page-cache/%s/%d_%d.jpg", evidenceID, pageNum, dpi)

	// Try serving from cache
	if h.serveFromCache(ctx, w, cacheKey) {
		return
	}

	// Download PDF from storage with size limit
	pdfReader, _, _, err := h.storage.GetObject(ctx, storageKey)
	if err != nil {
		h.logger.Error("read evidence file failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to load evidence file")
		return
	}
	pdfData, err := io.ReadAll(io.LimitReader(pdfReader, maxPDFReadSize))
	pdfReader.Close()
	if err != nil { // unreachable: in-memory and MinIO readers over a local network rarely return read errors
		h.logger.Error("read evidence file failed", "evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "failed to load evidence file")
		return
	}

	// Render the page
	imageBytes, err := renderPDFPageAsJPEG(pdfData, pageNum, dpi)
	if err != nil {
		h.logger.Error("render PDF page failed", "evidence_id", evidenceID, "page", pageNum, "dpi", dpi, "error", err)
		httputil.RespondError(w, http.StatusNotFound, "page not found")
		return
	}

	// Cache asynchronously
	go func() {
		if putErr := h.storage.PutObject(context.Background(), cacheKey, bytes.NewReader(imageBytes), int64(len(imageBytes)), "image/jpeg"); putErr != nil {
			// unreachable: goroutine runs after response is sent; assertion impossible
			h.logger.Warn("cache rendered page failed", "cache_key", cacheKey, "error", putErr)
		}
	}()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(imageBytes)
}

func parsePageDPI(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("dpi")
	if raw == "" {
		return defaultPageDPI, nil
	}
	dpi, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid dpi value")
	}
	if dpi < minPageDPI || dpi > maxPageDPI {
		return 0, fmt.Errorf("dpi must be between %d and %d", minPageDPI, maxPageDPI)
	}
	return dpi, nil
}

func (h *PagesHandler) serveFromCache(ctx context.Context, w http.ResponseWriter, cacheKey string) bool {
	rc, _, _, err := h.storage.GetObject(ctx, cacheKey)
	if err != nil {
		return false
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
	return true
}

func (h *PagesHandler) lookupEvidenceInfo(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, string, error) {
	var caseID uuid.UUID
	var storageKey *string
	err := h.db.QueryRow(ctx,
		`SELECT case_id, storage_key FROM evidence_items WHERE id = $1`,
		evidenceID,
	).Scan(&caseID, &storageKey)
	if err != nil {
		return uuid.Nil, "", err
	}
	if storageKey == nil {
		return uuid.Nil, "", fmt.Errorf("evidence has no storage key")
	}
	return caseID, *storageKey, nil
}

func renderPDFPageAsJPEG(pdfData []byte, pageNum, dpi int) ([]byte, error) {
	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer doc.Close()

	if pageNum < 1 || pageNum > doc.NumPage() {
		return nil, fmt.Errorf("page %d out of range (1-%d)", pageNum, doc.NumPage())
	}

	img, err := doc.ImageDPI(pageNum-1, float64(dpi))
	if err != nil { // unreachable: MuPDF only fails here on corrupt pages; out-of-range is caught above
		return nil, fmt.Errorf("render page %d: %w", pageNum, err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil { // unreachable: jpeg encoding to bytes.Buffer never fails
		return nil, fmt.Errorf("encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}
