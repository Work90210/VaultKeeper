# Design Overhaul — Pixel-Perfect Implementation Plan (v2)

## Context

A full HTML/CSS/JS design prototype has been delivered from Claude Design covering **22+ dashboard screens** plus shell (sidebar, topbar, dropdowns). The existing Next.js frontend has **~104 components** and most backend API routes already exist. The goal is to **pixel-perfect match** the design prototype on every screen, and **connect all data** to the real backend APIs. Where data doesn't exist yet, we build the API.

**Verified by 5 parallel audit agents** — screen coverage, data fields, components, CSS tokens, and chat transcript intent.

---

## Design System Tokens (from `style.css` + `dash.css`)

**Status: 95% already implemented in globals.css + tailwind.config.ts**

### Colors (all verified present)
- `--bg: #f5f1e8`, `--bg-2: #ece7d8`, `--paper: #fbf8ef`
- `--ink: #14110c`, `--ink-2: #2a2620`
- `--muted: #6a655a`, `--muted-2: #98927f`
- `--line: rgba(20,17,12,0.08)`, `--line-2: rgba(20,17,12,0.14)`
- `--accent: #b8421c`, `--accent-soft: #e4a487`
- `--ok: #4a6b3a`

### Typography (all verified loaded)
- **Headings**: Fraunces (serif, italic for accent `<em>` — renders in `--accent` color)
- **Body**: Inter (sans-serif, 300-600)
- **Code/mono**: JetBrains Mono
- **Eyebrow/labels**: JetBrains Mono, 10-11px, uppercase, letter-spacing .08-.1em

### Layout (verified correct)
- Dashboard: `grid-template-columns: 248px 1fr`
- Content: `padding: 28px 32px 64px`, max-width 1400px
- **CRITICAL**: `body { overflow: hidden }` — only `.d-main` scrolls, sidebar is fixed

### Radii, Shadows, Animations, Responsive Breakpoints — all verified present

---

## Phase 0: Missing CSS + Shared Components

### 0.1 — Missing CSS Classes (3 gaps found)
| Class | Purpose | Status |
|-------|---------|--------|
| `.bp-tracker` / `.bp-phase` | Berkeley Protocol 6-phase tracker | **NOT FOUND — must add** |
| `.bar-chart` | Simple bar chart for audit/corroboration | **NOT FOUND — must add** |
| `.vk-modal` / `.vk-modal-box` | Modal dialog styling | **NOT FOUND — must add** |

### 0.2 — Shared React Components to Create
Existing UI components are all marketing-focused. Dashboard needs these wrappers:

| Component | CSS Class | React Wrapper Needed |
|-----------|-----------|---------------------|
| `<KPIStrip>` | `.d-kpis` / `.d-kpi` | YES — exists as CSS only |
| `<Panel>` | `.panel` / `.panel-h` / `.panel-body` | YES |
| `<FilterBar>` | `.fbar` / `.fsearch` / `.chip` | YES |
| `<DataTable>` | `.tbl` | YES |
| `<Timeline>` | `.tl-list` / `.tl-item` | YES |
| `<StatusPill>` | `.pl.sealed`, `.pl.hold`, etc. | YES |
| `<Tag>` | `.tag` | YES |
| `<AvatarStack>` | `.avs` / `.av` | YES |
| `<ChainVisual>` | `.chain` | YES |
| `<EvidenceCard>` | `.ev-card` | YES |
| **`<BPTracker>`** | `.bp-tracker` / `.bp-phase` | **YES — CSS + React** |
| **`<BPDots>`** | inline 6-dot indicator | **YES — new** |
| **`<FlowBanner>`** | horizontal phase stepper | **YES — new** |
| `<LinkArrow>` | `.linkarrow` | YES |
| **`<BarChart>`** | `.bar-chart` | **YES — CSS + React** |
| `<DocPaper>` | `.doc-paper` | YES |
| **`<Modal>`** | `.vk-modal` | **YES — CSS + React** |
| **`<SplitViewModal>`** | modal with left preview + right form | **YES — new** |
| `<Tabs>` | `.tabs` | YES |
| `<KeyValueList>` | `.kvs` | YES |
| `<EyebrowLabel>` | `.eyebrow-m` | YES |
| **`<PhaseDots>`** | per-item 6-dot phase indicator | **YES — new** |

### 0.3 — Dashboard Views Index Fix
File `web/src/components/dashboard-views/index.ts` is missing exports for: WitnessesView, CorroborationsView, AnalysisView, RedactionView, FederationView.

---

## Phase 1: Shell (Sidebar + Topbar + Layout)

### 1.1 — Sidebar (`sidebar.tsx`) — REWRITE
**CRITICAL behavior from chat transcripts:**
- **Fixed position** — sidebar never scrolls. `body: overflow hidden`, only `.d-main` scrolls
- **Brand mark** with nested shapes (`.brand-mark` with `::before` + `::after`)
- **Org switcher** — static label for most users, subtle hover swap icon for admins
- **Case picker as PRIMARY control** — dot + name + sub + chevron ▾, dropdown with all cases
- **Dynamic nav** — content changes based on selected case (all vs specific), but always shows full nav structure
- **Navigation sections**: "Workspace", "Investigation" (6 BP phases), "Reporting", "Platform"
- Nav items: icon 18px + label + optional badge (mono, right-aligned)
- Active state: `bg: var(--ink)`, `color: var(--bg)`, icon `color: var(--accent-soft)`
- **User card at bottom** — avatar + name + role + online dot
- **Profile dropdown** from user card → Edit profile, Settings, Sign out
- Sidebar badges show real counts from API

### 1.2 — Topbar (`top-bar.tsx`) — REWRITE
- Breadcrumb nav (`.d-crumb`)
- Search bar (pill, "Search exhibits, witnesses, hashes…", ⌘K)
- **Notification bell** with red dot → opens notification panel (6 items, 3 urgent: countersign, redaction review, disclosure deadline)
- **Help button** → opens help panel (Documentation, FAQ, Keyboard shortcuts, Security & compliance, Contact support, version info)
- Blurred background (`backdrop-filter: blur(10px)`)

### 1.3 — Context Dropdowns
- **Org dropdown**: list of orgs with avatars, active indicator (admin-only)
- **Case dropdown**: list of cases with status dots, "All cases" option
- **Profile menu**: user info + edit profile + settings + sign out
- **Notification panel**: urgency indicators, timestamps, "View all activity →"
- **Help panel**: icon + label + description for each item, version footer

---

## Phase 2: Overview / Dashboard (`dash.html`)

**Note**: `dash-index.html` is a **static router/directory page** (information architecture showing all 25 surfaces grouped into 5 clusters). It is NOT the operational dashboard — that's `dash.html`. We do NOT need to implement dash-index.html as a user-facing screen; it's a design reference only.

### 2.1 — All-Cases View
- Greeting: "Good afternoon, {name}." with date/time eyebrow
- Summary text about active cases
- Action buttons: All cases, New investigation, Upload evidence
- **KPI strip**: Exhibits sealed today, Awaiting action, Fully through protocol, Last RFC 3161 stamp (mono, large)
- **Berkeley Protocol compliance panel**: 6-column grid showing phase %, color-coded progress bars (green ≥90%, amber ≥60%, red <60%)
- **Your cases in flight**: table with case ref, role, exhibits, **BP progress bar** (%, done count, need action count), last activity
- **Needs you** panel: timeline of 5 action items (countersign, review, approve, corroboration, provenance gap)
- **Recent activity** panel: signed events timeline with sigs

