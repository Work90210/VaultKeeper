# Sprint 21: Kubernetes Helm Chart & Compliance Preparation

**Phase:** 4 — Enterprise & Scale
**Duration:** Weeks 41-42
**Goal:** Package VaultKeeper for high-availability Kubernetes deployment and prepare for ISO 27001/SOC 2 compliance certifications. Ship v4.0.0.

---

## Prerequisites

- Phase 3 complete, Phase 4 sprints 19-20 done
- All features working in Docker Compose
- Production deployments validated

---

## Task Type

- [x] Infrastructure (Kubernetes/Helm)
- [x] Backend (Go — HA readiness)
- [x] Documentation (Compliance)

---

## Implementation Steps

### Step 1: Go Server HA Readiness

**Deliverable:** Ensure Go server is fully stateless and horizontally scalable.

**Checklist:**
- [ ] No in-memory state that doesn't survive restart
- [ ] JWKS cache: shared via Redis or refetched per instance
- [ ] Custody chain advisory locks: work across multiple instances (Postgres advisory locks are cluster-wide)
- [ ] Background jobs (backup, integrity verification): use Postgres-based distributed locking to prevent duplicate execution
- [ ] tus upload: shared storage backend (MinIO, not local disk)
- [ ] AI job queue: Postgres-based, workers on any instance can pick up jobs
- [ ] Session management: stateless JWT (no server-side sessions)
- [ ] Rate limiting: per-user via Redis (shared across instances) instead of per-instance memory

**Distributed locking:**
```go
// Postgres advisory lock for distributed coordination
func AcquireDistributedLock(ctx context.Context, db *pgxpool.Pool, lockID int64) (bool, error) {
    var acquired bool
    err := db.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
    return acquired, err
}
```

**Health endpoint enhancement:**
- `/health/live` → liveness probe (process alive)
- `/health/ready` → readiness probe (all dependencies connected)

**Tests:**
- Two server instances → both serve requests correctly
- Background job runs on only one instance (distributed lock)
- Rate limiting shared across instances (Redis)
- Health probes return correct status

### Step 2: Helm Chart

**Deliverable:** Production-ready Helm chart for Kubernetes deployment.

**Chart structure:**
```
charts/vaultkeeper/
├── Chart.yaml
├── values.yaml
├── values-production.yaml
├── values-staging.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── deployment.yaml          # Go API server (replicas configurable)
│   ├── service.yaml             # ClusterIP service
│   ├── ingress.yaml             # Ingress with TLS
│   ├── hpa.yaml                 # Horizontal Pod Autoscaler
│   ├── pdb.yaml                 # Pod Disruption Budget
│   ├── configmap.yaml           # Non-sensitive config
│   ├── secret.yaml              # Sensitive config (or ExternalSecrets)
│   ├── postgres/
│   │   ├── statefulset.yaml     # PostgreSQL (or use managed PG)
│   │   ├── service.yaml
│   │   └── pvc.yaml
│   ├── minio/
│   │   ├── statefulset.yaml     # MinIO (or use managed S3-compatible)
│   │   ├── service.yaml
│   │   └── pvc.yaml
│   ├── keycloak/
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   ├── meilisearch/
│   │   ├── statefulset.yaml
│   │   ├── service.yaml
│   │   └── pvc.yaml
│   ├── redis/
│   │   ├── deployment.yaml      # For shared rate limiting + caching
│   │   └── service.yaml
│   ├── cronjob-backup.yaml      # Scheduled backup job
│   ├── cronjob-integrity.yaml   # Scheduled integrity verification
│   ├── networkpolicy.yaml       # Network isolation rules
│   └── serviceaccount.yaml
└── tests/
    └── test-connection.yaml
```

**values.yaml key sections:**
```yaml
replicaCount: 2

image:
  repository: ghcr.io/vaultkeeper/vaultkeeper
  tag: "4.0.0"

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilization: 70

postgres:
  enabled: true               # false if using managed PG
  persistence:
    size: 100Gi
    storageClass: ssd

minio:
  enabled: true               # false if using managed S3
  persistence:
    size: 1Ti
    storageClass: ssd

ingress:
  enabled: true
  className: nginx
  tls:
    enabled: true
    secretName: vaultkeeper-tls

networkPolicy:
  enabled: true               # restrict pod-to-pod communication

resources:
  api:
    limits:
      cpu: "2"
      memory: 4Gi
    requests:
      cpu: "500m"
      memory: 1Gi
```

