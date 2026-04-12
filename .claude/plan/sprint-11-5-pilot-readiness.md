# Sprint 11.5: Pilot Readiness Hardening

**Phase:** 2 — Institutional Features (interstitial sprint, inserted between Sprint 11 and Sprint 12)
**Duration:** **4 weeks** with 3 engineers, or 3 weeks with 4 engineers (revised upward from 3 weeks based on audit findings — the `SystemRole` capability refactor across 22 call sites, the CLI verifier + `pkg/custodyhash` extraction with nested-module discipline, the `PlaceHold` resumable state machine, and the append-only extension to 11 tables are each multi-day items that compounded). Downstream sprints shift accordingly.
**Goal:** Close every trust-breaking gap that would end a pilot on day one. Make VaultKeeper provably safe to put in front of the ICC, Bellingcat, Amnesty, and other institutional users whose failure mode is "we tried it once, something felt off, we left." Ship the features that make legal teams, digital forensics investigators, and sysadmins all exhale.

This sprint is non-negotiable before any pilot onboarding. It is the pre-flight checklist for v2.0.0.

---

## Why this sprint exists

The 21-sprint plan builds a technically excellent evidence management system. A deep audit of the current codebase (as of this sprint's authoring) against the pilot-readiness rubric found nine gaps that are trust-breaking if missing:

1. **Upload integrity is server-side only.** `internal/evidence/handler.go:189` takes multipart form data, reads the file, and only then computes SHA-256 inside `service.Upload`. A file corrupted in transit would be hashed *after* corruption and stored with a hash that matches its corrupted bytes. No receipt, no detection, no loud failure. Client-side pre-hashing with server re-verification is the single most important missing feature.
2. **Append-only enforcement covers `custody_log` but nothing else.** Migration 003 already enforces append-only on `custody_log` via RLS (no UPDATE/DELETE policies, `FORCE ROW LEVEL SECURITY`, `vaultkeeper_app` role granted only `SELECT, INSERT`). This is good. But the new tables we'll add in this sprint — `upload_attempts_v1`, `evidence_status_transitions`, `integrity_checks`, `trigger_tamper_log` — need the same treatment, and so does the `notifications` table. Without this, a compromised admin or a buggy migration can erase parts of the audit record that RLS doesn't currently cover.
3. **Legal hold is enforced in Go only.** `cases.Service.EnsureNotOnHold` guards destructive paths in the service layer (`internal/cases/legal_hold.go` + `internal/app/legal_hold_adapter.go`). A superadmin with psql can run `DELETE FROM evidence_items WHERE ...` and bypass it entirely. MinIO storage has no retention lock — `mc rm` works on held evidence. Legal hold must be enforced at both the DB row level and the object storage level, in modes that cannot be reversed by administrative action.
4. **No enforced evidence lifecycle.** `evidence_items` has `tsa_status` (pending/stamped/failed/disabled) but no review/admissibility status. Sprint 19's configurable workflow engine is 14 weeks out. Institutional users expect an enforced "Ingested → Under Review → Verified → Admitted/Rejected" pipeline with role-gated transitions on day one.
5. **No standalone verification CLI.** Sprint 11's `internal/integrity/handler.go` is a server-side verifier that trusts the VaultKeeper database. There is no binary a defense counsel or auditor can run against an exported bundle on their own machine, without contacting our server. This is our single most powerful differentiator and it does not exist.
6. **No Berkeley Protocol compliance report.** No `internal/compliance/` package, no mapping file, no report generator. Regulators and funders will ask for this at the first pilot meeting.
7. **No air-gap mode.** Sprint 13 mentions "zero outbound" for Whisper. There is no `AIR_GAP` configuration flag, no `internal/airgap/` package, no egress allowlist, no startup canary, no documented air-gap deployment path. Institutional users in some jurisdictions have air-gap as a hard requirement.
8. **No preset role templates.** `internal/auth/context.go` ships three system roles (`RoleUser`, `RoleCaseAdmin`, `RoleSystemAdmin`) and case roles are configured per-user. A non-technical pilot admin cannot onboard their team without understanding the case-role matrix. An "Auditor" read-only role does not exist as a system role at all.
9. **No operator handbook, no first-run wizard, no app-level bootstrap.** `deploy/server-bootstrap/bootstrap.sh` is a Debian host-hardening script (SSH port, fail2ban, firewall). There is no script that walks a fresh operator from a clean host to a working, verified, first-evidence-uploaded VaultKeeper in under an hour. Sprint 6 shipped `docker compose up` but nobody has written down the full quickstart and nobody has measured it under SLO.

**Consequence of deferring:** every pilot before Sprint 11.5 ships becomes a support burden and a reputational risk. Every pilot after it ships is a lighthouse customer. The sprint is the delta between "technically correct" and "pilot-ready."

---

## Prerequisites and current-state notes

- Migrations at **019** (`019_migrations_tracking.up.sql`). New migrations in this sprint are **020–025**.
- `custody_log` append-only enforcement is already in place via migration **003** (RLS policies + role grants). **Do not duplicate this work.** Sprint 11.5 extends the same pattern to new tables.
- `vaultkeeper_app` and `vaultkeeper_readonly` roles exist.
- `SystemRole` enum is **four values** in `internal/auth/context.go`: `RoleAPIService=0`, `RoleUser=1`, `RoleCaseAdmin=2`, `RoleSystemAdmin=3`. The existing `RoleAPIService` value (used by background services and API-key authentication) **must be preserved**. Sprint 11.5 adds `RoleAuditor` as **value 4** at the end of the enum — it does NOT reorder existing values. The comparison operator `ac.SystemRole < minimum` currently used in `permissions.go:26` and in six other call sites is an *ordinal* check that must be replaced with explicit capability checks **before** `RoleAuditor` is introduced, because `RoleAuditor` is not "less than a user" in a meaningful privilege ordering — it has different capabilities (read breadth, no write). See Step 5 for the precise refactor.
- `cases.Service.EnsureNotOnHold` + `internal/app/legal_hold_adapter.go` already gate destructive evidence operations. Sprint 11.5 builds storage-layer enforcement *on top of* this, not instead of it.
- Upload handler is **multipart/form-data** (`r.ParseMultipartForm`, `r.FormFile("file")` at `internal/evidence/handler.go:189`), not tus. Client-side hash is carried as a form field (`X-Content-SHA256` as a header AND as a multipart field for resilience), not a tus upload metadata entry.
- `internal/integrity/handler.go` holds verification jobs in `sync.Map` (handler.go:115). They are lost on restart. Sprint 11.5 introduces the `integrity_checks` persistence table (which was scoped to Sprint 11 but has not shipped).
- `internal/audit/` currently contains only `logger.go` (the auth audit logger). The audit dashboard and audit log query layer planned for Sprint 11 have not shipped yet. Sprint 11.5 does NOT re-scope those — it assumes Sprint 11 delivers them. If Sprint 11 slips, the dependency is flagged in the "Risks" section below.
- `docker-compose.yml`, `Caddyfile`, `Caddyfile.dev` live at the repo root (not under `deploy/`). All file paths in this plan match the actual layout.
- `deploy/server-bootstrap/bootstrap.sh` is a **host-hardening** script (SSH, fail2ban, apt). The VaultKeeper application bootstrap introduced in Step 9 is a separate script (`scripts/app-bootstrap.sh`) that composes with the host script, it does not replace it.
- MinIO image is pinned to `minio/minio:latest` in the current compose file. Sprint 11.5 pins it to a specific version that supports Object Lock compliance mode (tested on `RELEASE.2024-10-13T13-34-11Z` or later — to be confirmed in Step 4).
- No `docs/` directory exists at the repo root. Sprint 11.5 creates it.
- Keycloak realm is imported from `keycloak/realm-export.json`. Preset roles in Step 5 extend this file.
- The evidence upload flow currently does NOT surface a "cancel" path to the client once multipart form parsing starts. Step 1 fixes this.

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)
- [x] Database (migrations 020–025)
- [x] Infrastructure (MinIO Object Lock, air-gap config, Docker Compose)
- [x] CLI tool (standalone Go module: `vaultkeeper-verify`)
- [x] Documentation (operator handbook, air-gap guide, Berkeley Protocol mapping, custody action catalog)

---

## Implementation Steps

### Step 1: Client-Side Hashing with End-to-End Verification

**What exists today:**
- `internal/evidence/handler.go:173–248` (`Upload` handler) reads a multipart upload, constructs `UploadInput`, and calls `service.Upload`.
- `internal/evidence/handler.go:606–677` (`UploadNewVersion` handler) is a **separate** multipart upload code path for new evidence versions. It also lacks client-hash enforcement. **Both handlers must be updated identically in Step 1.**
- `internal/evidence/service.go` hashes the reader as it writes to MinIO. Only server-side.
- `internal/integrity/handler.go` runs post-hoc verification jobs against stored files.
- `web/src/components/evidence/evidence-uploader.tsx` exists and uses `fetch` or a similar uploader (to be confirmed at implementation time — it does NOT perform client-side hashing today).

**What is net-new:**
- Client-side streaming SHA-256 computation before any upload bytes leave the browser.
- Multipart form field `client_sha256` AND matching header `X-Content-SHA256`, both carrying the same value. The form field is the source of truth (survives HTTP/2 header rewriting); the header is a fast-path for the server to reject malformed requests before parsing the body.
- Server-side comparison of client-declared hash against bytes received.
- Hard 409 rejection on mismatch, no evidence row written, custody chain entry recorded, CRITICAL notification fired.
- `upload_attempts_v1` table persisting every attempt (including rejections) for forensic review.

**Threat model this closes:**
- Flaky network causes bit flips mid-upload → currently stored successfully, hashed post-corruption, undetected forever.
- Malicious middlebox modifies file in transit → same.
- Buggy browser extension or antivirus modifies file before send → same.
- Server-side bug (wrong reader, wrong encoding) alters bytes between receipt and storage → undetected.
- Operator tampering with files in MinIO *after* storage is covered by Sprint 11's integrity verifier; that already works. Sprint 11.5 closes the *in-transit* window which is currently wide open.

**Frontend changes:**

New file `web/src/lib/upload-hasher.ts`:
```typescript
// Streaming SHA-256 computed in the browser before upload.
// Must handle files up to 5 GB without OOM on mid-range hardware.
//
// The Web Crypto `subtle.digest` API does NOT support incremental hashing.
// We have two options:
//   (a) Read the whole file into an ArrayBuffer, call subtle.digest once.
//       OOMs on files > ~500 MB on low-end hardware.
//   (b) Use a WASM SHA-256 implementation that supports streaming updates
//       (hash-wasm is ~25KB gzipped and benchmarks within 1.5x of native
//       Web Crypto).
//
// Decision: use hash-wasm. We already have a web/package.json we can add it
// to. The performance hit is acceptable and the memory safety is not
// negotiable.
export async function hashFileStreaming(
  file: File,
  onProgress: (bytesHashed: number, total: number) => void,
  signal?: AbortSignal,
): Promise<string> { /* ... */ }
```

`web/src/components/evidence/evidence-uploader.tsx` gains a pre-upload phase:
1. User selects a file.
2. Component enters `hashing` state; progress bar labeled via i18n key `upload.hashing.label`.
3. `hashFileStreaming` runs. Cancel button aborts via `AbortController`.
4. On completion, the 64-char hex hash is displayed in a collapsible "Integrity receipt" panel with a copy-to-clipboard button. Text: "This is the cryptographic fingerprint of your file. Save this value — you can verify it later against the server-stored hash to confirm nothing was altered in transit."
5. Component transitions to `uploading` state. Upload POSTs to `/api/cases/:caseID/evidence` with the file AND a `client_sha256` form field AND an `X-Content-SHA256` header. Both carry the same 64-char hex value.
6. On 201 response, the server-reported hash is shown next to the client hash with a green checkmark when they match.
7. On 409 response, an error modal appears: "Upload failed integrity check. The file that reached our server does not match the file on your device. This is usually caused by a flaky connection or antivirus interference. Your original file is untouched." Offers "Retry upload" (re-hashes and re-uploads) and "Download diagnostic report" (JSON with client hash, server hash, byte count, user agent, timestamp). **Does NOT** offer "Upload anyway."

**Backend changes:**

The changes described below apply **identically** to both `Upload` (line 173) and `UploadNewVersion` (line 606). A shared helper `extractAndValidateClientHash(r *http.Request) (string, error)` lives in `internal/evidence/upload_hash.go` and both handlers call it before any further processing. Additionally, `respondServiceError` in `handler.go:751` is extended to map `ErrHashMismatch` to HTTP 409 so the sentinel has a single mapping path (no inline `errors.Is` branches scattered across handlers).

`internal/evidence/handler.go` — `Upload` handler (line 173) modifications:
```go
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
    // ... existing auth + case ID parsing ...

    // Fast reject: if X-Content-SHA256 header is missing or malformed,
    // reject before parsing the body. Saves bandwidth for clients that
    // misunderstand the API.
    headerHash := r.Header.Get("X-Content-SHA256")
    if !isValidSHA256Hex(headerHash) {
        httputil.RespondError(w, http.StatusBadRequest,
            "missing or malformed X-Content-SHA256 header")
        return
    }

    if err := r.ParseMultipartForm(32 << 20); err != nil {
        httputil.RespondError(w, http.StatusBadRequest, "invalid multipart form")
        return
    }

    // Source of truth: the form field. If the header and form field disagree,
    // reject: that indicates a malformed client or a middlebox tampering.
    formHash := r.FormValue("client_sha256")
    if !isValidSHA256Hex(formHash) {
        httputil.RespondError(w, http.StatusBadRequest,
            "missing or malformed client_sha256 form field")
        return
    }
    if !strings.EqualFold(formHash, headerHash) {
        httputil.RespondError(w, http.StatusBadRequest,
            "client_sha256 header and form field disagree")
        return
    }

    file, header, err := r.FormFile("file")
    // ... existing ...

    // Record the attempt *before* we start processing. This row survives
    // even if the upload crashes mid-stream, so we have a forensic record
    // of every hash the client ever declared.
    attemptID, err := h.attemptRepo.Record(r.Context(), UploadAttempt{
        CaseID:     caseID,
        UserID:     ac.UserID,
        ClientHash: strings.ToLower(formHash),
        StartedAt:  time.Now().UTC(),
    })
    // handle err

    input := UploadInput{
        // ... existing fields ...
        ExpectedSHA256: strings.ToLower(formHash),
        AttemptID:      attemptID,
    }

    evidence, err := h.service.Upload(r.Context(), input)
    if err != nil {
        respondServiceError(w, err) // extended to map ErrHashMismatch → 409
        return
    }
    // ... existing success path ...
}
```

Note the error reference is unqualified `ErrHashMismatch` (same package). It is mapped to HTTP 409 via `respondServiceError`, not inline. `UploadNewVersion` follows the identical pattern.

`internal/evidence/service.go` — `Upload` modifications:
- Wrap the reader in a `sha256.New()` `io.TeeReader` on the way into MinIO (staging object path).
- After MinIO returns success, compare the computed digest against `input.ExpectedSHA256`.
- **If they match:** proceed with the existing TSA + `evidence_items` insert path. `evidence_items.sha256_hash` is set to the verified hash. All PG writes (evidence_items insert, custody_log insert, upload_attempt_events success row) occur in a single PG transaction.
- **If they differ:** PG-transaction-only path — insert an `upload_attempt_events` row with `event_type = 'hash_mismatch'` including both hashes, insert a `custody_log` entry with action `upload_hash_mismatch`, insert a `notification_outbox` row (new table, Step 1.1 below), and return `ErrHashMismatch`. **Do not delete the staging object inline.** The deletion is a compensation operation picked up by an async worker.

