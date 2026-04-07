# Sprint 2: Authentication & Authorization

**Phase:** 1 — Foundation
**Duration:** Weeks 3-4
**Goal:** Implement JWT validation middleware, two-level role system (system roles from Keycloak + case roles from Postgres), and permission enforcement on all API endpoints.

---

## Prerequisites

- Sprint 1 complete (Docker stack, DB schema, Keycloak realm, Go project structure)
- Keycloak running with `vaultkeeper` realm and configured clients
- Postgres with `case_roles` table migrated

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js auth flow)

---

## Implementation Steps

### Step 1: JWT Validation Middleware (`internal/auth/middleware.go`)

**Deliverable:** Chi middleware that validates Keycloak JWTs on every request.

**Flow:**
1. Extract `Authorization: Bearer <token>` from request header
2. Validate JWT signature against Keycloak's JWKS endpoint (cached, refreshed every 5 min)
3. Validate standard claims: `exp`, `iss`, `aud`
4. Extract user identity: `sub` (user ID), `email`, `preferred_username`
5. Extract system role from custom claim: `realm_access.roles` → map to VaultKeeper system role
6. Inject `AuthContext` into request context

**AuthContext structure:**
```go
type AuthContext struct {
    UserID       string   // Keycloak sub claim
    Email        string
    Username     string
    SystemRole   SystemRole  // system_admin, case_admin, user, api_service
    TokenExpiry  time.Time
    SessionID    string
    IPAddress    string
}
```

**JWKS caching:**
- Fetch JWKS from `{KEYCLOAK_URL}/realms/{REALM}/protocol/openid-connect/certs`
- Cache keys in memory with 5-minute TTL
- On signature validation failure, force-refresh JWKS once (key rotation scenario)
- If JWKS endpoint unreachable, use cached keys for up to 15 minutes (Keycloak outage tolerance per spec)

**Error responses (JSON):**
- Missing Authorization header → 401 `{"error": "authentication required"}`
- Invalid/expired token → 401 `{"error": "invalid or expired token"}`
- Keycloak unreachable + no cached keys → 502 `{"error": "authentication service unavailable"}`
- Never include token content in error responses

**JWKS outage resilience (per spec error handling):**
- If Keycloak unreachable but cached JWKS keys exist: validate JWT with cached keys for up to 15 minutes (matches access token max lifetime)
- Existing sessions with valid (non-expired) JWTs continue working until token expires
- No new logins possible while Keycloak is down (auth code exchange requires Keycloak)
- After 15 minutes with no JWKS refresh: reject all tokens (keys may have been rotated/revoked)
- On startup: pre-fetch JWKS before accepting any requests (fail-fast if Keycloak unreachable at boot)

**Excluded paths (no auth required):**
- `GET /health` — public health check
- `OPTIONS *` — CORS preflight

**Tests (100% coverage):**
- Valid JWT → AuthContext populated correctly
- Expired JWT → 401
- JWT with wrong signature → 401
- JWT with wrong audience → 401
- JWT with wrong issuer → 401
- Missing Authorization header → 401
- Malformed Authorization header (no "Bearer" prefix) → 401
- JWKS cache hit (no network call on second request)
- JWKS cache expiry (refresh after TTL)
- JWKS endpoint down + cache warm → success with cached keys
- JWKS endpoint down + cache cold → 502
- System role extraction from realm_access.roles
- Unknown role in token → default to lowest privilege ("user")
- `/health` endpoint bypasses auth
- OPTIONS request bypasses auth
- Concurrent requests don't race on JWKS cache

### Step 2: System Role Authorization (`internal/auth/permissions.go`)

**Deliverable:** Middleware/helper that enforces system role requirements per endpoint.

**System Role Hierarchy:**
```
system_admin > case_admin > user > api_service
```

**Permission matrix:**

| Endpoint | system_admin | case_admin | user | api_service |
|----------|:---:|:---:|:---:|:---:|
| POST /api/cases | x | x | - | - |
| GET /api/cases | x | x | x | x (scoped) |
| PATCH /api/cases/:id | x | x | - | - |
| POST /api/cases/:id/archive | x | x | - | - |
| POST /api/cases/:id/legal-hold | x | x | - | - |
| POST /api/cases/:id/roles | x | x | - | - |
| DELETE /api/cases/:id/roles/:userId | x | x | - | - |
| GET /api/health (detailed) | x | - | - | - |
| GET /api/audit | x | - | - | - |
| POST /api/cases/:id/evidence | x | x | x | x (scoped) |
| GET /api/cases/:id/evidence | x | x | x | x (scoped) |