**Network policies:**
- API pods → Postgres, MinIO, Meilisearch, Keycloak, Redis (allowed)
- API pods → internet (blocked, except TSA endpoint)
- Postgres, MinIO, Meilisearch → internet (blocked)
- Ingress → API pods only

**Tests:**
- `helm lint` passes
- `helm template` produces valid YAML
- `helm install --dry-run` succeeds
- Network policies block unauthorized traffic
- HPA scales pods under load
- PDB prevents all pods from being evicted simultaneously
- Pod restart → no data loss (PVCs intact)
- Rolling update → zero downtime

### Step 3: Monitoring & Observability for K8s

**Deliverable:** Prometheus metrics + Grafana dashboards.

**Prometheus metrics exposed at `/metrics`:**
```
vaultkeeper_http_requests_total{method, path, status}
vaultkeeper_http_request_duration_seconds{method, path}
vaultkeeper_evidence_upload_total
vaultkeeper_evidence_upload_size_bytes
vaultkeeper_custody_log_entries_total
vaultkeeper_integrity_verification_total{result}
vaultkeeper_backup_total{status}
vaultkeeper_ai_jobs_total{type, status}
vaultkeeper_active_users_current
vaultkeeper_evidence_items_total
vaultkeeper_storage_bytes_used
```

**Grafana dashboards:**
- Request rate, latency, error rate
- Evidence upload rate + size
- Storage utilization
- AI job queue depth + processing time
- Backup status
- Per-instance resource usage

**Alerting rules:**
- Error rate > 5% → warning
- P99 latency > 5s → warning
- Integrity verification failure → critical
- Backup failure → critical
- Disk usage > 85% → warning
- Disk usage > 95% → critical

### Step 4: Compliance Documentation

**Deliverable:** Documentation package for ISO 27001 / SOC 2 readiness.

**Documents:**
1. **Security Architecture Document**
   - Encryption (in transit, at rest, per-case)
   - Authentication + authorization model
   - Network isolation
   - Key management
   - Audit logging

2. **Data Processing Agreement (DPA) Template**
   - GDPR-compliant template
   - Data controller/processor roles
   - Data retention and deletion
   - Sub-processor list (Hetzner only for managed hosting)

3. **GDPR Compliance Statement** (separate from DPA — procurement teams expect a standalone document)
   - Data residency: all data stored in EU (Germany — Hetzner Falkenstein/Nuremberg datacenters)
   - No data transfers outside EU (no US cloud providers, no telemetry, no analytics)
   - Right to erasure: supported with GDPR conflict resolution workflow (Sprint 9)
   - Data portability: full case export as ZIP with open formats (CSV, JSON)
   - Privacy by design: encryption at rest + in transit, role-based access, witness identity encryption
   - Data breach notification: 72-hour timeline per GDPR Article 33
   - Data Protection Officer contact template
   - Note on Netherlands vs Germany data residency: "Hosted in Germany is fine under GDPR (EU-to-EU, no transfer issue). If an institution specifically requires Dutch data residency, deployment on their own infrastructure or a Dutch provider is available."

4. **Incident Response Plan**
   - Security incident classification
   - Response procedures
   - Notification timelines (GDPR: 72 hours)
   - Post-incident review

4. **Business Continuity Plan**
   - RTO: < 1 hour
   - RPO: < 24 hours
   - Backup verification schedule
   - Disaster recovery procedures

5. **Access Control Policy**
   - Role definitions
   - Principle of least privilege
   - Account lifecycle (creation, review, deactivation)
   - MFA requirements

6. **Change Management Policy**
   - Code review requirements
   - Testing requirements
   - Deployment procedures
   - Rollback procedures

7. **Penetration Test Report Template**
   - Scope definition
   - Methodology
   - Findings format
   - Remediation tracking

**Location:** `docs/compliance/`