**Rationale for outbox pattern:** PG and MinIO are not transactionally coupled. An inline `DeleteObject` after PG commit can fail (network error, MinIO down) leaving an orphan staging object AND a committed mismatch event — acceptable. An inline `DeleteObject` BEFORE PG commit can succeed while the PG commit later fails — unacceptable (we'd lose the audit trail of the attempt). The outbox pattern makes the audit-trail durable first and cleans up afterward. A reconciler in `internal/evidence/cleanup/` scans the outbox and issues MinIO deletes, retrying on failure, with a dead-letter after N attempts that escalates to a CRITICAL notification.

**Step 1.1: `notification_outbox` table** — append-only (enforced in Step 2), one row per pending out-of-band side-effect (MinIO delete, notification send). Consumed by `internal/evidence/cleanup/worker.go`. This table replaces ad-hoc "fire a notification" + "delete MinIO object" calls in the hash-mismatch path. It is NOT used for general-purpose messaging — only for compensation actions that must be durable.

New sentinel in `internal/evidence/errors.go`:
```go
var ErrHashMismatch = errors.New("upload hash mismatch")
```

**Migration 020: `020_upload_attempts.up.sql`**
```sql
BEGIN;

-- Every attempted upload, including rejections. The row is inserted
-- before the file is processed and never updated. State changes are
-- event-sourced via upload_attempt_events so the header stays immutable.
CREATE TABLE upload_attempts_v1 (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    user_id      UUID NOT NULL,
    client_hash  TEXT NOT NULL
        CHECK (client_hash ~ '^[0-9a-f]{64}$'),
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Composite index matching the query pattern "show all attempts for case X
-- ordered newest-first" (consistent with migration 019's pattern on
-- evidence_migrations and bulk_upload_jobs).
CREATE INDEX idx_upload_attempts_case_started
    ON upload_attempts_v1(case_id, started_at DESC);

CREATE TABLE upload_attempt_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id  UUID NOT NULL REFERENCES upload_attempts_v1(id) ON DELETE RESTRICT,
    event_type  TEXT NOT NULL
        CHECK (event_type IN (
            'bytes_received', 'hash_verified', 'hash_mismatch',
            'stored', 'rejected', 'aborted')),
    payload     JSONB NOT NULL DEFAULT '{}',
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_upload_attempt_events_attempt ON upload_attempt_events(attempt_id);

-- notification_outbox: durable compensation actions for MinIO/notification
-- side effects that cannot be rolled back with PG. Consumed by the cleanup
-- worker in internal/evidence/cleanup/.
CREATE TABLE notification_outbox (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action        TEXT NOT NULL
        CHECK (action IN ('minio_delete_object', 'notification_send',
                          'minio_copy_verify')),
    payload       JSONB NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 10,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,          -- set by the worker on success; row retained for audit
    dead_letter_at TIMESTAMPTZ,         -- set after max_attempts exceeded
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_outbox_pending
    ON notification_outbox(next_attempt_at)
    WHERE completed_at IS NULL AND dead_letter_at IS NULL;

-- Append-only enforcement on the three tables above is applied in
-- migration 021. notification_outbox is semi-append-only: worker updates
-- attempt_count and completed_at, so it uses a different policy (see 021).

COMMIT;
```

**Rate limiting:** both upload endpoints gain per-user rate limits at Caddy: 60 attempts/minute/user. This prevents `upload_attempts_v1` table DoS via malformed client-hash flooding. Configured in `Caddyfile` (and `Caddyfile.airgap`) as a new `rate_limit` block matching `/api/cases/*/evidence` and `/api/evidence/*/version`.

**Tests (unit):**
- `evidence.isValidSHA256Hex` — 64 lowercase hex passes, 63 chars fails, 65 chars fails, uppercase accepted, non-hex fails, empty fails.
- `evidence.service.Upload` happy path — client and server hashes match → evidence row + custody + TSA all written.
- `evidence.service.Upload` mismatch — returns `ErrHashMismatch`, `upload_attempt_events` row with `hash_mismatch`, `notification_outbox` row with `minio_delete_object`, custody event, NO evidence row. (Staging object is NOT deleted inline.)
- `evidence.service.Upload` with missing expected hash — panics / returns a programming error (caller bug, not a user input error; protected by the handler's validation).
- `evidence.handler.Upload` with missing `X-Content-SHA256` header → 400 with `missing_client_hash`.
- `evidence.handler.Upload` with malformed form field → 400 with `malformed_client_hash`.
- `evidence.handler.Upload` where header and form disagree → 400 with `hash_field_disagreement`.
- `evidence.handler.Upload` attempt record created even on reject path.
- **`evidence.handler.UploadNewVersion`**: identical validation and error mapping to `Upload`. Every `Upload` test above has a matching `UploadNewVersion` test.
- `evidence.cleanup.worker` — picks up `notification_outbox` rows with `next_attempt_at <= now()`, issues MinIO delete, marks `completed_at` on success; on failure increments `attempt_count` and backs off exponentially; after `max_attempts` sets `dead_letter_at` and fires CRITICAL notification via a separate synchronous path.
- `respondServiceError` — `ErrHashMismatch` maps to HTTP 409 with body `{"error":"upload_hash_mismatch"}`.

**Tests (frontend):**
- `upload-hasher.hashFileStreaming` — deterministic output for known test vectors.
- `upload-hasher.hashFileStreaming` — 0-byte file returns `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
- `upload-hasher.hashFileStreaming` — AbortSignal aborts mid-stream and rejects with `AbortError`.
- `upload-hasher.hashFileStreaming` — progress callback invoked at least once per 8 MiB chunk.
- `EvidenceUploader` component — renders `hashing` state with progress bar, transitions to `uploading` on hash complete, shows receipt panel.
- `EvidenceUploader` on 409 — shows error modal with diagnostic download, no "Upload anyway" affordance.

### Step 2: Extend Append-Only Enforcement to All Sensitive Tables

**What exists today:**
- Migration 003 enforces append-only on `custody_log` via RLS. `vaultkeeper_app` has `SELECT, INSERT` only; no UPDATE or DELETE policies exist; `FORCE ROW LEVEL SECURITY` blocks owner-level mutation.
- Migration 004 created `auth_audit_log` (login attempts, access denied events). **Verified: it has NO RLS.** Migration 021 must add it.
- `disclosures` table (migration 001): `vaultkeeper_app` has full INSERT/UPDATE/DELETE (migration 003 line 27 grants DML on `disclosures`). Disclosure events are trust-critical legal records; they must be append-only.
- `notifications` table has a mutable `read BOOLEAN` column, by design (users mark notifications as read). It **cannot** be fully append-only without breaking that flow. Sprint 11.5 does NOT make `notifications` append-only; instead, a separate `notification_read_events` table captures read-state transitions as append-only events, and the `read` column on `notifications` is treated as a materialized view of the latest event. See Migration 021.
- Sprint 11 creates the `integrity_checks` table as part of its audit/integrity work. Sprint 11.5 treats it as a prerequisite and extends append-only enforcement to it in Migration 021 — it does **not** duplicate the DDL. If Sprint 11 has not landed by Day 1 of Week 1, that is a Day-1 blocker (see Risks).

**What is net-new:**
- The same RLS pattern applied to: `upload_attempts_v1`, `upload_attempt_events`, `notification_outbox`, `notification_read_events`, `evidence_status_transitions` (Step 3), `legal_hold_events` (Step 4), `trigger_tamper_log` (new), **and the previously-uncovered `auth_audit_log` and `disclosures`**.
- RLS extension to `integrity_checks` (created by Sprint 11) so that the in-memory `sync.Map` state in `internal/integrity/handler.go:115` is replaced with durable, append-only state.
- A PostgreSQL DDL event trigger that records attempts to drop/modify any of the append-only-enforcing policies or triggers into `trigger_tamper_log`. The trigger is **SECURITY DEFINER** so the insert into `trigger_tamper_log` works under any invoking role.
- A new role `vaultkeeper_forensic_admin` that CAN delete from append-only tables, used only for operator-documented recovery procedures. Grants are revoked after each use. Its existence is noted in the operator handbook and its use is itself logged.
- An `ALTER ROLE` event trigger that logs all role grant modifications to `trigger_tamper_log` — this closes the bypass where an attacker with psql superuser grants LOGIN to `vaultkeeper_forensic_admin` to delete evidence silently.
- A CI integration test that, for every table marked append-only, exercises UPDATE/DELETE/TRUNCATE as `vaultkeeper_app` and asserts they fail.
- An explicit removal step: the `sync.Map` in `internal/integrity/handler.go:115` is deleted and all job state reads/writes go through the `integrity_checks` table. This is listed as a discrete sub-step to prevent dual state.

**Migration 021: `021_append_only_extensions.up.sql`**
```sql
BEGIN;

-- Establish a shared convention: every append-only table uses the
-- same RLS policy pattern as custody_log (migration 003).
-- Applied to 11 tables in this migration:
--   upload_attempts_v1
--   upload_attempt_events
--   notification_outbox        (semi-append; see special policy below)
--   notification_read_events   (see notifications split below)
--   evidence_status_transitions  (created in migration 022, enforced here)
--   legal_hold_events            (created in migration 023, enforced here)
--   trigger_tamper_log
--   auth_audit_log               (previously uncovered since migration 004)
--   disclosures                  (previously uncovered since migration 001)
--   integrity_checks             (Sprint 11 prerequisite; RLS added here)
--   compliance_reports           (created in migration 024, enforced here)

-- Helper macro pattern (repeated per table):
--
--   ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;
--   ALTER TABLE <t> FORCE ROW LEVEL SECURITY;
--   CREATE POLICY <t>_insert ON <t>
--       FOR INSERT TO vaultkeeper_app WITH CHECK (true);
--   CREATE POLICY <t>_select ON <t>
--       FOR SELECT TO vaultkeeper_app USING (true);
--   CREATE POLICY <t>_select_ro ON <t>
--       FOR SELECT TO vaultkeeper_readonly USING (true);
--   REVOKE UPDATE, DELETE, TRUNCATE ON <t> FROM vaultkeeper_app;
--   GRANT SELECT, INSERT ON <t> TO vaultkeeper_app;
--
-- The actual migration file contains 11 blocks. Abbreviated here.

-- Special case: notification_outbox (semi-append-only, worker updates
-- attempt_count and completed_at on rows it processes).
ALTER TABLE notification_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY notification_outbox_insert ON notification_outbox
    FOR INSERT TO vaultkeeper_app WITH CHECK (true);
CREATE POLICY notification_outbox_select ON notification_outbox
    FOR SELECT TO vaultkeeper_app USING (true);
-- Worker updates are constrained: only attempt_count, completed_at,
-- next_attempt_at, dead_letter_at. This is enforced in application code
-- via a named UPDATE; at the DB layer we allow UPDATE to vaultkeeper_app
-- but forbid modifying immutable fields via a row-level trigger.
CREATE POLICY notification_outbox_update ON notification_outbox
    FOR UPDATE TO vaultkeeper_app USING (true) WITH CHECK (true);
GRANT SELECT, INSERT, UPDATE ON notification_outbox TO vaultkeeper_app;
REVOKE DELETE, TRUNCATE ON notification_outbox FROM vaultkeeper_app;

CREATE OR REPLACE FUNCTION notification_outbox_immutable_fields()
    RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.action IS DISTINCT FROM OLD.action
       OR NEW.payload IS DISTINCT FROM OLD.payload
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.max_attempts IS DISTINCT FROM OLD.max_attempts THEN
        RAISE EXCEPTION
            'notification_outbox.{action,payload,created_at,max_attempts} are immutable'
            USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER notification_outbox_immutable
    BEFORE UPDATE ON notification_outbox
    FOR EACH ROW EXECUTE FUNCTION notification_outbox_immutable_fields();

-- notification_read_events: append-only record of notification-read
-- state transitions. The existing notifications.read column becomes a
-- derived field, synced via a trigger from the latest event.
CREATE TABLE notification_read_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE RESTRICT,
    user_id         UUID NOT NULL,
    event_type      TEXT NOT NULL CHECK (event_type IN ('read', 'unread')),
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_read_events_notif
    ON notification_read_events(notification_id, at DESC);
-- RLS on notification_read_events applied via the helper macro above.

-- Trigger tamper log
CREATE TABLE trigger_tamper_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_time    TIMESTAMPTZ NOT NULL DEFAULT now(),
    session_user  TEXT NOT NULL,
    current_user  TEXT NOT NULL,
    command_tag   TEXT NOT NULL,
    object_name   TEXT,
    sql_snippet   TEXT
);
-- RLS applied via the helper macro above.

-- Event trigger function runs as SECURITY DEFINER so the INSERT into
-- trigger_tamper_log succeeds under any invoking role (including the
-- migration superuser role). Owner of the function is set to a role
-- that has INSERT permission on trigger_tamper_log.
CREATE OR REPLACE FUNCTION log_ddl_on_append_only_tables() RETURNS event_trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    obj record;
    match_pattern TEXT := '(custody_log|upload_attempts_v1|upload_attempt_events|'
                          || 'notification_outbox|notification_read_events|'
                          || 'evidence_status_transitions|integrity_checks|'
                          || 'legal_hold_events|trigger_tamper_log|'
                          || 'auth_audit_log|disclosures|compliance_reports|'
                          || 'evidence_items_legal_hold_delete_guard|'
                          || 'cases_legal_hold_delete_guard)';
BEGIN
    FOR obj IN SELECT * FROM pg_event_trigger_ddl_commands()
    LOOP
        IF obj.object_identity ~ match_pattern
           OR tg_tag IN ('ALTER TABLE', 'DROP TABLE', 'DROP POLICY',
                         'ALTER POLICY', 'DROP TRIGGER', 'ALTER TRIGGER',
                         'REVOKE', 'GRANT')
        THEN
            INSERT INTO trigger_tamper_log (
                session_user, current_user, command_tag, object_name, sql_snippet
            ) VALUES (
                session_user, current_user, tg_tag, obj.object_identity,
                left(current_query(), 500)
            );
        END IF;
    END LOOP;
END;
$$;
ALTER FUNCTION log_ddl_on_append_only_tables()
    OWNER TO vaultkeeper_migrations;  -- owner role created in Step 2.1 below

CREATE EVENT TRIGGER append_only_ddl_audit
    ON ddl_command_end
    EXECUTE FUNCTION log_ddl_on_append_only_tables();

-- ALTER ROLE event trigger (separate event, different function signature).
-- Catches ALTER ROLE ... LOGIN and GRANT/REVOKE role-level operations
-- that the ddl_command_end trigger does NOT fire on.
CREATE OR REPLACE FUNCTION log_role_modifications() RETURNS event_trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF tg_tag IN ('ALTER ROLE', 'CREATE ROLE', 'DROP ROLE', 'GRANT ROLE', 'REVOKE ROLE') THEN
        INSERT INTO trigger_tamper_log (
            session_user, current_user, command_tag, object_name, sql_snippet
        ) VALUES (
            session_user, current_user, tg_tag, NULL,
            left(current_query(), 500)
        );
    END IF;
END;
$$;
ALTER FUNCTION log_role_modifications() OWNER TO vaultkeeper_migrations;

CREATE EVENT TRIGGER role_modifications_audit
    ON ddl_command_start   -- role commands fire on ddl_command_start, not end
    EXECUTE FUNCTION log_role_modifications();

-- Forensic recovery role. Login is disabled by default; operators grant
-- LOGIN explicitly only during a documented recovery procedure. Every
-- such grant fires role_modifications_audit above.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultkeeper_forensic_admin') THEN
        CREATE ROLE vaultkeeper_forensic_admin NOLOGIN;
    END IF;
END
$$;

-- Migration role for owning SECURITY DEFINER functions and performing
-- privileged schema operations. This role is distinct from both
-- vaultkeeper_app (runtime) and postgres (superuser). It is the owner of
-- the append-only tables and the event-trigger functions.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'vaultkeeper_migrations') THEN
        CREATE ROLE vaultkeeper_migrations NOLOGIN;
        GRANT INSERT ON trigger_tamper_log TO vaultkeeper_migrations;
    END IF;
END
$$;

COMMIT;
```

**Step 2.1: `sync.Map` removal in `internal/integrity/handler.go`.** The in-memory job map at line 115 is replaced with `integrity_checks`-backed state. A new method `integrity.Repository.ClaimActiveJob(caseID)` atomically inserts a row with `status='running'` and returns `ErrAlreadyRunning` on conflict (unique partial index on `case_id WHERE status='running'`). `GetStatus` reads from the table. `runVerification` updates the row with progress, then finalizes it on completion. This enables multi-replica deployments and survives restarts. The existing `sync.Map` field, the `jobs` member, and every reference to it are deleted — no dual state.

**Important notes on SECURITY DEFINER and RLS:**
- The event trigger function runs as `vaultkeeper_migrations` (the OWNER) because of `SECURITY DEFINER`, regardless of the invoking session.
- `vaultkeeper_migrations` has a narrow `INSERT` grant on `trigger_tamper_log` (granted in the same migration).
- `FORCE ROW LEVEL SECURITY` on `trigger_tamper_log` does NOT bypass ownership-based RLS behavior for SECURITY DEFINER functions — the RLS policy on `trigger_tamper_log` is evaluated against `vaultkeeper_migrations` as the current user. Therefore the INSERT policy must include `vaultkeeper_migrations` in its TO list:
  ```sql
  CREATE POLICY trigger_tamper_insert ON trigger_tamper_log
      FOR INSERT TO vaultkeeper_app, vaultkeeper_migrations WITH CHECK (true);
  ```
- The migration integration test must exercise this: run a DDL command as a test superuser, verify a row lands in `trigger_tamper_log`.

**Notifications split (keeping the read flag working):**
The existing `notifications.read BOOLEAN` flag is preserved as a **denormalized read model** of `notification_read_events`. Application code no longer writes directly to `notifications.read`; instead it inserts a row into `notification_read_events`, and a trigger syncs the latest state onto `notifications.read`. This preserves the existing query pattern "WHERE read = false" without compromising append-only.

```sql
CREATE OR REPLACE FUNCTION sync_notification_read_state() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE notifications
       SET read = (NEW.event_type = 'read')
     WHERE id = NEW.notification_id;
    RETURN NEW;
END;
$$;

CREATE TRIGGER notification_read_events_sync
    AFTER INSERT ON notification_read_events
    FOR EACH ROW EXECUTE FUNCTION sync_notification_read_state();
```
The `notifications` table itself is NOT made append-only. It retains UPDATE permission on the `read` column only — this is acceptable because the true audit trail lives in the append-only `notification_read_events` table.

**Important design notes:**
- We deliberately **do not use BEFORE triggers that RAISE EXCEPTION** for append-only enforcement. The RLS + role-grants approach is already working in migration 003 and has survived audit. Duplicating the enforcement with triggers adds complexity and a second mental model for operators to reason about. We extend the existing approach.
- The DDL event trigger only *records* tampering attempts; it does not block them. An operator who genuinely needs to recreate a policy during a schema upgrade must be able to do so. The tamper log ensures every such action is visible in the audit dashboard.
- `auth_audit_log` has **no RLS** (verified from migration 004). Migration 021 adds it.
- The DDL trigger uses a single `SECURITY DEFINER` function owned by `vaultkeeper_migrations` so that inserts into `trigger_tamper_log` succeed regardless of invoking role. Without this, inserts from a DDL context would fail under `FORCE ROW LEVEL SECURITY`.
- The `ALTER ROLE` event trigger is a **separate event trigger** on `ddl_command_start` because role commands do not fire `ddl_command_end`.

**Tests:**
- For each of the 11 append-only tables (plus `notification_outbox` as semi-append): run migration, attempt UPDATE / DELETE / TRUNCATE as `vaultkeeper_app` → each returns a PostgreSQL error.
- `notification_outbox` accepts targeted UPDATE of `attempt_count`, `completed_at`, `next_attempt_at`, `dead_letter_at`; modification of `action`, `payload`, `created_at`, `max_attempts` is blocked by `notification_outbox_immutable_fields`.
- INSERT succeeds as `vaultkeeper_app`.
- SELECT succeeds as both `vaultkeeper_app` and `vaultkeeper_readonly`.
- `DROP POLICY custody_log_insert` as the migration owner → succeeds BUT `trigger_tamper_log` gains a new row. Repeated for `auth_audit_log`, `disclosures`, `compliance_reports`.
- `ALTER ROLE vaultkeeper_forensic_admin LOGIN PASSWORD 'x'` executed as a test superuser → row in `trigger_tamper_log` with command_tag `ALTER ROLE` (closes the forensic-admin bypass).
- `ALTER TABLE evidence_items DISABLE TRIGGER evidence_items_legal_hold_delete_guard` → row in `trigger_tamper_log` (the trigger name itself is in the regex).
- **SECURITY DEFINER insert path:** run a DDL command as the test superuser (not `vaultkeeper_app`), verify a row lands in `trigger_tamper_log` despite `FORCE ROW LEVEL SECURITY` being active.
- Event trigger does NOT fire on DDL against unrelated tables (false-positive guard).
- **Notifications sync trigger:** insert a `read` event into `notification_read_events`, assert `notifications.read = true`. Insert an `unread` event, assert `notifications.read = false`. Verify the append-only log preserves both events.
- **Integrity jobs migration:** `integrity.Repository.ClaimActiveJob` returns `ErrAlreadyRunning` when a running row exists for the same case. GetStatus reads from the table, not from any in-memory state. Server restart preserves job state — stale `running` rows are reaped by a scheduled cleanup job after a configurable timeout.
- Performance: inserting 10,000 rows into `custody_log` with all policies and the event trigger active adds no more than 5% latency vs. a bare table.
- **CI integration test** `postgres_integration_test.go`: loops through every table in the curated `append_only_registry.json` and exercises the forbidden verbs. New sensitive tables added in later sprints must be registered or CI fails.

### Step 3: Enforced Evidence Lifecycle

**What exists today:**
- `evidence_items` has `tsa_status`, `is_current`, `version`, `destroyed_at`, `destroyed_by`, `destroy_reason` — but no review/admissibility status.
- Sprint 19's configurable workflow engine is planned but not built.
- Disclosures (Sprint 8) have their own state; classifications (Sprint 9) are independent of review state.

**What is net-new:**
- A non-configurable five-state review lifecycle on `evidence_items`:
  `ingested → under_review → verified → admitted | rejected`
- Role-gated transitions (Investigator / Prosecutor / Judge depending on direction and target state).
- Per-item lock flag that blocks all transitions (including unlocking) except by system admin.
- Per-item `evidence_status_transitions` row on every transition (append-only).
- Custody log entry on every transition.
- Notifications to case members on every transition.
- Frontend status badge shown in grid, detail, timeline, search, export.

**Explicit relationship to Sprint 19 (revised — earlier plan version contained a mapping claim that audit review falsified):**

Sprint 19's configurable workflow engine stores state as `workflow_states` (one row per evidence item with `current_stage TEXT` and an embedded `decisions JSONB` array) and `workflow_configs` (one row per case with a JSONB stage definition). Sprint 11.5's five-state lifecycle **is not cleanly subsumed** by Sprint 19's model:
- Sprint 11.5's `evidence_status_transitions` is an append-only transition *event log*; Sprint 19's `workflow_states` is a current-state *snapshot* with embedded decision history.
- Sprint 11.5's `rejected` terminal state has **no equivalent** in Sprint 19's default stage template (Sprint 19 models rejection as return-to-previous-stage, not a terminal state).
- Sprint 11.5's roles are hard-coded (prosecutor/judge); Sprint 19's are case-role-driven.

**Decision:** Sprint 11.5's lifecycle is the **permanent default model** for pilots. Sprint 19 will add a *configurable overlay* on top of it, not replace it. Specifically:
- Sprint 19 keeps `evidence_items.review_status` and `evidence_status_transitions` as the authoritative state.
- Sprint 19's `workflow_configs` becomes a per-case extension that can require *additional* checkpoints inside the `ingested → under_review → verified → admitted/rejected` pipeline (e.g., two-prosecutor sign-off before `verified → admitted`).
- The `rejected` state remains terminal; Sprint 19's "return to previous stage" model is layered atop it via an "unreject by judge" action already in Sprint 11.5.

Sprint 19's plan file will be updated in a separate change to reference this decision. Sprint 11.5's Step 3 **is** the workflow engine's state model; Sprint 19 just adds configurability on top.

**Migration 022: `022_evidence_lifecycle.up.sql`**
```sql
BEGIN;

ALTER TABLE evidence_items
    ADD COLUMN review_status TEXT NOT NULL DEFAULT 'ingested'
        CHECK (review_status IN ('ingested', 'under_review', 'verified', 'admitted', 'rejected')),
    ADD COLUMN review_status_locked BOOLEAN NOT NULL DEFAULT false,
    -- Monotonic version counter for optimistic concurrency. Timestamps are
    -- NOT suitable because two transitions within the same microsecond would
    -- both win the WHERE clause. Incremented atomically on every transition.
    ADD COLUMN review_version BIGINT NOT NULL DEFAULT 1,
    -- Display-only change metadata. review_version is the lock predicate.
    ADD COLUMN review_status_changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Actor is non-null for every new transition. Existing rows backfill
    -- to a sentinel UUID representing "system initialization" — tracked
    -- separately so we can distinguish genuine user actions from backfills.
    ADD COLUMN review_status_changed_by UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';

-- Add a CHECK that distinguishes the sentinel from a real actor for
-- any row that has left the ingested state. An item that has been
-- transitioned must carry a non-sentinel actor.
ALTER TABLE evidence_items ADD CONSTRAINT chk_review_actor_non_sentinel
    CHECK (
        review_status = 'ingested'
        OR review_status_changed_by <> '00000000-0000-0000-0000-000000000000'
    );

CREATE INDEX idx_evidence_review_status ON evidence_items(case_id, review_status);

CREATE TABLE evidence_status_transitions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    case_id         UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    from_status     TEXT NOT NULL,
    to_status       TEXT NOT NULL,
    from_version    BIGINT NOT NULL,  -- version before the transition
    to_version      BIGINT NOT NULL,  -- version after (from_version + 1)
    actor_user_id   UUID NOT NULL,
    reason          TEXT NOT NULL DEFAULT '',
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_status_transitions_evidence ON evidence_status_transitions(evidence_id, at DESC);
CREATE INDEX idx_status_transitions_case ON evidence_status_transitions(case_id);

-- Trigger guard: evidence_status_transitions.case_id MUST match
-- evidence_items.case_id at the time of the insert. Prevents denormalization
-- drift if evidence is ever reassigned between cases (not currently
-- supported, but defensive).
CREATE OR REPLACE FUNCTION enforce_status_transition_case_id() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    evidence_case UUID;
BEGIN
    SELECT case_id INTO evidence_case FROM evidence_items WHERE id = NEW.evidence_id;
    IF evidence_case IS DISTINCT FROM NEW.case_id THEN
        RAISE EXCEPTION 'evidence_status_transitions.case_id must match evidence_items.case_id'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER status_transition_case_id_guard
    BEFORE INSERT ON evidence_status_transitions
    FOR EACH ROW EXECUTE FUNCTION enforce_status_transition_case_id();

-- Append-only enforcement applied in migration 021.

COMMIT;
```

The column name is `review_status`, deliberately distinct from the existing `tsa_status`, to avoid any ambiguity in code review or SQL queries. Prior sprint plans called this field `status` — the actual column name is scoped to `review_status` to prevent collision with the existing `tsa_status` and the implicit `cases.status` column.

**Interfaces (`internal/lifecycle/service.go`, new):**

Per Go convention (keep interfaces small, accept interfaces — return structs), the lifecycle functionality is split into three narrow interfaces and a concrete `Service` struct that implements all three. Handlers depend only on the interface they need.

```go
type Status string

const (
    Ingested    Status = "ingested"
    UnderReview Status = "under_review"
    Verified    Status = "verified"
    Admitted    Status = "admitted"
    Rejected    Status = "rejected"
)

// Transitioner handles forward and backward state transitions.
type Transitioner interface {
    Transition(ctx context.Context, cmd TransitionCommand) (TransitionResult, error)
}

// Locker handles lock/unlock of individual evidence items.
type Locker interface {
    Lock(ctx context.Context, evidenceID uuid.UUID, actorID uuid.UUID, reason string) error
    Unlock(ctx context.Context, evidenceID uuid.UUID, actorID uuid.UUID, reason string) error
}

// HistoryReader returns the append-only transition log for an item.
type HistoryReader interface {
    History(ctx context.Context, evidenceID uuid.UUID) ([]Transition, error)
}

// TransitionCommand carries everything needed for one transition including
// the actor ID (explicit parameter per codebase convention) and the
// expected version for optimistic concurrency.
type TransitionCommand struct {
    EvidenceID      uuid.UUID
    To              Status
    Reason          string
    ActorID         uuid.UUID
    ExpectedVersion int64  // from If-Match header; 0 means "skip the check"
}

// TransitionResult carries the new version and the updated_at for the
// response body and for ETag generation.
type TransitionResult struct {
    Version    int64
    ChangedAt  time.Time
    FromStatus Status
    ToStatus   Status
}

