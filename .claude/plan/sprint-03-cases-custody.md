# Sprint 3: Cases CRUD & Custody Logging Middleware

**Phase:** 1 — Foundation
**Duration:** Weeks 5-6
**Goal:** Implement the full cases domain (CRUD, archive, legal hold) and the custody logging middleware that wraps every mutation. This sprint delivers the first user-visible business logic.

---

## Prerequisites

- Sprint 2 complete (JWT auth, role enforcement, API response envelope)
- Postgres with cases, case_roles, custody_log tables
- Auth middleware injecting AuthContext into request context

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Cases Repository (`internal/cases/repository.go`)

**Deliverable:** Data access layer for cases table with parameterized queries.

**Interface:**
```go
type Repository interface {
    Create(ctx context.Context, c Case) (Case, error)
    FindByID(ctx context.Context, id uuid.UUID) (Case, error)
    FindAll(ctx context.Context, filter CaseFilter, pagination Pagination) ([]Case, int, error)
    Update(ctx context.Context, id uuid.UUID, updates CaseUpdate) (Case, error)
    Archive(ctx context.Context, id uuid.UUID, archivedBy string) error
    SetLegalHold(ctx context.Context, id uuid.UUID, hold bool, setBy string) error
}
```

**CaseFilter:**
```go
type CaseFilter struct {
    UserID      string     // Filter cases by user's case_roles (JOIN)
    Status      []string   // active, closed, archived
    Jurisdiction string
    SearchQuery  string    // ILIKE on title, reference_code, description
}
```

**Pagination (cursor-based):**
```go
type Pagination struct {
    Limit  int       // default 50, max 200
    Cursor string    // base64-encoded UUID of last item
}

type PaginatedResult[T any] struct {
    Items      []T
    TotalCount int
    NextCursor string
    HasMore    bool
}
```

**Cursor-based pagination (used by ALL list endpoints throughout the application):**
```go
type Pagination struct {
    Limit  int    // default 50, max 200
    Cursor string // base64url-encoded UUID of last item
}

type PaginatedResult[T any] struct {
    Items      []T    `json:"items"`
    TotalCount int    `json:"total_count"`
    NextCursor string `json:"next_cursor"`  // empty if no more items
    HasMore    bool   `json:"has_more"`
}
```

**Applied to (per spec):**
- `GET /api/cases` — case list
- `GET /api/cases/:id/evidence` — evidence grid (the big one)
- `GET /api/evidence/:id/custody` — custody log per item
- `GET /api/cases/:id/custody` — full case custody log
- `GET /api/cases/:id/disclosures` — disclosures
- `GET /api/search` — search results
- `GET /api/notifications` — notification feed
- `GET /api/audit` — system audit log

**Cursor encoding:** base64url(last_item_UUID) — no offset-based pagination (per spec: "offset breaks when items are added during pagination")

**Key rules:**
- All queries use parameterized statements (`pgx` handles this natively)
- `FindAll` JOINs with `case_roles` to return only cases the user is assigned to
- System admins bypass the JOIN (see all cases)
- Cursor decoded server-side, validated as UUID
- Invalid cursor → 400 error, not SQL injection vector
- Limit capped at 200 (request for limit > 200 → silently cap to 200)
- Default limit: 50 when not specified

**Tests (100% coverage):**
- Create case → returns case with generated UUID and created_at
- Create case with duplicate reference_code → unique constraint error
- FindByID → returns correct case
- FindByID with non-existent ID → not found error
- FindAll with user filter → only returns assigned cases
- FindAll with system admin → returns all cases
- FindAll with status filter → correct filtering
- FindAll with search query → ILIKE matching
- FindAll pagination → cursor works, has_more correct, total_count accurate
- FindAll pagination → items added mid-pagination don't cause duplicates or skips
- FindAll pagination → cursor from deleted item → graceful handling (skip to next valid)
- FindAll with invalid cursor → 400 error
- FindAll with limit > 200 → capped to 200
- FindAll with limit = 0 → uses default 50
- FindAll with negative limit → 400 error
- Update case → returns updated case, created_at unchanged
- Update non-existent case → not found error
- Archive case → status set to 'archived'
- Archive already archived case → idempotent
- SetLegalHold → legal_hold boolean toggled
- SQL injection via filter fields → blocked by parameterized queries
- Connection pool exhaustion → timeout error, not crash

