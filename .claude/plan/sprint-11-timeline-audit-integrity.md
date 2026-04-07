# Sprint 11: Evidence Timeline, Audit Dashboard & Advanced Integrity

**Phase:** 2 — Institutional Features
**Duration:** Weeks 21-22
**Goal:** Build the visual evidence timeline for case narrative construction, admin audit dashboard for system oversight, and enhanced integrity verification with full RFC 3161 token verification.

---

## Prerequisites

- Sprint 10 complete (migration, bulk upload)
- Evidence items have source_date and uploaded_at timestamps

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Evidence Timeline Backend

**Deliverable:** API endpoint returning evidence items sorted chronologically for timeline visualization.

**Endpoint:** `GET /api/cases/:id/timeline`

**Query params:**
- `from` / `to` — date range filter
- `types` — MIME type filter
- `classifications` — classification filter
- `tags` — tag filter
- `group_by` — day, week, month

**Response:**
```json
{
    "data": {
        "groups": [
            {
                "date": "2024-03-15",
                "items": [
                    {
                        "id": "uuid",
                        "evidence_number": "ICC-UKR-2024-00001",
                        "title": "Satellite image — Bucha",
                        "source_date": "2024-03-15T10:00:00Z",
                        "mime_type": "image/tiff",
                        "classification": "confidential",
                        "thumbnail_url": "/api/evidence/uuid/thumbnail",
                        "tags": ["satellite", "bucha"]
                    }
                ]
            }
        ],
        "total_items": 142,
        "date_range": {"from": "2024-01-01", "to": "2024-12-31"}
    }
}
```

**Sorting priority:** `source_date` (when the evidence was created) > `uploaded_at` (when it was added to VaultKeeper).

**Tests:**
- Items sorted by source_date
- Date range filtering correct
- Grouping by day/week/month produces correct groups
- Role-based filtering applied (defence → disclosed only)
- Classification filtering applied
- Empty date range → all items
- Items without source_date → grouped by uploaded_at

### Step 2: Evidence Timeline Frontend

**Deliverable:** Visual chronological timeline for case narrative.

**Components:**
- `Timeline` — Horizontal scrollable timeline
  - Date axis with zoom levels (day, week, month, year)
  - Evidence items positioned on timeline by source_date
  - Thumbnail previews on hover
  - Click to navigate to evidence detail
  - Color-coded by classification or type
  - Scroll to zoom, drag to pan
- `TimelineFilters` — Filter sidebar
  - Date range picker
  - Type filter
  - Tag filter
  - Classification filter
- `TimelineExport` — Export timeline as PDF
  - Print-friendly layout
  - Include evidence numbers, dates, descriptions
  - Court-submittable format

**Tests:**
- Timeline renders with mock data
- Zoom levels change granularity
- Filters update displayed items
- Click navigates to detail
- Export produces valid PDF
- Empty timeline shows appropriate message

### Step 3: Audit Dashboard Backend

**Deliverable:** System-wide audit log and statistics for admins.

**Endpoint:** `GET /api/audit`

**Query params:**
- `user_id` — filter by user
- `action` — filter by action type
- `case_id` — filter by case
- `from` / `to` — date range
- `severity` — filter by severity (info, warning, critical)

**Aggregation endpoint:** `GET /api/audit/stats`
```json
{
    "data": {
        "total_events_today": 1547,
        "active_users_today": 23,
        "evidence_uploaded_today": 42,
        "integrity_warnings": 0,
        "last_backup": "2026-04-06T03:00:00Z",
        "backup_status": "completed",
        "pending_retention_expiries": 3,
        "failed_login_attempts_24h": 7,
        "cases_active": 12,
        "cases_on_legal_hold": 3,
        "total_evidence_items": 4521,
        "total_storage_gb": 847
    }
}
```

**Tests:**
- Audit log returns all events paginated
- Filtering by each param works
- Stats aggregation correct
- Non-admin → 403
- Large audit log (100,000 entries) → paginated correctly, response < 200ms