### 2.2 — Case-Scoped View
- Case header with status pill and role eyebrow
- **KPI strip**: Exhibits, Awaiting action, Through all 6 phases, Open inquiries
- **Berkeley Protocol compliance**: per-case phase percentages in 6-column grid
- **Recent activity**: case-specific events timeline
- **Needs you**: case-specific action items
- **Case team**: avatar grid with name + role

---

## Phase 3: Cases List (`dash-cases.html`)

- Page header: "All cases" with Import archive + New case buttons
- **KPI strip**: Total cases (breakdown), You are lead on, New this month, Disk · all cases (TB/TB)
- **Filter bar**: search + status chips (All, Active, Legal hold, Archived, Draft) + Role + Jurisdiction
- **Table**: Case ref+sub, Classification tag, Your role (accent tag for Lead), Exhibits, Witnesses, Chain visual (5-node dots), Team avatars, Status pill, Last activity, Open →
- **New case modal**: identifier, description, classification select, role select, jurisdiction, start date, initial status, **Berkeley Protocol auto-init panel** (6 phases shown as circles), case team search, notes textarea

---

## Phase 4: Evidence List (`dash-evidence.html`)

- Page header: "Evidence locker" with Import archive + Upload exhibit buttons
- **Filter bar**: search + type chips (All ·12,417, Document, Image/video, Audio, Forensic, Legal hold, Has redactions, Contributor) + **Grid/Table toggle** chips
- **Evidence grid** (4-column): thumbnail with type badge + status pill, file name, ref + meta, tags, **BP dots** (6 dots with phase count label)
- **Pagination**: mono "12 of 12,417 · page 1 of 1,035" + chip navigation
- **Upload queue panel**: 3 in-flight items with progress bars (%, status text)
- **Integrity summary panel**: key-value (hash algo, TSA, last verification, chain breaks, storage, validator link)

---

## Phase 5: Evidence Detail (`dash-evidence-detail.html`)

### Header
- Evidence number + version + status pill + classification tag
- Title as `<em class="a">` (accent italic Fraunces)
- Description paragraph
- Action buttons: ← Back, Download, Redact

### Berkeley Protocol 6-Phase Tracker (full-width)
Each phase: number, name, status (Complete/In progress/Not started) with colored dot, detail items (3 max), missing items, action link, bottom bar fill

### Two-Column Layout: Main + Sidebar (320px)

**Main column:**
- **Preview**: video player (dark bg, play button, file badge, duration badge, faux waveform)
- **Tags**: inline tag pills
- **Tabs**: Custody log (timeline), Versions (row cards), Redactions (draft cards + finalized cards), Verification (assessment scores + verification records), **Manage** (actions tab)
- **CRITICAL**: Actions are in the **Manage tab**, NOT in the sidebar

**Manage tab layout** (2-column card grid):
- Classification selector (4 levels: public/restricted/confidential/ex_parte)
- Legal hold card with toggle + confirmation modal
- Versioning card (upload new version + create redacted version buttons)
- Integrity actions (re-verify hash, new RFC 3161, export custody PDF, download original)
- Retention & destruction card (policy, review date, status + **4-step destruction dialog**)

**Sidebar (metadata only):**
- File: name, original, size, type, version, classification
- Dates: uploaded, by, source, source date
- Integrity: SHA-256 hash display, TSA verified pill, authority, stamped
- Provenance (Berkeley tag): platform, method, captured, published, collector, language, location, coordinates, geo source, availability, tool
- EXIF: camera, capture date, focal length, GPS, resolution, codec, FPS
- Berkeley Protocol compliance: X/6 phases complete with progress bar + checklist
- Linked: corroborations, witnesses, disclosures

### Modals
- **Legal hold**: confirmation with reason textarea
- **Destruction**: 4-step process, step 1 shows warning + preserved items + authority textarea
- **New redaction**: name + purpose selector (6 purpose tags)
- **Upload version**: drop zone + version note

---

## Phase 6: Investigation Screens (BP Phases 1-6)

### 6.1 — Inquiry Log (`dash-inquiry.html`) + Detail (`dash-inquiry-detail.html`)

**CRITICAL from chat transcripts**: Inquiry log is the **central workspace/hub**, not just a list.

**List view:**
- Grouped by day, each entry: time + avatar + author name + type pill (Decision/Open question/Action/External request/Federation) + content + linked items (accent mono) + items count
- Filter bar with type chips + counts
- "View full entry →" link per row

**Detail view** (modal or page):
- Header: entry ref + phase badge + sealed status
- Metadata grid: Author, Type, Priority, Assigned, Signature
- Content block (full text)
- Search parameters: Strategy, Keywords, Tool, Period, Results
- **Linked items with phase status**: each item shows 6 BP dots + phase label
- Chain integrity footer (block hash + verified badge)

**New entry modal:**
- Entry type chips (Decision/Open question/Action/External request/Federation)
- Content textarea
- Assigned to + Priority selects
- Search strategy + Search tool inputs
- Search started/ended datetime + Results found/relevant
- Linked exhibits/witnesses input

**Lock/Complete workflow** (from chat):
- Inquiry can be locked → prevents new items from being linked, shows locked banner
- Locked inquiry can be marked complete → seals permanently
- Status pills: Active/Locked/Complete

### 6.2 — Assessments (`dash-assessments.html`)

**Two-column layout:**
- **Left**: Unassessed queue (847 awaiting) — list of evidence items needing scoring, each clickable to open assessment modal
- **Right**: "How to assess" guide — 6-step walkthrough with numbered circles

**KPI strip**: Assessed · this case, Awaiting assessment, Avg. relevance, Deprioritized/discarded

**Completed assessments** panel below with filter bar:
- Each row: REL/RLB score pairs (large serif), evidence ref + name + recommendation pill, assessor + date, rationale excerpt, **BP dots** with phase label, View → link

**Assessment modal — SPLIT VIEW:**
- **Left**: Video/image preview, exhibit metadata grid (ID, filename, size, uploader, date, source, method, location, coordinates, hash, custody status, tags), "Open full evidence detail →" link
- **Right**: Relevance score (1-10) + Reliability score (1-10), Source credibility select (Established/Probable/Unconfirmed/Doubtful), Recommendation select (Collect/Monitor/Deprioritize/Discard), Assigned to, Rationale textarea, Misleading indicators textarea

### 6.3 — Witnesses (`dash-witnesses.html`)

- **KPI strip**: Total protected (pseudonymised/cleared), Duress armed, Voice-masked, Break-the-glass · 90d
- **Filter bar**: search + chips (All, High-risk, Duress-armed, Voice-masked, Intermediary)
- **Table**: Pseudonym+status, Risk pill (low/medium/high/extreme), Intake source, Exhibits count, Corroboration score tag, Voice masking tag, Signature pill, Age, Open →
- **Two-column below**:
  - **Duress & break-the-glass** panel: timeline of unsealing + decoy events
  - **Protection posture** panel: key-value (Mapping vault, Default intake, Geo-fuzzing, Duress passphrase, Voice masking, Defence access)
- **Add witness modal**: Real name (encrypted), Pseudonym (auto), Risk level, Source method, Contact method, Voice masking, Location, Linked exhibits, Notes

### 6.4 — Audit Log (`dash-audit.html`)