### Step 5: v4.0.0 Release

**Checklist:**
- [ ] All Phase 4 features working
- [ ] Helm chart tested on K8s cluster
- [ ] HPA, PDB, network policies configured
- [ ] Prometheus metrics exported
- [ ] Grafana dashboards created
- [ ] Compliance documentation complete
- [ ] Mobile app tested on iOS + Android
- [ ] Cross-case analytics working
- [ ] Workflow engine operational
- [ ] All tests passing, coverage >= 80%
- [ ] CHANGELOG.md v4.0.0
- [ ] Docker image + Helm chart tagged v4.0.0

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `charts/vaultkeeper/*` | Create | Helm chart |
| `internal/server/metrics.go` | Create | Prometheus metrics |
| `internal/server/health.go` | Modify | K8s liveness + readiness probes |
| `internal/distributed/lock.go` | Create | Distributed locking via Postgres |
| `docs/compliance/*` | Create | Compliance documentation package |
| `.github/workflows/helm.yml` | Create | Helm chart CI (lint, test, publish) |

---

## Definition of Done

- [ ] Helm chart deploys VaultKeeper on Kubernetes
- [ ] HPA scales API pods under load
- [ ] Network policies isolate services
- [ ] Zero-downtime rolling updates
- [ ] Distributed locking prevents duplicate background jobs
- [ ] Prometheus metrics exported
- [ ] Grafana dashboards operational
- [ ] Compliance documentation complete
- [ ] DPA template ready for institutional use
- [ ] GDPR compliance statement ready for procurement evaluation
- [ ] v4.0.0 released

---

## Security Checklist