**Implementation pattern:**
```go
// RequireSystemRole returns middleware that checks minimum system role
func RequireSystemRole(minimum SystemRole) func(http.Handler) http.Handler

// Usage in routes:
r.With(RequireSystemRole(CaseAdmin)).Post("/api/cases", casesHandler.Create)
```

**Tests (100% coverage):**
- Each role × each endpoint combination (full permission matrix)
- system_admin accessing case_admin endpoint → allowed
- user accessing case_admin endpoint → 403
- api_service accessing user endpoint → 403
- 403 response body: `{"error": "insufficient permissions"}`
- AuthContext missing from context → 500 (internal error, never happens in prod)

### Step 3: Case Role Authorization

**Deliverable:** Per-request case role resolution and enforcement.

**Flow (on any `/api/cases/:id/*` request):**
1. Extract `case_id` from URL
2. Look up `case_roles` for `(case_id, user_id)` in Postgres
3. If no role → 403 (user not assigned to this case)
4. System admins bypass case role checks (see all cases)
5. Inject `CaseRole` into request context alongside `AuthContext`

**Case Role Permission Matrix:**

| Action | investigator | prosecutor | defence | judge | observer | victim_rep |
|--------|:---:|:---:|:---:|:---:|:---:|:---:|
| View all evidence | x | x | - | x | x | - |
| View disclosed evidence | x | x | x | x | x | x |
| Upload evidence | x | x | - | - | - | - |
| Tag/classify evidence | x | x | - | - | - | - |
| Create disclosures | - | x | - | - | - | - |
| Apply redactions | x | x | - | - | - | - |
| See witness identity | x | x | - | case-by-case | - | - |
| Download evidence | x | x | x (disclosed) | x | - | - |

**Implementation pattern:**
```go
// RequireCaseRole checks the user has a specific case role for the case in the URL
func RequireCaseRole(allowed ...CaseRole) func(http.Handler) http.Handler

// EvidenceVisibilityFilter returns a query modifier based on case role
func EvidenceVisibilityFilter(role CaseRole) QueryFilter
```

**Caching:**
- Case roles queried on every request (they can change)
- Optional: short TTL cache (30 seconds) for repeated requests in same session
- On role revocation, cache is invalidated (Keycloak webhook or manual flush)

**Tests (100% coverage):**
- Each case role × each action combination
- User assigned as investigator in Case A, no role in Case B → can access A, 403 on B
- User with different roles in different cases → correct role applied per case
- System admin → bypasses case role check, sees all cases
- Defence user → only sees disclosed evidence items
- Observer → read-only (all mutations rejected)
- Role revocation → immediate access loss (no stale cache)
- Non-existent case_id → 404
- Valid case_id but user not assigned → 403

### Step 4: API Response Envelope

**Deliverable:** Consistent JSON response format across all endpoints.

**Success response:**
```json
{
  "data": { ... },
  "error": null,
  "meta": {
    "total": 150,
    "next_cursor": "base64...",
    "has_more": true
  }
}
```

**Error response:**
```json
{
  "data": null,
  "error": "human-readable error message",
  "meta": null
}
```

**Implementation:**
```go
func RespondJSON(w http.ResponseWriter, status int, data any)
func RespondError(w http.ResponseWriter, status int, message string)
func RespondPaginated(w http.ResponseWriter, data any, total int, cursor string, hasMore bool)
```

**Rules:**
- All responses set `Content-Type: application/json`
- Error messages never expose internal details (no stack traces, no SQL errors)
- 500 errors log full details server-side, return generic message to client
- Status codes: 200 (success), 201 (created), 400 (bad request), 401 (unauthorized), 403 (forbidden), 404 (not found), 409 (conflict), 413 (payload too large), 429 (rate limited), 500 (internal), 502 (upstream), 503 (unavailable), 507 (storage full)

