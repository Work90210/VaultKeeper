# Design Overhaul — Implementation Plan

## How To Use This Plan

The design prototype lives at `web/public/design/` (served at `localhost:3000/design/`) and also at `design-prototype/` in the project root. Both are identical and verified as the latest version. Every HTML file IS the spec. To implement a screen:

1. Read the design HTML file — that's the exact code of how it should look
2. Convert the HTML/JS to React/TSX in the corresponding component
3. CSS is already in `design-prototype/assets/style.css` and `design-prototype/assets/dash.css` — port to globals.css or Tailwind
4. Connect real data where the design uses hardcoded values
5. Where backend doesn't have the data yet — see "Backend Gaps" section below

**DO NOT describe the design in prose. READ THE CODE.**

---

## Design Files → React Components Map

### Shell (implements once, used everywhere)
| Design File | React Component | What To Do |
|-------------|----------------|------------|
| `design-prototype/assets/vk-shell-v2.js` | `web/src/components/layout/sidebar.tsx` | Convert renderDashShell() to React. Contains: sidebar, topbar, org dropdown, case picker, profile menu, notification panel, help panel |
| `design-prototype/assets/style.css` | `web/src/app/globals.css` | Port all CSS variables + component classes. Most already ported (95%), need `.bp-tracker`, `.bar-chart`, `.vk-modal` |
| `design-prototype/assets/dash.css` | `web/src/app/globals.css` | Port dashboard-specific classes. Most already ported |
| `design-prototype/assets/settings-components.js` | `web/src/components/settings/` | Reusable settings helpers: Panel, KVList, Toggle, RoleBadge, etc. |

### Dashboard Screens
| Design File | React Component | Connect Data From |
|-------------|----------------|-------------------|
| `design-prototype/dash.html` | `web/src/components/dashboard-views/overview-view.tsx` | **NEW:** `GET /api/dashboard/overview`, `GET /api/dashboard/berkeley-protocol`, `GET /api/dashboard/needs-you` |
| `design-prototype/dash-cases.html` | `web/src/components/cases/case-list.tsx` | `GET /api/cases` (needs enrichment: classification, exhibit count, witness count, team, BP progress) |
| `design-prototype/dash-evidence.html` | `web/src/components/dashboard-views/evidence-view.tsx` + `web/src/components/evidence/evidence-grid.tsx` | `GET /api/cases/{id}/evidence` (needs BP phase per item, type facet counts) |
| `design-prototype/dash-evidence-detail.html` | `web/src/components/evidence/evidence-detail.tsx` | Existing endpoints mostly work. Wire: legal hold, classification change, destruction steps 2-4 |
| `design-prototype/dash-inquiry.html` | `web/src/components/dashboard-views/inquiry-view.tsx` | `GET /api/cases/{id}/inquiry-logs` (needs: assigned_to, priority, sealed_status, many-to-many evidence links) |
| `design-prototype/dash-inquiry-detail.html` | `web/src/components/investigation/inquiry-log-detail.tsx` | `GET /api/inquiry-logs/{id}` (needs: search params, linked items with BP phase status, chain integrity) |
| `design-prototype/dash-assessments.html` | `web/src/components/dashboard-views/assessments-view.tsx` | `GET /api/cases/{id}/assessments` + **NEW:** `GET /api/cases/{id}/unassessed` + `GET /api/cases/{id}/assessment-stats` |
| `design-prototype/dash-witnesses.html` | `web/src/components/dashboard-views/witnesses-view.tsx` + `web/src/components/witnesses/witness-list.tsx` | `GET /api/cases/{id}/witnesses` (needs: risk_level, voice_masking, duress, corroboration score, signed_at) |
| `design-prototype/dash-audit.html` | `web/src/components/dashboard-views/audit-view.tsx` | **NEW:** `GET /api/cases/{id}/custody` (case-level, not evidence-level). Needs actor name JOIN, kind filtering |
| `design-prototype/dash-corroborations.html` | `web/src/components/dashboard-views/corroborations-view.tsx` | `GET /api/cases/{id}/corroborations` (needs: numeric score, status field, assigned_to, witness linking) |
| `design-prototype/dash-analysis.html` | `web/src/components/dashboard-views/analysis-view.tsx` | `GET /api/cases/{id}/analysis-notes` (needs: tags[], type mapping fix, "signed" terminal state, Rule 77 flag) |
| `design-prototype/dash-redaction.html` | `web/src/components/redaction/redaction-editor.tsx` + `collaborative-editor.tsx` | WebSocket collaboration (currently dead code — Yjs not wired). See "Collaboration Architecture" below |
| `design-prototype/dash-disclosures.html` | `web/src/components/dashboard-views/disclosures-view.tsx` + `web/src/components/disclosures/disclosure-list.tsx` | `GET /api/cases/{id}/disclosures` (needs: due_date, owners, wizard_step, countersigns) |
| `design-prototype/dash-reports.html` | `web/src/components/dashboard-views/reports-view.tsx` | **ENTIRELY NEW:** report_templates table, generated_reports table, generation endpoint. Existing InvestigationReport is wrong domain |
| `design-prototype/dash-search.html` | `web/src/components/search/search-results.tsx` | `GET /api/search` (needs: faceted response, latency metadata, query audit logging) |
| `design-prototype/dash-federation.html` | `web/src/components/dashboard-views/federation-view.tsx` | `GET /api/federation/peers` + **NEW:** `/stats`, `/protocol`, `/exchanges` |
| `design-prototype/dash-settings.html` | `web/src/app/[locale]/(app)/settings/page.tsx` | 7 tabs work, 4 tabs have NO backend (Retention, Keys, Storage, Danger zone) |
| `design-prototype/dash-investigation.html` | **NEW component needed** | 5-step investigation wizard |
| `design-prototype/dash-ceremonies.html` | **NEW component needed** | Key ceremonies page — **NEW:** `key_ceremonies` table |
| `design-prototype/dash-keyboard.html` | **NEW component needed** | Command palette + shortcuts overlay (client-side, no backend) |
| `design-prototype/dash-empty.html` | **NEW component needed** | Empty states for all screens (client-side) |
| `design-prototype/dash-print.html` | **NEW component needed** | Print-optimized custody dossier layout |
| `design-prototype/dash-profile.html` | `web/src/components/profile/profile-form.tsx` | `GET /api/profile` (exists) |
| `design-prototype/dash-night.html` | Already in globals.css | Dark mode already implemented, just verify |

