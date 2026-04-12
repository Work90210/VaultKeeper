# Berkeley Protocol Alignment & Capture Metadata Feature

## Overview

Two deliverables:
1. **Alignment document** (`docs/berkeley-protocol-alignment.md`) — maps every Berkeley Protocol requirement to VaultKeeper features + identifies gaps
2. **Capture metadata v1 feature** — schema + API + UI for Berkeley Protocol Annex 4 online evidence collection fields

## Task Type
- [x] Frontend (capture metadata form, detail display)
- [x] Backend (new table, Go structs, API endpoints, validation)
- [x] Fullstack (parallel)

---

## Part 1: Alignment Document

### Purpose
Defensible document showing Berkeley Protocol compliance posture. Becomes product roadmap — gaps = prioritized features.

### Output Format
`docs/berkeley-protocol-alignment.md` — standalone public-facing document containing:
1. **Introduction paragraph** — what the Berkeley Protocol is, why VaultKeeper aligns with it, link to OHCHR publication
2. **Alignment table** — reformatted version of the table below (3 columns: Requirement, VaultKeeper Feature, Status)
3. **Gap roadmap** — prioritized list of gaps with target release (v1/v2/v3), serves as public product roadmap
4. **Methodology note** — how alignment was assessed, disclaimer that this is self-assessed (not certified)

### Berkeley Protocol Structure (OHCHR/UC Berkeley, 2020)

**Principles**: Professional (accuracy, impartiality), Methodological (systematic, reproducible), Ethical (privacy, do-no-harm)

**Six-Phase Investigative Cycle**:
1. Online Inquiry — systematic source identification, documented search strategies
2. Preliminary Assessment — evaluate relevance/reliability before collection
3. Collection — forensically sound capture preserving metadata + chain of custody
4. Preservation — secure long-term archiving against deletion/degradation
5. Verification — source authentication, content verification, corroboration
6. Investigative Analysis — documented analytical reasoning

**Annex Templates**:
- Annex 1: Online Investigation Plan
- Annex 2: Digital Threat & Risk Assessment
- Annex 3: Digital Landscape Assessment
- Annex 4: Online Data Collection Form

**Key Preservation Criteria** (per Digital Evidence Toolkit mapping):
- Authenticity — item unchanged since collection (SHA-256 hashes)
- Availability — continual existence and retrievability
- Identity — unique identification distinguishable from other items
- Persistence — bit sequences intact
- Renderability — humans/machines can use the item
- Understandability — item can be interpreted in context
- Chain of Custody — chronological documentation of custodians (immutable ledger)

### Alignment Table

