package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// DatabaseChecker abstracts database connectivity checks.
type DatabaseChecker interface {
	Ping(ctx context.Context) error
}

// StorageChecker abstracts object storage connectivity checks.
type StorageChecker interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

// EvidenceCounter abstracts counting evidence rows for the detailed endpoint.
type EvidenceCounter interface {
	CountEvidence(ctx context.Context) (int64, error)
}

// BackupChecker provides the most recent backup information for the health endpoint.
type BackupChecker interface {
	GetLastBackupInfo(ctx context.Context) (completedAt time.Time, status string, err error)
}

const (
	healthCheckTimeout = 5 * time.Second
	defaultCacheTTL    = 60 * time.Second

	statusHealthy  = "healthy"
	statusDegraded = "degraded"
	statusUnhealthy = "unhealthy"

	serviceConnected    = "connected"
	serviceDisconnected = "disconnected"
)

// PublicHealth is the minimal response exposed without authentication.
type PublicHealth struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// DetailedHealth is the full response for system administrators.
type DetailedHealth struct {
	Status           string  `json:"status"`
	Version          string  `json:"version"`
	Postgres         string  `json:"postgres"`
	Minio            string  `json:"minio"`
	Meilisearch      string  `json:"meilisearch"`
	Keycloak         string  `json:"keycloak"`
	EvidenceCount    int64   `json:"evidence_count"`
	DiskUsagePercent float64 `json:"disk_usage_percent"`
	UptimeSeconds    int64   `json:"uptime_seconds"`
	LastBackup       string  `json:"last_backup,omitempty"`
	BackupStatus     string  `json:"backup_status,omitempty"`
}

// HealthHandler provides health check endpoints.
type HealthHandler struct {
	db             DatabaseChecker
	storage        StorageChecker
	storageBucket  string
	evidenceCount  EvidenceCounter
	backupChecker  BackupChecker
	searchURL      string
	keycloakURL    string
	keycloakRealm  string
	version        string
	startTime      time.Time
	audit          auth.AuditLogger
	httpClient     *http.Client

	cacheMu      sync.RWMutex
	cachedResult *DetailedHealth
	cachedAt     time.Time
	cacheTTL     time.Duration
}

// HealthHandlerOption applies optional configuration to HealthHandler.
type HealthHandlerOption func(*HealthHandler)

// WithEvidenceCounter sets an evidence counter for the detailed endpoint.
func WithEvidenceCounter(ec EvidenceCounter) HealthHandlerOption {
	return func(h *HealthHandler) { h.evidenceCount = ec }
}

// WithCacheTTL overrides the default 60-second cache duration.
func WithCacheTTL(ttl time.Duration) HealthHandlerOption {
	return func(h *HealthHandler) { h.cacheTTL = ttl }
}

// WithBackupChecker sets a backup checker for the detailed endpoint.
func WithBackupChecker(bc BackupChecker) HealthHandlerOption {
	return func(h *HealthHandler) { h.backupChecker = bc }
}

// NewHealthHandler constructs a HealthHandler with the required dependencies.
func NewHealthHandler(
	db DatabaseChecker,
	storage StorageChecker,
	storageBucket string,
	searchURL string,
	keycloakURL string,
	keycloakRealm string,
	version string,
	audit auth.AuditLogger,
	opts ...HealthHandlerOption,
) *HealthHandler {
	h := &HealthHandler{
		db:            db,
		storage:       storage,
		storageBucket: storageBucket,
		searchURL:     searchURL,
		keycloakURL:   keycloakURL,
		keycloakRealm: keycloakRealm,
		version:       version,
		startTime:     time.Now(),
		audit:         audit,
		cacheTTL:      defaultCacheTTL,
		httpClient: &http.Client{
			Timeout: healthCheckTimeout,
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers protected health routes under /api.
func (h *HealthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/health", func(r chi.Router) {
		r.With(auth.RequireSystemRole(auth.RoleSystemAdmin, h.audit)).Get("/", h.handleDetailedHealth)
	})
}

// PublicHealthCheck handles GET /health (no auth required).
func (h *HealthHandler) PublicHealthCheck(w http.ResponseWriter, r *http.Request) {
	result := h.getOrRefreshHealth(r.Context())

	status := statusHealthy
	if result.Postgres == serviceDisconnected || result.Minio == serviceDisconnected {
		status = statusUnhealthy
	}

	httputil.RespondJSON(w, httpStatusForHealth(status), PublicHealth{
		Status:  status,
		Version: h.version,
	})
}

// handleDetailedHealth handles GET /api/health (SystemAdmin only).
func (h *HealthHandler) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	result := h.getOrRefreshHealth(r.Context())
	httputil.RespondJSON(w, httpStatusForHealth(result.Status), result)
}

