# VaultKeeper — Sovereign Evidence Management Platform

## Product Plan & Technical Blueprint

---

## What This Is

A self-hosted, open-source evidence management system for international courts, tribunals, and human rights organizations. It replaces US-sanctionable tools like RelativityOne with a sovereignty-preserving alternative that institutions deploy on their own infrastructure.

**One-liner:** "The evidence locker that no foreign government can shut off."

---

## The Problem

The ICC paid $2.5M over 5 years for RelativityOne (US company, runs on Microsoft Azure). US sanctions now threaten access to that platform. 400+ international organizations in The Hague are rethinking their entire software stack. Nobody has built a sovereign, open-source alternative for evidence management.

---

## Target Customers (in order of approach)

1. **Small-to-mid NGOs in The Hague** — Humanity Hub orgs, documentation NGOs, transitional justice institutes (HiiL, T.M.C. Asser, CIVIC). 20-100 staff. No IT team. Need something simple and self-hosted.
2. **Mid-tier international bodies** — OPCW, Eurojust, Kosovo Specialist Chambers, Residual Mechanism for Criminal Tribunals. Dedicated IT, procurement processes, specific compliance requirements.
3. **The ICC and equivalents** — 800+ staff, complex multi-case environments, field operations in conflict zones. The whale contract.
4. **Expansion: International tribunals globally** — Arusha, Phnom Penh, Freetown, Pristina. Same needs, same sovereignty concerns.

---

## Tech Stack

| Layer | Choice | Why |
|-------|--------|-----|
| Backend | Go | Single binary deployment, no runtime deps, stdlib has crypto/http/io, scales without drama |
| Frontend | Next.js (React) | SSR for marketing pages, mature ecosystem for data tables/grids/document preview, every future hire knows React. Use next-intl for i18n from day one — ICC works in English + French, Europol in all EU languages |
| Database | PostgreSQL | JSONB for flexible evidence metadata, battle-tested, every sysadmin knows it |
| File Storage | MinIO (S3-compatible) | Self-hosted object storage, evidence files stay off the database, scales to petabytes |
| Auth | Keycloak | OIDC/SAML, open source, institutions already know it, handles complex role hierarchies |
| Search | Meilisearch | Simpler than Elasticsearch, fast full-text search, easy to self-host |
| Reverse Proxy | Caddy | Auto TLS via Let's Encrypt, rate limiting, zero-config HTTPS |
| Deployment | Docker Compose | One file, `docker compose up`, done. No Kubernetes required for initial deployments |
| Infrastructure | Terraform (Hetzner provider) | Programmatic provisioning of managed hosting instances. One `terraform apply` per new customer |
| Configuration | Ansible | Playbooks for server setup, Docker deployment, firewall config, backup scheduling |
| Monitoring | Uptime Kuma | Self-hosted, pings `/health` on all managed instances every 60s, alerts to phone |
| Backups | Hetzner Storage Box + rsync | Encrypted nightly Postgres dumps + MinIO snapshots to separate physical location |
| CI/CD | GitHub Actions | Standard, free for open source. Also triggers managed instance updates on new releases |
| License | AGPL-3.0 | Copyleft ensures sovereignty, same as Nextcloud. Prevents proprietary forks while allowing self-hosting |

---

## Architecture (Keep It Stupid Simple)

```
┌──────────────────────────────────────────────────┐
│                   Next.js (React)                  │
│         (Case UI, Evidence Grid, Timeline)          │
└──────────────────┬───────────────────────────────┘
                   │ HTTPS
┌──────────────────▼───────────────────────────────┐
│              Caddy (Reverse Proxy)                │
│          (Auto TLS / Let's Encrypt)               │
└──────────────────┬───────────────────────────────┘
                   │ REST API (JSON)
┌──────────────────▼───────────────────────────────┐
│                  Go API Server                    │
│  ┌─────────┐ ┌──────────┐ ┌───────────────────┐  │
│  │  Auth    │ │  Cases   │ │  Evidence Handler │  │
│  │Middleware│ │  Service │ │(hash + TSA + store)│  │
│  └─────────┘ └──────────┘ └───────────────────┘  │
│  ┌─────────────────┐ ┌────────────────────────┐  │
│  │  Custody Logger  │ │  Report Generator     │  │
│  │(append-only,     │ │  (PDF export)         │  │
│  │ hash-chained)    │ │                       │  │
│  └─────────────────┘ └────────────────────────┘  │
│  ┌─────────────────┐ ┌────────────────────────┐  │
│  │  Backup Runner   │ │  Notification Service │  │
│  │  (daily cron)    │ │  (in-app + SMTP)      │  │
│  └─────────────────┘ └────────────────────────┘  │
└───┬──────────────┬──────────────┬────────────────┘
    │              │              │        ┌──────────────┐
┌───▼───┐    ┌─────▼─────┐  ┌────▼────┐   │  RFC 3161    │
│Postgres│    │   MinIO   │  │Meili-   │   │  Timestamp   │
│ (data, │    │  (files,  │  │search   │   │  Authority   │
│  TLS)  │    │SSE encryp)│  │(search) │   │  (external)  │
└────────┘    └───────────┘  └─────────┘   └──────────────┘
```

**Key architectural decisions:**

- Evidence files NEVER touch the database. They go straight to MinIO. Postgres stores metadata + hash only.
- Every API call that touches evidence writes to the custody log. No exceptions. This is middleware, not optional.
- The Go server is stateless. You can run multiple instances behind a load balancer for larger deployments.
- Auth is fully delegated to Keycloak. The Go server validates JWT tokens. No user management code in the app.

---

## Security Architecture

This is non-negotiable. You're storing war crimes evidence and witness identities. An IT security officer will interrogate this in the first meeting.

**Encryption in transit:**
- TLS 1.3 everywhere. API server, MinIO, Postgres connections, Keycloak — all TLS. No exceptions.
- Docker Compose ships with a Caddy reverse proxy that auto-provisions Let's Encrypt certificates. Zero-config HTTPS.

**Encryption at rest:**
- MinIO server-side encryption (SSE-S3) enabled by default. Every evidence file is encrypted on disk. If someone steals the hard drive, they get ciphertext.
- PostgreSQL Transparent Data Encryption (TDE) or full-disk encryption (LUKS) on the host. The custody log, evidence metadata, and witness identities are encrypted at rest.
- Encryption keys managed via environment variables or a secrets manager (HashiCorp Vault for institutional deployments). Keys never stored alongside data.

**Per-evidence encryption (Phase 2):**
- Optional per-case encryption keys. Evidence in Case A is encrypted with Key A. Compromise of one case's key doesn't expose other cases.
- Key derivation from case-level secrets, managed through Keycloak or institution's existing KMS.

**RFC 3161 Trusted Timestamping:**
- On every evidence upload, the SHA-256 hash is sent to an independent RFC 3161 Timestamp Authority (TSA). The TSA returns a signed timestamp token proving the hash existed at that specific moment.
- Uses free European TSA services (FreeTSA.org, or national services like the Italian AgID TSA).
- This is critical for evidentiary value: without it, a defence lawyer can argue the hash was computed yesterday and backdated. With an independent timestamp, that argument collapses.
- The timestamp token is stored alongside the evidence item in Postgres.
- Implementation: ~20 lines of Go using `crypto/x509` and an HTTP call to the TSA endpoint.

**Immutable custody log:**
- The custody_log table is append-only. No UPDATE or DELETE operations permitted at the application level.
- Postgres Row-Level Security (RLS) policy enforces this. Even a database admin cannot silently modify historical log entries.
- Optional: custody log entries are hash-chained (each entry includes the hash of the previous entry), creating a tamper-evident linked list. Any modification breaks the chain.

**Network isolation:**
- MinIO and Postgres are not exposed to the internet. Only the Go API server and Keycloak have public-facing ports.
- Docker Compose internal networking ensures database and storage are only reachable from the application container.

**No outbound calls (air-gap compatible):**
- The application makes zero outbound network calls by default. No telemetry, no analytics, no phoning home.
- RFC 3161 timestamping is the one optional outbound call, and can be disabled for air-gapped deployments (falls back to local system clock with a logged warning).
- AI features (Phase 3) run entirely on-premises via Whisper/Ollama. No data leaves the server.

---

## Hetzner Infrastructure

All managed hosting runs on Hetzner Cloud (Germany — Falkenstein or Nuremberg datacenters). EU data residency, GDPR-compliant, no US jurisdiction.

**Server sizing per tier:**

| Tier | Server | Specs | Backup Storage | Monthly Infra Cost |
|------|--------|-------|---------------|-------------------|
| Starter (≤25 users, 500GB) | CPX31 | 4 vCPU, 8GB RAM, 160GB NVMe | Storage Box 1TB | ~€25-35/month |
| Professional (≤100 users, 2TB) | CPX41 | 8 vCPU, 16GB RAM, 240GB NVMe | Storage Box 5TB | ~€55-75/month |
| Institution (unlimited) | AX42 Dedicated | 8 cores, 64GB RAM, 2x512GB NVMe | Storage Box 10TB | ~€55-80/month |

