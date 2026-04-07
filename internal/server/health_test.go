package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Mock implementations ---

type mockDatabaseChecker struct {
	err error
}

func (m *mockDatabaseChecker) Ping(_ context.Context) error {
	return m.err
}

type mockStorageChecker struct {
	exists bool
	err    error
}

func (m *mockStorageChecker) BucketExists(_ context.Context, _ string) (bool, error) {
	return m.exists, m.err
}

type mockEvidenceCounter struct {
	count int64
	err   error
}

func (m *mockEvidenceCounter) CountEvidence(_ context.Context) (int64, error) {
	return m.count, m.err
}

type mockBackupChecker struct {
	completedAt time.Time
	status      string
	err         error
}

func (m *mockBackupChecker) GetLastBackupInfo(_ context.Context) (time.Time, string, error) {
	return m.completedAt, m.status, m.err
}

// --- Helpers ---

// envelope mirrors httputil's response wrapper.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error any             `json:"error"`
	Meta  any             `json:"meta"`
}

func newHealthHandler(
	t *testing.T,
	db DatabaseChecker,
	storage StorageChecker,
	searchServer *httptest.Server,
	keycloakServer *httptest.Server,
	opts ...HealthHandlerOption,
) *HealthHandler {
	t.Helper()

	searchURL := "http://unreachable-search:9999"
	if searchServer != nil {
		searchURL = searchServer.URL
	}
	keycloakURL := "http://unreachable-keycloak:9999"
	if keycloakServer != nil {
		keycloakURL = keycloakServer.URL
	}

	return NewHealthHandler(
		db, storage, "test-bucket",
		searchURL, keycloakURL, "testrealm",
		"1.0.0-test", nil, opts...,
	)
}

func healthySearchServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s
}

func unhealthySearchServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(s.Close)
	return s
}

func healthyKeycloakServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s
}

func unhealthyKeycloakServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(s.Close)
	return s
}

func decodePublicHealth(t *testing.T, body []byte) PublicHealth {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var ph PublicHealth
	if err := json.Unmarshal(env.Data, &ph); err != nil {
		t.Fatalf("unmarshal PublicHealth: %v", err)
	}
	return ph
}

func decodeDetailedHealth(t *testing.T, body []byte) DetailedHealth {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var dh DetailedHealth
	if err := json.Unmarshal(env.Data, &dh); err != nil {
		t.Fatalf("unmarshal DetailedHealth: %v", err)
	}
	return dh
}

// --- PublicHealthCheck tests ---

func TestPublicHealthCheck_AllHealthy(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusHealthy {
		t.Errorf("expected status %q, got %q", statusHealthy, ph.Status)
	}
	if ph.Version != "1.0.0-test" {
		t.Errorf("expected version %q, got %q", "1.0.0-test", ph.Version)
	}
}

func TestPublicHealthCheck_PostgresDown(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: errors.New("connection refused")},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusUnhealthy {
		t.Errorf("expected status %q, got %q", statusUnhealthy, ph.Status)
	}
}

func TestPublicHealthCheck_MinioDown(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: false, err: errors.New("connection refused")},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusUnhealthy {
		t.Errorf("expected status %q, got %q", statusUnhealthy, ph.Status)
	}
}

func TestPublicHealthCheck_MinioBucketNotExists(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: false, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusUnhealthy {
		t.Errorf("expected status %q, got %q", statusUnhealthy, ph.Status)
	}
}

func TestPublicHealthCheck_MeilisearchDown_StillHealthyPublic(t *testing.T) {
	// Meilisearch is non-critical, public endpoint only shows healthy/unhealthy
	// based on postgres+minio. Detailed shows "degraded" but public says "healthy".
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		unhealthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	// Public health only checks postgres+minio for unhealthy, so non-critical down
	// means we still get 200 and "healthy".
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusHealthy {
		t.Errorf("expected status %q, got %q", statusHealthy, ph.Status)
	}
}

func TestPublicHealthCheck_NeverExposesInternalDetails(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.PublicHealthCheck(rr, req)

	var env envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The data should only contain "status" and "version", nothing else.
	var raw map[string]any
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}

	allowedKeys := map[string]bool{"status": true, "version": true}
	for key := range raw {
		if !allowedKeys[key] {
			t.Errorf("public health response contains unexpected key %q", key)
		}
	}
}

// --- DetailedHealth (handleDetailedHealth) tests ---

