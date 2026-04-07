# Sprint 12: French i18n, CI/CD Rolling Updates, Chain Continuity Certificate & Case Archival

**Phase:** 2 — Institutional Features
**Duration:** Weeks 23-24
**Goal:** Complete Phase 2 with French translation, automated rolling updates to managed instances, court-submittable chain continuity certificates, case archival mode, and template SOPs. Ship v2.0.0.

---

## Prerequisites

- Sprints 7-11 complete (all Phase 2 features)

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)
- [x] Infrastructure (CI/CD)

---

## Implementation Steps

### Step 1: French Language Support

**Deliverable:** Complete French translation of all UI strings.

**Tasks:**
1. Translate all keys in `en.json` → `fr.json` (professional legal French, not machine translation)
2. Legal terminology review: ensure terms match ICC/tribunal French usage
3. Language switcher in header: EN | FR
4. Date/time formatting: `dd/MM/yyyy HH:mm` for French locale
5. Number formatting: French uses comma as decimal separator, period as thousands
6. PDF reports: generate in selected locale (custody reports in French)

**Key terminology:**
- Evidence → Pièce à conviction / Preuve
- Custody chain → Chaîne de traçabilité
- Case → Affaire / Dossier
- Witness → Témoin
- Disclosure → Communication / Divulgation
- Legal hold → Gel judiciaire
- Redaction → Caviardage

**Tests:**
- Every key in en.json has corresponding key in fr.json
- No missing translations (automated check)
- French locale renders all pages correctly
- Date formatting matches French conventions
- PDF reports generate in French
- Language switcher persists preference

### Step 2: Automated Rolling Updates (CI/CD)

**Deliverable:** GitHub Actions workflow that deploys new versions to all managed instances.

**release.yml enhancement:**
```yaml
jobs:
  deploy-managed:
    needs: [test, build-push]
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to managed instances
        run: |
          for instance in $MANAGED_INSTANCES; do
            ssh deploy@$instance "cd /opt/vaultkeeper && \
              docker compose pull && \
              docker compose up -d && \
              sleep 10 && \
              curl -sf http://localhost/health || docker compose rollback"
          done
```

**Deployment script (`infrastructure/scripts/rollout.sh`):**
1. Pull new Docker image on instance
2. `docker compose up -d` (zero-downtime restart)
3. Wait 10 seconds for health check
4. Verify `/health` returns 200 + correct version
5. If health check fails → rollback to previous image
6. If rollback also fails → alert (manual intervention needed)
7. Log deployment result

**Health check script (`infrastructure/scripts/health-check.sh`):**
```bash
#!/bin/bash
# Check all managed instances
for instance in $(cat instances.txt); do
    status=$(curl -sf "https://$instance/health" | jq -r '.status')
    version=$(curl -sf "https://$instance/health" | jq -r '.version')
    echo "$instance: $status ($version)"
done
```

**Managed instance inventory:**
- Stored in `infrastructure/ansible/inventory.yml`
- Also referenced by CI/CD for deployment targets
- SSH keys managed via GitHub Secrets

**Tests:**
- Deployment script handles successful upgrade
- Deployment script handles failed health check → rollback
- Rollback restores previous version
- Health check script reports correct status
- Multiple instances deployed in sequence (not parallel — safer)

### Step 3: Chain Continuity Certificate

**Deliverable:** Court-submittable PDF proving unbroken custody chain.

**Certificate contents:**
- Title: "Chain of Custody Continuity Certificate"
- Case reference + evidence number
- Date range covered
- Evidence hash at ingestion (with RFC 3161 timestamp)
- Complete chronological custody chain:
  - Each event: timestamp, actor, action, hash at time of action
  - RFC 3161 timestamp verification for ingestion event
- Custody chain hash verification: "Chain integrity verified — all [N] entries form an unbroken hash-linked sequence"
- Evidence current hash + RFC 3161 verification
- Attestation statement: "This certificate attests that evidence item [number] has maintained an unbroken, cryptographically verified chain of custody from [ingestion date] to [certificate date]."
- Digital signature (institution PKI or self-signed)
- QR code linking to verification URL

**Endpoint:**
```
GET /api/evidence/:id/chain-certificate → Generate Chain Continuity Certificate PDF
```

