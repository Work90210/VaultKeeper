# Sprint 1: Project Scaffolding & Infrastructure Foundation

**Phase:** 1 — Foundation
**Duration:** Weeks 1-2
**Goal:** Establish the complete development environment, Docker Compose stack, database schema, Go project structure, and CI pipeline so all subsequent sprints build on a solid, tested foundation.

---

## Prerequisites

- Go 1.22+ installed locally
- Node.js 20 LTS + pnpm installed locally
- Docker Desktop / Docker Engine + Docker Compose v2
- GitHub repository created (`vaultkeeper/vaultkeeper`, AGPL-3.0)
- Domain `vaultkeeper.eu` registered

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js scaffolding)
- [x] Infrastructure (Docker, CI)
- [x] Database (PostgreSQL schema + migrations)

---

## Implementation Steps

### Step 1: Initialize Go Module & Project Structure

**Deliverable:** Complete Go project skeleton following `internal/` package convention.

```
vaultkeeper/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── auth/
│   │   ├── middleware.go
│   │   ├── middleware_test.go
│   │   ├── permissions.go
│   │   └── permissions_test.go
│   ├── cases/
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   ├── service.go
│   │   ├── service_test.go
│   │   ├── repository.go
│   │   └── repository_test.go
│   ├── evidence/
│   │   ├── handler.go
│   │   ├── handler_test.go
│   │   ├── service.go
│   │   ├── service_test.go
│   │   ├── repository.go
│   │   ├── repository_test.go
│   │   └── storage.go
│   ├── custody/
│   │   ├── logger.go
│   │   ├── logger_test.go
│   │   ├── chain.go
│   │   ├── chain_test.go
│   │   ├── repository.go
│   │   └── repository_test.go
│   ├── integrity/
│   │   ├── verifier.go
│   │   ├── verifier_test.go
│   │   ├── tsa.go
│   │   └── tsa_test.go
│   ├── search/
│   │   ├── meilisearch.go
│   │   └── meilisearch_test.go
│   ├── notifications/
│   │   ├── service.go
│   │   ├── service_test.go
│   │   ├── repository.go
│   │   └── repository_test.go
│   ├── backup/
│   │   ├── runner.go
│   │   └── runner_test.go
│   ├── reports/
│   │   ├── pdf.go
│   │   └── pdf_test.go
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   └── server/
│       ├── server.go
│       └── routes.go
├── migrations/
├── web/
├── docker-compose.yml
├── docker-compose.dev.yml
├── Dockerfile
├── Caddyfile
├── .env.example
├── Makefile
├── README.md
├── LICENSE
├── CHANGELOG.md
├── .gitignore
├── .golangci.yml
└── .github/
    └── workflows/
        ├── ci.yml
        └── release.yml
```

**Tasks:**
1. `go mod init github.com/vaultkeeper/vaultkeeper`
2. Install core dependencies:
   - `github.com/go-chi/chi/v5` — HTTP router (lightweight, stdlib-compatible)
   - `github.com/jackc/pgx/v5` — PostgreSQL driver (best Go PG driver, connection pooling)
   - `github.com/golang-migrate/migrate/v4` — Database migrations
   - `github.com/minio/minio-go/v7` — MinIO client
   - `github.com/meilisearch/meilisearch-go` — Meilisearch client
   - `github.com/golang-jwt/jwt/v5` — JWT parsing/validation
   - `github.com/stretchr/testify` — Test assertions
   - `github.com/tus/tusd/v2` — tus resumable upload server
   - `github.com/google/uuid` — UUID generation
3. Create stub files for every package with interface definitions (no implementation yet)
4. Wire `cmd/server/main.go` with graceful shutdown, signal handling, config loading

**Tests:**
- `config_test.go`: Validate required env vars cause startup failure when missing
- `config_test.go`: Validate env var parsing (DATABASE_URL format, boolean parsing, integer parsing)
- `config_test.go`: Validate default values applied correctly