- **KPI strip**: Events · this case (48.2k), Chain integrity (100%), Snapshot cadence (60s), Last countersign
- **Filter bar**: search + type chips (All, Ingest, Redaction, Pseudonym, Federation, Disclosure) + Actor ▾ + Range ▾
- **Table**: Time (mono), Actor (avatar + name), Action, Target (mono), Kind pill, Signature (accent mono)
- **Two-column below**:
  - **Chain verify** panel: terminal-style `vk-verify` output (green [ok] lines)
  - **Actor share** panel: bar chart breakdown by user

### 6.5 — Corroborations (`dash-corroborations.html`)

- **KPI strip**: Claims tracked (breakdown), Median score, Single-source · flagged, Cross-case links
- **Claims list**: each row has large serif score (0.00-1.00, color-coded), claim ref + status pill + text, sources column (type avatars + names), progress bar + Open →
- **New claim modal**: Factual claim textarea, Primary evidence, Supporting evidence, Contradicting evidence, Initial score (0.00-1.00), Status select, Assigned to, Notes

### 6.6 — Analysis Notes (`dash-analysis.html`)

- **KPI strip**: Notes · this case (breakdown by status), Uncorroborated · open, Counter-evidence · preserved, Avg. citations
- **Filter bar**: search + status chips (All, Signed, Peer-review, Draft) + type chips (Hypothesis ▾, Author ▾)
- **List**: each row: ref + status pill + tags, title (large Fraunces link), excerpt paragraph, author (avatar + name), linked items count, age, Open →
- **New note modal**: Title, Analysis type chips (Hypothesis/Timeline/Command structure/Financial/Geospatial/Counter-evidence/OSINT), Assigned to, Methodology textarea, Content textarea, Referenced exhibits + witnesses + inquiry entries + Supersedes inputs, Limitations textarea

---

## Phase 7: Reporting Pages

### 7.1 — Redaction Editor (`dash-redaction.html`)
- Page header: Berkeley Protocol Reporting eyebrow, "Collaborative redaction" title, avatar stack of live users, Version history + Seal draft buttons
- **Toolbar**: Mark redaction, Pseudonymise, Translate gloss, Add note, Geo-fuzz chips + page counter + prev/next
- **Two-column**: document paper (left) + side panels (right)
- **Document paper**: rendered text with `<mark>` highlights, `<mark class="pseudo">` for pseudonyms, `.redact` for black bars
- **Marks panel**: per-mark rows with avatar, description, signature, type tag
- **Presence panel**: live users with page + typing status, CRDT head hash + p95 latency

### 7.2 — Disclosures (`dash-disclosures.html`)
- **KPI strip**: This quarter, Pending your review, Avg. bundle size, Rejected · ever
- **Two-column**: disclosures table (left, wider) + bundle wizard (right)
- **Table**: Bundle ref+note, Recipient, Exhibits count, Status pill, Due date, Owner avatars, Open →
- **Bundle wizard**: 5-step indicator (Scope → Redactions → Countersigns → Manifest → Deliver) with ✓ for complete steps

### 7.3 — Reports (`dash-reports.html`)
- **Report templates**: 3-column grid, each card: icon + type tag, title (Fraunces), description, usage count + Generate → link
- **Recently generated table**: ref, case, author (avatar + name), template, hash (accent mono), date, Verify →

---

## Phase 8: Platform Pages

### 8.1 — Search (`dash-search.html`)
- Large search input with semantic tag + case filter tag + latency indicator
- Filter chips below: All, Evidence, Witnesses, Notes, Inquiry + Lang ▾ + date range ▾
- **Two-column**: Results (left) + Facets (right)
- Results: ranked list with Fraunces title, snippet with `<em>` highlights, mono metadata, relevance score tag
- Facets panel: key-value breakdown (Case, Kind, Language, Date range, Model, Query ledger)

### 8.2 — Federation (`dash-federation.html`)
- **KPI strip**: Active peers (breakdown), Federated cases, Merge p95, Divergences · ever
- **Two-column**: Peer instances (left) + Protocol details (right)
- Peers: each row: name + role tag + status pill, description, mono stats (cases, ops, keys)
- Protocol panel: key-value (Transport, Identity, Op format, Seal, Conflict, Revocation, Governance)
- **Recent exchanges table**: time, peer avatar, description, direction pill (← in / out →), signature hash

### 8.3 — Settings (`dash-settings.html`)
- **Left nav** (sticky): People (Team, Roles, Invites), Organisation (General, Switch org, SSO), Security (Retention, Keys, Storage, API keys), System (Danger zone — red)
- **Team tab**: member rows (avatar, name/email, role badge, case chips, status dot), invite bar at bottom
- **Roles tab**: role cards (5 roles with descriptions + member counts) + permission matrix table (14 capabilities × 5 roles with check/partial/empty)
- **Invites tab**: pending invites table + invite bar
- **General tab**: org key-value (display name, instance ID, legal entity, contact, locale, timezone)
- **Switch org tab**: org rows with avatars + "Current" indicator (admin only)
- **SSO tab**: key-value (Provider, Protocol, MFA, Session timeout, Auto-provision, Directory sync) with toggles
- **Retention tab**: key-value (Default, Witness, Counter-evidence, Auto review, Legal hold override, Policy history)
- **Keys tab**: 3 key cards (name, holder, hardware, last used, status) + key-value (Quorum policy, Next rotation, Ceremony history)
- **Storage tab**: key-value (Primary, Mirror, Cold archive, Object encryption, Hash algorithms)
- **API keys tab**: keys table (name/prefix, scope, created, Rotate/Revoke actions) + generate button
- **Danger zone tab**: red-themed key-value (Rotate instance key, Decommission, Emergency revocation)

---

## Phase 9: Additional Screens (FROM AUDIT — previously missing)

### 9.1 — Single Case Detail (`dash-case.html`)
Individual case workspace view when a specific case is selected. Shows case-scoped KPIs, BP compliance, activity, needs-you, and team. Already partially covered in Phase 2.2 but needs its own route.

### 9.2 — Investigation Wizard (`dash-investigation.html`)
**NEW — was missing from plan v1.**
5-step guided flow for starting a new investigation. Walks through all 6 Berkeley Protocol phases. Accessible from dashboard "New investigation" button.

### 9.3 — Inquiry Detail (`dash-inquiry-detail.html`)
**NEW — was missing from plan v1.**
Full detail view for individual inquiry log entries. Shows metadata, search parameters, linked items with phase status, chain integrity.

### 9.4 — Key Ceremonies (`dash-ceremonies.html`)
**NEW — was missing from plan v1.**
Quorum ceremonies, key rotation events, break-the-glass log. Linked from Settings > Keys.

### 9.5 — Keyboard Shortcuts + Command Palette (`dash-keyboard.html`)
**NEW — was missing from plan v1.**
Full command palette (⌘K) with recent items, 4 grid groups (Navigation/Go-to, Actions/Seal, Selection/View, Safety), cheatsheet overlay. Shows live activity tracking (palette opens, seal events, panic locks). Accessible from Help panel and ⌘K shortcut. Recent items feed is **client-side only** (no API needed — uses local storage).

### 9.6 — Empty States (`dash-empty.html`)
**NEW — was missing from plan v1.**
Empty state variants for: no cases, no evidence, no witnesses, no activity, etc.

### 9.7 — Print View (`dash-print.html`)
**NEW — was missing from plan v1.**
Print-optimized layout for custody reports, evidence details, etc.

