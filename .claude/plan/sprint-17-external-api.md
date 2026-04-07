# Sprint 17: External API & API Key Management

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 33-34
**Goal:** Build the public REST API with scoped API key authentication for external system integrations (OTP Link, OSINT tools, partner institutions).

---

## Prerequisites

- Phase 2 complete (all core features operational)
- `api_keys` table in schema

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js — admin API key management UI)

---

## Implementation Steps

### Step 1: API Key Authentication Middleware

**Deliverable:** Alternative auth path for API key-based requests.

**Auth flow (API key):**
1. Extract API key from `X-API-Key` header (or `Authorization: ApiKey <key>`)
2. Hash the key (SHA-256)
3. Look up in `api_keys` table by key_hash
4. Verify: not revoked, not expired
5. Check case scope: key's `case_ids` array must include the requested case
6. Check permissions: read or read_write
7. Inject ApiKeyContext into request context
8. Update `last_used_at` on the key

**ApiKeyContext:**
```go
type ApiKeyContext struct {
    KeyID       uuid.UUID
    KeyName     string
    CaseIDs     []uuid.UUID
    Permissions string    // "read" or "read_write"
    CreatedBy   string    // admin who created the key
}
```

**Middleware chain:**
- Try JWT auth first (Keycloak tokens)
- If no JWT, try API key
- If neither → 401
- API key auth logs to custody chain with key ID (not key value)

