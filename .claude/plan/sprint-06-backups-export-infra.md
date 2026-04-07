# Sprint 6: Backups, Data Export, i18n & Infrastructure

**Phase:** 1 — Foundation
**Duration:** Weeks 11-12
**Goal:** Complete Phase 1 with automated encrypted backups, full case export, i18n wiring, Terraform/Ansible scaffolding, and final polish for a deployable v1.0.0 release.

---

## Prerequisites

- Sprints 1-5 complete (full stack running, cases, evidence, search, notifications, health)

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)
- [x] Infrastructure (Terraform/Ansible)

---

## Implementation Steps

### Step 1: Automated Backup Runner (`internal/backup/runner.go`)

**Deliverable:** Scheduled backup job — Postgres dump + MinIO snapshot → encrypted archive.

**Interface:**
```go
type BackupRunner interface {
    RunBackup(ctx context.Context) (BackupResult, error)
    RestoreBackup(ctx context.Context, backupPath string) error
    ListBackups(ctx context.Context) ([]BackupInfo, error)
    VerifyBackup(ctx context.Context, backupPath string) (BackupVerification, error)
}
```

**Backup flow:**
1. Write `backup_log` entry: status="started"
2. Dump Postgres via `pg_dump` (binary format, compressed)
3. Snapshot MinIO evidence bucket via `mc mirror`
4. Create combined archive (tar.gz)
5. Encrypt archive with AES-256-GCM using `BACKUP_ENCRYPTION_KEY`
6. Upload to `BACKUP_DESTINATION` (SFTP, S3, or local directory)
7. Verify upload (checksum comparison)
8. Update `backup_log`: status="completed", file_count, total_size
9. Clean old backups (retain 30 days by default)

**Scheduling:**
- Go `time.Ticker` goroutine (not OS cron — portable)
- Default schedule: daily at 03:00 UTC (configurable)
- Also callable on-demand via admin API

**Encryption:**
- AES-256-GCM (authenticated encryption)
- Key from `BACKUP_ENCRYPTION_KEY` env var
- Include key derivation (PBKDF2 or Argon2) for password-based keys
- Store encryption metadata header in backup file (algorithm, IV, key derivation params — NOT the key)

**Error handling (per spec):**
- Backup fails → backup_log status="failed", error_message logged
- Admin notification sent
- Next scheduled backup retries
- After 3 consecutive failures → CRITICAL alert

**Tests:**
- Full backup completes successfully
- Backup archive is encrypted (cannot decompress without key)
- Restore from backup → data matches original (CRITICAL: full restore test)
- Postgres dump includes all tables and data
- MinIO snapshot includes all evidence files
- Backup log entry created correctly
- Scheduled run triggers at configured time
- Failed backup → correct error handling + notification
- 3 consecutive failures → CRITICAL notification
- Old backups cleaned up (>30 days)
- Backup destination unreachable → logged, retried
- Backup verification → checksum matches
- On-demand backup via API works

**Disaster Recovery Test (must be automated, run quarterly):**
1. Create test data: case + evidence items + custody log entries
2. Run full backup
3. Destroy all data (drop database, clear MinIO bucket)
4. Restore from backup
5. Verify: all evidence items present with correct hashes
6. Verify: custody chain intact (hash chain valid)
7. Verify: all cases, roles, notifications restored
8. Measure RTO (target: < 1 hour)
9. Measure RPO (verify: data loss < 24 hours)
10. Log results to backup_log with `backup_type: "restore_test"`

**Restore procedure (documented in README and tested):**
```bash
# 1. Stop the application
docker compose down
# 2. Restore Postgres
gunzip < backup.sql.gz | docker compose exec -T postgres psql -U vaultkeeper
# 3. Restore MinIO
docker compose exec minio mc mirror /backup/minio-snapshot/ /data/
# 4. Start + verify
docker compose up -d
curl -sf https://instance.vaultkeeper.eu/health
# 5. Run integrity verification
curl -X POST https://instance.vaultkeeper.eu/api/cases/{id}/verify
```