**Certificate format specification (`docs/spec/chain-continuity-certificate.md`):**
```
CHAIN OF CUSTODY CONTINUITY CERTIFICATE
════════════════════════════════════════

Certificate ID:         [UUID]
Generated:              [timestamp]
VaultKeeper Version:    [version]

EVIDENCE IDENTIFICATION
───────────────────────
Evidence Number:        ICC-UKR-2024-00001
Case Reference:         ICC-UKR-2024
Title:                  [evidence title]
File Type:              application/pdf
File Size:              4,521,392 bytes

CRYPTOGRAPHIC VERIFICATION
──────────────────────────
SHA-256 Hash (ingestion):  [hex, 64 chars]
SHA-256 Hash (current):    [hex, 64 chars]
Hash Match:                ✓ Identical

RFC 3161 Timestamp:
  - Authority:             FreeTSA.org
  - Signed Time:           2024-03-15T10:30:00Z
  - Token Status:          ✓ Valid

CUSTODY CHAIN (N entries)
─────────────────────────
| # | Timestamp           | Actor          | Action      | Hash at Action  |
|---|---------------------|----------------|-------------|-----------------|
| 1 | 2024-03-15 10:30:00 | J. Smith (inv) | uploaded    | a1b2c3...       |
| 2 | 2024-03-15 10:35:00 | J. Smith (inv) | classified  | a1b2c3...       |
| 3 | 2024-03-16 09:00:00 | M. Jones (pro) | viewed      | a1b2c3...       |
...

CHAIN INTEGRITY
───────────────
Total Entries:           [N]
Hash Chain Status:       ✓ All [N] entries form an unbroken hash-linked sequence
First Entry Hash:        [hex]
Last Entry Hash:         [hex]

ATTESTATION
───────────
This certificate attests that evidence item [ICC-UKR-2024-00001] has 
maintained an unbroken, cryptographically verified chain of custody 
from [ingestion date] to [certificate date]. All [N] custody events 
are hash-chained and the file hash has remained unchanged throughout.

Signature Algorithm:     Ed25519
Signed By:               [instance identifier]
Signature:               [base64]
Verify At:               https://instance.vaultkeeper.eu/verify/[certificate-id]

[QR CODE linking to verification URL]
```

**Verification URL (`/verify/:certificateId`):**
- Public endpoint (no auth) — allows anyone with the certificate to verify
- Returns: certificate validity, current chain status, hash comparison
- This lets a court verify the certificate without VaultKeeper credentials

**Tests:**
- Certificate contains all custody events
- Hash chain verified and stated in certificate
- RFC 3161 timestamps verified and included
- Certificate date matches generation time
- Multi-page certificates for long custody chains
- Certificate in French when locale=fr
- QR code contains valid verification URL
- Verification URL returns correct status
- Signature verifiable with published public key
- Certificate for destroyed evidence → still generated (metadata preserved)

### Step 4: Case Archival Mode

**Deliverable:** Move closed cases to cold storage with active custody chain.

**Archival flow:**
1. Case Admin archives case (`POST /api/cases/:id/archive`)
2. Case status → "archived"
3. Evidence files flagged for cold storage migration (background job)
4. Cold storage migration:
   a. If `ARCHIVE_STORAGE_BUCKET` configured → move MinIO objects to archive bucket
   b. If `ARCHIVE_STORAGE_PATH` configured → rsync to Hetzner Storage Box via SFTP
   c. If neither configured → files stay in primary MinIO bucket (logged warning)