**SOLID Principles:**
- **S**: Each package owns one domain (cases, evidence, custody, etc.)
- **O**: Repository interfaces allow swapping implementations (real PG vs. test doubles)
- **L**: All repositories implement the same interface contract
- **I**: Narrow interfaces — `EvidenceReader`, `EvidenceWriter` not a god `EvidenceRepo`
- **D**: Services depend on interfaces, not concrete repositories

### Step 2: Environment Configuration Module

**Deliverable:** `internal/config/config.go` — typed config struct loaded from env vars with validation.

```go
// Pseudo-structure
type Config struct {
    DatabaseURL       string   // required, validated as postgres:// URL
    MinIOEndpoint     string   // required
    MinIOAccessKey    string   // required
    MinIOSecretKey    string   // required, min 8 chars
    MinIOBucket       string   // required
    MinIOUseSSL       bool     // default: true
    KeycloakURL       string   // required, validated as URL
    KeycloakRealm     string   // required
    KeycloakClientID  string   // required
    TSAURL            string   // required when TSAEnabled=true
    TSAEnabled        bool     // default: true
    MeilisearchURL    string   // required
    MeilisearchAPIKey string   // required
    SMTPHost          string   // optional
    SMTPPort          int      // default: 587
    SMTPUsername      string   // optional
    SMTPPassword      string   // optional
    SMTPFrom          string   // optional
    AppURL            string   // required, validated as URL
    AppEnv            string   // required: development|staging|production
    LogLevel          string   // default: info
    MaxUploadSize     int64    // default: 10GB
    BackupDestination string   // optional
    BackupEncKey      string   // required when BackupDestination set
    CORSOrigins       []string // default: empty (Caddy handles in prod)
    ServerPort        int      // default: 8080

    // Encryption (used by later sprints but validated from day one)
    WitnessEncryptionKey string // required in production (witness identity encryption, Sprint 7)
    MasterEncryptionKey  string // required in production (per-case encryption, Sprint 18)

    // Session management
    MaxConcurrentSessions int   // default: 3 (per user, configurable per institution)

    // Case reference code validation
    CaseReferenceRegex   string // default: "^[A-Z]+-[A-Z]+-\\d{4}(-\\d+)?$" (configurable per institution)

    // Archive storage
    ArchiveStorageBucket string // optional, separate MinIO bucket for cold storage
    ArchiveStoragePath   string // optional, SFTP path for archived case files
}
```

**Validation rules:**
- All required vars must be present and non-empty
- URLs validated with `url.Parse` — must have scheme + host
- DATABASE_URL must start with `postgres://` or `postgresql://`
- MaxUploadSize must be positive integer
- AppEnv must be one of: development, staging, production
- LogLevel must be one of: debug, info, warn, error
- If SMTP fields partially set, warn (all-or-nothing)
- NEVER log secret values — log key names only on validation failure

**Tests (100% coverage):**
- All required vars present → config loads successfully
- Each required var missing → specific error message
- Invalid DATABASE_URL format → error
- Invalid URL format for each URL field → error
- Boolean parsing: "true", "false", "1", "0", "", missing
- Integer parsing: valid, negative, zero, non-numeric
- Default values applied when optional vars absent
- Partial SMTP config → warning logged
- All sensitive fields redacted in String()/debug output
- WitnessEncryptionKey / MasterEncryptionKey → required when AppEnv=production, optional in development
- MaxConcurrentSessions → positive integer, default 3
- CaseReferenceRegex → valid regex, default pattern validates correctly
- ArchiveStorageBucket → optional, validated as non-empty if set

### Step 3: Database Schema & Migrations

**Deliverable:** Initial migration files creating all 9 core tables + indexes + RLS policies.

**Migration 001: `001_initial_schema.up.sql`**
- Create all 9 tables exactly as specified in the data model
- Add `CHECK` constraints:
  - `cases.status IN ('active', 'closed', 'archived')`
  - `case_roles.role IN ('investigator', 'prosecutor', 'defence', 'judge', 'observer', 'victim_representative')`
  - `evidence_items.classification IN ('public', 'restricted', 'confidential', 'ex_parte')`
  - `witnesses.protection_status IN ('standard', 'protected', 'high_risk')`
  - `api_keys.permissions IN ('read', 'read_write')`
  - `backup_log.status IN ('started', 'completed', 'failed')`