### Step 2: Backup Admin Endpoints

**Deliverable:** Admin endpoints for backup management.

```
POST   /api/admin/backups/run           → Trigger on-demand backup (System Admin)
GET    /api/admin/backups               → List backup history (System Admin)
GET    /api/admin/backups/:id/verify    → Verify backup integrity (System Admin)
```

### Step 3: Case Export (`internal/cases/export.go`)

**Deliverable:** Full case export as ZIP archive.

**Export contents:**
```
ICC-UKR-2024-export/
├── manifest.json          # Export metadata (date, version, case info, total items)
├── evidence/
│   ├── ICC-UKR-2024-00001_document.pdf
│   ├── ICC-UKR-2024-00002_photo.jpg
│   └── ...
├── metadata.csv           # All evidence metadata (columns match evidence_items)
├── custody_log.csv        # Full custody chain for all items
├── case.json              # Case details + role assignments
├── hashes.csv             # SHA-256 + TSA status for each item
└── README.txt             # Explanation of export format, how to verify
```

**Implementation:**
- Stream ZIP creation (don't buffer entire archive in memory)
- Include SHA-256 hash of each file in manifest
- Include all custody log entries
- Include case metadata and role assignments (no user PII — just Keycloak user IDs)
- Export is a custody event → logged in custody_log
- Defence users: export only contains disclosed evidence

**Tests:**
- Export produces valid ZIP
- All evidence files present
- metadata.csv matches Postgres data
- custody_log.csv complete
- Hashes in hashes.csv match file hashes
- Large case (1000 items) → streaming works, no OOM
- Defence user → only disclosed items in export
- Export logged in custody chain

### Step 4: Custody Report PDF (`internal/reports/pdf.go`)

**Deliverable:** Generate court-submittable custody report PDF.

**Report contents:**
- Header: VaultKeeper logo, case reference, report date
- Evidence item details: number, title, hash, TSA timestamp, upload date
- Complete custody chain: every action, who, when, IP, hash at time of action
- Hash chain verification status
- RFC 3161 timestamp verification for each entry
- Digital signature (self-signed for now, institution PKI in Phase 2)
- Footer: page numbers, generation timestamp

**Implementation:**
- Use `github.com/jung-kurt/gofpdf` or `github.com/unidoc/unipdf` for PDF generation
- Template-based layout
- Support per-evidence reports and full-case reports

**Endpoint:**
```
GET /api/evidence/:id/custody/export    → PDF for single evidence item
GET /api/cases/:id/custody/export       → PDF for entire case
```

**Tests:**
- PDF generated successfully
- PDF contains all required sections
- Hash values in PDF match database values
- TSA verification results in PDF match actual verification
- Large case → PDF generation completes within 30 seconds
- PDF is well-formed (parseable by pdf.js)

### Step 5: i18n Completion

**Deliverable:** All frontend strings use `next-intl`, English translations complete.

**Tasks:**
1. Audit every component — no hardcoded strings
2. Complete `en.json` with all translation keys organized by domain:
```json
{
    "common": { "save": "Save", "cancel": "Cancel", ... },
    "cases": { "title": "Cases", "create": "Create Case", ... },
    "evidence": { "upload": "Upload Evidence", "grid": {...}, ... },
    "custody": { "title": "Chain of Custody", ... },
    "search": { "placeholder": "Search evidence...", ... },
    "notifications": { "title": "Notifications", ... },
    "auth": { "login": "Sign In", "logout": "Sign Out", ... },
    "errors": { "not_found": "Not found", "forbidden": "Access denied", ... }
}
```
3. Create empty `fr.json` with same structure (French in Phase 2)
4. Language switcher component in header (shows available locales)
5. Date/time formatting respects locale

**Tests:**
- Every component renders with en locale
- No hardcoded strings in any component
- All keys in en.json referenced by at least one component
- No unused keys (dead translation detection)
- Locale switching works (en → fr route)
- Date formatting matches locale

### Step 6: Terraform Scaffolding

**Deliverable:** Terraform configuration for Hetzner Cloud provisioning.

```
infrastructure/
├── terraform/
│   ├── main.tf              # Hetzner provider, server, firewall, DNS, storage box
│   ├── variables.tf         # Customer name, tier, region, domain
│   ├── outputs.tf           # Server IP, storage box credentials
│   ├── versions.tf          # Provider version pins
│   └── customers/
│       └── example.tfvars   # Example customer configuration
```

**Resources created per customer:**
- `hcloud_server` — sized by tier (CPX31/CPX41/AX42)
- `hcloud_firewall` — ports 443, 22 (SSH from specific IPs only)
- `hcloud_ssh_key` — deployment SSH key
- DNS record (if using Hetzner DNS)
- Storage Box for backups (via Hetzner Robot API or manual)

**Terraform variables.tf:**
```hcl
variable "customer_name" { type = string }
variable "tier" { type = string, validation { condition = contains(["starter", "professional", "institution"], var.tier) } }
variable "region" { type = string, default = "fsn1" }  # Falkenstein
variable "domain" { type = string }  # e.g. "unhcr.vaultkeeper.eu"
variable "ssh_public_key" { type = string }
variable "backup_storage_size" { type = number, default = 1024 }  # GB
```

**Terraform outputs.tf:**
```hcl
output "server_ip" { value = hcloud_server.vaultkeeper.ipv4_address }
output "server_id" { value = hcloud_server.vaultkeeper.id }
output "storage_box_host" { value = "..." }  # Storage Box connection details
output "storage_box_user" { value = "..." }
```

**Tests:**
- `terraform validate` passes
- `terraform plan` with example.tfvars produces expected resources
- Starter tier → CPX31 server
- Professional tier → CPX41 server
- Institution tier → AX42 dedicated
- Firewall rules: only ports 443 and 22
- DNS record created correctly
- No hardcoded secrets in Terraform files
- `.terraform/` and `*.tfstate` in `.gitignore`

### Step 7: Uptime Kuma Monitoring Setup

**Deliverable:** Self-hosted monitoring for all managed instances.

**Uptime Kuma setup:**
- Separate Hetzner CX22 instance (~€4/month)
- Docker-based deployment of `louislam/uptime-kuma`
- Provisioned via Terraform (same as customer instances)
- Configured via Ansible role

**Monitoring configuration per customer instance:**
- **Public health check:** Ping `https://{instance}/health` every 60 seconds
  - Alert if: unreachable or returns "unhealthy"
  - Alert channels: Telegram (primary), email (secondary)
  - Alert delay: 2 consecutive failures before alerting (avoid flapping)
- **Detailed health check:** Separate scheduled job (Go cron or external) calls authenticated `/api/health`
  - Runs every 5 minutes with System Admin API key
  - Alerts if: any service unhealthy, backup > 36 hours old, disk > 85%
  - CRITICAL alert if: disk > 95%, backup > 72 hours, integrity verification failure

**Ansible role: `monitoring/`**
- Register new instance with Uptime Kuma via its API
- Configure alert channels
- Set up detailed health check cron job

**Tests:**
- Uptime Kuma deploys and starts correctly
- Health check monitors added via API
- Alert triggers when instance goes down
- Alert resolves when instance recovers

### Step 8: Ansible Deployment Playbooks

**Deliverable:** Ansible playbooks for server setup and application deployment.

```
infrastructure/
├── ansible/
│   ├── deploy.yml           # Full server setup + app deployment
│   ├── update.yml           # Pull new images + restart
│   ├── backup-verify.yml    # Verify backup integrity
│   ├── inventory.yml        # Customer servers
│   └── roles/
│       ├── docker/          # Install Docker + Compose
│       │   └── tasks/main.yml
│       ├── vaultkeeper/     # Deploy VaultKeeper stack
│       │   ├── tasks/main.yml
│       │   ├── templates/
│       │   │   ├── docker-compose.yml.j2
│       │   │   ├── .env.j2
│       │   │   └── Caddyfile.j2
│       │   └── vars/main.yml
│       ├── caddy/           # Caddy config + TLS
│       ├── backup/          # Backup cron setup
│       └── monitoring/      # Register with Uptime Kuma
```

**deploy.yml flow:**
1. Install Docker + Docker Compose
2. Create application directory + config
3. Template docker-compose.yml with customer-specific vars
4. Template .env with secrets
5. Pull Docker images
6. Start services
7. Wait for health check
8. Configure Caddy TLS for customer domain
9. Setup backup cron job
10. Configure Postgres automated vacuuming (autovacuum tuning for evidence-heavy workloads)
11. Register with Uptime Kuma

**Tests:**
- `ansible-lint` passes on all playbooks
- Syntax check passes
- Template rendering produces valid docker-compose.yml
- Template rendering produces valid .env

### Step 8: README & Documentation

**Deliverable:** Comprehensive README for self-hosted deployment.

**README sections:**
1. What is VaultKeeper (one paragraph)
2. Quick Start (docker compose up in 5 commands)
3. Prerequisites (Docker, domain, SMTP optional)
4. Configuration (.env variables explained)
5. Keycloak setup (create admin, configure realm)
6. First login + create first case
7. Backup & Restore procedure
8. Updating to new versions
9. Architecture overview (brief)
10. Security considerations
11. License (AGPL-3.0)
12. Contributing

### Step 9: Performance Benchmarks & Load Testing

**Deliverable:** Baseline performance targets validated before v1.0.0.

**Performance targets (per spec server tiers):**

| Operation | Target (Starter, 25 users) | Target (Professional, 100 users) |
|-----------|---------------------------|----------------------------------|
| List cases | < 200ms for 100 cases | < 200ms for 500 cases |
| List evidence (paginated) | < 200ms for 4,500 items | < 500ms for 20,000 items |
| Evidence upload (1MB) | < 2 seconds (excluding network) | < 2 seconds |
| Evidence upload (100MB) | < 30 seconds (excluding network) | < 30 seconds |
| Search query | < 300ms | < 500ms |
| Custody log query | < 200ms for 10,000 entries | < 500ms for 100,000 entries |
| Health check | < 50ms | < 50ms |
| PDF report generation | < 10 seconds for 100 entries | < 30 seconds for 1,000 entries |
| Integrity verification | < 5 min for 500 items | < 15 min for 5,000 items |

**Load testing tool:** `k6` (open source, scriptable, integrates with CI)

**Load test scenarios:**
1. **Concurrent users:** 25 users browsing cases + searching simultaneously
2. **Upload under load:** 5 concurrent uploads while 20 users browse
3. **Custody chain stress:** 10,000 custody log entries, verify chain in < 5 seconds
4. **Search at scale:** 50,000 indexed items, search response < 500ms
5. **Pagination at scale:** cursor-based pagination through 10,000 items

**CI integration:** Load tests run on release builds (not every PR — too slow).

**Tests:**
- Each performance target met on CI runner hardware
- No memory leaks during sustained load (Go pprof)
- Connection pool doesn't exhaust under concurrent load
- Response times degrade linearly, not exponentially, with load

### Step 10: v1.0.0 Release Preparation

**Deliverable:** Tagged release with Docker image.

**Checklist:**
- [ ] All Phase 1 features working end-to-end
- [ ] All tests passing (unit + integration + E2E)
- [ ] Coverage >= 80% on all Go packages
- [ ] Docker image builds and runs
- [ ] `docker compose up` → working system in <5 minutes
- [ ] README complete
- [ ] CHANGELOG.md with v1.0.0 entry
- [ ] `.env.example` complete
- [ ] No TODO comments in production code
- [ ] No hardcoded secrets
- [ ] Security headers verified (CSP, HSTS, X-Frame-Options)
- [ ] Rate limiting configured in Caddyfile
- [ ] Backup + restore tested
- [ ] AGPL-3.0 LICENSE present
- [ ] Git tag v1.0.0

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/backup/runner.go` | Create | Scheduled backup orchestration |
| `internal/backup/encrypt.go` | Create | AES-256-GCM backup encryption |
| `internal/cases/export.go` | Create | ZIP case export |
| `internal/reports/pdf.go` | Create | Custody report PDF generation |
| `web/src/messages/en.json` | Complete | All i18n translations |
| `web/src/messages/fr.json` | Create | Empty French template |
| `infrastructure/terraform/*` | Create | Hetzner provisioning (main.tf, variables.tf, outputs.tf) |
| `infrastructure/ansible/*` | Create | Deployment playbooks + roles |
| `infrastructure/monitoring/` | Create | Uptime Kuma Terraform + Ansible setup |
| `k6/load-test.js` | Create | k6 load test scenarios |
| `README.md` | Create | Full documentation |
| `CHANGELOG.md` | Create | v1.0.0 entry (auto-generated from conventional commits) |

---

## Definition of Done

- [ ] Automated backup runs daily, encrypted, uploaded to destination
- [ ] Backup restore tested and documented
- [ ] 3 consecutive backup failures trigger CRITICAL alert
- [ ] Case export produces valid ZIP with all evidence + metadata
- [ ] Custody report PDF generated with complete chain of custody
- [ ] All UI strings in i18n files (zero hardcoded strings)
- [ ] Terraform validates and plans correctly
- [ ] Ansible playbooks lint-clean
- [ ] README enables a sysadmin to deploy in 30 minutes
- [ ] v1.0.0 tagged and Docker image pushed
- [ ] All Phase 1 features passing E2E tests
- [ ] >= 80% test coverage across all Go packages
- [ ] Uptime Kuma monitoring deployed and alerting
- [ ] Disaster recovery test passes (backup → destroy → restore → verify)
- [ ] RTO < 1 hour measured and documented
- [ ] Performance benchmarks met for Starter tier
- [ ] Load test with 25 concurrent users passes
- [ ] Conventional commits enforced via CI

---

## Security Checklist

- [ ] Backup encryption key from env var, never in code
- [ ] Backup files encrypted with AES-256-GCM
- [ ] Backup destination credentials not logged
- [ ] Case export respects role-based access (defence → disclosed only)
- [ ] PDF report doesn't include more data than user has access to
- [ ] Ansible vault for secrets in playbooks
- [ ] Terraform state file excluded from git (.gitignore)
- [ ] SSH keys managed securely
- [ ] No default passwords in production configs
- [ ] All services TLS-encrypted

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 6 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/backup/runner.go`**: Full backup completes successfully, backup archive is encrypted (cannot decompress without key), Postgres dump includes all tables and data, MinIO snapshot includes all evidence files, backup_log entry created with correct status/timestamps/file_count/total_size, scheduled run triggers at configured time, failed backup sets status="failed" and logs error, admin notification sent on failure, 3 consecutive failures trigger CRITICAL notification, old backups cleaned (>30 days retention), backup destination unreachable logged and retried, on-demand backup via API works, backup verification checksum matches
- **`internal/backup/encrypt.go`**: AES-256-GCM encryption roundtrip produces original, wrong key fails decryption, tampered ciphertext fails GCM authentication, key derivation (PBKDF2/Argon2) produces consistent output, encryption metadata header written correctly (algorithm, IV, key derivation params — not the key)
- **`internal/cases/export.go`**: Export produces valid ZIP, all evidence files present in ZIP, metadata.csv matches Postgres data, custody_log.csv complete with all entries, hashes.csv values match actual file hashes, manifest.json contains correct metadata (date, version, case info, total items), large case (1000 items) streams without OOM, defence user export contains only disclosed items, export logged in custody chain, README.txt present and explains format
- **`internal/reports/pdf.go`**: PDF generated successfully, PDF contains header with case reference and date, PDF contains evidence item details (number, title, hash, TSA timestamp), PDF contains complete custody chain (action, who, when, IP, hash), PDF contains hash chain verification status, PDF contains RFC 3161 verification results, large case PDF completes within 30 seconds, PDF is well-formed (parseable by pdf.js), per-evidence report contains single item, full-case report contains all items
- **i18n**: Every component renders with en locale, no hardcoded strings in any component, all keys in en.json referenced by at least one component, no unused keys, locale switching works (en to fr route), date formatting matches locale

### Integration Tests (with testcontainers)

- **Full backup + restore (testcontainers: postgres + minio)**: Create test data (case + evidence + custody entries), run full backup, verify encrypted archive produced, destroy all data (drop database, clear MinIO), restore from backup, verify all evidence items present with correct hashes, verify custody chain intact (hash chain valid), verify all cases/roles/notifications restored, measure restore time (target: < 1 hour RTO)
- **Postgres pg_dump/restore (testcontainers/postgres:16-alpine)**: Full pg_dump of database with all 9 tables populated, restore to clean Postgres instance, verify row counts match, verify RLS policies intact after restore, verify sequences reset correctly
- **MinIO mirror (testcontainers/minio)**: Mirror evidence bucket to backup location, verify all objects present with matching ETags, verify SSE encryption maintained
- **Case export ZIP (testcontainers: postgres + minio)**: Export case with 50 evidence items, verify ZIP structure matches spec, verify each file hash in hashes.csv matches file content in ZIP, verify custody_log.csv row count matches database, verify metadata.csv columns match evidence_items schema
- **PDF generation (testcontainers/postgres)**: Generate custody report for case with 100 custody entries, verify PDF page count reasonable, verify all entries present in PDF, verify hash values match database
- **Terraform validation**: `terraform validate` passes on all .tf files, `terraform plan` with example.tfvars produces expected resource count, firewall rules only allow ports 443 and 22
- **Ansible linting**: `ansible-lint` passes on all playbooks, template rendering produces valid docker-compose.yml and .env

### E2E Automated Tests (Playwright)

- **`tests/e2e/backup/trigger-backup.spec.ts`**: Login as system_admin, navigate to admin > backups, click "Run Backup Now", verify progress indicator appears, wait for completion, verify backup appears in backup history list with status "completed", file count, and total size
- **`tests/e2e/backup/backup-history.spec.ts`**: Login as system_admin, navigate to admin > backups, verify list of past backups with timestamps, statuses, and sizes, click verify on a backup, confirm integrity check passes
- **`tests/e2e/backup/backup-restore.spec.ts`**: (Staging environment only) Trigger backup, verify completed, restore from the backup, verify application comes back healthy, verify evidence count matches pre-backup count, verify a specific evidence item is downloadable with correct hash
- **`tests/e2e/export/case-export.spec.ts`**: Login as investigator, navigate to case detail, click "Export Case", verify ZIP downloads, extract ZIP, verify manifest.json present with correct case info, verify evidence files present, verify metadata.csv and custody_log.csv present, verify hashes.csv matches file hashes
- **`tests/e2e/export/custody-pdf.spec.ts`**: Navigate to case custody tab, click "Export PDF", verify PDF downloads, open PDF and verify it contains case reference header, evidence item table with hashes, and custody chain entries with timestamps
- **`tests/e2e/export/defence-export.spec.ts`**: Login as defence user, export case, verify ZIP contains only disclosed evidence items, verify no undisclosed files or metadata in the export
- **`tests/e2e/infra/terraform-plan.spec.ts`**: (CI only) Run `terraform validate` and `terraform plan -var-file=customers/example.tfvars`, verify exit code 0, verify expected resources in plan output
- **`tests/e2e/infra/ansible-lint.spec.ts`**: (CI only) Run `ansible-lint` on all playbooks, verify exit code 0 with zero violations

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Login as system_admin, navigate to admin > backups, click "Run Backup Now"
   **Expected:** Backup job starts, progress shown, completes within a reasonable time (< 10 minutes for a small dataset)
   **Verify:** Backup appears in history with status "completed", file count matches evidence count, total size is reasonable, backup_log table entry has correct timestamps

2. [ ] **Action:** Locate the backup file on the configured BACKUP_DESTINATION (SFTP, S3, or local)
   **Expected:** Encrypted archive file present with recent timestamp in filename
   **Verify:** Attempt to decompress without the encryption key — fails (encrypted); with the key, archive extracts to a valid Postgres dump + MinIO snapshot

3. [ ] **Action:** Perform a full disaster recovery test: backup, then destroy all data, then restore
   **Expected:** After restore, application starts healthy, all cases/evidence/custody logs present
   **Verify:** Pick a specific evidence item, download it, verify SHA-256 hash matches the original; verify custody chain is intact (run integrity verification); measure total restore time (target: < 1 hour)

4. [ ] **Action:** Allow 3 backup attempts to fail (e.g., by misconfiguring BACKUP_DESTINATION temporarily)
   **Expected:** First failure sends admin notification, 3rd consecutive failure sends CRITICAL alert
   **Verify:** backup_log shows 3 entries with status="failed", notification bell shows CRITICAL backup notification

5. [ ] **Action:** Navigate to a case detail page, click "Export Case" as an investigator
   **Expected:** ZIP file downloads with the case reference code in the filename
   **Verify:** Extract the ZIP, verify: manifest.json has correct case metadata, evidence/ folder contains all evidence files, metadata.csv has one row per evidence item, custody_log.csv has all custody entries, hashes.csv SHA-256 values match file hashes computed locally, README.txt explains the format

6. [ ] **Action:** Login as defence role user, export the same case
   **Expected:** ZIP downloads but contains only disclosed evidence items
   **Verify:** Compare file count with the investigator's export — defence export has fewer files; no undisclosed filenames or metadata appear anywhere in the ZIP

7. [ ] **Action:** Navigate to evidence detail > custody tab, click "Export PDF" for a single evidence item
   **Expected:** PDF downloads with the evidence number in the filename
   **Verify:** Open PDF, verify it contains: header with VaultKeeper logo and case reference, evidence details (number, title, hash, TSA timestamp, upload date), complete custody chain entries (every action, who, when, IP, hash at action), hash chain verification status, page numbers and generation timestamp

8. [ ] **Action:** Export a full case custody report PDF (case-level, not per-evidence)
   **Expected:** PDF downloads with the case reference code in the filename
   **Verify:** PDF contains all evidence items and all custody entries across the entire case; verify it completes within 30 seconds for a case with ~100 entries

9. [ ] **Action:** Run `terraform validate` and `terraform plan -var-file=customers/example.tfvars`
   **Expected:** Both commands succeed with zero errors
   **Verify:** Plan output shows expected resources (hcloud_server, hcloud_firewall, hcloud_ssh_key); firewall rules only open ports 443 and 22; no hardcoded secrets in .tf files; .terraform/ and *.tfstate in .gitignore

10. [ ] **Action:** Run `ansible-lint` on all playbooks in infrastructure/ansible/
    **Expected:** Zero violations, exit code 0
    **Verify:** Template rendering: manually inspect generated docker-compose.yml.j2 and .env.j2 output for a test customer — verify valid YAML and correct variable substitution

11. [ ] **Action:** Verify all UI strings are in i18n files by switching locale from English to French
    **Expected:** All text changes to French keys (or shows translation keys if fr.json is empty placeholders)
    **Verify:** No English strings visible after switching to French locale (untranslated keys are acceptable as "[fr] key.name" format, not raw English); date formatting follows locale conventions

12. [ ] **Action:** Review the README.md for completeness
    **Expected:** README contains: quick start (5 commands), prerequisites, configuration guide, Keycloak setup, first login, backup/restore procedure, update procedure, architecture overview, security considerations, license
    **Verify:** Follow the quick start instructions on a clean machine — system deploys and serves the health endpoint within 5 minutes