**Data residency note:** Hetzner has no Netherlands datacenter. "Hosted in Germany" is fine under GDPR (EU-to-EU, no transfer issue). If an institution specifically requires Dutch data residency, offer deployment on their own infrastructure or use a Dutch provider (LeafCloud, TransIP) for that client. This affects maybe 5% of customers.

**Hetzner Storage Boxes for backups:**
- Separate physical location from the application server
- Accessible via SFTP, rsync, or SCP
- €3.50/month for 1TB, €11.50/month for 5TB
- Nightly encrypted backup job pushes Postgres dumps + MinIO snapshots here
- Backup retention: 30 days rolling

**Network security:**
- Hetzner Cloud Firewall per instance: only ports 443 (HTTPS) and 22 (SSH from your IP only) open
- Postgres, MinIO, Meilisearch, Keycloak admin — all behind Docker internal network, never publicly exposed
- Hetzner dedicated servers include basic DDoS mitigation. Cloud servers don't — Caddy rate limiting handles most abuse. For institutions facing state-level attacks (the ICC literally does), add a European CDN or Cloudflare in front.

---

## Deployment Automation

You cannot manually SSH into 15 customer instances every time you ship an update. This needs to be automated from the start.

**New customer provisioning (Terraform):**

```
# One command spins up a new customer's entire stack
terraform apply -var="customer=unhcr" -var="tier=professional" -var="region=fsn1"
```

This creates:
- Hetzner Cloud server (sized to tier)
- Cloud Firewall rules
- DNS record (unhcr.vaultkeeper.eu)
- Storage Box for backups
- Outputs the server IP for Ansible

**Server configuration (Ansible):**

```
# Configure the server and deploy the application
ansible-playbook deploy.yml -l unhcr
```

This playbook:
- Installs Docker
- Copies `docker-compose.yml` and environment config
- Pulls Docker images
- Starts all services (Go server, Caddy, Postgres, MinIO, Keycloak, Meilisearch)
- Configures Caddy TLS for the customer's domain
- Sets up nightly backup cron job to Storage Box
- Configures Postgres automated vacuuming
- Registers the instance with Uptime Kuma for monitoring

**Rolling updates (CI/CD):**

When you push a new release tag to GitHub:
1. GitHub Actions builds the new Docker image and pushes to container registry
2. A deployment script iterates through all managed customer instances
3. For each instance: SSH in, pull new image, `docker compose up -d` (zero-downtime rolling restart)
4. Verify `/health` endpoint returns 200 after restart
5. If health check fails, auto-rollback to previous image

This entire automation stack takes 2-3 days to build. It saves you hours every week once you have more than 3 customers.

**Folder structure:**

```
infrastructure/
├── terraform/
│   ├── main.tf              # Hetzner provider, server, firewall, DNS
│   ├── variables.tf         # Customer name, tier, region
│   ├── outputs.tf           # Server IP, Storage Box credentials
│   └── customers/
│       ├── unhcr.tfvars
│       ├── opcw.tfvars
│       └── hiil.tfvars
├── ansible/
│   ├── deploy.yml           # Full server setup + app deployment
│   ├── update.yml           # Pull new images + restart
│   ├── backup-verify.yml    # Verify backup integrity
│   ├── inventory.yml        # All managed customer servers
│   └── roles/
│       ├── docker/
│       ├── vaultkeeper/
│       ├── caddy/
│       ├── backup/
│       └── monitoring/
└── scripts/
    ├── rollout.sh           # Deploy new version to all instances
    └── health-check.sh      # Verify all instances are healthy
```

---

## Monitoring

**Uptime Kuma** — self-hosted, runs on a separate small Hetzner instance (CX22, ~€4/month).

Every VaultKeeper instance exposes two health endpoints:

**Public** (`/health`) — no auth, used by Uptime Kuma, returns only:
```json
{
  "status": "healthy",
  "version": "1.2.0"
}
```

**Detailed** (`/api/health`) — System Admin auth required, returns:
```json
{
  "status": "healthy",
  "version": "1.2.0",
  "postgres": "connected",
  "minio": "connected",
  "meilisearch": "connected",
  "last_backup": "2026-04-05T03:00:00Z",
  "backup_status": "completed",
  "evidence_count": 4521,
  "disk_usage_percent": 42
}
```

The public endpoint never exposes internal details like evidence count or disk usage — that's operational intelligence you don't want leaking.

Uptime Kuma pings the public `/health` endpoint every 60 seconds for each managed customer. Alerts go to your phone (Telegram, email, or push notification) if:
- Instance is unreachable (downtime)
- `/health` returns "unhealthy"

For detailed monitoring (backup failures, disk usage, service-level health), a separate scheduled job calls the authenticated `/api/health` endpoint with a System Admin API key and alerts if:
- Any service reports unhealthy (Postgres/MinIO/Meilisearch down)
- Last backup is older than 36 hours (backup job failed)
- Disk usage exceeds 85% (needs storage expansion)

**Total monitoring cost:** ~€4/month for the Uptime Kuma server. Monitors unlimited instances.

---

## Multi-Tenancy & User Management

### Multi-Tenancy Model: Single-Tenant, Multi-Department

Each customer (ICC, OPCW, an NGO) gets their own VaultKeeper instance. No data is shared between organizations. This is a hard requirement — no international court will accept evidence stored on a shared database.

However, WITHIN a single instance, you need to support multiple departments. The ICC has the Office of the Prosecutor (OTP), the Registry, the Defence teams, and the Chambers (judges). They all use the same VaultKeeper instance but see different things. This is handled through the two-level role system below.

### Two-Level Role System

**Level 1: System Roles (managed in Keycloak)**

These define what a user can do globally across the entire instance.

| System Role | What they can do |
|-------------|-----------------|
| System Admin | Manage users, configure system settings, view audit logs, manage backups, create/archive cases, assign case admins. Cannot see evidence unless also assigned a case role. |
| Case Admin | Create new cases, assign users to cases, set classifications and legal holds. Typically a senior prosecutor or head of investigations. |
| User | Can only access cases they've been explicitly assigned to. Cannot create cases or manage other users. Default role for most staff. |
| API Service | Non-human account for external system integrations (OTP Link, OSINT tools). Scoped to specific cases via API keys. |