- Add `NOT NULL` on all foreign keys
- Add `ON DELETE` policies:
  - `case_roles.case_id` → CASCADE (deleting a case removes role assignments)
  - `evidence_items.case_id` → RESTRICT (cannot delete case with evidence)
  - `custody_log.evidence_id` → RESTRICT (cannot delete evidence with custody entries)
  - `custody_log.case_id` → RESTRICT
  - `disclosures.case_id` → RESTRICT
  - `witnesses.case_id` → RESTRICT
  - `notifications.case_id` → SET NULL

**Migration 001: `001_initial_schema.down.sql`**
- Drop all tables in reverse dependency order
- Drop RLS policies first

**Migration 002: `002_indexes.up.sql`**
- All indexes from the spec (11 indexes)
- Partial indexes where specified (`WHERE is_current = true`, `WHERE read = false`, `WHERE revoked_at IS NULL`)

**Migration 002: `002_indexes.down.sql`**
- Drop all indexes

**Migration 003: `003_custody_rls.up.sql`**
- Enable Row-Level Security on `custody_log`
- Create policy: `INSERT` allowed for application role
- Create policy: `SELECT` allowed for application role
- **NO** `UPDATE` or `DELETE` policies — append-only enforcement
- Create separate `vaultkeeper_app` role for the application
- Create separate `vaultkeeper_readonly` role for reporting

**Migration 003: `003_custody_rls.down.sql`**
- Disable RLS on `custody_log`
- Drop policies and roles

**Migration runner in Go:**
- Auto-run pending migrations on server startup
- Log each migration applied
- Fail startup if any migration fails (fail fast)
- Lock to prevent concurrent migration from multiple instances

**Tests:**
- Migration up → all tables exist with correct columns and types
- Migration down → all tables removed cleanly
- Re-run up after down → clean state
- RLS policy: INSERT to custody_log succeeds
- RLS policy: UPDATE to custody_log fails with permission denied
- RLS policy: DELETE from custody_log fails with permission denied
- All CHECK constraints enforced (insert invalid status → error)
- Foreign key constraints enforced (insert evidence with invalid case_id → error)
- Unique constraints enforced (duplicate case_roles → error)
- Indexes exist and are used (EXPLAIN ANALYZE on key queries):
  - `idx_evidence_case_id` used by evidence-by-case query
  - `idx_evidence_case_current` used by current-version-only query
  - `idx_custody_evidence_id` used by custody-by-evidence query
  - `idx_case_roles_user` used by case-list-for-user query
  - `idx_notifications_user_unread` used by unread-count query
  - `idx_api_keys_hash` used by API key lookup query
  - Partial index conditions verified (WHERE is_current = true, WHERE read = false, WHERE revoked_at IS NULL)

**Security:**
- Application connects as `vaultkeeper_app` role, NOT superuser
- Migrations connect as superuser only during migration
- RLS enforced for all non-superuser connections
- `custody_log` is truly append-only at the database level

### Step 4: Docker Compose Stack

**Deliverable:** Complete `docker-compose.yml` + `docker-compose.dev.yml` for local development.

**Services:**

| Service | Image | Ports (internal) | Volumes |
|---------|-------|-------------------|---------|
| `api` | Built from Dockerfile | 8080 | None (stateless) |
| `caddy` | caddy:2-alpine | 80, 443 | caddy_data, caddy_config |
| `postgres` | postgres:16-alpine | 5432 | pg_data |
| `minio` | minio/minio:latest | 9000, 9001 (console) | minio_data |
| `keycloak` | quay.io/keycloak/keycloak:24.0 | 8443 | keycloak_data |
| `meilisearch` | getmeili/meilisearch:v1.7 | 7700 | meili_data |

**docker-compose.yml (production):**
- All services on internal `vaultkeeper` network
- Only Caddy exposed to host (ports 80, 443)
- Postgres, MinIO, Meilisearch NOT exposed to host
- Keycloak exposed only on 8443 (admin setup, then lock down)
- Health checks on all services
- Restart policy: `unless-stopped`
- Resource limits per service
- Named volumes for persistence