### Step 2: Cases Service (`internal/cases/service.go`)

**Deliverable:** Business logic layer enforcing invariants.

**Interface:**
```go
type Service interface {
    CreateCase(ctx context.Context, input CreateCaseInput) (Case, error)
    GetCase(ctx context.Context, id uuid.UUID) (Case, error)
    ListCases(ctx context.Context, filter CaseFilter, pagination Pagination) (PaginatedResult[Case], error)
    UpdateCase(ctx context.Context, id uuid.UUID, input UpdateCaseInput) (Case, error)
    ArchiveCase(ctx context.Context, id uuid.UUID) error
    SetLegalHold(ctx context.Context, id uuid.UUID, hold bool) error
    ExportCase(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) // ZIP stream
}
```

**Business rules:**
- `reference_code` validated against configurable regex: `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`
- `title` required, max 500 characters
- `description` max 10,000 characters
- `jurisdiction` max 200 characters
- `status` transitions: active → closed → archived (no backwards)
- Cannot archive a case with `legal_hold = true`
- Cannot delete evidence in a case with `legal_hold = true`
- `created_by` set from AuthContext, immutable
- `metadata` JSONB validated as valid JSON, max 1MB

**Tests (100% coverage):**
- Valid input → case created successfully
- Invalid reference_code format → validation error with specific message
- Empty title → validation error
- Title exceeding 500 chars → validation error
- Invalid status transition (archived → active) → error
- Archive with legal_hold → error with clear message
- SetLegalHold → custody_log entry created (tested via mock)
- ExportCase → ZIP contains metadata JSON + custody log CSV
- Input sanitization: HTML in title stripped via html.EscapeString
- Metadata exceeding 1MB → error

### Step 3: Cases Handler (`internal/cases/handler.go`)

**Deliverable:** HTTP handlers wired to chi router with auth middleware.

**Endpoints:**
```
POST   /api/cases                    → Create case (Case Admin+)
GET    /api/cases                    → List cases (filtered by role)
GET    /api/cases/:id                → Get case detail
PATCH  /api/cases/:id                → Update case metadata (Case Admin+)
POST   /api/cases/:id/archive        → Archive case (Case Admin+)
POST   /api/cases/:id/legal-hold     → Set/release legal hold (Case Admin+)
GET    /api/cases/:id/export          → Export full case as ZIP
```

**Request parsing:**
- JSON body parsed with `json.Decoder` (limit body size to 1MB)
- Path params via `chi.URLParam`
- Query params for filtering and pagination
- All input validated before passing to service

**Error mapping:**
- Service validation errors → 400
- Not found → 404
- Unique constraint violation → 409
- Permission errors → 403
- Internal errors → 500 (generic message, log details)

**Tests (100% coverage):**
- Each endpoint with valid input → correct status code + response body
- Each endpoint with invalid input → 400 with descriptive error
- Each endpoint with wrong role → 403
- Large request body → 413
- Invalid JSON → 400
- Missing required fields → 400 with field-specific error
- Non-existent case_id → 404
- Integration test: full CRUD lifecycle

### Step 4: Case Roles Management

**Deliverable:** Endpoints to assign/revoke case roles.

**Endpoints:**
```
POST   /api/cases/:id/roles           → Assign user to case with role (Case Admin+)
DELETE /api/cases/:id/roles/:userId    → Remove user from case (Case Admin+)
GET    /api/cases/:id/roles            → List case role assignments
```

**Input validation:**
- `user_id` must be a valid Keycloak user (verify via Keycloak admin API)
- `role` must be one of: investigator, prosecutor, defence, judge, observer, victim_representative
- Cannot assign multiple roles to same user in same case (UNIQUE constraint)
- Cannot remove your own case role (prevent lockout)
- System admins can manage roles on any case

**Custody logging for role changes:**
- Role granted → custody_log entry: `action: "role_granted"`, `details: {user_id, role, granted_by}`
- Role revoked → custody_log entry: `action: "role_revoked"`, `details: {user_id, role, revoked_by}`

**Tests:**
- Assign valid role → 201, custody_log entry created
- Assign duplicate role → 409
- Assign invalid role → 400
- Assign to non-existent user → 400 (verified via Keycloak)
- Revoke role → 200, custody_log entry created
- Revoke own role → 403
- List roles → returns all assignments for case
- Non-Case Admin → 403