---

## Backend Gaps (what data the design needs that doesn't exist)

### New Tables

```sql
-- BP phase tracking (FOUNDATIONAL — everything depends on this)
CREATE TABLE evidence_bp_phases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  evidence_id UUID NOT NULL REFERENCES evidence_items(id),
  phase INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 6),
  status TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started','in_progress','complete')),
  completed_at TIMESTAMPTZ,
  completed_by UUID REFERENCES users(id),
  UNIQUE(evidence_id, phase)
);
-- Auto-update via triggers or Go service when: inquiry linked → P1, assessment created → P2,
-- capture metadata filled → P3, custody seal → P4, verification created → P5, analysis signed → P6

-- Redaction operation log (for defence replay)
CREATE TABLE redaction_operations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  draft_id UUID NOT NULL REFERENCES redaction_drafts(id),
  evidence_id UUID NOT NULL,
  sequence_number BIGINT NOT NULL,
  operation_type TEXT NOT NULL CHECK (operation_type IN ('add_mark','modify_mark','delete_mark')),
  mark_id TEXT NOT NULL,
  mark_type TEXT NOT NULL CHECK (mark_type IN ('redact','pseudonymise','geo_fuzz','translate','annotate')),
  mark_data JSONB NOT NULL,
  previous_state JSONB,
  author_user_id UUID NOT NULL,
  author_username TEXT NOT NULL,
  op_hash TEXT NOT NULL,
  previous_op_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(draft_id, sequence_number)
);

-- Key ceremonies
CREATE TABLE key_ceremonies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  type TEXT NOT NULL, -- quarterly_rotation, emergency_revocation, initial_issuance, break_the_glass
  hardware_provider TEXT, -- YubiHSM2, Nitrokey HSM
  holders UUID[] NOT NULL,
  quorum_required INTEGER NOT NULL DEFAULT 2,
  quorum_achieved INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'initiated',
  initiated_by UUID NOT NULL,
  witnessed_by UUID[],
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Disclosure countersigns
CREATE TABLE disclosure_countersigns (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  disclosure_id UUID NOT NULL REFERENCES disclosures(id),
  user_id UUID NOT NULL,
  signed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  signature TEXT NOT NULL,
  UNIQUE(disclosure_id, user_id)
);

-- Report templates (predefined)
CREATE TABLE report_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL, -- Custody summary, Disclosure dossier, Ceremony minutes, etc.
  description TEXT,
  type TEXT NOT NULL CHECK (type IN ('standard','governance','legal','platform')),
  icon TEXT,
  generator_key TEXT NOT NULL UNIQUE -- used to route to correct generator function
);

-- Generated reports
CREATE TABLE generated_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES report_templates(id),
  case_id UUID,
  generated_by UUID NOT NULL,
  hash TEXT NOT NULL,
  sealed_at TIMESTAMPTZ,
  file_url TEXT,
  file_size BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Retention policy versions
CREATE TABLE retention_policy_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  version INTEGER NOT NULL,
  policy_json JSONB NOT NULL,
  changed_by UUID NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Inquiry log ↔ evidence many-to-many (replaces single FK)
CREATE TABLE inquiry_log_evidence (
  inquiry_log_id UUID NOT NULL REFERENCES investigation_inquiry_logs(id),
  evidence_id UUID NOT NULL REFERENCES evidence_items(id),
  PRIMARY KEY (inquiry_log_id, evidence_id)
);

-- Corroboration ↔ witness linking
CREATE TABLE corroboration_witnesses (
  claim_id UUID NOT NULL REFERENCES corroboration_claims(id),
  witness_id UUID NOT NULL REFERENCES witnesses(id),
  role_in_claim TEXT NOT NULL DEFAULT 'supporting',
  PRIMARY KEY (claim_id, witness_id)
);
```