**docker-compose.dev.yml (development override):**
- Postgres exposed on localhost:5432 (for local tooling)
- MinIO console exposed on localhost:9001
- Meilisearch exposed on localhost:7700
- Hot-reload for Go (via `air` or `watchexec`)
- Hot-reload for Next.js (default dev server)
- No TLS (Caddy in HTTP mode)
- Verbose logging

**Caddyfile:**
```
{your-domain} {
    reverse_proxy api:8080
    
    header {
        Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
        X-Frame-Options "DENY"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "no-referrer"
        Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; connect-src 'self'; frame-src 'none'"
    }
    
    rate_limit {
        zone authenticated {
            key {http.auth.bearer.sub}
            events 100
            window 1m
        }
        zone upload {
            key {http.auth.bearer.sub}
            events 10
            window 1m
        }
        zone search {
            key {http.auth.bearer.sub}
            events 30
            window 1m
        }
    }
    
    # /health endpoint excluded from rate limiting (monitoring needs frequent checks)
}
```

**Dockerfile (multi-stage):**
```
# Stage 1: Build Go binary
FROM golang:1.22-alpine AS go-builder
# ...

# Stage 2: Build Next.js
FROM node:20-alpine AS web-builder
# ...

# Stage 3: Runtime
FROM alpine:3.19
# Copy Go binary + Next.js static build
# Single image, single process (Go serves API + static files)
```

**.env.example:**
- All variables from the config spec
- Comments explaining each variable
- Secure default passwords clearly marked as "CHANGE ME"

**Tests:**
- `docker compose config` validates compose file syntax
- `docker compose up` starts all services (integration test)
- All health checks pass within 60 seconds
- Postgres accepts connections from api container
- MinIO accepts connections from api container
- Meilisearch accepts connections from api container
- Keycloak serves login page
- Caddy proxies to api container
- Services on internal network not reachable from host (prod config)

### Step 5: Structured Logging Setup

**Deliverable:** `slog`-based structured logging with request context.

**Implementation:**
- Use Go stdlib `log/slog` with JSON handler for production, text handler for development
- Middleware that injects request_id (UUID) into context for every request
- All log entries include: timestamp, level, request_id, user_id (from JWT), endpoint, method
- Log levels controlled by `APP_LOG_LEVEL` env var
- **NEVER** log: file contents, JWT tokens, passwords, API keys, witness identities, MinIO secrets

**Request logging middleware:**
- Log request start: method, path, user_id
- Log request end: method, path, status_code, duration_ms
- Log errors with full context but no sensitive data
- Skip logging for `/health` endpoint (too noisy from monitoring)

**Log rotation & retention:**
- Logs written to stdout (Docker captures via `docker logs`)
- Log retention: 90 days on the server (configured via Docker logging driver)
- In production: recommend `json-file` driver with `max-size: 50m`, `max-file: 30`
- For centralized logging: stdout compatible with Loki, Fluentd, or any log shipper

**Sensitive field redaction list (compile-time enforced):**
- JWT tokens (any field containing "token", "jwt", "bearer")
- Passwords (any field containing "password", "secret", "key")
- Witness identity fields (full_name, contact_info, location)
- API key plaintext values
- File content / evidence body
- MinIO credentials

**Tests:**
- Log output contains all required fields (timestamp, request_id, etc.)
- Sensitive fields are never present in log output (fuzz test with known sensitive values)
- Log level filtering works (DEBUG messages not logged at INFO level)
- Request ID propagates through middleware chain
- Health endpoint is excluded from request logging
- Structured JSON output parseable by log aggregators
- Log rotation config validated in Docker Compose

### Step 6: CI/CD Pipeline (GitHub Actions)

**Deliverable:** `.github/workflows/ci.yml` + `.github/workflows/release.yml`

