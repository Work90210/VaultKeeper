# Sprint 10: Data Migration Tool & Bulk Upload

**Phase:** 2 — Institutional Features
**Duration:** Weeks 19-20
**Goal:** Implement the 5-step data migration protocol with cryptographic hash bridging (for institutions migrating from Relativity/other systems) and bulk upload for batch evidence ingestion.

---

## Prerequisites

- Sprint 9 complete (classifications, legal hold, destruction)
- Evidence upload pipeline fully operational
- RFC 3161 timestamping operational

---

## Task Type

- [x] Backend (Go)
- [ ] Frontend (Next.js — minimal UI, primarily CLI tool)

---

## Implementation Steps

### Step 1: Migration Tool — Source Manifest Parser

**Deliverable:** Parse migration manifests from various sources.

**Supported manifest formats:**
- **CSV manifest** (universal): filename, sha256_hash, title, description, source, source_date, tags, classification, metadata
- **RelativityOne export** (structured): parse Relativity's export format (CSV + folder structure)
- **Folder structure** (bare files): directory of files with optional CSV metadata

**Interface:**
```go
type ManifestParser interface {
    Parse(ctx context.Context, source io.Reader, format string) ([]ManifestEntry, error)
}

type ManifestEntry struct {
    FilePath       string
    OriginalHash   string    // Hash from source system (if available)
    Title          string
    Description    string
    Source         string
    SourceDate     *time.Time
    Tags           []string
    Classification string
    Metadata       map[string]any
}
```

**Validation:**
- All entries must have a file path
- Hash format validated (64-char hex for SHA-256)
- Duplicate file paths → error
- Missing files in source directory → error (pre-check before ingestion)

**Tests:**
- CSV with all fields → parsed correctly
- CSV with optional fields missing → defaults applied
- Invalid hash format → error
- Duplicate file paths → error
- Empty manifest → error
- Large manifest (10,000 entries) → parsed within 5 seconds
- Malformed CSV → clear error with line number

### Step 2: Migration Tool — Verified Ingestion

**Deliverable:** Import evidence with dual-hash verification.

**Flow per file (Step 2 of the 5-step protocol):**
1. Read file from source (local path, ZIP, USB, network share)
2. Compute SHA-256 hash on ingestion
3. If source manifest has hash → compare hashes
4. If hashes match → proceed
5. If hashes mismatch → HALT migration, flag the file, alert
6. Store in MinIO with SSE encryption
7. Request RFC 3161 timestamp for the computed hash
8. Create evidence_items row with custody log entry:
```json
{
    "action": "migrated",
    "details": {
        "source_system": "RelativityOne",
        "source_hash": "a1b2c3...",
        "computed_hash": "a1b2c3...",
        "match": true,
        "manifest_entry": "row 42"
    }
}
```

**Batch processing:**
- Process files in parallel (configurable concurrency, default 4)
- Progress reporting: processed/total, current file, estimated time
- Resume capability: track processed files, skip on restart
- Atomic: if any file fails hash verification, entire batch can be rolled back

**Tests:**
- Single file ingestion → correct hash, custody logged
- Hash match → migration succeeds
- Hash mismatch → migration halts at that file
- Large file (1GB) → hash computed correctly
- Resume after interruption → already-processed files skipped
- 100 files in batch → all processed with correct progress
- TSA unavailable → migration continues (timestamps pending)
- MinIO full → error, no partial state

### Step 3: Migration Tool — RFC 3161 Event Timestamp

**Deliverable:** Timestamp the entire migration event.

**Step 3 of the 5-step protocol:**
1. Compute SHA-256 hash of the migration summary:
   - Concatenate: all file hashes + source system name + migration timestamp
   - Hash the concatenation
2. Request RFC 3161 timestamp for this "migration hash"
3. Store timestamp token in a dedicated migration record
4. This proves: "At [exact time], these [N] files with these [N] hashes were verified as migrated"