### Altered Tables

```sql
-- Cases
ALTER TABLE cases ADD COLUMN classification TEXT CHECK (classification IN ('criminal','investigation','monitoring','archival'));
ALTER TABLE cases ADD COLUMN start_date DATE;

-- Evidence
ALTER TABLE evidence_items ADD COLUMN publication_date TIMESTAMPTZ;
ALTER TABLE evidence_items ADD COLUMN is_counter_evidence BOOLEAN DEFAULT false;
ALTER TABLE evidence_items ADD COLUMN rule_77_disclosed BOOLEAN DEFAULT false;
ALTER TABLE evidence_items ADD COLUMN media_metadata JSONB; -- duration, codec, resolution, fps (extracted via ffprobe)

-- Assessments: widen scores + add fields
ALTER TABLE evidence_assessments ALTER COLUMN relevance_score TYPE INTEGER;
ALTER TABLE evidence_assessments DROP CONSTRAINT IF EXISTS evidence_assessments_relevance_score_check;
ALTER TABLE evidence_assessments ADD CONSTRAINT evidence_assessments_relevance_score_check CHECK (relevance_score BETWEEN 1 AND 10);
ALTER TABLE evidence_assessments ALTER COLUMN reliability_score TYPE INTEGER;
ALTER TABLE evidence_assessments DROP CONSTRAINT IF EXISTS evidence_assessments_reliability_score_check;
ALTER TABLE evidence_assessments ADD CONSTRAINT evidence_assessments_reliability_score_check CHECK (reliability_score BETWEEN 1 AND 10);
ALTER TABLE evidence_assessments ADD COLUMN assigned_to UUID;
ALTER TABLE evidence_assessments ADD COLUMN status TEXT DEFAULT 'sealed' CHECK (status IN ('draft','sealed'));
-- Fix credibility labels: credible→Probable, uncertain→Unconfirmed, unreliable→Doubtful
UPDATE evidence_assessments SET source_credibility = 'probable' WHERE source_credibility = 'credible';
UPDATE evidence_assessments SET source_credibility = 'unconfirmed' WHERE source_credibility = 'uncertain';
UPDATE evidence_assessments SET source_credibility = 'doubtful' WHERE source_credibility = 'unreliable';

-- Inquiry logs
ALTER TABLE investigation_inquiry_logs ADD COLUMN assigned_to UUID;
ALTER TABLE investigation_inquiry_logs ADD COLUMN priority TEXT DEFAULT 'normal' CHECK (priority IN ('normal','urgent','low'));
ALTER TABLE investigation_inquiry_logs ADD COLUMN sealed_status TEXT DEFAULT 'active' CHECK (sealed_status IN ('active','locked','complete'));
ALTER TABLE investigation_inquiry_logs ADD COLUMN sealed_at TIMESTAMPTZ;

-- Witnesses
ALTER TABLE witnesses ADD COLUMN risk_level TEXT DEFAULT 'low' CHECK (risk_level IN ('low','medium','high','extreme'));
ALTER TABLE witnesses ADD COLUMN voice_masking_enabled BOOLEAN DEFAULT false;
ALTER TABLE witnesses ADD COLUMN duress_passphrase_enabled BOOLEAN DEFAULT false;
ALTER TABLE witnesses ADD COLUMN signed_at TIMESTAMPTZ;

-- Analysis notes
ALTER TABLE investigative_analysis_notes ADD COLUMN tags TEXT[];
ALTER TABLE investigative_analysis_notes ADD COLUMN is_counter_evidence BOOLEAN DEFAULT false;

-- Corroboration claims
ALTER TABLE corroboration_claims ADD COLUMN score NUMERIC(3,2);
ALTER TABLE corroboration_claims ADD COLUMN status TEXT DEFAULT 'investigating' CHECK (status IN ('investigating','corroborated','weak','refuted'));
ALTER TABLE corroboration_claims ADD COLUMN assigned_to UUID;

-- Disclosures
ALTER TABLE disclosures ADD COLUMN due_date DATE;
ALTER TABLE disclosures ADD COLUMN owner_user_ids UUID[];
ALTER TABLE disclosures ADD COLUMN wizard_step INTEGER DEFAULT 1;
ALTER TABLE disclosures ADD COLUMN countersign_required INTEGER DEFAULT 0;
ALTER TABLE disclosures ADD COLUMN countersigns_received INTEGER DEFAULT 0;
ALTER TABLE disclosures ADD COLUMN manifest_hash TEXT;
ALTER TABLE disclosures ADD COLUMN bundle_url TEXT;

-- Federation peers
ALTER TABLE federation_peers ADD COLUMN role TEXT CHECK (role IN ('sub-chain','full-peer','read-only','cold-mirror','staging'));

-- Federation exchanges (for p95 latency tracking)
ALTER TABLE federation_exchanges ADD COLUMN exchange_duration_ms INTEGER;
```