**ci.yml (runs on every push/PR):**
```yaml
jobs:
  lint:
    - golangci-lint run
    - eslint (Next.js)
    - prettier check
  
  test-unit:
    - go test ./... -race -count=1 -coverprofile=coverage.out
    - coverage must be >= 80%
    
  test-integration:
    services:
      - postgres:16
      - minio
      - meilisearch
    steps:
      - go test ./... -tags=integration -race
      
  build:
    - docker build --target go-builder .
    - docker build --target web-builder .
    - docker build . (full image)
    
  security:
    - gosec ./...
    - trivy image scan
    - npm audit (Next.js)
```

**release.yml (runs on tag push v*):**
```yaml
jobs:
  test: (all of ci.yml)
  build-push:
    - Build Docker image
    - Tag: latest, semver (1.2.3), major.minor (1.2)
    - Push to GitHub Container Registry (ghcr.io/vaultkeeper/vaultkeeper)
  changelog:
    - Auto-generate CHANGELOG.md from conventional commits (release-please or standard-version)
    - Format: Keep a Changelog (https://keepachangelog.com)
    - Sections: Added, Changed, Fixed, Security, Deprecated, Removed
  deploy-staging:
    - Deploy to staging instance
    - Run E2E tests against staging
    - Manual approval gate before production
```

**CHANGELOG.md format:**
```markdown
# Changelog
## [1.0.0] - 2026-XX-XX
### Added
- Case CRUD with reference codes and jurisdiction tracking
- Evidence upload with SHA-256 hashing and RFC 3161 timestamping
...
### Security
- TLS 1.3 on all connections
- RLS on custody_log (append-only)
...
```

**Makefile targets:**
```makefile
dev          # docker compose -f docker-compose.yml -f docker-compose.dev.yml up
build        # go build + next build
test         # go test ./... -race
test-int     # go test -tags=integration
lint         # golangci-lint + eslint
migrate-up   # run migrations
migrate-down # rollback last migration
migrate-new  # create new migration pair
coverage     # go test -coverprofile + go tool cover
docker       # docker build
clean        # remove build artifacts
```

**Tests:**
- CI pipeline runs successfully on clean checkout
- Lint catches known violations
- Test job reports coverage percentage
- Build produces valid Docker image
- Image starts and responds on `/health`

### Step 7: Next.js Frontend Scaffolding

**Deliverable:** Next.js 14 app with app router, i18n, and design system foundation.

**Setup:**
```
web/
├── src/
│   ├── app/
│   │   ├── [locale]/
│   │   │   ├── layout.tsx
│   │   │   ├── page.tsx
│   │   │   ├── login/
│   │   │   │   └── page.tsx
│   │   │   ├── cases/
│   │   │   │   ├── page.tsx
│   │   │   │   └── [id]/
│   │   │   │       └── page.tsx
│   │   │   └── settings/
│   │   │       └── page.tsx
│   │   └── api/
│   │       └── health/
│   │           └── route.ts
│   ├── components/
│   │   ├── ui/          # shadcn/ui base components
│   │   ├── layout/      # Shell, Sidebar, Header
│   │   └── shared/      # Reusable business components
│   ├── lib/
│   │   ├── api.ts       # Typed API client (fetch wrapper)
│   │   ├── auth.ts      # Keycloak OIDC client helpers
│   │   └── utils.ts     # Shared utilities
│   ├── hooks/
│   │   └── use-auth.ts
│   ├── types/
│   │   └── index.ts     # Shared TypeScript types matching Go models
│   └── messages/
│       ├── en.json      # English translations
│       └── fr.json      # French (empty, ready for Phase 2)
├── next.config.ts
├── tailwind.config.ts
├── tsconfig.json
├── package.json
├── playwright.config.ts
└── tests/
    └── e2e/
        └── health.spec.ts
```