**Migration record (new table):**
```sql
CREATE TABLE migrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    source_system   TEXT NOT NULL,
    total_items     INT NOT NULL,
    matched_items   INT NOT NULL,
    mismatched_items INT NOT NULL DEFAULT 0,
    migration_hash  TEXT NOT NULL,
    tsa_token       BYTEA,
    tsa_timestamp   TIMESTAMPTZ,
    started_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    performed_by    TEXT NOT NULL,
    manifest_hash   TEXT NOT NULL,
    status          TEXT DEFAULT 'in_progress'
);
```

**Tests:**
- Migration hash computed correctly (deterministic)
- TSA timestamp obtained for migration event
- Migration record tracks all items
- Verification: re-compute migration hash → matches stored hash

### Step 4: Migration Attestation Certificate (PDF)

**Deliverable:** Auto-generated signed PDF for court submission.

**Certificate contents:**
- Header: "Migration Attestation Certificate"
- Migration date and time (from RFC 3161 timestamp)
- Source system name
- Total items migrated
- Hash verification results: N matched, 0 mismatched
- Table: each file with source hash, computed hash, match status
- Migration hash + RFC 3161 timestamp token (base64)
- Statement: "On [date], [N] evidence items were transferred from [source system] to VaultKeeper. Every file's source hash was verified against the hash computed on ingestion. All [N] matched. Zero discrepancies."
- Digital signature (self-signed for now)

**Certificate fields (detailed):**
```
MIGRATION ATTESTATION CERTIFICATE
══════════════════════════════════

Certificate ID:        [UUID]
Generated:             [date/time from RFC 3161 TSA]

Source System:          RelativityOne (or other)
Destination System:     VaultKeeper v[version]
Target Case:            [reference_code] - [title]

Migration Performed By: [user name / role]
Migration Date:         [start date] — [completion date]

VERIFICATION SUMMARY
────────────────────
Total Evidence Items:   [N]
Hash Verified (Match):  [N] ✓
Hash Mismatch:          0 ✗
Files Not Found:        0 ⚠

MIGRATION HASH
──────────────
SHA-256 of all file hashes:  [hex string]
RFC 3161 Timestamp:          [TSA timestamp]
Timestamp Authority:         [TSA name]
Token (base64):              [truncated... full in appendix]

ATTESTATION
───────────
On [date], [N] evidence items were transferred from [source system] 
to VaultKeeper. Every file's source hash was verified against the hash 
computed on ingestion. All [N] matched. Zero discrepancies.

Full file list with dual hashes: See Appendix A.

APPENDIX A: FILE VERIFICATION TABLE
────────────────────────────────────
| # | Filename | Source Hash | Computed Hash | Match |
|---|----------|-------------|---------------|-------|
| 1 | doc1.pdf | a1b2c3...   | a1b2c3...     | ✓     |
...

Signed: [VaultKeeper instance / self-signed certificate]
Signature Algorithm: Ed25519 (or RSA-SHA256)
```