**Tests:**
- Each response helper produces correct JSON structure
- Status code matches
- Error responses never contain internal details
- Paginated response includes all meta fields
- Content-Type header set correctly

### Step 5: Next.js Auth Integration

**Deliverable:** Keycloak OIDC login flow in Next.js via `next-auth`.

**Flow:**
1. User visits any protected page → redirected to Keycloak login
2. Keycloak authenticates (password, SSO, MFA) → redirects back with auth code
3. `next-auth` exchanges code for tokens → stores in secure HTTP-only cookie
4. API requests include JWT in Authorization header
5. Token refresh handled automatically by `next-auth`
6. Logout clears session + redirects to Keycloak logout endpoint

**Implementation:**
- `next-auth` with Keycloak provider (`vaultkeeper-web` client, PKCE)
- Session includes: user ID, email, system role, access token
- Access token refreshed automatically when < 60 seconds from expiry
- Protected layout component wraps all authenticated pages
- Login page with Keycloak redirect button
- Logout button in header/sidebar

**API client auth:**
```typescript
// Automatically inject access token from session
async function authenticatedFetch<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>> {
  const session = await getServerSession();
  if (!session?.accessToken) redirect("/login");
  
  return api<T>(path, {
    ...options,
    headers: {
      ...options?.headers,
      Authorization: `Bearer ${session.accessToken}`,
    },
  });
}
```

**Tests:**
- Unauthenticated user → redirect to login
- Successful login → session created, redirect to dashboard
- Session contains correct user data (id, email, role)
- API requests include Authorization header
- Token refresh works before expiry
- Expired session → redirect to login
- Logout clears session completely
- Keycloak unavailable → error page (not crash)

### Step 6: Custody Logging for Auth Events

**Deliverable:** All authentication and authorization events logged to custody_log.

**Events logged:**
- User login (successful) → `action: "user_login"`, details: `{ip, user_agent}`
- User login (failed) → `action: "login_failed"`, details: `{ip, user_agent, reason}`
- User logout → `action: "user_logout"`
- Case role granted → `action: "role_granted"`, details: `{case_id, role, granted_by}`
- Case role revoked → `action: "role_revoked"`, details: `{case_id, role, revoked_by}`
- Permission denied → `action: "access_denied"`, details: `{endpoint, required_role, actual_role}`

**Implementation:**
- Login/logout events captured via Keycloak event listener (webhook) or polled from Keycloak admin API
- Role grant/revoke events captured in the case roles handler
- Access denied events captured in permission middleware