### Step 5: Custody Logging Middleware (`internal/custody/logger.go`)

**Deliverable:** Middleware that automatically logs every mutation to the custody_log table.

**Architecture:**
- Chi middleware wrapping all case-scoped routes
- Intercepts AFTER the handler executes (response already committed)
- Captures: user_id, case_id, evidence_id (if applicable), action, details, IP, timestamp
- Computes hash chain: `SHA256(previous_entry_hash + current_entry_data)`

**Hash chain specification (tamper-evident linked list):**

Each custody_log entry includes `previous_log_hash` — the SHA-256 hash of the preceding entry. This creates a linked chain where modifying any historical entry breaks the chain from that point forward.

**Hash computation algorithm:**
```go
func ComputeLogHash(prev string, entry CustodyLogEntry) string {
    // Canonical serialization: pipe-delimited, UTC timestamps, sorted JSON details
    data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
        prev,                                              // previous entry hash (empty string for first entry)
        entry.ID.String(),                                 // entry UUID
        entry.UserID,                                      // actor
        entry.Action,                                      // action type
        entry.Timestamp.UTC().Format(time.RFC3339Nano),    // timestamp in UTC
        entry.EvidenceID.String(),                         // evidence UUID (empty if N/A)
        entry.CaseID.String(),                             // case UUID
        canonicalJSON(entry.Details),                       // deterministic JSON (sorted keys)
    )
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// canonicalJSON produces deterministic JSON with sorted keys
func canonicalJSON(v any) string { /* json.Marshal with sorted map keys */ }
```

**Chain rules:**
- First entry in a case: `previous_log_hash = ""` (empty string)
- Every subsequent entry: `previous_log_hash = SHA256(previous_entry_canonical_form)`
- The hash includes ALL fields that make the entry unique — modifying any field changes the hash
- Verification: walk the chain from first to last, recompute each hash, compare against stored hash
- Break detection: if `recomputed_hash != stored_previous_log_hash` for any entry, the chain is broken at that point
```

**Action mapping (what gets logged):**

| HTTP Method + Path | Action | Details |
|-------------------|--------|---------|
| POST /api/cases | case_created | {reference_code, title} |
| PATCH /api/cases/:id | case_updated | {changed_fields} |
| POST /api/cases/:id/archive | case_archived | {} |
| POST /api/cases/:id/legal-hold | legal_hold_set / legal_hold_released | {previous_value} |
| POST /api/cases/:id/roles | role_granted | {user_id, role} |
| DELETE /api/cases/:id/roles/:userId | role_revoked | {user_id, role} |
| GET /api/evidence/:id/download | evidence_downloaded | {file_hash} |
| POST /api/cases/:id/evidence | evidence_uploaded | {evidence_number, file_hash} |

**GET requests that are logged** (viewing is a custody event):
- `GET /api/evidence/:id` → "viewed" (metadata)
- `GET /api/evidence/:id/download` → "downloaded" (file access)

**GET requests NOT logged:**
- `GET /api/cases` → listing cases is not a custody event
- `GET /api/cases/:id/evidence` → listing is not viewing individual items
- `GET /health` → monitoring

**Concurrency:** Hash chain requires sequential ordering. Use a Postgres advisory lock per case_id when inserting custody_log entries to prevent race conditions on the hash chain.

**Tests (100% coverage):**
- Mutation request → custody_log entry created
- Log entry contains correct user_id, case_id, action, IP
- Hash chain: each entry's previous_log_hash matches prior entry's hash
- Hash chain verification: tamper with an entry → chain broken
- Concurrent mutations → hash chain remains consistent (advisory lock)
- Read-only GET → no custody log entry (for non-evidence endpoints)
- Evidence view → custody log entry created
- Evidence download → custody log entry created
- Handler failure (4xx/5xx) → no custody log entry (only log successful mutations)
- Large details JSON → stored correctly in JSONB
- IP extraction from X-Forwarded-For (behind Caddy)

### Step 6: Custody Chain Verification (`internal/custody/chain.go`)

**Deliverable:** Function to verify the integrity of the custody chain.

**Interface:**
```go
type ChainVerifier interface {
    VerifyEvidenceChain(ctx context.Context, evidenceID uuid.UUID) (ChainVerification, error)
    VerifyCaseChain(ctx context.Context, caseID uuid.UUID) (ChainVerification, error)
}