**Signing:**
- Self-signed for now (using the instance's Ed25519 keypair from federation, or generated on first use)
- In production: institution can provide their own PKI certificate for signing
- Signature covers: certificate body (everything above "Signed:")
- Verifiable: public key published at `/.well-known/vaultkeeper-signing-key`

**Tests:**
- PDF generated with correct data
- All file entries present in table
- Hash values match database records
- Timestamp matches RFC 3161 response
- PDF well-formed and readable
- Signature verifiable with public key
- Certificate in French when locale=fr

### Step 5: Migration CLI Command

**Deliverable:** CLI interface for running migrations (in addition to API).

```bash
# Run migration from CSV manifest + folder
vaultkeeper migrate \
    --case ICC-UKR-2024 \
    --source-system RelativityOne \
    --manifest /path/to/manifest.csv \
    --files /path/to/evidence-folder/ \
    --concurrency 4

# Generate attestation certificate for completed migration
vaultkeeper migrate certificate \
    --migration-id <uuid> \
    --output /path/to/certificate.pdf
```

**Implementation:**
- New `cmd/migrate/main.go` entry point
- Reuses same service layer as API
- Progress bar in terminal
- Dry-run mode (verify hashes without importing)
- Resume support (tracks state in local file)

### Step 6: Bulk Upload

**Deliverable:** Upload ZIP of multiple files, auto-extract and process each individually.

**Endpoint:** `POST /api/cases/:id/evidence/bulk`

**Flow:**
1. Receive ZIP file (via tus protocol for large archives)
2. Extract files to temp directory
3. For each file in ZIP:
   a. Hash, timestamp, store in MinIO
   b. Auto-detect MIME type
   c. Extract EXIF metadata (images)
   d. Generate evidence number
   e. Create evidence_items row + custody log entry
4. Optional: CSV metadata file inside ZIP (`_metadata.csv`) maps filenames to title/description/tags
5. Background processing via Postgres-backed job queue (goroutine pool, configurable concurrency, default 4)
6. Progress endpoint: `GET /api/cases/:id/evidence/bulk/:jobId/status`

**Bulk upload job queue (`internal/evidence/bulk_queue.go`):**
- Job created when ZIP received: `status: "extracting"`
- Extraction phase: unpack ZIP, validate entries → `status: "processing"`
- Processing phase: per-file hash/store/index → progress: `{processed: 45, total: 200, failed: 0}`
- Completion: `status: "completed"` or `status: "completed_with_errors"`
- Each file processed independently — one failure doesn't block others
- Failed files logged with error reason, available in job status response
- Job resumable: if server restarts mid-processing, resume from last unprocessed file

**ZIP validation:**
- Max total uncompressed size: 10x MAX_UPLOAD_SIZE (100GB default)
- Max files: 10,000 per bulk upload
- Reject zip bombs (uncompressed size > limit)
- No nested ZIPs
- No symlinks (security)
- No absolute paths (path traversal prevention)

**Tests:**
- ZIP with 5 files → all processed correctly
- ZIP with _metadata.csv → metadata applied to matching files
- ZIP with unknown file → processed with defaults
- Zip bomb detection → rejected
- Large ZIP (1000 files) → processes correctly with progress
- Interrupted processing → resumable
- File within ZIP fails → others still processed, failure logged
- Nested ZIP → rejected
- Symlink in ZIP → rejected
- Absolute path in ZIP → rejected

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/migration/parser.go` | Create | Manifest parsing (CSV, Relativity) |
| `internal/migration/ingester.go` | Create | Verified ingestion with hash bridging |
| `internal/migration/certificate.go` | Create | Attestation PDF generation |
| `internal/migration/service.go` | Create | Migration orchestration |
| `internal/migration/handler.go` | Create | Migration API endpoints |
| `internal/evidence/bulk.go` | Create | Bulk ZIP upload processing |
| `cmd/migrate/main.go` | Create | CLI entry point for migrations |
| `migrations/007_migrations_table.up.sql` | Create | Migrations tracking table |

---

## Definition of Done

- [ ] CSV manifest parsing works with all field combinations
- [ ] Hash bridging: source hash vs computed hash verified for every file
- [ ] Hash mismatch halts migration and flags the file
- [ ] RFC 3161 timestamp obtained for migration event
- [ ] Attestation certificate PDF generated with correct data
- [ ] CLI tool runs migration end-to-end
- [ ] Resume capability after interruption
- [ ] Bulk ZIP upload extracts, hashes, timestamps, stores each file
- [ ] Zip bomb detection prevents resource exhaustion
- [ ] Path traversal via ZIP filenames blocked
- [ ] Progress tracking for both migration and bulk upload
- [ ] 100% test coverage

---

## Security Checklist

- [ ] ZIP extraction validates against zip bombs (size ratio check)
- [ ] No symlink following in ZIP extraction
- [ ] No absolute paths from ZIP entries
- [ ] Temp files cleaned up after processing
- [ ] Migration CLI requires authentication (API key or admin credentials)
- [ ] Manifest parsing doesn't execute embedded content
- [ ] File paths sanitized from manifest entries
- [ ] Concurrent processing doesn't exceed resource limits

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 10 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `parser.ParseCSV` — valid CSV with all fields parsed correctly, optional fields default, malformed CSV returns error with line number
- `parser.ParseCSV` — empty manifest returns error, duplicate file paths returns error, invalid hash format (non-hex, wrong length) returns error
- `parser.ParseCSV` — 10,000-entry manifest parses within 5 seconds (benchmark test)
- `parser.ParseRelativity` — Relativity export format parsed into ManifestEntry structs
- `parser.ParseFolder` — directory with optional CSV metadata matched to files
- `ingester.IngestFile` — computes SHA-256, compares with source hash, stores in MinIO, creates evidence_items row, creates custody log entry
- `ingester.IngestFile` — hash match proceeds, hash mismatch halts and flags
- `ingester.IngestFile` — missing file in source directory returns pre-check error
- `ingester.BatchIngest` — parallel processing with configurable concurrency, progress reporting (processed/total/current file)
- `ingester.BatchIngest` — resume after interruption skips already-processed files
- `ingester.BatchIngest` — atomic rollback on hash verification failure
- `migration.ComputeMigrationHash` — deterministic hash of concatenated file hashes + source system + timestamp
- `migration.CreateMigrationRecord` — all fields persisted, TSA timestamp obtained
- `certificate.GenerateAttestationPDF` — PDF contains header, source system, item count, verification summary, file table, migration hash, attestation statement, signature
- `certificate.GenerateAttestationPDF` — each file entry in appendix has source hash, computed hash, match status
- `certificate.GenerateAttestationPDF` — French locale generates French certificate
- `certificate.VerifySignature` — signature verifiable with public key
- `bulk.ValidateZIP` — rejects zip bombs (uncompressed > 10x limit), nested ZIPs, symlinks, absolute paths
- `bulk.ValidateZIP` — rejects archives exceeding 10,000 files
- `bulk.ExtractAndProcess` — each file hashed, timestamped, stored, evidence_items row created
- `bulk.ExtractAndProcess` — _metadata.csv inside ZIP maps filenames to title/description/tags
- `bulk.JobStatus` — status transitions: extracting → processing → completed/completed_with_errors
- `bulk.ExtractAndProcess` — single file failure does not block others, failure logged in job status

### Integration Tests (testcontainers)

- Full CSV migration pipeline: create case, provide CSV manifest + folder of 10 test files, run migration, verify all 10 evidence_items rows created with correct hashes, custody log entries with "migrated" action, migration record with TSA timestamp
- Hash mismatch detection: provide CSV with one deliberately wrong hash, run migration — verify migration halts at that file, flags it, remaining files not processed (or processed depending on config), migration record shows 1 mismatch
- Hash bridging verification: provide manifest with source hashes, ingest files, verify custody log contains both source_hash and computed_hash for each file, verify they match
- Resume after interruption: start migration with 10 files, interrupt after 5, restart — verify only files 6-10 processed, final migration record shows all 10
- Migration RFC 3161 timestamp: complete migration, verify migration_hash computed deterministically, TSA token stored, re-compute migration hash from stored data — matches
- Attestation certificate generation: complete migration, generate PDF, verify PDF is well-formed, contains all file entries, hash values match database records, signature verifiable
- Bulk ZIP upload end-to-end: upload ZIP with 5 files + _metadata.csv, verify all 5 evidence_items created, metadata applied from CSV, progress endpoint shows correct counts, job status transitions to "completed"
- Bulk ZIP with failure: upload ZIP where one file is corrupt, verify other 4 files processed successfully, job status shows "completed_with_errors" with failure details
- Zip bomb rejection: upload a zip bomb (small compressed, huge uncompressed), verify rejected before extraction with clear error

### E2E Automated Tests (Playwright)

- CLI migration dry run: execute `vaultkeeper migrate --dry-run` with test manifest + files, verify output shows hash verification results without creating any evidence items in the database
- CLI migration full run: execute `vaultkeeper migrate` with CSV manifest + folder of 5 test files, verify progress bar output, verify all 5 items appear in case evidence grid, verify custody chain shows "migrated" action for each
- CLI attestation certificate: execute `vaultkeeper migrate certificate --migration-id <id>`, verify PDF file created at output path, open PDF and verify it contains migration summary and file table
- Bulk upload via UI: navigate to case, click bulk upload, select ZIP file, verify progress bar shows extraction then processing phases, verify progress updates (e.g., "3/5 files processed"), verify all files appear in evidence grid upon completion
- Bulk upload with metadata CSV: upload ZIP containing _metadata.csv, verify evidence items have titles and descriptions from the CSV, verify tags applied
- Bulk upload error handling: upload ZIP with one corrupt file, verify progress shows "completed with errors," verify error details accessible, verify other files processed successfully

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Prepare a CSV manifest with 5 entries (filename, sha256_hash, title, description, source, tags) and a folder with the corresponding 5 files. Run `vaultkeeper migrate --case ICC-TEST-2024 --source-system RelativityOne --manifest manifest.csv --files ./evidence-folder/ --concurrency 2`.
   **Expected:** CLI shows a progress bar advancing through each file. Each file's hash is computed and compared against the manifest hash. All 5 match. Migration completes with summary: "5/5 files migrated, 0 mismatches."
   **Verify:** Log in to VaultKeeper UI, navigate to case ICC-TEST-2024, verify all 5 evidence items present with correct titles and descriptions from the manifest. Check custody chain for each — "migrated" action with source_hash and computed_hash.

2. [ ] **Action:** Modify one file in the evidence folder (append a byte) so its hash no longer matches the manifest. Run the migration again.
   **Expected:** Migration processes files until it reaches the modified file. It detects the hash mismatch and halts. CLI output shows: "HASH MISMATCH: file X — source hash: abc123, computed hash: def456. Migration halted."
   **Verify:** The modified file was NOT ingested. Files processed before the mismatch are present. Migration record shows mismatch count = 1.

3. [ ] **Action:** After a successful migration, generate the attestation certificate: `vaultkeeper migrate certificate --migration-id <uuid> --output ./certificate.pdf`.
   **Expected:** PDF file created at the specified path. Open the PDF.
   **Verify:** Certificate contains: "Migration Attestation Certificate" header, source system "RelativityOne," total items = 5, hash verified = 5, mismatches = 0. Appendix A lists all 5 files with source hash, computed hash, and match checkmark. Migration hash and RFC 3161 timestamp are present. Digital signature is present.

4. [ ] **Action:** Re-run the same migration (same manifest, same files) after it was previously completed.
   **Expected:** CLI detects all 5 files already processed (resume capability). Output shows: "5/5 files already processed, skipping. Migration complete."
   **Verify:** No duplicate evidence items created. No duplicate custody log entries.

5. [ ] **Action:** Navigate to the case in the UI. Click "Bulk Upload." Select a ZIP file containing 8 files and a `_metadata.csv` mapping 6 of those files to titles/descriptions/tags.
   **Expected:** Upload begins. Progress indicator shows: "Extracting..." then "Processing: 1/8, 2/8... 8/8." All 8 files are processed. The 6 files with metadata entries have their titles, descriptions, and tags populated. The 2 files without metadata entries have default values.
   **Verify:** Evidence grid shows 8 new items. Click on each — verify metadata matches _metadata.csv for the 6 mapped files. The 2 unmapped files have auto-generated titles based on filename.

6. [ ] **Action:** Create a ZIP file that is a zip bomb (small compressed size but expands to >100GB). Attempt to upload it via bulk upload.
   **Expected:** Upload is rejected during validation with message: "ZIP rejected — uncompressed size exceeds maximum allowed (100GB)." No files are extracted.
   **Verify:** No temporary files left on disk. No evidence items created. Server remains responsive.

7. [ ] **Action:** Create a ZIP file containing a symlink and another containing a file with an absolute path (`/etc/passwd`). Attempt to upload each.
   **Expected:** Both are rejected with clear error messages: "ZIP rejected — symlinks not permitted" and "ZIP rejected — absolute paths not permitted."
   **Verify:** No files extracted. No evidence items created.

8. [ ] **Action:** Upload a ZIP where one of the files is truncated/corrupt (invalid JPEG). Verify the bulk upload completes for the other files.
   **Expected:** Progress shows "completed with errors." Status endpoint shows: processed = N-1, failed = 1, with error details for the corrupt file.
   **Verify:** All valid files are present in the evidence grid. The failed file is listed in the job status with its error reason. No partial evidence item was created for the failed file.