**Tests:**
- Each event type produces correct custody_log entry
- Log entries include correct user_id, ip_address, timestamp
- Hash chain maintained (previous_log_hash links to prior entry)
- Failed login attempt logged with IP
- Role change logged with both granter and grantee

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/auth/middleware.go` | Create | JWT validation, JWKS caching, AuthContext injection |
| `internal/auth/middleware_test.go` | Create | Full JWT validation test suite |
| `internal/auth/permissions.go` | Create | System role + case role enforcement |
| `internal/auth/permissions_test.go` | Create | Full permission matrix tests |
| `internal/auth/context.go` | Create | AuthContext and CaseRole types |
| `internal/auth/jwks.go` | Create | JWKS fetcher with caching |
| `internal/auth/jwks_test.go` | Create | JWKS cache behavior tests |
| `internal/server/response.go` | Create | JSON response helpers |
| `internal/server/response_test.go` | Create | Response format tests |
| `internal/custody/logger.go` | Modify | Add auth event logging |
| `web/src/lib/auth.ts` | Create | next-auth Keycloak provider config |
| `web/src/app/[locale]/login/page.tsx` | Create | Login page with Keycloak redirect |
| `web/src/components/layout/auth-guard.tsx` | Create | Protected route wrapper |
| `web/src/hooks/use-auth.ts` | Create | Auth state hook |

---

## Definition of Done

- [ ] All API endpoints require valid JWT (except /health)
- [ ] System role enforced on all endpoints per permission matrix
- [ ] Case role enforced on all case-scoped endpoints
- [ ] Defence users only see disclosed evidence
- [ ] System admins bypass case role checks
- [ ] JWT validation fails gracefully when Keycloak is down (cached keys)
- [ ] Next.js login/logout flow works end-to-end
- [ ] API client automatically includes auth token
- [ ] All auth events logged to custody_log with hash chain
- [ ] 100% test coverage on auth middleware and permissions
- [ ] No JWT tokens logged anywhere
- [ ] Error responses never expose internal details

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| JWKS endpoint latency on cold start | Pre-fetch JWKS on server startup before accepting requests |
| Token refresh race condition | Mutex on token refresh, queue concurrent requests |
| Case role query on every request (perf) | Short TTL cache (30s), indexed query, connection pooling |
| Keycloak version upgrade breaks JWT format | Pin Keycloak version, integration test JWT parsing in CI |
| next-auth session/token type mismatch | Strict TypeScript types for session, integration test full flow |

---

## Security Checklist

- [ ] JWT signature validated against Keycloak JWKS
- [ ] Token expiry enforced (15 min access token)
- [ ] Audience and issuer claims validated
- [ ] JWKS cache cannot be poisoned (only fetched from configured Keycloak URL)
- [ ] System role extracted from token claims, not from client-provided data
- [ ] Case roles queried from Postgres, not from token
- [ ] 403 responses don't leak information about why access was denied
- [ ] Failed login attempts logged with IP for audit
- [ ] No JWT tokens in log output
- [ ] CORS configured to only allow APP_URL origin
- [ ] HTTP-only, Secure, SameSite cookies for session
- [ ] PKCE flow used for frontend auth (prevents auth code interception)

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 2 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/auth/middleware.go`**: Valid JWT populates AuthContext correctly, expired JWT returns 401, wrong signature returns 401, wrong audience returns 401, wrong issuer returns 401, missing Authorization header returns 401, malformed header (no "Bearer" prefix) returns 401, JWKS cache hit avoids network call, JWKS cache expiry triggers refresh, JWKS endpoint down + warm cache returns success, JWKS endpoint down + cold cache returns 502, system role extraction from realm_access.roles, unknown role defaults to "user", `/health` bypasses auth, OPTIONS bypasses auth, concurrent requests do not race on JWKS cache
- **`internal/auth/permissions.go`**: Full permission matrix tested — every system role x every endpoint combination, system_admin accessing case_admin endpoint allowed, user accessing case_admin endpoint returns 403, api_service accessing user endpoint returns 403, 403 body matches `{"error": "insufficient permissions"}`, missing AuthContext returns 500
- **`internal/auth/context.go`** (case roles): Each case role x each action combination, user with role in Case A but not Case B gets 403 on B, different roles in different cases resolved correctly, system_admin bypasses case role check, defence only sees disclosed evidence, observer mutations all rejected, role revocation causes immediate access loss, non-existent case_id returns 404, valid case_id but unassigned user returns 403
- **`internal/server/response.go`**: RespondJSON produces correct structure, RespondError produces correct structure, RespondPaginated includes all meta fields, Content-Type header is `application/json`, error responses never contain stack traces or SQL errors, each status code mapped correctly
- **`internal/custody/logger.go`** (auth events): Login success logged with IP and user_agent, login failure logged with reason, logout logged, role granted logged with granter, role revoked logged with revoker, access denied logged with endpoint and roles, hash chain maintained across auth events

### Integration Tests (with testcontainers)

- **Keycloak + JWT flow (testcontainers/keycloak:24.0)**: Full token exchange (username/password to JWT), JWT validation against live JWKS endpoint, token refresh before expiry produces new valid token, expired token rejected by middleware, JWKS key rotation scenario (force-refresh on signature failure), brute force lockout after 5 failed logins, concurrent session limit enforced (default 3)
- **Postgres case roles (testcontainers/postgres:16-alpine)**: Case role lookup returns correct role for (case_id, user_id), no role returns empty/nil, role INSERT and DELETE reflected immediately in subsequent queries, unique constraint prevents duplicate role assignment, custody_log entries created for role changes with valid hash chain
- **Full auth middleware chain**: Request with valid JWT + valid case role proceeds to handler, request with valid JWT + no case role returns 403, request with expired JWT returns 401 before case role check, system_admin JWT bypasses case role lookup

### E2E Automated Tests (Playwright)

