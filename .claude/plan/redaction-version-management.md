# Redaction Version Management System

**Sprint:** 22b (Sprint 22 extension)  
**Priority:** High — core workflow for prosecutors and investigators  
**Depends on:** Sprint 22 base (collaborative redaction editor, PDF page renderer, draft persistence)

---

## Problem Statement

Prosecutors handling complex cases need to produce multiple redacted versions of the same evidence for different audiences and legal proceedings. A single document may require:

- A defence disclosure with witness identities redacted
- A public release with both witness identities and classified information redacted
- A court submission with only privileged communications redacted
- An internal review copy with minimal redactions for prosecution strategy

Currently, redacted copies are unnamed derivatives with auto-generated descriptions. There is no way to name them, categorize them by purpose, manage multiple drafts in progress, or select a specific redacted version when creating a disclosure. At scale (5–100 redacted versions per evidence item), the current system is unmanageable.

---

## User Flows

### Flow 1: Creating a New Redacted Version

**Actor:** Prosecutor or investigator with case role  
**Starting point:** Evidence detail page

```
1. User views evidence detail page for a PDF document
2. User clicks "New Redacted Version" button
3. System shows the Draft Creation dialog:
   ┌──────────────────────────────────────────────┐
   │  New Redacted Version                        │
   │                                              │
   │  Name                                        │
   │  ┌──────────────────────────────────────────┐│
   │  │ Q1 Defence Disclosure                    ││
   │  └──────────────────────────────────────────┘│
   │                                              │
   │  Purpose                                     │
   │  ┌──────────────────────────────────────────┐│
   │  │ Disclosure to Defence            ▾       ││
   │  └──────────────────────────────────────────┘│
   │                                              │
   │  Purpose options:                            │
   │  · Disclosure to Defence                     │
   │  · Disclosure to Prosecution                 │
   │  · Public Release                            │
   │  · Court Submission                          │
   │  · Witness Protection                        │
   │  · Internal Review                           │
   │                                              │
   │               [Cancel]  [Create & Edit]      │
   └──────────────────────────────────────────────┘

4. User enters name and selects purpose
5. System creates a draft record in the database
6. System opens the redaction editor with this draft loaded
7. User draws redaction areas, fills reasons per area
8. System auto-saves draft every 1.5 seconds
9. User can close the editor at any time — work is preserved
```

### Flow 2: Resuming an Existing Draft

**Actor:** Same or different prosecutor on the case  
**Starting point:** Evidence detail page

```
1. User views evidence detail page
2. The "Redacted Versions" panel shows active drafts:

   REDACTED VERSIONS
   ┌─────────────────────────────────────────────────────────────┐
   │ DRAFTS IN PROGRESS                                         │
   │                                                             │
   │  Q1 Defence Disclosure           DEFENCE                   │
   │  12 areas · Last saved 08 Apr 14:32 · admin.test           │
   │                                        [Resume] [Discard]  │
   │                                                             │
   │  Witness A Protection            WITNESS                   │
   │  3 areas · Last saved 07 Apr 09:15 · prosecutor.jones      │
   │                                        [Resume] [Discard]  │
   ├─────────────────────────────────────────────────────────────┤
   │ FINALIZED                                                   │
   │                                                             │
   │  Public Release v1               PUBLIC                    │
   │  8 areas · Finalized 05 Apr 2026 · admin.test              │
   │  ICC-01/04-01/07-00005-R-PUBLIC-V1                         │
   │                                        [View] [Download]   │
   ├─────────────────────────────────────────────────────────────┤
   │                    [+ New Redacted Version]                 │
   └─────────────────────────────────────────────────────────────┘

3. User clicks "Resume" on "Q1 Defence Disclosure"
4. System opens the redaction editor with the saved draft state
5. All previous redaction areas, reasons, and positions are restored
6. User continues editing, auto-save continues
```

### Flow 3: Finalizing a Draft

**Actor:** Prosecutor with redact permission  
**Starting point:** Inside the redaction editor