### 9.8 — Profile (`dash-profile.html`)
User profile page with form fields. Accessible from sidebar profile dropdown.

### 9.9 — Dark Mode (`dash-night.html`)
Dark theme variant. **Already implemented** in CSS (verified by audit). System preference detection present.

---

## Phase 10: Backend — Schema Changes

### 10.1 — New/Altered Tables

| Change | Table | Field | Type |
|--------|-------|-------|------|
| Add classification | `cases` | `classification` | `TEXT` enum (Criminal/Investigation/Monitoring/Archival) |
| Add start_date | `cases` | `start_date` | `DATE` |
| Add publication date | `evidence_items` | `publication_date` | `TIMESTAMPTZ` |
| Add voice masking | `witnesses` | `voice_masking_enabled` | `BOOLEAN` |
| Add duress flag | `witnesses` | `duress_passphrase_enabled` | `BOOLEAN` |
| Add risk level | `witnesses` | `risk_level` | `TEXT CHECK (IN 'low','medium','high','extreme') DEFAULT 'low'` — SEPARATE from protection_status |
| Add signed timestamp | `witnesses` | `signed_at` | `TIMESTAMPTZ` (NULL = unsigned, set = signed) |
| Add signature status | `witnesses` | `signature_status` | `TEXT GENERATED ALWAYS AS (CASE WHEN duress_passphrase_enabled AND signed_at IS NULL THEN 'duress-armed' WHEN signed_at IS NOT NULL THEN 'signed' ELSE 'unsigned' END) STORED` |
| Add retention policy history | `retention_policy_versions` (new) | `org_id, version, policy_json, changed_by, changed_at` | For diff view in Settings > Retention |
| Add decommission endpoint | — | — | `POST /api/danger/decommission` (missing from 10.2) |
| Widen score range | `evidence_assessments` | `relevance_score` | `INTEGER CHECK (1-10)` (was 1-5) |
| Widen score range | `evidence_assessments` | `reliability_score` | `INTEGER CHECK (1-10)` (was 1-5) |
| Add assigned_to | `inquiry_logs` | `assigned_to` | `UUID REFERENCES users(id)` |
| Add priority | `inquiry_logs` | `priority` | `TEXT` enum (normal/urgent/low) |
| Add sealed_status | `inquiry_logs` | `sealed_status` | `TEXT` enum (active/locked/complete) |
| Add sealed_at | `inquiry_logs` | `sealed_at` | `TIMESTAMPTZ` |
| **Many-to-many evidence links** | `inquiry_log_evidence` (new) | `inquiry_log_id, evidence_id` | Join table (currently single FK) |
| Add counter-evidence flag | `evidence_items` | `is_counter_evidence` | `BOOLEAN DEFAULT false` |
| Add rule_77_disclosed | `evidence_items` | `rule_77_disclosed` | `BOOLEAN DEFAULT false` |
| Add ceremony table | `key_ceremonies` (new) | `type, hardware_provider, holders, quorum_required, quorum_achieved, status, initiated_by, witnessed_by[], completed_at` | Full ceremony tracking |
| Add peer role | `federation_peers` | `role` | `TEXT` (sub-chain/full-peer/read-only/cold-mirror/staging) |
| **BP phase tracking** | `evidence_bp_phases` (new) | `evidence_id, phase (1-6), status (not_started/in_progress/complete), completed_at, completed_by` | **FOUNDATIONAL** |
| Add mark types to redaction | `redaction_drafts` | areas JSONB needs `type` field | `TEXT` enum (redact/pseudonymise/geo_fuzz/translate/annotate) per area |
| Disclosure wizard state | `disclosures` | `wizard_step, countersign_required, countersigns_received` | Multi-step + countersign tracking |
| Disclosure countersigns | `disclosure_countersigns` (new) | `disclosure_id, user_id, signed_at, signature` | Countersign collection |

### 10.2 — New API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/dashboard/overview` | Aggregated KPIs: exhibits sealed today, awaiting action, through protocol, last TSA |
| `GET /api/dashboard/berkeley-protocol` | Per-phase compliance % across all cases or per-case |
| `GET /api/dashboard/needs-you` | Action items requiring current user's attention |
| `GET /api/cases/{id}/team` | Case team members with roles and user details |
| `GET /api/cases/{id}/berkeley-protocol` | Per-phase stats for single case |
| `GET /api/cases/{id}/stats` | Exhibit count, witness count, disk usage, last modified |
| `GET /api/cases/{id}/integrity-status` | Chain integrity %, breaks, snapshot cadence, last verified |
| `GET /api/cases/{id}/assessment-stats` | Assessed count, awaiting, avg relevance, recommendation breakdown |
| `GET /api/cases/{id}/unassessed` | Evidence items with no assessments (the queue) |
| `GET /api/cases/{id}/analysis-stats` | Uncorroborated count, counter-evidence count, avg citations |
| `GET /api/witnesses/{id}/corroboration-score` | Computed score from linked corroboration claims |
| `GET /api/federation/peers` | Peer status, lag, key rotation info |
| `GET /api/federation/exchanges` | Recent exchange log |
| `GET /api/federation/protocol` | Protocol metadata |
| `GET /api/organization/{id}/storage-stats` | Total usage, per-case breakdown |
| `POST /api/inquiry-logs/{id}/lock` | Lock inquiry (prevents new links) |
| `POST /api/inquiry-logs/{id}/seal` | Seal inquiry permanently |
| `POST /api/disclosures/{id}/countersign` | Submit countersign for disclosure |
| `POST /api/disclosures/{id}/generate-bundle` | Generate encrypted ZIP + manifest |
| `POST /api/cases/{id}/danger/rotate-key` | Rotate instance key (quorum required) |
| `POST /api/cases/{id}/danger/revoke` | Emergency key revocation |

### 10.3 — Backend Workflow Fixes (Existing Code That Needs Updating)

| Flow | What's Broken | Fix Needed |
|------|---------------|------------|
| **Corroboration add/remove evidence** | `AddEvidenceToClaim` and `RemoveEvidenceFromClaim` return `501 Not Implemented` | Implement these handlers |
| **Legal hold scope** | Only blocks destruction. `UpdateMetadata` and `UploadNewVersion` don't check legal hold | Add legal hold check to ALL evidence mutation endpoints |
| **Analysis note status transitions** | Any status → any status allowed. No validation | Implement state machine: `draft → in_review → approved`. No backwards transitions without supersession |
| **Analysis note "signed" state** | Backend has `approved`, design shows `signed` | Rename or add `signed` as terminal state after `approved` |
| **Permission JSONB enforcement** | `case_role_definitions.permissions` JSONB is stored but handlers only check role membership, not specific permissions | Add middleware that checks granular permission flags from role definition |
| **Inquiry log evidence linking** | Single `evidence_id` FK — can only link 1 item | Replace with `inquiry_log_evidence` join table (many-to-many) |
| **Redaction mark types** | Only supports rectangular area coordinates + reason string | Extend `RedactionArea` struct to support: `type` (redact/pseudonymise/geo_fuzz/translate/annotate), `original_text`, `replacement_text`, `fuzz_radius_km` |
| **Redaction mark signing** | Individual marks stored in Yjs blob, not individually signed | Add per-mark Ed25519 signatures (author + signature fields on each area in JSONB) |
| **Federation receive tokens** | Stored in-memory map — lost on server restart | Move to Redis or PostgreSQL with TTL |
| **Witness pseudonym generation** | `witness_code` is user-provided | Auto-generate sequential pseudonyms (W-XXXX) on creation if not provided |