### New API Endpoints

```
# Dashboard aggregation
GET  /api/dashboard/overview              → KPIs + delta vs yesterday
GET  /api/dashboard/berkeley-protocol     → per-phase % (all cases or ?case_id=X)
GET  /api/dashboard/needs-you             → 7 action types UNION query (?case_id=X optional)

# Case enrichment
GET  /api/cases/stats                     → total/active/hold/sealed counts, lead-on, new-this-month
GET  /api/cases/{id}/team                 → members with roles + user details
GET  /api/cases/{id}/berkeley-protocol    → per-phase stats for single case
GET  /api/cases/{id}/stats                → exhibit count, witness count, disk usage, last activity
GET  /api/cases/{id}/integrity-status     → chain %, breaks, snapshot cadence, last verified
GET  /api/cases/{id}/custody              → case-level custody log (not evidence-level) with actor name JOIN
GET  /api/cases/{id}/unassessed           → evidence items with no assessments
GET  /api/cases/{id}/assessment-stats     → assessed count, avg relevance, recommendation breakdown
GET  /api/cases/{id}/analysis-stats       → uncorroborated count, counter-evidence, avg citations
GET  /api/cases/{id}/witness-stats        → total, duress count, voice-masked count, break-glass 90d
GET  /api/cases/{id}/corroboration-stats  → claims breakdown, median score, single-source count
GET  /api/cases/{id}/disclosure-stats     → this-quarter, pending-review, avg bundle, rejected
GET  /api/cases/{id}/evidence-facets      → count by type (doc/img/audio/forensic/legal-hold)
GET  /api/cases/{id}/audit-stats          → total events, actor share, last countersign

# Assessment CRUD fix
PUT  /api/assessments/{id}                → wire existing repo method to handler

# Inquiry workflow
POST /api/inquiry-logs/{id}/lock          → lock inquiry
POST /api/inquiry-logs/{id}/seal          → seal permanently

# Corroboration fix
POST /api/corroborations/{id}/evidence    → implement (currently returns 501)
DELETE /api/corroborations/{id}/evidence/{eid} → implement (currently returns 501)

# Disclosure workflow
POST /api/disclosures/{id}/countersign    → submit countersign
POST /api/disclosures/{id}/generate-bundle → generate encrypted ZIP

# Redaction workflow
POST /api/evidence/{id}/redact/drafts/{did}/seal → freeze draft (different from finalize)
GET  /api/evidence/{id}/redact/drafts/{did}/operations → defence replay

# Federation
GET  /api/federation/stats                → peers breakdown, federated cases, p95 latency, divergences
GET  /api/federation/protocol             → transport, identity, op format, seal, conflict, revocation
POST /api/federation/peers/invite         → invite peer

# Reports (ENTIRELY NEW)
GET  /api/reports/templates               → list 6 predefined templates with usage counts
POST /api/reports/generate                → generate report from template
GET  /api/reports                         → list generated reports
GET  /api/reports/{id}/verify             → verify report hash integrity

# Settings (4 missing tabs)
GET  /api/organization/{id}/retention-policy     → retention policy config
GET  /api/organization/{id}/retention-policy-history → versions for diff view
GET  /api/organization/{id}/storage-stats        → MinIO usage
GET  /api/organization/{id}/key-ceremonies       → ceremony log
POST /api/danger/rotate-key                       → 3-of-3 quorum
POST /api/danger/decommission                     → sealed archive handoff
POST /api/danger/revoke                           → emergency broadcast

# Misc
POST /api/cases/{id}/import              → import archive ZIP
POST /api/cases/{id}/audit/export        → export audit range as PDF
```

