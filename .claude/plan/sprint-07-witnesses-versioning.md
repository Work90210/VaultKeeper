# Sprint 7: Witness Management & Evidence Versioning

**Phase:** 2 — Institutional Features
**Duration:** Weeks 13-14
**Goal:** Implement witness/source management with identity separation and application-level encryption, plus evidence versioning with full version history and hash chain continuity.

---

## Prerequisites

- Phase 1 (v1.0.0) complete and deployed
- Evidence upload, custody logging, role-based access all operational

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Witness Identity Encryption

**Deliverable:** Application-level encryption for witness identity fields (name, contact, location).

**Encryption scheme:**
- Algorithm: AES-256-GCM (authenticated encryption with associated data)
- Key source: separate environment variable `WITNESS_ENCRYPTION_KEY` (distinct from backup key)
- Key derivation: HKDF from master key + witness_id as context (each witness gets unique derived key)
- Encrypted fields: `full_name`, `contact_info`, `location`
- Non-encrypted fields: `witness_code`, `statement_summary`, `related_evidence`, `protection_status`
- Encryption happens in the service layer, transparent to handler and repository
- Ciphertext stored as base64 in TEXT columns (no schema change needed)

**Key rotation support:**
- Store key version alongside ciphertext (prepend 1-byte version indicator)
- On rotation: decrypt with old key, re-encrypt with new key (batch job)
- Multiple key versions supported simultaneously during rotation

**Key rotation procedure (`internal/witnesses/key_rotation.go`):**
1. Admin triggers key rotation via `POST /api/admin/witness-keys/rotate`
2. New key version created (WITNESS_ENCRYPTION_KEY_V2 env var, or derived with new salt)
3. Background job iterates all witnesses:
   a. Decrypt identity fields with old key (version from ciphertext header)
   b. Re-encrypt with new key (new version in header)
   c. Update database row (atomic per-witness)
   d. Log custody event: "witness_key_rotated" (no identity content in log)
4. Progress tracked: X/total witnesses re-encrypted
5. Job is resumable (if interrupted, continue from last processed witness)
6. Old key retained in config until rotation complete (dual-key period)
7. After all witnesses re-encrypted: old key can be removed from config

**Ciphertext format:**
```
[1 byte: key version][12 bytes: nonce][N bytes: ciphertext][16 bytes: GCM tag]
```

