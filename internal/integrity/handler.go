package integrity

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// hexDecodeString is a package-level var so tests can inject failures for the
// otherwise-unreachable hex.DecodeString error path in verifyItem. The stored
// SHA256Hash is always valid hex when VerifyFileHash succeeds, making the
// real decode infallible in practice.
var hexDecodeString = hex.DecodeString

// VerifiableItem represents an evidence item with the fields needed for verification.
type VerifiableItem struct {
	ID         uuid.UUID
	CaseID     uuid.UUID
	StorageKey string
	SHA256Hash string
	TSAToken   []byte
	TSAStatus  string
	Filename   string
}

// EvidenceLoader loads evidence items for verification.
type EvidenceLoader interface {
	ListByCaseForVerification(ctx context.Context, caseID uuid.UUID) ([]VerifiableItem, error)
}

// CustodyLogger records custody chain events.
type CustodyLogger interface {
	RecordEvidenceEvent(ctx context.Context, caseID, evidenceID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// NotificationEvent describes a notification to send.
type NotificationEvent struct {
	Type   string
	CaseID uuid.UUID
	Title  string
	Body   string
}

// EvidenceFlagger persists an integrity warning flag on an evidence item.
type EvidenceFlagger interface {
	FlagIntegrityWarning(ctx context.Context, evidenceID uuid.UUID) error
}

// Notifier sends notifications for critical events.
type Notifier interface {
	Notify(ctx context.Context, event NotificationEvent) error
}

// VerificationJob tracks the progress of an async verification run.
type VerificationJob struct {
	ID         string    `json:"id"`
	CaseID     uuid.UUID `json:"case_id"`
	Status     string    `json:"status"`
	Total      int       `json:"total"`
	Verified   int       `json:"verified"`
	Mismatches int       `json:"mismatches"`
	Missing    int       `json:"missing"`
	StartedAt  time.Time `json:"started_at"`
	Error      string    `json:"error,omitempty"`
	mu         sync.Mutex
}

func (j *VerificationJob) snapshot() jobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return jobSnapshot{
		ID:         j.ID,
		CaseID:     j.CaseID,
		Status:     j.Status,
		Total:      j.Total,
		Verified:   j.Verified,
		Mismatches: j.Mismatches,
		Missing:    j.Missing,
		StartedAt:  j.StartedAt,
		Error:      j.Error,
	}
}

// jobSnapshot is an immutable copy of job state for JSON serialization.
type jobSnapshot struct {
	ID         string    `json:"id"`
	CaseID     uuid.UUID `json:"case_id"`
	Status     string    `json:"status"`
	Total      int       `json:"total"`
	Verified   int       `json:"verified"`
	Mismatches int       `json:"mismatches"`
	Missing    int       `json:"missing"`
	StartedAt  time.Time `json:"started_at"`
	Error      string    `json:"error,omitempty"`
}

// Handler provides HTTP endpoints for integrity verification.
type Handler struct {
	evidenceLoader  EvidenceLoader
	fileReader      FileReader
	tsaVerifier     TimestampAuthority
	custody         CustodyLogger
	notifier        Notifier
	evidenceFlagger EvidenceFlagger
	jobs            sync.Map // map[string]*VerificationJob keyed by caseID
	logger          *slog.Logger
	audit           auth.AuditLogger
}

// NewHandler creates a new integrity verification handler.
func NewHandler(
	evidenceLoader EvidenceLoader,
	fileReader FileReader,
	tsaVerifier TimestampAuthority,
	custodyLogger CustodyLogger,
	notifier Notifier,
	evidenceFlagger EvidenceFlagger,
	logger *slog.Logger,
	audit auth.AuditLogger,
) *Handler {
	return &Handler{
		evidenceLoader:  evidenceLoader,
		fileReader:      fileReader,
		tsaVerifier:     tsaVerifier,
		custody:         custodyLogger,
		notifier:        notifier,
		evidenceFlagger: evidenceFlagger,
		logger:          logger,
		audit:           audit,
	}
}

// RegisterRoutes mounts integrity verification routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/cases/{id}/verify", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Post("/", h.StartVerification)
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Get("/status", h.GetStatus)
	})
}

// StartVerification launches an asynchronous integrity check for all evidence in a case.
func (h *Handler) StartVerification(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Reject if a verification is already running for this case.
	caseKey := caseID.String()
	if existing, loaded := h.jobs.Load(caseKey); loaded {
		job := existing.(*VerificationJob)
		job.mu.Lock()
		status := job.Status
		job.mu.Unlock()
		if status == "running" {
			httputil.RespondError(w, http.StatusConflict, "verification already running for this case")
			return
		}
	}

	jobID := uuid.New().String()
	job := &VerificationJob{
		ID:        jobID,
		CaseID:    caseID,
		Status:    "running",
		StartedAt: time.Now().UTC(),
	}
	h.jobs.Store(caseKey, job)

	go h.runVerification(context.Background(), job, ac.UserID)

	httputil.RespondJSON(w, http.StatusAccepted, map[string]string{
		"job_id": jobID,
	})
}

// GetStatus returns the current progress of a verification job for a case.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case ID")
		return
	}

	existing, ok := h.jobs.Load(caseID.String())
	if !ok {
		httputil.RespondError(w, http.StatusNotFound, "no verification job found for this case")
		return
	}

	job := existing.(*VerificationJob)
	httputil.RespondJSON(w, http.StatusOK, job.snapshot())
}