**Key decisions:**
- `next-intl` configured from day one — all strings via `useTranslations()`
- `shadcn/ui` for component library (Radix primitives, Tailwind styling)
- Typed API client matching Go API response envelope
- Keycloak OIDC flow via `next-auth` with Keycloak provider
- Strict TypeScript (`strict: true`, no `any`)
- **State management:** `@tanstack/react-query` for server state (API data fetching, caching, invalidation). No Redux/Zustand needed — server state is the primary concern, component state is minimal.
- **Form handling:** `react-hook-form` + `zod` for schema validation (matches Go-side validation)
- **Build output:** Static export for marketing pages (`output: 'export'` for /), SSR for authenticated app pages
- **Image optimization:** `next/image` with MinIO as image source, thumbnails served from Go API
- **Font optimization:** `next/font/google` for Inter or system font stack
- **Bundle analysis:** `@next/bundle-analyzer` in CI to track bundle size regression

**Conventional commits enforcement:**
- `commitlint` configured in CI (`feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`, `perf:`, `ci:`)
- Changelog auto-generated from conventional commits via `standard-version` or `release-please`
- Git hooks via `husky` + `lint-staged` for pre-commit linting

**API client pattern:**
```typescript
// Typed response envelope matching Go API
interface ApiResponse<T> {
  data: T | null;
  error: string | null;
  meta?: { total: number; next_cursor: string; has_more: boolean };
}

// Typed fetch wrapper with auth token injection
async function api<T>(path: string, options?: RequestInit): Promise<ApiResponse<T>>
```

**Tests:**
- Next.js builds without errors (`next build`)
- i18n routing works (en, fr locales)
- API client handles success/error responses
- Health page renders
- TypeScript strict mode passes with zero errors
- ESLint + Prettier pass

### Step 8: Keycloak Realm Configuration

**Deliverable:** Pre-configured Keycloak realm export for VaultKeeper.

**Realm: `vaultkeeper`**
- Client: `vaultkeeper-api` (confidential, service account enabled)
- Client: `vaultkeeper-web` (public, PKCE flow)
- Realm roles: `system_admin`, `case_admin`, `user`, `api_service`
- Default role: `user`
- Token settings:
  - Access token lifetime: 15 minutes
  - Refresh token lifetime: 8 hours
  - Refresh token rotation: enabled
- Password policy: 12+ characters, 1 uppercase, 1 number, 1 special
- Brute force protection: 5 failures → 15 min lockout
- Required actions: MFA enrollment (optional, configurable per institution)
- Custom token mapper: include `system_role` claim in JWT
- CORS: configured for `APP_URL`

**SSO Federation (LDAP/AD) preparation:**
- Realm export includes LDAP federation provider template (disabled by default)
- LDAP config fields: connection URL, bind DN, user search base, group mapping
- SAML Identity Provider broker template (disabled by default)
- User provisioning: Keycloak syncs users from LDAP on login or via scheduled sync
- Group-to-role mapping: LDAP groups map to VaultKeeper system roles
- Documentation: step-by-step guide for institution IT to enable LDAP/SAML federation
- This is configuration-only — institutions enable it themselves. VaultKeeper validates JWT tokens regardless of how Keycloak authenticated the user.

**Export as JSON** for reproducible setup in Docker.

**Tests:**
- Keycloak starts with imported realm
- OIDC discovery endpoint accessible
- Token exchange works (username/password → JWT)
- JWT contains expected claims (sub, system_role, email)
- Token refresh works
- Expired tokens rejected
- Brute force lockout activates after 5 failures
- Password policy enforced
- LDAP federation template present in realm export (disabled)
- SAML broker template present in realm export (disabled)
- Concurrent session limit configurable (default 3 per user)

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `cmd/server/main.go` | Create | Entry point, DI wiring, graceful shutdown |
| `internal/config/config.go` | Create | Env var loading + validation |
| `internal/server/server.go` | Create | HTTP server setup, middleware chain |
| `internal/server/routes.go` | Create | Route registration (stubs) |
| `migrations/001_initial_schema.up.sql` | Create | 9 core tables |
| `migrations/002_indexes.up.sql` | Create | 11 indexes |
| `migrations/003_custody_rls.up.sql` | Create | RLS append-only enforcement |
| `docker-compose.yml` | Create | Full production stack |
| `docker-compose.dev.yml` | Create | Dev overrides |
| `Dockerfile` | Create | Multi-stage build |
| `Caddyfile` | Create | Reverse proxy + security headers |
| `.env.example` | Create | All env vars documented |
| `Makefile` | Create | Dev workflow targets |
| `.github/workflows/ci.yml` | Create | CI pipeline |
| `.golangci.yml` | Create | Linter config |
| `web/` (entire tree) | Create | Next.js scaffolding |
| `keycloak/realm-export.json` | Create | Pre-configured realm |