| # | Berkeley Protocol Requirement | VaultKeeper Feature | Status | Notes |
|---|------|------|--------|-------|
| **Principles** | | | | |
| P1 | Accuracy/impartiality in evidence handling | Role-based access, classification system, audit trail | **Covered** | 6 roles with granular classification access matrix |
| P2 | Reproducible, documented processes | Chain of custody log with hash chaining, upload attempt tracking | **Covered** | Append-only custody events, forensic upload audit |
| P3 | Privacy protection | Witness encryption, GDPR erasure, redaction with purpose codes | **Covered** | Encrypted PII, right-to-be-forgotten workflow |
| P4 | Do-no-harm / protect investigators | — | **Gap** | No investigator safety tooling (operational security guidance, anonymization) |
| **Phase 1: Online Inquiry** | | | | |
| 1.1 | Document search strategies & parameters | — | **Gap** | Need `evidence_inquiry_logs` table (v2) |
| 1.2 | Record search tools/engines used | — | **Gap** | Part of inquiry log |
| 1.3 | Document discovery timeline | — | **Gap** | Part of inquiry log |
| **Phase 2: Preliminary Assessment** | | | | |
| 2.1 | Evaluate relevance before collection | — | **Gap** | Need assessment workflow (v2) |
| 2.2 | Evaluate reliability/credibility | — | **Gap** | Need reliability indicators (v2) |
| 2.3 | Filter misleading/unreliable sources | — | **Gap** | Part of assessment workflow |
| **Phase 3: Collection** | | | | |
| 3.1 | Forensically sound capture | SHA-256 hash at upload, client hash verification (constant-time), TSA timestamps | **Covered** | RFC 3161 TSA, hash mismatch detection |
| 3.2 | Preserve original metadata | EXIF extraction, original filename preservation | **Partial** | Good for images; weak for web/social captures |
| 3.3 | Record source URL | `source` field (free text) | **Partial** | Exists but unstructured — need dedicated URL field |
| 3.4 | Record source platform | — | **Gap** | Need platform enum field |
| 3.5 | Record capture method | — | **Gap** | Need capture_method enum |
| 3.6 | Record capture timestamp (when collected) | `source_date` partially overlaps | **Partial** | Semantics conflated — need distinct capture_timestamp vs publication_timestamp |
| 3.7 | Record original publication timestamp | `source_date` loosely maps | **Partial** | Need explicit separation |
| 3.8 | Hash at capture | SHA-256 computed server-side + client X-Content-SHA256 verification | **Covered** | Strong integrity |
| 3.9 | Record collector identity | `uploaded_by` + chain of custody actor | **Partial** | Exists but not structured as Berkeley "collector" role |
| 3.10 | Record content creator account/profile | — | **Gap** | Need structured account fields |
| 3.11 | Record content description | `description` field on evidence | **Partial** | Exists but serves dual purpose |
| 3.12 | Record geolocation | EXIF extraction covers images | **Partial** | Weak for non-image web captures |
| 3.13 | Record content language | — | **Gap** | Need language code field |
| 3.14 | Record content availability status (live/deleted) | — | **Gap** | Important for ephemeral content |
| 3.15 | Record browser/tool used for capture | — | **Gap** | Need capture environment metadata |
| 3.16 | Record network context (VPN, Tor, etc.) | — | **Gap** | Operational security metadata |
| **Phase 4: Preservation** | | | | |
| 4.1 | Secure long-term archiving | MinIO object storage, TSA timestamps, retention controls | **Covered** | |
| 4.2 | Prevent deletion/degradation | Retention dates, legal hold, destruction authority requirements | **Covered** | Min 10-char legal citation required |
| 4.3 | Maintain content + contextual metadata | Evidence versioning with parent_id, EXIF preservation | **Partial** | Strong for images (EXIF); weak for web/social captures without capture metadata |
| 4.4 | Evidentiary vs working copies | Redaction creates derivative with parent link | **Covered** | Original preserved, redacted copy is new item |
| 4.5 | SHA-256 hash preservation | `sha256_hash` column, indexed | **Covered** | Federal standard per Digital Evidence Toolkit |
| 4.6 | Immutable audit trail | Chain of custody with hash chaining (previous_hash → current) | **Covered** | Merkle-tree-like chain integrity |
| **Phase 5: Verification** | | | | |
| 5.1 | Source authentication | v1 capture metadata: `verification_status` field | **Partial** | Basic status tracking in v1; full workflow in v2 |
| 5.2 | Content verification (reverse image, geolocation) | v1 capture metadata: `verification_notes` field | **Partial** | Free-text notes in v1; structured verification records in v2 |
| 5.3 | Multi-source corroboration | — | **Gap** | Need corroboration workflow (v2) |
| **Phase 6: Investigative Analysis** | | | | |
| 6.1 | Documented analytical reasoning | — | **Gap** | Need analysis notes table (v2) |
| 6.2 | Iterative refinement through earlier phases | — | **Gap** | Workflow support (v2) |
| **Security** | | | | |
| S1 | Encryption and access controls | Classification system, encrypted witness PII, role-based access | **Covered** | |
| S2 | Investigator anonymity | — | **Gap** | No anonymization tooling |
| S3 | Separate professional/personal activities | — | **Out of scope** | Operational, not software |
| S4 | Isolate concurrent investigations | Case-based data isolation, per-case roles | **Covered** | |
| S5 | Regular security assessments | — | **Operational** | Process, not feature |
| **Reporting** | | | | |
| R1 | Document investigation purpose/methods | — | **Gap** | Need investigation plan template (v2) |
| R2 | Present findings with supporting evidence | Search + export functionality | **Partial** | Basic export exists |
| R3 | Maintain transparency about limitations | — | **Gap** | Need structured limitations/caveats (v2) |

### Gap Summary

| Priority | Gap | Addressable In |
|----------|-----|----------------|
| **HIGH** | Structured capture metadata (3.3-3.16) | **v1 — this sprint** |
| **HIGH** | Verification status/notes (5.1-5.2) | **v1 — included in capture metadata** |
| MEDIUM | Investigation plan/inquiry logs (1.1-1.3) | v2 |
| MEDIUM | Preliminary assessment workflow (2.1-2.3) | v2 |
| MEDIUM | Corroboration workflow (5.3) | v2 |
| MEDIUM | Investigative analysis notes (6.1-6.2) | v2 |
| LOW | Investigator safety tooling (P4, S2) | v3 |
| LOW | Reporting templates (R1, R3) | v3 |