// runVerification performs the actual verification in the background.
func (h *Handler) runVerification(ctx context.Context, job *VerificationJob, actorUserID string) {
	h.logger.Info("starting integrity verification",
		"job_id", job.ID,
		"case_id", job.CaseID,
	)

	items, err := h.evidenceLoader.ListByCaseForVerification(ctx, job.CaseID)
	if err != nil {
		h.setJobFailed(job, fmt.Sprintf("load evidence: %v", err))
		return
	}

	job.mu.Lock()
	job.Total = len(items)
	job.mu.Unlock()

	for _, item := range items {
		h.verifyItem(ctx, job, item, actorUserID)
	}

	job.mu.Lock()
	if job.Status == "running" {
		job.Status = "completed"
	}
	job.mu.Unlock()

	snap := job.snapshot()
	h.logger.Info("integrity verification completed",
		"job_id", snap.ID,
		"case_id", snap.CaseID,
		"total", snap.Total,
		"verified", snap.Verified,
		"mismatches", snap.Mismatches,
		"missing", snap.Missing,
	)
}

func (h *Handler) verifyItem(ctx context.Context, job *VerificationJob, item VerifiableItem, actorUserID string) {
	computed, err := VerifyFileHash(ctx, h.fileReader, item.StorageKey, item.SHA256Hash)
	if err != nil {
		if computed != "" {
			// Hash mismatch — file exists but hash differs.
			h.handleMismatch(ctx, job, item, actorUserID, computed)
			return
		}
		// File missing or unreadable.
		h.handleMissing(ctx, job, item, actorUserID, err)
		return
	}

	// Hash verified — check TSA token if present.
	if len(item.TSAToken) > 0 {
		digest, decodeErr := hexDecodeString(item.SHA256Hash)
		if decodeErr != nil {
			h.logger.Error("decode stored hash for TSA verification",
				"evidence_id", item.ID,
				"error", decodeErr,
			)
		} else if tsaErr := h.tsaVerifier.VerifyTimestamp(ctx, item.TSAToken, digest); tsaErr != nil {
			h.logger.Warn("TSA token verification failed",
				"evidence_id", item.ID,
				"error", tsaErr,
			)
		}
	}

	job.mu.Lock()
	job.Verified++
	job.mu.Unlock()
}

func (h *Handler) handleMismatch(ctx context.Context, job *VerificationJob, item VerifiableItem, actorUserID, computedHash string) {
	job.mu.Lock()
	job.Mismatches++
	job.mu.Unlock()

	if err := h.evidenceFlagger.FlagIntegrityWarning(ctx, item.ID); err != nil {
		h.logger.Error("flag integrity warning on evidence item",
			"evidence_id", item.ID,
			"error", err,
		)
	}

	h.logger.Error("INTEGRITY ALERT: hash mismatch",
		"evidence_id", item.ID,
		"case_id", item.CaseID,
		"filename", item.Filename,
		"expected", item.SHA256Hash,
		"computed", computedHash,
	)

	detail := map[string]string{
		"expected_hash": item.SHA256Hash,
		"computed_hash": computedHash,
		"filename":      item.Filename,
		"storage_key":   item.StorageKey,
	}

	if err := h.custody.RecordEvidenceEvent(
		ctx, item.CaseID, item.ID,
		"INTEGRITY ALERT — stored hash does not match computed hash",
		actorUserID, detail,
	); err != nil {
		h.logger.Error("record custody event for mismatch",
			"evidence_id", item.ID,
			"error", err,
		)
	}

	if err := h.notifier.Notify(ctx, NotificationEvent{
		Type:   "integrity_warning",
		CaseID: item.CaseID,
		Title:  "Evidence Integrity Alert",
		Body: fmt.Sprintf(
			"Hash mismatch detected for evidence %q (ID: %s). Expected %s, computed %s.",
			item.Filename, item.ID, item.SHA256Hash, computedHash,
		),
	}); err != nil {
		h.logger.Error("send mismatch notification",
			"evidence_id", item.ID,
			"error", err,
		)
	}
}

func (h *Handler) handleMissing(ctx context.Context, job *VerificationJob, item VerifiableItem, actorUserID string, readErr error) {
	job.mu.Lock()
	job.Missing++
	job.mu.Unlock()

	if err := h.evidenceFlagger.FlagIntegrityWarning(ctx, item.ID); err != nil {
		h.logger.Error("flag integrity warning on evidence item",
			"evidence_id", item.ID,
			"error", err,
		)
	}

	h.logger.Error("evidence file missing or unreadable",
		"evidence_id", item.ID,
		"case_id", item.CaseID,
		"filename", item.Filename,
		"storage_key", item.StorageKey,
		"error", readErr,
	)

	detail := map[string]string{
		"filename":    item.Filename,
		"storage_key": item.StorageKey,
		"error":       readErr.Error(),
	}

	if err := h.custody.RecordEvidenceEvent(
		ctx, item.CaseID, item.ID,
		"INTEGRITY ALERT — evidence file missing or unreadable",
		actorUserID, detail,
	); err != nil {
		h.logger.Error("record custody event for missing file",
			"evidence_id", item.ID,
			"error", err,
		)
	}

	if err := h.notifier.Notify(ctx, NotificationEvent{
		Type:   "integrity_warning",
		CaseID: item.CaseID,
		Title:  "Evidence File Missing",
		Body: fmt.Sprintf(
			"Evidence file %q (ID: %s) is missing or unreadable: %v",
			item.Filename, item.ID, readErr,
		),
	}); err != nil {
		h.logger.Error("send missing file notification",
			"evidence_id", item.ID,
			"error", err,
		)
	}
}

func (h *Handler) setJobFailed(job *VerificationJob, errMsg string) {
	job.mu.Lock()
	job.Status = "failed"
	job.Error = errMsg
	job.mu.Unlock()

	h.logger.Error("integrity verification failed",
		"job_id", job.ID,
		"case_id", job.CaseID,
		"error", errMsg,
	)
}