func TestDetailedHealth_AllServicesUp(t *testing.T) {
	backupTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithEvidenceCounter(&mockEvidenceCounter{count: 42}),
		WithBackupChecker(&mockBackupChecker{
			completedAt: backupTime,
			status:      "completed",
		}),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusHealthy {
		t.Errorf("expected status %q, got %q", statusHealthy, dh.Status)
	}
	if dh.Postgres != serviceConnected {
		t.Errorf("expected postgres %q, got %q", serviceConnected, dh.Postgres)
	}
	if dh.Minio != serviceConnected {
		t.Errorf("expected minio %q, got %q", serviceConnected, dh.Minio)
	}
	if dh.Meilisearch != serviceConnected {
		t.Errorf("expected meilisearch %q, got %q", serviceConnected, dh.Meilisearch)
	}
	if dh.Keycloak != serviceConnected {
		t.Errorf("expected keycloak %q, got %q", serviceConnected, dh.Keycloak)
	}
	if dh.Version != "1.0.0-test" {
		t.Errorf("expected version %q, got %q", "1.0.0-test", dh.Version)
	}
	if dh.EvidenceCount != 42 {
		t.Errorf("expected evidence count 42, got %d", dh.EvidenceCount)
	}
	if dh.UptimeSeconds < 0 {
		t.Error("uptime should be non-negative")
	}
	if dh.LastBackup != backupTime.Format(time.RFC3339) {
		t.Errorf("expected last_backup %q, got %q", backupTime.Format(time.RFC3339), dh.LastBackup)
	}
	if dh.BackupStatus != "completed" {
		t.Errorf("expected backup_status %q, got %q", "completed", dh.BackupStatus)
	}
}

func TestDetailedHealth_NonCriticalDown_Degraded(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		unhealthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for degraded, got %d", rr.Code)
	}

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusDegraded {
		t.Errorf("expected status %q, got %q", statusDegraded, dh.Status)
	}
	if dh.Meilisearch != serviceDisconnected {
		t.Errorf("expected meilisearch %q, got %q", serviceDisconnected, dh.Meilisearch)
	}
}

func TestDetailedHealth_KeycloakDown_Degraded(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		unhealthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for degraded, got %d", rr.Code)
	}

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusDegraded {
		t.Errorf("expected status %q, got %q", statusDegraded, dh.Status)
	}
	if dh.Keycloak != serviceDisconnected {
		t.Errorf("expected keycloak %q, got %q", serviceDisconnected, dh.Keycloak)
	}
}

func TestDetailedHealth_CriticalDown_Unhealthy(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: errors.New("pg down")},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusUnhealthy {
		t.Errorf("expected status %q, got %q", statusUnhealthy, dh.Status)
	}
	if dh.Postgres != serviceDisconnected {
		t.Errorf("expected postgres %q, got %q", serviceDisconnected, dh.Postgres)
	}
}

func TestDetailedHealth_EvidenceCountError_ReturnsZero(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithEvidenceCounter(&mockEvidenceCounter{count: 0, err: errors.New("db error")}),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.EvidenceCount != 0 {
		t.Errorf("expected evidence count 0 on error, got %d", dh.EvidenceCount)
	}
}

func TestDetailedHealth_NoEvidenceCounter_ReturnsZero(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.EvidenceCount != 0 {
		t.Errorf("expected evidence count 0, got %d", dh.EvidenceCount)
	}
}

func TestDetailedHealth_UptimeCalculated(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)
	// Backdate the start time so uptime is measurably > 0.
	h.startTime = time.Now().Add(-10 * time.Second)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.UptimeSeconds < 9 {
		t.Errorf("expected uptime >= 9 seconds, got %d", dh.UptimeSeconds)
	}
}

func TestDetailedHealth_BackupInfoIncluded(t *testing.T) {
	backupTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithBackupChecker(&mockBackupChecker{
			completedAt: backupTime,
			status:      "success",
		}),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.LastBackup != backupTime.Format(time.RFC3339) {
		t.Errorf("expected last_backup %q, got %q", backupTime.Format(time.RFC3339), dh.LastBackup)
	}
	if dh.BackupStatus != "success" {
		t.Errorf("expected backup_status %q, got %q", "success", dh.BackupStatus)
	}
}

func TestDetailedHealth_BackupCheckerError_OmitsBackupFields(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithBackupChecker(&mockBackupChecker{err: errors.New("backup error")}),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.LastBackup != "" {
		t.Errorf("expected empty last_backup on error, got %q", dh.LastBackup)
	}
	if dh.BackupStatus != "" {
		t.Errorf("expected empty backup_status on error, got %q", dh.BackupStatus)
	}
}

func TestDetailedHealth_NoBackupChecker_OmitsBackupFields(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.LastBackup != "" {
		t.Errorf("expected empty last_backup, got %q", dh.LastBackup)
	}
	if dh.BackupStatus != "" {
		t.Errorf("expected empty backup_status, got %q", dh.BackupStatus)
	}
}