---

## Part 2: Capture Metadata v1 Feature

### Architecture Decision: Separate Table

**`evidence_capture_metadata`** — 1:1 relationship with `evidence_items`.

Rationale:
- Berkeley metadata is domain-specific to online investigations, not universal
- Avoids nullable-column sprawl on core evidence table
- Preserves future path for multi-capture events
- Easier RBAC for sensitive fields (network context, collector identity)
- Cleaner migration (additive only, no backfill required)

### Implementation Steps

#### Step 1: Database Migration (migration 021)

File: `migrations/021_berkeley_capture_metadata.up.sql`

```sql
BEGIN;

CREATE TABLE evidence_capture_metadata (
    id                          UUID PRIMARY KEY,
    -- RESTRICT: capture provenance is part of chain of custody — must be
    -- explicitly removed before evidence can be deleted
    evidence_id                 UUID NOT NULL UNIQUE
                                REFERENCES evidence_items(id) ON DELETE RESTRICT,

    -- Source identification
    source_url                  TEXT,
    canonical_url               TEXT,
    platform                    TEXT CHECK (platform IN (
                                    'x', 'facebook', 'instagram', 'youtube',
                                    'telegram', 'tiktok', 'whatsapp', 'signal',
                                    'reddit', 'web', 'other'
                                )),
    platform_content_type       TEXT CHECK (platform_content_type IN (
                                    'post', 'profile', 'video', 'image',
                                    'comment', 'story', 'livestream',
                                    'channel', 'page', 'other'
                                )),

    -- Capture context
    capture_method              TEXT NOT NULL CHECK (capture_method IN (
                                    'screenshot', 'screen_recording', 'web_archive',
                                    'api_export', 'manual_download', 'browser_save',
                                    'forensic_tool', 'other'
                                )),
    capture_timestamp           TIMESTAMPTZ NOT NULL,
    publication_timestamp       TIMESTAMPTZ,
    -- NOTE: publication_timestamp may precede, equal, or exceed capture_timestamp
    -- in edge cases (leaked content, timezone misentry, platform backdating).
    -- Validation is app-level warning, not DB constraint.

    -- Collector identity (no FK — users are in Keycloak/SSO, not a local table)
    -- SECURITY: collector_display_name is encrypted at the application layer
    -- (same pattern as witness PII) to protect investigator identity at rest.
    collector_user_id           UUID,
    collector_display_name_encrypted BYTEA,

    -- Content creator
    creator_account_handle      TEXT,
    creator_account_display_name TEXT,
    creator_account_url         TEXT,
    creator_account_id          TEXT,

    -- Content metadata
    content_description         TEXT,
    content_language            TEXT,

    -- Geolocation
    geo_latitude                NUMERIC(9,6),
    geo_longitude               NUMERIC(9,6),
    geo_place_name              TEXT,
    geo_source                  TEXT CHECK (geo_source IN (
                                    'exif', 'platform_metadata', 'manual_entry',
                                    'derived', 'unknown'
                                )),

    -- Availability
    availability_status         TEXT CHECK (availability_status IN (
                                    'accessible', 'deleted', 'geo_blocked',
                                    'login_required', 'account_suspended',
                                    'removed', 'unavailable', 'unknown'
                                )),
    was_live                    BOOLEAN,
    was_deleted                 BOOLEAN,

    -- Capture environment
    capture_tool_name           TEXT,
    capture_tool_version        TEXT,
    browser_name                TEXT,
    browser_version             TEXT,
    browser_user_agent          TEXT,

    -- Network context (JSONB for flexibility + sensitivity)
    -- Expected shape: {"vpn_used": bool, "tor_used": bool, "proxy_used": bool,
    --                   "capture_ip_region": "XX", "notes": "..."}
    -- SECURITY: NEVER indexed in search. Access gated to investigator/prosecutor roles.
    network_context             JSONB,

    -- Preservation & verification
    preservation_notes          TEXT,
    verification_status         TEXT NOT NULL DEFAULT 'unverified' CHECK (verification_status IN (
                                    'unverified', 'partially_verified', 'verified', 'disputed'
                                )),
    verification_notes          TEXT,

    -- Schema evolution
    metadata_schema_version     INTEGER NOT NULL DEFAULT 1,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Constraints
ALTER TABLE evidence_capture_metadata
ADD CONSTRAINT chk_capture_geo_pair
CHECK (
    (geo_latitude IS NULL AND geo_longitude IS NULL) OR
    (geo_latitude IS NOT NULL AND geo_longitude IS NOT NULL)
);

-- SECURITY: Once verified, updates require elevated role (enforced at app layer).
-- Verification status changes always generate custody events.

-- Indexes
CREATE INDEX idx_capture_metadata_platform
    ON evidence_capture_metadata(platform);
CREATE INDEX idx_capture_metadata_capture_ts
    ON evidence_capture_metadata(capture_timestamp);
CREATE INDEX idx_capture_metadata_verification
    ON evidence_capture_metadata(verification_status);
CREATE INDEX idx_capture_metadata_collector
    ON evidence_capture_metadata(collector_user_id)
    WHERE collector_user_id IS NOT NULL;

COMMIT;
```

