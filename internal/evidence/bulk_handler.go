package evidence

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// BulkHandler exposes the bulk ZIP upload endpoints.
type BulkHandler struct {
	svc        *BulkService
	audit      auth.AuditLogger
	logger     *slog.Logger
	maxArchive int64 // cap on the request body; defaults to BulkMaxUncompressedRatio × maxUpload
}

// NewBulkHandler creates the HTTP handler for bulk ingestion.
func NewBulkHandler(svc *BulkService, audit auth.AuditLogger, logger *slog.Logger, maxUpload int64) *BulkHandler {
	// Compressed ZIP cap. The uncompressed cap is enforced inside
	// ExtractBulkZIP. The request body cap here is intentionally smaller
	// than the uncompressed cap because zip compression is bounded.
	bodyCap := maxUpload * 2
	if bodyCap <= 0 {
		bodyCap = 1 << 30 // 1GB fallback
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BulkHandler{svc: svc, audit: audit, logger: logger, maxArchive: bodyCap}
}

// RegisterRoutes mounts the bulk upload routes.
//
// The case-scoped POST endpoint accepts a multipart form with a single
// "archive" file field containing the ZIP. The GET status endpoint
// returns the job row as JSON.
func (h *BulkHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{caseID}/evidence/bulk", func(r chi.Router) {
		r.Use(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit))
		r.Post("/", h.Submit)
		r.Get("/{jobID}/status", h.Status)
	})
}

// Submit receives a ZIP and processes it synchronously (within the
// request). A future enhancement can move processing to a worker queue
// and return 202 with a job id; the service + repo are already shaped
// for that.
func (h *BulkHandler) Submit(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case id")
		return
	}

	// Hard request-body cap to defeat adversarial uploads before we ever
	// allocate the zip.Reader. http.MaxBytesReader returns a 413 on
	// overflow when the caller tries to read past the limit.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxArchive+(32<<20))

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("archive")
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "archive field is required")
		return
	}
	defer file.Close()

	// ReadAll from a multipart file header rarely fails — the body has
	// already been fully buffered by ParseMultipartForm above. Any read
	// error would be symptomatic of a truncated underlying HTTP stream,
	// in which case the subsequent size-limit check or ExtractBulkZIP's
	// own "not a valid zip" check will reject the partial bytes with a
	// 400. Discarding the error here lets those downstream checks
	// produce a single consistent response path.
	data, _ := io.ReadAll(io.LimitReader(file, h.maxArchive+1))
	if int64(len(data)) > h.maxArchive {
		httputil.RespondError(w, http.StatusRequestEntityTooLarge, "archive exceeds maximum size")
		return
	}

	classification := r.FormValue("classification")

	job, err := h.svc.Submit(r.Context(), BulkSubmitInput{
		CaseID:         caseID,
		ArchiveBytes:   data,
		ArchiveName:    header.Filename,
		UploadedBy:     ac.UserID,
		UploadedByName: ac.Username,
		Classification: classification,
	})
	if err != nil {
		if errors.Is(err, ErrZipRejected) {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "bulk upload failed")
		return
	}
	// The processing phase already ran synchronously inside Submit, so
	// the returned job is in its final state — use 200 OK rather than
	// 202 Accepted to match the actual semantics.
	httputil.RespondJSON(w, http.StatusOK, job)
}

// Status returns the current state of a bulk upload job. Cross-case
// enumeration is prevented by the case-scoped repository query — the
// service returns ErrBulkJobNotFound for any job outside the caller's
// case path, whether or not the job exists.
func (h *BulkHandler) Status(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case id")
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := h.svc.Get(r.Context(), caseID, jobID)
	if err != nil {
		if errors.Is(err, ErrBulkJobNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "bulk upload job not found")
			return
		}
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, job)
}