func TestDetailedHealth_BothNonCriticalDown_Degraded(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		unhealthySearchServer(t),
		unhealthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusDegraded {
		t.Errorf("expected status %q, got %q", statusDegraded, dh.Status)
	}
}

func TestDetailedHealth_CriticalAndNonCriticalDown_Unhealthy(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: errors.New("pg down")},
		&mockStorageChecker{exists: true, err: nil},
		unhealthySearchServer(t),
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Status != statusUnhealthy {
		t.Errorf("expected status %q, got %q", statusUnhealthy, dh.Status)
	}
}

// --- Caching tests ---

func TestCaching_SecondCallWithinTTLReturnsCached(t *testing.T) {
	callCount := 0
	db := &mockDatabaseChecker{err: nil}
	storage := &mockStorageChecker{exists: true, err: nil}

	searchSrv := healthySearchServer(t)
	keycloakSrv := healthyKeycloakServer(t)

	h := newHealthHandler(t, db, storage, searchSrv, keycloakSrv,
		WithCacheTTL(60*time.Second),
		WithEvidenceCounter(&mockEvidenceCounter{count: 10}),
	)

	// First call populates cache.
	req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr1 := httptest.NewRecorder()
	h.PublicHealthCheck(rr1, req1)
	_ = callCount

	// Close the servers so any real HTTP call would fail.
	searchSrv.Close()
	keycloakSrv.Close()

	// Make db and storage return errors -- but cache should prevent re-check.
	db.err = errors.New("should not be called")
	storage.err = errors.New("should not be called")

	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr2 := httptest.NewRecorder()
	h.PublicHealthCheck(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 from cache, got %d", rr2.Code)
	}

	ph := decodePublicHealth(t, rr2.Body.Bytes())
	if ph.Status != statusHealthy {
		t.Errorf("expected cached healthy status, got %q", ph.Status)
	}
}

func TestCaching_ExpiredCacheRefreshes(t *testing.T) {
	db := &mockDatabaseChecker{err: nil}
	storage := &mockStorageChecker{exists: true, err: nil}

	h := newHealthHandler(t, db, storage,
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithCacheTTL(1*time.Millisecond),
	)

	// First call populates cache.
	req1 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr1 := httptest.NewRecorder()
	h.handleDetailedHealth(rr1, req1)

	dh1 := decodeDetailedHealth(t, rr1.Body.Bytes())
	if dh1.Status != statusHealthy {
		t.Fatalf("expected healthy, got %q", dh1.Status)
	}

	// Wait for cache to expire.
	time.Sleep(5 * time.Millisecond)

	// Now make postgres fail -- cache is expired so it should re-check.
	db.err = errors.New("pg down")

	req2 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr2 := httptest.NewRecorder()
	h.handleDetailedHealth(rr2, req2)

	dh2 := decodeDetailedHealth(t, rr2.Body.Bytes())
	if dh2.Status != statusUnhealthy {
		t.Errorf("expected unhealthy after cache expiry, got %q", dh2.Status)
	}
}

func TestCaching_UptimeUpdatedOnCachedResult(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		healthyKeycloakServer(t),
		WithCacheTTL(10*time.Second),
	)
	h.startTime = time.Now().Add(-100 * time.Second)

	// First call
	req1 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr1 := httptest.NewRecorder()
	h.handleDetailedHealth(rr1, req1)
	dh1 := decodeDetailedHealth(t, rr1.Body.Bytes())

	// Second call (cached) -- uptime should still reflect real time
	time.Sleep(5 * time.Millisecond)
	req2 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr2 := httptest.NewRecorder()
	h.handleDetailedHealth(rr2, req2)
	dh2 := decodeDetailedHealth(t, rr2.Body.Bytes())

	if dh2.UptimeSeconds < dh1.UptimeSeconds {
		t.Error("uptime should not decrease on cached result")
	}
}

// --- Functional options tests ---

func TestWithEvidenceCounter(t *testing.T) {
	ec := &mockEvidenceCounter{count: 99}
	h := NewHealthHandler(
		&mockDatabaseChecker{}, &mockStorageChecker{exists: true}, "b",
		"http://search", "http://kc", "realm", "v", nil,
		WithEvidenceCounter(ec),
	)
	if h.evidenceCount != ec {
		t.Error("WithEvidenceCounter did not set evidence counter")
	}
}

func TestWithCacheTTL(t *testing.T) {
	h := NewHealthHandler(
		&mockDatabaseChecker{}, &mockStorageChecker{exists: true}, "b",
		"http://search", "http://kc", "realm", "v", nil,
		WithCacheTTL(5*time.Minute),
	)
	if h.cacheTTL != 5*time.Minute {
		t.Errorf("expected cacheTTL 5m, got %v", h.cacheTTL)
	}
}