// getOrRefreshHealth returns a cached result if fresh, otherwise runs checks.
func (h *HealthHandler) getOrRefreshHealth(ctx context.Context) DetailedHealth {
	h.cacheMu.RLock()
	cached := h.cachedResult
	cachedAt := h.cachedAt
	h.cacheMu.RUnlock()

	if cached != nil && time.Since(cachedAt) < h.cacheTTL {
		// Return cached with updated uptime.
		result := *cached
		result.UptimeSeconds = int64(time.Since(h.startTime).Seconds())
		return result
	}

	result := h.runChecks(ctx)

	h.cacheMu.Lock()
	h.cachedResult = &result
	h.cachedAt = time.Now()
	h.cacheMu.Unlock()

	return result
}

// serviceCheck holds the outcome of a single service health probe.
type serviceCheck struct {
	name   string
	status string
	err    error
}

// runChecks probes all services concurrently and assembles the result.
func (h *HealthHandler) runChecks(ctx context.Context) DetailedHealth {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	results := make(chan serviceCheck, 4)

	go h.checkPostgres(checkCtx, results)
	go h.checkMinio(checkCtx, results)
	go h.checkMeilisearch(checkCtx, results)
	go h.checkKeycloak(checkCtx, results)

	services := make(map[string]string, 4)
	for i := 0; i < 4; i++ {
		sc := <-results
		services[sc.name] = sc.status
	}

	// Determine overall status.
	// Critical: postgres, minio. Non-critical: meilisearch, keycloak.
	criticalDown := services["postgres"] == serviceDisconnected ||
		services["minio"] == serviceDisconnected
	nonCriticalDown := services["meilisearch"] == serviceDisconnected ||
		services["keycloak"] == serviceDisconnected

	overallStatus := statusHealthy
	if criticalDown {
		overallStatus = statusUnhealthy
	} else if nonCriticalDown {
		overallStatus = statusDegraded
	}

	var evidenceCount int64
	if h.evidenceCount != nil {
		countCtx, countCancel := context.WithTimeout(ctx, healthCheckTimeout)
		defer countCancel()
		count, err := h.evidenceCount.CountEvidence(countCtx)
		if err == nil {
			evidenceCount = count
		}
	}

	var lastBackup string
	var backupStatus string
	if h.backupChecker != nil {
		backupCtx, backupCancel := context.WithTimeout(ctx, healthCheckTimeout)
		defer backupCancel()
		completedAt, bStatus, bErr := h.backupChecker.GetLastBackupInfo(backupCtx)
		if bErr == nil {
			lastBackup = completedAt.Format(time.RFC3339)
			backupStatus = bStatus
		}
	}

	return DetailedHealth{
		Status:           overallStatus,
		Version:          h.version,
		Postgres:         services["postgres"],
		Minio:            services["minio"],
		Meilisearch:      services["meilisearch"],
		Keycloak:         services["keycloak"],
		EvidenceCount:    evidenceCount,
		DiskUsagePercent: 0, // Placeholder: requires OS-level inspection
		UptimeSeconds:    int64(time.Since(h.startTime).Seconds()),
		LastBackup:       lastBackup,
		BackupStatus:     backupStatus,
	}
}

func (h *HealthHandler) checkPostgres(ctx context.Context, out chan<- serviceCheck) {
	err := h.db.Ping(ctx)
	status := serviceConnected
	if err != nil {
		status = serviceDisconnected
	}
	out <- serviceCheck{name: "postgres", status: status, err: err}
}

func (h *HealthHandler) checkMinio(ctx context.Context, out chan<- serviceCheck) {
	exists, err := h.storage.BucketExists(ctx, h.storageBucket)
	status := serviceConnected
	if err != nil || !exists {
		status = serviceDisconnected
	}
	out <- serviceCheck{name: "minio", status: status, err: err}
}

func (h *HealthHandler) checkMeilisearch(ctx context.Context, out chan<- serviceCheck) {
	url := fmt.Sprintf("%s/health", h.searchURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		out <- serviceCheck{name: "meilisearch", status: serviceDisconnected, err: err}
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		out <- serviceCheck{name: "meilisearch", status: serviceDisconnected, err: err}
		return
	}
	defer resp.Body.Close()

	status := serviceConnected
	if resp.StatusCode != http.StatusOK {
		status = serviceDisconnected
		err = fmt.Errorf("meilisearch returned status %d", resp.StatusCode)
	}
	out <- serviceCheck{name: "meilisearch", status: status, err: err}
}

func (h *HealthHandler) checkKeycloak(ctx context.Context, out chan<- serviceCheck) {
	url := fmt.Sprintf("%s/realms/%s", h.keycloakURL, h.keycloakRealm)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		out <- serviceCheck{name: "keycloak", status: serviceDisconnected, err: err}
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		out <- serviceCheck{name: "keycloak", status: serviceDisconnected, err: err}
		return
	}
	defer resp.Body.Close()

	status := serviceConnected
	if resp.StatusCode != http.StatusOK {
		status = serviceDisconnected
		err = fmt.Errorf("keycloak returned status %d", resp.StatusCode)
	}
	out <- serviceCheck{name: "keycloak", status: status, err: err}
}

func httpStatusForHealth(status string) int {
	if status == statusHealthy || status == statusDegraded {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}