type ChainVerification struct {
    Valid       bool
    TotalEntries int
    VerifiedAt  time.Time
    Breaks      []ChainBreak  // empty if valid
}

type ChainBreak struct {
    EntryID      uuid.UUID
    Position     int
    ExpectedHash string
    ActualHash   string
    Timestamp    time.Time
}
```

**Tests:**
- Valid chain → verification passes
- Tampered entry → verification fails, break identified at correct position
- Empty chain → valid (no entries to verify)
- Single entry chain → valid (no previous hash to check)
- Chain with 10,000 entries → completes in < 5 seconds

### Step 7: Custody Endpoints

**Deliverable:** API endpoints for viewing custody logs.

**Endpoints:**
```
GET /api/evidence/:id/custody      → Get custody chain for evidence item
GET /api/cases/:id/custody         → Get full case custody log
```

**Both endpoints:**
- Cursor-based pagination
- Sortable by timestamp (default: newest first)
- Include hash chain verification status
- Response includes computed hash for each entry (client can verify)

**Tests:**
- Paginated results correct
- Entries ordered by timestamp
- Hash chain visible in response
- Defence user → only sees custody entries for disclosed evidence

### Step 8: Cases Frontend

**Deliverable:** Next.js pages for case management.

**Pages:**
- `/cases` — Case list with search, filter by status, pagination
- `/cases/new` — Create case form (Case Admin+ only)
- `/cases/[id]` — Case detail with tabs: Overview, Evidence (placeholder), Custody Log, Settings
- `/cases/[id]/settings` — Edit case, manage roles, legal hold toggle

**Components:**
- `CaseList` — Data table with sorting, filtering, pagination
- `CaseForm` — Create/edit form with validation
- `CaseRoleManager` — Assign/revoke roles with user search
- `CustodyLogTable` — Paginated custody log with hash chain indicator
- `LegalHoldBadge` — Visual indicator for legal hold status
- `CaseStatusBadge` — Color-coded status (active=green, closed=yellow, archived=gray)

**UX details:**
- Empty state: "No cases yet" with CTA to create first case
- Loading states: skeleton loaders
- Error states: toast notifications for mutations, inline errors for forms
- Confirmation dialog for destructive actions (archive, legal hold toggle)
- All text via `useTranslations()` (i18n ready)

**Tests:**
- Case list renders with mock data
- Create form validation (required fields, reference_code format)
- Role assignment UI works
- Custody log displays correctly with pagination
- Legal hold toggle shows confirmation dialog
- Empty states render correctly
- Error states display correctly
- i18n keys all present in en.json

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/cases/repository.go` | Create | Postgres CRUD for cases |
| `internal/cases/service.go` | Create | Business logic, validation |
| `internal/cases/handler.go` | Create | HTTP handlers |
| `internal/cases/models.go` | Create | Case, CaseFilter, CaseUpdate types |
| `internal/custody/logger.go` | Create | Custody logging middleware |
| `internal/custody/chain.go` | Create | Hash chain computation + verification |
| `internal/custody/repository.go` | Create | Custody log Postgres queries |
| `internal/custody/models.go` | Create | CustodyLogEntry, ChainVerification types |
| `internal/server/routes.go` | Modify | Wire case + custody routes |
| `web/src/app/[locale]/cases/page.tsx` | Create | Case list page |
| `web/src/app/[locale]/cases/new/page.tsx` | Create | Create case page |
| `web/src/app/[locale]/cases/[id]/page.tsx` | Create | Case detail page |
| `web/src/components/cases/*` | Create | Case UI components |

---

## Definition of Done

- [ ] Full cases CRUD working end-to-end (API + UI)
- [ ] Case roles management working (assign, revoke, list)
- [ ] Custody logging middleware captures all mutations
- [ ] Hash chain computed correctly on every log entry
- [ ] Hash chain verification detects tampered entries
- [ ] Defence users only see cases they're assigned to
- [ ] Case Admin+ required for create/update/archive operations
- [ ] Legal hold prevents case archival
- [ ] Reference code format validated
- [ ] All input sanitized (HTML escaped)
- [ ] Cursor-based pagination on all list endpoints
- [ ] 100% test coverage on repository, service, handler, custody
- [ ] Frontend pages render correctly with mock data
- [ ] All UI strings in i18n files