func TestWithBackupChecker(t *testing.T) {
	bc := &mockBackupChecker{}
	h := NewHealthHandler(
		&mockDatabaseChecker{}, &mockStorageChecker{exists: true}, "b",
		"http://search", "http://kc", "realm", "v", nil,
		WithBackupChecker(bc),
	)
	if h.backupChecker != bc {
		t.Error("WithBackupChecker did not set backup checker")
	}
}

func TestDefaultCacheTTL(t *testing.T) {
	h := NewHealthHandler(
		&mockDatabaseChecker{}, &mockStorageChecker{exists: true}, "b",
		"http://search", "http://kc", "realm", "v", nil,
	)
	if h.cacheTTL != defaultCacheTTL {
		t.Errorf("expected default cacheTTL %v, got %v", defaultCacheTTL, h.cacheTTL)
	}
}

// --- httpStatusForHealth tests ---

func TestHttpStatusForHealth(t *testing.T) {
	tests := []struct {
		status   string
		expected int
	}{
		{statusHealthy, http.StatusOK},
		{statusDegraded, http.StatusOK},
		{statusUnhealthy, http.StatusServiceUnavailable},
		{"unknown", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := httpStatusForHealth(tt.status)
			if got != tt.expected {
				t.Errorf("httpStatusForHealth(%q) = %d, want %d", tt.status, got, tt.expected)
			}
		})
	}
}

// --- Meilisearch HTTP check tests ---

func TestCheckMeilisearch_Unreachable(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		nil, // nil search server = unreachable URL
		healthyKeycloakServer(t),
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Meilisearch != serviceDisconnected {
		t.Errorf("expected meilisearch %q, got %q", serviceDisconnected, dh.Meilisearch)
	}
}

func TestCheckMeilisearch_InvalidURL(t *testing.T) {
	// A URL with a control character makes NewRequestWithContext fail.
	h := NewHealthHandler(
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		"test-bucket",
		"http://invalid\x00url", // triggers NewRequestWithContext error
		"http://unreachable-keycloak:9999",
		"testrealm", "1.0.0-test", nil,
	)

	out := make(chan serviceCheck, 1)
	h.checkMeilisearch(context.Background(), out)
	sc := <-out

	if sc.status != serviceDisconnected {
		t.Errorf("expected %q, got %q", serviceDisconnected, sc.status)
	}
	if sc.err == nil {
		t.Error("expected error for invalid URL")
	}
}

// --- Keycloak HTTP check tests ---

func TestCheckKeycloak_Unreachable(t *testing.T) {
	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		healthySearchServer(t),
		nil, // nil keycloak server = unreachable URL
	)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	h.handleDetailedHealth(rr, req)

	dh := decodeDetailedHealth(t, rr.Body.Bytes())
	if dh.Keycloak != serviceDisconnected {
		t.Errorf("expected keycloak %q, got %q", serviceDisconnected, dh.Keycloak)
	}
}

func TestCheckKeycloak_InvalidURL(t *testing.T) {
	h := NewHealthHandler(
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		"test-bucket",
		"http://unreachable-search:9999",
		"http://invalid\x00url", // triggers NewRequestWithContext error
		"testrealm", "1.0.0-test", nil,
	)

	out := make(chan serviceCheck, 1)
	h.checkKeycloak(context.Background(), out)
	sc := <-out

	if sc.status != serviceDisconnected {
		t.Errorf("expected %q, got %q", serviceDisconnected, sc.status)
	}
	if sc.err == nil {
		t.Error("expected error for invalid URL")
	}
}

// --- NewHealthHandler tests ---

func TestNewHealthHandler_SetsFields(t *testing.T) {
	db := &mockDatabaseChecker{}
	storage := &mockStorageChecker{}
	h := NewHealthHandler(db, storage, "mybucket",
		"http://search:7700", "http://kc:8080", "master",
		"2.0.0", nil,
	)

	if h.db != db {
		t.Error("db not set")
	}
	if h.storage != storage {
		t.Error("storage not set")
	}
	if h.storageBucket != "mybucket" {
		t.Errorf("storageBucket = %q, want %q", h.storageBucket, "mybucket")
	}
	if h.searchURL != "http://search:7700" {
		t.Errorf("searchURL = %q", h.searchURL)
	}
	if h.keycloakURL != "http://kc:8080" {
		t.Errorf("keycloakURL = %q", h.keycloakURL)
	}
	if h.keycloakRealm != "master" {
		t.Errorf("keycloakRealm = %q", h.keycloakRealm)
	}
	if h.version != "2.0.0" {
		t.Errorf("version = %q", h.version)
	}
	if h.startTime.IsZero() {
		t.Error("startTime not set")
	}
	if h.httpClient == nil {
		t.Error("httpClient not set")
	}
}