```
1. User has completed all redaction areas and filled all reasons
2. User clicks "Finalize" button in the editor sidebar
3. System shows the Finalization Confirmation dialog:
   ┌──────────────────────────────────────────────┐
   │  Finalize Redaction                          │
   │                                              │
   │  Name: Q1 Defence Disclosure                 │
   │  Purpose: Disclosure to Defence              │
   │  Areas: 12 across 3 pages                    │
   │                                              │
   │  This will:                                  │
   │  · Create a permanently redacted copy        │
   │  · Destroy text layer in redacted areas      │
   │  · Generate TSA timestamp for integrity      │
   │  · Log all redaction details to custody chain │
   │                                              │
   │  The original evidence is preserved.          │
   │                                              │
   │  Evidence Number:                            │
   │  ICC-01/04-01/07-00005-R-DEFENCE-Q1          │
   │                                              │
   │  Description (optional)                      │
   │  ┌──────────────────────────────────────────┐│
   │  │ Prepared for Q1 defence disclosure       ││
   │  └──────────────────────────────────────────┘│
   │                                              │
   │               [Cancel]  [Finalize]           │
   └──────────────────────────────────────────────┘

4. User clicks "Finalize"
5. System executes in a single transaction:
   a. Locks the draft row (prevents concurrent finalization)
   b. Validates all areas have reasons
   c. Downloads the original PDF from storage
   d. Rasterizes each page at 300 DPI (destroys text layer)
   e. Paints black rectangles over each redaction area
   f. Reconstructs the PDF from rasterized images
   g. Computes SHA-256 hash of the redacted file
   h. Requests TSA timestamp from DigiCert
   i. Stores redacted file in MinIO
   j. Creates new evidence_items record as derivative:
      - parent_id → original evidence
      - evidence_number → ICC-01/04-01/07-00005-R-DEFENCE-Q1
      - is_current = false (derivative, not replacement)
      - tags include 'redacted'
      - redaction_name, redaction_purpose, redaction_area_count,
        redaction_author_id, redaction_finalized_at populated
   k. Marks draft status = 'applied'
   l. Logs custody events for each redaction area (page, reason, coordinates)
   m. Logs audit event: "redaction_finalized"
6. System closes the editor
7. Evidence detail page refreshes — new finalized version appears in panel
```

### Flow 4: Using a Redacted Version in a Disclosure

**Actor:** Prosecutor creating a disclosure package  
**Starting point:** Disclosure creation form

```
1. User is creating a new disclosure for defence counsel
2. User selects evidence items to include
3. For each evidence item with redacted versions, system shows a version picker:
   ┌──────────────────────────────────────────────┐
   │  Select version to disclose                  │
   │                                              │
   │  ○ Original (unredacted)                     │
   │    ICC-01/04-01/07-00005                     │
   │                                              │
   │  ● Q1 Defence Disclosure                     │
   │    ICC-01/04-01/07-00005-R-DEFENCE-Q1        │
   │    12 areas · Finalized 08 Apr 2026          │
   │                                              │
   │  ○ Public Release v1                         │
   │    ICC-01/04-01/07-00005-R-PUBLIC-V1         │
   │    8 areas · Finalized 05 Apr 2026           │
   │                                              │
   └──────────────────────────────────────────────┘
4. User selects the appropriate redacted version
5. System stores the selected evidence_item.id in the disclosure record
6. Defence counsel receives access to the selected redacted copy only
```

### Flow 5: Discarding a Draft

**Actor:** Any user with case role  
**Starting point:** Redacted Versions panel or inside the editor

```
1. User clicks "Discard" on a draft in the panel (or "Discard Draft" in the editor)
2. System shows confirmation:
   "Discard 'Q1 Defence Disclosure'? This cannot be undone."
3. User confirms
4. System marks draft status = 'discarded' (soft delete, audit trail preserved)
5. Draft disappears from the panel
6. The draft name becomes available for reuse
```

---

## Data Model Changes

### Migration 015: redaction_version_management

**1. Purpose enumeration**

```sql
CREATE TYPE redaction_purpose AS ENUM (
  'disclosure_defence',
  'disclosure_prosecution',
  'public_release',
  'court_submission',
  'witness_protection',
  'internal_review'
);
```

**2. Enhance redaction_drafts — support multiple named drafts per evidence**

```sql
ALTER TABLE redaction_drafts
  ADD COLUMN name TEXT,
  ADD COLUMN purpose redaction_purpose,
  ADD COLUMN area_count INTEGER NOT NULL DEFAULT 0;

-- Backfill existing drafts with sensible defaults
UPDATE redaction_drafts
SET name = 'Draft ' || to_char(created_at, 'YYYY-MM-DD HH24:MI'),
    purpose = 'internal_review';

ALTER TABLE redaction_drafts
  ALTER COLUMN name SET NOT NULL,
  ALTER COLUMN purpose SET NOT NULL;

-- Replace single-draft-per-evidence constraint with per-name uniqueness
DROP INDEX IF EXISTS idx_redaction_drafts_evidence_draft;
CREATE UNIQUE INDEX idx_redaction_drafts_unique_name
  ON redaction_drafts (evidence_id, lower(name)) WHERE status = 'draft';
CREATE INDEX idx_redaction_drafts_evidence_status
  ON redaction_drafts (evidence_id, status, last_saved_at DESC);
```