System roles are managed entirely in Keycloak. Keycloak handles user creation, password policies, MFA enforcement, SSO federation (SAML/OIDC to the institution's existing Active Directory or LDAP), and session management. VaultKeeper never stores passwords — it validates JWT tokens from Keycloak and reads the system role from the token claims.

**Level 2: Case Roles (managed in VaultKeeper)**

These define what a user can see and do within a specific case. A user can have different roles in different cases.

| Case Role | Evidence Access | Can Upload | Can Tag/Classify | Can Disclose | Can Redact | Sees Witness Identity |
|-----------|----------------|------------|-----------------|-------------|------------|----------------------|
| Investigator | All items | Yes | Yes | No | Yes | Yes |
| Prosecutor | All items | Yes | Yes | Yes | Yes | Yes |
| Defence | Disclosed items only | No | No (own notes only) | No | No | No (pseudonyms only) |
| Judge | All items (read-only) | No | No | No | No | Case-by-case basis |
| Observer | All items (read-only) | No | No | No | No | No |
| Victim Representative | Disclosed items + victim-related | No | No | No | No | No |

**How it works in practice:**

1. System Admin creates a Keycloak account for a new prosecutor, assigns system role "User"
2. Case Admin assigns that prosecutor to Case ICC-UKR-2024 with case role "Prosecutor"
3. Prosecutor logs in, sees only ICC-UKR-2024 in their case list
4. Within that case, they can view all evidence, upload new items, tag, classify, create disclosures, and apply redactions
5. If the same person is also assigned to Case ICC-AFG-2023 as "Observer", they can view that case's evidence but cannot modify anything
6. Every action is logged in the custody chain with their user ID, role at time of action, timestamp, and IP

### User Onboarding Flow

1. System Admin creates user in Keycloak (or user is auto-provisioned via SSO/LDAP sync)
2. System Admin or Case Admin assigns user to one or more cases with specific case roles
3. User receives email notification with login link
4. On first login, user sees only the cases they're assigned to
5. Role assignment itself is logged in the custody chain: "User X granted Prosecutor role on Case Y by Admin Z at [timestamp]"

For small NGOs on the Starter tier, the org admin is probably also the system admin and creates a handful of users manually. For the ICC with 800+ staff, Keycloak federates with their existing identity provider — users are provisioned automatically, and case admins assign case roles as needed.

### API Key Management

For external system integrations (Phase 3), API keys are:
- Created by System Admins, scoped to specific cases
- Each key has a name, expiry date, and list of permitted cases
- Keys can be read-only (pull evidence metadata) or read-write (submit new evidence)
- All API actions are logged in the custody chain with the API key ID instead of a user ID
- Keys can be revoked instantly by any System Admin

### What Gets Logged in the Custody Chain

Every user management action is logged, not just evidence actions:
- User created / deactivated
- System role changed
- Case role granted / revoked
- API key created / revoked
- Login / logout / failed login attempt
- Password reset / MFA enrollment

This means the institution has a complete audit trail of not just "who touched the evidence" but "who had the ability to touch the evidence at any point in time."

---

## Data Model (9 core tables)

```sql
-- Cases: the top-level container
CREATE TABLE cases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code  TEXT UNIQUE NOT NULL,     -- e.g. "ICC-UKR-2024"
    title           TEXT NOT NULL,
    description     TEXT,
    jurisdiction    TEXT,
    status          TEXT DEFAULT 'active',    -- active, closed, archived
    legal_hold      BOOLEAN DEFAULT false,    -- when true, nothing in this case can be deleted
    retention_until TIMESTAMPTZ,             -- earliest date evidence can be destroyed
    created_at      TIMESTAMPTZ DEFAULT now(),
    created_by      TEXT NOT NULL,            -- keycloak user ID
    metadata        JSONB DEFAULT '{}'        -- flexible fields per institution
);

-- Case Roles: who can see what in each case
CREATE TABLE case_roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    user_id         TEXT NOT NULL,            -- keycloak user ID
    role            TEXT NOT NULL,            -- investigator, prosecutor, defence, judge, observer, victim_representative
    granted_at      TIMESTAMPTZ DEFAULT now(),
    granted_by      TEXT NOT NULL,
    UNIQUE(case_id, user_id)
);

-- Evidence Items: metadata + hash, files live in MinIO
CREATE TABLE evidence_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    evidence_number TEXT NOT NULL,             -- e.g. "ICC-UKR-2024-00001"
    version         INT DEFAULT 1,            -- increments on re-upload
    parent_id       UUID REFERENCES evidence_items(id), -- links to previous version
    title           TEXT NOT NULL,
    description     TEXT,
    file_key        TEXT NOT NULL,             -- MinIO object key
    file_name       TEXT NOT NULL,             -- original filename
    file_size       BIGINT NOT NULL,
    mime_type       TEXT NOT NULL,
    sha256_hash     TEXT NOT NULL,             -- computed at upload
    tsa_token       BYTEA,                    -- RFC 3161 timestamp token from independent TSA
    tsa_timestamp   TIMESTAMPTZ,              -- timestamp from the TSA response
    classification  TEXT DEFAULT 'restricted', -- public, restricted, confidential, ex_parte
    tags            TEXT[] DEFAULT '{}',
    source          TEXT,                      -- where/who it came from
    source_date     TIMESTAMPTZ,              -- when the evidence was originally created
    uploaded_at     TIMESTAMPTZ DEFAULT now(),
    uploaded_by     TEXT NOT NULL,
    is_current      BOOLEAN DEFAULT true,      -- false for superseded versions
    destroyed_at    TIMESTAMPTZ,              -- null unless formally destroyed
    destroyed_by    TEXT,
    destruction_authority TEXT,                -- court order reference or legal basis
    metadata        JSONB DEFAULT '{}'         -- EXIF data, GPS coords, custom fields
);

-- Custody Log: immutable append-only log of every action
CREATE TABLE custody_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id),
    case_id         UUID REFERENCES cases(id),
    user_id         TEXT NOT NULL,
    action          TEXT NOT NULL,             -- uploaded, viewed, downloaded, shared, exported, classified, tagged, disclosed, destroyed, version_created, legal_hold_set, legal_hold_released
    details         JSONB DEFAULT '{}',        -- action-specific context
    ip_address      INET,
    file_hash_at_action TEXT,                  -- hash at time of action, proves no tampering
    previous_log_hash TEXT,                    -- hash of previous log entry (tamper-evident chain)
    timestamp       TIMESTAMPTZ DEFAULT now()
);

-- Disclosures: tracking what was shared with defence/other parties
CREATE TABLE disclosures (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    disclosed_to    TEXT NOT NULL,             -- role or specific user
    disclosed_by    TEXT NOT NULL,
    disclosed_at    TIMESTAMPTZ DEFAULT now(),
    evidence_ids    UUID[] NOT NULL,           -- which items were disclosed
    notes           TEXT,
    redacted        BOOLEAN DEFAULT false      -- whether redacted versions were provided
);

-- Witnesses: identity separated from statements
CREATE TABLE witnesses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id),
    witness_code    TEXT NOT NULL,              -- pseudonym, e.g. "W-001"
    -- Identity fields (highly restricted access)
    full_name       TEXT,                       -- encrypted at application level
    contact_info    TEXT,                       -- encrypted at application level
    location        TEXT,                       -- encrypted at application level
    protection_status TEXT DEFAULT 'standard',  -- standard, protected, high_risk
    -- Non-identity fields (broader access)
    statement_summary TEXT,
    related_evidence UUID[] DEFAULT '{}',       -- links to evidence items
    created_at      TIMESTAMPTZ DEFAULT now(),
    created_by      TEXT NOT NULL
);

-- Notifications: event-driven alerts
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         TEXT NOT NULL,
    case_id         UUID REFERENCES cases(id),
    event_type      TEXT NOT NULL,              -- evidence_uploaded, disclosure_created, integrity_warning, legal_hold_changed, retention_expiring
    title           TEXT NOT NULL,
    body            TEXT,
    read            BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Backups: log of automated backup runs
CREATE TABLE backup_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    backup_type     TEXT NOT NULL,              -- full, incremental
    destination     TEXT NOT NULL,              -- local path, S3 bucket, remote server
    status          TEXT NOT NULL,              -- started, completed, failed
    file_count      INT,
    total_size      BIGINT,
    encrypted       BOOLEAN DEFAULT true,
    started_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    error_message   TEXT
);

-- API Keys: for external system integrations
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,              -- human-readable label, e.g. "OTP Link integration"
    key_hash        TEXT NOT NULL,              -- SHA-256 hash of the API key (never store plaintext)
    created_by      TEXT NOT NULL,              -- system admin who created it
    case_ids        UUID[] DEFAULT '{}',        -- which cases this key can access (empty = none)
    permissions     TEXT DEFAULT 'read',        -- read, read_write
    expires_at      TIMESTAMPTZ,               -- null = no expiry (not recommended)
    revoked_at      TIMESTAMPTZ,               -- null = active
    revoked_by      TEXT,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

**Nine tables.** The additions from the original five: witnesses (identity separation), notifications (event alerts), backup_log (audit trail for backups), and api_keys (external integrations). Plus versioning, retention, legal hold, destruction tracking, and RFC 3161 timestamp tokens integrated into the existing tables.

---

## API Specification (Core Endpoints)

Without this you'll design the API ad-hoc. These are the Phase 1 routes:

```
Authentication (all requests):
  Authorization: Bearer <JWT from Keycloak>
  All responses: JSON
  All mutations: logged to custody_log

Cases:
  POST   /api/cases                          → Create case (Case Admin+)
  GET    /api/cases                          → List cases (filtered by user's case_roles)
  GET    /api/cases/:id                      → Get case detail
  PATCH  /api/cases/:id                      → Update case metadata (Case Admin+)
  POST   /api/cases/:id/archive              → Archive case (Case Admin+)
  POST   /api/cases/:id/legal-hold           → Set/release legal hold (Case Admin+)
  GET    /api/cases/:id/export               → Export full case as ZIP

Evidence:
  POST   /api/cases/:id/evidence             → Upload evidence (multipart, chunked)
  GET    /api/cases/:id/evidence              → List evidence in case (filtered by role)
  GET    /api/evidence/:id                    → Get evidence metadata
  GET    /api/evidence/:id/download           → Download evidence file (logged)
  GET    /api/evidence/:id/thumbnail          → Get thumbnail/preview
  GET    /api/evidence/:id/versions           → Get version history
  PATCH  /api/evidence/:id                    → Update metadata/tags/classification
  POST   /api/evidence/:id/version            → Upload new version (links to parent)
  DELETE /api/evidence/:id                    → Destroy evidence (requires legal authority, blocked by legal hold)

Custody:
  GET    /api/evidence/:id/custody            → Get custody chain for item
  GET    /api/cases/:id/custody               → Get full case custody log
  GET    /api/evidence/:id/custody/export      → Export custody report as PDF
  GET    /api/evidence/:id/chain-certificate   → Generate Chain Continuity Certificate (Phase 2)

Integrity:
  POST   /api/cases/:id/verify                → Re-hash all evidence, compare against stored hashes
  GET    /api/cases/:id/verify/status          → Check verification status (async job)

Disclosures (Phase 2):
  POST   /api/cases/:id/disclosures            → Create disclosure package
  GET    /api/cases/:id/disclosures            → List disclosures

Witnesses (Phase 2):
  POST   /api/cases/:id/witnesses              → Create witness record
  GET    /api/cases/:id/witnesses              → List witnesses (identity filtered by role)
  GET    /api/witnesses/:id                    → Get witness detail (identity fields encrypted)

Roles:
  POST   /api/cases/:id/roles                  → Assign user to case with role (Case Admin+)
  DELETE /api/cases/:id/roles/:userId           → Remove user from case (Case Admin+)
  GET    /api/cases/:id/roles                  → List case role assignments

Search:
  GET    /api/search?q=&case_id=&type=&tag=    → Full-text search across evidence

System:
  GET    /health                               → Public health check (returns only "healthy"/"unhealthy" + version — no internal details)
  GET    /api/health                           → Authenticated detailed health (System Admin only — full service status, evidence count, disk usage, backup status)
  GET    /api/audit                            → System audit log (System Admin only)
  GET    /api/notifications                    → Current user's notifications
  PATCH  /api/notifications/:id/read           → Mark notification read
```

---

## File Upload Flow

Evidence files can be massive — the ICC processes multi-GB video files over unstable field connections. Normal HTTP upload breaks on files over ~100MB.

**Chunked/resumable upload (tus protocol):**
- Use the tus.io resumable upload protocol. Go has a mature tus server library (`github.com/tus/tusd`).
- Client uploads file in 5MB chunks. If connection drops, resume from last completed chunk.
- On completion, Go server: computes SHA-256 over the assembled file → sends hash to RFC 3161 TSA → moves file to MinIO with SSE → writes evidence_items row → writes custody_log entry.
- Max file size: configurable per instance (default 10GB, adjustable for institutional needs).
- Upload progress visible in the UI — investigators uploading 4GB of drone footage from eastern Congo over satellite need to see progress.

**File type validation:**
- Accept all file types. Evidence can be anything — .doc, .pdf, .mp4, .jpg, .wav, .xlsx, proprietary forensic formats.
- MIME type detection via Go's `http.DetectContentType` + file extension. Store both.
- No server-side file execution. Files are stored as opaque blobs in MinIO and served as downloads. Never render untrusted content server-side.

---

## Document & Media Preview

Legal teams need to preview evidence without downloading every file:

**Phase 1:**
- Images: server-generated thumbnails on upload (Go image processing or ImageMagick). Full-size view in browser.
- PDF: render in browser using pdf.js (client-side, no server processing of document content).
- Audio/Video: HTML5 `<video>` / `<audio>` player with native browser codecs. No transcoding.
- All other files: show metadata + download button. No preview.

**Phase 2:**
- Office documents (.docx, .xlsx): convert to PDF server-side using LibreOffice headless for preview. Original file preserved unchanged.
- Video frame extraction: generate thumbnail from first frame on upload.

**Security note:** Preview is always read-only. The original file is never modified. Preview generation runs in an isolated process. For PDFs, rendering happens client-side (pdf.js) — the server never interprets PDF content, preventing PDF-based exploits.

---

## Evidence Annotations & Comments

Different from tags. Legal teams need to attach observations to evidence without modifying the original file:

- "This video at 14:22 shows the defendant at the checkpoint"
- "Compare with witness statement W-003 paragraph 12"
- "Relevant to Count 3 — destruction of cultural property"

**Implementation:** A separate `annotations` table (case_id, evidence_id, user_id, content, timestamp_in_media for audio/video, page_number for documents). Annotations are per-user visible but shareable. Annotations themselves become part of the custody record.

**Not in the 9-table data model** — this is a Phase 2 addition. Keep it as a simple table when the time comes, don't over-engineer it.

---

## Security Hardening Details

Items not covered in the Security Architecture section that you'll need for implementation:

**Session management:**
- JWT access token lifetime: 15 minutes (short-lived, forces frequent re-auth against Keycloak)
- Refresh token lifetime: 8 hours (matches a work day)
- Refresh token rotation: enabled (each refresh issues a new refresh token, old one invalidated)
- Concurrent session limit: configurable per institution (default: 3 active sessions per user)
- On role revocation: all active sessions for that user are invalidated immediately (Keycloak admin API call)

**Input validation:**
- All user input (case titles, evidence descriptions, tags, annotations) sanitized via Go's `html.EscapeString` before storage
- Parameterized SQL queries everywhere — never string concatenation (Go's `database/sql` enforces this naturally)
- Evidence filenames sanitized on upload: strip path separators, null bytes, control characters. Store sanitized name, log original name in metadata.
- Tag values: alphanumeric + hyphens + underscores only, max 100 characters
- Case reference codes: validated against configurable regex pattern (e.g. `^[A-Z]+-[A-Z]+-\d{4}(-\d+)?$`)

**Rate limiting:**
- API rate limits enforced at Caddy level:
  - Authenticated endpoints: 100 requests/minute per user
  - Upload endpoint: 10 uploads/minute per user (prevents flood)
  - Search endpoint: 30 requests/minute per user
  - `/health` endpoint: no limit (monitoring needs frequent checks)
  - Failed login attempts: 5 per minute per IP, then 15-minute lockout (configured in Keycloak)
- API key rate limits: configurable per key (default: 60 requests/minute)

**Content Security Policy (CSP):**
- Caddy sets strict CSP headers on all responses:
  - `default-src 'self'`
  - `script-src 'self'` (no inline scripts)
  - `style-src 'self' 'unsafe-inline'` (Next.js requires inline styles)
  - `img-src 'self' blob: data:` (for thumbnails and previews)
  - `connect-src 'self'` (API calls only to own origin)
  - `frame-src 'none'` (no iframes)
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- Referrer-Policy: no-referrer

**Structured application logging (separate from custody chain):**
- Use Go's `slog` (structured logging, stdlib since Go 1.21)
- Log levels: ERROR, WARN, INFO, DEBUG
- All logs include: timestamp, request_id, user_id, endpoint, duration_ms
- Logs written to stdout (Docker captures to container logs)
- Sensitive data NEVER logged: file contents, witness identities, JWT tokens, passwords, API keys
- Log retention: 90 days on the server, configurable

---

## Database Indexes

The SQL schema has no indexes. Without these, queries degrade badly at scale:

```sql
-- Evidence lookups by case (the most common query)
CREATE INDEX idx_evidence_case_id ON evidence_items(case_id);
CREATE INDEX idx_evidence_case_current ON evidence_items(case_id, is_current) WHERE is_current = true;

-- Hash lookups for integrity verification
CREATE INDEX idx_evidence_hash ON evidence_items(sha256_hash);

-- Custody log queries (always filtered by evidence or case)
CREATE INDEX idx_custody_evidence_id ON custody_log(evidence_id);
CREATE INDEX idx_custody_case_id ON custody_log(case_id);
CREATE INDEX idx_custody_timestamp ON custody_log(timestamp);

-- Case role lookups (checked on every API call)
CREATE INDEX idx_case_roles_user ON case_roles(user_id);
CREATE INDEX idx_case_roles_case ON case_roles(case_id);

-- Notification feed
CREATE INDEX idx_notifications_user_unread ON notifications(user_id, read) WHERE read = false;

-- API key validation
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash) WHERE revoked_at IS NULL;

-- Witness lookups
CREATE INDEX idx_witnesses_case ON witnesses(case_id);

-- Disclosure lookups
CREATE INDEX idx_disclosures_case ON disclosures(case_id);
```

---

## Error Handling for Critical Paths

Define what happens when things go wrong:

| Failure | Behavior |
|---------|----------|
| RFC 3161 TSA unreachable | Evidence upload still succeeds. `tsa_token` set to NULL, `tsa_timestamp` set to NULL. Custody log entry notes "TSA unavailable — timestamp pending." Background retry job attempts TSA request every 5 minutes for 24 hours. Admin notification sent. |
| MinIO disk full | Upload rejected with HTTP 507. Admin notification sent immediately. Existing evidence remains accessible. |
| Hash mismatch during integrity verification | Evidence item flagged as `integrity_warning` in metadata. Admin notification sent with severity CRITICAL. Item remains accessible but UI shows warning badge. Custody log entry: "INTEGRITY ALERT — stored hash does not match computed hash." |
| Postgres connection lost | API returns HTTP 503. Health endpoint reports unhealthy. Uptime Kuma triggers alert. Caddy serves cached health status for 60 seconds before failing open. |
| Keycloak unreachable | All authenticated requests fail with HTTP 502. Existing sessions with valid (non-expired) JWTs continue to work until token expires (max 15 minutes). No new logins possible. |
| Backup job fails | Backup_log entry with status "failed" and error_message. Admin notification sent. Next scheduled backup retries. After 3 consecutive failures, CRITICAL alert. |
| File upload interrupted mid-chunk | tus protocol handles this. Client resumes from last completed chunk. No partial files stored in MinIO. Incomplete uploads auto-expire after 24 hours. |

---

## Backup Restore Procedure

Backups are well-documented. Restore isn't. You need this in the README and it needs to be tested.

```bash
# 1. Stop the application
docker compose down

# 2. Restore Postgres from backup
gunzip < /backup/vaultkeeper-db-2026-04-05.sql.gz | \
  docker compose exec -T postgres psql -U vaultkeeper

# 3. Restore MinIO data from backup
docker compose exec minio mc mirror \
  /backup/minio-snapshot-2026-04-05/ /data/

# 4. Verify integrity
docker compose up -d
curl https://yourinstance.vaultkeeper.eu/api/cases/1/verify

# 5. Check health
curl https://yourinstance.vaultkeeper.eu/health
```

**RTO (Recovery Time Objective):** Under 1 hour for full restore from backup.
**RPO (Recovery Point Objective):** 24 hours maximum data loss (daily backups). Institutions requiring lower RPO can configure backup frequency.

Restore procedure must be tested quarterly. Document results in the backup_log table.

---

## Go Project Structure

The infrastructure folder structure is documented. The application itself needs one too:

```
vaultkeeper/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, wires everything together
├── internal/
│   ├── auth/
│   │   ├── middleware.go         # JWT validation, role extraction from Keycloak tokens
│   │   └── permissions.go       # Case role permission checks
│   ├── cases/
│   │   ├── handler.go           # HTTP handlers for /api/cases/*
│   │   ├── service.go           # Business logic (create, archive, legal hold)
│   │   └── repository.go        # Postgres queries
│   ├── evidence/
│   │   ├── handler.go           # HTTP handlers for /api/evidence/*
│   │   ├── service.go           # Business logic (upload, hash, TSA, classify)
│   │   ├── repository.go        # Postgres queries
│   │   └── storage.go           # MinIO operations (put, get, delete)
│   ├── custody/
│   │   ├── logger.go            # Append-only custody log writer (middleware)
│   │   ├── chain.go             # Hash-chain computation and verification
│   │   └── repository.go        # Postgres queries
│   ├── integrity/
│   │   ├── verifier.go          # Re-hash all files, compare against stored hashes
│   │   └── tsa.go               # RFC 3161 timestamp client
│   ├── search/
│   │   └── meilisearch.go       # Meilisearch indexing and query
│   ├── notifications/
│   │   ├── service.go           # In-app + SMTP notification dispatch
│   │   └── repository.go
│   ├── backup/
│   │   └── runner.go            # Scheduled backup job (Postgres dump + MinIO snapshot)
│   ├── reports/
│   │   └── pdf.go               # Custody report and certificate PDF generation
│   └── config/
│       └── config.go            # Environment variable loading and validation
├── migrations/
│   ├── 001_initial_schema.up.sql
│   ├── 001_initial_schema.down.sql
│   ├── 002_indexes.up.sql
│   └── 002_indexes.down.sql
├── web/                          # Next.js frontend (separate build)
│   ├── src/
│   │   ├── app/                  # Next.js app router pages
│   │   ├── components/           # React components
│   │   ├── lib/                  # API client, auth helpers
│   │   └── messages/             # i18n translation files (en.json, fr.json)
│   ├── next.config.js
│   └── package.json
├── docker-compose.yml            # All services (Go, Caddy, Postgres, MinIO, Keycloak, Meilisearch)
├── Dockerfile                    # Multi-stage: build Go binary + Next.js static, serve from single image
├── Caddyfile                     # Reverse proxy config, TLS, CSP headers, rate limiting
├── .env.example                  # Template for required environment variables
├── Makefile                      # dev, build, test, migrate, lint targets
├── README.md
├── LICENSE                       # AGPL-3.0
└── CHANGELOG.md                  # Semantic versioning changelog
```

**Key decisions:**
- `internal/` package prevents external imports — Go convention for private application code.
- Each domain (cases, evidence, custody) has handler → service → repository layers. Handler parses HTTP, service has business logic, repository talks to Postgres. Clean separation, easy to test.
- Custody logger is middleware, not a service. It wraps every handler automatically.
- Frontend and backend live in the same repo but build separately. Docker multi-stage build produces one image serving both.

---

## Database Migrations

Schema changes need version control. Without this you'll be running raw SQL on production.

**Tool:** golang-migrate (`github.com/golang-migrate/migrate`). Standard in Go projects.

```bash
# Create a new migration
migrate create -ext sql -dir migrations -seq add_annotations_table

# Run migrations (up)
migrate -path migrations -database "$DATABASE_URL" up

# Rollback last migration
migrate -path migrations -database "$DATABASE_URL" down 1
```

**Rules:**
- Every schema change is a migration file, never a manual ALTER TABLE.
- Migrations run automatically on application startup (Go server checks and applies pending migrations before accepting requests).
- Down migrations are mandatory for every up migration — you need to be able to rollback.
- Migration files are numbered sequentially and checked into Git.
- Test migrations against a copy of production data before deploying.

---

## Environment Configuration

The Go server reads all configuration from environment variables. No config files, no flags. This is the Docker/12-factor way.

**Required variables (`.env.example`):**

```env
# Database
DATABASE_URL=postgres://vaultkeeper:password@postgres:5432/vaultkeeper?sslmode=require

# MinIO
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=vaultkeeper
MINIO_SECRET_KEY=changeme
MINIO_BUCKET=evidence
MINIO_USE_SSL=true

# Keycloak
KEYCLOAK_URL=https://keycloak:8443
KEYCLOAK_REALM=vaultkeeper
KEYCLOAK_CLIENT_ID=vaultkeeper-api

# RFC 3161 Timestamping
TSA_URL=https://freetsa.org/tsr
TSA_ENABLED=true

# Meilisearch
MEILISEARCH_URL=http://meilisearch:7700
MEILISEARCH_API_KEY=changeme

# SMTP (notifications)
SMTP_HOST=smtp.institution.org
SMTP_PORT=587
SMTP_USERNAME=vaultkeeper@institution.org
SMTP_PASSWORD=changeme
SMTP_FROM=VaultKeeper <noreply@institution.org>

# Application
APP_URL=https://yourinstance.vaultkeeper.eu
APP_ENV=production
APP_LOG_LEVEL=info
MAX_UPLOAD_SIZE=10737418240  # 10GB in bytes
BACKUP_DESTINATION=sftp://backup@storagebox.hetzner.com/vaultkeeper
BACKUP_ENCRYPTION_KEY=changeme

# CORS (development only — in production Caddy handles this)
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

**Validation:** The Go server validates all required variables on startup and refuses to start if any are missing or malformed. Fail fast, fail loud.

---

## Testing Strategy

For an evidence management system where bugs have legal consequences, testing isn't optional.

**Unit tests (run on every commit):**
- SHA-256 hash computation produces correct output for known inputs
- RFC 3161 timestamp token parsing and verification
- Permission matrix: every role × every action combination tested
- Custody log hash-chain computation and tamper detection
- Evidence number generation (sequential, no gaps, no duplicates)
- Input validation (filenames, tags, reference codes)
- Redaction: original file untouched, redacted copy has different hash

**Integration tests (run on every PR):**
- Full evidence lifecycle: upload → hash → timestamp → store → view → download → verify
- Role-based access: user with "defence" role cannot see undisclosed evidence
- Legal hold: evidence deletion blocked when legal hold is active
- Custody log: every action produces correct log entry with correct hash chain
- Backup: Postgres dump + MinIO snapshot + encrypted upload + successful restore
- Migration protocol: import from CSV manifest, verify all hashes match

**End-to-end tests (run before release):**
- Full user flow: Keycloak login → create case → upload evidence → assign roles → disclose → generate custody report PDF
- Multi-user scenario: prosecutor uploads, defence can only see disclosed items
- Integrity verification: tamper with a file in MinIO, verify the system detects it

**Testing tools:**
- Go stdlib `testing` package + `testify` for assertions
- Testcontainers-go for spinning up Postgres/MinIO/Keycloak in integration tests
- Playwright for E2E tests against the Next.js frontend

**CI pipeline:**
```
Push to main → Unit tests → Integration tests (testcontainers) → Build Docker image
Release tag  → All tests → Build + push image → Deploy to staging → E2E tests → Deploy to managed instances
```

---

## API Pagination

List endpoints will return thousands of items. Without pagination, responses become unusable.

**Cursor-based pagination** (not offset-based — offset breaks when items are added during pagination):

```
GET /api/cases/123/evidence?limit=50&cursor=eyJpZCI6Ijk4NyJ9

Response:
{
  "items": [...],
  "next_cursor": "eyJpZCI6IjkzNyJ9",   // base64-encoded last item ID
  "has_more": true,
  "total_count": 4521
}
```

**Applied to:**
- `GET /api/cases` (paginate case list)
- `GET /api/cases/:id/evidence` (paginate evidence grid — this is the big one)
- `GET /api/evidence/:id/custody` (paginate custody log — can have thousands of entries)
- `GET /api/cases/:id/custody` (paginate full case custody log)
- `GET /api/cases/:id/disclosures` (paginate disclosures)
- `GET /api/search` (paginate search results)
- `GET /api/notifications` (paginate notification feed)
- `GET /api/audit` (paginate system audit log)

Default limit: 50 items. Max limit: 200 items. Configurable per request.

---

## Release & Versioning

**Semantic versioning:** MAJOR.MINOR.PATCH (e.g. v1.2.3)
- MAJOR: breaking API changes or schema migrations that require manual intervention
- MINOR: new features, backward-compatible
- PATCH: bug fixes, security patches

**CHANGELOG.md** updated on every release. Institutional customers need to know what changed — their IT teams review changelogs before approving updates.

**Git tags** trigger the CI/CD pipeline. Tag `v1.2.3` → build → test → push image → deploy to managed instances.

**Docker image tags:** `vaultkeeper:latest`, `vaultkeeper:1.2.3`, `vaultkeeper:1.2`. Managed instances pin to minor version (auto-receive patches, manual upgrade for minors).

---

## Phases

### Phase 1: Foundation (Weeks 1-12)

**Goal:** A working, deployable product that an NGO can use tomorrow.

**Timeline reality check:** If you're working at Yuki full-time, this is evenings and weekends — budget 16-20 weeks instead of 12. The Yuki salary keeps the lights on while institutional sales cycles play out.

**Features:**

- **Case CRUD** — Create, list, view, edit, archive cases. Each case has a reference code, title, description, jurisdiction, status.
- **Evidence upload with auto-hashing + trusted timestamping** — Upload any file type. Server computes SHA-256 on receipt, sends hash to an RFC 3161 Timestamp Authority (FreeTSA.org), stores both hash and timestamp token in Postgres, file in MinIO with server-side encryption. Auto-extract EXIF metadata from images (GPS, timestamp, camera model). Auto-generate sequential evidence numbers.
- **Evidence grid view** — Browse all evidence in a case. Thumbnail previews for images. Filter by type, tag, date, classification. Sort by upload date, evidence number, source date.
- **Custody logging (middleware)** — Every API call that reads or writes evidence creates an immutable, hash-chained log entry. Upload, view, download, tag, classify, share — all logged with user, timestamp, IP, current file hash, and hash of the previous log entry. Append-only enforcement via Postgres RLS policies.
- **Role-based access per case** — Two-level permission system. System roles (System Admin, Case Admin, User) managed in Keycloak control who can create cases and manage users. Case roles (investigator, prosecutor, defence, judge, observer) assigned per case control who sees what evidence. Defence users only see disclosed items. Judges get read-only access. Full permission matrix enforced at API level. SSO federation with institution's existing Active Directory/LDAP via Keycloak — users don't create new passwords, they log in with their existing institutional credentials.
- **User management** — System Admins create and deactivate users in Keycloak (or users auto-provision via SSO). Case Admins assign case roles. Every user management action (role granted, role revoked, user deactivated) is logged in the custody chain. User onboarding via email invitation with login link.
- **Custody report export** — Select an evidence item or entire case → generate a PDF showing complete chain of custody with RFC 3161 timestamp verification. Every person who touched it, when, what they did. Formatted for court submission.
- **Basic search** — Full-text search across evidence titles, descriptions, tags. Filter by case.
- **Encryption** — TLS 1.3 on all connections (Caddy reverse proxy with auto Let's Encrypt). MinIO server-side encryption at rest enabled by default. Postgres connections over TLS.
- **Automated daily backups** — Cron job (or Go background goroutine) that dumps Postgres and snapshots MinIO data to an encrypted backup archive. Configurable destination (local directory, remote S3 bucket, or SFTP server). Backup log table tracks every run. Restore tested and documented in README.
- **Notifications** — In-app notification feed. Events: evidence uploaded, new user added to case, integrity warning. Email notifications via SMTP (configurable). No external notification services — just standard SMTP to the institution's own mail server.
- **Data export** — Full case export: all evidence files + metadata CSV + custody logs + case structure as a ZIP archive. Standard, open format. Any institution can leave VaultKeeper and take everything with them. This is a sovereignty feature, not a bug.
- **Docker Compose deployment** — Single `docker-compose.yml` spins up Go server, Caddy (TLS), Postgres, MinIO, Keycloak, Meilisearch. README with setup instructions a sysadmin can follow in 30 minutes.
- **Internationalization (i18n) from day one** — Use next-intl for all UI strings. Ship English only in Phase 1, but never hardcode strings. The ICC works in English and French. Europol uses all EU languages. The OPCW uses six UN languages. Retrofitting i18n later is painful — wiring it in from the start costs nothing.
- **Health endpoint** — `/health` JSON endpoint reporting service status (Postgres, MinIO, Meilisearch connectivity), last backup time, evidence count, disk usage, app version. Used by Uptime Kuma for monitoring managed instances.
- **Terraform + Ansible scaffolding** — Infrastructure-as-code for managed hosting from the start. Terraform provisions Hetzner instances, Ansible deploys the app. Even if you only have one managed customer, this saves you from manual ops debt later.
- **AGPL-3.0 on GitHub** — Open source from day one.

**Deliverable:** A Docker image anyone can deploy. List it on GitHub, write a clear README, post on Hacker News and the Nextcloud community forums.

---

### Phase 2: Institutional Features (Weeks 13-24)

**Goal:** Features that win paid support contracts with mid-tier institutions.

**Features:**

- **Witness/source management** — Separate identity from content using the witnesses table. A witness record has two layers: the identity (name, location, contact — encrypted at application level, highly restricted access) and the statement (narrative, dates, supporting evidence — broader access). Different roles see different layers. This is a Rome Statute requirement. Witness identities are encrypted with a separate key from general evidence.
- **Evidence versioning** — When a corrected or updated version of a document is uploaded, the original stays in the vault with its hash. The new version gets its own hash, its own custody chain, and a parent_id linking it to the original. Previous versions are marked `is_current = false`. Full version history visible in the UI. Nothing is ever silently overwritten.
- **Document redaction** — Upload a document or image, draw redaction boxes in the UI, generate a redacted copy as a new file. Original stays in the vault with full access controls. The redacted version gets its own hash, timestamp token, and custody chain. Linked to the original via versioning.
- **Disclosure workflow** — Prosecutor selects evidence items → optionally applies redactions → creates a disclosure package → system logs what was disclosed, to whom, when, with what redactions. Defence user now sees those items in their case view. Notifications sent automatically.
- **Confidentiality classifications** — Four levels: Public, Restricted, Confidential, Ex Parte. Each has configurable access rules per role. Ex Parte items are only visible to one side (prosecution OR defence, not both).
- **Legal hold and retention** — Cases can be placed on legal hold (nothing can be deleted or destroyed). Evidence items have optional retention periods ("preserve until 2049"). Destruction requires explicit authority (court order reference), is logged in the custody chain, and is blocked if a legal hold is active. GDPR erasure requests vs. legal preservation conflicts are surfaced as warnings for the admin to resolve — the system doesn't auto-decide.
- **Audited evidence destruction** — When destruction is authorized, the file is removed from MinIO, but the evidence_items row and full custody log are preserved permanently. The log records who destroyed it, when, and under what authority. The hash remains as proof the item once existed.
- **Evidence timeline** — Visual chronological view. Drag evidence items onto a date axis. Build the narrative of a case visually. Export timeline as PDF for court submissions.
- **Tagging and taxonomy** — Custom tag hierarchies per institution. Predefined sets for common use: evidence type (document, photo, video, audio, physical), source type (witness, open source, intercept, forensic), relevance (critical, supporting, background).
- **Bulk upload** — Upload a ZIP of 200 field photos. System unpacks, hashes each one individually, gets RFC 3161 timestamps, auto-extracts GPS/timestamp metadata, creates individual evidence items. Background processing via Go goroutines.
- **Data migration tool and protocol** — The hardest part of migration isn't technical — it's legal. When evidence moves from Relativity to VaultKeeper, a defence lawyer will probe the gap: "How do you prove nothing was altered during transfer?" The answer is cryptographic hash bridging:

  **Migration Protocol (5 steps):**
  1. **Source manifest export** — Before migration, export a signed manifest from the old system listing every evidence item with filename, metadata, and hash. This is the "closing document" of the old chain. The institution's IT officer and a legal representative sign off on it.
  2. **Verified ingestion** — VaultKeeper's migration tool imports each file, computes SHA-256 on ingestion, and compares against the source manifest hash. Each custody log entry records both hashes: `"action": "migrated", "source_system": "RelativityOne", "source_hash": "a1b2c3...", "computed_hash": "a1b2c3...", "match": true`. Any mismatch halts the migration and flags the file.
  3. **RFC 3161 timestamping** — The entire migration event is timestamped by an independent TSA, proving the exact moment the hash verification occurred.
  4. **Migration Attestation Certificate** — Auto-generated signed PDF stating: "On [date], [N] evidence items were transferred from [source system] to VaultKeeper. Every file's source hash was verified against the hash computed on ingestion. All [N] matched. Zero discrepancies. Full file list with dual hashes attached." This document is court-submittable.
  5. **Parallel operation period** — Both systems run simultaneously for a validation period (typically 30-90 days). Old system stays read-only. Any discrepancy can be caught before the old system is decommissioned.

  **Supported migration sources:** RelativityOne (via its export API), shared network drives, Google Drive, local folders, USB media. Accept a ZIP/folder of files with a CSV manifest (filename, date, source, case reference) for systems without an export API. This unblocks institutions switching from any source, not just Relativity.
- **Audit dashboard** — Admin view showing system activity: who logged in, what cases were accessed, any integrity warnings (hash mismatch = tampering detected), backup status, retention expiry warnings. Exportable audit report for institutional compliance.
- **Integrity verification** — One-click "verify all" that re-hashes every file in MinIO and compares against stored hashes. Also verifies RFC 3161 timestamp tokens. Flags any discrepancies. Run on schedule or on-demand. This is the core trust feature.
- **French language support** — Full UI translation to French. ICC's second official language. Opens the door to Francophone African tribunals and French-speaking NGOs. Add language switcher to the UI.
- **Automated rolling updates** — CI/CD pipeline that builds Docker images on release, pushes to registry, and deploys to all managed Hetzner instances with health check verification and automatic rollback on failure.
- **Chain Continuity Certificate** — A signed PDF that cryptographically proves the unbroken sequence of custody events for any evidence item from ingestion to present. Includes all hashes, RFC 3161 timestamps, and accessor identities. This document gets submitted to courts. Once a judge accepts it, VaultKeeper's format becomes part of the case record.
- **Case archival mode** — When a case closes, evidence moves to cold storage (cheaper MinIO tier or Hetzner Storage Box). Custody chain and verification remain active. Reduced archival support rate. Institutions keep paying for 25+ years to maintain closed case archives.
- **Template SOPs and workflow guides** — Provide customizable Standard Operating Procedure documents that institutions adapt for their internal processes. These reference VaultKeeper by name in investigation workflows. Once an institution's procedures cite your product, removing it means rewriting every procedure.

**Deliverable:** Version 2.0 that's ready to demo to institutional IT teams. This is what you walk into HSD Campus with.

---

### Phase 3: AI & Advanced Features (Weeks 25-36)

**Goal:** Features that justify six-figure contracts and differentiate from anything else on the market.

**Features:**

- **AI transcription** — Upload audio/video evidence. Background job runs Whisper (self-hosted, no data leaves the server) to generate text transcripts. Transcripts become searchable. Supports 90+ languages. Critical for field interviews and intercepted communications.
- **AI translation** — Machine translation of documents and transcripts using self-hosted models (Mistral via Ollama or similar). ICC works across Arabic, French, Lingala, Ukrainian, Darija — even rough translation saves hundreds of hours of human translator time. All processing on-premises.
- **Entity extraction** — AI scans evidence text and pulls out names, locations, dates, organizations. Displays as a knowledge graph per case. "Show me every piece of evidence that mentions Colonel X in Bunia between March and June 2024."
- **Advanced search** — Semantic search across evidence. Not just keyword matching but meaning-based retrieval. "Find evidence related to attacks on civilian infrastructure" returns relevant items even if they don't contain those exact words.
- **Multi-language OCR** — Scanned documents and handwritten notes get OCR'd and become searchable. Critical for field-collected paper evidence.
- **API for external systems** — REST API with scoped API key auth for integrating with external evidence submission portals (like the ICC's OTP Link), OSINT tools, or partner institutions' systems. API keys created by System Admins, scoped to specific cases, with read or read-write permissions, expiry dates, and instant revocation. All API actions logged in custody chain with the key ID.
- **Per-case encryption keys** — Optional encryption key isolation. Evidence in Case A is encrypted with Key A. Compromise of one case doesn't expose others. Key management via institution's existing KMS or Keycloak.
- **Federation (stretch goal)** — Two VaultKeeper instances at different institutions can share specific evidence items with cryptographic verification that nothing was altered in transit. An investigation team in one country shares evidence with a tribunal in another. Publish the federation protocol as an open specification — you define the standard, competitors must become "VaultKeeper-compatible."
- **Custody report format specification** — Publish the chain-of-custody report format as an open, documented spec. Present it at International Bar Association events. The goal: tribunals begin expecting this format. Once it's cited in procedural rules, any competing tool has to match your format to be taken seriously.

**Deliverable:** A product that can genuinely compete with Relativity's core evidence features, running entirely on sovereign infrastructure.

---

### Phase 4: Enterprise & Scale (Weeks 37+)

**Goal:** Handle ICC-scale deployments and expand beyond The Hague.

**Features:**

- **Multi-case analytics** — Cross-case pattern detection. "This witness appears in 3 different cases." "This location has 47 pieces of evidence across 5 investigations."
- **Advanced workflow engine** — Configurable approval workflows. Evidence goes through review stages: collected → processed → reviewed → approved → disclosed. Each stage requires sign-off from specific roles.
- **Mobile evidence capture** — Flutter mobile app (you know Flutter — use it where it shines) for field investigators. Capture photo/video/audio in the field, auto-tag with GPS and timestamp, encrypt locally, sync to VaultKeeper when connectivity is available. Offline-first.
- **Helm chart for Kubernetes** — For institutions that need high availability and scaling. ICC-scale deployment with multiple replicas, automated failover, horizontal scaling.
- **Compliance certifications** — Pursue ISO 27001, SOC 2 Type II. These are procurement requirements for larger institutions. Expensive and slow, but unlocks the biggest contracts.
- **On-premises appliance** — Pre-configured hardware box (NUC or similar) with VaultKeeper pre-installed. Ship it to a field office in eastern Congo. Plug in, power on, start collecting evidence. Syncs to headquarters when satellite connectivity is available.

---

## Go-To-Market Strategy

### Weeks 1-12: Build in public

- Open source on GitHub from day one
- Write 2-3 blog posts about the sovereignty problem (ICC sanctions, Relativity dependency, why evidence systems must be sovereign)
- Post the project on Hacker News, Reddit r/selfhosted, Nextcloud forums
- Share on LinkedIn targeting the international law / legal tech community
- Submit to Awesome Self-Hosted lists
- **Submit NLnet / NGI Zero Commons Fund application for VaultKeeper** — same process as GovLens application. This is textbook NGI territory: open source, sovereignty-preserving, European public interest infrastructure. Target €25-50K to cover development time while institutional sales cycles play out.
- **Prepare procurement-ready documents:** Data Processing Agreement (DPA) template, security architecture whitepaper, GDPR compliance statement, technical documentation for IT security review. These cost nothing to create but remove friction when an institution's procurement team evaluates you.

### Weeks 12-16: Local network activation

- Walk into HSD Campus (The Hague Security Delta) with a working demo
- Attend Humanity Hub events, introduce the product to NGO directors
- Reach out to T.M.C. Asser Instituut, HiiL — they're accessible, locally connected
- Contact the ICC's IKEMS section (the team that manages their evidence platform)
- Attend Nextcloud Summit in Munich (June 2026) — even though you're standalone, the sovereign tech community overlaps heavily

### Weeks 16-24: First contracts

- Offer free deployment + 3 months support to 1-2 small NGOs in exchange for case study / testimonial
- Use those case studies to approach mid-tier institutions
- Respond to any relevant procurement tenders (TenderNed, EU tenders)

### Weeks 24+: Institutional sales

- Leverage first institutional reference customer to approach others
- Present at International Bar Association events, legal tech conferences
- Partner with Nextcloud / openDesk ecosystem for co-marketing (you integrate with their stack, they reference you as a specialized legal module)

---

## How This Fits Your Current Situation

**Yuki:** If you land the senior Flutter role, keep it. Yuki salary covers your living expenses and removes all financial pressure from VaultKeeper. Build VaultKeeper evenings/weekends. Phase 1 takes 16-20 weeks instead of 12 at this pace, which is fine — institutional sales cycles are slow anyway.

**Estonian OÜ:** VaultKeeper sits perfectly under the OÜ you're setting up (Trelvio / Convael / Trovael). Same holding structure as GovLens, separate product line, tax-efficient profit retention. Institutional support contracts are invoiced from the OÜ. This keeps Dutch salary income (Yuki) separate from business income (VaultKeeper).

**GovLens:** VaultKeeper and GovLens serve adjacent but distinct markets. GovLens is democratic transparency for citizens. VaultKeeper is evidence integrity for legal institutions. The credibility from one reinforces the other — both are sovereignty-preserving, open source, European public interest infrastructure. If NLnet funds both, you have two funded open source projects under one OÜ.

**NLnet funding:** You already know the application process from GovLens (ref: 2026-04-393). Submit a separate VaultKeeper application to NGI Zero Commons Fund. The deliverable structure is similar: open source milestones with a self-hosted infrastructure commitment. Target €25-50K. This bridges the gap between "building the product" and "landing the first paying institutional contract."

---

## Pricing & Hosting

### Hosting Options

**Option A: Self-Hosted (Large Institutions)** — ICC, Europol, OPCW. They download the Docker image, deploy on their own servers. Their data never leaves their infrastructure. You provide support remotely. Your cost to serve: near zero.

**Option B: Managed Hosting (Small NGOs)** — You deploy a dedicated instance on Hetzner Cloud (Germany). Each customer gets their own isolated VM, database, and storage — not multi-tenant. You handle updates, backups, monitoring. They get a URL and login.

**Option C: Managed Deployment on Customer Infrastructure (Mid-Tier)** — You deploy on THEIR servers, they own the infra, you manage the app remotely.

---

### Pricing Tiers

#### Community (Free)

- Full application, all features, AGPL-3.0 open source
- Self-host it yourself, modify it, redistribute it
- Community support via GitHub issues
- No SLA, no guaranteed response time

#### Starter — Managed Hosting

**€250/month** (billed annually: €2,750/year)

- Dedicated VaultKeeper instance on Hetzner Cloud (Germany)
- Up to 25 users, 500 GB evidence storage
- Daily encrypted backups to Hetzner Storage Box (30 day retention)
- Email support, 2 business day response time
- Monthly updates applied automatically via CI/CD
- SSL (auto Let's Encrypt), custom domain
- Uptime monitoring, 99.5% uptime target
- Add-ons: +€5/month per 100 GB storage, +€8/month per user beyond 25

**Your margin:** Infra costs ~€30/month. Gross margin ~88%.

#### Professional — Managed or Self-Hosted

**€750/month** (billed annually: €8,500/year)

- Everything in Starter, plus:
- Up to 100 users, 2 TB evidence storage
- AI features: transcription (Whisper), translation, OCR (when available)
- Priority email + video call support, 1 business day response time
- Quarterly security report
- SSO integration (SAML/OIDC with your identity provider)
- 99.9% uptime target

#### Institution — Self-Hosted or Managed Deployment

**Custom, starting at €25,000/year**

- Everything in Professional, plus:
- Unlimited users and storage
- Dedicated support contact (a person, not a ticket queue)
- 4-hour response for critical issues, 24/7 for severity 1
- Custom module development (scoped and quoted separately)
- On-site deployment support if needed
- Annual security audit and penetration test
- Custom training sessions for staff
- Priority influence on product roadmap
- SLA with contractual uptime guarantees

Typical range: €25,000 - 80,000/year depending on scale.

#### Archival — Closed Case Storage

**€100/month per archived case group** (billed annually: €1,100/year)

- Evidence from closed cases moved to cold storage (Hetzner Storage Box)
- Custody chain and integrity verification remain fully active
- Chain Continuity Certificates still generated on request
- Annual integrity verification (re-hash all files, verify RFC 3161 timestamps)
- Evidence retrievable within 24 hours

This is the quietest, stickiest revenue stream. Institutions can't turn it off without violating 25+ year preservation obligations. Margin exceeds 90%.

---

### One-Time Services

| Service | Price |
|---------|-------|
| Initial deployment & configuration | €5,000 - 15,000 |
| Data migration (includes hash-bridging protocol, attestation certificate, parallel validation) | €8,000 - 25,000 |
| Custom module development | €10,000 - 40,000 |
| Staff training (remote, half-day) | €2,500 |
| Staff training (on-site, full day) | €5,000 + travel |
| Security audit & pen test | €8,000 - 15,000 |

---

### Revenue Projections (Conservative)

| Year | Customers | Revenue |
|------|-----------|---------|
| Year 1 | 1-2 small NGOs (free/low-cost pilots) | €10,000 - 30,000 |
| Year 2 | 3-5 paying institutions | €100,000 - 250,000 |
| Year 3 | 5-10 institutions + 1 large contract | €250,000 - 500,000 |
| Year 4+ | 10-15 institutions, international expansion | €500,000 - 1,000,000 |

### Pricing Philosophy

- **Never gate features.** Every feature is in the free tier. Paid tiers sell support, hosting, and peace of mind.
- **Flat annual pricing, unlimited uploads.** Never charge per evidence item or per GB. You never want an investigator hesitating about whether to upload a photo because it'll cost more.
- **The free tier is the sales funnel.** An NGO deploys the free version, realizes they need backups/SSO/someone to call when it breaks, upgrades to Starter or Professional.
- **Competitive positioning:** The ICC paid $500K/year for Relativity. You offer the same core value at 5-15% of that cost, fully sovereign, open source, and self-hostable.

---

## Why They Never Leave (The Moat)

The product's lock-in isn't contractual. It's evidentiary. Once an institution starts using VaultKeeper, leaving becomes legally dangerous.

### 1. The Custody Chain Is the Court Record

An ICC investigation spans 5-15 years. From the moment evidence enters VaultKeeper — hashed, RFC 3161 timestamped, every access logged — that custody chain becomes part of the legal record. If the prosecution switches to a different system mid-case, the defence will immediately challenge it: "How can you verify nothing was altered during migration? The chain of custody has a gap." No prosecutor wants to risk a case collapsing over a software migration. They stay.

**What to build:** In Phase 2, add a "Chain Continuity Certificate" — a signed PDF that cryptographically proves the unbroken sequence of custody events for any evidence item from ingestion to present. This document gets submitted to courts. Once a judge accepts it, VaultKeeper's format becomes part of the case law.

### 2. Cross-Institutional Network Effects

When the ICC uses VaultKeeper and shares evidence with the Kosovo Specialist Chambers via federation, both institutions need to be on VaultKeeper (or at least speak its protocol) to verify the custody chain end-to-end. Every new institution that joins the network makes it harder for existing members to leave. This is the SWIFT effect — the standard becomes the standard because everyone else uses the standard.

**What to build:** In Phase 3, the federation protocol. Make it an open spec so other tools can implement it, but VaultKeeper is the reference implementation. You're not locking them into your code — you're locking them into your protocol.

### 3. Institutional Muscle Memory

Staff get trained on VaultKeeper. Investigation workflows get built around it. SOPs reference specific VaultKeeper features ("upload to case vault, apply classification per Protocol 7, generate custody report for disclosure"). When an institution has 200 staff who know the system and 50 documented procedures that reference it, the switching cost isn't technical — it's organizational. Nobody wants to retrain an entire investigation team.

**What to build:** From Phase 2 onward, provide template SOPs and investigation workflow guides that institutions customize. These documents reference VaultKeeper by name. Once an institution's internal procedures cite your product, removing it means rewriting every procedure.

### 4. Integration Depth

Every system that feeds evidence into VaultKeeper is another hook. OTP Link submits evidence via VaultKeeper's API. OSINT tools push scraped social media posts into the vault. The mobile field app syncs photos from conflict zones. Satellite imagery providers send files via the API. Each integration is another dependency that makes switching painful.

**What to build:** A well-documented, stable REST API from Phase 1. Make it easy for other tools to integrate. Every integration someone builds is another anchor.

### 5. The Archive Problem

After a case closes, evidence must be preserved for decades. The ICC keeps records for a minimum of 25 years. If an institution leaves VaultKeeper, they need to export everything, migrate it to a new system, AND somehow maintain the cryptographic proof that the exported data matches the originals. That migration itself creates a custody chain gap. It's simpler to just keep paying the support contract on a system that's already working.

**What to build:** Make the long-term archive story explicit. In Phase 2, add a "case archival" mode — case is closed, evidence is moved to cold storage (cheaper MinIO tier or Hetzner Storage Box), but the custody chain and verification remain active. Charge a reduced archival support rate (€100/month instead of €250). The institution keeps paying you for 25 years to store closed cases. Multiply that by dozens of cases per institution.

### 6. Court-Accepted Format Standardization

This is the long game. If VaultKeeper custody reports become the standard format that international tribunals accept and expect, any competitor has to produce reports in the same format to be taken seriously. You define the format. You are the reference implementation. Competitors become "VaultKeeper-compatible."

**What to build:** Publish the custody report format as an open specification. Present it at International Bar Association conferences. Get it cited in tribunal procedural rules. This takes years but creates an unassailable position.

### The Honest Truth About This Moat

None of these moats work on day one. On day one, you're just another tool. The moat builds over time — every evidence item uploaded, every custody report submitted to a court, every staff member trained, every integration connected, every case that spans another year. By year 3, an institution that started with VaultKeeper would need a catastrophic reason to leave. By year 5, it's practically impossible.

This is why the free tier strategy matters. Get them in early, even for free. The longer they use it, the deeper the moat gets. The custody chain doesn't care whether they're on the free tier or the paid tier — it locks them in either way.

---

## Naming Candidates

The product name should convey: security, evidence, sovereignty, trust.

| Name | Domain Available? | Notes |
|------|------------------|-------|
| VaultKeeper | Check vaultkeeper.eu | Evokes security + guardianship |
| EvidenceVault | Check evidencevault.eu | Literal, clear, boring (good) |
| Custodian | Check custodian.legal | Chain of custody reference |
| Veritas | Check veritas.legal | Latin for truth, used in legal contexts |
| SovereignVault | Check sovereignvault.eu | Hits the sovereignty angle hard |
| ChainProof | Check chainproof.eu | Chain of custody + cryptographic proof |

Pick a name, grab the `.eu` domain, and move on. Don't overthink this.

---

## Key Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Institutional sales cycles take 6-18 months | High | Start with small NGOs. Keep Yuki job for income stability. Target below-€50K contracts that don't require formal tender. |
| A funded startup enters the same space | Medium | Your open source + EU presence + local network is a moat. Funded startups optimize for US markets first. |
| Institutions build in-house instead | Low | They don't have engineering teams. ICC is hiring one sysadmin, not building software. |
| Nextcloud/openDesk builds this themselves | Low | They're a platform company, not a legal tech company. They'd more likely partner with you. |
| Technical complexity of AI features | Medium | Phase 3 AI features use existing models (Whisper, Mistral). You're integrating, not training. Delay if needed — the product is valuable without AI. |
| You get overwhelmed as a solo founder | High | Phase 1-2 is manageable solo. If contracts ramp in Phase 3, use revenue to hire one Go dev. The OÜ structure you're setting up handles this. |

---

## Immediate Next Steps

1. **Pick a name** and grab the `.eu` domain
2. **Initialize the Go project** — `go mod init`, basic HTTP server, Postgres connection, file upload endpoint with SHA-256 hashing + RFC 3161 timestamping
3. **Set up the Docker Compose** — Go server + Caddy (TLS) + Postgres + MinIO (SSE enabled) + Keycloak + Meilisearch
4. **Build the evidence upload + custody log first** — this is the core value proposition
5. **Add automated encrypted backups** — daily Postgres dump + MinIO snapshot, encrypted, logged
6. **Add data export** — full case export as ZIP (files + metadata CSV + custody logs)
7. **Push to GitHub under AGPL-3.0** — public from day one
8. **Write procurement docs** — DPA template, security whitepaper, GDPR compliance statement
9. **Submit NLnet / NGI Zero Commons Fund application** — target €25-50K
10. **Write one blog post** — "Why the ICC can't use US software anymore, and what we're building instead"
11. **Walk into HSD Campus** and start talking