5. After migration: update `evidence_items.file_key` to point to archive location
6. Custody chain entry: `action: "archived"`, details include storage destination
7. Custody chain and integrity verification remain active (hashes don't change)
8. Evidence metadata still queryable from Postgres (no change)
9. Evidence files retrievable within 24 hours (async restore from cold storage)

**Archived case behavior:**
- Read-only (no new evidence uploads)
- Custody log still accessible
- Integrity verification still runs (on schedule)
- Chain Continuity Certificates still generatable
- Evidence download triggers "retrieval from archive" (async, notification when ready)
- Legal hold still enforceable (prevents destruction even when archived)

**Archival pricing note (for managed instances):** Archived cases on cheaper storage tier → archival pricing of €100/month per case group.

**Tests:**
- Archive case → status changes, files queued for cold storage
- Upload to archived case → rejected
- Custody log still accessible
- Integrity verification still works
- Evidence download from archive → async retrieval
- Legal hold on archived case → respected
- Unarchive → files move back to hot storage

### Step 5: Template SOPs & Workflow Guides

**Deliverable:** Customizable Standard Operating Procedure documents.

**SOP documents (Markdown, exportable as PDF):**
1. **Evidence Collection SOP** — Steps for field investigators to collect, document, and upload evidence
2. **Chain of Custody SOP** — How custody is maintained within VaultKeeper, what triggers logging
3. **Disclosure SOP** — Steps for prosecutors to review, redact, and disclose evidence
4. **Data Migration SOP** — Steps for migrating from another system with hash bridging
5. **Incident Response SOP** — What to do when integrity verification fails
6. **Backup & Restore SOP** — Backup verification and restore procedures

**Location:** `docs/sops/` in the repository, also accessible via admin UI at `/admin/sops`.

**SOP format:**
- Written in Markdown with YAML frontmatter (title, version, last_updated, applicable_roles)
- Exportable as PDF via the same PDF generation engine as custody reports
- Customizable: institutions can fork the SOPs and modify via admin UI
- Versioned: each SOP has a version number, changes tracked

**Admin UI for SOPs:**
- View SOP list
- View individual SOP (rendered Markdown)
- Export SOP as PDF
- Future (Phase 3+): customize SOP content per institution

**Key feature:** SOPs reference VaultKeeper by name and include specific VaultKeeper screenshots/UI references. This creates institutional dependency — removing VaultKeeper means rewriting every procedure.

### Step 6: v2.0.0 Release

**Deliverable:** Tagged release with all Phase 2 features.

**Checklist:**
- [ ] All Phase 2 features working end-to-end
- [ ] French translation complete and reviewed
- [ ] Rolling updates tested on staging
- [ ] Chain Continuity Certificate generates correctly in EN + FR
- [ ] Case archival moves files to cold storage
- [ ] Migration tool tested with realistic data sets
- [ ] All tests passing (unit + integration + E2E)
- [ ] Coverage >= 80% on all Go packages
- [ ] CHANGELOG.md v2.0.0 entry
- [ ] README updated with new features
- [ ] SOP documents reviewed
- [ ] Docker image tagged v2.0.0

**Office document preview (Phase 2 spec item):**
- .docx/.xlsx → LibreOffice headless → PDF → pdf.js in browser
- Dockerfile adds LibreOffice headless layer
- Preview generation runs in background on upload
- Original file unchanged

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `web/src/messages/fr.json` | Complete | Full French translation |
| `internal/reports/chain_certificate.go` | Create | Chain Continuity Certificate PDF |
| `internal/cases/archival.go` | Create | Case archival + cold storage migration |
| `infrastructure/scripts/rollout.sh` | Create | Rolling update script |
| `infrastructure/scripts/health-check.sh` | Create | Health check across instances |
| `.github/workflows/release.yml` | Modify | Add managed instance deployment |
| `docs/sops/*.md` | Create | SOP templates |
| `internal/evidence/preview.go` | Create | LibreOffice document preview |

---

## Definition of Done

- [ ] French translation complete (0 missing keys)
- [ ] Rolling updates deploy + rollback tested
- [ ] Chain Continuity Certificate court-ready in EN + FR
- [ ] Case archival moves files to cold storage
- [ ] Archived cases read-only with active custody chain
- [ ] Office document preview via LibreOffice headless
- [ ] SOP documents complete and referenceable
- [ ] v2.0.0 tagged and deployed to staging
- [ ] All E2E tests passing

---

## Security Checklist

- [ ] Deployment SSH keys stored in GitHub Secrets (not in code)
- [ ] Rolling update rollback is automatic on health check failure
- [ ] Certificate digital signature verifiable
- [ ] Archived evidence files still encrypted at rest
- [ ] Cold storage access requires same authentication
- [ ] SOP documents don't contain actual credentials or real case data

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 12 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `i18n.LoadMessages("fr")` — all keys from en.json have corresponding entries in fr.json, zero missing keys
- `i18n.FormatDate("fr")` — outputs dd/MM/yyyy HH:mm format for French locale
- `i18n.FormatNumber("fr")` — uses comma as decimal separator, period as thousands separator
- `i18n.SwitchLocale` — persists user preference, subsequent requests use new locale
- `i18n.TranslateKey` — returns French string for French locale, English string for English locale, falls back to English for missing key
- `reports.GenerateCustodyReportPDF("fr")` — all labels, headings, and attestation text in French legal terminology
- `reports.GenerateChainCertificate("fr")` — certificate title "Certificat de continuite de la chaine de tracabilite," attestation statement in French, all field labels in French
- `reports.GenerateChainCertificate("en")` — English certificate generated correctly alongside French
- `certificate.Sign` — Ed25519 signature verifiable with public key
- `certificate.GenerateQRCode` — QR code contains valid verification URL
- `certificate.VerifyEndpoint` — public endpoint returns certificate validity, chain status, hash comparison without authentication
- `archival.ArchiveCase` — status changes to "archived," files queued for cold storage, custody log entry created
- `archival.ArchiveCase` — rejects archive if legal hold is active
- `archival.ArchiveCase` — archived case blocks new uploads (returns 409)
- `archival.UnarchiveCase` — moves files back to hot storage, status returns to active
- `archival.ColdStorageMigration` — moves objects to archive bucket when ARCHIVE_STORAGE_BUCKET configured, uses SFTP when ARCHIVE_STORAGE_PATH configured, logs warning when neither configured
- `rollout.Deploy` — pulls new image, restarts, health check passes — success
- `rollout.Deploy` — health check fails — automatic rollback to previous version
- `rollout.HealthCheck` — parses /health response, validates status + version fields

### Integration Tests (testcontainers)

- French locale end-to-end: set user locale to French, create evidence, view custody chain — all UI-facing strings in French, dates in dd/MM/yyyy format, numbers with French separators
- French PDF generation: generate custody report PDF with French locale, parse PDF text — labels in French ("Chaine de tracabilite," "Preuve," "Temoin")
- Chain continuity certificate bilingual: generate certificate in English, then in French for the same evidence — both contain identical custody events but all text in respective languages
- Chain continuity certificate verification: generate certificate, extract QR code URL, visit verification endpoint — returns valid status with matching chain data, no authentication required
- Certificate for destroyed evidence: destroy evidence, generate chain continuity certificate — certificate generated successfully, shows destruction event in chain, attestation notes destruction
- Case archival pipeline: create case with 5 evidence items, archive case, verify files moved to archive bucket in MinIO, verify evidence_items.file_key updated to archive location, verify custody log entry for each item, attempt upload — rejected with 409
- Archived case integrity: archive case, run integrity verification — verification still completes successfully against archived files
- Archived case unarchive: archive case, unarchive, verify files move back to hot storage, uploads now allowed
- Rolling update simulation: deploy new version (test container), health check passes — deployment marked successful; deploy broken version, health check fails — rollback triggered, previous version restored

### E2E Automated Tests (Playwright)

- Language switcher: click EN|FR toggle in header, verify all visible text switches to French, refresh page — French persists, switch back to English — all text in English
- French evidence management: with French locale active, create a case, upload evidence, view custody chain — every label, button, and message displayed in French legal terminology
- French date and number formatting: with French locale, view evidence detail — dates show as dd/MM/yyyy, file sizes use comma decimal separator (e.g., "4,5 Mo")
- Chain continuity certificate in English: navigate to evidence detail, click "Generate Chain Certificate," verify PDF downloads, open PDF — verify certificate contains all custody events, hash verification, RFC 3161 timestamp info, attestation statement in English, QR code present
- Chain continuity certificate in French: switch to French locale, generate same certificate — verify all text in French, field labels in French, attestation statement in French
- Certificate verification via QR: scan or extract QR code URL from certificate PDF, navigate to that URL — public page shows certificate validity status without requiring login
- Case archival: navigate to case settings, click "Archive Case," confirm, verify case status changes to "Archived," verify read-only badge appears, attempt to upload evidence — verify upload button disabled or returns error, verify custody log and evidence grid still accessible
- Rolling update deployment: trigger deployment via CI (or simulate), verify /health endpoint returns new version after deployment, verify all services operational

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Log in to VaultKeeper. Click the language switcher (EN | FR) in the header to switch to French.
   **Expected:** All UI text switches to French. Navigation menu, buttons, labels, error messages, and tooltips are all in French. Legal terminology matches ICC/tribunal usage (e.g., "Chaine de tracabilite" not "Chaine de garde").
   **Verify:** Navigate through every major page (dashboard, case list, evidence grid, evidence detail, custody chain, disclosures, audit log). Confirm no English text remains except proper nouns and technical identifiers (UUIDs, hashes).

2. [ ] **Action:** With French locale active, check date formatting on the evidence detail page and custody chain.
   **Expected:** All dates display in dd/MM/yyyy HH:mm format (e.g., "06/04/2026 14:30"). File sizes use comma as decimal separator (e.g., "4,5 Mo").
   **Verify:** Compare with the same page in English locale — dates show as MM/dd/yyyy or ISO format, file sizes use period decimal.

3. [ ] **Action:** Navigate to an evidence item with a full custody chain. Click "Generate Chain of Custody Certificate" (or "Generer le certificat de chaine de tracabilite" in French).
   **Expected:** A PDF downloads. Open it.
   **Verify (English):** Certificate contains: title "Chain of Custody Continuity Certificate," evidence number, case reference, SHA-256 hash at ingestion and current, RFC 3161 timestamp verification, complete custody chain table (every event with timestamp/actor/action/hash), chain integrity statement, attestation statement, digital signature, QR code.
   **Verify (French):** Same certificate in French contains: "Certificat de continuite de la chaine de tracabilite," all field labels in French, attestation in French, same data values.

4. [ ] **Action:** Extract or scan the QR code from the generated certificate. Navigate to the URL in a browser (incognito/private window, not logged in).
   **Expected:** A public verification page loads without requiring authentication. It displays: certificate validity (valid/invalid), evidence hash comparison (matches/mismatches), chain integrity status.
   **Verify:** The displayed information matches the certificate PDF. The page clearly states the verification result.

5. [ ] **Action:** Navigate to a completed case. Go to case settings and click "Archive Case." Confirm in the dialog.
   **Expected:** Case status changes to "Archived." A read-only indicator appears on the case header. Custody log shows "archived" event.
   **Verify:** Evidence grid is still viewable. Custody chain is still accessible. Attempt to upload new evidence — upload button is disabled or clicking it shows "Cannot upload to archived case."

6. [ ] **Action:** As an archived case, attempt to download an evidence item.
   **Expected:** If cold storage is configured, a message appears: "Evidence is in cold storage. Retrieval has been requested and will be available within 24 hours. You will be notified when ready."
   **Verify:** After retrieval completes (or if cold storage is not configured), the file downloads correctly with the same hash as the original.

7. [ ] **Action:** Run integrity verification on an archived case.
   **Expected:** Verification runs against archived files. Results show all items verified with correct hashes. Chain integrity confirmed.
   **Verify:** Integrity check record created in integrity_checks table with correct item counts.

8. [ ] **Action:** Trigger a rolling update deployment (merge to main or tag a release). Monitor the deployment.
   **Expected:** CI pipeline builds the new Docker image, pushes to registry, deploys to managed instances sequentially. Each instance: pull image, restart, health check passes, confirmed.
   **Verify:** Visit /health on each managed instance — returns 200 with the new version number. All services operational (evidence upload, search, custody chain all functional).

9. [ ] **Action:** Simulate a failed deployment by pushing a broken Docker image (or temporarily breaking the /health endpoint). Trigger rolling update.
   **Expected:** Deployment detects health check failure. Automatic rollback to previous image. Alert generated for manual intervention.
   **Verify:** Visit /health — returns the previous (working) version. No data loss or corruption. Alert/notification received.

10. [ ] **Action:** Generate a Chain Continuity Certificate for evidence that has been destroyed (metadata preserved, file deleted).
    **Expected:** Certificate is generated successfully. It includes the full custody chain up to and including the destruction event. Attestation notes the evidence was destroyed on [date] under authority [reference].
    **Verify:** Certificate contains destruction entry in chain table. Hash at ingestion is preserved. Statement notes the item no longer exists but its chain of custody is intact.