// Service is the concrete implementation that satisfies all three interfaces.
type Service struct {
    repo     Repository
    custody  CustodyLogger
    notifier Notifier
}
```

**Optimistic concurrency: `If-Match` ETag, not `If-Unmodified-Since`.**

HTTP `If-Unmodified-Since` is second-granular and unsuitable for microsecond-rate transitions. The API uses `If-Match` with an opaque ETag that encodes the monotonic `review_version`:

- Client reads `GET /api/evidence/:id` → response carries `ETag: "v:{review_version}"` (e.g. `ETag: "v:42"`).
- Client submits transition with `If-Match: "v:42"`.
- Server parses the ETag, extracts the version integer, runs `UPDATE evidence_items SET review_status = ?, review_version = review_version + 1, ... WHERE id = ? AND review_version = ?`. If `RowsAffected() == 0`, the version is stale → 409 Conflict.
- Missing `If-Match` header → 428 Precondition Required (per RFC 6585).
- The successful response carries the new ETag for the next request.

This matches the existing `ExpectedClassification` optimistic-concurrency pattern in `internal/evidence/repository.go`.

**Transition matrix:**

| From | To | Allowed roles (case) | Reason required |
|------|----|----------------------|-----------------|
| `ingested` | `under_review` | investigator, prosecutor | no |
| `under_review` | `verified` | prosecutor, judge | no |
| `under_review` | `ingested` | prosecutor | **yes** (backward) |
| `verified` | `admitted` | prosecutor, judge | no |
| `verified` | `rejected` | judge | **yes** |
| `verified` | `under_review` | prosecutor | **yes** (backward) |
| `admitted` | `verified` | judge | **yes** (backward, un-admit) |
| `rejected` | `verified` | judge | **yes** (backward, un-reject) |

**Endpoints:**
```
POST /api/evidence/:id/review/transition  { to, reason } → 200 | 403 | 409 | 422
POST /api/evidence/:id/review/lock        { reason }     → 200
POST /api/evidence/:id/review/unlock      { reason }     → 200
GET  /api/evidence/:id/review/history                    → 200 with transition list
```

**Concurrency:** see the `If-Match` ETag pattern above. The `review_version` BIGINT is the lock predicate; `review_status_changed_at` is display-only.

**Frontend:**

- New component `web/src/components/evidence/review-status-badge.tsx` — color-coded pill with i18n labels. Gray `ingested`, blue `under_review`, teal `verified`, navy `admitted`, amber `rejected`. Lock icon overlay when `review_status_locked`.
- New component `web/src/components/evidence/review-status-panel.tsx` — detail page panel showing current state, last changed by/at, transition history, role-gated action buttons. Disabled buttons show a tooltip explaining why ("Only Prosecutors or Judges can verify an item").
- `evidence-grid.tsx` and `evidence-detail.tsx` render the badge.
- Timeline (`web/src/components/timeline/*` — pending Sprint 11 completion) and search results also render the badge once they exist.
- i18n strings added to `web/src/messages/en.json` and `web/src/messages/fr.json` for every state, every transition verb, every error message, every tooltip.

**Tests:**
- Unit: each valid transition succeeds for each correct role, invalid transitions return `invalid_transition`, wrong role returns 403, locked item blocks all transitions, backward transition without reason returns 422 `reason_required`.
- Unit: lock/unlock toggle, only system admin can unlock a locked item.
- Unit: custody log entry written in same transaction as the status update.
- Integration (testcontainers): end-to-end upload → ingested → under_review → verified → admitted with role switching; assert custody_log entries, transitions table entries, evidence_items.review_status all consistent.
- Integration: concurrent transitions — two goroutines Transition same item → one 200, one 409.
- Integration: interaction with legal hold — transition on a held case is allowed (metadata, not destructive) but the transition event records `legal_hold_active=true` in its detail.
- Integration: interaction with disclosure (Sprint 8) — attempting to disclose an `ingested` or `under_review` item returns 422 `not_yet_reviewable`; `verified` and `admitted` items can be disclosed.
- E2E: Playwright flow exercising the full lifecycle from upload to admission with role switching.

### Step 4: Legal Hold at the Storage Layer

**What exists today:**
- `cases.legal_hold` boolean (migration 005).
- `cases.Service.EnsureNotOnHold` gates destructive operations in the cases service.
- `internal/app/legal_hold_adapter.go` bridges the cases sentinel error `cases.ErrLegalHoldActive` to the evidence sentinel `evidence.ErrLegalHoldActive`.
- GDPR erasure conflict resolution (Sprint 9, migration 017) checks `legal_hold` before proceeding.
- Frontend `legal-hold-control.tsx` toggles the flag.

**What is net-new:**
- A second MinIO bucket `evidence-locked` configured with Object Lock (compliance retention mode, 100-year retention period at object creation).
- On hold placement: every evidence object in the case is copied to `evidence-locked`, the original is deleted, `evidence_items.storage_bucket` is updated to `'evidence-locked'`. Custody log entries for every moved object.
- On hold release: DB flag flips, but objects remain in `evidence-locked` (compliance mode does not permit early release of retention).
- PostgreSQL row-level triggers that block `DELETE FROM evidence_items` and `DELETE FROM cases` when the row is on hold, at the storage layer. This is the only case in Sprint 11.5 where we use a RAISE EXCEPTION trigger — because RLS does not support conditional deletion based on joined table state, and this is a defense-in-depth layer beyond the service-layer `EnsureNotOnHold` check.
- A new append-only `legal_hold_events` table capturing place/release events with reason text.
- MinIO image pinned to a version that supports Object Lock compliance mode (target: `minio/minio:RELEASE.2024-10-13T13-34-11Z` or later; actual version confirmed at implementation time against the MinIO release notes for compliance-mode stability).

**Threat model this closes:**
- Admin under duress runs `DELETE FROM evidence_items WHERE case_id = ?` via psql → blocked by trigger.
- Admin runs `mc rm local/evidence-locked/<key>` → blocked by MinIO Object Lock.
- Compromised backup job tries to overwrite an object → blocked.
- Well-intentioned cleanup script deletes a held item → blocked.
- Insider attempting to hide evidence before a subpoena → blocked at both layers.

**Migration 023: `023_legal_hold_storage.up.sql`**
```sql
BEGIN;

ALTER TABLE evidence_items
    ADD COLUMN storage_bucket TEXT NOT NULL DEFAULT 'evidence'
        CHECK (storage_bucket IN ('evidence', 'evidence-locked')),
    ADD COLUMN legal_hold_migration_state TEXT NOT NULL DEFAULT 'none'
        CHECK (legal_hold_migration_state IN (
            'none', 'pending', 'copied', 'verified', 'original_deleted', 'done'));

CREATE INDEX idx_evidence_lhms ON evidence_items(case_id, legal_hold_migration_state)
    WHERE legal_hold_migration_state <> 'none';

ALTER TABLE cases
    ADD COLUMN ever_held BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE legal_hold_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id     UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    event_type  TEXT NOT NULL CHECK (event_type IN ('placed', 'released')),
    actor_user_id UUID NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    object_count INT,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- object_count is required on 'placed' events and must be NULL on 'released'.
    CONSTRAINT chk_legal_hold_object_count
        CHECK (
            (event_type = 'placed' AND object_count IS NOT NULL AND object_count >= 0)
            OR (event_type = 'released' AND object_count IS NULL)
        )
);
CREATE INDEX idx_legal_hold_events_case ON legal_hold_events(case_id, at DESC);
-- Append-only enforcement applied in migration 021.

-- Row-level delete guards. These are BEFORE DELETE triggers that RAISE
-- EXCEPTION if the parent case is held. They are a storage-layer backstop
-- for cases.Service.EnsureNotOnHold.
CREATE OR REPLACE FUNCTION reject_held_evidence_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    case_held BOOLEAN;
BEGIN
    SELECT legal_hold INTO case_held FROM cases WHERE id = OLD.case_id;
    IF case_held THEN
        RAISE EXCEPTION 'evidence % belongs to a case on legal hold and cannot be deleted', OLD.id
            USING ERRCODE = 'insufficient_privilege',
                  HINT = 'release the legal hold via the admin API, which requires system admin role and logs the event';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER evidence_items_legal_hold_delete_guard
    BEFORE DELETE ON evidence_items
    FOR EACH ROW EXECUTE FUNCTION reject_held_evidence_delete();

CREATE OR REPLACE FUNCTION reject_held_case_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.legal_hold THEN
        RAISE EXCEPTION 'case % is on legal hold and cannot be deleted', OLD.id
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER cases_legal_hold_delete_guard
    BEFORE DELETE ON cases
    FOR EACH ROW EXECUTE FUNCTION reject_held_case_delete();

COMMIT;
```

**Docker Compose changes (root `docker-compose.yml`):**

Add a `minio-setup` one-shot service that runs `mc` commands to create the two buckets with the appropriate settings. This service was not present in the current compose file; the buckets are created lazily by the application today. Moving bucket creation into compose makes the deployment reproducible.

```yaml
  minio-setup:
    image: minio/mc:RELEASE.2024-10-08T09-37-26Z   # pinned
    depends_on:
      minio:
        condition: service_healthy
    environment:
      MC_HOST_local: http://${MINIO_ACCESS_KEY}:${MINIO_SECRET_KEY}@minio:9000
    entrypoint: >
      /bin/sh -c "
      set -e;
      mc mb --ignore-existing local/evidence;
      mc mb --with-lock --ignore-existing local/evidence-locked;
      mc retention set --default compliance 100y local/evidence-locked;
      mc encrypt set sse-s3 local/evidence;
      mc encrypt set sse-s3 local/evidence-locked;
      mc anonymous set none local/evidence;
      mc anonymous set none local/evidence-locked;
      "
    networks:
      - vaultkeeper
```

And pin the MinIO image itself:
```yaml
  minio:
    image: minio/minio:RELEASE.2024-10-13T13-34-11Z   # verified for Object Lock compliance mode
```

**Go implementation:**

New package `internal/legalhold/storage.go`, split into two narrow interfaces (small-interface convention):

```go
type HoldPlacer interface {
    PlaceHold(ctx context.Context, req PlaceHoldRequest) (PlaceHoldResult, error)
}

type HoldReleaser interface {
    ReleaseHold(ctx context.Context, req ReleaseHoldRequest) error
}

type PlaceHoldRequest struct {
    CaseID  uuid.UUID
    ActorID uuid.UUID
    Reason  string
}

type PlaceHoldResult struct {
    CaseID       uuid.UUID
    ObjectsMoved int
    Resumable    bool  // true if partial; caller should re-invoke to finish
    FailedItems  []ItemMigrationError
}
```

**`PlaceHold` as a resumable per-item state machine.**

The previous draft claimed per-item atomicity across PG and MinIO. That is impossible — MinIO has no transactions. The corrected design uses a new per-row state column on `evidence_items`:

```sql
-- Part of migration 023
ALTER TABLE evidence_items
    ADD COLUMN legal_hold_migration_state TEXT NOT NULL DEFAULT 'none'
        CHECK (legal_hold_migration_state IN (
            'none', 'pending', 'copied', 'verified', 'original_deleted', 'done'));
```

Each evidence item moves through the states: `none → pending → copied → verified → original_deleted → done`. Every transition is a small PG update, and each step reads the current state to decide what to do next. The operation is:
- **Idempotent**: re-running on an item already in `done` is a no-op.
- **Resumable**: a crash mid-migration leaves the item in a known state; the next invocation picks up from there.
- **Observable**: the admin dashboard shows progress per item.

Algorithm:
1. Inside a short PG transaction: select all evidence items for the case where `legal_hold_migration_state != 'done'` `FOR UPDATE`. Mark each as `pending`. Commit.
2. For each item (respecting `ctx.Done()` between items — see context handling below):
   a. If state `pending`: `CopyObject` evidence → evidence-locked with 100-year compliance retention. On success, UPDATE state `copied`. On error: record in `FailedItems`, continue with next item (do not abort the whole batch).
   b. If state `copied`: HeadObject on the copy, compare ETag to the source's ETag. On match, UPDATE state `verified`. On mismatch, delete the bad copy, revert state to `pending`, record in `FailedItems`, continue.
   c. If state `verified`: `RemoveObject` from source bucket. UPDATE state `original_deleted`. On error: record, continue.
   d. If state `original_deleted`: UPDATE `evidence_items.storage_bucket = 'evidence-locked'`, state `done`. INSERT custody_log entry `moved_to_locked_bucket` with source and destination keys. (Same PG txn — both PG writes.)
3. After all items processed, in one PG transaction: if every item for the case is in state `done`, set `cases.legal_hold = true, cases.ever_held = true`. Insert `legal_hold_events` row with event_type `placed`, actor, reason, `object_count = count of done items`. Otherwise return `Resumable: true` — the operator sees the partial state in the UI and can re-invoke `PlaceHold` which will resume exactly where it stopped.
4. An out-of-band reconciler job scans every minute for cases with `legal_hold = true` and any `evidence_items` where `storage_bucket = 'evidence'` (drift detection — should be empty under normal operation). If drift is found, it logs a CRITICAL notification naming the items.

**Context handling:** `ctx` propagates from the HTTP request throughout. The for-loop checks `ctx.Err()` between items and stops cleanly on cancellation, leaving already-done items in `done` and pending items in whatever state they reached. Cancellation is NOT a failure — it's a pause. The `Resumable: true` flag communicates this to the caller. The admin UI exposes a "resume migration" button for partially-placed holds. `context.Background()` is NEVER used inside the migration loop.

**Pipelined concurrency:** within a single `PlaceHold` call, items are processed in bounded parallelism (default: 4 goroutines, configurable). Each goroutine runs the per-item state machine independently. The `FOR UPDATE` in step 1 serializes concurrent `PlaceHold` invocations against the same case.

**`ReleaseHold` algorithm:**
1. Short PG transaction: `UPDATE cases SET legal_hold = false WHERE id = ?`. `INSERT INTO legal_hold_events` with event_type `released`, actor, reason (object_count NULL — see CHECK constraint below).
2. Objects are NOT moved back. Compliance-mode Object Lock does not permit lifting retention early. They remain in `evidence-locked` until the 100-year retention expires. The application reads from the right bucket using `evidence_items.storage_bucket`.
3. The operator handbook prominently warns that hold release is a flag flip only; the storage-layer protection is permanent for already-placed items.

**Cross-Region Replication (CRR) caveat (new section added to operator handbook):** MinIO's default CRR does NOT replicate Object Lock retention metadata. Operators running multi-region deployments MUST configure CRR with `--replicate existing-objects,delete,delete-marker,replica-metadata-sync` and enable Object Lock on the destination bucket at creation. The operator handbook's air-gap guide includes a dedicated section on this. Without it, a CRR destination is a deletable copy of evidence that appears to operators as a backup but cryptographically is not.

**New custody actions** (added to the catalog in `docs/custody-actions.md`):
- `moved_to_locked_bucket` — detail: `{source_key, destination_key, object_size}`.
- `legal_hold_placed` — detail: `{reason, object_count}`.
- `legal_hold_released` — detail: `{reason}`.

**Integration with existing `cases.Service.EnsureNotOnHold`:**
- The service-layer check remains. Storage-layer enforcement is a backstop, not a replacement.
- `internal/app/legal_hold_adapter.go` gains a new method `PlaceHold(caseID, actor, reason)` that calls the new StorageEnforcer and then the cases service to flip the flag within a single transaction.
- The existing `legal-hold-control.tsx` frontend component gains a confirmation dialog explaining "Placing a legal hold is effectively permanent. Objects are moved to a compliance-locked storage bucket and cannot be deleted for 100 years by anyone, including system administrators. Are you sure?"

**Tests:**
- Unit: `PlaceHold` happy path — all items moved, custody events written, DB flag set.
- Unit: `PlaceHold` partial failure — one `CopyObject` fails → operation aborts, partial results returned, already-moved items documented, no DB flag flip.
- Unit: `ReleaseHold` — flag flips, objects remain in locked bucket.
- Integration (testcontainers with MinIO): place hold on case with 5 items → all 5 in `evidence-locked`, originals gone from `evidence`, `DELETE FROM evidence_items WHERE id = ?` fails with `insufficient_privilege`.
- Integration: `mc rm local/evidence-locked/<key>` fails with MinIO retention error.
- Integration: after `ReleaseHold`, objects still in `evidence-locked`, the app can still read them via the `storage_bucket` switch.
- Integration: interaction with GDPR erasure — erasure on held item fails loudly.
- Integration: interaction with upload — new uploads during a hold go to `evidence` (not `evidence-locked`) and are included in the next hold-placement if a subsequent hold is placed. (Operator docs explain this.)
- Performance: `PlaceHold` on 1000-item case completes within 10 minutes on commodity hardware.
- Manual E2E: documented procedure for operators to verify the storage lock is working against their deployment.

### Step 5: Preset Role Templates + Auditor Read-Only Role

**What exists today — verified by reading `internal/auth/context.go`:**
- `SystemRole` is an `int` enum with **four** values, not three:
  ```go
  const (
      RoleAPIService SystemRole = iota  // 0 — background services, API keys
      RoleUser                           // 1
      RoleCaseAdmin                      // 2
      RoleSystemAdmin                    // 3
  )
  ```
- `ParseSystemRole` accepts the strings `"api_service"`, `"user"`, `"case_admin"`, `"system_admin"`.
- `internal/auth/permissions.go:26` uses `ac.SystemRole < minimum` for gating — an ordinal check that treats the enum as a strict privilege ladder.
- Case roles are stored in `case_roles` table and queried per case. The UI does not surface preset templates.
- `keycloak/realm-export.json` defines the realm. No preset-role client mappings exist.

**What is net-new:**
- A **fifth** system role `RoleAuditor` added at the end of the enum (value **4**, not 0 — inserting at 0 would silently clobber `RoleAPIService` and break API-key authentication on the day of rollout). The enum becomes:
  ```go
  const (
      RoleAPIService SystemRole = iota  // 0 — unchanged
      RoleUser                           // 1 — unchanged
      RoleCaseAdmin                      // 2 — unchanged
      RoleSystemAdmin                    // 3 — unchanged
      RoleAuditor                        // 4 — NEW
  )
  ```
  `ParseSystemRole` gains a case for `"auditor"`. `String()` gains a case for `RoleAuditor`. No existing integer values shift.
- **Capability-check refactor.** The ordinal comparison `ac.SystemRole < minimum` is semantically broken once `RoleAuditor` joins the enum: an auditor (value 4) is not "more privileged than system admin" (value 3), but the ordinal check would treat it that way. Therefore **before** `RoleAuditor` is added, every ordinal comparison is replaced with explicit capability functions:
  - `RequireSystemAdmin(audit)` — system admin only.
  - `RequireCaseAdmin(audit)` — case admin OR system admin (two explicit cases, not `>= RoleCaseAdmin`).
  - `RequireAuthenticatedWrite(audit)` — rejects `RoleAuditor`; accepts `RoleAPIService`, `RoleUser`, `RoleCaseAdmin`, `RoleSystemAdmin`.
  - `RequireAuthenticatedRead(audit)` — accepts all five roles.
- The refactor touches **22 call sites of `RequireSystemRole` across 13 files** (confirmed by grep) plus 10 direct `ac.SystemRole <` comparisons across 7 files (confirmed). Each is mapped mechanically to the appropriate capability function. The grep-based CI rule forbids any remaining `ac.SystemRole <` or `ac.SystemRole >` patterns.
- **No DB migration is needed for this step.** `SystemRole` is not persisted in a PG table — it is resolved from Keycloak JWT claims via `ParseSystemRole`. The plan's earlier migration 024 targeted a `user_system_roles` table that does not exist anywhere in the schema (verified by grep across all migrations 001–019). Migration 024 is **removed** from this sprint entirely.
- A preset role template concept: `Investigator`, `Reviewer / Prosecutor`, `Legal Counsel (Defence)`, `Auditor`. Each preset is a combination of a system role (`user` or `auditor`) and a default case role assigned when the user is added to a case.
- A UI flow in the user invite dialog that shows four preset cards and applies them in one click. The raw permission matrix remains accessible via a "Customize" link.
- Keycloak realm export updated with the four preset role names as realm roles so Keycloak assignments stay in sync with the application state.
- A `make keycloak-seed` target (new) that applies the realm import on fresh deployments via `kcadm.sh`.

**No DB migration.** Role state is carried in Keycloak JWT claims and resolved in-process by `ParseSystemRole`. There is no `user_system_roles` table in VaultKeeper's schema (verified by grep across all migrations). Sprint 11.5 skips migration number 024; the migration sequence is 020, 021, 022, 023, 024 (→ formerly 025, compliance_reports), 025 (→ formerly 026, system_settings). The renumbering is reflected in the Migrations Summary table below.

**Go changes:**

`internal/auth/context.go` updates (minimal diff — **no reordering of existing values**):
```go
type SystemRole int

const (
    RoleAPIService SystemRole = iota  // 0 — unchanged
    RoleUser                           // 1 — unchanged
    RoleCaseAdmin                      // 2 — unchanged
    RoleSystemAdmin                    // 3 — unchanged
    RoleAuditor                        // 4 — NEW, appended at end
)

func (r SystemRole) String() string {
    switch r {
    case RoleAPIService:  return "api_service"
    case RoleUser:        return "user"
    case RoleCaseAdmin:   return "case_admin"
    case RoleSystemAdmin: return "system_admin"
    case RoleAuditor:     return "auditor"   // NEW
    }
    return "unknown"
}

func ParseSystemRole(s string) (SystemRole, bool) {
    switch s {
    case "api_service":  return RoleAPIService, true
    case "user":         return RoleUser, true
    case "case_admin":   return RoleCaseAdmin, true
    case "system_admin": return RoleSystemAdmin, true
    case "auditor":      return RoleAuditor, true   // NEW
    }
    return 0, false
}

// IsReadOnly returns true for roles that must not mutate state.
// Auditor is the only read-only system role.
func (r SystemRole) IsReadOnly() bool {
    return r == RoleAuditor
}
```

`internal/auth/permissions.go` rewrites:
```go
// The previous ordinal comparison `ac.SystemRole < minimum` is replaced
// with explicit capability checks. This is safer than enum arithmetic
// because RoleAuditor now breaks the strict ordering assumption (an
// auditor is NOT "less than" a user — it has different capabilities).

func RequireSystemAdmin(audit AuditLogger) func(http.Handler) http.Handler {
    return requireCapability(capSystemAdmin, audit)
}

func RequireCaseAdmin(audit AuditLogger) func(http.Handler) http.Handler {
    return requireCapability(capCaseAdmin, audit)
}

func RequireAuthenticatedWrite(audit AuditLogger) func(http.Handler) http.Handler {
    // Rejects auditor; accepts user, case_admin, system_admin.
    return requireCapability(capWrite, audit)
}

func RequireAuthenticatedRead(audit AuditLogger) func(http.Handler) http.Handler {
    // Accepts all roles including auditor.
    return requireCapability(capRead, audit)
}
```

Every call site that uses `RequireSystemRole(RoleUser, ...)` for generic-authenticated-user gating is migrated to `RequireAuthenticatedRead` or `RequireAuthenticatedWrite` depending on whether the endpoint is read or write. This is a **mechanical but widespread change**. A linter rule is added to catch any remaining ordinal comparisons.

**Preset definitions (`internal/auth/presets.go`, new):**
```go
type RolePreset struct {
    ID              string
    DisplayNameKey  string // i18n key
    DescriptionKey  string // i18n key
    SystemRole      SystemRole
    DefaultCaseRole CaseRole
    Icon            string // named icon for UI
}

var Presets = []RolePreset{
    {
        ID:              "investigator",
        DisplayNameKey:  "preset.investigator.name",
        DescriptionKey:  "preset.investigator.description",
        SystemRole:      RoleUser,
        DefaultCaseRole: CaseRoleInvestigator,
        Icon:            "search",
    },
    {
        ID:              "reviewer",
        DisplayNameKey:  "preset.reviewer.name",
        DescriptionKey:  "preset.reviewer.description",
        SystemRole:      RoleUser,
        DefaultCaseRole: CaseRoleProsecutor,
        Icon:            "scale",
    },
    {
        ID:              "defence",
        DisplayNameKey:  "preset.defence.name",
        DescriptionKey:  "preset.defence.description",
        SystemRole:      RoleUser,
        DefaultCaseRole: CaseRoleDefence,
        Icon:            "shield",
    },
    {
        ID:              "auditor",
        DisplayNameKey:  "preset.auditor.name",
        DescriptionKey:  "preset.auditor.description",
        SystemRole:      RoleAuditor,
        DefaultCaseRole: CaseRoleObserver, // read-only case role
        Icon:            "eye",
    },
}
```

**Keycloak realm export updates:**
- `keycloak/realm-export.json` gains four realm roles matching the preset IDs.
- Each realm role has client role mappings that match the application's `SystemRole` values.
- A new `make keycloak-seed` target applies the import via `kcadm.sh` in a one-shot container, idempotent on reruns.

**Frontend changes:**
- `web/src/components/auth/user-invite-dialog.tsx` (new — or modification of an existing component if one exists) shows four preset cards with icon, title, description, and "Customize permissions" link.
- Advanced permission matrix component moves behind the customize link; it still exists for power users.
- Translations added for preset names, descriptions, tooltips, and customize link in `en.json` and `fr.json`.

**Tests:**
- Unit: `SystemRole.IsReadOnly` returns true only for `RoleAuditor`.
- Unit: `requireCapability(capWrite)` rejects auditor, accepts user/case_admin/system_admin.
- Unit: `requireCapability(capRead)` accepts all roles.
- Unit: ordinal comparison `ac.SystemRole < minimum` no longer appears in the codebase (enforced by a `go vet` custom check or grep test in CI).
- Integration: auditor logs in via Keycloak, navigates to case, evidence, custody log, audit dashboard → all return 200. Attempts to POST to any mutation endpoint → all return 403 with message `read_only_role`.
- Integration: inviting a user via the `investigator` preset assigns the correct Keycloak realm role and the correct case role.
- Integration: `make keycloak-seed` is idempotent — running twice does not duplicate roles.
- E2E: admin opens invite dialog, clicks Auditor preset, submits; new user logs in; attempts to upload evidence; sees "Auditor accounts are read-only" error.

**Careful note on witness identity visibility:**
The Auditor role has read breadth across cases it is a member of — but witness identity encryption (Sprint 7) is gated by a separate Witness Protection role and is NOT unlocked by Auditor. Auditors see witness pseudonyms only. This is called out explicitly in the preset description and tested.

### Step 6: Standalone `vaultkeeper-verify` CLI

**What exists today:**
- `internal/integrity/handler.go` runs server-side verification of stored evidence against declared hashes and TSA tokens.
- No standalone CLI exists.
- No `tools/` directory in the repo.
- Sprint 6 defined an export bundle format (ZIP) — implementation details of the current export format need to be confirmed at implementation time and matched exactly by the verifier.

**What is net-new:**
- A new Go module at `tools/vaultkeeper-verify/` with its own `go.mod`, independent of the main `go.mod` so the binary does NOT pull in `go-chi`, Keycloak client, MinIO SDK, or Postgres drivers. Dependencies are narrowly scoped: stdlib + `google/uuid` + `digitorus/timestamp` + `digitorus/pkcs7` + `pkg/custodyhash`.
- A single-file binary buildable with `go build -trimpath -buildvcs=false -o vaultkeeper-verify ./`.
- Reproducible cross-platform builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64.
- Two output modes: human-readable text report, and machine-readable JSON.
- Exit codes: 0 verified, 1 integrity failure, 2 warning-only, 3 IO/usage error.
- CLI flags: `--json`, `--strict`, `--offline` (default), `--trust-store <path>`, `--extract-to <path>`. See detail below.
- ZIP-slip and decompression-bomb protections: reject bundles with `..`, absolute paths, symlinks, or total uncompressed size > 50 GB.
- Release binaries signed with `cosign` and published with a transparency-log entry. Release workflow documents the public key and the verification procedure.
- AGPL-3.0 license file in the subdirectory, matching the main repo.

**Bundle format verification:**
Before implementation, Step 6 requires reading the current export implementation (likely in `internal/cases/export.go` — confirmed present by earlier grep). The verifier's understanding of the bundle format MUST match what the exporter produces. If the current format lacks fields the verifier needs (e.g., a Merkle root, per-item inclusion proofs, a detached manifest signature), Step 6 includes updating the exporter to add them and runs an integration test that exports-then-verifies in a single flow. This may expand the scope of Step 6 significantly; the risk is flagged in the risks section.

**Verification algorithm:**

1. Open the ZIP, validate structure (expected top-level entries: `manifest.json`, optional `manifest.sig`, `custody/chain.jsonl`, `evidence/`, optional `merkle.json`).
2. Parse `manifest.json` against a strict schema. Reject unknown fields only with `--strict`.
3. For each evidence item in the manifest:
   a. Open the item's file in `evidence/<id>/`.
   b. Compute SHA-256.
   c. Compare against `metadata.json.sha256_hash` and `sha256` sidecar (if present). Error on mismatch.
   d. If `tsa.tsr` exists, parse as RFC 3161 (`crypto/x509`, `encoding/asn1`), verify the TSA signature against the embedded trust store, verify the hash bound inside the token matches the computed hash.
4. Replay the custody chain from `custody/chain.jsonl`:
   - Starting `previous_hash = ""` (convention from migration 001).
   - For each entry, compute `H(previous_hash || action || actor || detail || timestamp)` using the exact same normalization the server uses. (The server's hash computation must be published in a reference implementation that the CLI imports; we'll factor it into a shared, zero-dependency package.)
   - Compare with the recorded `hash_value`. Error on mismatch.
5. If `merkle.json` exists, reconstruct the Merkle tree from leaf hashes and compare roots. Error on mismatch.
6. If `manifest.sig` exists, verify the Ed25519 signature over `manifest.json` using the public key referenced in the manifest. Error on invalid signature.
7. Print report.

**Design non-goal:** the verifier does NOT contact the VaultKeeper server, CRL endpoints, OCSP, or any network service. The trust store is embedded at build time. An `--online` flag could fetch CRLs in a future release; Sprint 11.5 explicitly ships with `--offline` behavior only.

**Module structure:**
```
tools/vaultkeeper-verify/
├── go.mod              # tiny, only stdlib + one ASN.1/TSR parser if needed
├── go.sum
├── LICENSE             # AGPL-3.0
├── README.md           # build + usage
├── main.go
├── verifier.go
├── bundle.go
├── chain.go
├── merkle.go
├── tsa.go
├── report.go
├── trust_store.go      # embedded TSA certs
└── testdata/
    ├── happy_path_bundle.zip
    ├── tampered_file_bundle.zip
    ├── tampered_chain_bundle.zip
    ├── invalid_tsa_bundle.zip
    └── missing_file_bundle.zip
```

**Shared hash-computation package — module structure resolved.**

The earlier draft said `pkg/custodyhash/` should be "stdlib only" and importable by a separate-module CLI verifier. Both claims required clarification:

1. **Existing `internal/custody` hash logic uses `github.com/google/uuid`** for formatting the actor/evidence/case UUIDs in the canonical hash input. It is NOT stdlib-only.
2. **Separate Go modules cannot cleanly import packages from their parent module.** Options are `replace` directives (breaks reproducible binary downloads), nested modules with publish-and-pin discipline, or vendoring.

**Resolution:**

- `pkg/custodyhash/` is extracted as a **nested Go module** with its own `go.mod` at `pkg/custodyhash/go.mod`. The main VaultKeeper module imports it via a `replace github.com/vaultkeeper/vaultkeeper/pkg/custodyhash => ./pkg/custodyhash` directive during development and a pinned version tag in releases.
- The CLI verifier's `tools/vaultkeeper-verify/go.mod` imports `pkg/custodyhash` via its own `replace` directive pointing at the relative path. Releases of the CLI pin a specific `pkg/custodyhash` commit via a semver tag.
- The CLI takes **three** third-party dependencies (the "stdlib-only" claim in the earlier draft was wrong — it is corrected here):
  1. `github.com/google/uuid` (~12 KB object code) — needed for UUID formatting in the custody chain.
  2. `github.com/digitorus/timestamp` — **already present in the main `go.mod`** (confirmed by reading it). Needed for RFC 3161 TSR parsing. Stdlib's `crypto/x509` + `encoding/asn1` do not provide `TimeStampToken` / `TSTInfo` structures.
  3. `github.com/digitorus/pkcs7` (transitive through `digitorus/timestamp`) for the PKCS#7 envelope of the TSA response.
- Binary size budget: ~15 MB compressed is conservative. Rough measurement against a stripped `go build -trimpath -buildvcs=false`: stdlib zip/sha256/ed25519/html-template ~8 MB; `digitorus/timestamp` + `digitorus/pkcs7` + `google/uuid` ~150 KB additional code. Total comfortably under 15 MB.
- **Parity guarantee:** `pkg/custodyhash` must match the existing `internal/custody` hash computation **byte-for-byte on a golden corpus of 1000 fixture inputs**. A parity test (`TestCustodyHashParity`) runs on every PR comparing the two implementations against a committed corpus in `internal/custody/testdata/golden_hashes.json`. Any change to either path that alters an output triggers CI failure, forcing the dev to update both in lockstep. This prevents silent drift after the initial extraction.
- **String-based API.** `pkg/custodyhash` accepts `string` parameters (hex IDs) rather than `uuid.UUID` types. This keeps the public surface minimal and documents exactly what is serialized into the hash input. Callers stringify before invoking. The current `internal/custody` code is refactored to call the new package with `.String()`-ified UUIDs.

**Tests:**
- Unit: each verification step against known-good and known-bad test fixtures.
- Fuzz test: `FuzzVerifyBundle` runs for 60 seconds in CI with 10k random mutations of a base bundle. Every mutation must either verify successfully (no harm done) or fail gracefully. No crashes. No infinite loops.
- Cross-platform build: CI matrix builds for all five target platforms; binaries are checksummed and uploaded as release artifacts.
- Reproducibility: two CI runners build the binary; their outputs are byte-identical.
- Size budget: CI enforces binary size < 15 MB (compressed). Fails build if exceeded.
- Network isolation: CI runs the verifier under `unshare -n` (or equivalent) to prove no network syscalls happen. Any attempted `connect()` causes test failure.
- Integration: server-side export of a populated test case → run verifier on the resulting ZIP → exit 0 with clean report.
- Integration: tamper tests — flip one byte in a file, edit one custody entry, corrupt one TSA token, delete one file. Each mutation produces exit 1 with a clear error pointing at the exact item.

### Step 7: Berkeley Protocol Compliance Report Generator

**What exists today:**
- No `internal/compliance/` package.
- No Berkeley Protocol mapping.
- `internal/reports/` exists (confirmed earlier) but its scope and capabilities are to be read at implementation time.
- Sprint 11's timeline export produces PDFs; this may establish a PDF toolchain we can reuse.

**What is net-new:**
- A new `internal/compliance/` package with:
  - `berkeley.go` — report generation service.
  - `berkeley_protocol_mapping.yaml` — versioned clause-to-control mapping. **Embedded at compile time via `//go:embed`.** The YAML is NOT read from disk at runtime — doing so would allow an insider with filesystem write access to modify SQL queries between startup validation and query execution (SQL injection via YAML). Embedding it in the binary means the YAML is as tamper-proof as the binary itself (signed via cosign in Step 6).
  - `templates/berkeley_report.html` — HTML template also embedded via `//go:embed`.
- A new endpoint `POST /api/cases/:id/compliance/berkeley` that generates a PDF and returns a report ID. **Gated by `RequireSystemAdmin` explicitly** — not merely "non-admin → 403." Auditors cannot trigger report generation because the report's personnel-log section surfaces audit-log data that an auditor could otherwise only see via the dashboard (closing the elevation-of-read-privilege gap flagged in the security review).
- A new endpoint `GET /api/cases/:id/compliance/reports/:reportID.pdf` that downloads the PDF. Gated by `RequireAuthenticatedRead` (any role that can see the case can see its reports, including auditors).
- A new case settings tab `CaseComplianceTab` with the generate button, language selector, and report history. The Generate button is disabled (with tooltip) for any user who is not system admin.
- Custody log entries on every report generation (`action: "compliance_report_generated"`).
- Prominent caveats in the report and in the UI about self-assessment vs. third-party certification.
- **Rate limit** at Caddy: 5 report generations per user per hour. Report generation is heavy (1000 items under 60 s) and trivially DoS'd otherwise.
- **Witness-linked audit entries excluded from the personnel log section.** The SQL query that populates the personnel log chapter filters out any custody_log row whose `evidence_id` refers to an evidence item linked (directly or transitively via disclosure records) to a witness with `protection_status IN ('protected', 'high_risk')`. Auditors who do get authorized report copies must not learn access patterns for protected witnesses through correlation.
- **Compliance reports are append-only.** The previous draft treated `compliance_reports` as a regular table subject to retention-based pruning. On review, the stored PDFs are themselves legal records: once a report is generated for a given case state, that specific PDF must remain discoverable. The `compliance_reports` table is added to the append-only registry in migration 021. Reports that become outdated due to case-state changes are not deleted — they gain an `invalidated_at TIMESTAMPTZ` and `invalidated_reason TEXT` column via a separate append-only `compliance_report_invalidations` table.

**Report structure:**
1. **Cover page** — case title, investigator, report generation timestamp, VaultKeeper version, mapping version.
2. **Executive summary** — item count, custody events, integrity checks, disclosure count, hold status, compliance pass/partial/na rollup.
3. **Chapter-by-chapter mapping** — one page per chapter with clause language, VaultKeeper's handling, case-specific data points, and an indicator per clause.
4. **Chain of custody summary** — table of first/last events per item, chain root hash (matches the Step 6 verifier's computation).
5. **Integrity verification results** — most recent full verification, warnings.
6. **Personnel log** — who accessed what (auditor-friendly slice of the audit log).
7. **Caveats page** — "This report is an automated self-assessment… not a certification."
8. **Signed attestation page** — the admin who generated the report signs off; PDF is sealed with a hash of its own contents (excluding that field) displayed prominently.

**Mapping file design:**
```yaml
# internal/compliance/berkeley_protocol_mapping.yaml
version: "2022-rev1"
source: "https://www.ohchr.org/en/publications/policy-and-methodological-publications/berkeley-protocol-digital-open-source"
chapters:
  - id: principles
    title_i18n: "compliance.berkeley.chapter.principles"
    clauses:
      - id: "2.1.1"
        title_i18n: "compliance.berkeley.clause.do_no_harm"
        system_controls:
          - witness_encryption_at_rest
          - classification_enforcement
          - legal_hold_storage_layer
        case_data_points:
          - name: witness_count
            query: "SELECT count(*) FROM witnesses WHERE case_id = $1"
          - name: restricted_item_count
            query: "SELECT count(*) FROM evidence_items WHERE case_id = $1 AND classification = 'restricted'"
        eligible_indicators:
          - value: pass
            condition: "witness_encryption_verified AND no_plaintext_witness_exports"
          - value: partial
            condition: "witness_encryption_verified AND has_plaintext_witness_exports"
          - value: fail
            condition: "not witness_encryption_verified"
  # ... further chapters ...
```

Queries are parameterized (`$1` is the case ID). The mapping file is validated at compile time (via a Go test that parses it) AND at package init (via `//go:embed`). A malformed YAML file fails the build, not the runtime. **There is no path for an attacker to modify the embedded YAML at runtime.**

**Condition evaluation is over a curated named-flag set, not free-form expressions.**

`eligible_indicators[].condition` strings are tokenized into a flat list of known boolean flags connected with `AND`/`OR`/`NOT`. The evaluator rejects any token not in a hard-coded allowlist (e.g., `witness_encryption_verified`, `no_plaintext_witness_exports`, `custody_chain_unbroken`, `rfc3161_all_valid`). Adding a new flag requires a Go code change — flags cannot be introduced by editing the YAML alone. This prevents an attacker (even one who could modify the YAML, which we've already blocked via `go:embed`) from causing arbitrary side effects via the condition evaluator.

**Generation pipeline:**
1. Service reads the mapping YAML (cached).
2. For each clause, executes the parameterized queries to collect data points.
3. Evaluates each indicator condition against the data points.
4. Renders each chapter into a Go `html/template` → HTML → PDF.
5. PDF is hashed (SHA-256 of its own bytes, excluding the "this document hash" field which is computed and then stamped by editing a placeholder).
6. PDF is stored in MinIO under `compliance-reports/<case_id>/<timestamp>_<lang>.pdf`.
7. `custody_log` entry is written with `action: "compliance_report_generated"`, detail containing report ID, generator, mapping version.
8. Report metadata row inserted into a new `compliance_reports` table (see migration below).

**Migration 024: `024_compliance_reports.up.sql`** (renumbered from 025 because the phantom auditor-role migration is removed)
```sql
BEGIN;

CREATE TABLE compliance_reports (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id           UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    report_type       TEXT NOT NULL CHECK (report_type IN ('berkeley_protocol')),
    mapping_version   TEXT NOT NULL,
    mapping_snapshot  JSONB NOT NULL,   -- full mapping contents at generation time
    language          TEXT NOT NULL CHECK (language IN ('en', 'fr')),
    storage_key       TEXT NOT NULL,
    content_hash      TEXT NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    generated_by      UUID NOT NULL,
    generated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_compliance_reports_case_time
    ON compliance_reports(case_id, generated_at DESC);

-- Append-only: each row is a discrete legal record. Invalidation is
-- recorded in a companion table, not by UPDATE or DELETE.
CREATE TABLE compliance_report_invalidations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id       UUID NOT NULL REFERENCES compliance_reports(id) ON DELETE RESTRICT,
    invalidated_by  UUID NOT NULL,
    reason          TEXT NOT NULL,
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_compliance_invalidations_report
    ON compliance_report_invalidations(report_id);

-- Both tables are append-only; enforcement added in migration 021.

COMMIT;
```

`compliance_reports` is append-only. A UI "invalidate report" action inserts into `compliance_report_invalidations` — it does not delete or update the original row. The stored PDF in MinIO is retained indefinitely (pilot users need to be able to prove which assessment was presented to regulators at a point in time).

The `mapping_snapshot JSONB` column carries the full YAML content-as-JSON at generation time. This makes reports reproducible even if the embedded mapping is later updated — auditors can see exactly which clauses and queries were evaluated for this particular report.

**PDF rendering toolchain:**
Defer to Sprint 11's timeline export choice. If Sprint 11 uses `wkhtmltopdf` (simple, battle-tested, air-gap friendly) we reuse it. If Sprint 11 uses headless Chromium (larger footprint, better CSS), we reuse that. Step 7 does NOT introduce a new PDF toolchain.

**Frontend:**
- `web/src/components/cases/case-compliance-tab.tsx` (new) — new tab in case settings showing:
  - Generate button (disabled for non-admins, with tooltip).
  - Language selector.
  - Recent reports list with download links.
  - Warning banner above the generate button with caveats text.
- i18n strings for all UI labels, clause titles, chapter titles, and caveats text in `en.json` and `fr.json`. French is shipped in Step 7 (not deferred to Sprint 12) because a Berkeley Protocol report for a francophone institution must be in French from day one.

**Legal review:**
The caveats text is drafted, shown to project counsel (or a competent legal advisor), and finalized before release. The mapping file's clause language is quoted from the published Berkeley Protocol with attribution in a dedicated references page of the report. Attribution and fair-use claims are reviewed at the same time.

**Tests:**
- Unit: mapping YAML schema validation — valid file parses, invalid file fails startup.
- Unit: data-point queries against a fixture case return expected counts.
- Unit: indicator condition evaluation for each eligible outcome.
- Unit: report HTML template renders without errors for empty case, populated case, case with integrity warnings.
- Integration: generate report on a populated test case → PDF produced, stored in MinIO, row in `compliance_reports`, custody event recorded.
- Integration: PDF is deterministic (byte-identical) for identical case state except for the generation timestamp field.
- Integration: French report generated → all user-visible strings in French (verified by parsing the PDF and checking for English sentinel words).
- Integration: non-admin attempts generation → 403 with `insufficient_privilege`.
- Integration: mapping version mismatch — load an older version of the YAML, regenerate, verify the report's mapping_version field matches.
- E2E: case settings → compliance tab → generate → download → PDF opens, has expected structure.

### Step 8: Air-Gap Mode

**What exists today:**
- No `internal/airgap/` package.
- No `AIR_GAP` config flag in `internal/config/config.go`.
- Sprint 13 mentions "zero outbound" for Whisper but Whisper is not yet integrated.
- HTTP clients in various packages (TSA, SMTP for notifications, Meilisearch client, etc.) are constructed ad-hoc.

**What is net-new:**
- A new `internal/airgap/` package containing:
  - `guard.go` — an `http.RoundTripper` that blocks requests to non-allowlisted hosts.
  - `startup.go` — startup canary that refuses to start if public DNS resolves.
  - `config.go` — config validation that cross-checks other config values when `AIR_GAP=true`.
- A new `VAULTKEEPER_AIR_GAP` env var, parsed in `internal/config/config.go`.
- An HTTP client factory that all HTTP clients in the codebase must use.
- A `go vet` custom check (or a grep-based CI check) that forbids raw `http.Client{...}` literals outside the factory package.
- New Caddyfile snippet and compose override for infrastructure-level egress blocking.
- New `docs/air-gap.md` operator guide.

**Threat model this closes:**
- Deployment has no internet and the application must not hang on external calls.
- Deployment has internet but policy forbids outbound for data sovereignty.
- A dependency silently starts phoning home after an update.
- Telemetry accidentally left on.
- Misconfiguration where an admin thinks air-gap is on but it isn't.

**Enforcement layers:**

1. **Config layer (`internal/config/config.go`):**
```go
type Config struct {
    // ... existing fields ...
    AirGap             bool     `env:"VAULTKEEPER_AIR_GAP" default:"false"`
    AirGapAllowedHosts []string `env:"VAULTKEEPER_AIR_GAP_ALLOWED_HOSTS" separator:","`
    AirGapAllowedCIDRs []string `env:"VAULTKEEPER_AIR_GAP_ALLOWED_CIDRS" separator:","`
    AirGapStrict       bool     `env:"VAULTKEEPER_AIR_GAP_STRICT" default:"true"`
}

// Validate cross-checks air-gap mode against EVERY configured outbound
// endpoint, not just TSA and SMTP. This is the only chokepoint that
// catches infrastructure misconfiguration before the process starts.
func (c Config) Validate() error {
    if !c.AirGap {
        return nil
    }
    if c.ACME.Enabled {
        return errors.New("air-gap: ACME must be disabled (VAULTKEEPER_ACME_ENABLED=false)")
    }
    // Every host we will dial from this process, enumerated explicitly.
    hostsToCheck := []struct{ label, host string }{
        {"TSA",         c.TSA.URL},
        {"SMTP",        c.SMTP.Host},
        {"Postgres",    c.Database.Host},
        {"MinIO",       c.Storage.Endpoint},
        {"Meilisearch", c.Search.URL},
        {"Keycloak",    c.Auth.KeycloakURL},
    }
    for _, h := range hostsToCheck {
        if h.host == "" {
            continue // feature not configured
        }
        if !isAllowlisted(h.host, c.AirGapAllowedHosts, c.AirGapAllowedCIDRs) {
            return fmt.Errorf("air-gap: %s host %q is not on the allowlist", h.label, h.host)
        }
    }
    return nil
}
```

Every field in this list must exist in the actual config struct. If a new outbound endpoint is added in a future sprint, its host check MUST be added here — enforced by a CI check that greps for hosts in config struct vs. the hostsToCheck list. Missing a host means the process will still start in air-gap mode while one component quietly reaches the internet. This CI check is as critical as the append-only registry check in Step 2.

2. **HTTP client factory (`internal/httpfactory/factory.go`, new):**
```go
// NewClient returns an *http.Client whose Transport respects the air-gap
// mode. ALL HTTP clients in the codebase must be constructed through this
// factory. A CI lint rule enforces this.
func NewClient(cfg airgap.Config, opts ...Option) *http.Client {
    base := http.DefaultTransport.(*http.Transport).Clone()
    var transport http.RoundTripper = base
    if cfg.Enabled {
        transport = airgap.NewGuard(base, cfg.AllowedHosts)
    }
    return &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }
}
```

Every existing ad-hoc `http.Client{}` construction in the codebase is migrated to `httpfactory.NewClient`. A linter rule (`go vet` custom check or ripgrep-based CI check) forbids `http.Client{...}` literals outside `internal/httpfactory/`.

3. **Guard (`internal/airgap/guard.go`) — DNS-rebinding-resistant.**

The earlier draft performed a named-host allowlist check without resolving the hostname to IPs. An attacker who controlled DNS for an allowlisted name could point it at a public IP and bypass the guard entirely. The corrected guard resolves every hostname at check time and verifies **every** resolved IP is on the allowlist.

```go
type Guard struct {
    inner        http.RoundTripper
    allowedCIDRs []*net.IPNet
    resolver     *net.Resolver  // injectable for tests + air-gap DNS override
}

// RoundTrip resolves the request host to one or more IPs via the
// configured resolver and refuses the request unless EVERY resolved IP
// falls within an allowlisted CIDR. Short-TTL DNS caching is deliberately
// disabled — we re-resolve on every request. This defeats DNS rebinding.
func (g *Guard) RoundTrip(req *http.Request) (*http.Response, error) {
    host := req.URL.Hostname()

    // 1. Direct IP literal: check the literal against allowlisted CIDRs.
    //    Covers both IPv4 (127.0.0.1) and IPv6 (::1, fe80::...) literals.
    if ip := net.ParseIP(host); ip != nil {
        if !g.ipInAllowlist(ip) {
            return nil, &ErrAirGapBlocked{Host: host, Reason: "literal_ip_not_allowlisted"}
        }
        return g.inner.RoundTrip(req)
    }

    // 2. Named host: resolve via the configured resolver. We do NOT
    //    consult the allowedHosts map before this — name-based
    //    allowlisting alone is exploitable by DNS rebinding.
    ips, err := g.resolver.LookupIPAddr(req.Context(), host)
    if err != nil {
        return nil, &ErrAirGapBlocked{Host: host, Reason: "resolution_failed: " + err.Error()}
    }
    if len(ips) == 0 {
        return nil, &ErrAirGapBlocked{Host: host, Reason: "no_ips_resolved"}
    }
    for _, addr := range ips {
        if !g.ipInAllowlist(addr.IP) {
            return nil, &ErrAirGapBlocked{
                Host:   host,
                Reason: fmt.Sprintf("resolved_ip_%s_not_allowlisted", addr.IP),
            }
        }
    }
    return g.inner.RoundTrip(req)
}

func (g *Guard) ipInAllowlist(ip net.IP) bool {
    for _, cidr := range g.allowedCIDRs {
        if cidr.Contains(ip) {
            return true
        }
    }
    return false
}
```

**Critical notes:**
- `VAULTKEEPER_AIR_GAP_ALLOWED_HOSTS` (the named-host allowlist from the earlier draft) is **removed**. It was a footgun. Replaced entirely by `VAULTKEEPER_AIR_GAP_ALLOWED_CIDRS`.
- Operators configure their internal DNS to resolve internal hostnames (`keycloak.internal`, etc.) to RFC1918 addresses and add those RFC1918 CIDRs to the allowlist. Any IPv4 or IPv6 address that resolves outside the configured CIDRs — including from DNS rebinding — is refused.
- `net.Resolver` is injected so tests can use a fake resolver that returns specific IP sets. This is necessary to test the rebinding defense.
- IP literal requests (MinIO pods with pod IPs, etc.) are also checked against the CIDR allowlist directly.
- The guard does NOT consult the OS DNS cache — it calls `LookupIPAddr` on every request. This adds latency but is the only way to defeat short-TTL rebinding. A caller that makes many requests to the same internal host can add an application-layer cache if needed; the guard is deliberately conservative.

4. **HTTP client factory re-scoped.** Because several existing HTTP clients in the codebase are constructed against SDKs that accept `http.RoundTripper` rather than `*http.Client` (notably the MinIO Go SDK), the factory exposes both:
```go
// Client returns an *http.Client wrapped for air-gap enforcement.
func Client(cfg airgap.Config, opts ...Option) *http.Client

// Transport returns a raw http.RoundTripper wrapped for air-gap enforcement.
// Used by SDKs that construct their own *http.Client internally but accept
// a custom transport (e.g. minio.Options.Transport).
func Transport(cfg airgap.Config) http.RoundTripper
```

Grep-confirmed migration targets (5 direct + SDK wiring):
- `cmd/migrate/main.go:231` — Client
- `cmd/migrate/main.go:308` — Client
- `internal/auth/jwks.go:51` — Client (JWKS fetch for Keycloak)
- `internal/integrity/tsa.go:33` — Client (RFC 3161 TSA)
- `internal/server/health.go:131` — Client
- `cmd/server/main.go` — MinIO SDK construction uses `minio.Options{Transport: httpfactory.Transport(cfg.AirGap)}`. This is the SDK's supported injection point.
- `internal/search/meilisearch.go` — review at implementation time; if it constructs its own client, wire through factory.

A CI lint rule (ripgrep-based) forbids `http.Client{` literal construction outside `internal/httpfactory/`. Test file exceptions (`*_test.go`) are permitted.

5. **Startup canary (`internal/airgap/startup.go`) — multi-layer check.**

A single DNS-based canary is unreliable: some corporate DNS servers with wildcard records resolve `*.invalid` against RFC 2606, and a hard failure on DNS alone would break legitimate air-gap deployments with quirky DNS. The corrected startup check distinguishes **DNS resolution** from **actual TCP egress** and only refuses to start if the latter succeeds.

```go
// CheckCanary runs a layered egress probe. In strict air-gap mode, it
// refuses startup only if we can actually reach a public endpoint by TCP
// — DNS resolution alone is a warning, not a blocker.
func CheckCanary(ctx context.Context, cfg Config) error {
    if !cfg.Enabled || !cfg.Strict {
        return nil
    }

    // Layer 1: try to open a TCP connection to a known public IP.
    // This bypasses DNS entirely. If it succeeds, we have true egress.
    // 1.1.1.1:80 is Cloudflare's DNS resolver — a stable public IP.
    // The short timeout ensures this check doesn't delay startup on
    // legitimately air-gapped hosts.
    probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    conn, err := (&net.Dialer{}).DialContext(probeCtx, "tcp", "1.1.1.1:80")
    if err == nil {
        _ = conn.Close()
        return errors.New(
            "air-gap strict mode: TCP egress to 1.1.1.1:80 succeeded — " +
            "host has external connectivity. Disable air-gap or enforce egress " +
            "rules at the network layer (iptables, netns, k8s NetworkPolicy).")
    }

    // Layer 2: DNS canary. This is a warning, not a failure — some
    // split-horizon DNS setups resolve `.invalid` names despite RFC 2606.
    // A resolved canary is logged as WARN so the operator can investigate
    // their DNS config, but startup continues.
    resolver := net.DefaultResolver
    _, dnsErr := resolver.LookupHost(ctx, "airgap-canary.vaultkeeper.invalid")
    if dnsErr == nil {
        slog.Warn("air-gap: canary .invalid hostname resolved — check split-horizon DNS")
    }

    return nil
}
```

The operator handbook's air-gap section documents both layers and gives iptables/nftables/k8s NetworkPolicy snippets for enforcing egress denial at the OS layer. Relying solely on the Go guard is not enough — the PDF renderer subprocess and PG/MinIO native TCP connections are outside the guard's reach. **Egress must be enforced at the OS layer in production air-gap deployments.**

5. **Caddy layer (deployment):**
New `Caddyfile.airgap` snippet that is imported from the main `Caddyfile` when `AIR_GAP=true`. The snippet does not add dial rules (Caddy is an ingress proxy; egress is controlled at the Docker network layer or systemd layer).

6. **Docker Compose override (`docker-compose.airgap.yml`, new):**
```yaml
services:
  api:
    environment:
      VAULTKEEPER_AIR_GAP: "true"
      VAULTKEEPER_AIR_GAP_STRICT: "true"
  caddy:
    environment:
      CADDY_AIR_GAP: "true"
  # No 'minio' override — MinIO in air-gap is the same binary, just
  # never reached externally. DNS resolution outside the compose
  # network is blocked at the host or k8s layer per operator docs.
```

7. **Air-gap test in CI:**
A dedicated CI job runs the full test suite inside a Linux network namespace with `iptables -A OUTPUT -j DROP`, permitting only loopback. All tests must pass. Any attempted external call causes a test failure with a clear stack trace.

**Documentation (`docs/air-gap.md`, new):**
- Prerequisites: internal TSA (freetsa recommended), operator-provided TLS cert, internal DNS, internal SMTP (if email notifications wanted).
- Setup walkthrough.
- Backup strategy without cross-region replication.
- Common pitfalls: time sync (TSA verification requires accurate clock), cert trust, internal CA setup.

**Tests:**
- Unit: guard allows allowlisted host, blocks public host, blocks IP literal outside allowlist.
- Unit: `CheckCanary` in strict mode with public DNS resolver returns error.
- Unit: `Config.Validate` rejects `AIR_GAP=true + ACME=true`.
- Unit: `Config.Validate` rejects external SMTP host not on allowlist.
- Integration: reflection walk of every package under `internal/`, every `http.Client` must come from `httpfactory.NewClient` — enforced by test.
- CI: full suite runs in a netns with egress blocked; passes.
- Manual: pull the network cable on a deployed instance, run through the full user journey, confirm nothing hangs or times out.

### Step 9: Operator Handbook, App Bootstrap, and First-Run Wizard

**What exists today:**
- `deploy/server-bootstrap/bootstrap.sh` — host-hardening script (SSH, fail2ban, apt). Runs on a fresh Debian 12 box.
- `docker-compose.yml` at repo root.
- `Caddyfile` at repo root.
- No `docs/` directory.
- No app-level bootstrap script.
- No first-run wizard.
- No deployment SLO measurement.

**What is net-new:**
- A new `docs/` directory containing:
  - `operator-handbook.md` — the full handbook.
  - `operator-handbook.pdf` — built from the markdown via pandoc in CI, attached to releases.
  - `air-gap.md` — the air-gap guide from Step 8.
  - `custody-actions.md` — the catalog of every custody action in the codebase.
  - `berkeley-protocol.md` — mapping reference and caveats.
- A new `scripts/app-bootstrap.sh` — application-level bootstrap that runs AFTER `deploy/server-bootstrap/bootstrap.sh` (or on any host that already has Docker). Generates secrets, prompts for domain, generates Caddyfile from template, runs `docker compose up -d`, runs migrations, runs Keycloak seed, opens browser to first-run wizard URL.
- A new first-run wizard at `web/src/app/[locale]/admin/first-run/page.tsx`.
- A new table `system_settings` (migration — note this is migration 025 and collides with Step 7's compliance_reports; actual numbering is resolved by assigning `025` to compliance_reports and `026` to system_settings, see Migrations summary below).
- A CI job that measures wall-clock time of the quickstart path on a fresh Hetzner CX11.

**Migration numbering correction:** the plan originally listed migrations 020–026 with a phantom 024 (`user_system_roles`). With that phantom removed and the renumbering cascaded, the final sequence is **020–025**: 020 upload_attempts, 021 append_only_extensions, 022 evidence_lifecycle, 023 legal_hold_storage, 024 compliance_reports (previously 025), 025 system_settings (previously 026).

**Migration 025: `025_system_settings.up.sql`**
```sql
BEGIN;

CREATE TABLE system_settings (
    key          TEXT PRIMARY KEY,
    value        JSONB NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   UUID
);

-- Restrict mutation to vaultkeeper_forensic_admin. The application role
-- can read settings but cannot arbitrarily overwrite them. The single
-- exception is the first-run wizard, which uses a SECURITY DEFINER function
-- to perform the completion transaction atomically.
ALTER TABLE system_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE system_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY system_settings_select ON system_settings
    FOR SELECT TO vaultkeeper_app USING (true);
-- No INSERT/UPDATE/DELETE policies for vaultkeeper_app.
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON system_settings FROM vaultkeeper_app;
GRANT SELECT ON system_settings TO vaultkeeper_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON system_settings TO vaultkeeper_forensic_admin;

-- SECURITY DEFINER function owned by vaultkeeper_migrations, used by the
-- first-run wizard handler to perform the completion transaction.
CREATE OR REPLACE FUNCTION complete_first_run(
    admin_user_id UUID,
    supplied_token_hash TEXT
) RETURNS VOID
LANGUAGE plpgsql SECURITY DEFINER SET search_path = public AS $$
DECLARE
    stored_hash TEXT;
    completed BOOLEAN;
BEGIN
    SELECT value->>'hash' INTO stored_hash
      FROM system_settings WHERE key = 'first_run_token_hash';
    SELECT (value)::boolean INTO completed
      FROM system_settings WHERE key = 'first_run_complete';

    IF completed THEN
        RAISE EXCEPTION 'first-run already completed' USING ERRCODE = '42501';
    END IF;
    IF stored_hash IS NULL OR stored_hash <> supplied_token_hash THEN
        RAISE EXCEPTION 'first-run token invalid' USING ERRCODE = '42501';
    END IF;

    UPDATE system_settings SET value = 'true'::jsonb, updated_at = now(),
                                updated_by = admin_user_id
     WHERE key = 'first_run_complete';
    DELETE FROM system_settings WHERE key = 'first_run_token_hash';
END;
$$;
ALTER FUNCTION complete_first_run(UUID, TEXT) OWNER TO vaultkeeper_migrations;
GRANT EXECUTE ON FUNCTION complete_first_run(UUID, TEXT) TO vaultkeeper_app;

-- Seed the first-run state as false. The token hash is inserted by
-- scripts/app-bootstrap.sh at bootstrap time, not in the migration.
INSERT INTO system_settings (key, value) VALUES
    ('first_run_complete', 'false'::jsonb);

COMMIT;
```

With this design, `vaultkeeper_app` cannot flip `first_run_complete` back to false via a direct UPDATE, cannot insert its own token, and cannot bypass the wizard. The only way to complete first-run is through the SECURITY DEFINER function, which validates the token hash.

**App bootstrap script (`scripts/app-bootstrap.sh`, new):**
```bash
#!/usr/bin/env bash
set -euo pipefail
# Application-level bootstrap for VaultKeeper.
# Run AFTER host-level bootstrap (deploy/server-bootstrap/bootstrap.sh).
# Idempotent where practical.
#
# Usage:
#   bash scripts/app-bootstrap.sh [--air-gap]
#
# Prompts for:
#   - Domain (skipped in --air-gap mode)
#   - Admin email (used for Keycloak initial admin)
#   - Time zone
#
# Produces:
#   - .env with generated secrets
#   - Caddyfile populated with domain (or air-gap template)
#   - docker compose stack running
#   - Database migrated
#   - Keycloak seeded with preset roles
#   - Opens https://<domain>/admin/first-run in the operator's browser

# Validation:
#   - Docker available and version >= 24
#   - docker compose v2 available
#   - 20 GB disk free
#   - 4 GB RAM
#   - Ports 80, 443 available (unless --air-gap)
#   - Domain DNS resolves to this host (unless --air-gap)

# ... full script ...
```

**First-run wizard (`web/src/app/[locale]/admin/first-run/page.tsx`, new) — with one-time token.**

The wizard is unauthenticated by necessity (no admin exists yet) but it **must not be reachable by arbitrary attackers**. A bare unauthenticated route means any attacker with network access to the server between service start and wizard completion can create the first system admin and take over the instance. The correct design uses a one-time bootstrap token.

**Bootstrap token lifecycle:**
1. `scripts/app-bootstrap.sh` generates a 256-bit random token, writes it to `/var/lib/vaultkeeper/first-run-token` with mode `0600` owned by root, and prints it to the terminal:
   ```
   ===================================================================
   FIRST-RUN BOOTSTRAP TOKEN
   
   Open the following URL in your browser within the next 60 minutes:
   
     https://vaultkeeper.example/admin/first-run?token=<32-byte-hex>
   
   Do not share this URL. It grants full administrative access to a
   brand-new VaultKeeper instance. Once the wizard completes, the URL
   stops working.
   ===================================================================
   ```
2. The token is also written into the `system_settings` table as a single row with `key = 'first_run_token_hash', value = sha256(token)`. The plaintext is never stored server-side.
3. The wizard's GET and POST handlers both validate the `token` query parameter (or `X-First-Run-Token` header) against the hashed value in the DB. Invalid/missing token → 410 Gone with message "First-run bootstrap is not active or has already completed. If you are provisioning a new instance, run scripts/app-bootstrap.sh again to generate a new token."
4. Token expiry: 60 minutes after generation. An expired token is a 410.
5. On wizard completion, the handler deletes the token row from `system_settings` AND sets `first_run_complete = true` in the same PG transaction. The on-disk `/var/lib/vaultkeeper/first-run-token` file is deleted by the bootstrap completion handler.
6. After completion, `/admin/first-run` returns **410 Gone** (not 404). 410 is semantically correct — the resource existed and is permanently unavailable — and gives monitoring systems an unambiguous signal. It also differentiates "first-run already done" from "typo in URL" for the operator.

**Wizard steps (unchanged from earlier draft except step 1 requires a valid token):**
1. **Token validation** — request must carry a valid unexpired token. Failure → 410.
2. **Welcome** — shows version, project info, link to handbook.
3. **Create initial system admin** — password policy enforced (16+ chars, upper/lower/digit/symbol, not a known-breached password per embedded HIBP k-anonymity top-1M corpus).
4. **Connectivity tests** — pings PG, MinIO, Keycloak, SMTP (or skipped in air-gap), TSA (internal or skipped). Red/green indicators.
5. **Test upload** — operator selects a small file from disk, it goes through the full Step 1 pipeline (client hash → upload → server verify → TSA → custody → `evidence_items` insert). Shows success with both hashes displayed side-by-side.
6. **Test export + verify** — server generates a bundle containing just that one test item, the wizard exercises `vaultkeeper-verify` as a subprocess, and shows the resulting report. Proves the full integrity story end-to-end BEFORE a pilot user ever touches the system. The subprocess receives the bundle path as a command-line argument that is NEVER constructed from user input (the wizard owns the path); no command injection surface.
7. **Mark first-run complete** — atomic PG transaction: insert admin, delete `first_run_token_hash` row, set `first_run_complete = true`. After this, `/admin/first-run` returns 410 Gone permanently.

**Middleware gate (`web/src/middleware.ts`):**
- New unauthenticated endpoint `GET /api/system/first-run-status` returns `{complete: bool}`. This is the only way the Edge-runtime middleware can check DB state.
- When `complete == false`: every route except `/admin/first-run`, `/api/system/first-run/*`, and static assets returns a redirect to `/admin/first-run?token=...` (middleware preserves the token query if present).
- When `complete == true`: `/admin/first-run` returns 410, everything else is normal.
- The middleware is added to the existing `web/src/middleware.ts` (which already handles next-intl locale routing) as a second stage after the locale matcher.

**Rate limit:** token validation endpoint has a strict rate limit of 5 attempts per minute per IP at Caddy. Prevents brute-forcing the token within the 60-minute window.

**Backup set extension (new sub-step in Step 9):**

The current Sprint 6 backup implementation captures the PG database via `pg_dump` and the `evidence` MinIO bucket via an `mc mirror` pass. Sprint 11.5 introduces eight new PG tables and one new MinIO bucket, all of which must be in the backup set:

| Resource | Type | Action required |
|---|---|---|
| `upload_attempts_v1`, `upload_attempt_events` | PG tables | captured by `pg_dump` — **verify in test** |
| `notification_outbox`, `notification_read_events` | PG tables | captured by `pg_dump` — **verify** |
| `evidence_status_transitions` | PG table | captured by `pg_dump` — **verify** |
| `legal_hold_events` | PG table | captured by `pg_dump` — **verify** |
| `trigger_tamper_log` | PG table | captured by `pg_dump` — **verify** |
| `compliance_reports`, `compliance_report_invalidations` | PG tables | captured by `pg_dump` — **verify** |
| `evidence-locked` MinIO bucket | object storage | **must be added explicitly** to the backup script's bucket list |
| `integrity_checks` | PG table (Sprint 11) | verify present in backup |
| `append_only_registry.json` | config file in repo | backed up with code, not data |

Sprint 11.5 adds a new integration test `TestBackupCoverageSprint115` that:
1. Creates a fixture case with at least one row in each of the eight new tables and at least one object in `evidence-locked`.
2. Runs the backup script.
3. Parses the backup artifacts (PG dump + MinIO tarball) and asserts every fixture row AND every bucket object is present.
4. Fails the build if any new table or bucket is missing.

This test also serves as the canonical place for **future** sprints to add coverage checks: any new sensitive table must update `TestBackupCoverage` or CI fails.

**Rollback runbook (`docs/rollback-runbook.md`, new):**

Because legal-hold placements into compliance-mode Object Lock are **irreversible**, a full rollback from v1.9.0-pilot-ready to a prior release is NOT the same as a normal downgrade. The runbook documents four distinct rollback scenarios:

1. **No legal holds placed during the pilot.** Safe rollback: run `migrate down` through 020, restart the prior binary. Objects remain in `evidence` bucket. No data loss.
2. **Legal holds placed, no `PlaceHold` has moved objects yet (all items in `pending` state).** Safe rollback: `migrate down` reverts the state column; the pending migration rows are discarded; objects remain in `evidence`. No data loss.
3. **Legal holds placed, objects moved to `evidence-locked`.** **Partial rollback only.** The `migrate down` reverts DB state but the objects remain in `evidence-locked` for 100 years. The prior binary does not know about `storage_bucket` or `evidence-locked`, so it will fail to find the held evidence and the case will appear broken. Operators in this scenario must either: (a) stay on v1.9.0-pilot-ready and fix forward, or (b) run a custom one-off migration that copies objects back from `evidence-locked` to `evidence` (the copy is permitted because it leaves the locked originals in place; the regular-bucket copies are then deletable per normal rules). The runbook provides the one-off SQL and `mc` commands.
4. **First-run wizard already completed, but revert needed to pre-11.5 binary.** The `system_settings` table and the new `first_run_complete` state are unknown to the prior binary but are harmless (unused columns, unknown table). Safe but produces an orphaned `system_settings` table after downgrade.

The runbook's prominent header warns: **"Rolling back after a legal hold has been placed is not reversible for the held objects. Plan your pilot cutover so that no legal hold is placed until you have confidence in v1.9.0-pilot-ready."**

**Post-restore reconciliation runbook (`docs/post-restore-reconciliation.md`, new):**

When the PG database is restored from a backup, state may diverge from the MinIO layer. The reconciliation script:
1. Queries every `cases.legal_hold = true` row.
2. For each held case, queries its `evidence_items.storage_bucket` values.
3. For any item where `storage_bucket = 'evidence-locked'` but the object does not exist in `evidence-locked` (rare — indicates backup of DB predates the object move), flags the item in a `reconciliation_report.json` and emits a CRITICAL notification.
4. For any item where `storage_bucket = 'evidence'` but the object exists in `evidence-locked` (DB was restored to a state before `PlaceHold` completed, but MinIO has the moved object), resumes the `PlaceHold` state machine for that item.
5. Produces a human-readable summary for operator review.

**Deployment SLO CI job (`.github/workflows/deployment-slo.yml`, new):**
```yaml
name: Deployment SLO
on:
  schedule:
    - cron: '0 4 * * *'   # nightly
jobs:
  measure:
    runs-on: ubuntu-latest
    steps:
      - name: Provision fresh Hetzner CX11
        run: |
          # Terraform provisions a fresh box.
          # SSH in, run deploy/server-bootstrap/bootstrap.sh.
          # Then run scripts/app-bootstrap.sh with test env vars.
          # Then run scripts/first-run-automation.sh (simulates the wizard).
          # Measure wall clock from boot to "first evidence verified."
      - name: Fail if SLO exceeded
        run: |
          if (( $SECONDS > 3600 )); then
            echo "Deployment SLO exceeded: $SECONDS seconds"
            exit 1
          fi
```

This job publishes the measured time to a badge in the README.

**Tests:**
- Unit (bash): `scripts/app-bootstrap.sh --help` works; prereq validation rejects missing Docker with clear error.
- Unit (Go): first-run gate middleware redirects pre-completion, 404s post-completion.
- Integration: run `scripts/app-bootstrap.sh` in a CI container, assert all files produced, all services up, migrations applied.
- Integration: first-run wizard steps 1–5 complete programmatically, `system_settings.first_run_complete` flips to `true`.
- Manual: one human engineer runs through the handbook on a clean VM with a stopwatch, reports the actual time, and compares against the 60-minute SLO. This is the final gate for Sprint 11.5 to be considered "done."

---

## Cross-Cutting Concerns

### Backwards compatibility

- Sprint 11.5 migrations are additive. Evidence uploaded before Sprint 11.5 has no `upload_attempts_v1` row and is grandfathered (verified only by server-side hash, no client-hash receipt). The handbook documents this clearly.
- `review_status` defaults to `ingested` for all pre-existing rows.
- Pre-existing legal holds remain at service-layer enforcement only until a system admin runs the `migrate-legal-hold-to-storage-layer` one-shot tool (new binary in `cmd/migrate-legal-hold/`). The tool iterates existing held cases and calls `PlaceHold` for each. Custody log entries document the migration.
- The `SystemRole` enum change reorders values. The on-disk representation is the string name (`"user"`, `"auditor"`, etc.), not the integer — so no data migration is required, but every ordinal comparison in the code must be replaced. The grep-based CI check enforces this.

### Downgrade / rollback

- Every migration has a `down.sql` that non-destructively removes new structures.
- The append-only event trigger in Step 2 (migration 021) records any attempt to drop the policies during down-migration, so the downgrade itself leaves an audit trail.
- Legal hold migration from service-layer-only to storage-layer is a one-way door for already-moved objects. Compliance-mode retention on objects in `evidence-locked` cannot be lifted for 100 years. This is documented prominently in the handbook and in the migration's SQL comments.

### i18n

- Every user-facing string added in Sprint 11.5 is added to both `web/src/messages/en.json` and `web/src/messages/fr.json` from day one.
- Translation keys follow the existing hierarchy (`lifecycle.status.admitted`, `legal_hold.placed`, `preset.auditor.description`, etc.).
- Operator handbook ships English in Sprint 11.5; French is a Sprint 12 task.
- Berkeley Protocol report ships both English AND French in Sprint 11.5 (Step 7) because francophone institutions need it from day one.

### Custody action catalog (`docs/custody-actions.md`, new)

Every custody action string in the codebase is catalogued with its semantics and payload schema. New actions introduced in Sprint 11.5:

| Action | Introduced in | Payload fields | Notes |
|--------|---------------|----------------|-------|
| `upload_hash_mismatch` | Step 1 | `client_hash`, `server_hash`, `bytes_received`, `user_agent` | Fired when client-declared hash doesn't match server-computed hash |
| `review_status_transition` | Step 3 | `from`, `to`, `reason`, `actor`, `locked` | Every lifecycle state change |
| `review_status_locked` | Step 3 | `reason` | Item locked in current state |
| `review_status_unlocked` | Step 3 | `reason` | Item unlocked |
| `moved_to_locked_bucket` | Step 4 | `source_key`, `destination_key`, `object_size` | Object moved during hold placement |
| `legal_hold_placed` | Step 4 | `reason`, `object_count` | Case-level event |
| `legal_hold_released` | Step 4 | `reason` | Case-level event |
| `compliance_report_generated` | Step 7 | `report_id`, `mapping_version`, `language` | Berkeley Protocol PDF generated |
| `first_run_completed` | Step 9 | `admin_email` | First-run wizard complete |

Existing actions (not changed by this sprint) are also catalogued for completeness. The catalog is the source of truth for the `vaultkeeper-verify` CLI's custody chain replay.

### Performance budgets

- Client-side hashing on a 100 MB file: under 10 s on a 2020-era laptop (hash-wasm benchmark).
- Append-only policy enforcement: under 5% overhead on custody_log INSERT throughput.
- Legal hold placement on a 100-item case: under 2 minutes.
- Berkeley Protocol report generation on a 1000-item case: under 60 seconds.
- `vaultkeeper-verify` against a 100-item bundle: under 10 seconds.
- First-run wizard total time (once infrastructure is up): under 5 minutes.
- Full app-bootstrap from clean VM to "first evidence verified": under 60 minutes (enforced by CI).

### Rate limiting (new cross-cutting section)

Every new write endpoint introduced in Sprint 11.5 gains a Caddy-level rate limit. These are configured in a new `(rate_limits)` snippet in `Caddyfile` (and mirrored in `Caddyfile.airgap`):

| Endpoint | Limit | Scope |
|---|---|---|
| `POST /api/cases/*/evidence` (Upload) | 60/min | per user |
| `POST /api/evidence/*/version` (UploadNewVersion) | 60/min | per user |
| `POST /api/evidence/*/review/transition` | 120/min | per user |
| `POST /api/evidence/*/review/lock` | 30/min | per user |
| `POST /api/cases/*/legal-hold/place` | 5/hour | per user + global 30/hour |
| `POST /api/cases/*/compliance/berkeley` | 5/hour | per user |
| `POST /api/system/first-run/*` | 5/min | per IP |

Existing endpoints (Sprint 5 search, Sprint 10 bulk upload) already have rate limits. Sprint 11.5 does not reduce or remove those.

### Partitioning strategy for append-only tables

Append-only tables grow without bound. For multi-year ICC-scale deployments the largest growth vectors are:
- `custody_log` (every mutation)
- `upload_attempt_events` (every upload, including rejections)
- `auth_audit_log` (every login and access decision)
- `evidence_status_transitions` (every lifecycle change)
- `integrity_checks` (every scheduled verification)

Sprint 11.5 introduces **declarative range partitioning by year on `timestamp` / `at`** for these five tables, using PG 16 native partitioning. Each year becomes a child table. A scheduled cron job creates next year's partition on December 1. A future retention policy (Sprint 9's infrastructure) can detach old partitions without touching the active set — this is why we do this now, even though the pilot case-load fits in a single partition: retrofitting partitioning onto a live 50M-row table requires `pg_partman` or a full table swap, and it is cheaper to do now.

Partitioning is added in migration 021 at the same time as the RLS policies. An integration test creates 3 years of fixture data and asserts that partition pruning kicks in for year-filtered queries.

### Accessibility (new DoD requirement)

Every new UI surface introduced in Sprint 11.5 must meet **WCAG 2.1 AA**:
- First-run wizard (`/admin/first-run`)
- User invite dialog with preset cards
- Evidence review status badge + panel
- Case compliance tab + report history
- Upload error modal (409 hash mismatch)
- Legal hold confirmation dialog

Acceptance criteria: keyboard navigation, visible focus states, ARIA labels on interactive elements, color contrast ≥ 4.5:1 for text, no content conveyed by color alone. A Playwright + axe-core automated check runs on every PR touching these files and fails the build on any AA violation.

### Non-goals (explicit)

The plan explicitly declares the following out-of-scope for Sprint 11.5:
- **End-user training materials.** The handbook is for operators, not investigators. Pilots are expected to train their own users; the plan does not provide training content.
- **Metrics / observability beyond health endpoints.** Prometheus/OpenTelemetry integration is deferred to a future sprint. Operators can scrape the existing health endpoints for basic uptime monitoring. This is a pragmatic trade-off to keep Sprint 11.5 scope bounded.
- **i18n beyond English + French.** Arabic, Spanish, Russian, and other UN languages are deferred. The translation key structure is designed to support them; the actual translations are a Sprint 12+ deliverable.
- **Browser support beyond modern Chromium, Safari, Firefox (current + 1 prior major version).** hash-wasm requires WebAssembly, which is universal in supported browsers. IE11 and older mobile browsers are explicitly unsupported.
- **Zero-telemetry guarantee is already in effect** (VaultKeeper has never shipped telemetry). The operator handbook reiterates this explicitly so that pilot security reviewers can check the box without auditing the source.
- **Compliance frameworks beyond Berkeley Protocol.** ISO 27001, SOC 2, NIST are Sprint 21. HIPAA is not in scope at all.

### Supply-chain provenance

- `hash-wasm` NPM dependency pinned to an exact version, SHA-256 recorded in `web/package-lock.json`.
- All Go dependencies pinned in `go.mod` / `go.sum` (standard practice).
- CLI verifier releases signed with `cosign`; public key published in the repo and the handbook.
- Operator handbook PDF hashed and the SHA-256 published in the release notes.
- SBOM generated for every release using `syft` and attached to the GitHub release.

### Handbook drift prevention

The deployment SLO CI job (Step 9) measures automation only, not the handbook text. To prevent the handbook drifting out of sync with the code:
- A `docs/handbook-test-plan.md` file defines a minimal scripted walkthrough.
- A new CI job `handbook-drift-check.yml` parses the handbook's quickstart section, extracts all command invocations, and executes them in a container. Any exit-code mismatch or missing binary fails the build.
- The full manual walkthrough by a human is still required at release cut, but the command-level drift check runs on every PR that touches `docs/operator-handbook.md`.

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `web/package.json` | Modify | Add hash-wasm dependency |
| `web/src/lib/upload-hasher.ts` | Create | Client-side streaming SHA-256 via hash-wasm |
| `web/src/components/evidence/evidence-uploader.tsx` | Modify | Pre-upload hash phase, receipt panel, 409 modal |
| `internal/evidence/upload_hash.go` | Create | Shared X-Content-SHA256 validator used by both Upload and UploadNewVersion |
| `internal/evidence/handler.go` | Modify | Wire hash validation into BOTH Upload and UploadNewVersion; respondServiceError maps ErrHashMismatch → 409 |
| `internal/evidence/service.go` | Modify | TeeReader for server-side hash; compare against client hash; outbox pattern for compensation |
| `internal/evidence/errors.go` | Modify | Add ErrHashMismatch sentinel |
| `internal/evidence/upload_attempts.go` | Create | Attempt repository |
| `internal/evidence/cleanup/worker.go` | Create | notification_outbox consumer; MinIO delete compensation with exponential backoff |
| `migrations/020_upload_attempts.up.sql` | Create | upload_attempts_v1, upload_attempt_events, notification_outbox |
| `migrations/020_upload_attempts.down.sql` | Create | |
| `migrations/021_append_only_extensions.up.sql` | Create | RLS on 11 tables + trigger_tamper_log + SECURITY DEFINER event triggers + partitioning |
| `migrations/021_append_only_extensions.down.sql` | Create | |
| `migrations/022_evidence_lifecycle.up.sql` | Create | review_status + review_version + evidence_status_transitions |
| `migrations/022_evidence_lifecycle.down.sql` | Create | |
| `migrations/023_legal_hold_storage.up.sql` | Create | storage_bucket + legal_hold_migration_state + legal_hold_events + delete guards |
| `migrations/023_legal_hold_storage.down.sql` | Create | |
| `migrations/024_compliance_reports.up.sql` | Create | compliance_reports + compliance_report_invalidations (append-only) |
| `migrations/024_compliance_reports.down.sql` | Create | |
| `migrations/025_system_settings.up.sql` | Create | system_settings + first_run_complete + SECURITY DEFINER completion function |
| `migrations/025_system_settings.down.sql` | Create | |
| `internal/lifecycle/service.go` | Create | Review lifecycle service |
| `internal/lifecycle/handler.go` | Create | Transition/Lock/Unlock/History endpoints |
| `internal/lifecycle/repository.go` | Create | |
| `internal/legalhold/storage.go` | Create | PlaceHold / ReleaseHold with MinIO Object Lock |
| `internal/legalhold/repository.go` | Create | legal_hold_events repository |
| `internal/app/legal_hold_adapter.go` | Modify | Wire StorageEnforcer into the existing adapter |
| `internal/auth/context.go` | Modify | Append RoleAuditor = 4 (NO reorder, RoleAPIService remains at 0); add IsReadOnly method |
| `internal/auth/permissions.go` | Modify | Replace `ac.SystemRole <` ordinal comparisons with capability checks — 22 RequireSystemRole call sites + 10 direct comparisons across 13 files |
| `internal/auth/capability.go` | Create | New capability check functions (RequireSystemAdmin, RequireCaseAdmin, RequireAuthenticatedWrite, RequireAuthenticatedRead) |
| `internal/auth/presets.go` | Create | Role preset definitions |
| `internal/httpfactory/factory.go` | Create | HTTP client factory |
| `internal/airgap/guard.go` | Create | Egress guard RoundTripper |
| `internal/airgap/startup.go` | Create | DNS canary |
| `internal/airgap/config.go` | Create | Config validation |
| `internal/config/config.go` | Modify | Add AirGap fields |
| `internal/compliance/berkeley.go` | Create | Report generator |
| `internal/compliance/berkeley_protocol_mapping.yaml` | Create | Clause-to-control mapping |
| `internal/compliance/templates/berkeley_report.html` | Create | PDF template |
| `internal/compliance/repository.go` | Create | compliance_reports CRUD |
| `pkg/custodyhash/hash.go` | Create | Shared, zero-dep custody-hash computation |
| `pkg/custodyhash/hash_test.go` | Create | Test vectors + parity with existing internal/custody |
| `internal/custody/hash.go` | Modify | Switch to pkg/custodyhash |
| `tools/vaultkeeper-verify/go.mod` | Create | Standalone module |
| `tools/vaultkeeper-verify/main.go` | Create | CLI entry point |
| `tools/vaultkeeper-verify/verifier.go` | Create | Core verification |
| `tools/vaultkeeper-verify/bundle.go` | Create | ZIP parsing |
| `tools/vaultkeeper-verify/chain.go` | Create | Custody replay |
| `tools/vaultkeeper-verify/merkle.go` | Create | Merkle tree reconstruction |
| `tools/vaultkeeper-verify/tsa.go` | Create | RFC 3161 token verification |
| `tools/vaultkeeper-verify/report.go` | Create | Human and JSON output |
| `tools/vaultkeeper-verify/trust_store.go` | Create | Embedded TSA roots |
| `tools/vaultkeeper-verify/LICENSE` | Create | AGPL-3.0 |
| `tools/vaultkeeper-verify/README.md` | Create | Build + usage |
| `tools/vaultkeeper-verify/testdata/*.zip` | Create | Fixture bundles |
| `cmd/migrate-legal-hold/main.go` | Create | One-shot migration tool for existing held cases |
| `scripts/app-bootstrap.sh` | Create | Application-level bootstrap |
| `scripts/first-run-automation.sh` | Create | CI helper for measuring SLO |
| `scripts/measure-deployment-slo.sh` | Create | Wall-clock measurement |
| `docker-compose.yml` | Modify | Pin MinIO version, add minio-setup service |
| `docker-compose.airgap.yml` | Create | Air-gap override |
| `Caddyfile.airgap` | Create | Air-gap Caddy snippet |
| `keycloak/realm-export.json` | Modify | Add preset realm roles |
| `Makefile` | Modify | Add `keycloak-seed` target |
| `web/src/app/[locale]/admin/first-run/page.tsx` | Create | First-run wizard |
| `web/src/middleware.ts` | Modify | First-run gate |
| `web/src/components/evidence/review-status-badge.tsx` | Create | Status pill |
| `web/src/components/evidence/review-status-panel.tsx` | Create | Detail panel + transitions |
| `web/src/components/cases/case-compliance-tab.tsx` | Create | Berkeley Protocol report UI |
| `web/src/components/auth/user-invite-dialog.tsx` | Create/Modify | Preset picker + customize link |
| `web/src/messages/en.json` | Modify | All new strings |
| `web/src/messages/fr.json` | Modify | All new strings |
| `docs/operator-handbook.md` | Create | Full handbook |
| `docs/air-gap.md` | Create | Air-gap deployment guide (includes host-layer egress enforcement snippets) |
| `docs/custody-actions.md` | Create | Catalog |
| `docs/berkeley-protocol.md` | Create | Mapping reference + caveats |
| `docs/rollback-runbook.md` | Create | Four rollback scenarios with held-object caveat |
| `docs/post-restore-reconciliation.md` | Create | DB-vs-MinIO reconciliation procedure |
| `docs/handbook-test-plan.md` | Create | Command list for the drift-check CI job |
| `append_only_registry.json` | Create | Curated list of append-only tables consumed by CI |
| `.github/workflows/deployment-slo.yml` | Create | Nightly SLO measurement |
| `.github/workflows/vaultkeeper-verify-release.yml` | Create | CLI cosign-signed cross-platform builds on tag |
| `.github/workflows/air-gap-test.yml` | Create | Full suite inside netns egress-drop |
| `.github/workflows/append-only-check.yml` | Create | Registry check for append-only tables |
| `.github/workflows/backup-coverage.yml` | Create | Assert every new table and `evidence-locked` are in the backup set |
| `.github/workflows/handbook-drift-check.yml` | Create | Parse operator handbook and execute commands in a container |
| `.github/workflows/air-gap-config-check.yml` | Create | Verify every config host appears in Validate() |
| `.github/workflows/a11y-check.yml` | Create | axe-core scans on new UI surfaces |
| `.github/workflows/system-role-ordinal-check.yml` | Create | Grep-based lint against `ac.SystemRole <` / `>` patterns |
| `.github/workflows/http-client-lint.yml` | Create | Forbid raw `http.Client{}` outside `internal/httpfactory` |

---

## Migrations summary (final sequence)

| # | File | Description |
|---|------|-------------|
| 020 | `020_upload_attempts.up.sql` | upload_attempts_v1, upload_attempt_events, notification_outbox |
| 021 | `021_append_only_extensions.up.sql` | RLS on 11 tables (including previously-uncovered auth_audit_log, disclosures, integrity_checks) + trigger_tamper_log + DDL event trigger with SECURITY DEFINER + ALTER ROLE event trigger + vaultkeeper_migrations role + notification_read_events + yearly partitioning for append-only tables |
| 022 | `022_evidence_lifecycle.up.sql` | review_status + review_version (monotonic counter) + evidence_status_transitions |
| 023 | `023_legal_hold_storage.up.sql` | storage_bucket + legal_hold_migration_state + ever_held + legal_hold_events (with CHECK on object_count by event_type) + delete guards |
| 024 | `024_compliance_reports.up.sql` | compliance_reports + compliance_report_invalidations (both append-only, with mapping_snapshot JSONB) |
| 025 | `025_system_settings.up.sql` | system_settings KV + first_run_complete seed + SECURITY DEFINER completion function |

**Sprint 11.5 ships six migrations, not seven.** The earlier draft's migration 024 (`auditor_role`) targeted a `user_system_roles` table that does not exist in the schema — it was removed entirely because the auditor role change is resolved via Keycloak realm import and Go enum only, with no PG state change needed.

All migrations reversible. All tested via `migrate up` + `migrate down` + `migrate up` in CI. Rollback semantics for migrations 022 and 023 have caveats documented in `docs/rollback-runbook.md` — specifically, a legal hold that has already moved objects to compliance-mode Object Lock cannot be fully rolled back at the storage layer.

---

## Definition of Done

- [ ] Client-side SHA-256 hashing ships in the evidence uploader with a visible receipt
- [ ] Server validates `X-Content-SHA256` header + `client_sha256` form field and rejects mismatches with 409
- [ ] **Both** `Upload` and `UploadNewVersion` handlers enforce client hash (verified by identical test suites)
- [ ] `upload_attempts_v1`, `upload_attempt_events`, `notification_outbox` record every attempt and every compensation action
- [ ] Mismatch path does NOT inline-delete MinIO objects; deletion happens via `notification_outbox` + cleanup worker
- [ ] 11 tables are append-only at the DB layer (custody_log existing, plus: upload_attempts_v1, upload_attempt_events, notification_read_events, evidence_status_transitions, legal_hold_events, trigger_tamper_log, auth_audit_log, disclosures, integrity_checks, compliance_reports, compliance_report_invalidations)
- [ ] `notification_outbox` is semi-append-only: worker can update attempt_count/completed_at/dead_letter_at but NOT action/payload/created_at
- [ ] Previously-uncovered `auth_audit_log` and `disclosures` gain RLS in this sprint
- [ ] Trigger-tampering DDL leaves entries in `trigger_tamper_log` via SECURITY DEFINER event trigger
- [ ] `ALTER ROLE` events (including grants to `vaultkeeper_forensic_admin`) are captured by the separate `ddl_command_start` role-modification trigger
- [ ] `ALTER TABLE ... DISABLE TRIGGER` on any held-delete-guard fires the tamper log
- [ ] A CI test exercises UPDATE/DELETE/TRUNCATE on every append-only table via the curated `append_only_registry.json` and fails the build if any succeed or if a new table is unregistered
- [ ] `sync.Map` removed from `internal/integrity/handler.go`; all job state persists in `integrity_checks` via the new repository
- [ ] Append-only tables are yearly range-partitioned (declarative partitioning in migration 021)
- [ ] `evidence_items.review_status` tracks the five-state lifecycle
- [ ] Lifecycle transitions are role-gated, reason-capturing when backward, custody-logged, with explicit actorID parameter
- [ ] Optimistic concurrency uses `review_version` BIGINT counter + `If-Match` ETag (not `If-Unmodified-Since`)
- [ ] Items can be locked at any stage
- [ ] Legal hold placement is a **resumable per-item state machine** with `evidence_items.legal_hold_migration_state` column
- [ ] Legal hold copies evidence to `evidence-locked` bucket with 100-year Object Lock compliance retention
- [ ] DB-level delete guards block removal of held rows regardless of role
- [ ] `mc rm` on locked objects fails with MinIO retention error (tested)
- [ ] Partial-failure `PlaceHold` is resumable; operator UI exposes a "resume migration" button
- [ ] Out-of-band reconciler detects held cases with evidence still in `evidence` bucket and logs CRITICAL
- [ ] MinIO CRR caveat documented prominently in operator handbook
- [ ] A migration tool (`cmd/migrate-legal-hold`) exists to move existing held cases from service-layer-only to storage-layer enforcement
- [ ] `legal_hold_events.object_count` is non-null on `placed` events, null on `released` events (enforced by CHECK)
- [ ] **`RoleAPIService = 0` remains unchanged** in the SystemRole enum; `RoleAuditor` is appended as value 4
- [ ] `ac.SystemRole <` / `>` ordinal comparisons removed from the codebase (enforced by CI grep check)
- [ ] `RoleAuditor` exists and is read-only across every endpoint including bulk, signed-URL, and report-generation paths
- [ ] Auditor role does NOT unlock witness identity decryption
- [ ] **Phantom migration 024 (`user_system_roles`) removed** — the auditor role change is Keycloak + Go enum only, zero DB migration
- [ ] Four preset role cards selectable from the user invite dialog
- [ ] Keycloak seed script imports preset realm roles idempotently
- [ ] `vaultkeeper-verify` CLI ships as a standalone Go module with no imports from `internal/`
- [ ] CLI cross-platform reproducible builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [ ] CLI binary size under 15 MB (enforced in CI)
- [ ] CLI fuzz test runs 60 s in CI with zero crashes (`FuzzVerifyBundle`)
- [ ] CLI passes `unshare -n` network isolation test (zero network syscalls)
- [ ] CLI verifies happy-path bundles and detects every tamper mode in the fixtures
- [ ] CLI rejects ZIP-slip, decompression bomb, symlink, and absolute-path bundle entries
- [ ] CLI release binaries signed with cosign; transparency-log entry published
- [ ] CLI `--trust-store` flag accepts operator-provided PEM file
- [ ] CLI build fails if any embedded TSA cert is within 180 days of expiry
- [ ] `pkg/custodyhash` is the single source of truth for custody hash computation, imported by both the server and the CLI via a replace directive
- [ ] `pkg/custodyhash` uses string parameters (not `uuid.UUID`) at the public API
- [ ] Byte-parity test (`TestCustodyHashParity`) runs on every PR against a 1000-entry golden corpus
- [ ] Berkeley Protocol report generates from live case data in under 60 seconds on 1k-item cases
- [ ] Report ships in English AND French
- [ ] Caveats are displayed prominently in the UI and in the PDF
- [ ] Berkeley Protocol YAML is embedded via `//go:embed` at compile time — **never read from disk at runtime**
- [ ] Berkeley Protocol condition evaluator uses a curated named-flag allowlist, no free-form expressions
- [ ] Report generation requires `RequireSystemAdmin` (not merely non-auditor)
- [ ] Witness-linked audit events excluded from the personnel log chapter of the report
- [ ] Compliance reports are append-only; invalidation is a separate append-only event
- [ ] Compliance report generation rate-limited to 5/hour/user at Caddy
- [ ] Mapping YAML validated at compile time (Go test parses it)
- [ ] Legal review of the caveats text is complete (sign-off documented)
- [ ] `VAULTKEEPER_AIR_GAP=true` blocks outbound HTTP — **with DNS rebinding defense** (resolves every hostname to IP list on every request, checks each IP against CIDR allowlist)
- [ ] Air-gap guard covers IPv4 and IPv6 literal hosts
- [ ] Air-gap config validation covers **every** configured outbound host: TSA, SMTP, DB, MinIO, Meilisearch, Keycloak
- [ ] `httpfactory` exposes both `Client()` and `Transport()` so MinIO SDK can inject the guard via `minio.Options{Transport: ...}`
- [ ] Every HTTP client in `internal/` is constructed through `internal/httpfactory/`
- [ ] CI check forbids `http.Client{...}` literals outside `internal/httpfactory/`
- [ ] CI check verifies every config host appears in `Validate()`
- [ ] Startup canary uses layered TCP probe (hard fail) + DNS resolution (soft warning)
- [ ] Operator handbook documents OS-layer egress enforcement (iptables/nftables/k8s NetworkPolicy) as the authoritative air-gap mechanism
- [ ] Full test suite passes inside a Linux netns with egress dropped
- [ ] `scripts/app-bootstrap.sh` takes a clean Debian 12 VM to working system in under 60 minutes (CI-measured nightly)
- [ ] Bootstrap script writes a one-time 256-bit token to `/var/lib/vaultkeeper/first-run-token` (mode 0600) and to `system_settings.first_run_token_hash`
- [ ] First-run wizard requires a valid unexpired token; missing/invalid token returns 410 Gone
- [ ] Token expires 60 minutes after generation
- [ ] Token endpoint rate-limited to 5 attempts/minute/IP at Caddy
- [ ] First-run completion uses `complete_first_run()` SECURITY DEFINER function — atomic admin creation + token deletion + state flip
- [ ] After first-run completion, `/admin/first-run` returns 410 Gone (not 404)
- [ ] `docs/operator-handbook.md` published as Markdown AND PDF, attached to releases
- [ ] `docs/air-gap.md`, `docs/custody-actions.md`, `docs/berkeley-protocol.md`, `docs/rollback-runbook.md`, `docs/post-restore-reconciliation.md`, `docs/handbook-test-plan.md` all shipped
- [ ] All Sprint 11.5 tables captured by the backup script — verified by `TestBackupCoverageSprint115`
- [ ] `evidence-locked` MinIO bucket explicitly added to the backup bucket list
- [ ] Post-restore reconciliation script ships and is documented
- [ ] Rollback runbook documents all four scenarios including the held-object one-way door
- [ ] Rate limits added to Caddyfile for all new write endpoints and compliance report generation
- [ ] Append-only tables are yearly range-partitioned; partitioning test with 3 years of fixture data asserts partition pruning
- [ ] WCAG 2.1 AA compliance on every new UI surface (Playwright + axe-core)
- [ ] hash-wasm NPM dependency pinned with exact version + SHA in lockfile
- [ ] SBOM generated for every release using syft and attached to the GitHub release
- [ ] Handbook drift check CI job parses quickstart and executes commands on every PR touching handbook files
- [ ] Non-goals explicitly documented
- [ ] 100% test coverage on all new code in Sprint 11.5
- [ ] Every feature has English + French translations where user-facing
- [ ] `MINIO_IMAGE_VERSION` pinned to a version with verified Object Lock compliance-mode support

---

## Security Checklist

- [ ] Client-side hash is displayed as a copyable receipt before upload begins
- [ ] Hash mismatch rejects upload with 409, **no inline MinIO delete** (async cleanup via outbox), no "upload anyway" affordance
- [ ] **Both `Upload` and `UploadNewVersion` handlers enforce client hash** — verified by identical test suites
- [ ] Multipart boundary smuggling: duplicate `file` fields rejected; parsed body is the only body hashed
- [ ] Append-only RLS cannot be bypassed by `vaultkeeper_app` (verified against each of 11 append-only tables)
- [ ] `auth_audit_log` and `disclosures` now have RLS (both were previously uncovered)
- [ ] `trigger_tamper_log` captures DDL against append-only tables AND `ALTER TABLE ... DISABLE TRIGGER` AND `ALTER ROLE` grants
- [ ] DDL event trigger is `SECURITY DEFINER` owned by `vaultkeeper_migrations`; inserts into `trigger_tamper_log` succeed under any invoking role
- [ ] Separate `ddl_command_start` event trigger catches `vaultkeeper_forensic_admin` LOGIN grants
- [ ] `vaultkeeper_forensic_admin` exists as NOLOGIN by default; operator runbook documents two-person authorization for LOGIN grants
- [ ] Legal hold storage enforcement uses MinIO **compliance** mode (not governance) — irreversible, documented prominently in UI and handbook
- [ ] Legal hold placement is resumable per-item; partial failures do not leave unprotected originals
- [ ] GDPR erasure requests against held items surface a conflict, never silently succeed
- [ ] MinIO CRR caveat documented in operator handbook (Object Lock is NOT replicated by default)
- [ ] `vaultkeeper-verify` has no network capability in default mode; enforced by CI netns test
- [ ] `vaultkeeper-verify` performs no untrusted deserialization (strict schema + fuzz tested 60 s on every PR)
- [ ] `vaultkeeper-verify` rejects ZIP-slip, decompression bomb, symlink, absolute-path bundle entries
- [ ] `vaultkeeper-verify` release binaries signed with cosign; transparency-log entry published
- [ ] `vaultkeeper-verify` `--trust-store` flag accepts operator-provided PEM; embedded store has 180-day expiry guard
- [ ] Berkeley Protocol YAML embedded via `//go:embed` — **not read from disk at runtime**
- [ ] Berkeley Protocol condition evaluator uses curated named-flag allowlist — no free-form expression eval
- [ ] Berkeley Protocol report runs in a sandboxed PDF renderer with no FS access beyond tmp
- [ ] Report template cannot include arbitrary user HTML (strict Go `html/template`, all fields escaped)
- [ ] Report generation endpoint requires `RequireSystemAdmin` (auditors blocked)
- [ ] Witness-linked custody events excluded from the personnel log chapter
- [ ] `compliance_reports` is append-only; invalidation is a separate append-only event
- [ ] **Air-gap guard resolves every hostname on every request** (DNS rebinding defense) and rejects if ANY resolved IP is outside the CIDR allowlist
- [ ] Air-gap guard handles IPv4 AND IPv6 literals
- [ ] Air-gap config `Validate()` checks every configured outbound endpoint: TSA, SMTP, DB, MinIO, Meilisearch, Keycloak
- [ ] Air-gap canary uses layered TCP probe + DNS warning (not DNS-only)
- [ ] OS-layer egress enforcement (iptables/nftables/k8s NetworkPolicy) documented as the authoritative air-gap layer — not the Go guard alone
- [ ] `internal/httpfactory/` `Client()` and `Transport()` helpers; MinIO SDK wired via `minio.Options{Transport: httpfactory.Transport(...)}`
- [ ] CI grep lint forbids raw `http.Client{}` outside factory
- [ ] CI grep lint forbids `ac.SystemRole <` / `>` patterns
- [ ] Auditor role cannot mutate anything (capability check at every mutation endpoint, including bulk, signed-URL, report generation)
- [ ] Auditor role does NOT unlock witness identity decryption
- [ ] `RoleAPIService = 0` remains unchanged; `RoleAuditor = 4` appended — verified in enum-value test
- [ ] Bootstrap script writes secrets with mode 0600, never world-readable
- [ ] First-run wizard token is one-time 256-bit, single-use, stored hashed in DB, expires after 60 minutes, rate-limited at Caddy
- [ ] First-run completion uses `complete_first_run()` SECURITY DEFINER function — atomic
- [ ] `/admin/first-run` returns 410 Gone (not 404) after completion
- [ ] Preset role assignments are logged to the audit trail
- [ ] `cmd/migrate-legal-hold` requires system admin role and logs every moved object
- [ ] All Sprint 11.5 tables + `evidence-locked` bucket covered by the backup script (enforced by `TestBackupCoverageSprint115`)
- [ ] Rollback runbook documents the held-object one-way door prominently

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 11.5 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

#### Client-side hashing & server verification (Step 1)
- `upload-hasher.hashFileStreaming` — known-vector outputs for 0-byte, 1-byte, 64-byte, 8-MiB, and 2-GB synthetic files
- `upload-hasher.hashFileStreaming` — AbortSignal aborts cleanly and rejects with AbortError
- `upload-hasher.hashFileStreaming` — progress callback invoked at least once per chunk
- `evidence.isValidSHA256Hex` — accepts 64-char lowercase hex, rejects 63/65 chars, rejects non-hex, accepts uppercase
- `evidence.handler.Upload` — missing `X-Content-SHA256` header returns 400 `missing_client_hash`
- `evidence.handler.Upload` — header and form field disagree returns 400 `hash_field_disagreement`
- `evidence.handler.Upload` — attempt row created before service.Upload is called (verified by hook)
- `evidence.service.Upload` — happy path: hashes match, evidence row + TSA + custody all written
- `evidence.service.Upload` — mismatch: returns `ErrHashMismatch`, staging object deleted, custody entry written, notification fired, NO evidence row
- `evidence.service.Upload` — storage read error after write: returns wrapped error, operator intervention required

#### Append-only enforcement (Step 2)
- For each of the six tables: integration test runs UPDATE, DELETE, TRUNCATE as `vaultkeeper_app`, asserts each fails
- Integration: INSERT as `vaultkeeper_app` succeeds
- Integration: SELECT as `vaultkeeper_app` and `vaultkeeper_readonly` both succeed
- Integration: `DROP POLICY` as migration owner succeeds and adds a row to `trigger_tamper_log`
- Integration: event trigger does NOT fire on DDL against unrelated tables (false-positive guard)
- Benchmark: INSERT overhead on `custody_log` with policies active vs. without, fails if overhead > 5%
- CI guard: `append_only_check.yml` enumerates tables from `append_only_registry.json` and exercises forbidden verbs on each

#### Evidence lifecycle (Step 3)
- Unit: every valid transition × every correct role matrix entry succeeds
- Unit: every invalid transition returns `invalid_transition`
- Unit: wrong role returns 403
- Unit: backward transitions without reason return 422 `reason_required`
- Unit: locked item blocks every transition (including unlock by non-system-admin)
- Unit: transition writes `evidence_items` update + `evidence_status_transitions` row + `custody_log` row in the same transaction; tx abort rolls all three back
- Unit: optimistic lock conflict returns 409
- Integration: end-to-end upload → ingested → under_review → verified → admitted with role switching; assert consistency across three tables
- Integration: two concurrent transitions on same item → one 200, one 409
- Integration: interaction with disclosure — attempting to disclose `ingested`/`under_review` returns 422 `not_yet_reviewable`
- E2E: Playwright runs the full lifecycle with role switching

#### Legal hold storage (Step 4)
- Unit: `PlaceHold` happy path — all items moved, events written, DB updated
- Unit: `PlaceHold` partial failure — operation aborts, partial results listed, no DB flag flip
- Unit: `ReleaseHold` — DB flag flips, objects remain in locked bucket
- Unit: `PlaceHold` is idempotent on already-held case
- Integration (testcontainers + MinIO): place hold on 10-item case, verify all in `evidence-locked`, `DELETE FROM evidence_items` fails
- Integration: `mc rm local/evidence-locked/<key>` fails with retention error
- Integration: after `ReleaseHold`, objects remain in locked bucket and app reads them correctly via `storage_bucket` switch
- Integration: GDPR erasure on held item fails loudly
- Integration: new upload during hold goes to `evidence` bucket, gets moved on next hold placement
- Performance: place hold on 1000-item case completes within 10 minutes

#### Preset roles (Step 5)
- Unit: `SystemRole.IsReadOnly()` true only for `RoleAuditor`
- Unit: `requireCapability(capWrite)` rejects auditor, accepts user/case_admin/system_admin
- Unit: `requireCapability(capRead)` accepts all four roles
- Unit: `requireCapability(capSystemAdmin)` accepts only system_admin
- Unit: each preset produces the expected (system_role, case_role) pair
- Unit: auditor cannot unlock witness identity decryption
- Integration: auditor logs in, reads every listed resource, every POST/PATCH/DELETE returns 403 `read_only_role`
- Integration: `make keycloak-seed` idempotent
- CI guard: grep for `ac.SystemRole <` or `ac.SystemRole >` in `internal/` — zero matches
- E2E: invite dialog preset cards, select Auditor, new user login, read/write checks

#### CLI verifier (Step 6)
- Unit: `verify.ReadBundle` parses each fixture correctly; structural errors produce clear messages
- Unit: `verify.CheckEvidenceHash` pass and fail cases with exact error strings
- Unit: `verify.CheckTSA` pass, malformed token, signature mismatch, expired cert (warning)
- Unit: `verify.ReplayCustodyChain` matches byte-for-byte with `pkg/custodyhash` output; tampered entry detected at the exact index
- Unit: `verify.ReconstructMerkle` root match pass, single-leaf mutation fail
- Unit: `verify.VerifyManifestSignature` ed25519 pass, wrong-key fail
- Unit: `verify.HumanReport` / `verify.JSONReport` produce expected content for each result kind
- Unit: exit codes map correctly to result kinds
- Fuzz: `FuzzVerifyBundle` 60 seconds in CI, zero crashes
- Integration: server exports bundle → CLI verifies → exit 0
- Integration: every tamper fixture → exit 1 with the correct error
- CI: cross-platform build matrix + binary size budget + reproducibility check
- CI: network isolation test (`unshare -n` or equivalent) with no syscall escape
- Parity: `pkg/custodyhash` produces byte-identical output to existing `internal/custody` hash computation on a corpus of golden inputs

#### Berkeley Protocol report (Step 7)
- Unit: mapping YAML valid file parses, invalid file fails startup with clear error
- Unit: data-point queries return expected counts on fixture case
- Unit: indicator condition evaluation for every eligible outcome
- Unit: HTML template renders without errors for empty, populated, and warning-laden cases
- Integration: generate on populated case → PDF produced, stored in MinIO, `compliance_reports` row, custody event
- Integration: deterministic output — two generations of same case state produce byte-identical PDFs except for the generation timestamp
- Integration: French report — no English sentinel words
- Integration: non-admin → 403

#### Air-gap (Step 8)
- Unit: `Guard.RoundTrip` allowlisted host pass, public host block, IP literal outside allowlist block
- Unit: `CheckCanary` in strict mode with public DNS → error
- Unit: `Config.Validate` rejects `AIR_GAP=true + ACME=true`
- Unit: `Config.Validate` rejects external SMTP host not on allowlist
- Unit: reflection walk of every package under `internal/` — every `*http.Client` must come from `httpfactory.NewClient`
- CI: full suite in netns with egress blocked
- CI: lint rule forbids raw `http.Client{...}` literals outside `internal/httpfactory/`

#### Bootstrap + first-run (Step 9)
- Unit (bash): `scripts/app-bootstrap.sh --help` returns 0 and prints usage
- Unit (bash): prereq validation rejects missing Docker
- Unit (bash): generated secrets are ≥ 32 bytes and unique per run
- Unit (Go): first-run gate middleware redirects pre-completion, 404s post-completion
- Unit (Go): first-run completion token is single-use and expires
- Integration: run `scripts/app-bootstrap.sh` end-to-end in a CI container, verify all files produced
- Integration: first-run wizard programmatic walk-through flips `first_run_complete` to `true`
- CI nightly: deployment SLO measurement on fresh Hetzner CX11, fails build if > 60 min
- Manual: one-human handbook walkthrough with stopwatch before release cut

### Integration Tests (testcontainers)

- Full upload verification: PG + MinIO + app stack, upload file with correct client hash → 201 + evidence row; flipped byte → 409 + custody entry + no evidence row
- Lifecycle end-to-end: upload → transition through all states with role switching
- Legal hold end-to-end: place hold on 10-item case, attempt psql DELETE, attempt mc rm, verify both blocked
- Preset role end-to-end: auditor preset, login, comprehensive read/write matrix
- Export + verify end-to-end: server exports bundle, CLI verifies happy path and tamper paths
- Berkeley report end-to-end: generate, download, inspect PDF structure
- Air-gap end-to-end: full suite in egress-blocked netns
- Bootstrap end-to-end: clean container → scripts/app-bootstrap.sh → services up → migrations applied → first-run wizard completes

### E2E Automated Tests (Playwright)

- Client-side hash receipt visible on upload page
- Mid-upload corruption simulation via route mocking → 409 modal
- Lifecycle UI: upload → Send to Review → Mark Verified → Admit → Lock; verify transition history panel
- Locked item blocks transitions with clear tooltip
- Legal hold placement from case settings, badge appears, evidence grid shows lock indicators
- Preset onboarding: invite with Legal Counsel (Defence) card, new user login, disclosed-only view verified
- CLI verify demo: download bundle → subprocess verify → report matches expected
- Berkeley Protocol report: compliance tab → generate → download PDF → structure assertions
- First-run wizard: fresh instance, route gate, wizard walkthrough, post-completion 404
- Air-gap mode: stack with `AIR_GAP=true`, full upload → download → verify flow, external fetch attempts error clearly

**CI enforcement:** CI blocks merge if coverage drops below 100% for new Sprint 11.5 code, OR the deployment SLO job exceeds 60 minutes, OR the air-gap suite detects any unauthorized outbound call, OR the append-only registry check finds an unregistered sensitive table, OR the CLI binary size budget is exceeded, OR a raw `http.Client{...}` literal sneaks in outside the factory, OR an ordinal `SystemRole` comparison sneaks in.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** On the evidence upload page, select a 500 MB file from disk.
   **Expected:** A progress bar labeled "Hashing locally…" appears and completes in 30 s or less on a recent laptop. A 64-character hex receipt is displayed. Upload then begins. On completion, the detail page shows the same hash.
   **Verify:** Copy the hash to a local note. Navigate to the evidence detail page later and confirm it matches. Run `sha256sum` on the original file and confirm the hash matches again.

2. [ ] **Action:** In a browser dev-tools network tab, interpose on the upload request and flip a byte in the file body mid-upload.
   **Expected:** Upload returns 409. Modal displays "Upload failed integrity check." The file does not appear in the evidence grid. The audit log shows an `upload_hash_mismatch` entry.
   **Verify:** The uploader's original file on disk is untouched. A CRITICAL notification appears in the admin inbox.

3. [ ] **Action:** Open a psql shell as `vaultkeeper_app` and run `UPDATE custody_log SET detail = 'tampered' WHERE id = (SELECT id FROM custody_log LIMIT 1)`.
   **Expected:** PostgreSQL raises a permission/policy error, no rows modified.
   **Verify:** Repeat for DELETE and TRUNCATE — all fail. Repeat for `upload_attempts_v1`, `evidence_status_transitions`, `integrity_checks`, `legal_hold_events`, `trigger_tamper_log` — all fail.

4. [ ] **Action:** As a system admin, attempt `DROP POLICY custody_log_insert ON custody_log` via psql.
   **Expected:** The DROP succeeds. Immediately check `trigger_tamper_log` — new row with session user, SQL, timestamp.
   **Verify:** Recreate the policy. Confirm append-only enforcement restored.

5. [ ] **Action:** Upload an evidence item. Observe status "Ingested." Click "Send to Review" as Investigator.
   **Expected:** Status changes to "Under Review." Transition history panel shows the entry with your user and timestamp.
   **Verify:** `custody_log` has a `review_status_transition` entry.

6. [ ] **Action:** As a Prosecutor, click "Mark Verified" then "Admit." Then click "Lock."
   **Expected:** Status progresses through Verified → Admitted with the admitted badge, then the lock icon appears. "Reject" button is disabled with tooltip explaining the lock.
   **Verify:** Transition history shows all three events.

7. [ ] **Action:** As a Judge, unlock the item and click "Return to Verified." Enter reason "Re-review requested by bench."
   **Expected:** Transition completes. History panel shows the reason.
   **Verify:** Submitting without a reason is blocked with `reason_required`.

8. [ ] **Action:** Navigate to case settings and place the case on Legal Hold.
   **Expected:** Confirmation dialog warns the operation is effectively permanent. On confirm, a background job copies evidence to the locked bucket. Within minutes (depending on case size), all items show the "Held" indicator.
   **Verify:** Open MinIO console, navigate to `evidence-locked`, all objects present. Attempt `mc rm local/evidence-locked/<key>` — retention error. Attempt `DELETE FROM evidence_items WHERE case_id = ?` via psql — trigger rejects.

9. [ ] **Action:** Release the legal hold from case settings.
   **Expected:** Legal hold badge disappears. Objects remain in the locked bucket.
   **Verify:** `legal_hold_events` has a `released` row. Evidence remains accessible via the web UI.

10. [ ] **Action:** As system admin, open user invite dialog, click the Auditor preset card, invite a test user.
    **Expected:** Invite sent. New user has system role `auditor` in Keycloak.
    **Verify:** Add auditor to a case. Log in as auditor. Case, evidence, custody, audit dashboard all visible. Upload attempt → 403 `read_only_role`. Legal hold attempt → 403. Witness identities shown only as pseudonyms.

11. [ ] **Action:** Export a case bundle from the case settings page. Save the ZIP.
    **Expected:** ZIP downloads.
    **Verify:** Run `vaultkeeper-verify path/to/bundle.zip`. Human report shows all sections PASS. Exit code 0. Run with `--json`, parse with `jq`.

12. [ ] **Action:** Unzip the bundle, flip one byte in an evidence file, re-zip, re-run `vaultkeeper-verify`.
    **Expected:** Exit code 1. Report identifies the exact item with expected vs. computed hash.
    **Verify:** Report does not crash, correct item ID shown.

13. [ ] **Action:** From the case compliance tab, click "Generate Berkeley Protocol Report," select English.
    **Expected:** Within 60 s, a new report appears in the history list. Download the PDF.
    **Verify:** Cover page has case title, exporter, timestamp, mapping version. Executive summary counts match. Chapter mapping present. Caveats prominent. Signed attestation page present.

14. [ ] **Action:** Regenerate the same report in French.
    **Expected:** All strings in French, structure identical.
    **Verify:** No English leakage. Date formatting French.

15. [ ] **Action:** Stop the stack. Set `VAULTKEEPER_AIR_GAP=true` and an invalid external TSA URL in `.env`. Restart.
    **Expected:** App refuses to start with a clear startup error naming the TSA URL as an air-gap violation.
    **Verify:** Fix the config to point to an internal TSA (or the in-tree test TSA). Restart — app boots. Upload succeeds. Attempt to configure external SMTP in admin settings — UI error: "Air-gap mode active."

16. [ ] **Action:** Use `iptables -A OUTPUT -j DROP` on the host (or a network namespace) to block all outbound traffic. Restart the app.
    **Expected:** App starts, local operations work.
    **Verify:** Upload, export, verify, report generation, lifecycle transitions, legal hold placement all work without network errors.

17. [ ] **Action:** Provision a fresh Debian 12 VM (4 GB RAM, 20 GB disk). Run `deploy/server-bootstrap/bootstrap.sh`, then `scripts/app-bootstrap.sh`. Follow the operator handbook quickstart.
    **Expected:** Wall-clock time from VM ready to "first evidence uploaded and verified" under 60 minutes.
    **Verify:** Stopwatch it. Record the time. If > 60 min, file a P0 issue against the handbook.

18. [ ] **Action:** On the fresh VM, navigate to the app before completing first-run.
    **Expected:** Every route redirects to `/admin/first-run`. Wizard welcomes, prompts for admin creation, runs connectivity tests, prompts for test upload, runs bundle verification via `vaultkeeper-verify` subprocess.
    **Verify:** All five wizard steps pass. After completion, `/admin/first-run` returns 404. System ready.

19. [ ] **Action:** Run the `cmd/migrate-legal-hold` tool against a deployment with a pre-existing legal hold on a case (simulating an upgrade from pre-11.5).
    **Expected:** Tool iterates existing held cases, calls `PlaceHold` for each, logs every moved object, exits 0.
    **Verify:** After the tool runs, objects are in `evidence-locked`, DB trigger now blocks deletion.

---

## Rollout Plan (4 weeks, 3 engineers)

**Day 0 — Discovery & prerequisite confirmation.** Single-day kickoff before Week 1. Batch all "confirmed at implementation time" reads: verify Sprint 11 has landed `integrity_checks` and the audit dashboard; read current `internal/cases/export.go` to confirm bundle format; grep all `http.Client` call sites; grep all `ac.SystemRole <` comparisons; confirm MinIO version pin works with Object Lock compliance mode; confirm `digitorus/timestamp` pin. Any blocker surfaced here moves Week 1 start back by a day.

**Week 1** — Client hashing + append-only RLS refactor prep.
- Day 1–3: migration 020 (upload_attempts_v1, notification_outbox) + Step 1 backend + frontend hasher + both Upload and UploadNewVersion handlers.
- Day 4–5: migration 021 (append-only RLS for 11 tables, SECURITY DEFINER event triggers, partitioning, `vaultkeeper_migrations` role) + append-only registry CI guard.
- Day 6: Sprint 2.1 — remove `sync.Map` from `internal/integrity/handler.go`, wire through `integrity_checks` repository.
- Day 7: integration testing of Week 1 deliverables behind `VK_FEATURE_PILOT_HARDENING=true`.

**Week 2** — Lifecycle + legal hold storage + SystemRole refactor.
- Day 8–9: migration 022 + `internal/lifecycle/` package (Transitioner/Locker/HistoryReader) + If-Match ETag + frontend lifecycle UI.
- Day 10: `SystemRole` capability refactor — the pre-cursor to adding `RoleAuditor`. Replace all 22 RequireSystemRole call sites and 10 ordinal comparisons across 13 files. Add CI grep check.
- Day 11: Add `RoleAuditor = 4` + presets + Keycloak seed + invite dialog.
- Day 12–13: migration 023 + `internal/legalhold/storage.go` (resumable per-item state machine) + `cmd/migrate-legal-hold` + reconciler + MinIO compose pinning.
- Day 14: integration testing of lifecycle + legal hold + auditor flows.

**Week 3** — Air-gap + CLI verifier + `pkg/custodyhash` extraction.
- Day 15–16: `pkg/custodyhash/` nested module + parity test + golden corpus + refactor `internal/custody/chain.go` to use it.
- Day 17: `internal/airgap/` (DNS-rebinding-resistant guard, multi-layer canary, config validation over all outbound hosts) + `internal/httpfactory/` + migration of 5 existing HTTP clients + MinIO Transport injection.
- Day 18: `docker-compose.airgap.yml` + egress-blocked CI job.
- Day 19–20: `tools/vaultkeeper-verify/` standalone module with replace directive + fuzz tests + cosign-signed release workflow + cross-platform builds + testdata bundles.
- Day 21: integration — server export → CLI verify → happy + tamper matrix.

**Week 4** — Berkeley Protocol + operator handbook + first-run wizard + polish.
- Day 22–23: migration 024 + `internal/compliance/` (go:embed YAML, curated condition evaluator, system-admin gating, witness-linked exclusion) + French translation + legal review closeout + case compliance tab.
- Day 24–25: migration 025 + `scripts/app-bootstrap.sh` (with one-time token generation) + first-run wizard (5 steps with token validation) + middleware gate + 410 Gone post-completion.
- Day 26: `docs/operator-handbook.md`, `docs/air-gap.md`, `docs/custody-actions.md`, `docs/berkeley-protocol.md`, `docs/rollback-runbook.md`, `docs/post-restore-reconciliation.md`, `docs/handbook-test-plan.md` all drafted. Handbook-drift CI job. Backup-coverage test.
- Day 27: WCAG 2.1 AA accessibility pass on all new UI surfaces + axe-core CI integration.
- Day 28: manual E2E full checklist against a fresh Hetzner CX11 VM (deployment SLO measurement with stopwatch). If > 60 min, file P0 and iterate.

**Merge gate:** all Definition of Done items checked, all tests passing, all manual checklist items signed off, operator handbook PDF attached to build artifacts, deployment SLO badge green, all CI checks green (including append-only registry, HTTP factory lint, SystemRole ordinal lint, air-gap config check, a11y, backup coverage, handbook drift).

**Tag:** `v1.9.0-pilot-ready` (pre-v2.0.0 hardening release). Sprint 12 tags v2.0.0.

**Descoping options if timeline slips:** defer French for the Berkeley Protocol report to Sprint 12 (English ships in 11.5); defer the deployment SLO CI workflow (run manually at release cut); defer the handbook-drift CI check (manual walkthrough suffices at release cut). These three items together can absorb up to a week of slippage without cutting pilot-readiness functionality.

---

## Downstream Impacts

- **Sprint 12** (French i18n, CI/CD, chain certificates): operator handbook already shipped in English; Sprint 12 adds French translation. Chain certificate work uses `pkg/custodyhash` as its reference for hash parity. The CLI verifier is the reference implementation for external verification.
- **Sprint 13** (Whisper transcription): air-gap mode is already enforced at the HTTP client layer; Whisper inherits the guard automatically because it uses `internal/httpfactory/`.
- **Sprint 14** (Translation + OCR): same.
- **Sprint 19** (configurable workflow engine): subsumes the five-state lifecycle from Step 3. Migration path: existing `review_status` values map 1:1 to default stage template. `evidence_status_transitions` becomes a read-only historical table after Sprint 19 swaps in `workflow_states`.
- **Sprint 21** (compliance): Berkeley Protocol report complements ISO 27001 / SOC 2 readiness. The YAML mapping structure extends to additional frameworks without code changes.

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| MinIO Object Lock semantics differ across versions | Low | High | Pin MinIO version, test against pinned version in CI, confirm on Day 0 |
| hash-wasm performance below target on low-end hardware | Medium | Medium | Benchmark on reference hardware in CI, provide fallback for files < 100 MB |
| CLI binary size creep | Medium | Low | CI size budget check, fail build if > 15 MB |
| Air-gap guard misses a dynamically-constructed HTTP client or raw TCP path | Medium | Critical | `internal/httpfactory/` factory + CI grep lint forbidding raw `http.Client{}` outside factory + reflection walk test + **OS-layer egress enforcement documented as the authoritative layer** in the operator handbook |
| **DNS rebinding against the air-gap guard** | Medium | Critical | Guard resolves every hostname to IP list on every request; rejects if any IP is not in the CIDR allowlist; the named-host allowlist has been removed from the plan because it was exploitable; test suite includes fake-resolver rebinding scenarios |
| Deployment SLO fluctuates with Hetzner provisioning latency | High | Low | Average over 3 runs per night, alert only on sustained regressions |
| Berkeley Protocol mapping becomes stale | High | Medium | Mapping has a `version` field; `mapping_snapshot` JSONB stored with each report for reproducibility; admin dashboard shows mapping age indicator |
| Append-only RLS prevents legitimate data repair | Medium | Medium | `vaultkeeper_forensic_admin` role + documented recovery runbook + every recovery action logged via the `ddl_command_start` event trigger that catches ALTER ROLE grants |
| **`vaultkeeper_forensic_admin` silent privilege escalation** | Medium | Critical | Separate `ddl_command_start` event trigger catches `ALTER ROLE ... LOGIN`; any grant is recorded in `trigger_tamper_log`; operator handbook documents a two-person authorization workflow |
| Legal hold migration loses an object during partial failure | Medium | High | Resumable per-item state machine with `legal_hold_migration_state` column; each item moves through states independently; `PlaceHold` is idempotent and can be re-invoked to resume; out-of-band reconciler detects drift and logs CRITICAL |
| **`UploadNewVersion` bypasses Step 1 client-hash enforcement** | High | Critical | Step 1 explicitly covers BOTH handlers; shared helper `extractAndValidateClientHash`; identical test suites for both endpoints |
| **Berkeley Protocol YAML SQL injection via disk path** | Medium | Critical | YAML is embedded via `//go:embed` at compile time; condition evaluator uses curated named-flag allowlist; report generation gated by `RequireSystemAdmin` |
| **Berkeley Protocol report leaks witness-access patterns to Auditors** | Medium | High | Report generation requires `RequireSystemAdmin` (auditors blocked); witness-linked custody events excluded from the personnel log section |
| **First-run wizard race between legitimate operator and attacker** | High | Critical | One-time 256-bit token from bootstrap script, mode-0600 file, hashed in DB, 60-minute expiry, rate-limited at Caddy; route returns 410 Gone post-completion |
| **`RoleAPIService` enum collision** | — | — | **Resolved in plan revision.** `RoleAPIService = 0` unchanged; `RoleAuditor = 4` appended at end. |
| **`user_system_roles` phantom migration** | — | — | **Resolved in plan revision.** Migration 024 removed; role change is Keycloak + Go enum only. |
| **`auth_audit_log` and `disclosures` mutable by `vaultkeeper_app`** | — | — | **Resolved in plan revision.** Both added to migration 021's append-only extensions. |
| **"Same transaction" spanning PG + MinIO + notifications** | — | — | **Resolved in plan revision.** Outbox pattern via `notification_outbox` table; compensation happens async via cleanup worker. |
| **Sprint 19 migration path claim was wrong** | — | — | **Resolved in plan revision.** Sprint 11.5's lifecycle is the permanent default; Sprint 19 adds a configurable overlay, not a replacement. |
| Existing export bundle format lacks fields the CLI needs | Medium | High | Day 0 discovery reads `internal/cases/export.go`; if fields missing, Week 3 scope expands; risk flagged in Day 0 summary |
| **`pkg/custodyhash` drifts from server's hash computation** | Medium | Critical | Golden-corpus parity test on every PR; any mismatch fails CI |
| Sprint 11 prerequisites not landed (`integrity_checks`, audit dashboard) | Medium | High | Day 0 verification task; if Sprint 11 incomplete, Sprint 11.5 start is delayed |
| **MinIO CRR does not replicate Object Lock retention by default** | Medium | Critical | Operator handbook has a dedicated CRR configuration section for compliance-mode replication |
| First-run wizard token leaks via server logs | Low | High | Token in POST body or header, never URL query string after the initial GET; request-logging middleware redacts these fields |
| **`compliance_reports` previously deletable** | — | — | **Resolved in plan revision.** Table is now append-only; invalidation via separate `compliance_report_invalidations` append-only table. |
| **`system_settings.first_run_complete` flippable by `vaultkeeper_app`** | — | — | **Resolved in plan revision.** RLS restricts writes to `vaultkeeper_forensic_admin`; first-run completion uses SECURITY DEFINER function. |
| **`notifications.read` flag contradiction with append-only claim** | — | — | **Resolved in plan revision.** Split: `notification_read_events` is append-only; `notifications.read` is a materialized view synced by trigger. |
| Sprint 11 (prerequisite) slips and the `integrity_checks` table or audit dashboard is not ready | Medium | High | Sprint 11.5's Step 2 creates the `integrity_checks` table itself if Sprint 11 has not shipped it; the dependency is checked at the start of Week 1 and raised as a blocker if absent |
| `SystemRole` ordinal comparison refactor breaks an undiscovered call site | Medium | Medium | Grep-based CI check runs on every PR; first refactor pass starts with an exhaustive audit of every call site |
| Current export bundle format lacks fields the CLI needs (Merkle root, inclusion proofs, manifest signature) | Medium | High | Step 6 Day 15 starts with a read of the current exporter; if fields are missing, Week 3 scope expands to add them; risk flagged in Week 3 kickoff |
| `pkg/custodyhash` extraction drifts from existing `internal/custody` hash computation | Medium | Critical | Byte-parity tests against a golden corpus; refactor is a pure extraction (no semantic change), reviewed explicitly |
| Legal review of Berkeley Protocol caveats delays release | Medium | Medium | Legal review scheduled for Week 2, not Week 3; placeholder caveats in Week 1 unblock development |

---

## Open Questions — resolved by audit review

The earlier draft of this plan carried nine open questions. Audit review has resolved most of them; the remainder are still open but are no longer blockers for Day 1 kickoff.

### RESOLVED

1. **TSA dependency in air-gap mode.** **DECIDED: option (b), operator runs `freetsa` locally.** Documented in `docs/air-gap.md`. Option (a) (in-tree ephemeral TSA) kept as a future enhancement for pilots that cannot stand up their own TSA in the first hour.
2. **MinIO compliance vs. governance mode.** **DECIDED: compliance mode.** The irreversibility is documented prominently in the operator handbook (big warning box before the legal-hold section), in the rollback runbook (held-object one-way door), and in the UI's legal-hold confirmation dialog.
3. **Evidence lifecycle vs. Sprint 19 workflow engine.** **DECIDED: Sprint 11.5's five-state model is the permanent default.** Sprint 19 adds a configurable overlay on top, not a replacement. See Step 3 "Explicit relationship to Sprint 19" for the full rationale — the earlier claim that Sprint 19 cleanly subsumes this was factually wrong and the audit confirmed it.
4. **Auditor role case-level read scope.** **DECIDED: pseudonyms only.** Auditor never unlocks witness identity decryption. Enforced by `RequireWitnessProtection` guard on decryption endpoints, independent of `RoleAuditor`.
5. **Append-only registry placement.** **DECIDED: JSON file** (`append_only_registry.json`) at repo root, consumed by CI and readable by operators without reading Go code.
6. **`SystemRole` integer values.** **DECIDED: refactor to capability checks first, then append `RoleAuditor = 4` at the end of the enum.** `RoleAPIService = 0` remains unchanged. This is the Day 10 sequence in the Rollout Plan.
7. **Sprint 11 completion status.** **HANDLED ON DAY 0.** The Day 0 discovery task now confirms Sprint 11 has delivered `integrity_checks`, the audit dashboard, and the timeline. If it has not, Day 0 escalates and Week 1 starts on a delayed schedule. Sprint 11.5 does NOT duplicate `integrity_checks` creation — it relies on Sprint 11 and extends RLS only.

### STILL OPEN (non-blocker, resolve during implementation)

8. **Berkeley Protocol caveats legal review.** Still requires project counsel sign-off. Scheduled for Week 4 (not Week 2 as previously stated — the caveats text doesn't need to exist until the report template is written in Week 4). Placeholder caveats unblock earlier development.
9. **Operator handbook licensing.** Handbook may embed copyrighted diagrams (Berkeley Protocol, RFC 3161). Confirm attribution approach with the project's AGPL-3.0 stance. Decision deadline: before the v1.9.0 tag.
10. **MinIO image version pin.** The exact release tag that supports Object Lock compliance mode cleanly must be confirmed against MinIO release notes. Current target: `RELEASE.2024-10-13T13-34-11Z` or later. Confirmed on Day 0.

---

## Success Criteria

This sprint succeeds if, after its completion, the following is true:

A skeptical sysadmin at the ICC can:

- Stand up a working VaultKeeper instance in under an hour from a bare VM (Step 9).
- Upload an evidence file and see the hash receipt before any bytes leave the machine (Step 1).
- Place the case on legal hold and confirm, via `psql` AND `mc`, that the evidence cannot be deleted by anyone (Step 4).
- Export the case and run `vaultkeeper-verify` to confirm the bundle's integrity without trusting the VaultKeeper server (Step 6).
- Generate a Berkeley Protocol compliance report with one click, in English or French (Step 7).
- Put the entire system in air-gap mode with one environment variable, pull the network cable, and continue using it (Step 8).
- Invite three colleagues using the preset roles (Investigator, Reviewer, Defence) and have them productive in under 10 minutes, including an Auditor who can see the audit trail but touch nothing (Step 5).
- Move evidence through the five-state review lifecycle and lock items at Admitted with role-gated transitions (Step 3).
- Run `UPDATE custody_log SET ...` and watch it fail with a permission error (Step 2, confirming migration 003's work extends to every new append-only table).

If any of these fail, the sprint is not done. Pilot readiness is not negotiable.