Down migration:
```sql
DROP TABLE IF EXISTS evidence_capture_metadata;
```

#### Step 2: Go Domain Model

File: `internal/evidence/capture_metadata.go`

```go
// Enum constants for platform, capture_method, availability_status, verification_status, geo_source
// (must match CHECK constraints in migration exactly — single source of truth is DB)
//
// EvidenceCaptureMetadata struct with all fields
//   - CollectorDisplayName uses encryption (same pattern as witness PII)
//   - NetworkContext is map[string]any from JSONB
//
// CaptureMetadataInput struct with validation
// Validate() method enforcing:
//   - capture_method required (from enum)
//   - capture_timestamp required, not zero
//   - publication_timestamp vs capture_timestamp: WARN if pub > capture (not reject)
//   - source_url must be http/https scheme if present (prevent javascript: URI injection)
//   - creator_account_url must be http/https scheme if present
//   - content_language BCP 47 / ISO 639-1 if present
//   - geo pair constraint (both or neither)
//   - network_context schema validation (only allowed keys: vpn_used, tor_used,
//     proxy_used, capture_ip_region, notes)
//
// SECURITY: Response serialization must gate sensitive fields by role:
//   - collector_user_id, collector_display_name: investigator/prosecutor/judge only
//   - network_context: investigator/prosecutor only (never defence/observer/victim_rep)
//   - All URL fields rendered in frontend must enforce http/https scheme check
```

Key enums:
- **Platform**: x, facebook, instagram, youtube, telegram, tiktok, whatsapp, signal, web, other
- **CaptureMethod**: screenshot, screen_recording, web_archive, api_export, manual_download, browser_save, forensic_tool, other
- **AvailabilityStatus**: accessible, deleted, geo_blocked, login_required, account_suspended, removed, unavailable, unknown
- **VerificationStatus**: unverified, partially_verified, verified, disputed
- **GeoSource**: exif, platform_metadata, manual_entry, derived, unknown

#### Step 3: Repository

File: `internal/evidence/capture_metadata_repository.go`

```go
// CaptureMetadataRepository interface:
//   GetByEvidenceID(ctx, evidenceID) → (*EvidenceCaptureMetadata, error)
//   UpsertByEvidenceID(ctx, metadata) → error
//   DeleteByEvidenceID(ctx, evidenceID) → error
//
// PostgreSQL implementation with INSERT ... ON CONFLICT (evidence_id) DO UPDATE
// All operations within transaction scope
```

#### Step 4: Service Layer

File: update `internal/evidence/service.go`

```go
// Add CaptureMetadataRepository dependency to EvidenceService
// UpsertCaptureMetadata(ctx, evidenceID, input, actor):
//   1. Validate input
//   2. Check authorization (CanEditEvidence)
//   3. Upsert metadata in transaction
//   4. Record custody event: "capture_metadata_upserted"
//   5. Update search index with new fields
//
// GetEvidence extended to LEFT JOIN capture_metadata
// Evidence response includes optional capture_metadata object
```

#### Step 5: HTTP Handler

File: update `internal/evidence/handler.go`

```go
// New endpoints:
//   PUT  /api/cases/{caseID}/evidence/{evidenceID}/capture-metadata
//   GET  /api/cases/{caseID}/evidence/{evidenceID}/capture-metadata
//
// PUT handler:
//   1. Parse JSON body → CaptureMetadataInput
//   2. Call service.UpsertCaptureMetadata
//   3. Return 200 with saved metadata
//
// GET handler:
//   1. Call service.GetCaptureMetadata
//   2. Return 200 with metadata or 404
//
// Existing GET /evidence/{id} extended to include capture_metadata in response
```

#### Step 6: Search Integration

File: update `internal/search/models.go` and `internal/search/meilisearch.go`

