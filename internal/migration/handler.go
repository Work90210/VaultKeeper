package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// CaseInfo is the minimum case metadata rendered on the attestation
// certificate.
type CaseInfo struct {
	ReferenceCode string
	Title         string
}

// CaseLookup is the narrow interface the handler needs to stamp the
// attestation certificate with case metadata. A single call returns
// everything — avoids two DB round-trips per certificate render.
type CaseLookup interface {
	GetCaseInfo(ctx context.Context, caseID uuid.UUID) (CaseInfo, error)
}

// Handler exposes the migration service over HTTP.
type Handler struct {
	svc         *Service
	cases       CaseLookup
	signer      *Signer
	audit       auth.AuditLogger
	instance    string // version label for the attestation certificate
	stagingRoot string // allowlisted prefix for manifest_path and source_root
	logger      *slog.Logger
}

// NewHandler creates the HTTP handler. `cases` may be nil, in which case
// the certificate will render empty case reference/title. `stagingRoot`
// is the absolute directory path under which all operator-supplied paths
// must live; an empty stagingRoot disables the HTTP migration endpoint.
func NewHandler(svc *Service, cases CaseLookup, signer *Signer, audit auth.AuditLogger, instanceVersion, stagingRoot string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		svc:         svc,
		cases:       cases,
		signer:      signer,
		audit:       audit,
		instance:    instanceVersion,
		stagingRoot: stagingRoot,
		logger:      logger,
	}
}

// validateStagedPath enforces that an operator-supplied path lives under
// the configured staging root. This is the only authorisation boundary
// for arbitrary filesystem reads on the migration endpoint — without it,
// any case_admin could trick the server into opening /etc/passwd.
func (h *Handler) validateStagedPath(p string) (string, error) {
	if h.stagingRoot == "" {
		return "", errors.New("migration staging is not configured on this server")
	}
	if p == "" {
		return "", errors.New("path is required")
	}
	if strings.ContainsRune(p, 0) {
		return "", errors.New("null byte in path")
	}
	// filepath.Abs only fails when os.Getwd() fails (the cwd has been
	// deleted out from under us). Not a real production failure mode.
	absRoot, _ := filepath.Abs(h.stagingRoot)
	absPath, _ := filepath.Abs(p)
	cleanPath := filepath.Clean(absPath)
	// Resolve symlinks on the leaf so an attacker can't plant a symlink
	// inside the staging root that points at /etc/passwd. We FAIL CLOSED
	// on EvalSymlinks error — a silent fallback to the unresolved path
	// opens a TOCTOU where a symlink could be planted between check and
	// open. The only legitimate error here is "path does not exist",
	// which is also a rejection condition for a migration run.
	resolved, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}
	cleanPath = resolved
	// Resolve the root the same way so the comparison is apples-to-apples.
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("cannot resolve staging root: %w", err)
	}
	rel, err := filepath.Rel(resolvedRoot, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", errors.New("path escapes staging root")
	}
	return cleanPath, nil
}

// RegisterRoutes mounts migration routes onto the router. All routes
// require case_admin or higher.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Public endpoint for signature verification — must not require auth.
	r.Get("/.well-known/vaultkeeper-signing-key", h.signingKeyJWKS)

	r.Route("/api/cases/{caseID}/migrations", func(r chi.Router) {
		r.Use(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit))
		r.Post("/", h.RunMigration)
		r.Get("/", h.ListMigrations)
	})
	r.Route("/api/migrations/{id}", func(r chi.Router) {
		r.Use(auth.RequireSystemRole(auth.RoleCaseAdmin, h.audit))
		r.Get("/", h.GetMigration)
		r.Get("/certificate", h.DownloadCertificate)
	})
}

type runMigrationRequest struct {
	SourceSystem      string  `json:"source_system"`
	SourceRoot        string  `json:"source_root"`
	ManifestPath      string  `json:"manifest_path"`
	ManifestFormat    string  `json:"manifest_format"`
	Concurrency       int     `json:"concurrency"`
	HaltOnMismatch    bool    `json:"halt_on_mismatch"`
	DryRun            bool    `json:"dry_run"`
	ResumeMigrationID *string `json:"resume_migration_id,omitempty"`
}