---

## Definition of Done

- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes with >= 80% coverage on config package
- [ ] `docker compose up` starts all 6 services
- [ ] All service health checks pass
- [ ] Postgres migrations run successfully on startup
- [ ] RLS policies prevent UPDATE/DELETE on custody_log
- [ ] Keycloak realm imported, token exchange works
- [ ] Next.js builds and serves at localhost:3000
- [ ] i18n routing works (en/fr)
- [ ] CI pipeline runs green on GitHub Actions
- [ ] `make test`, `make lint`, `make build` all pass
- [ ] No hardcoded secrets in codebase
- [ ] `.env.example` documents all variables
- [ ] AGPL-3.0 LICENSE file present
- [ ] README with setup instructions

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Keycloak config complexity | Use realm export JSON for reproducible setup; test in CI |
| Docker Compose networking issues | Integration test that verifies cross-service connectivity |
| Migration runner race condition (multi-instance) | Advisory lock in Postgres during migration |
| Go module dependency conflicts | Pin all dependencies to exact versions in go.sum |
| Next.js + Go single-image build complexity | Test multi-stage Dockerfile in CI; keep fallback to separate images |

---

## Security Checklist

- [ ] No secrets in source code (all from env vars)
- [ ] `.env` in `.gitignore`
- [ ] Postgres RLS enabled on custody_log
- [ ] Application uses non-superuser DB role
- [ ] Caddy sets all security headers (HSTS, CSP, X-Frame-Options, etc.)
- [ ] Docker internal network isolates DB/MinIO/Meilisearch
- [ ] Keycloak brute force protection enabled
- [ ] JWT token lifetimes configured (15 min access, 8 hr refresh)
- [ ] `gosec` runs in CI
- [ ] `trivy` scans Docker image in CI
- [ ] No services expose ports to host in production compose

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 1 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/config/config.go`**: Every validation path — required vars present, each required var missing, invalid URL formats, invalid DATABASE_URL, boolean parsing edge cases ("true", "false", "1", "0", empty, missing), integer parsing (valid, negative, zero, non-numeric), default values, partial SMTP config warning, sensitive field redaction in `String()`, WitnessEncryptionKey/MasterEncryptionKey required in production, MaxConcurrentSessions positive integer + default, CaseReferenceRegex valid regex + default pattern, ArchiveStorageBucket optional validation
- **`cmd/server/main.go`**: Graceful shutdown on SIGTERM, SIGINT signal handling, config loading failure exits with non-zero code
- **Migration runner**: Pending migrations applied in order, failed migration aborts startup, advisory lock prevents concurrent migration, re-run after rollback produces clean state
- **Logging middleware**: JSON output contains all required fields (timestamp, request_id, level, user_id, endpoint, method), sensitive fields never present in output, log level filtering, request ID propagation, `/health` excluded from request logging

### Integration Tests (with testcontainers)

- **Postgres (testcontainers/postgres:16-alpine)**: Migration 001 up creates all 9 tables with correct columns/types, migration down removes all tables, re-run up after down produces clean state, RLS policy INSERT on custody_log succeeds, RLS policy UPDATE on custody_log fails with permission denied, RLS policy DELETE on custody_log fails with permission denied, all CHECK constraints enforced, foreign key constraints enforced, unique constraints enforced, all 11 indexes exist and are used (EXPLAIN ANALYZE)
- **MinIO (testcontainers/minio)**: API container can connect and create/read/delete objects, SSE-S3 encryption headers present on stored objects, bucket policy prevents public access
- **Meilisearch (testcontainers/meilisearch:v1.7)**: API container can connect and create/query indexes
- **Keycloak (testcontainers/keycloak:24.0)**: Realm import succeeds, OIDC discovery endpoint accessible, token exchange works (username/password to JWT), JWT contains expected claims (sub, system_role, email), token refresh works, expired tokens rejected, brute force lockout after 5 failures, password policy enforced
- **Docker Compose full stack**: All 6 services start and health checks pass within 60 seconds, cross-service connectivity verified (API to Postgres, API to MinIO, API to Meilisearch, Caddy to API)

### E2E Automated Tests (Playwright)

- **`tests/e2e/health.spec.ts`**: Navigate to `/health` endpoint, verify JSON response contains `"status": "healthy"` and `"version"` field
- **`tests/e2e/scaffold-smoke.spec.ts`**: Verify Next.js app loads at localhost:3000, verify i18n routing works (`/en` and `/fr` both render), verify login page renders with Keycloak redirect button, verify strict TypeScript build produces zero errors

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Run `docker compose up` from a clean checkout with only `.env` configured
   **Expected:** All 6 services (api, caddy, postgres, minio, keycloak, meilisearch) start within 120 seconds
   **Verify:** `docker compose ps` shows all services as "healthy"

2. [ ] **Action:** Check Postgres migrations by connecting to the database
   **Expected:** All 9 core tables exist with correct columns, 11 indexes created, RLS enabled on custody_log
   **Verify:** Run `\dt` in psql, confirm tables; run `\di` to confirm indexes; attempt `UPDATE custody_log SET action='x'` as vaultkeeper_app role and confirm it fails

3. [ ] **Action:** Open Keycloak admin console at localhost:8443
   **Expected:** `vaultkeeper` realm exists with both clients (vaultkeeper-api, vaultkeeper-web), realm roles present
   **Verify:** Navigate to Realm Settings, confirm token lifetimes (15 min access, 8 hr refresh), confirm brute force protection enabled, confirm password policy (12+ chars)

4. [ ] **Action:** Open MinIO console at localhost:9001
   **Expected:** `evidence` bucket exists with SSE-S3 encryption enabled
   **Verify:** Upload a test file via console, confirm encryption icon appears on the object

5. [ ] **Action:** Open Meilisearch dashboard at localhost:7700
   **Expected:** Meilisearch is running and accessible
   **Verify:** Hit `/health` endpoint, confirm `{"status":"available"}`

6. [ ] **Action:** Open Next.js app at localhost:3000
   **Expected:** App loads, displays login page or landing page
   **Verify:** Navigate to `/en` and `/fr` locales, confirm both render without errors; check browser console for zero JavaScript errors

7. [ ] **Action:** Run `make test` from the project root
   **Expected:** All unit tests pass with >= 80% coverage on config package
   **Verify:** Coverage report shows no uncovered lines in critical paths (config validation, migration runner)

8. [ ] **Action:** Run `make lint` from the project root
   **Expected:** golangci-lint and ESLint both pass with zero violations
   **Verify:** Exit code 0, no warnings or errors in output

9. [ ] **Action:** Run `make build` to build the multi-stage Docker image
   **Expected:** Image builds successfully, Go binary and Next.js static output both present in final image
   **Verify:** `docker run --rm <image> /app/server --version` prints version; image size is reasonable (< 500MB)

10. [ ] **Action:** Push a commit to a feature branch and verify CI pipeline
    **Expected:** GitHub Actions CI runs all jobs (lint, test-unit, test-integration, build, security)
    **Verify:** All jobs green, coverage report uploaded, Docker image built, `gosec` and `trivy` produce no critical findings

11. [ ] **Action:** Verify `.env.example` contains all required variables
    **Expected:** Every variable referenced in `internal/config/config.go` is documented in `.env.example`
    **Verify:** Diff the two files; no undocumented variables exist; all passwords marked "CHANGE ME"

12. [ ] **Action:** Grep entire codebase for hardcoded secrets
    **Expected:** Zero hardcoded API keys, passwords, tokens, or encryption keys
    **Verify:** `grep -rn "password\|secret\|api_key\|token" --include="*.go" --include="*.ts"` returns only variable names/config references, never literal values