**3. Add redaction metadata to evidence_items — self-describing finalized copies**

```sql
ALTER TABLE evidence_items
  ADD COLUMN redaction_name TEXT,
  ADD COLUMN redaction_purpose redaction_purpose,
  ADD COLUMN redaction_area_count INTEGER,
  ADD COLUMN redaction_author_id UUID,
  ADD COLUMN redaction_finalized_at TIMESTAMPTZ;

-- Optimize queries for listing derivatives of an original
CREATE INDEX idx_evidence_redaction_parent
  ON evidence_items (parent_id, created_at DESC)
  WHERE parent_id IS NOT NULL;
```

**Why these choices:**
- Metadata on `evidence_items` directly (not a join table) — keeps the existing disclosure `parent_id` join working, each redacted copy is self-describing
- `redaction_purpose` as a Postgres enum — enforced at the database level, prevents invalid values
- Nullable columns on `evidence_items` — only populated for redacted derivatives, no impact on non-redacted evidence
- Name uniqueness scoped per evidence item + active drafts only — discarded/applied names can be reused

---

## API Design

### Draft Management (multi-draft)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/evidence/{id}/redact/drafts` | Create a named draft with purpose |
| `GET` | `/api/evidence/{id}/redact/drafts` | List all non-discarded drafts for this evidence |
| `GET` | `/api/evidence/{id}/redact/drafts/{draftId}` | Load a specific draft (areas + metadata) |
| `PUT` | `/api/evidence/{id}/redact/drafts/{draftId}` | Auto-save: update areas, name, or purpose |
| `DELETE` | `/api/evidence/{id}/redact/drafts/{draftId}` | Soft-delete (status → discarded) |

### Finalization

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/evidence/{id}/redact/drafts/{draftId}/finalize` | Finalize draft → create permanent redacted copy |

### Management View

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/evidence/{id}/redactions` | Combined view: finalized versions + active drafts |