- [ ] Kubernetes secrets encrypted at rest (etcd encryption)
- [ ] Network policies block unauthorized pod communication
- [ ] Service accounts with minimal RBAC permissions
- [ ] Container images scanned for vulnerabilities (Trivy)
- [ ] No privileged containers
- [ ] Pod security standards enforced (restricted profile)
- [ ] Ingress TLS configured correctly
- [ ] Secrets not in Helm values (use ExternalSecrets operator)
- [ ] Compliance documentation reviewed by security professional

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 21 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/distributed/lock.go`** — `AcquireDistributedLock`: returns true when lock available; returns false when lock already held by another connection; released lock can be re-acquired; lock ID is deterministic for given input
- **`internal/server/health.go`** — `/health/live`: returns 200 when process is running; returns correct JSON body
- **`internal/server/health.go`** — `/health/ready`: returns 200 when Postgres, MinIO, Meilisearch, Keycloak, and Redis all reachable; returns 503 when any dependency unreachable; response body lists each dependency status
- **`internal/server/metrics.go`** — Prometheus counter `vaultkeeper_http_requests_total`: increments on each request with correct method, path, and status labels
- **`internal/server/metrics.go`** — Prometheus histogram `vaultkeeper_http_request_duration_seconds`: records request duration; buckets are appropriate for expected latency range
- **`internal/server/metrics.go`** — all custom metrics registered: `evidence_upload_total`, `evidence_upload_size_bytes`, `custody_log_entries_total`, `integrity_verification_total`, `backup_total`, `ai_jobs_total`, `active_users_current`, `evidence_items_total`, `storage_bytes_used`
- **Rate limiting via Redis** — rate limit state shared across two service instances (mocked); per-user limit enforced globally not per-instance
- **Background job distributed locking** — only one instance acquires lock for backup job; only one instance acquires lock for integrity verification job; second instance skips job gracefully
- **Stateless server verification** — no in-memory state persists between requests that would fail on a different instance (JWKS, sessions, rate counters all externalized)
- **Helm chart validation** — `helm lint` passes with default values; `helm lint` passes with production values; `helm lint` passes with staging values
- **Helm template rendering** — `helm template` with default values produces valid Kubernetes YAML; all required resources present (deployment, service, ingress, HPA, PDB, configmap, networkpolicy, serviceaccount)
- **Helm template conditional rendering** — postgres.enabled=false omits Postgres StatefulSet; minio.enabled=false omits MinIO StatefulSet; networkPolicy.enabled=false omits NetworkPolicy; autoscaling.enabled=false omits HPA
- **HPA configuration** — minReplicas >= 2; targetCPUUtilization between 50-90; maxReplicas > minReplicas
- **PDB configuration** — maxUnavailable or minAvailable set such that at least 1 pod always running
- **Network policy rules** — API pods can reach Postgres, MinIO, Meilisearch, Keycloak, Redis; API pods cannot reach internet (except TSA); data stores cannot reach internet; ingress can only reach API pods

### Integration Tests (testcontainers)

- **Two-instance HA test** — spin up 2 Go server instances against same Postgres and Redis, send requests to both, verify both serve correctly; kill one instance, verify remaining instance handles all traffic
- **Distributed lock contention** — spin up 2 instances, both attempt to acquire backup job lock simultaneously, verify exactly one succeeds and the other skips
- **Shared rate limiting** — spin up 2 instances sharing Redis, send 30 requests to instance A and 30 to instance B for the same API key (limit 60), verify request 61 (on either instance) returns 429
- **Health probe integration** — start server with all dependencies running, verify `/health/ready` returns 200; stop Postgres, verify `/health/ready` returns 503; restart Postgres, verify `/health/ready` recovers to 200
- **Prometheus metrics scrape** — start server, make 10 requests, scrape `/metrics` endpoint, verify `vaultkeeper_http_requests_total` shows count of 10; verify histogram has observations
- **Helm install dry-run** — run `helm install --dry-run` against a kind/k3s cluster in testcontainer, verify no errors; validate all resources would be created
- **Rolling update zero-downtime** — deploy v1 to k3s cluster, start continuous requests, deploy v2 via rolling update, verify zero failed requests during rollout
- **Network policy enforcement** — deploy to k3s with network policies, attempt connection from API pod to external URL, verify blocked; attempt connection from API pod to Postgres, verify allowed

### E2E Automated Tests (Playwright)

Note: Sprint 21 E2E tests combine infrastructure validation (via kubectl/helm CLI) and web UI verification.

- **Helm deployment verification** — after `helm install`, verify all pods reach Running state; verify services have ClusterIP assigned; verify ingress has address assigned
- **Application accessible via ingress** — navigate to the ingress URL, verify VaultKeeper login page loads; log in; verify dashboard renders with data
- **Health endpoints accessible** — GET /health/live returns 200; GET /health/ready returns 200 with all dependencies "ok"
- **Prometheus metrics endpoint** — GET /metrics returns Prometheus text format with all expected metric names
- **HPA scaling behavior** — generate load (50 concurrent users), observe HPA scales pods from 2 to 3+; reduce load, observe HPA scales back down
- **Compliance docs accessible** — verify `docs/compliance/` directory contains all required documents: Security Architecture, DPA Template, GDPR Statement, Incident Response Plan, Business Continuity Plan, Access Control Policy, Change Management Policy, Penetration Test Report Template
- **Grafana dashboard loads** — if Grafana deployed, navigate to Grafana, verify VaultKeeper dashboard loads with panels for request rate, latency, error rate, and storage utilization

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Run `helm install vaultkeeper charts/vaultkeeper/ -f charts/vaultkeeper/values-production.yaml` on a Kubernetes cluster (or staging equivalent)
   **Expected:** All pods start and reach Running status within 5 minutes: API server (2+ replicas), Postgres, MinIO, Meilisearch, Keycloak, Redis; all services have ClusterIP; ingress has external address
   **Verify:** `kubectl get pods` shows all pods Running with READY containers; `kubectl get svc` shows all services; `kubectl get ingress` shows address and TLS configured; no CrashLoopBackOff or Error states

2. [ ] **Action:** Access the VaultKeeper web UI via the ingress URL (HTTPS)
   **Expected:** Login page loads over HTTPS with valid TLS certificate; log in with Keycloak credentials; dashboard renders with all features operational
   **Verify:** Browser shows padlock icon (valid TLS); no mixed content warnings; all pages load correctly; evidence upload/download works through the ingress

3. [ ] **Action:** Check the HPA configuration and trigger autoscaling by generating sustained load (e.g., 100 concurrent users via k6 or similar load testing tool for 5 minutes)
   **Expected:** HPA detects CPU utilization above target (70%); scales API pods from 2 to 3 or more within 2 minutes; request latency remains acceptable during scaling
   **Verify:** `kubectl get hpa` shows TARGETS above threshold and REPLICAS increased; `kubectl get pods` shows additional API pods Running; after load stops, pods scale back down within 5 minutes

4. [ ] **Action:** Verify network policies by attempting unauthorized network access from within a pod
   **Expected:** API pod can connect to Postgres (port 5432), MinIO (port 9000), Meilisearch (port 7700), Keycloak (port 8080), Redis (port 6379); API pod CANNOT connect to external internet (except TSA endpoint); Postgres pod CANNOT connect to external internet
   **Verify:** `kubectl exec` into API pod, verify `curl postgres:5432` succeeds; verify `curl https://example.com` is blocked (timeout or connection refused); exec into Postgres pod, verify external access blocked