**Rate limiting per key:**
- Default: 60 requests/minute per API key (per spec)
- Configurable per key: `rate_limit` column in api_keys table (int, requests/minute)
- Rate limit enforcement: in-memory sliding window counter per key ID (or Redis for multi-instance)
- Rate limit headers in response:
  - `X-RateLimit-Limit: 60` (the key's configured limit)
  - `X-RateLimit-Remaining: 45` (remaining requests in current window)
  - `X-RateLimit-Reset: 1680000000` (Unix timestamp when window resets)
- 429 response includes `Retry-After` header (seconds until next window)
- Separate from Caddy-level rate limiting (Caddy handles per-user JWT, Go handles per-API-key)

**Tests:**
- Valid API key → authenticated
- Expired key → 401
- Revoked key → 401
- Key accessing unauthorized case → 403
- Read-only key attempting write → 403
- Rate limit exceeded → 429 with retry-after header
- Key hash lookup (not plaintext comparison)
- Custody log entries use key ID, never key value
- last_used_at updated on each use
- Concurrent requests with same key → rate limit applies

### Step 2: API Key Management Service

**Interface:**
```go
type APIKeyService interface {
    CreateKey(ctx context.Context, input CreateKeyInput) (APIKeyResponse, error)
    ListKeys(ctx context.Context, pagination Pagination) (PaginatedResult[APIKeyInfo], error)
    RevokeKey(ctx context.Context, keyID uuid.UUID) error
    UpdateKey(ctx context.Context, keyID uuid.UUID, input UpdateKeyInput) (APIKeyInfo, error)
}

type CreateKeyInput struct {
    Name        string
    CaseIDs     []uuid.UUID
    Permissions string      // "read" or "read_write"
    ExpiresAt   *time.Time  // nil = no expiry (not recommended)
    RateLimit   int         // requests/minute, default 60
}

type APIKeyResponse struct {
    Key     string    // Plaintext key — ONLY returned once at creation
    KeyInfo APIKeyInfo
}
```

**Key generation:**
- 48 random bytes → base64url encoded → 64-character key
- Prefix: `vk_` for VaultKeeper keys (easy identification)
- Example: `vk_a1b2c3d4e5f6...`
- Store SHA-256(key) in database, NEVER store plaintext
- Return plaintext to admin ONCE at creation, never again

**Custody logging:**
- Key created → logged with key name, scoped cases, created_by
- Key revoked → logged with key ID, revoked_by
- Key used → each API call logged with key ID in custody chain

**Tests:**
- Create key → plaintext returned, hash stored
- List keys → shows info but NOT plaintext
- Revoke key → immediately unusable
- Update key → scope/permissions/expiry modified
- Key format: starts with "vk_", 64+ characters
- Plaintext never stored in database
- Only System Admin can manage keys

### Step 3: API Key Management Endpoints

```
POST   /api/admin/api-keys              → Create new API key (System Admin)
GET    /api/admin/api-keys              → List all API keys (System Admin)
PATCH  /api/admin/api-keys/:id          → Update key (scope, permissions, expiry)
DELETE /api/admin/api-keys/:id          → Revoke key
```

### Step 4: External API Documentation

**Deliverable:** OpenAPI 3.0 spec for the external API.

**Documentation scope:**
- All existing endpoints accessible via API key
- Authentication methods (JWT + API key)
- Request/response schemas
- Error codes
- Rate limiting
- Pagination
- Code examples (cURL, Python, JavaScript)

**Endpoint:** `GET /api/docs` — Swagger UI

**Implementation:** Generate OpenAPI spec from Go code annotations or maintain manually.

### Step 5: API Key Management Frontend

**Components:**
- `APIKeyList` — Table of API keys
  - Name, scoped cases, permissions, created date, last used, status
  - Revoke button with confirmation
- `APIKeyForm` — Create new key
  - Name input
  - Case selector (multi-select)
  - Permissions (read / read-write radio)
  - Expiry date picker (optional)
  - Rate limit input
- `APIKeyCreated` — One-time display of plaintext key
  - Warning: "This key will only be shown once"
  - Copy button
  - "I have saved this key" confirmation to dismiss

### Step 6: Webhook Notifications (Optional)

**Deliverable:** Outbound webhooks for external system integration.

**Events:**
- evidence_uploaded
- evidence_destroyed
- disclosure_created
- integrity_warning

**Webhook config:**
```go
type WebhookConfig struct {
    URL     string
    Secret  string   // HMAC signing key
    Events  []string
    CaseIDs []uuid.UUID
}
```

**Delivery:**
- HMAC-SHA256 signature in `X-Webhook-Signature` header
- Retry: 3 attempts with exponential backoff
- Timeout: 10 seconds per delivery
- Delivery log in database

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/auth/apikey.go` | Create | API key auth middleware |
| `internal/apikeys/service.go` | Create | Key management service |
| `internal/apikeys/handler.go` | Create | Key management endpoints |
| `internal/webhooks/service.go` | Create | Webhook dispatch |
| `internal/server/middleware.go` | Modify | Combined auth (JWT + API key) |
| `docs/api/openapi.yaml` | Create | OpenAPI 3.0 spec |
| `web/src/app/[locale]/admin/api-keys/*` | Create | Key management UI |

---

## Definition of Done

- [ ] API key auth works alongside JWT auth
- [ ] Keys scoped to specific cases + permissions
- [ ] Key plaintext returned once, only hash stored
- [ ] Rate limiting per key with proper headers
- [ ] Expired/revoked keys rejected immediately
- [ ] All API key actions logged to custody chain
- [ ] OpenAPI documentation complete and accurate
- [ ] Swagger UI accessible at /api/docs
- [ ] Key management UI for system admins
- [ ] Webhook delivery with HMAC signing
- [ ] 100% test coverage

---

## Security Checklist

- [ ] API key plaintext NEVER stored in database
- [ ] API key NEVER logged (only key ID)
- [ ] Key hash uses SHA-256 (not MD5/SHA-1)
- [ ] Rate limiting prevents abuse
- [ ] Case scope enforced at middleware level
- [ ] Read-only keys cannot perform writes
- [ ] Revocation is immediate (no grace period)
- [ ] Webhook secrets unique per endpoint
- [ ] Webhook payload doesn't include sensitive evidence content
- [ ] API docs don't expose internal implementation details

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 17 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/auth/apikey.go`** — middleware extracts key from `X-API-Key` header; extracts from `Authorization: ApiKey <key>` header; missing both headers falls through to JWT auth; both missing and no JWT returns 401
- **`internal/auth/apikey.go`** — key hash lookup: SHA-256 hash computed correctly; valid hash found in DB populates ApiKeyContext; unknown hash returns 401
- **`internal/auth/apikey.go`** — key validation: expired key returns 401; revoked key returns 401; active key with future expiry succeeds
- **`internal/auth/apikey.go`** — case scope enforcement: key scoped to case A accessing case A succeeds; key scoped to case A accessing case B returns 403; key scoped to multiple cases works for each
- **`internal/auth/apikey.go`** — permission enforcement: read-only key on GET succeeds; read-only key on POST/PUT/DELETE returns 403; read_write key on all methods succeeds
- **Rate limiting** — first request within limit succeeds; 61st request in same minute returns 429; `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` headers present on every response; `Retry-After` header present on 429; custom rate limit per key honored
- **`internal/apikeys/service.go`** — `CreateKey`: returns plaintext key starting with "vk_"; key is 64+ characters; SHA-256 hash stored in DB; plaintext never stored
- **`internal/apikeys/service.go`** — `ListKeys`: returns key info without plaintext; includes name, scoped cases, permissions, created date, last used
- **`internal/apikeys/service.go`** — `RevokeKey`: key marked revoked; subsequent auth attempts with that key fail; revocation is immediate
- **`internal/apikeys/service.go`** — `UpdateKey`: scope, permissions, expiry, rate limit modifiable; updated fields persisted
- **Custody logging** — key creation logged with key name and created_by (not plaintext); key revocation logged with key ID and revoked_by; API call logged with key ID (never key value)
- **`last_used_at` tracking** — updated on each authenticated request; not updated on failed auth
- **`internal/webhooks/service.go`** — webhook delivery: POST to configured URL with event payload; HMAC-SHA256 signature in `X-Webhook-Signature` header; signature verifiable with webhook secret
- **Webhook retry** — first failure retries up to 3 times; exponential backoff between retries; timeout at 10 seconds per attempt; delivery logged in database
- **Webhook filtering** — webhook only fires for subscribed events; webhook only fires for scoped case IDs; unsubscribed event does not trigger delivery
- **OpenAPI spec** — spec parses as valid OpenAPI 3.0; all endpoints documented; request/response schemas match actual API behavior

### Integration Tests (testcontainers)

- **API key auth end-to-end** — create key via admin endpoint, use key to authenticate GET request to evidence endpoint, verify 200 response with correct data
- **JWT + API key coexistence** — same endpoint accessible via JWT token and via API key; both return identical data for same user/scope
- **Rate limiting under concurrency** — send 100 concurrent requests with same API key (limit 60/min), verify exactly 60 succeed and 40 return 429
- **Key revocation propagation** — create key, make successful request, revoke key, make another request, verify 401
- **Webhook delivery end-to-end** — configure webhook, upload evidence, verify webhook POST received at target URL with correct payload and valid HMAC signature
- **Webhook retry on failure** — configure webhook to a URL that returns 500 twice then 200, verify 3 delivery attempts logged, final status is "delivered"
- **Custody chain integration** — create key, use key for 3 API calls, verify custody log has entries for key creation and all 3 API calls with key ID
- **Scoped key isolation** — create key scoped to case A, attempt to access case B evidence, verify 403; access case A evidence, verify 200

### E2E Automated Tests (Playwright)

- **API key creation flow** — log in as System Admin, navigate to API Keys page, fill creation form (name, cases, permissions, expiry), submit, verify plaintext key displayed once with copy button and warning
- **API key list display** — after creating 3 keys, verify all appear in table with name, scoped cases, permissions, status, and last used date (no plaintext shown)
- **API key revocation** — click revoke on a key, confirm dialog, verify key status changes to "revoked" in the list
- **One-time key display** — after creating a key, navigate away and back, verify plaintext key is no longer visible anywhere in the UI
- **Swagger UI accessible** — navigate to /api/docs, verify Swagger UI loads with all endpoints documented
- **Swagger UI try-it-out** — use Swagger UI's "Try it out" feature with a valid API key, verify successful response
- **Rate limit visible in headers** — make an API call via the external API, inspect response headers for `X-RateLimit-Limit` and `X-RateLimit-Remaining`
- **Webhook configuration UI** — if webhook management UI exists, create a webhook subscription, verify it appears in the list with URL, events, and status

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Log in as System Admin, navigate to Admin > API Keys, and create a new API key named "OSINT Integration" scoped to Case A with read-only permissions and 90-day expiry
   **Expected:** A plaintext key starting with "vk_" is displayed with a warning "This key will only be shown once" and a copy button
   **Verify:** Copy the key; refresh the page; confirm the plaintext key is no longer visible anywhere; confirm the key appears in the list with correct name, scope, permissions, and expiry date

2. [ ] **Action:** Using cURL or Postman, make a GET request to `/api/cases/:caseA/evidence` with the header `X-API-Key: <the key from step 1>`
   **Expected:** 200 response with evidence items from Case A; response includes `X-RateLimit-Limit: 60` and `X-RateLimit-Remaining` headers
   **Verify:** Response body matches expected evidence data; rate limit headers are present and correct

3. [ ] **Action:** Using the same read-only key, attempt a POST request to create evidence in Case A
   **Expected:** 403 Forbidden response indicating insufficient permissions
   **Verify:** Response body includes clear error message about read-only key; no evidence item was created

4. [ ] **Action:** Using the same key scoped to Case A, attempt a GET request to `/api/cases/:caseB/evidence`
   **Expected:** 403 Forbidden response indicating the key is not scoped to Case B
   **Verify:** No data from Case B is returned; error message is clear about scope restriction

5. [ ] **Action:** Send 65 rapid-fire GET requests within one minute using the same API key (with default 60/min limit)
   **Expected:** First 60 requests return 200; requests 61-65 return 429 Too Many Requests with `Retry-After` header
   **Verify:** 429 response includes `Retry-After` with seconds until next window; `X-RateLimit-Remaining` shows 0; after waiting for the reset window, requests succeed again

6. [ ] **Action:** Revoke the API key from the admin UI by clicking the revoke button and confirming
   **Expected:** Key status changes to "Revoked" immediately in the list
   **Verify:** Immediately attempt an API call with the revoked key; confirm 401 Unauthorized response; check custody log shows revocation event with admin's identity

7. [ ] **Action:** Navigate to `/api/docs` in a browser
   **Expected:** Swagger UI loads showing all API endpoints organized by resource, with request/response schemas, authentication methods, and error codes
   **Verify:** Spot-check 3 endpoints: evidence list, evidence upload, custody chain export; confirm schemas match actual API behavior; confirm code examples (cURL, Python, JavaScript) are present

8. [ ] **Action:** Configure a webhook for the "evidence_uploaded" event targeting a RequestBin or similar HTTP capture service, then upload an evidence item
   **Expected:** Webhook POST delivered to the target URL within 5 seconds of upload; payload includes event type, evidence ID, case ID, and timestamp
   **Verify:** Check the `X-Webhook-Signature` header; compute HMAC-SHA256 of the payload body using the webhook secret; confirm the signature matches

9. [ ] **Action:** Configure a webhook targeting a URL that is intentionally down (e.g., non-existent host), then trigger the subscribed event
   **Expected:** Webhook delivery fails; system retries 3 times with exponential backoff; delivery log shows 3 failed attempts
   **Verify:** Check webhook delivery log in admin UI or database; confirm 3 attempts with increasing intervals; confirm no further retries after the 3rd failure

10. [ ] **Action:** Create a read_write API key, use it to upload an evidence item via the API, then check the custody chain for that evidence item
    **Expected:** Custody chain includes an entry for the upload with the API key ID (not the key value) as the actor
    **Verify:** Custody log entry shows key ID, not plaintext key; actor is identified as the API key name; upload action recorded with correct timestamp