### Disclosure Integration

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/disclosures/redaction-options?evidence_id={id}` | Original + all finalized versions for picker |

### Request/Response Examples

**Create draft:**
```json
POST /api/evidence/{id}/redact/drafts
{
  "name": "Q1 Defence Disclosure",
  "purpose": "disclosure_defence"
}
→ 201 { "data": { "draft_id": "uuid", "name": "...", "purpose": "..." } }
```

**Auto-save:**
```json
PUT /api/evidence/{id}/redact/drafts/{draftId}
{
  "areas": [...],
  "name": "Q1 Defence Disclosure",
  "purpose": "disclosure_defence"
}
→ 200 { "data": { "draft_id": "uuid", "last_saved_at": "..." } }
```

**Finalize:**
```json
POST /api/evidence/{id}/redact/drafts/{draftId}/finalize
{
  "description": "Prepared for Q1 defence disclosure",
  "classification": "restricted"
}
→ 201 { "data": { "evidence_id": "uuid", "evidence_number": "ICC-01/04-01/07-00005-R-DEFENCE-Q1" } }
```

**Management view:**
```json
GET /api/evidence/{id}/redactions
→ 200 {
  "data": {
    "finalized": [
      {
        "id": "uuid",
        "evidence_number": "ICC-01/04-01/07-00005-R-PUBLIC-V1",
        "name": "Public Release v1",
        "purpose": "public_release",
        "area_count": 8,
        "author": "admin.test",
        "finalized_at": "2026-04-05T10:00:00Z"
      }
    ],
    "drafts": [
      {
        "id": "uuid",
        "name": "Q1 Defence Disclosure",
        "purpose": "disclosure_defence",
        "area_count": 12,
        "author": "admin.test",
        "last_saved_at": "2026-04-08T14:32:00Z"
      }
    ]
  }
}
```

---

## Evidence Number Strategy

Finalized redacted copies receive a meaningful, human-readable evidence number derived from the original:

```
{original_number}-R-{PURPOSE_CODE}-{NAME_SLUG}
```

| Purpose | Code |
|---------|------|
| Disclosure to Defence | `DEFENCE` |
| Disclosure to Prosecution | `PROSECUTION` |
| Public Release | `PUBLIC` |
| Court Submission | `COURT` |
| Witness Protection | `WITNESS` |
| Internal Review | `INTERNAL` |

**Examples:**
- `ICC-01/04-01/07-00005-R-DEFENCE-Q1`
- `ICC-01/04-01/07-00005-R-PUBLIC-V1`
- `ICC-01/04-01/07-00005-R-WITNESS-WITNESS-A`

**Collision handling:** If the generated number already exists, append `-2`, `-3`, etc.

---

## Chain of Custody

Every state transition is logged as an immutable custody event:

| Event | When | Detail fields |
|-------|------|---------------|
| `redaction_draft_created` | User creates a new draft | draft_id, name, purpose |
| `redaction_draft_updated` | Auto-save fires | draft_id, area_count |
| `redaction_draft_discarded` | User discards a draft | draft_id, name, reason |
| `redaction_finalized` | Draft is finalized | draft_id, derived_evidence_id, name, purpose, area_count, evidence_number |
| `redacted_evidence_created` | New evidence item created | parent_id, draft_id, hash, tsa_status |
| Per-area events | During finalization | page, x, y, w, h, reason (one event per redaction area) |

---

## Implementation Steps

| Step | Scope | Deliverable |
|------|-------|-------------|
| **1** | Database | Migration 015: enum, draft columns, evidence columns, indexes, backfill |
| **2** | Go types | `RedactionPurpose` enum, draft struct, management view struct |
| **3** | Go repository | Multi-draft CRUD queries, derivative listing queries |
| **4** | Go service | Evidence number generation with slug + collision handling |
| **5** | Go handler | New REST endpoints for draft CRUD + management view |
| **6** | Go service | Finalize-from-draft transactional flow with metadata propagation |
| **7** | React | Draft picker dialog (create new / resume existing) |
| **8** | React | Editor integration (draft-specific load/save, name/purpose in sidebar) |
| **9** | React | Redacted Versions panel on evidence detail page |
| **10** | Go + React | Disclosure version picker integration |
| **11** | Go | Tests: migration, draft CRUD, finalize transaction, disclosure regression |

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `migrations/015_redaction_version_management.up.sql` | Create | Enum, columns, indexes, backfill |
| `migrations/015_redaction_version_management.down.sql` | Create | Revert all schema changes |
| `internal/evidence/models.go` | Modify | RedactionPurpose type, draft structs, evidence metadata fields |
| `internal/evidence/numbering.go` | Create | Evidence number suffix generation + collision handling |
| `internal/evidence/draft_handler.go` | Rewrite | Multi-draft REST handler with named drafts |
| `internal/evidence/redaction.go` | Modify | Finalize-from-draft flow, metadata propagation |
| `internal/evidence/repository.go` | Modify | Draft CRUD queries, list derivatives query |
| `internal/disclosures/handler.go` | Modify | Redaction version picker endpoint |
| `web/src/components/redaction/draft-picker.tsx` | Create | Draft selection/creation dialog |
| `web/src/components/redaction/collaborative-editor.tsx` | Modify | Draft-specific load/save, name/purpose fields |
| `web/src/components/evidence/redacted-versions.tsx` | Create | Management panel component |
| `web/src/components/evidence/evidence-detail.tsx` | Modify | Integrate redacted versions panel |
| `web/src/types/index.ts` | Modify | Add redaction metadata to EvidenceItem type |

---

## Architectural Decisions

| Decision | Rationale |
|----------|-----------|
| Redaction metadata lives on `evidence_items` | Self-describing derivatives; no extra join for disclosures; existing `parent_id` pattern preserved |
| Original evidence stays `is_current = true` | Source of truth for legal proceedings; redacted copies are parallel derivatives, never replacements |
| Multiple concurrent drafts per evidence | Prosecutors work on different redaction packages simultaneously; scoped name uniqueness prevents confusion |
| Finalize requires explicit `draft_id` | Legal workflows demand explicitness; prevents accidental finalization of wrong draft |
| Additive API changes only | Existing `/versions`, disclosure joins, and evidence queries continue working unchanged |
| Every transition audited to custody chain | Legal compliance; every draft creation, save, discard, and finalization is an immutable record |
| Purpose as Postgres enum | Database-level enforcement; prevents invalid values; enables efficient indexing and filtering |

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing disclosures | Additive-only schema changes; existing `parent_id` join completely unaffected |
| Draft name collisions across concurrent users | Unique partial index on `(evidence_id, lower(name)) WHERE status = 'draft'`; clear error message on collision |
| Evidence number suffix collisions | Query for existing siblings before generating; append sequential suffix on collision |
| Large number of versions (50–100+) | Paginated API; collapsed UI sections with counts; search/filter by purpose |
| Migration on populated database | New columns are nullable; backfill assigns sensible defaults to existing drafts |
| Concurrent finalization of same draft | `SELECT ... FOR UPDATE` lock on draft row; second request gets 409 Conflict |
| Long-running finalization (large PDFs) | Existing 256MB limit on redaction service; async processing not needed at current scale |

---

## SESSION_ID (for /ccg:execute use)

- CODEX_SESSION: codex-1775655603-23528
- GEMINI_SESSION: N/A (quota exhausted)