```go
// Add to EvidenceSearchDoc:
//   Platform           *string
//   CaptureMethod      *string
//   SourceURL          *string
//   ContentLanguage    *string
//   VerificationStatus *string
//   CaptureTimestamp   *string  // ISO 8601 — for date range filtering
//
// Filterable attributes: platform, verification_status, capture_timestamp
// Searchable attributes: source_url, content_language
//
// SECURITY: NEVER index these fields in Meilisearch:
//   - network_context (operational security — cross-investigation correlation risk)
//   - collector_user_id / collector_display_name (investigator exposure)
//   - browser_user_agent (fingerprinting vector)
//   - geo_latitude / geo_longitude (location sensitivity)
```

#### Step 7: Frontend — Types

File: update `web/src/types/index.ts`

```typescript
// Add CaptureMetadata interface with all fields
// Add to EvidenceItem: capture_metadata?: CaptureMetadata
// Add enum constants: PLATFORMS, CAPTURE_METHODS, AVAILABILITY_STATUSES, etc.
```

#### Step 8: Frontend — Upload Form Enhancement

File: update `web/src/components/evidence/evidence-uploader.tsx`

UX approach — **progressive disclosure with evidence type toggle**:

```
[Evidence Type: ○ File Upload  ● Online Capture]

When "Online Capture" selected, expand Berkeley fields section:

┌─ Capture Details ──────────────────────────────────┐
│ Source URL     [https://...]                        │
│ Platform       [▼ Select platform]                  │
│ Capture Method [▼ Select method]                    │
│ Capture Date   [📅 date picker]                     │
│ Published Date [📅 date picker] (optional)          │
├─ Content Creator (optional) ───────────────────────┤
│ Account Handle [@...]                               │
│ Display Name   [...]                                │
│ Profile URL    [https://...]                        │
├─ Content Details ──────────────────────────────────┤
│ Description    [textarea]                           │
│ Language       [▼ Select language]                  │
│ Status at Capture [▼ accessible/deleted/...]        │
├─ Capture Environment (optional) ──────────────────┤
│ Tool/Browser   [...]                                │
│ Network        [▼ Direct / VPN / Tor]               │
│ Collector Notes [textarea]                          │
└────────────────────────────────────────────────────┘
```

Design notes:
- "Online Capture" toggle replaces existing Source/SourceDate fields with structured Berkeley fields
- Collapsible sections for progressive disclosure
- Required fields: Source URL, Platform, Capture Method, Capture Date
- Everything else optional
- When evidence type is "File Upload" (default), existing Source/SourceDate remain as-is
- Integrates with existing classification, tags, description fields

#### Step 9: Frontend — Evidence Detail Display

File: update `web/src/components/evidence/evidence-detail.tsx`

```
When capture_metadata present, show "Capture Provenance" section:

┌─ Capture Provenance ──────────────────────────────┐
│ Source     x.com/user/status/123  ↗                │
│ Platform   X (Twitter)                             │
│ Method     Screenshot                              │
│ Captured   2026-04-10 14:30 UTC                    │
│ Published  2026-04-09 08:15 UTC                    │
│ Status     Accessible at capture                   │
│ Language   English                                 │
│                                                    │
│ 🔒 Collector  Jane Doe                             │
│    (investigator/prosecutor/judge only)             │
│                                                    │
│ ▸ Content Creator                                  │
│   @username · Display Name · profile ↗             │
│                                                    │
│ 🔒 Capture Environment                             │
│   Firefox 137.0 · VPN active                       │
│   (investigator/prosecutor only)                    │
│                                                    │
│ Verification: ● Unverified                         │
│ [Add Verification Notes]                           │
└────────────────────────────────────────────────────┘

SECURITY: Fields marked 🔒 are role-gated in the API response.
Defence/observer/victim_representative see these sections omitted entirely.
URL fields (Source, Profile URL) are rendered with scheme validation —
only http/https hrefs are clickable; other schemes render as plain text.
```

#### Step 10: i18n

File: update `web/src/messages/en.json` and `web/src/messages/fr.json`

Add translation keys for all field labels, enum values, section headers, validation messages.

#### Step 11: Search Filter Integration

File: update `web/src/components/search/search-filters.tsx`

Add filters for: Platform, Verification Status, Capture Date Range.

#### Step 12: Evidence Grid — Online Capture Indicator

File: update `web/src/components/evidence/evidence-grid.tsx`

When an evidence item has `capture_metadata` present, show a small indicator in the grid row
to distinguish OSINT captures from file uploads at a glance.