### Broken Handlers To Fix

```
1. legal_hold — only blocks destruction, not other mutations → add check to UpdateMetadata, UploadNewVersion, ApplyRedactions
2. analysis_notes status transitions — no validation → add state machine (draft → in_review → approved/signed)
3. corroboration AddEvidence/RemoveEvidence — return 501 → implement
4. permission JSONB — stored but not enforced → add middleware checking specific permission flags
5. federation receive tokens — in-memory map → move to Redis or PostgreSQL with TTL
6. witness pseudonym — user-provided → auto-generate W-XXXX on creation if not provided
7. sidebar counts — N+1 calls per case → single aggregation query
8. investigation-api.ts — client-side auth pattern → refactor to authenticatedFetch server-side
9. case context provider — never populated → wire case picker → setCaseData()
10. overview KPIs — all hardcoded mock → connect to /api/dashboard/overview
```

### Collaboration Architecture (Redaction Editor)

**Current reality:** Yjs is dead code. Frontend uses plain React state + REST auto-save. No real-time collaboration works.

**Architecture (3-tier):**

**Tier 1 — Real-time sync:** Either wire Yjs properly OR keep REST with WebSocket broadcast. Decision point.

**Tier 2 — Operation log (`redaction_operations` table):** Each mark operation (add/modify/delete) captured client-side from existing React callbacks, batched with auto-save (1.5s), server assigns sequence numbers + hash chain per-draft. NOT per-keystroke — per-mark.

**Tier 3 — Custody chain (milestones only):** Only 4 events: draft_created, draft_sealed, finalized, discarded. NO individual ops in custody chain (advisory lock bottleneck).

**Seal = freeze:** Hash entire operation log, write to custody chain, no more ops accepted.

**Defence replay:** `GET /api/evidence/{id}/redact/drafts/{did}/operations` returns all ops in sequence.

### Architectural Decisions

1. **AD-1:** Scope `overflow: hidden` to dashboard layout only — don't break marketing pages
2. **AD-2:** Tailwind utilities referencing CSS vars inside components (not new global CSS classes)
3. **AD-3:** React Server Components first, `"use client"` only on interactive leaves
4. **AD-4:** Merge BP components: `<BPIndicator variant="full|dots|inline">`
5. **AD-5:** BP phase = event-driven denormalized table, NOT API-time 6-table JOIN
6. **AD-6:** Dashboard KPIs = direct indexed queries on summary tables, no Redis at 14k scale
7. **AD-7:** Split migrations by bounded context (5 migrations, not 1 monolith)
8. **AD-8:** "Needs you" = single UNION ALL query, 7 action types
9. **AD-9:** Federation peer role = TEXT with CHECK constraint
10. **AD-10:** Key ceremonies = separate table, not custody log
11. **AD-11:** Evidence grid: `@tanstack/react-table`, `next/image`, SVG sprite for BP dots
12. **AD-12:** Real-time sidebar = SSE (Server-Sent Events), not polling
13. **AD-13:** Optimistic UI via React Query mutations (assessments, inquiry log)