---

## Security Checklist

- [ ] All SQL queries parameterized (no string concatenation)
- [ ] Input validation on all user-provided fields
- [ ] HTML escaped before storage
- [ ] Case access filtered by user's case_roles
- [ ] Custody log append-only (RLS enforced)
- [ ] Hash chain prevents silent log modification
- [ ] IP address captured from X-Forwarded-For (Caddy)
- [ ] Error responses don't leak internal details
- [ ] Reference code regex prevents injection
- [ ] JSON body size limited to 1MB

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 3 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/cases/repository.go`**: Create returns case with generated UUID and created_at, duplicate reference_code returns unique constraint error, FindByID returns correct case, FindByID with non-existent ID returns not found, FindAll with user filter returns only assigned cases, FindAll with system admin returns all, FindAll status/search/jurisdiction filters, cursor pagination correctness (has_more, total_count), items added mid-pagination cause no duplicates/skips, invalid cursor returns 400, limit > 200 capped silently, limit = 0 defaults to 50, negative limit returns 400, Update returns updated case with unchanged created_at, Update non-existent returns not found, Archive sets status to archived (idempotent), SetLegalHold toggles boolean, SQL injection via filter fields blocked, connection pool exhaustion returns timeout not crash
- **`internal/cases/service.go`**: Valid input creates case, invalid reference_code format returns specific validation error, empty title returns error, title > 500 chars returns error, description > 10000 chars returns error, invalid status transition (archived to active) returns error, archive with legal_hold returns clear error, SetLegalHold creates custody_log entry, ExportCase returns ZIP with metadata + custody CSV, HTML in title stripped, metadata > 1MB returns error
- **`internal/cases/handler.go`**: Each endpoint with valid input returns correct status + body, each endpoint with invalid input returns 400, each endpoint with wrong role returns 403, large body returns 413, invalid JSON returns 400, missing required fields return 400 with field-specific error, non-existent case_id returns 404, full CRUD lifecycle integration
- **Case roles handler**: Assign valid role returns 201 + custody_log entry, duplicate role returns 409, invalid role returns 400, non-existent Keycloak user returns 400, revoke role returns 200 + custody_log entry, revoke own role returns 403, list roles returns all assignments, non-Case Admin returns 403
- **`internal/custody/logger.go`**: Mutation request creates custody_log entry, entry contains correct user_id/case_id/action/IP, hash chain — each entry's previous_log_hash matches prior hash, tampered entry breaks chain, concurrent mutations maintain consistent chain (advisory lock), read-only GET creates no entry (non-evidence endpoints), evidence view creates entry, evidence download creates entry, handler failure (4xx/5xx) creates no entry, large details JSON stored correctly, IP extracted from X-Forwarded-For
- **`internal/custody/chain.go`**: Valid chain passes verification, tampered entry fails at correct position, empty chain valid, single entry chain valid, 10000-entry chain completes in < 5 seconds

### Integration Tests (with testcontainers)

- **Postgres cases CRUD (testcontainers/postgres:16-alpine)**: Full lifecycle — create, read, update, archive, legal hold — verified against real Postgres with migrations applied, unique constraint on reference_code enforced, foreign key from case_roles to cases enforced, CHECK constraint on status values enforced, cursor pagination with concurrent inserts, advisory lock on custody_log hash chain under concurrent writes
- **Custody hash chain integrity (testcontainers/postgres:16-alpine)**: Insert 100 custody entries with concurrent goroutines, verify hash chain is unbroken end-to-end, manually tamper with one row, verify chain verification detects break at correct position
- **Case roles + Keycloak (testcontainers/keycloak:24.0 + postgres)**: Assign role to real Keycloak user, verify case_roles row created, verify custody_log entry, revoke role, verify access denied on subsequent request

### E2E Automated Tests (Playwright)

- **`tests/e2e/cases/create-case.spec.ts`**: Login as case_admin, navigate to /cases/new, fill in reference code (valid format), title, description, jurisdiction, submit, verify redirect to case detail page, verify case appears in case list
- **`tests/e2e/cases/list-cases.spec.ts`**: Login, navigate to /cases, verify list renders with existing cases, apply status filter (active), verify filtered results, apply search query, verify matching results, paginate through results
- **`tests/e2e/cases/update-case.spec.ts`**: Navigate to case detail, click edit/settings, modify title and description, save, verify updated values displayed, verify custody log shows "case_updated" entry
- **`tests/e2e/cases/archive-case.spec.ts`**: Navigate to case settings, click archive button, confirm dialog, verify case status changes to "archived", verify case appears in archived filter, verify cannot archive case with legal_hold (toggle legal hold first, attempt archive, see error)
- **`tests/e2e/cases/legal-hold.spec.ts`**: Navigate to case settings, toggle legal hold on, confirm dialog, verify legal hold badge appears, attempt to archive — verify error message, toggle legal hold off, verify badge removed
- **`tests/e2e/cases/role-assignment.spec.ts`**: Navigate to case settings > roles, add user with "investigator" role, verify user appears in role list, verify custody log shows "role_granted", remove user role, verify custody log shows "role_revoked"
- **`tests/e2e/custody/custody-log.spec.ts`**: Navigate to case detail > custody tab, verify log entries appear with timestamps and actions, verify hash chain indicator shows "valid", paginate through entries, verify newest-first ordering

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Login as case_admin, navigate to /cases/new, enter reference code "ICC-UKR-2024", title "Ukraine Investigation", submit
   **Expected:** Case created successfully, redirected to case detail page with all fields displayed
   **Verify:** Case appears in /cases list, reference code and title match, created_at timestamp is current, status is "active"

2. [ ] **Action:** On the case list page, test search by typing the reference code into the search box
   **Expected:** Only matching cases appear in the filtered list
   **Verify:** Clear search, all cases reappear; filter by status "active", verify correct filtering; paginate if > 50 cases exist

3. [ ] **Action:** Navigate to case detail, click edit, change the title and description, save
   **Expected:** Updated values displayed immediately on the case detail page
   **Verify:** Open the Custody Log tab, verify a "case_updated" entry exists with the changed fields listed in details

4. [ ] **Action:** Navigate to case settings, click "Set Legal Hold", confirm the dialog
   **Expected:** Legal hold badge appears on the case, status bar shows legal hold active
   **Verify:** Attempt to archive the case — verify an error message appears stating legal hold prevents archival

5. [ ] **Action:** Release the legal hold, then archive the case
   **Expected:** Legal hold badge removed, archive succeeds, case status changes to "archived"
   **Verify:** Case appears under "archived" filter in case list; custody log shows both "legal_hold_released" and "case_archived" entries

6. [ ] **Action:** Navigate to case settings > Roles, assign a test user as "defence" role
   **Expected:** User appears in the role list with "defence" badge, custody log shows "role_granted"
   **Verify:** Log in as that defence user, navigate to /cases, verify only cases with assigned roles are visible; verify no access to cases without a role (403, not listed)

7. [ ] **Action:** As case_admin, assign a user as "investigator", then revoke the role
   **Expected:** Role revoked successfully, custody log shows "role_revoked" with both granter and grantee
   **Verify:** Log in as that user, verify the case is no longer accessible (403)

8. [ ] **Action:** Attempt to create a case with invalid reference code format (e.g., "invalid-code")
   **Expected:** Validation error displayed inline on the form with specific message about expected format
   **Verify:** Case is NOT created; no entry in case list; no custody log entry generated

9. [ ] **Action:** Navigate to a case's Custody Log tab, scroll through entries
   **Expected:** All entries ordered newest-first with timestamps, user IDs, actions, and IP addresses
   **Verify:** Each entry shows a hash chain indicator; clicking "Verify Chain" confirms the chain is intact; pagination works for long chains

10. [ ] **Action:** Attempt to assign the same role twice to the same user on the same case
    **Expected:** 409 Conflict error with clear message about duplicate role
    **Verify:** Only one role assignment exists in the database; no duplicate custody log entries

11. [ ] **Action:** As "defence" role user, navigate to the case detail
    **Expected:** Case is visible but only disclosed evidence items appear in the evidence tab (or placeholder message if no disclosures yet)
    **Verify:** No undisclosed evidence metadata, titles, or counts are leaked anywhere on the page

12. [ ] **Action:** Send a request with a JSON body larger than 1MB (e.g., very large metadata field)
    **Expected:** 413 Payload Too Large response
    **Verify:** Response body is the standard error envelope; no server crash or memory spike