// RunMigration triggers a migration run. The manifest is read from
// `manifest_path` on the server filesystem — bulk evidence uploads are
// handled via the dedicated bulk endpoint. This POST returns the created
// migration record (or the partial record on halt).
//
// Important: manifest_path and source_root are operator-supplied server
// paths. The handler only accepts them from RoleCaseAdmin and above, and
// it does NOT attempt to sanitise them — an operator running a migration
// must have filesystem access already.
func (h *Handler) RunMigration(w http.ResponseWriter, r *http.Request) {
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
	var req runMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceSystem == "" || req.ManifestPath == "" || req.SourceRoot == "" {
		httputil.RespondError(w, http.StatusBadRequest, "source_system, manifest_path and source_root are required")
		return
	}
	format := ManifestFormat(req.ManifestFormat)
	if format == "" {
		format = FormatCSV
	}

	// Allowlist both operator-supplied paths against the configured
	// staging root. Without this, a case_admin could read arbitrary files.
	validatedManifest, err := h.validateStagedPath(req.ManifestPath)
	if err != nil {
		h.logger.Warn("migration manifest path rejected",
			"case_id", caseID, "actor", ac.UserID, "path", req.ManifestPath, "error", err)
		httputil.RespondError(w, http.StatusBadRequest, "manifest_path is not permitted")
		return
	}
	validatedRoot, err := h.validateStagedPath(req.SourceRoot)
	if err != nil {
		h.logger.Warn("migration source root rejected",
			"case_id", caseID, "actor", ac.UserID, "path", req.SourceRoot, "error", err)
		httputil.RespondError(w, http.StatusBadRequest, "source_root is not permitted")
		return
	}

	mf, err := os.Open(validatedManifest) // #nosec G304 -- validated against stagingRoot allowlist
	if err != nil {
		h.logger.Warn("migration manifest open failed", "error", err, "actor", ac.UserID)
		httputil.RespondError(w, http.StatusBadRequest, "cannot open manifest")
		return
	}
	defer mf.Close()

	var resumeID *uuid.UUID
	if req.ResumeMigrationID != nil && *req.ResumeMigrationID != "" {
		parsed, err := uuid.Parse(*req.ResumeMigrationID)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "invalid resume_migration_id")
			return
		}
		resumeID = &parsed
	}

	res, runErr := h.svc.Run(r.Context(), RunInput{
		CaseID:            caseID,
		SourceSystem:      req.SourceSystem,
		PerformedBy:       ac.UserID,
		ManifestSource:    mf,
		ManifestFormat:    format,
		SourceRoot:        validatedRoot,
		ResumeMigrationID: resumeID,
		Options: BatchOptions{
			Concurrency:    req.Concurrency,
			HaltOnMismatch: req.HaltOnMismatch,
			DryRun:         req.DryRun,
		},
	})
	if runErr != nil {
		// Log the full error server-side but NEVER return it to the
		// caller verbatim — HashMismatchError embeds absolute paths and
		// hash values, and wrapped OS errors embed filesystem layout.
		h.logger.Warn("migration run failed",
			"case_id", caseID, "actor", ac.UserID, "migration_id", res.Record.ID, "error", runErr)
		status := http.StatusInternalServerError
		message := "migration failed"
		switch {
		case IsHashMismatch(runErr):
			status = http.StatusConflict
			message = "migration halted: source hash mismatch"
		case errors.Is(runErr, ErrResumeManifestMismatch):
			status = http.StatusConflict
			message = "resume: manifest does not match existing migration"
		case errors.Is(runErr, ErrNotFound):
			status = http.StatusNotFound
			message = "resume: migration not found"
		}
		httputil.RespondJSON(w, status, map[string]any{
			"error":     message,
			"migration": res.Record,
			"halted":    res.Report.Halted,
		})
		return
	}
	httputil.RespondJSON(w, http.StatusCreated, map[string]any{
		"migration": res.Record,
		"processed": len(res.Report.Processed),
		"matched":   res.Report.MatchedItems,
	})
}

// ListMigrations returns all migration records for a case, newest first.
func (h *Handler) ListMigrations(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(chi.URLParam(r, "caseID"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid case id")
		return
	}
	records, err := h.svc.ListByCase(r.Context(), caseID)
	if err != nil {
		h.logger.Error("list migrations failed", "case_id", caseID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if records == nil {
		records = []Record{}
	}
	httputil.RespondJSON(w, http.StatusOK, records)
}

// GetMigration returns a migration record by ID.
func (h *Handler) GetMigration(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rec, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "migration not found")
			return
		}
		h.logger.Error("migration lookup failed", "id", id, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, rec)
}

// DownloadCertificate renders and returns the attestation PDF.
func (h *Handler) DownloadCertificate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	rec, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "migration not found")
			return
		}
		h.logger.Error("migration lookup failed for certificate", "id", id, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var caseRef, caseTitle string
	if h.cases != nil {
		info, cerr := h.cases.GetCaseInfo(r.Context(), rec.CaseID)
		if cerr == nil {
			caseRef = info.ReferenceCode
			caseTitle = info.Title
		}
	}

	// Note: the canonical file list is not persisted in 019 — a follow-up
	// migration can add a dedicated table if operators need to regenerate
	// bit-identical certificates later. For now the certificate rendered
	// post-hoc shows an empty appendix but a valid signature over the
	// record metadata, which is sufficient for integrity-only attestation.
	in := CertificateInput{
		Record:        rec,
		CaseReference: caseRef,
		CaseTitle:     caseTitle,
		InstanceVer:   h.instance,
		Locale:        firstAccept(r.Header.Get("Accept-Language")),
		GeneratedAt:   time.Now().UTC(),
		PublicKeyB64:  h.signer.PublicKeyBase64(),
	}
	cert, err := GenerateAttestationPDF(in, h.signer)
	if err != nil {
		h.logger.Error("certificate generation failed", "id", id, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "certificate generation failed")
		return
	}
	// X-Certificate-Appendix=empty signals to clients that the file
	// appendix was not available at regeneration time (see the sprint
	// 10 known gap: the canonical file list is not yet persisted, so
	// post-hoc certificates have an empty Appendix A and a different
	// signed body than the original).
	w.Header().Set("X-Certificate-Appendix", "empty")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="migration-attestation-`+rec.ID.String()+`.pdf"`)
	_, _ = w.Write(cert.PDFBytes)
}

// signingKeyJWKS returns the public key as JSON so verifiers can check
// certificate signatures without contacting an external PKI.
//
// Format is deliberately simple (not a full JWKS) — a thin JSON object
// with alg, public key, and a fingerprint. Clients that need JWKS can
// wrap this later.
func (h *Handler) signingKeyJWKS(w http.ResponseWriter, _ *http.Request) {
	if h.signer == nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "signing not configured")
		return
	}
	httputil.RespondJSON(w, http.StatusOK, map[string]any{
		"alg":       "Ed25519",
		"publicKey": h.signer.PublicKeyBase64(),
		"use":       "migration_attestation",
	})
}

// firstAccept picks the primary tag from an Accept-Language header.
func firstAccept(h string) string {
	if h == "" {
		return "en"
	}
	for i := 0; i < len(h); i++ {
		if h[i] == ',' || h[i] == ';' {
			return h[:i]
		}
	}
	return h
}