**Tests:**
- Encrypt → decrypt roundtrip produces original value
- Different witnesses produce different ciphertext (unique derived keys)
- Tampered ciphertext → decryption fails (GCM authentication)
- Missing encryption key → startup failure (fail fast)
- Key rotation → old data still decryptable, new data uses new key
- Null fields → remain null (don't encrypt null)
- Empty string → encrypted (different from null)

### Step 2: Witness Repository & Service

**Deliverable:** CRUD operations for witnesses with identity separation.

**Repository interface:**
```go
type WitnessRepository interface {
    Create(ctx context.Context, w Witness) (Witness, error)
    FindByID(ctx context.Context, id uuid.UUID) (Witness, error)
    FindByCase(ctx context.Context, caseID uuid.UUID, pagination Pagination) (PaginatedResult[Witness], error)
    Update(ctx context.Context, id uuid.UUID, updates WitnessUpdate) (Witness, error)
}
```

**Service — identity filtering by role:**
```go
func (s *Service) GetWitness(ctx context.Context, id uuid.UUID) (WitnessView, error) {
    // Fetch full witness record
    // Check caller's case role
    // If investigator/prosecutor → return full record (decrypt identity fields)
    // If defence/observer/victim_rep → return pseudonymized view (witness_code only, identity fields nil)
    // If judge → case-by-case basis (check dedicated flag on witness record)
}
```

**WitnessView (returned to clients):**
```go
type WitnessView struct {
    ID                uuid.UUID
    CaseID            uuid.UUID
    WitnessCode       string   // always visible (e.g., "W-001")
    FullName          *string  // nil for restricted roles
    ContactInfo       *string  // nil for restricted roles
    Location          *string  // nil for restricted roles
    ProtectionStatus  string   // always visible
    StatementSummary  string   // always visible
    RelatedEvidence   []uuid.UUID
    IdentityVisible   bool     // indicates whether identity was included
}
```

**Custody logging for witness operations:**
- Witness created → logged
- Witness identity viewed (by authorized role) → logged with `action: "witness_identity_accessed"`
- Witness updated → logged with changed fields (identity fields logged as "updated" but content not included)

**Tests:**
- Create witness → stored with encrypted identity fields
- Get witness as investigator → full identity visible
- Get witness as prosecutor → full identity visible
- Get witness as defence → only pseudonym visible, identity fields nil
- Get witness as observer → only pseudonym visible
- Get witness as judge → depends on case-by-case flag
- Update witness → identity re-encrypted, custody logged
- List witnesses → paginated, identity filtered by role
- Witness code uniqueness enforced per case
- Related evidence links valid (foreign key check)
- Protection status transitions logged

### Step 3: Witness Endpoints & Frontend

**Endpoints:**
```
POST   /api/cases/:id/witnesses          → Create witness record (Investigator+)
GET    /api/cases/:id/witnesses           → List witnesses (identity filtered by role)
GET    /api/witnesses/:id                 → Get witness detail
PATCH  /api/witnesses/:id                 → Update witness (Investigator+)
```

**Frontend components:**
- `WitnessList` — Table with witness code, protection status, statement summary
  - Identity columns show "[RESTRICTED]" for defence/observer
  - Protection status badge (standard=green, protected=yellow, high_risk=red)
- `WitnessForm` — Create/edit form
  - Pseudonym (witness code) — auto-generated or manual
  - Identity fields section with "SENSITIVE" warning
  - Statement summary (textarea)
  - Related evidence (multi-select linking to evidence items)
  - Protection status selector
- `WitnessDetail` — Full detail view
  - Identity section (visible/hidden based on role)
  - Statement summary
  - Linked evidence with navigation
  - Custody log for this witness

### Step 4: Evidence Versioning

**Deliverable:** Upload new versions of evidence with full version chain.

**Version flow:**
1. User uploads new version via `POST /api/evidence/:id/version`
2. Original item marked `is_current = false`
3. New item created with:
   - `parent_id` pointing to original
   - `version` incremented
   - Own SHA-256 hash + RFC 3161 timestamp
   - Own custody chain
   - Same evidence number (e.g., ICC-UKR-2024-00001 v2)
4. Custody log: `action: "version_created"`, links both old and new items
5. Search index updated (only current version searchable by default)

**Version history query:**
```go
func (r *Repository) FindVersionHistory(ctx context.Context, evidenceID uuid.UUID) ([]EvidenceItem, error) {
    // Walk parent_id chain to find all versions
    // Return ordered by version number (ascending)
    // Include hash + timestamp for each version
}
```

**Rules:**
- Original file is NEVER modified or replaced
- Each version is a completely independent evidence item (own hash, own custody chain)
- `is_current` marks the latest version — only one per lineage
- Version history is always available (even for non-current versions)
- Defence users → only see disclosed versions

**Frontend:**
- `VersionHistory` component on evidence detail page
  - Timeline view showing all versions
  - Each version shows: version number, hash, upload date, uploader, change description
  - Click to view any version's details
  - Current version highlighted
  - Compare button (side-by-side metadata diff)

**Tests:**
- Upload new version → original marked non-current
- Version number incremented correctly
- Parent_id chain intact
- Each version has unique hash
- Each version has own TSA timestamp
- Custody log entry links both versions
- Version history returns all versions in order
- Search returns only current versions (by default)
- Defence → only sees disclosed versions in history
- Delete non-current version → blocked (preserved for legal record)
- 10 versions of same item → chain intact

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/witnesses/service.go` | Create | Witness business logic + identity filtering |
| `internal/witnesses/repository.go` | Create | Witness Postgres queries |
| `internal/witnesses/handler.go` | Create | Witness HTTP handlers |
| `internal/witnesses/encryption.go` | Create | AES-256-GCM identity encryption |
| `internal/evidence/versioning.go` | Create | Version chain logic |
| `internal/evidence/service.go` | Modify | Add versioning methods |
| `internal/evidence/handler.go` | Modify | Add version endpoints |
| `web/src/components/witnesses/*` | Create | Witness UI components |
| `web/src/components/evidence/VersionHistory.tsx` | Create | Version timeline |
| `migrations/004_witness_indexes.up.sql` | Create | Additional witness indexes |

---

## Definition of Done

- [ ] Witness identity encrypted at rest (AES-256-GCM)
- [ ] Defence users see only pseudonyms, never identity
- [ ] Investigators/prosecutors see full identity (decrypted)
- [ ] Witness operations logged to custody chain
- [ ] Identity access specifically logged
- [ ] Evidence versioning creates independent items with parent_id chain
- [ ] Original versions preserved, never modified
- [ ] Version history traversable in UI
- [ ] Each version has own hash + TSA timestamp
- [ ] Key rotation mechanism tested
- [ ] 100% test coverage on encryption, witness service, versioning

---

## Security Checklist

- [ ] Witness identity encrypted with unique derived key per witness
- [ ] Encryption key from env var, never hardcoded
- [ ] Identity fields never logged (not in slog, not in custody log details)
- [ ] Role-based identity filtering cannot be bypassed
- [ ] GCM authentication prevents ciphertext tampering
- [ ] Key version stored with ciphertext for rotation support
- [ ] Defence role definitively excluded from identity access
- [ ] Version chain immutable (no rewriting parent_id)

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 7 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/witnesses/encryption.go`**: Encrypt then decrypt roundtrip produces original value, different witnesses produce different ciphertext (unique derived keys via HKDF), tampered ciphertext fails GCM authentication, missing WITNESS_ENCRYPTION_KEY causes startup failure (fail fast), key rotation — old data decryptable with old key version, new data uses new key, null fields remain null (not encrypted), empty string encrypted (distinct from null), ciphertext format correct (1 byte version + 12 bytes nonce + N bytes ciphertext + 16 bytes GCM tag), key version byte correctly prepended and read
- **`internal/witnesses/service.go`**: Create witness stores encrypted identity fields, GetWitness as investigator returns full identity (decrypted), GetWitness as prosecutor returns full identity, GetWitness as defence returns only pseudonym (identity fields nil, IdentityVisible=false), GetWitness as observer returns only pseudonym, GetWitness as judge checks case-by-case flag, Update witness re-encrypts identity fields and logs custody (content not in log), list witnesses paginated with identity filtered by role, witness code uniqueness enforced per case, related evidence links validated (foreign key), protection status transitions logged, identity access logged as "witness_identity_accessed"
- **`internal/witnesses/repository.go`**: CRUD operations against Postgres, FindByCase with pagination, encrypted fields stored as base64 TEXT, witness_code unique per case constraint
- **`internal/witnesses/handler.go`**: POST creates witness (investigator+ only), GET list filters identity by role, GET detail returns correct view per role, PATCH updates witness (investigator+ only), non-investigator POST returns 403, non-existent witness returns 404
- **`internal/witnesses/key_rotation.go`**: Rotation job decrypts with old key and re-encrypts with new key, rotation is per-witness atomic, rotation is resumable (interrupted job continues from last processed), progress tracked (X/total), dual-key period supports both versions simultaneously, custody log entry per witness (no identity content), old key removable after all witnesses re-encrypted
- **`internal/evidence/versioning.go`**: Upload new version marks original as non-current, new version has incremented version number, parent_id points to original, new version has own SHA-256 hash, new version has own TSA timestamp, custody log entry links both old and new items, version history returns all versions in order (ascending), search index updated (only current version searchable by default), defence sees only disclosed versions, delete non-current version blocked, 10 versions of same item produces intact chain
- **`internal/evidence/service.go`** (version additions): POST /evidence/:id/version creates new version with all pipeline steps (hash, TSA, MinIO, custody), PATCH on non-current version rejected, original file never modified
- **`internal/evidence/handler.go`** (version additions): GET /evidence/:id/versions returns version history, POST /evidence/:id/version returns 201 with new version metadata

### Integration Tests (with testcontainers)

- **Witness encryption + Postgres (testcontainers/postgres:16-alpine)**: Create witness with identity fields, verify stored values in database are base64 ciphertext (not plaintext), decrypt and verify match, query by witness_code works (non-encrypted field), witness_code unique constraint per case enforced
- **Witness identity filtering + roles (testcontainers: postgres + keycloak)**: Create witness, fetch as investigator (full identity), fetch as defence (pseudonym only, identity null), fetch as observer (pseudonym only), verify custody_log has "witness_identity_accessed" entry for investigator access, verify NO "witness_identity_accessed" entry for defence (identity was not accessed)
- **Key rotation (testcontainers/postgres:16-alpine)**: Create 10 witnesses with key V1, trigger rotation to V2, verify all 10 re-encrypted (ciphertext changed, plaintext identical after decrypt), verify custody_log has 10 "witness_key_rotated" entries, interrupt rotation mid-way (kill goroutine), restart and verify it resumes from last processed
- **Evidence versioning (testcontainers: postgres + minio + meilisearch)**: Upload evidence item V1, upload V2 via version endpoint, verify V1 marked is_current=false, verify V2 is_current=true, verify both have unique hashes and TSA tokens, verify parent_id chain intact, verify Meilisearch only indexes V2 (current), verify custody_log entries for both uploads and the version_created event, verify version history endpoint returns [V1, V2] in order
- **Defence visibility on versions (testcontainers: postgres)**: Create evidence V1 and V2, disclose V1 only, query as defence — only V1 visible in version history, V2 absent

### E2E Automated Tests (Playwright)

- **`tests/e2e/witnesses/create-witness.spec.ts`**: Login as investigator, navigate to case > witnesses tab, click "Add Witness", fill in witness code, full name, contact info, location, protection status, statement summary, link related evidence, submit, verify witness appears in list with witness code and protection status badge
- **`tests/e2e/witnesses/identity-restriction.spec.ts`**: Login as defence user, navigate to case > witnesses tab, verify witness list shows witness codes and protection status but identity columns show "[RESTRICTED]", click on a witness detail, verify full_name/contact_info/location fields are absent or show "[RESTRICTED]", verify no identity data in page source (View Source check)
- **`tests/e2e/witnesses/identity-visible.spec.ts`**: Login as investigator, navigate to same case > witnesses tab, verify identity columns show actual names, click witness detail, verify full_name/contact_info/location displayed in cleartext, verify custody log shows "witness_identity_accessed" entry
- **`tests/e2e/witnesses/update-witness.spec.ts`**: Login as investigator, navigate to witness detail, click edit, change protection status and statement summary, save, verify updated values displayed, verify custody log shows "witness_updated" entry (no identity content in log details)
- **`tests/e2e/versioning/upload-version.spec.ts`**: Login as investigator, navigate to evidence detail, click "Upload New Version", upload a modified file, verify new version appears in version history timeline, verify original version listed as non-current, verify new version has own hash and TSA timestamp
- **`tests/e2e/versioning/version-history.spec.ts`**: Navigate to evidence detail > version history tab, verify timeline shows all versions with version numbers, hashes, upload dates, and uploaders, verify current version highlighted, click a previous version to view its details, verify previous version's hash differs from current
- **`tests/e2e/versioning/defence-versions.spec.ts`**: Login as defence user, navigate to disclosed evidence detail, verify version history only shows disclosed versions, verify non-disclosed versions are absent (not hidden, absent)

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Login as investigator, navigate to a case's witnesses tab, click "Add Witness"
   **Expected:** Witness creation form appears with fields: witness code (auto-suggested), full name, contact info, location, protection status, statement summary, related evidence selector
   **Verify:** Fill in all fields including identity (full name: "Test Witness", contact: "test@example.org", location: "The Hague"), select protection status "protected", link 2 evidence items, submit — witness appears in list

2. [ ] **Action:** Connect to the Postgres database directly and query the witnesses table for the just-created witness
   **Expected:** The `full_name`, `contact_info`, and `location` columns contain base64 ciphertext, NOT plaintext
   **Verify:** Values are not human-readable; `witness_code`, `statement_summary`, and `protection_status` are stored in plaintext; ciphertext length is reasonable (not empty, not excessively long)

3. [ ] **Action:** Login as defence role user on the same case, navigate to witnesses tab
   **Expected:** Witness list shows witness codes and protection status badges, but identity columns show "[RESTRICTED]"
   **Verify:** Click on the witness detail — full_name, contact_info, location fields are completely absent or show "[RESTRICTED]"; View Page Source confirms no identity strings anywhere in the HTML; Network tab shows API response with identity fields as null

4. [ ] **Action:** Login as investigator, navigate to the same witness detail
   **Expected:** Full identity visible — name, contact, location all displayed in cleartext
   **Verify:** Custody log for the case shows a "witness_identity_accessed" entry with your user ID and timestamp; this entry does NOT contain the actual identity values in its details

5. [ ] **Action:** Login as observer role user, navigate to witnesses tab
   **Expected:** Same as defence — only pseudonyms visible, identity restricted
   **Verify:** Attempt to access witness identity via direct API call (`GET /api/witnesses/:id`) — response has identity fields as null

6. [ ] **Action:** (System admin task) Trigger witness encryption key rotation via admin API or UI
   **Expected:** Background job starts, progress indicator shows X/total witnesses re-encrypted
   **Verify:** After completion, log in as investigator, access a witness — identity still correctly decrypted; query database — ciphertext has changed (new key version byte); custody log shows "witness_key_rotated" entries for each witness

7. [ ] **Action:** Navigate to an evidence item's detail page, click "Upload New Version", select a modified file
   **Expected:** New version uploaded with progress bar, version history updates to show V1 (non-current) and V2 (current, highlighted)
   **Verify:** V2 has a different SHA-256 hash than V1; V2 has its own TSA timestamp; V1 is still downloadable; evidence number remains the same (e.g., ICC-UKR-2024-00001 v2)

8. [ ] **Action:** Navigate to the evidence grid for the case
   **Expected:** Only current versions shown in the grid (V2 in this case, not V1)
   **Verify:** Search for the evidence item — only current version appears in search results; filter "all versions" if such an option exists shows both

9. [ ] **Action:** On the evidence detail page, click the version history tab
   **Expected:** Timeline view showing all versions with version numbers, hashes, upload dates, uploaders, and change descriptions
   **Verify:** Click on V1 — navigates to V1's detail page with its own hash, TSA timestamp, and custody log; "Compare" button (if present) shows metadata diff between V1 and V2

10. [ ] **Action:** Login as defence user, navigate to an evidence item that has multiple versions but only V1 is disclosed
    **Expected:** Version history shows only V1; V2 is completely absent
    **Verify:** Direct API call to `/api/evidence/:id/versions` as defence returns only disclosed versions; no version count or metadata about undisclosed versions leaked

11. [ ] **Action:** Attempt to delete a non-current version (V1) of an evidence item
    **Expected:** Deletion blocked with error message indicating historical versions cannot be destroyed (preserved for legal record)
    **Verify:** V1 remains in MinIO, evidence_items row unchanged, no "destroyed" custody log entry

12. [ ] **Action:** Upload 5 sequential versions of the same evidence item, then verify the version chain
    **Expected:** Version history shows V1 through V5, each with unique hash and TSA timestamp, parent_id chain links V5→V4→V3→V2→V1
    **Verify:** Only V5 is marked is_current=true; all previous versions accessible via version history; custody log has "version_created" entries for each transition; integrity verification passes on all versions