### Step 4: Audit Dashboard Frontend

**Components:**
- `AuditDashboard` — Admin overview page
  - Key metrics cards (uploads today, active users, storage used, backup status)
  - Integrity status (green checkmark or red warning)
  - Recent activity feed (last 20 events)
  - Quick links to cases on legal hold, pending retention expiries
- `AuditLog` — Searchable, filterable audit log
  - Table: timestamp, user, action, case, evidence, IP, severity
  - Color-coded severity (info=gray, warning=yellow, critical=red)
  - Filter panel: user, action, case, date range
  - Export as CSV
- `AuditReport` — Exportable compliance report
  - PDF export of audit activity for date range
  - Designed for institutional compliance reviews

### Step 5: Advanced Integrity Verification

**Deliverable:** Enhanced verification with full RFC 3161 token verification and scheduled runs.

**Enhancements over Sprint 5's basic verification:**
- Verify RFC 3161 timestamp tokens (validate TSA signature, check hash binding)
- Verify custody chain hash integrity (detect any modified entries)
- Scheduled verification: configurable (default: weekly Sunday 02:00)
- Partial verification: verify a random 10% sample daily (statistical integrity check)
- Verification results stored in new `integrity_checks` table:

```sql
CREATE TABLE integrity_checks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    check_type      TEXT NOT NULL,  -- full, sample, single_item
    total_items     INT NOT NULL,
    verified_items  INT NOT NULL,
    failed_items    INT NOT NULL DEFAULT 0,
    chain_valid     BOOLEAN,
    started_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    performed_by    TEXT NOT NULL,
    details         JSONB DEFAULT '{}'
);
```

**Admin notifications:**
- Integrity check complete → info notification with summary
- Hash mismatch found → CRITICAL notification
- TSA token invalid → WARNING notification
- Chain break detected → CRITICAL notification

**Integrity warning flagging (per spec error handling):**
- Hash mismatch → set `evidence_items.metadata.integrity_warning = true`
- UI shows red warning badge on evidence item in grid/detail
- Warning badge tooltip: "INTEGRITY ALERT — stored hash does not match computed hash"
- Item remains accessible (not blocked) but users are informed
- Custody log entry: `action: "integrity_alert"`, `details: {stored_hash, computed_hash, difference}`
- Clear warning: admin can re-upload correct file via versioning → new version gets clean hash

**Endpoint enhancements:**
```
POST /api/cases/:id/verify               → Full verification
POST /api/cases/:id/verify/sample        → Random sample verification (10%)
GET  /api/cases/:id/verify/history        → Past verification results
GET  /api/cases/:id/verify/status         → Current verification job status
```

**Tests:**
- Full verification detects tampered file
- Full verification detects invalid TSA token
- Full verification detects broken custody chain
- Sample verification checks approximately 10% of items
- Scheduled verification triggers at configured time
- Verification results stored correctly
- CRITICAL notification on failure
- Large case (5000 items) → full verification completes within 5 minutes
- Concurrent verification and upload → no conflicts

### Step 6: Annotations Table (Phase 2 Addition)

**Deliverable:** Evidence annotations/comments system.

**Migration 008:** `008_annotations_table.up.sql`
```sql
CREATE TABLE annotations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL,
    case_id         UUID REFERENCES cases(id) NOT NULL,
    user_id         TEXT NOT NULL,
    content         TEXT NOT NULL,
    timestamp_in_media FLOAT,          -- seconds for audio/video
    page_number     INT,                -- for documents
    shared          BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_annotations_evidence ON annotations(evidence_id);
CREATE INDEX idx_annotations_case ON annotations(case_id);
```

**Interface:**
```go
type AnnotationService interface {
    Create(ctx context.Context, input CreateAnnotationInput) (Annotation, error)
    List(ctx context.Context, evidenceID uuid.UUID, pagination Pagination) (PaginatedResult[Annotation], error)
    Update(ctx context.Context, id uuid.UUID, content string) (Annotation, error)
    Delete(ctx context.Context, id uuid.UUID) error
    Share(ctx context.Context, id uuid.UUID) error
}
```

