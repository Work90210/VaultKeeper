# Sprint 5: Search, Notifications & Health Endpoints

**Phase:** 1 — Foundation
**Duration:** Weeks 9-10
**Goal:** Implement full-text search via Meilisearch, in-app + email notification system, and health monitoring endpoints. These complete the core user experience for daily evidence work.

---

## Prerequisites

- Sprint 4 complete (evidence upload, MinIO storage, evidence indexed in Meilisearch)
- SMTP configuration available for email notifications
- All evidence items indexed on upload

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Meilisearch Integration (`internal/search/meilisearch.go`)

**Deliverable:** Search indexing and query service.

**Interface:**
```go
type SearchService interface {
    IndexEvidence(ctx context.Context, item EvidenceItem) error
    RemoveEvidence(ctx context.Context, id uuid.UUID) error
    Search(ctx context.Context, query SearchQuery) (SearchResult, error)
    ReindexAll(ctx context.Context, caseID uuid.UUID) error
}

type SearchQuery struct {
    Query          string
    CaseID         *uuid.UUID    // scope to specific case
    MimeTypes      []string
    Tags           []string
    Classifications []string
    DateFrom       *time.Time
    DateTo         *time.Time
    Limit          int           // default 50, max 200
    Offset         int
}

type SearchResult struct {
    Hits       []SearchHit
    TotalHits  int
    Query      string
    ProcessingTimeMs int
}

type SearchHit struct {
    EvidenceID    uuid.UUID
    CaseID        uuid.UUID
    Title         string
    Description   string
    EvidenceNumber string
    Highlights    map[string][]string  // field → highlighted snippets
    Score         float64
}
```

**Meilisearch index configuration:**
- Index name: `evidence`
- Searchable attributes: `title`, `description`, `tags`, `evidence_number`, `source`, `file_name`
- Filterable attributes: `case_id`, `mime_type`, `classification`, `tags`, `source_date`, `uploaded_at`
- Sortable attributes: `uploaded_at`, `source_date`, `evidence_number`
- Ranking rules: default Meilisearch ranking (typo tolerance, word proximity)
- Distinct attribute: none (show all versions)
- Stop words: configured per language (English default)
- Synonyms: configurable per institution (e.g., "evidence" = "exhibit")
- Typo tolerance: enabled (default Meilisearch settings)
- Pagination: offset-based (Meilisearch native, cursor conversion at API layer)
- Index refresh: synchronous on single-item updates, async batch for bulk/migration
- Facets: `mime_type`, `classification`, `tags` (for filter counts in UI)

**Document format for indexing:**
```json
{
    "id": "uuid",
    "case_id": "uuid",
    "title": "witness statement transcription",
    "description": "...",
    "evidence_number": "ICC-UKR-2024-00001",
    "tags": ["witness", "transcription"],
    "source": "Field Office Kyiv",
    "file_name": "statement_w001.pdf",
    "mime_type": "application/pdf",
    "classification": "confidential",
    "source_date": "2024-03-15T00:00:00Z",
    "uploaded_at": "2026-04-06T10:30:00Z",
    "is_current": true
}
```

**Security: Role-based search filtering:**
- Search results MUST be filtered by user's case access
- Defence users: results further filtered to disclosed evidence only
- Implementation: add `case_id` filter based on user's case_roles
- NEVER return search results for cases the user isn't assigned to

**Tests:**
- Index evidence → searchable in Meilisearch
- Search by title → matching results returned
- Search by description → matching results returned
- Search by tag → filtered results
- Search with typo → still finds results (Meilisearch typo tolerance)
- Search scoped to case → only that case's evidence
- Search with MIME type filter → correct filtering
- Search with date range → correct filtering
- Search with classification filter → correct filtering
- Empty query → returns recent items (browsing mode)
- Remove evidence → no longer in search results
- Reindex → all case evidence fresh in Meilisearch
- Role filtering: user can only see results from assigned cases
- Defence user → only disclosed evidence in results
- Large result set → pagination works
- Meilisearch down → graceful error, API returns 503

### Step 2: Search Endpoint