- **`tests/e2e/login.spec.ts`**: Navigate to protected page, verify redirect to Keycloak login, enter valid credentials, verify redirect back to app with session, verify header shows username and role
- **`tests/e2e/logout.spec.ts`**: From authenticated session, click logout button, verify redirect to login page, attempt to navigate to protected page, verify redirect back to login (session cleared)
- **`tests/e2e/token-refresh.spec.ts`**: Login, wait for access token to approach expiry (or mock short-lived token), verify session remains active without re-login, verify API calls continue to succeed
- **`tests/e2e/permission-denied.spec.ts`**: Login as user with "user" system role, attempt to navigate to case admin endpoint (e.g., create case), verify 403 or "Access Denied" message displayed, verify no data leakage in error response
- **`tests/e2e/role-enforcement.spec.ts`**: Login as case_admin, create a case (should succeed), login as user role, attempt to create a case (should fail with 403), login as system_admin, access detailed health endpoint (should succeed)

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Open the app in a browser without being logged in, navigate to `/cases`
   **Expected:** Immediately redirected to Keycloak login page
   **Verify:** URL changes to Keycloak domain, login form visible, no flash of protected content

2. [ ] **Action:** Enter valid credentials (test user with "user" system role) on Keycloak login page
   **Expected:** Redirected back to the app, session established, header displays username and role
   **Verify:** Browser DevTools > Application > Cookies shows secure, HTTP-only session cookie; no JWT visible in localStorage

3. [ ] **Action:** Open browser DevTools > Network, make any API request from the app
   **Expected:** Request includes `Authorization: Bearer <token>` header
   **Verify:** Token is a valid JWT (paste into jwt.io, verify claims: sub, email, realm_access.roles, exp, iss, aud)

4. [ ] **Action:** As a "user" role, attempt to access `POST /api/cases` via curl or the UI "Create Case" button (if visible)
   **Expected:** 403 Forbidden response with `{"error": "insufficient permissions"}`
   **Verify:** Response body contains no internal details (no stack trace, no SQL, no role names beyond the generic message)

5. [ ] **Action:** As a "case_admin" role, create a case, then attempt to access it as a different user with no case role
   **Expected:** Different user gets 403 when accessing the case
   **Verify:** Response is 403, not 404 (confirm it's an authorization failure, not a "case doesn't exist" response — wait, per spec this should be 403 for unassigned user)

6. [ ] **Action:** Wait for access token to expire (15 minutes) or force short-lived token in dev config
   **Expected:** App automatically refreshes the token without user interaction
   **Verify:** No redirect to login page, API calls continue working, new token visible in Network tab

7. [ ] **Action:** Click the Logout button in the app header/sidebar
   **Expected:** Session cleared, redirected to Keycloak logout, then to login page
   **Verify:** Try to access a protected page — redirected to login; session cookie removed; back button does not restore session

8. [ ] **Action:** Enter wrong password 5 times on Keycloak login
   **Expected:** Account locked out for 15 minutes
   **Verify:** 6th attempt with correct password still fails with lockout message; wait 15 minutes, then correct password works

9. [ ] **Action:** As system_admin, assign a user as "defence" on a case, then log in as that defence user
   **Expected:** Defence user can see the case but only disclosed evidence items
   **Verify:** Evidence list shows only items that have been included in a disclosure; non-disclosed items are completely absent (not greyed out, absent)

10. [ ] **Action:** Open the custody log for a case where login/logout events occurred
    **Expected:** Auth events (login, logout, role granted, access denied) appear in the custody log with correct timestamps and IP addresses
    **Verify:** Each entry has a valid previous_log_hash linking to the prior entry; hash chain is unbroken

11. [ ] **Action:** Stop the Keycloak container (`docker compose stop keycloak`), then immediately make an API request with a valid (non-expired) JWT
    **Expected:** API request succeeds (JWKS cached for up to 15 minutes)
    **Verify:** Response is 200, not 502; after 15 minutes with Keycloak still down, new requests fail with 502

12. [ ] **Action:** Inspect CORS headers on API responses
    **Expected:** `Access-Control-Allow-Origin` matches only the configured `APP_URL`
    **Verify:** Request from a different origin is blocked; preflight OPTIONS requests return correct CORS headers