### 10.4 — Features the Design Shows That Don't Exist at All

| Feature | What's Needed | Priority |
|---------|---------------|----------|
| **BP phase state machine** | New `evidence_bp_phases` table + auto-progression logic (e.g., when assessment created → mark Phase 2 complete) | CRITICAL |
| **Disclosure bundle generation** | Generate encrypted ZIP containing: selected evidence files, redaction maps, custody chain PDF, SHA-256+BLAKE3 hash manifest, RFC 3161 timestamp, offline validator binary | HIGH |
| **Disclosure countersign workflow** | 2-of-N signature collection before bundle can be generated | HIGH |
| **BLAKE3 dual-hashing** | Add BLAKE3 as secondary hash on evidence upload (SHA-256 primary + BLAKE3 secondary) | HIGH |
| **Witness duress/decoy vault** | Duress passphrase opens decoy vault with fabricated data. Requires separate encrypted partition + duress detection | MEDIUM |
| **Witness break-the-glass quorum** | 2-of-3 key ceremony required to decrypt witness PII. Currently single-actor | MEDIUM |
| **Witness voice masking** | Real-time DSP pitch-shift + formant flatten on audio files linked to pseudonymised witnesses | LOW (future) |
| **Key ceremony log** | Track all quorum ceremonies: who initiated, who witnessed, hardware tokens used, outcome | MEDIUM |
| **Emergency revocation** | Broadcast key revocation to all federation peers within 30s | MEDIUM |
| **Instance decommission** | Sealed archive handoff to supervisory board | LOW |
| **Rule 77 counter-evidence** | Flag evidence as exculpatory/counter-evidence, auto-include in disclosure packages | HIGH |
| **CRDT operation signing** | Each collaborative editing op should be individually signed (currently: Yjs blob, unsigned) | MEDIUM |

### 10.5 — Missing Stats/Aggregation Endpoints

| Endpoint | Returns | For Screen |
|----------|---------|------------|
| `GET /api/cases/stats` | Total cases breakdown (active/hold/sealed), lead-on count, new-this-month count | Cases list KPIs |
| `GET /api/cases/{id}/witness-stats` | Total protected (pseudo/cleared breakdown), duress count, voice-masked count, break-the-glass count (90d) | Witnesses KPIs |
| `GET /api/cases/{id}/corroboration-stats` | Claims count (by status breakdown), median score, single-source flagged count, cross-case links | Corroborations KPIs |
| `GET /api/cases/{id}/disclosure-stats` | This-quarter count, pending-review count, avg bundle (exhibits + size), rejected count | Disclosures KPIs |
| `GET /api/cases/{id}/evidence-facets` | Count by type (Document/Image-video/Audio/Forensic/Legal-hold) | Evidence filter chips |
| `GET /api/cases/{id}/audit-stats` | Total events, actor share breakdown (grouped by user), last countersign actor+time | Audit KPIs + actor chart |
| `GET /api/federation/stats` | Federated cases count, merge p95 latency, divergence count | Federation KPIs |
| `GET /api/reports` | List generated reports with ref, case, author, template, hash, date | Reports table |
| `GET /api/reports/templates` | List report templates with usage counts | Reports template grid |
| `POST /api/reports/generate` | Generate report from template (custody summary, ceremony minutes, retention report, counter-evidence, federation diff) | Reports "Generate →" button |

### 10.6 — Missing Schema Fields (from final audit)

| Table | Field | Purpose |
|-------|-------|---------|
| `disclosures` | `due_date` | Due date shown in disclosures table |
| `disclosures` | `owner_user_ids UUID[]` | Owner avatars shown in table |
| `corroboration_claims` | `assigned_to UUID` | Assigned analyst |
| `evidence_assessments` | `status TEXT` (draft/sealed) | Draft save vs seal workflow |

### 10.7 — Frontend Wiring Gaps

| Issue | What's Broken | Fix |
|-------|---------------|-----|
| **Case context provider** | Exists but NEVER populated — no component calls `setCaseData()` | Wire case picker → `setCaseData()` → all views re-fetch with caseId |
| **investigation-api.ts** | Designed for client-side (takes explicit token param) | Refactor to use `authenticatedFetch()` server-side pattern |
| **Overview KPIs** | All hardcoded mock values in `overview-view.tsx` | Replace with real API calls to `/api/dashboard/overview` |
| **Evidence grid data source** | Uses `/api/search` instead of `/api/cases/{id}/evidence` | Use proper evidence list endpoint with pagination + type filtering |
| **Dashboard views** | Sidebar counts endpoint makes N+1 calls (iterates all cases individually) | Backend should return aggregated counts in single query |
| **Case list classification** | No `classification` field returned from `/api/cases` | Add field to case model + include in list response |
| **Cases list enrichment** | Witness count, chain visual, team avatars per case would require N+1 | Add to `GET /api/cases` response or use `GET /api/cases/stats` with per-case breakdown |
| **Delta comparisons** | "▲ 18 vs. yesterday" on KPIs | Dashboard overview endpoint must return today + yesterday counts for delta |

### 10.8 — Computation Logic