```
Grid columns: Evidence # | [icon] | Title | Type | Size | Classification | Uploaded
                                                                          ↑
                                                         Add "🌐" or globe icon badge
                                                         next to Type when capture_metadata
                                                         exists. Tooltip: "Online Capture"
```

Implementation:
- Existing grid is `grid-cols-[100px_40px_1fr_70px_80px_110px_100px]`
- No column change needed — overlay icon on the Type column or add small badge after type label
- Only show when `capture_metadata` is non-null in the API response
- Add i18n key: `evidenceGrid.onlineCaptureIndicator`

#### Step 13: Evidence Export — Include Capture Metadata

File: update `internal/cases/export.go`

Existing `writeMetadataCSV()` exports 12 columns from `evidence_items`. Extend to include
capture metadata for Berkeley Protocol compliance in legal exports.

Current CSV columns:
```
evidence_number, filename, original_name, mime_type, size_bytes, sha256_hash,
classification, description, uploaded_by, tsa_status, tsa_timestamp, created_at
```

Add capture metadata columns (append to existing header):
```
source_url, platform, capture_method, capture_timestamp, publication_timestamp,
content_language, availability_status, verification_status
```

Implementation:
- `ExportService` needs new dependency: `CaptureMetadataExporter` interface
  ```go
  type CaptureMetadataExporter interface {
      GetByEvidenceIDs(ctx context.Context, evidenceIDs []uuid.UUID) (map[uuid.UUID]*evidence.EvidenceCaptureMetadata, error)
  }
  ```
- Batch-fetch capture metadata for all exported evidence in one query
- Sensitive fields (collector identity, network context) are EXCLUDED from export CSV
  unless caller role is investigator/prosecutor
- Add `capture_metadata.csv` as separate file in export ZIP (alternative to extending metadata.csv)
  — keeps backward compatibility for existing metadata.csv consumers
- Update `internal/cases/export_test.go` and `internal/cases/export_handler_test.go`

Files to add to key files table:
- `internal/cases/export.go` — Modify (add capture metadata to export)
- `internal/cases/export_test.go` — Modify (test capture metadata in export)

#### Step 14: Bulk Upload — Berkeley Fields in _metadata.csv (v2 — documented)

Current `BulkMetadata` struct supports: `Title`, `Description`, `Tags`, `Classification`, `Source`, `SourceDate`.

Extending `_metadata.csv` to accept Berkeley capture fields is deferred to v2 because:
- Bulk upload is for batch file ingestion (typically physical evidence, lab results, document dumps)
- OSINT captures are typically collected one-at-a-time with per-item metadata
- Adding 15+ optional columns to bulk CSV increases complexity and error surface

**v2 plan**: Add optional columns to `_metadata.csv`:
```
source_url, platform, capture_method, capture_timestamp, publication_timestamp,
creator_account_handle, content_language, availability_status
```
Parse into `CaptureMetadataInput`, create `evidence_capture_metadata` rows alongside evidence items.

#### Step 15: Custody Event Action Registration

Existing custody action strings (from codebase grep):
- `evidence_uploaded`
- `evidence_destroyed`
- `tag_renamed`, `tags_merged`, `tag_deleted`
- `witness_created`, `witness_updated`
- `case_exported`

New action to add: **`capture_metadata_upserted`**

This action must:
1. Follow existing naming convention: `{domain}_{verb}` in snake_case
2. Be registered in custody log with detail map including:
   ```go
   map[string]string{
       "platform":            metadata.Platform,
       "capture_method":      metadata.CaptureMethod,
       "verification_status": metadata.VerificationStatus,
       "source_url_present":  "true",  // boolean, not the actual URL (sensitive)
   }
   ```
3. If `verification_status` changes specifically, record a separate action: **`capture_metadata_verified`** or **`capture_metadata_disputed`** — distinct from general upsert to enable audit filtering for verification events
4. Include in `internal/custody/` action documentation if any exists

Additional custody actions for this feature:
- `capture_metadata_upserted` — metadata created or updated
- `capture_metadata_verification_changed` — verification_status specifically changed (from/to recorded in detail)

### Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `docs/berkeley-protocol-alignment.md` | Create | Alignment mapping document |
| `migrations/021_berkeley_capture_metadata.up.sql` | Create | New table + indexes + CHECK constraints |
| `migrations/021_berkeley_capture_metadata.down.sql` | Create | Drop table |
| `internal/evidence/capture_metadata.go` | Create | Domain model + enums + validation + role-gated serialization |
| `internal/evidence/capture_metadata_test.go` | Create | Unit tests: validation, enum checks, URL scheme, geo pair, role gating |
| `internal/evidence/capture_metadata_repository.go` | Create | Repository interface + PG impl |
| `internal/evidence/capture_metadata_repository_integration_test.go` | Create | Integration tests: upsert, get, constraint enforcement |
| `internal/evidence/service.go` | Modify | Add capture metadata methods + custody event logging |
| `internal/evidence/service_test.go` | Modify | Add capture metadata service tests |
| `internal/evidence/handler.go` | Modify | Add PUT/GET endpoints |
| `internal/evidence/handler_test.go` | Modify | Add capture metadata handler tests |
| `internal/search/models.go` | Modify | Add capture fields to search doc (exclude sensitive fields) |
| `internal/search/meilisearch.go` | Modify | Index new filterable/searchable fields |
| `internal/server/server.go` | Modify | Register new routes |
| `cmd/server/main.go` | Modify | Wire CaptureMetadataRepository into dependency injection |
| `web/src/types/index.ts` | Modify | Add CaptureMetadata interface + enum constants |
| `web/src/lib/evidence-api.ts` | Modify | Add capture metadata API calls |
| `web/src/components/evidence/evidence-uploader.tsx` | Modify | Add online capture form with progressive disclosure |
| `web/src/components/evidence/evidence-detail.tsx` | Modify | Display capture provenance (role-gated fields) |
| `web/src/components/search/search-filters.tsx` | Modify | Add platform/verification/capture date filters |
| `web/src/components/evidence/evidence-grid.tsx` | Modify | Add online capture indicator badge |
| `internal/cases/export.go` | Modify | Add capture metadata to case export ZIP |
| `internal/cases/export_test.go` | Modify | Test capture metadata in export |
| `web/src/messages/en.json` | Modify | English translations (see i18n key inventory below) |
| `web/src/messages/fr.json` | Modify | French translations (see i18n key inventory below) |

### Source/SourceDate Transition Strategy

Existing `source` (TEXT) and `source_date` (TIMESTAMPTZ) on `evidence_items` coexist with new capture metadata:

- **New uploads with "Online Capture" toggle**: Berkeley fields populate `evidence_capture_metadata`. `source` and `source_date` on `evidence_items` are left empty (not duplicated).
- **New uploads with "File Upload" (default)**: Existing `source`/`source_date` fields work as before. No capture metadata row created.
- **Existing records**: Retain their `source`/`source_date` values. No automatic backfill.
- **Post-upload edit**: If a user adds capture metadata to an existing item that already has `source`/`source_date`, both coexist. The detail page shows "Capture Provenance" section when capture_metadata exists, and falls back to displaying `source`/`source_date` in the general metadata grid when it doesn't.
- **No data migration**: Existing `source` values are free-text (not reliably parseable as URLs). Manual re-entry via capture metadata form is the intended path.

### Testing Strategy (TDD — 80%+ coverage required)

Test files written FIRST per project workflow:

**Unit tests** (`capture_metadata_test.go`):
- Validate() rejects missing capture_method, zero capture_timestamp
- Validate() warns (not rejects) when publication_timestamp > capture_timestamp
- Validate() rejects non-http/https source_url and creator_account_url
- Validate() rejects geo_latitude without geo_longitude and vice versa
- Validate() rejects invalid BCP 47 language codes
- Validate() rejects unknown enum values for platform, capture_method, etc.
- Role-gated serialization: defence role sees redacted collector/network fields
- network_context schema validation rejects unknown keys

**Integration tests** (`capture_metadata_repository_integration_test.go`):
- Upsert creates new row, second upsert updates existing
- UNIQUE constraint on evidence_id enforced
- CHECK constraints fire for invalid enum values
- ON DELETE RESTRICT prevents evidence deletion while metadata exists
- Geo pair constraint fires correctly
- updated_at changes on upsert

**Handler tests** (`handler_test.go` additions):
- PUT /capture-metadata returns 200 with valid input
- PUT /capture-metadata returns 400 for invalid input
- PUT /capture-metadata returns 403 for unauthorized role
- GET /capture-metadata returns 404 when none exists
- GET /evidence/{id} includes capture_metadata when present
- Sensitive fields redacted based on caller role

**Service tests** (`service_test.go` additions):
- UpsertCaptureMetadata records custody event
- UpsertCaptureMetadata blocks update of verified metadata without elevated role
- Search indexing excludes sensitive fields

### i18n Key Inventory

Keys to add to `en.json` and `fr.json`:

**Section headers**: `captureProvenance`, `captureDetails`, `contentCreator`, `contentDetails`, `captureEnvironment`

**Field labels** (22 keys): `sourceUrl`, `canonicalUrl`, `platform`, `platformContentType`, `captureMethod`, `captureTimestamp`, `publicationTimestamp`, `collectorName`, `creatorHandle`, `creatorDisplayName`, `creatorProfileUrl`, `creatorAccountId`, `contentDescription`, `contentLanguage`, `geoLocation`, `geoPlaceName`, `availabilityStatus`, `wasLive`, `wasDeleted`, `captureTool`, `networkContext`, `verificationStatus`

**Platform enum values** (11 keys): `platform.x`, `platform.facebook`, `platform.instagram`, `platform.youtube`, `platform.telegram`, `platform.tiktok`, `platform.whatsapp`, `platform.signal`, `platform.reddit`, `platform.web`, `platform.other`

**Capture method enum values** (8 keys): `captureMethod.screenshot`, `captureMethod.screenRecording`, `captureMethod.webArchive`, `captureMethod.apiExport`, `captureMethod.manualDownload`, `captureMethod.browserSave`, `captureMethod.forensicTool`, `captureMethod.other`

**Availability status enum values** (8 keys): `availability.accessible`, `availability.deleted`, `availability.geoBlocked`, `availability.loginRequired`, `availability.accountSuspended`, `availability.removed`, `availability.unavailable`, `availability.unknown`

**Verification status enum values** (4 keys): `verification.unverified`, `verification.partiallyVerified`, `verification.verified`, `verification.disputed`

**Geo source enum values** (5 keys): `geoSource.exif`, `geoSource.platformMetadata`, `geoSource.manualEntry`, `geoSource.derived`, `geoSource.unknown`

**Validation messages** (8 keys): `validation.captureMethodRequired`, `validation.captureTimestampRequired`, `validation.invalidSourceUrl`, `validation.invalidProfileUrl`, `validation.invalidLanguageCode`, `validation.geoPairRequired`, `validation.publicationAfterCapture` (warning), `validation.verifiedMetadataLocked`

**Toggle/UI** (4 keys): `evidenceType.fileUpload`, `evidenceType.onlineCapture`, `addVerificationNotes`, `evidenceGrid.onlineCaptureIndicator`

### Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Scope creep into full 6-phase workflow | Strict v1 boundary: capture metadata only. Phase 1/2/5/6 tables deferred to v2 |
| Network context JSONB leaking sensitive ops data | NEVER indexed in search. Access gated to investigator/prosecutor only. Expected shape documented in migration |
| Collector identity exposure | `collector_display_name` encrypted at rest (BYTEA). Response serialization redacts for defence/observer/victim_rep roles |
| Source/SourceDate duplication with new fields | Online Capture toggle uses Berkeley fields; File Upload keeps existing fields. No backfill. Coexistence documented above |
| Migration complexity on existing data | Additive only — no backfill. Old evidence has no capture_metadata row. ON DELETE RESTRICT protects provenance |
| Search index bloat | Only index platform, verification_status, source_url, language, capture_timestamp. Sensitive fields explicitly excluded |
| URL injection (stored XSS, javascript: URIs) | Validate() enforces http/https scheme. Frontend renders URLs with scheme check before constructing anchor hrefs |
| Enum drift across SQL/Go/TypeScript | CHECK constraints are source of truth. Go constants must match. Frontend constants derived from shared types. PR review checklist item |
| Verified metadata tampering | Once verification_status = 'verified', updates require prosecutor+ role. All changes generate custody events |
| Timezone misentry on timestamps | App-level warning (not DB constraint) when publication_timestamp > capture_timestamp. UTC storage throughout |
| GDPR erasure for collector identity | Collector fields can be pseudonymized via existing GDPR workflow. Custody chain hash preserves integrity even after pseudonymization |

### v2 Roadmap (deferred)

1. `evidence_inquiry_logs` — search strategies, tools, dates (Phase 1)
2. `evidence_assessments` — relevance/reliability scoring (Phase 2)
3. `evidence_verification_records` — structured verification workflow (Phase 5)
4. `evidence_analysis_notes` — documented analytical reasoning (Phase 6)
5. Investigation plan templates (Annex 1)
6. Threat/risk assessment templates (Annex 2)
7. Digital landscape assessment (Annex 3)
8. Bulk upload `_metadata.csv` — optional Berkeley capture fields (see Step 14)

### SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1776000958-15613
- GEMINI_SESSION: (unavailable — quota exhausted)