**Endpoints:**
```
POST   /api/evidence/:id/annotations            → Create annotation
GET    /api/evidence/:id/annotations             → List annotations (paginated)
PATCH  /api/annotations/:id                      → Update annotation content
DELETE /api/annotations/:id                      → Delete annotation
POST   /api/annotations/:id/share                → Share annotation with case members
```

**Rules:**
- Annotations are per-user by default (only creator sees them)
- Shared annotations visible to all case members with access to that evidence
- Annotations logged to custody chain (shared annotations only — private notes are not custody events)
- Defence → only sees annotations on disclosed evidence
- Annotations are NOT evidence items — they don't get hashed or timestamped
- Max annotation length: 10,000 characters
- Annotation content sanitized (HTML escaped, XSS prevention)

**Tests:**
- Create annotation → stored correctly
- Annotation with media timestamp → linked to specific time
- Annotation with page number → linked to specific page
- Non-shared annotation → only visible to creator
- Shared annotation → visible to all case members
- Delete annotation → removed (not a custody item itself)
- Annotation on disclosed evidence → visible to defence (if shared)
- Annotation on non-disclosed evidence → invisible to defence

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/evidence/timeline.go` | Create | Timeline query logic |
| `internal/audit/service.go` | Create | Audit aggregation + log queries |
| `internal/audit/handler.go` | Create | Audit API endpoints |
| `internal/integrity/verifier.go` | Modify | Enhanced verification + scheduling |
| `internal/annotations/service.go` | Create | Annotation CRUD |
| `internal/annotations/handler.go` | Create | Annotation endpoints |
| `migrations/008_annotations.up.sql` | Create | Annotations table |
| `migrations/009_integrity_checks.up.sql` | Create | Integrity checks table |
| `web/src/components/timeline/*` | Create | Timeline components |
| `web/src/app/[locale]/admin/audit/*` | Create | Audit dashboard pages |

---

## Definition of Done

- [ ] Timeline displays evidence chronologically with correct grouping
- [ ] Timeline respects role-based access
- [ ] Audit dashboard shows system metrics and activity log
- [ ] Audit log filterable and exportable
- [ ] Integrity verification includes RFC 3161 token verification
- [ ] Custody chain hash verification detects breaks
- [ ] Scheduled verification runs weekly
- [ ] Sample verification runs daily
- [ ] Annotations support media timestamps and page numbers
- [ ] Shared vs private annotations work correctly
- [ ] 100% test coverage on all new code

---

## Security Checklist

- [ ] Audit dashboard restricted to System Admin only
- [ ] Audit log export doesn't include sensitive data
- [ ] Timeline respects classification access rules
- [ ] Annotation content sanitized (XSS prevention)
- [ ] Integrity verification results logged immutably
- [ ] CRITICAL alerts cannot be silenced without acknowledgment

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 11 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `timeline.BuildTimeline` — items sorted by source_date, falls back to uploaded_at when source_date is nil
- `timeline.BuildTimeline` — date range filtering returns only items within from/to bounds
- `timeline.BuildTimeline` — grouping by day/week/month produces correct group keys and item counts
- `timeline.BuildTimeline` — classification filtering applied (defence user gets only disclosed items)
- `timeline.BuildTimeline` — tag filtering returns only items with matching tags
- `timeline.BuildTimeline` — empty date range returns all items, empty result set returns valid empty response
- `audit.GetAuditLog` — pagination correct (page 1 = first N, page 2 = next N), total count accurate
- `audit.GetAuditLog` — filtering by user_id, action, case_id, date range, severity each work independently and combined
- `audit.GetAuditStats` — aggregation returns correct counts for total_events_today, active_users, evidence_uploaded, integrity_warnings, storage_gb
- `audit.GetAuditStats` — non-admin caller returns 403
- `integrity.VerifyFull` — detects tampered file (hash mismatch), detects invalid TSA token, detects broken custody chain link
- `integrity.VerifyFull` — all-clean case returns verified_items = total_items, failed_items = 0
- `integrity.VerifySample` — selects approximately 10% of items randomly, stores results in integrity_checks table
- `integrity.VerifyChainIntegrity` — validates hash-linked sequence, detects inserted/deleted/modified entries
- `integrity.FlagIntegrityWarning` — sets evidence_items.metadata.integrity_warning = true, creates custody log entry
- `integrity.ScheduleVerification` — triggers at configured time (weekly Sunday 02:00), respects timezone
- `annotations.Create` — stores content, links to evidence, sets user_id, validates max 10,000 chars, sanitizes HTML
- `annotations.Create` — media timestamp linked to audio/video, page number linked to document
- `annotations.List` — non-shared annotations visible only to creator, shared annotations visible to all case members
- `annotations.Share` — changes visibility, creates custody log entry (shared annotations only)
- `annotations.Delete` — removes annotation, no custody log (private annotations are not custody events)
- `annotations.Update` — updates content, preserves timestamps, sanitizes input

### Integration Tests (testcontainers)

- Timeline with role-based access: create case with evidence at multiple classification levels, query timeline as defence user — only disclosed items appear, query as prosecutor — all items appear
- Timeline grouping with real data: insert 20 evidence items spanning 3 months, query grouped by month — 3 groups with correct item counts
- Audit log at scale: insert 100,000 audit events, query with pagination — response under 200ms, page size respected, total count correct
- Audit log filtering: insert events from multiple users/actions/cases, filter by each dimension — correct results returned
- Full integrity verification: upload 10 evidence items, tamper with one file in MinIO directly (bypass application), run full verification — detects the tampered file, creates CRITICAL notification, sets integrity_warning flag on the item
- TSA token verification: upload evidence with valid RFC 3161 timestamp, corrupt the TSA token in the database, run verification — detects invalid TSA token, creates WARNING notification
- Custody chain integrity: create evidence with 5 custody events, modify one event's hash directly in database, run verification — detects chain break, creates CRITICAL notification
- Sample verification: create case with 100 evidence items, run sample verification — approximately 10 items checked, results stored in integrity_checks table
- Annotation visibility: create private annotation as user A, query as user A — visible, query as user B — not visible, share annotation, query as user B — now visible
- Annotation on disclosed evidence: create shared annotation on disclosed evidence, query as defence — visible; create shared annotation on non-disclosed evidence, query as defence — not visible

### E2E Automated Tests (Playwright)

- Timeline rendering: navigate to case timeline page, verify evidence items displayed chronologically on horizontal timeline, zoom in/out changes granularity (day/week/month), hover over item shows thumbnail preview, click item navigates to evidence detail
- Timeline filtering: apply date range filter, verify only items within range displayed; apply tag filter, verify filtered; apply classification filter, verify filtered; clear all filters, verify all items restored
- Timeline export: click export as PDF, verify PDF downloaded, open PDF — verify it contains evidence numbers, dates, and descriptions in a print-friendly court-submittable format
- Audit dashboard: log in as admin, navigate to audit dashboard, verify metric cards display (uploads today, active users, storage, backup status), verify integrity status indicator (green checkmark when clean), verify recent activity feed shows last 20 events
- Audit log search: navigate to audit log, filter by user — results scoped to that user, filter by action type — results scoped, filter by date range — results scoped, export as CSV — file downloads with correct data
- Integrity verification: navigate to case, click "Run Integrity Verification," verify progress indicator, wait for completion, verify results page shows total items checked, verified count, and any failures
- Integrity warning display: tamper with a file via API/database, run verification, verify the evidence item in the grid now shows a red warning badge, hover over badge — tooltip shows "INTEGRITY ALERT — stored hash does not match computed hash"
- Annotations: navigate to evidence detail, add annotation with text and media timestamp (for video) or page number (for PDF), verify annotation appears in annotation panel, share annotation, log in as another case member — verify shared annotation visible
- Annotation on video: add annotation at timestamp 1:30, click annotation — verify media player seeks to 1:30

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Navigate to a case with 20+ evidence items spanning several months. Open the Evidence Timeline view.
   **Expected:** A horizontal scrollable timeline displays evidence items positioned by their source_date. Items are color-coded by classification or type. Date axis labels match the zoom level.
   **Verify:** Scroll left/right to pan. Zoom in to day view — items spread out. Zoom out to month view — items clustered. Hover over an item — thumbnail and title appear.

2. [ ] **Action:** Click on an evidence item in the timeline.
   **Expected:** Navigation to the evidence detail page for that item.
   **Verify:** The evidence detail page loads with the correct item. Back button returns to the timeline at the same scroll position.

3. [ ] **Action:** Apply filters: set date range to a specific month, select "Confidential" classification, and add a tag filter.
   **Expected:** Timeline updates to show only items matching all three filters. Item count in the header updates.
   **Verify:** Each visible item falls within the date range, has "Confidential" classification, and has the specified tag.

4. [ ] **Action:** Click "Export Timeline as PDF."
   **Expected:** A PDF downloads containing the timeline data in a print-friendly tabular format suitable for court submission. Includes evidence numbers, dates, titles, and classifications.
   **Verify:** Open the PDF — data matches what was displayed on screen. Format is clean and professional.

5. [ ] **Action:** Log in as System Admin. Navigate to the Audit Dashboard.
   **Expected:** Dashboard shows metric cards: total events today, active users today, evidence uploaded today, integrity warnings (0 if clean), last backup time/status, cases on legal hold, total evidence items, total storage.
   **Verify:** Cross-reference one metric (e.g., evidence uploaded today) with a manual count from the evidence grid. Numbers match.

6. [ ] **Action:** Navigate to the Audit Log. Filter by a specific user, then by action type "uploaded," then by a date range.
   **Expected:** Log table shows only entries matching all active filters. Each row shows: timestamp, user, action, case, evidence number, IP, severity (color-coded).
   **Verify:** Remove one filter — more results appear. Export as CSV — file contains the filtered data.

7. [ ] **Action:** Navigate to a case with evidence. Click "Run Integrity Verification."
   **Expected:** Progress indicator appears showing items being checked. Upon completion, results summary shows: total items, verified count, failed count (0 if clean), chain status.
   **Verify:** All items show "verified." Chain integrity states "unbroken hash-linked sequence." Integrity status on audit dashboard shows green checkmark.

8. [ ] **Action:** Directly modify a file in MinIO storage (bypass the application) to simulate tampering. Run integrity verification again.
   **Expected:** Verification detects the tampered file. Results show 1 failed item. A CRITICAL notification is generated.
   **Verify:** The tampered evidence item now shows a red warning badge in the evidence grid. Hover over badge — tooltip reads "INTEGRITY ALERT — stored hash does not match computed hash." Custody chain shows an "integrity_alert" entry with stored_hash and computed_hash values.

9. [ ] **Action:** Navigate to an audio/video evidence item. Add an annotation: type "Witness identifies suspect at this point" and set the media timestamp to 2:45.
   **Expected:** Annotation appears in the annotation panel linked to timestamp 2:45. Annotation is private (only visible to you).
   **Verify:** Click the annotation — media player seeks to 2:45. Log in as another user — annotation is not visible.

10. [ ] **Action:** Share the annotation from step 9. Then log in as another case member.
    **Expected:** The shared annotation is now visible to all case members with access to that evidence item. Custody chain shows a "shared" event for the annotation.
    **Verify:** The other case member sees the annotation with the correct text and timestamp link. Defence users only see it if the evidence is disclosed to them.