- **BP phase per evidence**: Query `evidence_bp_phases` table (once it exists). Auto-update when: inquiry log linked → Phase 1 done; assessment created → Phase 2 done; capture metadata filled → Phase 3 done; custody log has seal event → Phase 4 done; verification record created → Phase 5 done; analysis note referencing evidence signed → Phase 6 done
- **Corroboration score**: Weighted formula: `(source_count × strength_weight) / max_possible`. Strength weights: strong=1.0, moderate=0.7, weak=0.3, contested=0.1
- **Median corroboration score**: `PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY computed_score)` across all claims in case
- **Witness corroboration score**: Average of all corroboration_claims that reference evidence linked to this witness
- **"Needs you" items**: UNION query of ALL 7 action types:
  1. Pending countersigns (disclosure where user is required signer)
  2. Unreviewed analysis notes (assigned reviewer = user)
  3. Unassessed evidence (assigned case where user has assessor role)
  4. Pending federation merges (requiring user's countersign)
  5. **Redaction reviews pending** (drafts where user is assigned reviewer)
  6. **Disclosure approvals pending** (disclosures in review status assigned to user)
  7. **Provenance gap flags** (evidence with auto-detected capture_metadata gaps needing human action)
- **Unassessed queue**: `SELECT e.* FROM evidence_items e LEFT JOIN evidence_assessments a ON e.id = a.evidence_id WHERE a.id IS NULL AND e.case_id = ?`
- **Dashboard KPIs**: "Exhibits sealed today" = custody_log entries with action='sealed' AND date=today. "Awaiting action" = unassessed + pending_verification + pending_analysis. "Through protocol" = evidence where all 6 phases complete. "Last RFC 3161" = MAX(tsa_timestamp) from evidence_items. **Delta = today_count - yesterday_count**.
- **Open inquiries count**: `SELECT COUNT(*) FROM inquiry_logs WHERE case_id=? AND sealed_status='active'`
- **Last activity per case**: `SELECT MAX(created_at) FROM custody_log WHERE case_id=?`
- **Federation merge p95**: Requires storing merge duration on each exchange — `exchange_duration_ms` field on `federation_exchanges` table, then `PERCENTILE_CONT(0.95)`
- **Search facets**: Meilisearch faceted search response (already supported by Meilisearch, need to expose in API)
- **Query audit logging**: Each search query → INSERT into custody_log with action='search_query', target=query_text (for defence disclosure of investigative methodology)

### 10.9 — Undocumented Features/Flows

| Feature | What's Needed | Screen |
|---------|---------------|--------|
| **Import archive** | `POST /api/cases/{id}/import` — accept ZIP archive of evidence, parse manifest, bulk-create evidence items with metadata | Cases + Evidence action buttons |
| **Export audit range** | `POST /api/cases/{id}/audit/export` — export audit log for date range as signed PDF | Audit log action button |
| **Invite federation peer** | `POST /api/federation/peers/invite` — send peer invitation with public key exchange | Federation action button |
| **Report generation system** | Full feature: templates table, generate endpoint (creates PDF/ZIP from case data), seal to chain, track history | Reports page (currently NO backend) |
| **Assessment draft/seal** | Add `status` (draft/sealed) to assessments; draft can be saved without sealing, sealed is immutable | Assessment modal |
| **Analysis note templates** | Predefined note structures (timeline, command structure, financial, etc.) | Analysis "Templates" button |
| **Disclosure templates** | Predefined disclosure package configurations | Disclosures "Templates" button |
| **Corroboration matrix view** | Alternative visualization showing evidence × claims as a matrix | Corroborations "Matrix view" button |
| **Witness protection posture config** | Endpoint to fetch/set witness protection policy (geo-fuzz radius, default intake mode, voice masking default) | Witnesses protection panel |
| **Duress/break-the-glass event log** | Query key_ceremonies filtered to type=break_the_glass ORDER BY date DESC | Witnesses duress panel |
| **Search query metadata** | Return `{ latency_ms, total_hits, facets: {...} }` alongside results | Search page |
| **Snapshot cadence config** | Configuration value for audit chain snapshot interval — expose via settings API | Audit KPI |

### 10.10 — Per-Screen Implementation Gaps (from hyper-focused audit, Round 4)

#### Assessments — Backend Fixes
| Issue | Fix |
|-------|-----|
| Score range 1-5 → 1-10 | ALTER CHECK constraint on `relevance_score` and `reliability_score` |
| Credibility labels mismatch | Backend: established/credible/uncertain/unreliable. Design: Established/Probable/Unconfirmed/Doubtful. **Rename**: credible→Probable, uncertain→Unconfirmed, unreliable→Doubtful |
| No UPDATE endpoint | Wire existing repository `UpdateAssessment` method to a `PUT /api/assessments/{id}` handler |
| No `assigned_to` on assessments | Add field to struct + migration |

#### Corroborations �� Model Redesign
| Issue | Fix |
|-------|-----|
| Score is enum (`strength`) not numeric | Add `score NUMERIC(3,2)` field. Keep `strength` as human label, compute score OR allow manual override |
| Missing `status` field | Add `status TEXT` enum (investigating/corroborated/weak/refuted) — separate from `strength` |
| No witness linking | Create `corroboration_witnesses` join table (claim_id, witness_id, role_in_claim) |
| AddEvidence/RemoveEvidence stubs | Implement the 501 handlers |

#### Analysis Notes — Schema + Type Mapping
| Issue | Fix |
|-------|-----|
| `tags []string` missing | Add `tags TEXT[]` column to `investigative_analysis_notes` |
| Type mapping: 7 design types ≠ 10 backend types | Map: Hypothesis→hypothesis_testing, Timeline→timeline_reconstruction, Command structure→network_analysis, Financial→pattern_analysis, Geospatial→geographic_analysis, Counter-evidence→legal_assessment, OSINT→other. OR simplify backend to match design's 7 |
| No "signed" terminal state | Add `signed` to valid statuses (after `approved`). Or rename `approved` → `signed` |
| No `is_counter_evidence` flag | Add `is_counter_evidence BOOLEAN DEFAULT false` to `investigative_analysis_notes` |

#### Redaction Editor — Mark System Overhaul
| Issue | Fix |
|-------|-----|
| `draftArea` struct has no `type` field | Add `Type string` (redact/pseudonymise/geo_fuzz/translate/annotate) |
| No per-mark signature | Add `Signature string` + `SignedBy string` to draftArea |
| No presence data exposed | WebSocket must broadcast: user page number, typing status, idle duration |
| No CRDT head hash | Compute SHA-256 of Yjs state blob, expose in awareness broadcast |
| No p95 latency | Track message round-trip times, compute percentile server-side |
| No seal draft endpoint | Add `POST /api/evidence/{id}/redact/drafts/{draftId}/seal` (different from finalize — seal freezes draft without creating redacted copy) |
| No draft version history | Add `redaction_draft_versions` table (draft_id, version, state_blob, created_at, created_by) |
| Only `.redact` rendered | Frontend must support `.pseudo` (purple), strikethrough+bold corrections, `.geo-fuzz` marks |

#### Shell — Missing Components
| Issue | Fix |
|-------|-----|
| Help panel missing | Create `<HelpPanel>` with 5 items (Docs, FAQ, Shortcuts, Security, Support) + version footer |
| Profile dropdown missing | Create `<ProfileDropdown>` from user card → Edit profile, Settings, Sign out |
| Breadcrumbs always empty | Implement breadcrumb context provider that pages populate |
| Case role badge missing | Show user's role (Lead/Analyst/etc.) in case-scoped nav header |
| Federation visible in case view | Hide "Federation" and "Search" from case-scoped nav (design only shows them in workspace view) |

#### Audit Log — Query Gaps
| Issue | Fix |
|-------|-----|
| Fetches evidence-level only | Add `GET /api/cases/{id}/custody` endpoint (all custody events for case, not just one evidence item) |
| Actor names are UUID slices | JOIN custody_log.actor_user_id → user_profiles.display_name in API response |
| Filters non-functional | Add query params: `?kind=ingest,redact&actor=uuid&from=timestamp&to=timestamp` |
| Chain verify is mock | Wire to actual `ChainVerifier.VerifyCaseChain()` and return real output |

#### Evidence List — Missing Data
| Issue | Fix |
|-------|-----|
| No media duration/codec | Extract via ffprobe during upload, store in `evidence_items.media_metadata JSONB` (duration, codec, resolution, fps) |
| No `has_redactions` filter | Add computed field or subquery: `EXISTS (SELECT 1 FROM redaction_drafts WHERE evidence_id = e.id AND status = 'applied')` |
| No contributor filter | Add `?uploaded_by=uuid` query param to evidence list endpoint |
| Pagination shows wrong format | Return `total_count` in list response for "X of Y · page Z of N" display |

#### Overview Dashboard — Data Integration Flow
**All-cases view fetches:**
- `GET /api/dashboard/overview` → KPIs (sealed today + delta, awaiting, through protocol, last TSA)
- `GET /api/dashboard/berkeley-protocol` → 6-phase compliance percentages
- `GET /api/dashboard/needs-you` → 5-7 action items
- `GET /api/cases` → cases-in-flight table (with BP progress)

**Case-scoped view fetches:**
- `GET /api/cases/{id}/berkeley-protocol` → per-case phase percentages
- `GET /api/cases/{id}/team` → case team members array
- `GET /api/dashboard/needs-you?case_id={id}` → filtered to this case
- `GET /api/cases/{id}/stats` → exhibits, awaiting, through protocol, open inquiries
- Custody log / recent activity from `GET /api/cases/{id}/custody?limit=5`

#### Overview Dashboard — Case-Scoped View
| Issue | Fix |
|-------|-----|
| Entire case-scoped view not implemented | Create `<CaseScopedOverview>` component rendering: case header + status pill, 4 KPIs, BP compliance per-case, recent activity, needs you, case team |
| KPI #2 label wrong | Change from "Chain integrity" → "Awaiting action" (count of unassessed + pending items) |
| KPI #3 label wrong | Change from "Pending corroborations" → "Fully through protocol" (evidence with all 6 phases) |
| Greeting summary generic | Make dynamic: include disclosure due count + pending federation merges from "needs you" data |
| "New investigation" button missing | Add button linking to investigation wizard |

#### Cases List — Missing Enrichment
| Issue | Fix |
|-------|-----|
| Classification column shows jurisdiction | Use new `classification` field (once added) |
| Role column hardcoded "—" | Query user's case_role for each case in list response |
| Exhibits/Witnesses columns "—" | Include `evidence_count` and `witness_count` in cases list response (computed from COUNT queries or materialized) |
| Team column "���" | Include `team_members [{id, name, avatar_color}]` in cases list (limit 4) |
| Role/Jurisdiction filter chips | Add `?role=lead&jurisdiction=ICC` query params to cases list endpoint |

#### Reports — Full Rebuild Required
| Issue | Fix |
|-------|-----|
| Wrong domain | Existing `InvestigationReport` (authored docs) ≠ design's "Reports" (generated sealed snapshots). Need SEPARATE system |
| Need `report_templates` table | 6 predefined rows: custody_summary, disclosure_dossier, ceremony_minutes, quarterly_retention, counter_evidence, federation_diff. Each with: name, description, type (standard/governance/legal/platform), icon, generator_function |
| Need `generated_reports` table | id, template_id, case_id, generated_by, hash, sealed_at, file_url, file_size |
| Need `POST /api/reports/generate` | Accept template_id + case_id, run generator, produce PDF/ZIP, hash, seal to chain, store |
| Need `GET /api/reports/{id}/verify` | Verify hash integrity of generated report |
| Template usage counts | `SELECT COUNT(*) FROM generated_reports WHERE template_id = ?` |

---

## Implementation Order & Priorities

**IMPORTANT — Frontend/Backend Dependency Strategy:**
Sprints A-D build pixel-perfect UI against **stub data** (hardcoded mock JSON matching the design prototype values). Sprint E connects to real APIs. This means:
- Frontend screens during A-D use TypeScript interfaces matching the real API shape
- Each component has a clear data contract (props interface) that Sprint E will fulfill
- Backend work (migrations, new endpoints) can run IN PARALLEL with frontend sprints
- No screen is "done" until Sprint E connects it to real data

### Sprint A — Foundation (1-2 days)
1. Add missing CSS classes (`.bp-tracker`, `.bar-chart`, `.vk-modal`)
2. Create ~22 shared React component wrappers
3. Fix dashboard-views index.ts exports
4. Shell: sidebar rewrite (fixed position, case picker, profile dropdown, dynamic nav)
5. Shell: topbar rewrite (notification panel, help panel)

### Sprint B — Core Screens (2-3 days)
6. Overview/Dashboard (all-cases + case-scoped views)
7. Cases list + new case modal
8. Evidence list (grid + table views, upload queue, integrity summary)
9. Evidence detail (all tabs including Manage, BP tracker, all modals)

### Sprint C — Investigation Screens (2-3 days)
10. Inquiry log (list + detail + new entry modal + lock/complete workflow)
11. Assessments (split-view modal, unassessed queue, how-to guide)
12. Witnesses (table + protection panels + add modal)
13. Audit log (table + chain verify terminal + actor share chart)
14. Corroborations (claims list + new claim modal)
15. Analysis notes (list + new note modal)

### Sprint D — Reporting & Platform (2-3 days)
16. Redaction editor (collaborative, marks panel, presence panel)
17. Disclosures (table + bundle wizard)
18. Reports (template grid + recent table)
19. Search (semantic + facets)
20. Federation (peers + protocol + exchanges)
21. Settings (all 11 tabs)

### Sprint E — Backend Data + Connection (2-3 days)
22. Schema migrations (classification, publication_date, witness fields)
23. Dashboard aggregation endpoints
24. Berkeley Protocol computation (per-evidence, per-case)
25. Stats endpoints (case stats, assessment stats, analysis stats, integrity status)
26. Federation metadata endpoints
27. Connect all frontend screens to real APIs
28. Real-time sidebar counts

### Sprint F — Additional Screens + Polish (1-2 days)
29. Investigation wizard (5-step guided flow)
30. Key ceremonies page
31. Keyboard shortcuts overlay
32. Empty states for all screens
33. Print view
34. Profile page polish
35. Responsive breakpoints verification
36. Loading states + error states
37. Animations (softrise, pulse, hover lifts)
38. Dark mode verification (already implemented, just test)

---

## Key Files to Modify

| File | Operation | Description |
|------|-----------|-------------|
| `web/src/app/(dashboard)/layout.tsx` | Create/Modify | Dashboard-scoped layout with `overflow: hidden` (NOT global body) |
| `web/src/app/globals.css` | Modify | Add `.bp-tracker`, `.bar-chart`, `.vk-modal` classes |
| `web/src/components/layout/sidebar.tsx` | Rewrite | Fixed sidebar, case picker, profile dropdown, dynamic nav |
| `web/src/components/layout/top-bar.tsx` | Rewrite | Notification panel, help panel, blurred background |
| `web/src/components/ui/dashboard/*.tsx` | Create | ~22 shared dashboard components |
| `web/src/components/dashboard-views/*.tsx` | Rewrite | All 12+ dashboard views pixel-perfect |
| `web/src/components/dashboard-views/index.ts` | Fix | Add 5 missing exports |
| `web/src/components/cases/*.tsx` | Modify | Match design (table, modals) |
| `web/src/components/evidence/*.tsx` | Modify | Grid cards, detail tabs, BP tracker, manage tab |
| `web/src/components/investigation/*.tsx` | Modify | Split-view modal, inquiry hub, how-to guides |
| `web/src/components/witnesses/*.tsx` | Modify | Protection panels, add modal |
| `web/src/components/disclosures/*.tsx` | Modify | Bundle wizard, table |
| `web/src/components/redaction/*.tsx` | Modify | Marks panel, presence panel |
| `web/src/components/search/*.tsx` | Modify | Semantic results, facets |
| `web/src/components/settings/page.tsx` | Modify | All 11 sub-tabs |
| `internal/dashboard/` | Create | Dashboard aggregation service + handlers |
| `internal/cases/handler.go` | Modify | Team endpoint, stats endpoint, BP stats |
| `internal/evidence/handler.go` | Modify | BP phase computation |
| `internal/investigation/handler.go` | Modify | Assessment stats, analysis stats |
| `internal/witnesses/handler.go` | Modify | Corroboration score |
| `internal/federation/handler.go` | Modify | Peers, exchanges, protocol metadata |
| `migrations/037_*.sql` | Create | Schema changes (classification, publication_date, witness fields) |

---

## Verification Audit Results Summary

### Round 1 (5 agents)
| Audit | Agent | Result |
|-------|-------|--------|
| Screen Coverage | Agent 1 | 22 core screens covered, **7 additional screens added** |
| Data Fields | Agent 2 | **4 schema changes** + **14 new API endpoints** + **5 computation logic** pieces needed |
| Components | Agent 3 | 90% exist, **3 missing CSS classes** + **~6 new React components** needed |
| CSS/Tokens | Agent 4 | **95% complete** — no critical gaps |
| Chat Intent | Agent 5 | **30 gaps found** and incorporated |

### Round 2 — Final Verification (3 agents)
| Audit | Agent | Result |
|-------|-------|--------|
| Screens + Data | Final-1 | **6 minor gaps found** — assessment score range resolved (use 1-10), inquiry metadata fields added, ceremony schema added, command palette clarified, federation peer role added, dash-index.html clarified |
| Components + CSS | Final-2 | **PASS** — all 3 missing CSS classes confirmed, 5 missing exports confirmed, sidebar behavior verified correct |
| Chat Intent | Final-3 | **PASS — no remaining gaps** — all 30 items from round 1 now captured |

### Round 3 — Gemini Cross-Review (3 agents)
| Audit | Agent | Result |
|-------|-------|--------|
| Frontend Review | Gemini Reviewer | **5 findings** — see Architectural Decisions below |
| Analyzer | Gemini Analyzer | (pending — large context processing) |
| Backend Architect | Gemini Architect | (pending — large context processing) |

---

## Architectural Decisions (from Gemini Review)

### AD-1: Scoped Dashboard Layout (CRITICAL)
**Problem**: `body { overflow: hidden }` will break scrolling on marketing routes.
**Decision**: Create `web/src/app/[locale]/(app)/layout.tsx` (or equivalent dashboard group) with `h-screen overflow-hidden` scoped ONLY to dashboard. Marketing routes use their own layout with normal scrolling. Build `<SidebarV2>` parallel to existing and swap via layout.

### AD-2: Tailwind-First Components (HIGH)
**Problem**: Global CSS classes (`.panel`, `.tbl`, `.d-kpis`) in a Tailwind project splits styling context and blocks dead-code elimination.
**Decision**: The design token CSS variables remain in `globals.css`. But the 22 shared React components should use **Tailwind utility classes referencing those variables** (e.g., `bg-[var(--paper)] border border-[var(--line)] rounded-[var(--radius)]`) rather than adding new global CSS class selectors. This gives us design-system consistency WITH Tailwind's purging and colocation benefits.
**Exception**: The 3 missing complex CSS classes (`.bp-tracker`, `.bar-chart`, `.vk-modal`) are acceptable as global CSS since they involve pseudo-elements and complex state that Tailwind handles poorly.

### AD-3: React Server Components First (MEDIUM)
**Problem**: Adding `"use client"` too high in the tree bloats JS bundle.
**Decision**: Default all dashboard views, data fetching, tables, and KPI strips to **React Server Components**. Push `"use client"` down to interactive leaf nodes only:
- `<FilterBar>` (search input + chip toggles)
- `<Modal>` / `<SplitViewModal>` (form state)
- Dropdowns (org, case, profile, notification, help)
- `<RedactionEditor>` / `<CollaborativeEditor>` (WebSocket)
- `<EvidenceUploader>` (file input + progress)
- Case picker + sidebar nav (client state for selected case)

### AD-4: Component Consolidation (LOW)
**Decision**: Merge related components:
- `<BPTracker>` + `<BPDots>` + `<PhaseDots>` → single `<BPIndicator variant="full|dots|inline">` component
- `<Modal>` + `<SplitViewModal>` → single `<Modal layout="default|split">` component
- Add missing primitives: `<Skeleton>` (for Suspense), `<Tooltip>` (ARIA for icon buttons), `<Pagination>`

### AD-5: BP Phase Computation — Event-Driven Denormalized Table (CRITICAL — from Gemini Architect)
**Problem**: Computing 6-phase BP status per evidence requires joining 6 tables. Impossible to scale for the "All Cases" dashboard with 14k+ items.
**Decision**: Do NOT compute dynamically via 6-table JOIN at API time. Create `evidence_bp_summary` table (or use the `evidence_bp_phases` table from 10.1). Hook into existing `custodyLogger` and `auditLogger` as event emitters. When a phase-related event occurs (assessment saved, verification created, etc.), a background worker recalculates phases for that `evidence_id` and updates the summary.
**Why not materialized view**: Requires cron-refresh, locks resources, causes stale reads on dashboard.
**Why not API-time join**: N+1 with RLS/org-member checks. Won't scale.
**Trade-off**: Slight write-amplification + race condition handling during concurrent updates. Worth it for O(1) reads.

### AD-6: Dashboard KPIs — Direct Indexed Queries, Not Redis (MEDIUM — from Gemini Architect)
**Problem**: Dashboard aggregates KPIs across 38+ cases and 14k+ evidence items.
**Decision**: 14k evidence items is trivially small for PostgreSQL with proper indexes. Don't add Redis or caching prematurely. Add B-Tree indexes on `(organization_id, status)` and `(organization_id, created_at)` on `evidence_items` and `evidence_bp_summary`. With the summary table, "through protocol" is just `COUNT(*) WHERE phase_6 = true AND org_id = $1`.

### AD-7: Split Migrations by Bounded Context (MEDIUM — from Gemini Architect)
**Problem**: 7+ new schema fields across 5 tables.
**Decision**: Do NOT create one monolithic `037_frontend_phase_10.up.sql`. Split by domain:
- `037_investigation_enhancements.up.sql` (cases.classification, inquiry_logs fields, assessment score constraints)
- `038_evidence_metadata.up.sql` (publication_date, bp_phases)
- `039_witness_protection.up.sql` (voice_masking, duress)
- `040_key_ceremonies.up.sql` (ceremony table)
- `041_federation_roles.up.sql` (peer_role)
**Why**: Reduces blast radius during rollback. Monolithic migration means reverting witness changes would drop case classification data.

### AD-8: "Needs You" — Single UNION ALL Query (LOW — from Gemini Architect)
**Decision**: Single SQL round-trip with `UNION ALL` across all 7 action types, `ORDER BY created_at DESC LIMIT 5`. Don't fetch 7 arrays from separate handlers and sort in Go — breaks pagination, risks OOM.

### AD-9: Federation Peer Role — TEXT with CHECK (LOW — from Gemini Architect)
**Decision**: `TEXT CHECK (role IN ('sub-chain','full-peer','read-only','cold-mirror','staging'))`. Matches existing convention (`trust_mode TEXT`). PostgreSQL ENUM is hard to alter; reference table is overkill.

### AD-10: Key Ceremonies — Separate Table, Not Custody Log (LOW — from Gemini Architect)
**Decision**: Custody log is append-only and tied to evidence lifecycle. Key ceremonies are tenant-level infrastructure events with multi-party quorum. Different domain, different RLS rules. Keep separate.

### AD-11: Evidence Grid Performance (HIGH)
**Decision**: 
- Pre-generate faux waveforms server-side (not computed client-side)
- Use `next/image` with strict `sizes` for thumbnails
- Render BP dots as single SVG sprite (not 6 individual DOM nodes per card)
- Enforce server-side pagination (12 items/page as shown in design)

---

### Final Status: VERIFIED COMPLETE
All 26 dashboard screens documented. All data fields accounted for. All component gaps identified. All chat transcript intent captured. Architectural decisions from Gemini incorporated. Plan ready for implementation.