**Deliverable:** `GET /api/search` with full-text search + filters.

**Query parameters:**
```
GET /api/search?q=witness+statement&case_id=uuid&type=application/pdf&tag=witness&classification=confidential&from=2024-01-01&to=2024-12-31&limit=50&offset=0
```

**Response:**
```json
{
    "data": {
        "hits": [...],
        "total_hits": 142,
        "query": "witness statement",
        "processing_time_ms": 12
    },
    "error": null
}
```

**Rate limit:** 30 requests/minute per user (enforced at Caddy level).

**Tests:**
- All query params work independently and in combination
- Empty query → recent items
- Invalid params → 400 with specific error
- Rate limit exceeded → 429
- Results respect role-based access

### Step 3: Search Frontend

**Deliverable:** Search UI with instant results and faceted filters.

**Components:**
- `SearchBar` — Global search input in header/sidebar
  - Debounced input (300ms)
  - Search suggestions (recent queries)
  - Clear button
- `SearchResults` — Results page with filters
  - Result cards with highlighted snippets
  - File type icon
  - Case name + evidence number
  - Source date
  - Click to navigate to evidence detail
- `SearchFilters` — Sidebar filter panel
  - Case filter (dropdown of user's cases)
  - File type filter (checkboxes: documents, images, video, audio, other)
  - Classification filter
  - Date range picker
  - Tag filter (multi-select)
- `SearchEmpty` — Empty state when no results

**Tests:**
- Search input triggers query after debounce
- Results render with highlights
- Filters update results
- Click result → navigate to evidence detail
- Empty state renders when no results
- Loading state shows skeleton

### Step 4: Notification Service (`internal/notifications/service.go`)

**Deliverable:** In-app and email notification dispatch.

**Interface:**
```go
type NotificationService interface {
    Notify(ctx context.Context, event NotificationEvent) error
    GetUserNotifications(ctx context.Context, userID string, pagination Pagination) (PaginatedResult[Notification], error)
    MarkRead(ctx context.Context, id uuid.UUID, userID string) error
    MarkAllRead(ctx context.Context, userID string) error
    GetUnreadCount(ctx context.Context, userID string) (int, error)
}

type NotificationEvent struct {
    Type    string    // evidence_uploaded, user_added_to_case, integrity_warning, legal_hold_changed, retention_expiring, backup_failed
    CaseID  uuid.UUID
    Title   string
    Body    string
    // Recipients determined by event type + case roles
}
```

**Event routing (who gets notified):**

| Event | Recipients |
|-------|-----------|
| evidence_uploaded | All users assigned to the case |
| user_added_to_case | The added user |
| integrity_warning | System admins + case admins for the case |
| legal_hold_changed | All users assigned to the case |
| retention_expiring | System admins |
| backup_failed | System admins |

**Email notifications:**
- Sent via SMTP (configurable, optional)
- HTML + plaintext multipart
- Respect user preferences (if email notifications disabled, skip)
- Queue emails, don't block the main request
- Include case reference code and action summary
- NEVER include evidence content or witness identities in email body
- Email template: simple, professional, no tracking pixels

**Implementation details:**
- Notifications created in Postgres (notifications table)
- Email dispatch runs as a background goroutine
- Failed email delivery → log error, notification still saved in-app
- No external notification services — just SMTP

**Tests:**
- Evidence upload → notification created for all case members
- User added to case → notification created for that user
- Integrity warning → notification for admins
- MarkRead → notification.read = true
- MarkAllRead → all user notifications marked read
- GetUnreadCount → correct count
- Pagination works
- Email sent when SMTP configured
- Email not sent when SMTP not configured (no error)
- Email delivery failure → logged, in-app notification still created
- Email body never contains sensitive data
- Concurrent notifications don't deadlock

### Step 5: Notification Endpoints

**Deliverable:** API endpoints for notification management.

```
GET    /api/notifications                → List user's notifications (paginated)
PATCH  /api/notifications/:id/read       → Mark notification as read
POST   /api/notifications/read-all       → Mark all as read
GET    /api/notifications/unread-count    → Get unread count
```

**Tests:**
- List returns only current user's notifications
- Pagination correct
- Mark read → 200
- Mark other user's notification → 403
- Unread count accurate after mark read

### Step 6: Notification Frontend

**Deliverable:** Notification bell + dropdown + page.

**Components:**
- `NotificationBell` — Header icon with unread count badge
  - Polls unread count every 30 seconds (or WebSocket in future)
  - Badge shows count (max "99+")
- `NotificationDropdown` — Quick view of recent notifications
  - Last 5 notifications
  - Click to navigate to related case/evidence
  - "Mark all read" button
  - "View all" link to full page
- `NotificationList` — Full notification page
  - All notifications, paginated
  - Filter: unread/all
  - Timestamp relative ("2 hours ago")
  - Event type icon

**Tests:**
- Bell shows unread count
- Dropdown renders recent notifications
- Click notification → navigates to case
- Mark read → badge count decreases
- Empty state when no notifications

### Step 7: Health Endpoints

**Deliverable:** Public + authenticated health check endpoints.

**Public: `GET /health` (no auth)**
```json
{
    "status": "healthy",
    "version": "1.0.0"
}
```
- Returns "healthy" only if ALL critical services are reachable
- Returns "unhealthy" if any critical service is down
- NEVER exposes internal details (evidence count, disk usage, etc.)
- Rate limit: none (Uptime Kuma pings every 60 seconds)

**Detailed: `GET /api/health` (System Admin auth)**
```json
{
    "status": "healthy",
    "version": "1.0.0",
    "postgres": "connected",
    "minio": "connected",
    "meilisearch": "connected",
    "keycloak": "connected",
    "last_backup": "2026-04-05T03:00:00Z",
    "backup_status": "completed",
    "evidence_count": 4521,
    "disk_usage_percent": 42,
    "uptime_seconds": 86400
}
```

**Health check caching (per spec — error handling for Postgres down):**
- Cache last health check result for 60 seconds
- If Postgres is down and cache is warm → return cached status for up to 60 seconds
- After 60 seconds stale cache → return "unhealthy"
- Cache is per-instance in-memory (no external dependency for health caching)
- This prevents cascading failures when monitoring hammers /health during an outage

**Health check implementation:**
- Postgres: `SELECT 1` with 5-second timeout
- MinIO: `BucketExists("evidence")` with 5-second timeout
- Meilisearch: `GET /health` with 5-second timeout
- Keycloak: `GET /realms/vaultkeeper` with 5-second timeout
- Each check runs in parallel goroutine
- Overall status: "healthy" if all pass, "degraded" if non-critical fails, "unhealthy" if critical fails
- Critical services: Postgres, MinIO
- Non-critical services: Meilisearch, Keycloak (cached JWTs still work)

**Tests:**
- All services up → "healthy"
- Postgres down → "unhealthy"
- MinIO down → "unhealthy"
- Meilisearch down → "degraded"
- Keycloak down → "degraded"
- Public endpoint → no internal details
- Detailed endpoint without auth → 401
- Detailed endpoint with non-admin → 403
- Health check timeout → "unhealthy" for that service
- Version string from build-time injection
- Health cache: Postgres down → cached result served for 60 seconds
- Health cache: cache expired + Postgres down → "unhealthy"
- Health cache: Postgres recovers → fresh result served immediately

### Step 8: Integrity Verification Endpoints

**Deliverable:** Verify evidence integrity by re-hashing.

**Endpoints:**
```
POST /api/cases/:id/verify            → Start async verification job
GET  /api/cases/:id/verify/status     → Check verification status
```

**Verification flow:**
1. Start background job (goroutine with context)
2. For each evidence item in case (current versions only):
   a. Read file from MinIO
   b. Compute SHA-256 hash
   c. Compare against stored hash in evidence_items
   d. Verify RFC 3161 timestamp token (if present)
3. Report results: total items, verified items, mismatches, missing files

**Mismatch handling (per spec):**
- Flag evidence item with `integrity_warning` in metadata
- Send CRITICAL admin notification
- Custody log entry: `"INTEGRITY ALERT — stored hash does not match computed hash"`
- Item remains accessible but UI shows warning badge

**Tests:**
- All items match → verification passes
- Tampered file (modified in MinIO) → mismatch detected
- Missing file (deleted from MinIO) → flagged
- TSA token verification → valid/invalid
- Large case (1000 items) → completes within timeout
- Concurrent verification jobs → don't interfere
- Status endpoint → returns progress during job

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/search/meilisearch.go` | Create | Search indexing + queries |
| `internal/search/handler.go` | Create | Search endpoint handler |
| `internal/notifications/service.go` | Create | Notification dispatch logic |
| `internal/notifications/repository.go` | Create | Notification Postgres queries |
| `internal/notifications/handler.go` | Create | Notification API handlers |
| `internal/notifications/email.go` | Create | SMTP email dispatch |
| `internal/server/health.go` | Create | Health check handlers |
| `internal/integrity/verifier.go` | Create | Evidence integrity verification |
| `web/src/components/search/*` | Create | Search UI components |
| `web/src/components/notifications/*` | Create | Notification UI components |

---

## Definition of Done

- [ ] Full-text search returns relevant results with highlighting
- [ ] Search filtered by user's case access (role-based)
- [ ] Defence users only see disclosed evidence in search
- [ ] All evidence items indexed on upload
- [ ] In-app notifications for all specified events
- [ ] Email notifications sent via SMTP when configured
- [ ] Notification bell shows unread count
- [ ] Public /health endpoint returns status + version only
- [ ] Detailed /api/health returns all service statuses
- [ ] Integrity verification detects tampered files
- [ ] Hash mismatch triggers CRITICAL notification
- [ ] 100% test coverage on all Go packages
- [ ] Search UI responsive and instant (<300ms)
- [ ] All UI strings in i18n files

---

## Security Checklist

- [ ] Search results filtered by user's case roles
- [ ] Defence users never see undisclosed evidence in search
- [ ] Public health endpoint exposes no internal details
- [ ] Email notifications never contain evidence content or witness identities
- [ ] Notification endpoints scoped to current user only
- [ ] Search queries sanitized (no injection into Meilisearch)
- [ ] SMTP credentials not logged
- [ ] Integrity warnings logged to custody chain with CRITICAL severity
- [ ] Health check timeouts prevent hanging

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 5 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/search/meilisearch.go`**: IndexEvidence adds document to Meilisearch, RemoveEvidence removes document, Search by title returns matches, Search by description returns matches, Search by tag filters correctly, Search with typo returns results (typo tolerance), Search scoped to case returns only that case's results, Search with MIME type filter, Search with date range filter, Search with classification filter, empty query returns recent items, role filtering (user sees only assigned case results), defence user sees only disclosed evidence, Meilisearch down returns 503, pagination offset/limit work, facets return correct counts
- **`internal/search/handler.go`**: All query params work independently and in combination, empty query returns recent items, invalid params return 400, results respect role-based access, rate limit exceeded returns 429
- **`internal/notifications/service.go`**: Evidence upload creates notification for all case members, user added to case creates notification for that user, integrity warning creates notification for admins, MarkRead sets notification.read to true, MarkAllRead marks all user notifications, GetUnreadCount returns correct count, pagination works, email sent when SMTP configured, email not sent when SMTP not configured (no error), email delivery failure logged but in-app notification still created, email body never contains evidence content or witness identity, concurrent notifications do not deadlock
- **`internal/notifications/email.go`**: HTML + plaintext multipart email generated, email includes case reference code and action summary, SMTP credentials not logged, email template renders correctly, failed SMTP connection handled gracefully
- **`internal/notifications/handler.go`**: List returns only current user's notifications, pagination correct, mark read returns 200, mark other user's notification returns 403, unread count accurate after mark read
- **`internal/server/health.go`**: All services up returns "healthy", Postgres down returns "unhealthy", MinIO down returns "unhealthy", Meilisearch down returns "degraded", Keycloak down returns "degraded", public endpoint returns no internal details, detailed endpoint without auth returns 401, detailed endpoint with non-admin returns 403, timeout on health check returns "unhealthy" for that service, version string from build-time injection, health cache serves cached result for 60 seconds when Postgres down, expired cache + Postgres down returns "unhealthy", Postgres recovery returns fresh result immediately
- **`internal/integrity/verifier.go`**: All items match passes verification, tampered file detected (modified in MinIO), missing file flagged, TSA token verification valid/invalid, large case (1000 items) completes within timeout, concurrent verification jobs don't interfere, status endpoint returns progress, mismatch triggers CRITICAL notification and custody log entry

### Integration Tests (with testcontainers)

- **Meilisearch indexing (testcontainers/meilisearch:v1.7)**: Index 100 evidence items, search returns relevant results with highlights, faceted filters return correct counts, remove item and verify no longer searchable, reindex case and verify all items fresh, typo tolerance works
- **Meilisearch + Postgres role filtering (testcontainers/meilisearch + postgres)**: Index evidence from 3 cases, search as user assigned to 1 case returns only that case's results, search as system_admin returns all, search as defence returns only disclosed items
- **SMTP email dispatch (testcontainers/mailhog or greenmail)**: Notification triggers email to case members, email contains case reference and action summary, email does not contain evidence content or witness identities, HTML and plaintext parts both present, failed SMTP does not block in-app notification creation
- **Postgres notifications (testcontainers/postgres:16-alpine)**: Notification CRUD with correct user scoping, unread count query uses partial index (EXPLAIN ANALYZE on idx_notifications_user_unread), mark read and mark all read
- **Health check with service failures (testcontainers: postgres + minio + meilisearch)**: All services healthy returns correct detailed response, stop Postgres container and verify "unhealthy" status, stop Meilisearch and verify "degraded", health cache behavior under service outage, parallel health checks complete within 5-second timeout
- **Integrity verification (testcontainers: postgres + minio)**: Upload 10 items, run verification and verify all pass, tamper with one file in MinIO, run verification and verify mismatch detected with correct item identified, verify custody log entry and CRITICAL notification created

### E2E Automated Tests (Playwright)

- **`tests/e2e/search/search-basic.spec.ts`**: Login, type a search query in the global search bar, verify results appear after 300ms debounce, verify result cards show highlighted snippets with file type icon, case name, and evidence number, click a result and verify navigation to evidence detail
- **`tests/e2e/search/search-filters.spec.ts`**: Perform a search, apply case filter (select a specific case), verify results narrow to that case only, apply file type filter (e.g., "documents"), verify only PDFs/DOCs shown, apply date range filter, verify results within range, apply classification filter, apply tag filter, verify all filters combine correctly
- **`tests/e2e/search/search-empty.spec.ts`**: Search for a query with no matching results, verify empty state renders with helpful message, verify filters are still interactive
- **`tests/e2e/notifications/notification-bell.spec.ts`**: Login, verify notification bell in header shows unread count badge, trigger an event (e.g., upload evidence in another session/tab), verify badge count increments (within 30 seconds polling), click bell, verify dropdown shows recent notifications
- **`tests/e2e/notifications/notification-actions.spec.ts`**: Click a notification in the dropdown, verify navigation to the related case/evidence, click "Mark all read", verify badge count goes to zero, navigate to full notifications page, verify all notifications listed with pagination
- **`tests/e2e/notifications/email-notification.spec.ts`**: (Requires mailhog/test SMTP) Trigger an evidence upload, verify email received by case members in mailhog, verify email subject contains case reference, verify email body does not contain evidence file content or witness names
- **`tests/e2e/health/health-public.spec.ts`**: Navigate to `/health` (no auth), verify JSON response with `"status"` and `"version"` only, verify no internal details (evidence count, disk usage) exposed
- **`tests/e2e/health/health-detailed.spec.ts`**: Login as system_admin, call `GET /api/health`, verify response includes postgres/minio/meilisearch/keycloak statuses, evidence_count, disk_usage_percent, last_backup, uptime_seconds
- **`tests/e2e/integrity/verify-case.spec.ts`**: Login as system_admin, navigate to case settings, click "Verify Integrity", verify progress indicator appears, wait for completion, verify all items show "verified" status, verify custody log shows verification event

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Login, navigate to the global search bar in the header, type "witness statement"
   **Expected:** Results appear within 300ms showing matching evidence items with highlighted text snippets
   **Verify:** Results show correct case names, evidence numbers, and file type icons; clicking a result navigates to the evidence detail page

2. [ ] **Action:** On the search results page, apply the case filter to a specific case
   **Expected:** Results narrow to only evidence from that case
   **Verify:** All result items show the selected case name; removing the filter restores full results

3. [ ] **Action:** Apply multiple filters simultaneously: file type "documents" + classification "confidential" + date range last 30 days
   **Expected:** Results show only documents classified as confidential within the date range
   **Verify:** Result count decreases with each filter applied; clearing all filters restores full results

4. [ ] **Action:** Search as a defence role user
   **Expected:** Only evidence items that have been disclosed to the defence appear in results
   **Verify:** Search for a known undisclosed evidence title — zero results; search for a disclosed item — found correctly

5. [ ] **Action:** Check the notification bell icon in the header after logging in
   **Expected:** Bell shows a badge with the unread notification count (or no badge if zero unread)
   **Verify:** Click the bell, dropdown shows the 5 most recent notifications with timestamps and event descriptions

6. [ ] **Action:** In a second browser/tab, upload a new evidence item to a case you are assigned to
   **Expected:** Within 30 seconds, the notification bell badge count increments in the first tab
   **Verify:** Click the bell, new "evidence_uploaded" notification appears at the top of the dropdown with the correct case reference

7. [ ] **Action:** Click a notification in the dropdown
   **Expected:** Navigation to the related case or evidence detail page
   **Verify:** The notification is marked as read (badge count decreases); returning to the dropdown, the notification appears in read state

8. [ ] **Action:** Navigate to the full notifications page, click "Mark all read"
   **Expected:** All notifications marked as read, badge count goes to zero
   **Verify:** Refresh the page, all notifications still show as read; badge remains at zero

9. [ ] **Action:** (With SMTP configured and test mailbox) Upload evidence to a case with 3 assigned users
   **Expected:** All 3 users receive an email notification about the upload
   **Verify:** Email contains case reference code and action summary ("New evidence uploaded: {title}"); email does NOT contain file content, hashes, or witness identities

10. [ ] **Action:** Navigate to `/health` in a browser (no authentication)
    **Expected:** JSON response with only `"status"` and `"version"` fields
    **Verify:** No evidence count, disk usage, service names, or internal details visible; response is "healthy" when all services running

11. [ ] **Action:** Login as system_admin, call `GET /api/health` via the admin dashboard or curl
    **Expected:** Detailed response showing postgres, minio, meilisearch, keycloak statuses, plus evidence_count, disk_usage_percent, last_backup, uptime_seconds
    **Verify:** Each service shows "connected"; try stopping Meilisearch container, re-check — status should show "degraded" with meilisearch "disconnected"

12. [ ] **Action:** Login as system_admin, navigate to a case, trigger "Verify Integrity"
    **Expected:** Progress indicator shows verification running (X/total items verified), completes with all items passing
    **Verify:** Custody log shows "integrity_verification" entry; no integrity warnings or CRITICAL notifications (unless a file was tampered with)

13. [ ] **Action:** (Advanced) Manually modify a file directly in MinIO (bypass the application), then trigger integrity verification
    **Expected:** Verification detects the tampered file, flags it with integrity warning, sends CRITICAL notification to admins
    **Verify:** Custody log shows "INTEGRITY ALERT" entry for the specific evidence item; notification bell shows critical notification; evidence detail page shows warning badge on the affected item

14. [ ] **Action:** Login as non-admin user, attempt to access `GET /api/health` (detailed endpoint)
    **Expected:** 403 Forbidden response
    **Verify:** Response body is standard error envelope with no internal health details leaked