5. [ ] **Action:** Access the Prometheus metrics endpoint at `/metrics` on the API server
   **Expected:** Prometheus text format response with all VaultKeeper custom metrics: http_requests_total, http_request_duration_seconds, evidence_upload_total, custody_log_entries_total, integrity_verification_total, backup_total, ai_jobs_total, active_users_current, evidence_items_total, storage_bytes_used
   **Verify:** Each metric has appropriate labels (method, path, status for HTTP metrics); counters are incrementing correctly after making a few requests; histograms have observations in expected buckets

6. [ ] **Action:** If Grafana is deployed, open the VaultKeeper dashboard
   **Expected:** Dashboard loads with panels for: request rate (req/s), request latency (p50, p95, p99), error rate (%), evidence upload rate, storage utilization, AI job queue depth, backup status
   **Verify:** Panels show real data (not "No data"); time range selector works; panels refresh automatically; alert rules visible for critical thresholds (integrity failure, backup failure, disk > 95%)

7. [ ] **Action:** Simulate a pod failure by deleting one API server pod while the other is running
   **Expected:** Kubernetes restarts the deleted pod automatically; during the restart, the remaining pod handles all traffic; no user-visible downtime; PDB prevents both pods from being deleted simultaneously
   **Verify:** `kubectl delete pod <api-pod-1>`; immediately access the UI and verify it works; `kubectl get pods` shows replacement pod starting; within 60 seconds, 2 pods running again; attempt to delete both pods simultaneously via `kubectl delete pod <both>` and verify PDB blocks it

8. [ ] **Action:** Perform a rolling update by changing the image tag in values.yaml and running `helm upgrade`
   **Expected:** New pods start with updated image while old pods continue serving traffic; once new pods are ready, old pods terminate; zero requests fail during the rollout
   **Verify:** During upgrade, run continuous requests (curl loop or k6); verify zero 5xx errors; `kubectl rollout status deployment/vaultkeeper-api` shows successful rollout; `kubectl get pods` shows all pods running new image

9. [ ] **Action:** Review all compliance documents in `docs/compliance/` for completeness and accuracy
   **Expected:** All 7+ documents present: Security Architecture, DPA Template, GDPR Compliance Statement, Incident Response Plan, Business Continuity Plan, Access Control Policy, Change Management Policy, Penetration Test Report Template
   **Verify:** Each document is internally consistent; Security Architecture matches actual implementation (encryption, auth, network isolation); DPA Template includes all GDPR-required clauses; GDPR statement addresses EU data residency, right to erasure, data portability; Incident Response Plan has 72-hour notification timeline; Business Continuity Plan has RTO < 1 hour and RPO < 24 hours

10. [ ] **Action:** Verify Kubernetes secrets are not exposed in Helm values or pod environment
    **Expected:** Sensitive values (master encryption key, database password, Keycloak admin password, API key secrets) are NOT in values.yaml; they are referenced via ExternalSecrets or Kubernetes Secret objects; `kubectl get secret` shows secrets exist; pod env vars reference secrets, not plaintext
    **Verify:** `helm get values vaultkeeper` does not contain any passwords or keys; `kubectl describe pod <api-pod>` shows env vars sourced from Secret references; etcd encryption is enabled for the cluster (verify via cluster admin)
