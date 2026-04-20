# Implementation Plan: Investigation UX Polish -- 60 Gaps

## Overview

The investigation workspace in VaultKeeper has 10 React components, a working Go backend with 30+ endpoints, and all TypeScript types defined. However, the frontend is disconnected: 4 forms are not wired in, record cards are inconsistent, there are no status transitions, no cross-referencing pickers, and no dashboard. This plan closes all 60 identified UX gaps across 13 work packages, turning the prototype into a production-grade investigation tool for criminal investigations under the Berkeley Protocol.

## Architecture Summary (Current State)

**Backend (COMPLETE)**:
- `internal/investigation/handler.go` -- all routes registered, only `DeleteInquiryLog`, `AddEvidenceToClaim`, `RemoveEvidenceFromClaim` return 501
- `internal/investigation/service.go` -- full CRUD, case membership checks, self-verification prevention
- Report statuses: `draft -> in_review -> approved -> published -> withdrawn`
- Publish endpoint exists (`POST /api/reports/{id}/publish`) -- requires `approved` status
- UpdateReport exists in repository but **no HTTP endpoint for status transitions** (only PublishReport)
- Template instance update endpoint exists (`PUT /api/template-instances/{id}`) with status + content

**Frontend (GAPS)**:
- `investigation-page-client.tsx` (831 lines) -- main workspace, 8 tabs, fetches data client-side
- `inquiry-log-form.tsx` -- WIRED, saves via POST, calls `window.location.reload()`
- `analysis-note-editor.tsx` -- WIRED, saves, raw UUID inputs for related items
- `report-builder.tsx` -- WIRED (create only), no status transitions, no evidence/analysis pickers
- `template-editor.tsx` -- EXISTS but NOT WIRED (needs `templateId` prop, no picker in TemplatesSection)
- `corroboration-builder.tsx` -- EXISTS but NOT WIRED (needs `evidenceItems` prop, not passed)
- `safety-profile-form.tsx` -- EXISTS but NOT WIRED (needs `userId` prop, not available)
- `assessment-form.tsx` -- EXISTS but NOT WIRED (needs `evidenceId`, should be on evidence detail)
- `verification-form.tsx` -- EXISTS but NOT WIRED (needs `evidenceId`, should be on evidence detail)
- `evidence-detail.tsx` -- no investigation integration (no assess/verify buttons)

**Missing from case-detail.tsx**: `session.user.id` not passed to `CaseDetail` -> `InvestigationPageClient`

---

## WP-1: Wire All Disconnected Forms

### Gap Count: 5 gaps

### Step 1.1: Pass `userId` down from server page

**File**: `web/src/app/[locale]/(app)/cases/[id]/page.tsx`
- **Action**: Add `userId={session.user.id}` prop to `<CaseDetail>`
- **API**: None (data already in session)

**File**: `web/src/components/cases/case-detail.tsx`
- **Action**: Add `userId: string` to CaseDetail props interface, pass through to `<InvestigationPageClient userId={userId} />`

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Action**: Add `userId: string` to `InvestigationPageClientProps`

### Step 1.2: Wire SafetyProfileForm into Safety section

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change**: Replace placeholder text in SafetySection `showForm` block with:
```
<SafetyProfileForm
  caseId={caseId}
  userId={userId}
  accessToken={accessToken}
  onSaved={() => {
    setShowForm(false);
    refreshSafetyProfiles();
  }}
/>
```
- **API**: `PUT /api/cases/{caseID}/safety-profiles/{userID}` (already implemented)
- **Dependency**: Step 1.1 (needs userId)
- **Risk**: Low -- SafetyProfileForm already works, just needs wiring

### Step 1.3: Wire CorroborationBuilder into Corroborations section

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change in CorroborationsSection**: Fetch evidence items for the case when form opens
```
const [evidenceItems, setEvidenceItems] = useState<EvidenceOption[]>([]);

// Fetch on mount or when showForm opens
useEffect(() => {
  if (!showForm) return;
  fetch(`${API}/api/cases/${caseId}/evidence?current_only=true`, {
    headers: { Authorization: `Bearer ${accessToken}` }
  })
  .then(r => r.ok ? r.json() : null)
  .then(json => {
    const items = (json?.data || []).map(e => ({
      id: e.id,
      evidence_number: e.evidence_number,
      title: e.title || e.original_name,
    }));
    setEvidenceItems(items);
  });
}, [showForm, caseId, accessToken]);
```
- Replace placeholder with `<CorroborationBuilder caseId={caseId} evidenceItems={evidenceItems} accessToken={accessToken} onSaved={...} />`
- **API**: `GET /api/cases/{caseID}/evidence` (existing), `POST /api/cases/{caseID}/corroborations` (existing)
- **Risk**: Low

### Step 1.4: Wire TemplateEditor into Templates section

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change in TemplatesSection**:
  - Add `selectedTemplateId` state
  - Make the "Available Templates" list items clickable -- on click, set `selectedTemplateId` and show form
  - When `showForm && selectedTemplateId`, render `<TemplateEditor templateId={selectedTemplateId} caseId={caseId} accessToken={accessToken} onSaved={...} />`
- **ASCII wireframe**:
```
┌──────────────────────────────────────────────────┐
│ Templates                          [Fill Template]│
├──────────────────────────────────────────────────┤
│ AVAILABLE TEMPLATES                               │
│ ┌──────────────────────────────────────────────┐  │
│ │ ▶ Investigation Plan · investigation_plan    │  │
│ │   Systematic plan for online investigations  │  │
│ ├──────────────────────────────────────────────┤  │
│ │ ▶ Threat Assessment · threat_assessment      │  │
│ │   Digital threat and risk assessment          │  │
│ ├──────────────────────────────────────────────┤  │
│ │ ▶ Digital Landscape · digital_landscape      │  │
│ │   Assessment of digital landscape             │  │
│ └──────────────────────────────────────────────┘  │
│                                                   │
│ INSTANCES (2 records)                             │
│ ┌──────────────────────────────────────────────┐  │
│ │ Investigation Plan · Draft    12 Apr 2026 ▾  │  │
│ │ Threat Assessment · Active    10 Apr 2026 ▾  │  │
│ └──────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```
- **API**: `GET /api/templates/{id}` (existing), `POST /api/cases/{caseID}/template-instances` (existing)
- **Risk**: Low

### Step 1.5: Wire AssessmentForm + VerificationForm into evidence detail

Deferred to WP-4 (evidence-investigation integration). These forms need the evidence detail page context, not the investigation tab.

### Testing Strategy (WP-1)
- Unit: Verify each section renders the form component with correct props
- Integration: Submit each form, verify API call, verify data refresh
- E2E: Full flow -- navigate to investigation tab, open each section, fill and save

---

## WP-2: Investigation Dashboard

### Gap Count: 5 gaps

### Step 2.1: Create InvestigationDashboard component

**File**: `web/src/components/investigation/investigation-dashboard.tsx` (NEW)

**Props**:
```typescript
interface InvestigationDashboardProps {
  readonly inquiryLogCount: number;
  readonly assessmentCount: number;
  readonly verificationCount: number;
  readonly corroborationCount: number;
  readonly analysisNoteCount: number;
  readonly reportsByStatus: Record<string, number>;
  readonly totalEvidence: number;
  readonly assessedEvidenceCount: number;
  readonly verifiedEvidenceCount: number;
  readonly onNavigateToTab: (tab: TabKey) => void;
}
```

**ASCII wireframe**:
```
┌────────────────────────────────────────────────────────────┐
│ INVESTIGATION OVERVIEW                                      │
├─────────┬─────────┬──────────┬──────────┬─────────────────┤
│ Evidence│ Assessed│ Verified │ Unverif. │ Reports         │
│   42    │   28    │   22     │    20    │ 2 draft, 1 pub  │
├─────────┴─────────┴──────────┴──────────┴─────────────────┤
│                                                             │
│ BERKELEY PROTOCOL PHASES                                    │
│ ━━●━━━━━━━●━━━━━━━●━━━━━━━●━━━━━━━○━━━━━━━○              │
│  Inquiry  Assess  Collect  Preserve  Verify  Analyze      │
│   (12)    (28)     (42)     (42)     (22)     (5)         │
│                                                             │
│ NEEDS ATTENTION                                             │
│  · 14 evidence items unassessed → Assessments tab          │
│  · 20 evidence items unverified → Verifications tab        │
│  · 2 reports in draft → Reports tab                         │
│  · 1 analysis note in review → Analysis tab                 │
└────────────────────────────────────────────────────────────┘
```

**Implementation notes**:
- Counter cards use the existing `card-inset` class with the design system color tokens
- Phase progress: 6 circles connected by a line, filled (green-tinted) if phase has records, hollow if empty
- "Needs Attention" queue: clickable links that call `onNavigateToTab()`
- assessedEvidenceCount / verifiedEvidenceCount require fetching `GET /api/cases/{caseID}/evidence` and cross-referencing with assessments/verifications. For now, use the counts from the investigation data (count unique evidence_ids in assessments array, count unique evidence_ids in verifications array)
- Keep component under 200 lines

### Step 2.2: Integrate dashboard above tabs

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change**: Import and render `<InvestigationDashboard>` above the `<nav>` tab bar
- Compute counts from live data arrays (e.g., `liveReports.filter(r => r.status === 'draft').length`)
- Pass `handleTabChange` as `onNavigateToTab`

### Testing Strategy (WP-2)
- Unit: Render dashboard with various count combinations, including all zeros
- Visual: Screenshot test of phase progress indicator
- E2E: Click "Needs Attention" links, verify correct tab activates

---

## WP-3: Report Workflow

### Gap Count: 6 gaps

### Step 3.1: Add report status transition endpoint (Backend)

**File**: `internal/investigation/handler.go`
- **Add route**: `r.Post("/status", h.TransitionReportStatus)` inside the `/api/reports/{id}` group
- **New handler function**: `TransitionReportStatus` -- accepts `{"status": "in_review"}`, validates allowed transitions

**File**: `internal/investigation/service.go`
- **Add function**: `TransitionReportStatus(ctx, id, newStatus, actorID, actorRole) (InvestigationReport, error)`
- **Valid transitions**:
  - `draft -> in_review` (author)
  - `in_review -> approved` (prosecutor/judge only)
  - `approved -> published` (already exists via PublishReport)
  - `any -> withdrawn` (author or prosecutor/judge)
  - `withdrawn -> draft` (author, to re-edit)
- Uses existing `repo.UpdateReport()`

**API**: `POST /api/reports/{id}/status` with body `{"status": "in_review"}`

### Step 3.2: Report status transition buttons in UI

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change in ReportsSection**: Inside the ExpandableCard for each report, add a status transition button bar:
```
┌─────────────────────────────────────────────────────┐
│ Quarterly Analysis Report                            │
│ interim · draft · 3 sections          12 Apr 2026 ▾ │
│─────────────────────────────────────────────────────│
│  Report Type: Interim       Status: Draft            │
│  ...sections...                                      │
│                                                      │
│  ┌─────────────────────┐  ┌───────────┐             │
│  │ Submit for Review ▶  │  │ Edit ✎   │             │
│  └─────────────────────┘  └───────────┘             │
└─────────────────────────────────────────────────────┘
```
- Buttons change based on current status:
  - `draft`: "Submit for Review", "Edit", "Withdraw"
  - `in_review`: "Approve" (if prosecutor/judge), "Withdraw"
  - `approved`: "Publish" (if prosecutor/judge), "Withdraw"
  - `published`: no transitions (final state, read-only)
  - `withdrawn`: "Reopen as Draft"
- Each button calls `POST /api/reports/{id}/status` or `POST /api/reports/{id}/publish`
- After success, update `liveReports` state (no page reload)

### Step 3.3: Report edit mode

**File**: `web/src/components/investigation/report-builder.tsx`
- **Add optional prop**: `existingReport?: InvestigationReport`
- When `existingReport` is provided, pre-fill form state from it
- Change save action to `PUT /api/reports/{id}` instead of POST
- Title changes from "Build Report" to "Edit Report"

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change in ReportsSection**: The "Edit" button in ExpandableCard sets `editingReport` state, shows `<ReportBuilder existingReport={report} ... />`

**Backend dependency**: Need `PUT /api/reports/{id}` endpoint.

**File**: `internal/investigation/handler.go`
- **Add**: `r.Put("/", h.UpdateReport)` inside `/api/reports/{id}` group
- **New handler**: `UpdateReport` -- only allowed when status is `draft` or `withdrawn`

### Step 3.4: Report read/preview mode

**File**: `web/src/components/investigation/report-preview.tsx` (NEW, ~150 lines)

**Props**:
```typescript
interface ReportPreviewProps {
  readonly report: InvestigationReport;
  readonly onClose: () => void;
}
```

**ASCII wireframe**:
```
┌────────────────────────────────────────────────────┐
│ ← Back to Reports                                   │
│                                                      │
│ QUARTERLY ANALYSIS REPORT                            │
│ Interim Report · Published 12 Apr 2026               │
│ Author: John Smith · Approved by: Jane Doe           │
│                                                      │
│ ═══════════════════════════════════════════════════  │
│ 1. PURPOSE                                           │
│ ─────────                                            │
│ This report summarizes the investigation findings... │
│                                                      │
│ 2. METHODOLOGY                                       │
│ ──────────                                           │
│ The following analytical methods were applied...     │
│                                                      │
│ ═══════════════════════════════════════════════════  │
│ TRANSPARENCY                                         │
│ Limitations: Limited access to X platform data       │
│ Caveats: Source accounts may be pseudonymous          │
│ Assumptions: Content dates are accurate               │
│                                                      │
│ REFERENCED EVIDENCE (3 items)                        │
│ · EV-2026-001 — Screenshot of suspect post           │
│ · EV-2026-003 — Satellite imagery                    │
│ · EV-2026-007 — Witness statement                    │
└────────────────────────────────────────────────────┘
```

- **Change in ReportsSection**: Report title in ExpandableCard becomes clickable link to preview mode
- Add `viewingReport` state to ReportsSection

### Step 3.5: Evidence and analysis reference pickers in ReportBuilder

Deferred to WP-6 (Cross-Referencing & Pickers) since it depends on the EvidencePicker component.

### Testing Strategy (WP-3)
- Unit: Status transition validation (valid/invalid transitions)
- Integration: Full draft -> review -> approved -> published flow via API
- E2E: Create report, submit for review, approve, publish -- verify UI updates

---

## WP-4: Evidence-Investigation Integration

### Gap Count: 7 gaps

### Step 4.1: Add "Assess Evidence" and "Verify Evidence" buttons to evidence detail

**File**: `web/src/components/evidence/evidence-detail.tsx`
- **Add imports**: `AssessmentForm`, `VerificationForm` from investigation components
- **Add state**: `showAssessmentForm`, `showVerificationForm`, `assessments`, `verifications`, `claims`
- **Add effect**: Fetch assessments, verifications, and corroboration claims for this evidence item on mount
  - `GET /api/evidence/{evidenceID}/assessments`
  - `GET /api/evidence/{evidenceID}/verifications`
  - `GET /api/evidence/{evidenceID}/corroborations`
- **Add props**: `caseId: string` must be passed from the server page (look up from evidence.case_id)

**ASCII wireframe (new section below Description, above Hash verification)**:
```
┌────────────────────────────────────────────────────┐
│ INVESTIGATION                                        │
│                                                      │
│ ┌──────────────┐  ┌───────────────┐                 │
│ │ Assess ✎     │  │ Verify ✓      │                 │
│ └──────────────┘  └───────────────┘                 │
│                                                      │
│ ASSESSMENT SUMMARY                                   │
│ ┌──────────────────────────────────────────────────┐│
│ │ Relevance: ●●●●○  4/5  · Reliability: ●●●○○ 3/5││
│ │ Source: Credible · Recommendation: Collect        ││
│ │ Assessed by John Smith · 10 Apr 2026              ││
│ └──────────────────────────────────────────────────┘│
│                                                      │
│ VERIFICATION RECORDS (2)                             │
│ ┌──────────────────────────────────────────────────┐│
│ │ Source Authentication · Authentic · High conf. ▾  ││
│ │ Geolocation Verification · Likely Authentic    ▾  ││
│ └──────────────────────────────────────────────────┘│
│                                                      │
│ CORROBORATION CLAIMS (1)                             │
│ ┌──────────────────────────────────────────────────┐│
│ │ "Suspect was present at location on 2024-03-15"   ││
│ │ Event Occurrence · Strong · Supporting role        ││
│ └──────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────┘
```

**API endpoints used**:
- `GET /api/evidence/{evidenceID}/assessments` (existing)
- `GET /api/evidence/{evidenceID}/verifications` (existing)
- `GET /api/evidence/{evidenceID}/corroborations` (existing)
- `POST /api/evidence/{evidenceID}/assessments` (existing, via AssessmentForm)
- `POST /api/evidence/{evidenceID}/verifications` (existing, via VerificationForm)

### Step 4.2: Assessment summary card

**File**: `web/src/components/investigation/assessment-summary-card.tsx` (NEW, ~80 lines)

**Props**:
```typescript
interface AssessmentSummaryCardProps {
  readonly assessment: EvidenceAssessment;
}
```
- Display score dots (filled circles), source credibility badge, recommendation badge
- Reuse the `ScoreDots` display style from AssessmentForm (read-only version)

### Step 4.3: Pass caseId to evidence detail page

**File**: `web/src/app/[locale]/(app)/evidence/[id]/page.tsx`
- The `evidence.case_id` is already on the evidence object -- use it directly
- Pass `accessToken` and `caseId={evidence.case_id}` to EvidenceDetail

**File**: `web/src/components/evidence/evidence-detail.tsx`
- Add `caseId: string` to props (derive from `evidence.case_id` if not passed)

### Step 4.4: Unverified evidence indicator in evidence grid

**File**: `web/src/components/evidence/evidence-grid.tsx`
- Add optional `verificationStatus` field to the grid item display
- If `capture_metadata?.verification_status === 'unverified'`, show a small amber dot/badge next to the evidence number
- No new API call needed -- capture metadata is already on the evidence item

### Step 4.5: Evidence grid quick filters

**File**: `web/src/components/evidence/evidence-page-client.tsx`
- Add filter buttons above the grid: "All", "Unassessed", "Unverified"
- "Unassessed" filter: client-side filter using the assessment data (or backend query param if available)
- For MVP, add query param to `GET /api/cases/{caseID}/evidence?unassessed=true` -- this requires a backend JOIN which may be deferred

**Alternative (client-side)**: Fetch all assessments for the case, build a Set of assessed evidence IDs, filter evidence list client-side. This avoids backend changes.

### Testing Strategy (WP-4)
- Unit: AssessmentSummaryCard renders correctly for all score/credibility combinations
- Integration: Click "Assess", fill form, verify assessment appears on evidence detail
- E2E: Full assess + verify flow on evidence detail page

---

## WP-5: Template Workflow

### Gap Count: 5 gaps

### Step 5.1: Template picker as selectable cards

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change TemplatesSection**: Transform "Available Templates" from plain text rows into clickable cards:

**ASCII wireframe**:
```
┌──────────────────────────────────────────────────┐
│ Templates                          [Fill Template]│
├──────────────────────────────────────────────────┤
│ SELECT A TEMPLATE                                 │
│ ┌─────────────────┐ ┌─────────────────┐          │
│ │ 📋               │ │ ⚠️               │          │
│ │ Investigation    │ │ Threat           │          │
│ │ Plan             │ │ Assessment       │          │
│ │                  │ │                  │          │
│ │ Systematic plan  │ │ Digital threat & │          │
│ │ for online inv.  │ │ risk assessment  │          │
│ │                  │ │                  │          │
│ │ [Use Template]   │ │ [Use Template]   │          │
│ └─────────────────┘ └─────────────────┘          │
│ ┌─────────────────┐                               │
│ │ 🌐               │                               │
│ │ Digital          │                               │
│ │ Landscape        │                               │
│ │                  │                               │
│ │ Assessment of    │                               │
│ │ digital land...  │                               │
│ │                  │                               │
│ │ [Use Template]   │                               │
│ └─────────────────┘                               │
└──────────────────────────────────────────────────┘
```

- Each card click sets `selectedTemplateId` and shows the TemplateEditor below
- Use `grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3` layout
- Cards use `card` class with hover effect

### Step 5.2: Template instance detail with content preview

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change TemplatesSection instance list**: Use `ExpandableCard` for each instance
- Title: template name (lookup from templates array by `template_id`)
- Subtitle: status badge + date
- Expanded content: show each filled section as label/value pairs
- Show content preview (first 100 chars of first section) in subtitle

### Step 5.3: Template instance status transitions

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- Add status transition buttons inside ExpandableCard for template instances:
  - `draft -> active`: "Activate" button
  - `active -> completed`: "Mark Complete" button
- Call `PUT /api/template-instances/{id}` with `{"status": "active", "content": {...}}`
- Backend endpoint already exists and validates status transitions

### Step 5.4: Template instance edit view

**File**: `web/src/components/investigation/template-editor.tsx`
- **Add optional prop**: `existingInstance?: TemplateInstance`
- When provided, pre-fill sectionContents from `existingInstance.content`
- Change save button to "Update Instance" and call `PUT /api/template-instances/{id}`
- Title changes from template name to "Edit: [template name]"

### Testing Strategy (WP-5)
- Unit: Template card selection, instance status transitions
- Integration: Select template, fill sections, save, verify instance appears
- E2E: Full template lifecycle -- select -> fill -> save -> activate -> complete

---

## WP-6: Cross-Referencing & Pickers

### Gap Count: 5 gaps

### Step 6.1: Create EvidencePicker component

**File**: `web/src/components/investigation/evidence-picker.tsx` (NEW, ~120 lines)

**Props**:
```typescript
interface EvidencePickerProps {
  readonly caseId: string;
  readonly accessToken: string;
  readonly selectedIds: readonly string[];
  readonly onSelect: (ids: readonly string[]) => void;
  readonly label?: string;
  readonly maxItems?: number;
}
```

**ASCII wireframe**:
```
┌────────────────────────────────────────────┐
│ Evidence References                         │
│ ┌────────────────────────────────────────┐  │
│ │ 🔍 Search evidence...                 │  │
│ └────────────────────────────────────────┘  │
│ ┌────────────────────────────────────────┐  │
│ │ □ EV-2026-001 — Screenshot of post     │  │
│ │ ☑ EV-2026-003 — Satellite imagery      │  │
│ │ □ EV-2026-005 — Audio recording        │  │
│ │ ☑ EV-2026-007 — Witness statement      │  │
│ └────────────────────────────────────────┘  │
│ Selected: EV-2026-003, EV-2026-007          │
└────────────────────────────────────────────┘
```

**Implementation**:
- Fetch `GET /api/cases/{caseID}/evidence?current_only=true` on mount
- Client-side search filter on `evidence_number` and `title`
- Multi-select checkboxes
- Display selected items as badges below the dropdown
- Debounced search input

### Step 6.2: Create RecordPicker component

**File**: `web/src/components/investigation/record-picker.tsx` (NEW, ~100 lines)

**Props**:
```typescript
interface RecordPickerProps {
  readonly items: readonly { id: string; label: string; subtitle?: string }[];
  readonly selectedIds: readonly string[];
  readonly onSelect: (ids: readonly string[]) => void;
  readonly label: string;
}
```
- Generic picker for inquiry logs, assessments, verifications
- Same search/select pattern as EvidencePicker but without API fetch (items passed as prop)

### Step 6.3: Replace raw UUID inputs in AnalysisNoteEditor

**File**: `web/src/components/investigation/analysis-note-editor.tsx`
- **Change**: Replace the 4 comma-separated UUID text inputs with picker components:
  - "Evidence IDs" -> `<EvidencePicker caseId={caseId} ...>` (needs caseId prop addition)
  - "Inquiry IDs" -> `<RecordPicker items={inquiryLogs.map(l => ({id: l.id, label: l.objective}))} ...>`
  - "Assessment IDs" -> `<RecordPicker items={assessments.map(...)} ...>`
  - "Verification IDs" -> `<RecordPicker items={verifications.map(...)} ...>`
- **New props needed**: `inquiryLogs`, `assessments`, `verifications` arrays (or fetch internally)
- For simplicity, the AnalysisNoteEditor will accept a `caseId` prop and fetch these lists itself

### Step 6.4: Add evidence reference picker to ReportBuilder

**File**: `web/src/components/investigation/report-builder.tsx`
- **Add props**: `caseId: string` (already available from parent)
- **Add section** below "Transparency": `<EvidencePicker caseId={caseId} accessToken={accessToken} selectedIds={form.referencedEvidenceIds} onSelect={...} label="Referenced Evidence" />`
- Same for analysis references: `<RecordPicker items={analysisNotes} selectedIds={form.referencedAnalysisIds} ...>`
- Update form state to include `referencedEvidenceIds: string[]` and `referencedAnalysisIds: string[]`
- Send these in the POST body

### Step 6.5: Pass evidence items to CorroborationBuilder (already done in WP-1.3)

No additional work -- WP-1.3 fetches evidence items and passes them as props.

### Testing Strategy (WP-6)
- Unit: EvidencePicker renders items, search filters correctly, multi-select works
- Unit: RecordPicker same
- Integration: AnalysisNoteEditor saves with picked IDs instead of raw UUIDs
- E2E: Create analysis note with evidence picker, verify related_evidence_ids saved

---

## WP-7: Tab Polish & Navigation

### Gap Count: 5 gaps

### Step 7.1: Badge counts on sub-tabs

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change TABS rendering**: Compute counts from live data, show as badges

```typescript
const tabCounts: Record<TabKey, number> = {
  'inquiry-logs': liveInquiryLogs.length,
  assessments: assessments.length,
  verifications: verifications.length,
  corroborations: liveCorroborations.length,
  analysis: liveAnalysisNotes.length,
  templates: liveTemplateInstances.length,
  reports: liveReports.length,
  safety: liveSafetyProfiles.length,
};
```

- Render count badge after label text: `{tab.label} <span className="badge-count">{count}</span>`
- Only show badge when count > 0
- Badge styling: small rounded pill, subtle background

### Step 7.2: ExpandableCard for all record types

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change**: AssessmentsSection, VerificationsSection, CorroborationsSection, SafetySection -- replace flat `<div>` cards with `<ExpandableCard>`:

For **AssessmentsSection**:
```
<ExpandableCard
  title={`Relevance ${a.relevance_score}/5 · Reliability ${a.reliability_score}/5`}
  subtitle={`${a.source_credibility} · ${a.recommendation}`}
  badge={new Date(a.created_at).toLocaleDateString()}
>
  <div className="grid grid-cols-2 gap-[var(--space-md)]">
    <DetailField label="Relevance Rationale" value={a.relevance_rationale} full />
    <DetailField label="Reliability Rationale" value={a.reliability_rationale} full />
    <DetailField label="Source Credibility" value={a.source_credibility} />
    <DetailField label="Recommendation" value={a.recommendation} />
    {a.methodology && <DetailField label="Methodology" value={a.methodology} full />}
    {a.misleading_indicators.length > 0 && (
      <DetailField label="Misleading Indicators" value={a.misleading_indicators.join(', ')} full />
    )}
  </div>
</ExpandableCard>
```

Similar expansion for VerificationsSection, CorroborationsSection, SafetySection.

### Step 7.3: Mobile-responsive tab bar

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change nav element**: Add `overflow-x-auto` and `scrollbar-thin` classes
- Wrap in a container with `max-w-full` and `-mx-[var(--space-lg)] px-[var(--space-lg)]` for edge-to-edge scroll on mobile
- Add `whitespace-nowrap` to tab buttons
- Optional: add horizontal scroll shadow indicators

### Step 7.4: Phase status indicators

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change TABS**: Add a small colored dot before each tab label indicating phase status:
  - Empty (gray dot): no records
  - In progress (amber dot): has records but incomplete
  - Complete (green dot): phase has sufficient records
- Phase mapping:
  - inquiry-logs -> Phase 1 (Inquiry)
  - assessments -> Phase 2 (Assessment)
  - verifications -> Phase 5 (Verification)
  - corroborations -> Phase 5 (Verification)
  - analysis -> Phase 6 (Analysis)
  - templates -> Annex templates
  - reports -> Reporting
  - safety -> Security

### Testing Strategy (WP-7)
- Visual: Screenshot tests at mobile/desktop breakpoints
- Unit: Badge count computation
- Accessibility: Verify ExpandableCard has aria-expanded

---

## WP-8: State Management & Error Handling

### Gap Count: 6 gaps

### Step 8.1: Replace window.location.reload() with state refresh

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Create refresh functions** for each data type:

```typescript
const refreshInquiryLogs = useCallback(async () => {
  const API = process.env.NEXT_PUBLIC_API_URL || '';
  const res = await fetch(`${API}/api/cases/${caseId}/inquiry-logs`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (res.ok) {
    const json = await res.json();
    if (json?.data) setLiveInquiryLogs(json.data);
  }
}, [caseId, accessToken]);
```

- Create similar functions for: corroborations, analysisNotes, templates, templateInstances, reports, safetyProfiles
- Pass refresh function as `onSaved` callback instead of `window.location.reload()`
- For assessments/verifications (which are per-evidence), add fetch endpoints:
  - `GET /api/cases/{caseID}/assessments` -- needs backend (or aggregate from evidence list)

### Step 8.2: Error banner component

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add state**: `fetchError: string | null` at the top level
- **Change**: Wrap the `Promise.all()` catch handler to set `fetchError`
- Display error banner above tabs when `fetchError` is set:
```
{fetchError && (
  <div className="banner-error mb-[var(--space-md)]">
    Failed to load investigation data. <button onClick={retry}>Retry</button>
  </div>
)}
```

### Step 8.3: Loading states

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add state**: `loading: boolean` (true during initial fetch)
- Show a loading skeleton above tabs during fetch:
```
{loading && (
  <div className="space-y-[var(--space-sm)]">
    <div className="skeleton h-8 w-full rounded" />
    <div className="skeleton h-32 w-full rounded" />
  </div>
)}
```

**File**: `web/src/app/globals.css`
- **Add**: `.skeleton` class with pulse animation:
```css
.skeleton {
  background: linear-gradient(90deg, var(--bg-inset) 25%, var(--border-subtle) 50%, var(--bg-inset) 75%);
  background-size: 200% 100%;
  animation: skeleton-pulse 1.5s ease-in-out infinite;
}
@keyframes skeleton-pulse {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
```

### Step 8.4: Confirmation dialogs before destructive actions

**File**: `web/src/components/ui/confirm-dialog.tsx` (NEW, ~60 lines)

**Props**:
```typescript
interface ConfirmDialogProps {
  readonly title: string;
  readonly message: string;
  readonly confirmLabel: string;
  readonly variant: 'danger' | 'warning';
  readonly onConfirm: () => void;
  readonly onCancel: () => void;
}
```

- Modal overlay with confirm/cancel buttons
- Use in InquiryLogForm Reset button, any future Delete actions
- Replace `window.confirm()` calls in case-detail.tsx (separate PR)

### Step 8.5: Dirty form detection

**File**: Create custom hook `web/src/hooks/use-dirty-form.ts` (NEW, ~30 lines)

```typescript
export function useDirtyForm<T>(initialState: T, currentState: T): boolean {
  return JSON.stringify(initialState) !== JSON.stringify(currentState);
}
```

- Use `beforeunload` event listener when form is dirty
- Apply to ReportBuilder and TemplateEditor (longest forms)

### Step 8.6: Auto-save for report builder and template editor

**File**: Create custom hook `web/src/hooks/use-auto-save.ts` (NEW, ~40 lines)

```typescript
export function useAutoSave(
  data: unknown,
  saveFn: () => Promise<void>,
  delayMs: number = 3000,
  enabled: boolean = true
): { lastSavedAt: Date | null; saving: boolean }
```

- Debounced auto-save using `useEffect` + `setTimeout`
- Show "Saved at HH:MM" indicator in the UI
- Only enabled for drafts (not published/approved reports)
- Requires the report edit endpoint from WP-3.3

### Testing Strategy (WP-8)
- Unit: useDirtyForm hook with various state changes
- Unit: useAutoSave fires after delay, does not fire during debounce
- Integration: Verify no page reload after save, data refreshes in-place
- E2E: Create inquiry log, verify it appears in list without page reload

---

## WP-9: Search, Filter & Sort

### Gap Count: 5 gaps

### Step 9.1: Search input at top of investigation workspace

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add**: Search input above the tab bar (below dashboard if present)
- Client-side filter across ALL record types in the active tab
- Search matches against: title, objective, claim_summary, content, notes fields
- Uses `useDebounce` hook (300ms)

**ASCII wireframe**:
```
┌────────────────────────────────────────────────────┐
│ ┌──────────────────────────────────────────────┐    │
│ │ 🔍 Search investigation records...            │    │
│ └──────────────────────────────────────────────┘    │
│                                                      │
│ Inquiry Logs │ Assessments │ Verifications │ ...     │
└────────────────────────────────────────────────────┘
```

### Step 9.2: Filter dropdowns per section

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add filter bar** to each section (below SectionHeader):
  - Inquiry Logs: filter by search_tool
  - Analysis Notes: filter by analysis_type, status
  - Reports: filter by report_type, status
  - Verifications: filter by verification_type, finding
  - Corroborations: filter by claim_type, strength
- Each filter is a `<select>` with "All" as default
- Client-side filtering on the live data arrays

### Step 9.3: Sort toggles

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add**: Sort button (toggle newest/oldest) to SectionHeader component
- Default: newest first (already the case from API response)
- Sort by `created_at` ascending/descending
- For assessments: additional sort by relevance_score, reliability_score

### Step 9.4: Pagination controls for long lists

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Add**: Simple pagination (20 items per page) for sections with many records
- Show "Showing 1-20 of 45" text and Previous/Next buttons
- Client-side pagination on the filtered/sorted array
- Only show pagination when items > 20

### Testing Strategy (WP-9)
- Unit: Search filter matches correct records
- Unit: Sort order toggles correctly
- Integration: Filter by status, verify correct subset shown

---

## WP-10: Notification Integration

### Gap Count: 5 gaps

### Step 10.1: Add investigation notification event types

**File**: `internal/notifications/models.go`
- **Add constants**:
```go
EventEvidenceNeedsVerification = "evidence_needs_verification"
EventReportSubmittedForReview  = "report_submitted_for_review"
EventAnalysisNoteSuperseded    = "analysis_note_superseded"
EventSafetyProfileUpdated      = "safety_profile_updated"
EventReportPublished           = "report_published"
```

### Step 10.2: Emit notifications from investigation service

**File**: `internal/investigation/service.go`
- **Add**: `notifier` field on Service struct (interface with `Notify(ctx, event)`)
- **Add**: `WithNotifier(notifier) *Service` functional option
- **Emit on**:
  - `CreateAssessment` -> if finding is not "authentic" or "likely_authentic", emit `EventEvidenceNeedsVerification`
  - `TransitionReportStatus` to `in_review` -> emit `EventReportSubmittedForReview`
  - `UpdateAnalysisNote` with `superseded` status -> emit `EventAnalysisNoteSuperseded`
  - `UpsertSafetyProfile` -> emit `EventSafetyProfileUpdated`
  - `PublishReport` -> emit `EventReportPublished`

### Step 10.3: Add notification routing rules

**File**: `internal/notifications/service.go`
- **Add cases** to `resolveRecipients()` for the new event types:
  - `EventReportSubmittedForReview` -> all prosecutors/judges on the case
  - `EventEvidenceNeedsVerification` -> all investigators on the case
  - `EventAnalysisNoteSuperseded` -> original author
  - `EventSafetyProfileUpdated` -> target user
  - `EventReportPublished` -> all case members

### Step 10.4: Wire notifier into investigation service

**File**: `internal/app/wire.go` (or equivalent wiring file)
- **Change**: Call `investigationService.WithNotifier(notificationAdapter)` during app initialization

### Testing Strategy (WP-10)
- Unit: Each service method emits correct event type
- Unit: Recipient resolution for each event type
- Integration: Create assessment, verify notification created for case investigators

---

## WP-11: Export & Legal Packaging

### Gap Count: 4 gaps

### Step 11.1: Add investigation data to case export

**File**: `internal/cases/export.go`
- **Add interface**: `InvestigationExporter` with methods:
```go
type InvestigationExporter interface {
  ListInquiryLogs(ctx context.Context, caseID uuid.UUID) ([]investigation.InquiryLog, error)
  ListAnalysisNotes(ctx context.Context, caseID uuid.UUID) ([]investigation.AnalysisNote, error)
  ListReports(ctx context.Context, caseID uuid.UUID) ([]investigation.InvestigationReport, error)
  ListCorroborationClaims(ctx context.Context, caseID uuid.UUID) ([]investigation.CorroborationClaim, error)
}
```
- **Note**: Assessments and verifications are per-evidence, so export needs a bulk list-by-case query

**File**: `internal/investigation/repository.go`
- **Add**: `ListAssessmentsByCase(ctx, caseID) ([]EvidenceAssessment, error)`
- **Add**: `ListVerificationsByCase(ctx, caseID) ([]VerificationRecord, error)`

**File**: `internal/investigation/pg_repository.go`
- **Implement** the two new repository methods with simple `WHERE case_id = $1` queries

### Step 11.2: Generate CSV files for each investigation record type

**File**: `internal/cases/export.go`
- **Add function**: `writeInquiryLogsCSV(zc zipCreator, logs []investigation.InquiryLog) error`
  - Columns: id, objective, search_strategy, search_tool, search_url, search_started_at, search_ended_at, results_count, results_relevant, results_collected, keywords, notes
- **Add function**: `writeAssessmentsCSV(zc zipCreator, assessments []investigation.EvidenceAssessment) error`
  - Columns: id, evidence_id, relevance_score, reliability_score, source_credibility, recommendation, methodology, created_at
- **Add function**: `writeVerificationsCSV(zc zipCreator, records []investigation.VerificationRecord) error`
  - Columns: id, evidence_id, verification_type, finding, confidence_level, methodology, tools_used, created_at
- **Add function**: `writeCorroborationsCSV(zc zipCreator, claims []investigation.CorroborationClaim) error`
  - Columns: id, claim_summary, claim_type, strength, evidence_count, created_at
- **Add function**: `writeAnalysisNotesCSV(zc zipCreator, notes []investigation.AnalysisNote) error`
  - Columns: id, title, analysis_type, status, content_length, methodology, created_at

All CSVs go into an `investigation/` directory within the ZIP.

### Step 11.3: Export reports as JSON

**File**: `internal/cases/export.go`
- **Add function**: `writeReportsJSON(zc zipCreator, reports []investigation.InvestigationReport) error`
- Write each report as `investigation/reports/{report_id}.json` in the ZIP
- Full JSON including sections, limitations, caveats, assumptions

### Step 11.4: Update export README.txt

**File**: `internal/cases/export.go`
- **Update** the README template to include:
```
investigation/
  inquiry_logs.csv        - Online inquiry search logs
  assessments.csv         - Evidence assessments
  verifications.csv       - Verification records
  corroborations.csv      - Corroboration claims
  analysis_notes.csv      - Analysis notes
  reports/                - Investigation reports (JSON)
    {report_id}.json
```

### Testing Strategy (WP-11)
- Unit: CSV generation for each record type with edge cases (empty arrays, null fields)
- Integration: Full export ZIP with investigation data, verify all files present
- Existing export tests should continue passing (backward compatible)

---

## WP-12: Accessibility & i18n

### Gap Count: 5 gaps

### Step 12.1: Add htmlFor/id pairs on all form inputs

**Files** to audit and fix:
- `web/src/components/investigation/inquiry-log-form.tsx` -- most labels missing `htmlFor`/`id`
- `web/src/components/investigation/corroboration-builder.tsx` -- missing
- `web/src/components/investigation/verification-form.tsx` -- missing
- `web/src/components/investigation/assessment-form.tsx` -- missing

**Pattern**: Every `<label className="field-label">` needs `htmlFor="unique-id"` and corresponding `id` on the input.

Naming convention: `{form-prefix}-{field-name}`, e.g., `ilf-objective`, `af-relevance-score`, `vf-methodology`

### Step 12.2: aria-expanded on ExpandableCard

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Change ExpandableCard**: Add `aria-expanded={expanded}` to the button element
- Add `role="region"` and `aria-labelledby` to the expanded content div
- Add unique `id` to the card title for `aria-labelledby` reference

### Step 12.3: Keyboard navigation for score dots

**File**: `web/src/components/investigation/assessment-form.tsx`
- **Change ScoreDots**: Add keyboard handling:
  - ArrowRight/ArrowDown: increment score
  - ArrowLeft/ArrowUp: decrement score
  - Home: score = 1, End: score = 5
- Add `tabIndex={0}` to the container, use `onKeyDown`
- Already has `role="radiogroup"` and `role="radio"` (good)

### Step 12.4: i18n translation keys

**File**: `web/src/messages/en.json` (or equivalent i18n config)
- **Add investigation namespace** with keys for all labels:
```json
{
  "investigation": {
    "tabs": {
      "inquiry_logs": "Inquiry Logs",
      "assessments": "Assessments",
      ...
    },
    "forms": {
      "objective": "Objective",
      "search_strategy": "Search Strategy",
      ...
    },
    "status": {
      "draft": "Draft",
      "in_review": "In Review",
      ...
    }
  }
}
```
- Replace all hardcoded strings in investigation components with `t('investigation.tabs.inquiry_logs')`
- **Scope**: Investigation components only (do not touch other components in this PR)

### Step 12.5: Screen reader announcements for form save/error

**File**: `web/src/components/ui/announcer.tsx` (NEW, ~30 lines)
- Create a visually hidden ARIA live region component
- Use `aria-live="polite"` for success, `aria-live="assertive"` for errors
- Import and use in all form components after save/error

### Testing Strategy (WP-12)
- Accessibility audit: axe-core scan of all investigation pages
- Manual: Tab through all forms, verify focus order
- Manual: Screen reader test of form save/error announcements
- i18n: Verify all strings render from translation keys

---

## WP-13: Visual Design Consistency

### Gap Count: 5 gaps

### Step 13.1: Consistent StatusBadge component

**File**: `web/src/components/ui/status-badge.tsx` (NEW, ~50 lines)

**Props**:
```typescript
interface StatusBadgeProps {
  readonly status: string;
  readonly variant?: 'report' | 'analysis' | 'template' | 'verification';
}
```

**Color mapping**:
```
draft:      --status-archived / --status-archived-bg
in_review:  --amber-accent / --amber-subtle
approved:   --status-active / --status-active-bg
published:  --status-active / --status-active-bg (brighter)
withdrawn:  --status-hold / --status-hold-bg
superseded: --status-hold / --status-hold-bg
authentic:  --status-active / --status-active-bg
manipulated: --status-hold / --status-hold-bg
```

Replace all inline status styling across investigation components with `<StatusBadge>`.

### Step 13.2: Phase color coding

**File**: `web/src/app/globals.css`
- **Add**: Phase-specific subtle background colors:
```css
.phase-inquiry    { background-color: oklch(0.965 0.015 220); }
.phase-assessment { background-color: oklch(0.965 0.015 70); }
.phase-collection { background-color: oklch(0.965 0.015 145); }
.phase-preservation { background-color: oklch(0.965 0.015 280); }
.phase-verification { background-color: oklch(0.965 0.015 40); }
.phase-analysis   { background-color: oklch(0.965 0.015 340); }
```
- Apply as subtle border-left accent on section headers (2px left border)

### Step 13.3: Loading skeleton components

**File**: `web/src/components/ui/skeleton.tsx` (NEW, ~40 lines)

```typescript
export function Skeleton({ className, ...props }: { className?: string }) {
  return <div className={`skeleton rounded ${className || ''}`} {...props} />;
}

export function CardSkeleton() { /* 3 lines of skeleton */ }
export function TableRowSkeleton() { /* table row shape */ }
```

Uses the `.skeleton` CSS from WP-8.3.

### Step 13.4: Consistent empty state design

**File**: `web/src/components/investigation/investigation-page-client.tsx`
- **Enhance EmptyState component**: Add icon and action button

```typescript
function EmptyState({
  message,
  icon,
  actionLabel,
  onAction,
}: {
  readonly message: string;
  readonly icon?: 'search' | 'file' | 'shield' | 'chart';
  readonly actionLabel?: string;
  readonly onAction?: () => void;
}) {
```

**ASCII wireframe**:
```
┌──────────────────────────────────────┐
│                                      │
│          ┌────────────┐              │
│          │     📋      │              │
│          └────────────┘              │
│                                      │
│   No inquiry logs recorded yet.      │
│                                      │
│        [Create First Log]            │
│                                      │
└──────────────────────────────────────┘
```

### Step 13.5: Typography hierarchy review

**File**: All investigation components
- **Audit and normalize**:
  - Section titles: `font-[family-name:var(--font-heading)] text-lg`
  - Card titles: `text-sm font-medium`
  - Field labels: `field-label` class (already consistent)
  - Body text: `text-sm` with `--text-secondary`
  - Mono values: `font-[family-name:var(--font-mono)] text-xs`
- Ensure no component uses raw `font-size` or `font-weight` values
- All investigation components should use design system classes exclusively

### Testing Strategy (WP-13)
- Visual: Screenshot regression tests at light/dark themes
- Manual: Review all empty states, loading states, status badges
- Accessibility: Contrast ratio check on all badge color combinations

---

## Implementation Order

### Phase 1: Foundation (WP-1, WP-8 steps 8.1-8.3)
**Estimated**: 3-4 hours
- Wire all forms (immediate user value)
- Replace window.location.reload() (critical UX fix)
- Add loading states and error handling
- **Deliverable**: All 8 investigation sections are functional

### Phase 2: Core Experience (WP-2, WP-3, WP-7)
**Estimated**: 5-6 hours
- Investigation dashboard
- Report workflow (status transitions, edit, preview)
- Tab polish (badges, expandable cards, mobile)
- **Deliverable**: Investigation workspace feels complete and professional

### Phase 3: Evidence Integration (WP-4, WP-6)
**Estimated**: 4-5 hours
- Evidence-investigation bridge (assess/verify on evidence detail)
- Cross-referencing pickers (replace raw UUID inputs)
- **Deliverable**: Evidence and investigation are seamlessly connected

### Phase 4: Templates & Navigation (WP-5, WP-9)
**Estimated**: 3-4 hours
- Template workflow (picker, instance lifecycle)
- Search, filter, sort
- **Deliverable**: Full template lifecycle, easy to find records

### Phase 5: Integration & Polish (WP-10, WP-11, WP-12, WP-13)
**Estimated**: 5-6 hours
- Notification integration (backend only, minimal frontend)
- Export augmentation (CSV/JSON in ZIP)
- Accessibility fixes
- Visual design consistency
- **Deliverable**: Production-ready investigation workspace

---

## Backend Changes Summary

| Endpoint | Method | Status | WP |
|----------|--------|--------|-----|
| `/api/reports/{id}/status` | POST | **NEW** | WP-3 |
| `/api/reports/{id}` | PUT | **NEW** | WP-3 |
| `/api/cases/{caseID}/assessments` | GET | **NEW** (list all case assessments) | WP-11 |
| `/api/cases/{caseID}/verifications` | GET | **NEW** (list all case verifications) | WP-11 |

All other endpoints already exist and are functional.

---

## New Files Summary

| File | Lines | WP |
|------|-------|----|
| `web/src/components/investigation/investigation-dashboard.tsx` | ~200 | WP-2 |
| `web/src/components/investigation/report-preview.tsx` | ~150 | WP-3 |
| `web/src/components/investigation/evidence-picker.tsx` | ~120 | WP-6 |
| `web/src/components/investigation/record-picker.tsx` | ~100 | WP-6 |
| `web/src/components/investigation/assessment-summary-card.tsx` | ~80 | WP-4 |
| `web/src/components/ui/confirm-dialog.tsx` | ~60 | WP-8 |
| `web/src/components/ui/status-badge.tsx` | ~50 | WP-13 |
| `web/src/components/ui/skeleton.tsx` | ~40 | WP-13 |
| `web/src/components/ui/announcer.tsx` | ~30 | WP-12 |
| `web/src/hooks/use-dirty-form.ts` | ~30 | WP-8 |
| `web/src/hooks/use-auto-save.ts` | ~40 | WP-8 |

---

## Risks & Mitigations

- **Risk**: Report status transition endpoint does not exist yet
  - Mitigation: Simple endpoint, follows existing PublishReport pattern. Use `repo.UpdateReport()` which already exists.

- **Risk**: Evidence-investigation integration adds significant weight to evidence-detail.tsx (already large)
  - Mitigation: Extract investigation section into `web/src/components/evidence/evidence-investigation-panel.tsx` sub-component (~150 lines)

- **Risk**: Client-side filtering for "unassessed evidence" is O(n*m) and may be slow for large cases
  - Mitigation: For MVP, limit to first 200 evidence items. Add backend query param in follow-up.

- **Risk**: Auto-save conflicts if two users edit same report
  - Mitigation: Auto-save only for draft status. Add `updated_at` optimistic locking check on save.

- **Risk**: ExpandableCard refactor touches 4 sections -- risk of breaking existing display
  - Mitigation: ExpandableCard is already proven (used in inquiry logs, analysis, reports). Apply same pattern.

---

## Success Criteria

- [ ] All 8 investigation sections render functional forms (no placeholder text)
- [ ] Investigation dashboard shows accurate counts and phase progress
- [ ] Reports can transition through full lifecycle (draft -> published)
- [ ] Evidence detail page shows assess/verify buttons and existing records
- [ ] Templates can be selected, filled, and progressed through statuses
- [ ] No raw UUID inputs remain -- all replaced with searchable pickers
- [ ] All sub-tabs show record counts
- [ ] All record types use ExpandableCard with consistent design
- [ ] No window.location.reload() calls remain
- [ ] Error banners display for failed fetches
- [ ] Loading skeletons show during data fetch
- [ ] Case export ZIP includes investigation CSVs and report JSON files
- [ ] All form inputs have htmlFor/id pairs
- [ ] ExpandableCard has aria-expanded
- [ ] Score dots are keyboard navigable
- [ ] Notification events fire for key investigation actions

---

## Key File Paths

All paths are absolute from project root `/Users/kylefuhri/development/Personal/VaultKeeper/`:

**Modified files**:
- `web/src/components/investigation/investigation-page-client.tsx` -- primary file, most changes
- `web/src/components/investigation/report-builder.tsx` -- edit mode, evidence picker
- `web/src/components/investigation/template-editor.tsx` -- edit mode for instances
- `web/src/components/investigation/analysis-note-editor.tsx` -- replace UUID inputs with pickers
- `web/src/components/investigation/assessment-form.tsx` -- keyboard nav for score dots
- `web/src/components/evidence/evidence-detail.tsx` -- investigation integration panel
- `web/src/components/cases/case-detail.tsx` -- pass userId prop
- `web/src/app/[locale]/(app)/cases/[id]/page.tsx` -- pass userId from session
- `web/src/app/globals.css` -- skeleton animation, phase colors
- `internal/investigation/handler.go` -- report status transition, report update endpoints
- `internal/investigation/service.go` -- TransitionReportStatus, notifier integration
- `internal/investigation/pg_repository.go` -- ListAssessmentsByCase, ListVerificationsByCase
- `internal/investigation/repository.go` -- new repository interface methods
- `internal/cases/export.go` -- investigation CSV/JSON export
- `internal/notifications/models.go` -- new event types

**New files**:
- `web/src/components/investigation/investigation-dashboard.tsx`
- `web/src/components/investigation/report-preview.tsx`
- `web/src/components/investigation/evidence-picker.tsx`
- `web/src/components/investigation/record-picker.tsx`
- `web/src/components/investigation/assessment-summary-card.tsx`
- `web/src/components/ui/confirm-dialog.tsx`
- `web/src/components/ui/status-badge.tsx`
- `web/src/components/ui/skeleton.tsx`
- `web/src/components/ui/announcer.tsx`
- `web/src/hooks/use-dirty-form.ts`
- `web/src/hooks/use-auto-save.ts`

---

## WP-14: Missing Gap Coverage (23 Remaining Gaps)

### 14.1 Batch Operations (D-03)
**Gap**: No batch assessment/verification operations on evidence.
**Solution**: Add checkbox selection to evidence grid rows. When items selected, show floating action bar with "Assess Selected" and "Verify Selected" buttons. Opens a batch form that applies the same assessment/verification to all selected items.
**Files**: `web/src/components/evidence/evidence-grid.tsx` (add checkboxes + selection state), new `web/src/components/investigation/batch-action-bar.tsx`
**API**: Multiple sequential POST calls to `/api/evidence/{id}/assessments` and `/api/evidence/{id}/verifications`

### 14.2 Report PDF Export (E-03)
**Gap**: No PDF export for reports.
**Solution**: Add "Export PDF" button on report detail/preview. Use browser print API with a print-optimized stylesheet. For server-side PDF, add `GET /api/reports/{id}/export?format=pdf` endpoint using Go HTML-to-PDF library.
**Files**: `web/src/components/investigation/report-preview.tsx` (add print button + print CSS), `internal/investigation/handler.go` (export endpoint)
**Approach**: Phase 1: browser `window.print()` with `@media print` stylesheet. Phase 2: server-side PDF generation.

### 14.3 Claim Editing — Implement 501 Endpoints (F-03)
**Gap**: AddEvidenceToClaim and RemoveEvidenceFromClaim return 501.
**Solution**: Implement the handler methods. Add "Add Evidence" button and "Remove" (X) button on claim detail view.
**Files**: `internal/investigation/handler.go` (replace 501 stubs with real implementations), `web/src/components/investigation/investigation-page-client.tsx` (claim detail with add/remove UI)
**API**: `POST /api/corroborations/{id}/evidence`, `DELETE /api/corroborations/{id}/evidence/{evidenceID}`

### 14.4 Corroboration Visualization (F-02)
**Gap**: No visual map of how evidence relates to claims.
**Solution**: Add a "Corroboration Map" view showing claims as nodes with evidence items linked. Use a simple CSS grid layout (not a full graph library). Claims as cards, evidence as connected badges.
**Files**: New `web/src/components/investigation/corroboration-map.tsx`
**Approach**: Simple 2-column layout: claims on left, evidence on right, with connecting lines via CSS borders. Toggle between list and map views.

### 14.5 Reviewer Sign-Off UI (G-02)
**Gap**: Verification records have reviewer fields but no UI to assign or approve.
**Solution**: Add "Assign Reviewer" dropdown and "Approve" / "Reject" buttons on verification record detail. New backend endpoint `PUT /api/verifications/{id}/review`.
**Files**: `internal/investigation/handler.go` (new review endpoint), `internal/investigation/service.go` (ReviewVerificationRecord method), `web/src/components/investigation/investigation-page-client.tsx` (reviewer buttons on verification cards)
**Schema**: Uses existing `reviewer`, `reviewer_approved`, `reviewer_notes`, `reviewed_at` columns.

### 14.6 Self-Verification Warning (G-03)
**Gap**: Backend prevents self-verification but UI doesn't warn until submission fails.
**Solution**: On evidence detail, check if current user is the uploader. If so, show "You uploaded this evidence — a different reviewer must verify it" message and disable the Verify button.
**Files**: `web/src/components/evidence/evidence-detail.tsx` (compare session user with evidence.uploaded_by)

### 14.7 Claim Detail View (F-04)
**Gap**: Corroboration claims don't show linked evidence items or their roles.
**Solution**: Use ExpandableCard for claims. Expanded view shows: claim summary, type, strength, analysis notes, and a table of linked evidence (evidence number, title, role badge: primary/supporting/contextual/contradicting).
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (CorroborationsSection)

### 14.8 Contradicting Evidence Highlighting (F-05)
**Gap**: Evidence with role "contradicting" is not visually distinct.
**Solution**: In claim detail view, render contradicting evidence with a red-tinted background and a warning icon. Add a "Contradictions" count badge on the claim card header.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (CorroborationsSection ExpandableCard)

### 14.9 Verification Record Detail View (G-04)
**Gap**: Verifications show type/finding/confidence but not methodology, tools, sources, limitations.
**Solution**: Apply ExpandableCard to verification records. Expanded view shows all fields in a detail grid.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (VerificationsSection)

### 14.10 OPSEC Warnings (H-02)
**Gap**: Safety profile warnings not surfaced to users.
**Solution**: When SafetyProfileForm saves and the API returns warnings (e.g., "pseudonym recommended"), display them as amber banners below the form. Also show persistent OPSEC reminder at top of investigation page when the user's safety profile has elevated/high_risk opsec level.
**Files**: `web/src/components/investigation/safety-profile-form.tsx` (display warnings), `web/src/components/investigation/investigation-page-client.tsx` (OPSEC reminder banner)

### 14.11 Investigator Self-Service Safety View (H-03)
**Gap**: Investigators can't see their own safety profile.
**Solution**: Add "My Safety Profile" card at the top of the Safety tab when the current user has a profile. Use `GET /api/cases/{caseID}/safety-profiles/mine` endpoint.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (SafetySection adds a "Your Profile" card using client-side fetch)

### 14.12 Pseudonym in Audit Trails (H-04)
**Gap**: Pseudonym not used when use_pseudonym is true.
**Solution**: Backend: when recording custody events for a user with `use_pseudonym=true`, resolve the pseudonym and use it as the actor display name. Frontend: custody chain and activity displays use pseudonym when available.
**Files**: `internal/investigation/service.go` (lookup safety profile, use pseudonym in custody event detail), `web/src/components/evidence/evidence-detail.tsx` (custody log displays pseudonym)
**Note**: This requires the service to lookup safety profiles before recording events — adds a dependency.

### 14.13 Investigation Timeline (I-03)
**Gap**: No chronological view across all investigation activity.
**Solution**: New "Timeline" sub-tab (or toggle on dashboard) showing all investigation events sorted by date. Combine inquiry logs, assessments, verifications, claims, analysis notes, reports into a unified feed with type icons and timestamps.
**Files**: New `web/src/components/investigation/investigation-timeline.tsx`, add as optional view toggle in investigation dashboard
**API**: Fetch all record types, merge client-side, sort by created_at

### 14.14 Assignment & Delegation (J-02)
**Gap**: Can't assign evidence to specific investigators for assessment/verification.
**Solution**: Add "Assign to" dropdown on evidence items and investigation records. New `assigned_to` column on key tables (deferred — use existing `performed_by`/`assessed_by` fields as implicit assignment for now). Phase 1: show who created each record. Phase 2: add explicit assignment.
**Files**: Phase 1 is display-only — show author names on all cards.
**Note**: Full assignment system deferred to future sprint; Phase 1 covers the gap partially.

### 14.15 Collaborative Editing Indicators (J-03)
**Gap**: No presence awareness when multiple users work on same case.
**Solution**: The existing WebSocket infrastructure (Yjs + y-websocket for redaction) can be extended. Phase 1: show "X users viewing this case" indicator in the case header. Phase 2: per-record locking.
**Files**: `web/src/components/cases/case-detail.tsx` (presence indicator)
**Note**: Full collaborative editing deferred; presence indicator is Phase 1.

### 14.16 Audit Trail in UI (K-01)
**Gap**: Investigation custody events not shown in UI.
**Solution**: Add "Activity" tab to investigation workspace showing custody events filtered to investigation actions (inquiry_log_created, assessment_created, verification_record_created, etc.). Reuse existing custody log display pattern from evidence detail.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (add Activity tab), fetch from `/api/cases/{caseID}/custody-log` (new case-level endpoint needed)
**Backend**: Add `GET /api/cases/{caseID}/investigation-activity` endpoint that filters custody_log by investigation action types.

### 14.17 Unified Activity Feed (K-02)
**Gap**: No single view of all case activity.
**Solution**: Merged into K-01 Activity tab — shows all investigation events in chronological order. The timeline (I-03) also covers this from a different angle.

### 14.18 Offline/Network Failure Handling (N-05)
**Gap**: No indication when network unavailable, no retry.
**Solution**: Add `navigator.onLine` check. Show persistent "Offline — changes will not be saved" banner when offline. Queue form submissions and retry when connection restored (using localStorage).
**Files**: New `web/src/components/common/offline-banner.tsx`, wrap investigation forms with offline detection

### 14.19 Template Instance Approval (C-04)
**Gap**: No approval workflow for template instances.
**Solution**: Add "Submit for Approval" and "Approve" buttons on template instance detail view. Use existing `approved_by`/`approved_at` columns. Transition: draft → active (submitted) → completed (approved).
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (TemplatesSection detail view)
**API**: `PUT /api/template-instances/{id}` with status change

### 14.20 Template Instance Export (C-06)
**Gap**: Can't export filled template to PDF.
**Solution**: Add "Print" button on template instance detail. Use `@media print` stylesheet for clean output.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (print button on template detail)

### 14.21 Digital Signatures on Reports (E-06)
**Gap**: No signature mechanism for court submission.
**Solution**: Phase 1: Add "Sign" button that records a cryptographic hash of the report content + signer's user ID + timestamp, stored in a new `report_signatures` JSONB field. Phase 2: integrate with document signing service.
**Files**: `internal/investigation/report.go` (add Signatures field), migration to add column, handler endpoint
**Note**: Full PKI signing deferred; hash-based attestation is Phase 1.

### 14.22 Dense Mobile Layouts (O-05)
**Gap**: Multi-column form grids collapse poorly on mobile.
**Solution**: Review all form grids. Ensure `grid-cols-2` becomes `grid-cols-1` on `sm:` breakpoint. Add `sm:` responsive prefixes throughout investigation forms.
**Files**: All 8 form components in `web/src/components/investigation/`

### 14.23 Nested Tab Bar Confusion (P-02)
**Gap**: Two rows of tabs on mobile is confusing.
**Solution**: On mobile (< 768px), collapse the investigation sub-tabs into a dropdown select instead of horizontal tabs. Show current section as a select value.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (responsive tab rendering)

### 14.24 Delete Operations (Q-01)
**Gap**: DeleteInquiryLog and other deletes return 501.
**Solution**: Implement delete handlers with confirmation dialogs in UI. Soft-delete pattern (mark as deleted, don't remove from DB) to preserve audit trail.
**Files**: `internal/investigation/handler.go`, `internal/investigation/service.go`, `internal/investigation/pg_repository.go`

### 14.25 Rich Text in Reports (Q-04)
**Gap**: Report sections are plain text only.
**Solution**: Phase 1: Support Markdown in section content with a preview toggle. Phase 2: integrate a lightweight rich text editor (e.g., Tiptap/ProseMirror).
**Files**: `web/src/components/investigation/report-builder.tsx` (add Markdown preview toggle)

### 14.26 Analysis Note Version History (Q-05)
**Gap**: No UI for supersession chain.
**Solution**: When viewing a superseded note, show "This note has been superseded by [newer note]" banner with link. When viewing the superseding note, show "This supersedes [older note]" link. Add "Supersede" button that creates a new note pre-filled from the current one.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (AnalysisSection ExpandableCard), `internal/investigation/handler.go` (supersede endpoint)

### 14.27 Template Instance Duplication (C-05)
**Gap**: Can't duplicate a template instance for reuse.
**Solution**: Add "Duplicate" button on template instance detail. Creates a new draft instance with the same content.
**Files**: `web/src/components/investigation/investigation-page-client.tsx`

### 14.28 Safety Briefing Expiry (H-05)
**Gap**: No renewal mechanism for safety briefings.
**Solution**: Show amber "Briefing expired" badge when safety_briefing_date is > 90 days ago. Show in dashboard "Needs Attention" queue.
**Files**: `web/src/components/investigation/investigation-page-client.tsx` (SafetySection + dashboard)

### 14.29 Skip-to-Content & Landmarks (M-04)
**Gap**: No ARIA landmarks or skip navigation.
**Solution**: Add `role="main"`, `role="navigation"`, `aria-label` to investigation sections. Add skip-to-content link.
**Files**: `web/src/components/investigation/investigation-page-client.tsx`

### 14.30 Sticky Form Actions (P-03)
**Gap**: Save button not visible on long forms without scrolling.
**Solution**: Make form action bar `sticky bottom-0` with a subtle top border and background blur.
**Files**: All form components — wrap action buttons in sticky container

---

## Updated Gap Coverage

| Severity | Total | Covered | Missing |
|----------|-------|---------|---------|
| CRITICAL | 9 | 9 | 0 |
| HIGH | 26 | 26 | 0 |
| MEDIUM | 21 | 21 | 0 |
| LOW | 4 | 4 | 0 |
| **TOTAL** | **60** | **60** | **0** |

All 60 gaps now have implementation plans.


---

## WP-0: Pre-Execution Fixes (Must Do Before Any Other WP)

### 0.1 Fix Tab URL Parameter Collision — CRITICAL
**Bug**: `handleTabChange` in `investigation-page-client.tsx` sets `params.set('tab', tab)` which collides with the parent case page's `tab` param. Investigation sub-tab clicks break case-level navigation.
**Fix**: When embedded in case page, use `inv` parameter: `params.set('inv', tab)`. The component already reads from `searchParams.get('inv')` (line 72) but the setter doesn't match.
**File**: `web/src/components/investigation/investigation-page-client.tsx` line 112

### 0.2 Fix Stale Assessment/Verification State — HIGH
**Bug**: Lines 142-146 pass `items={assessments}` and `items={verifications}` (original props) instead of live state variables.
**Fix**: Add `liveAssessments`/`liveVerifications` state variables. Fetch them in useEffect (requires case-level endpoint — see 0.5). Until endpoint exists, keep as props but add TODO.
**File**: `web/src/components/investigation/investigation-page-client.tsx`

### 0.3 Pass userRole to InvestigationPageClient — HIGH
**Fix**: Add `userRole: string` to props. In case-detail.tsx, derive from session: `session.user.systemRole`. Pass through to InvestigationPageClient. Use role to conditionally show/hide buttons and tabs.
**Files**: `web/src/components/cases/case-detail.tsx`, `web/src/components/investigation/investigation-page-client.tsx`

### 0.4 Role-Based Tab Visibility — HIGH
**Fix**: Filter TABS array by role. Hide Safety tab for non-prosecutor/judge. Hide "New" buttons on sections where the user's role can't create records. Hide "Approve"/"Publish" buttons for non-authorized roles.
**File**: `web/src/components/investigation/investigation-page-client.tsx`

### 0.5 Create GET /api/cases/{caseID}/assessments Endpoint — HIGH
**Fix**: New handler + service method that lists all assessments for a case (not per-evidence). Required for dashboard counters, tab badge counts, and evidence grid filters.
**Files**: `internal/investigation/handler.go`, `internal/investigation/service.go`, `internal/investigation/pg_repository.go`
**SQL**: `SELECT * FROM evidence_assessments WHERE case_id = $1 ORDER BY created_at DESC`

### 0.6 Fix Score Display /10 → /5 — MEDIUM (live bug)
**Fix**: In `investigation-page-client.tsx` line 324, change `/10` to `/5` in the assessments display.
**File**: `web/src/components/investigation/investigation-page-client.tsx`

### 0.7 Clean Up Empty <p> Tags — LOW
**Fix**: Remove leftover empty `<p>` tags at lines ~503-504 and ~638-639.
**File**: `web/src/components/investigation/investigation-page-client.tsx`

---

## Additional Items for Existing WPs

### WP-1 Addition: 1.6 — Gate SafetyProfileForm Behind Role Check
**Gap**: SafetyProfileForm renders for all roles but save fails 403 for non-prosecutor/judge.
**Fix**: Only show "New Profile" button when `userRole` is prosecutor/judge. Show read-only "Your Profile" for investigators (via GET /mine endpoint).

### WP-3 Addition: 3.6 — Report Empty References Warning
**Gap**: Reports created with empty referenced_evidence_ids silently.
**Fix**: In report preview (WP-3.4), show amber warning: "No evidence items referenced in this report."

### WP-6 Addition: 6.5 — Inquiry Log Evidence Link Picker
**Gap**: DB column `evidence_id` on inquiry logs has no UI.
**Fix**: Add optional EvidencePicker to InquiryLogForm for linking a log to the evidence it discovered.

### WP-6 Addition: 6.6 — Missing Inquiry Log Form Fields
**Gap**: `search_operators` and `search_tool_version` DB columns have no form fields.
**Fix**: Add "Search Operators" text input and "Tool Version" text input to InquiryLogForm.

### WP-7 Addition: 7.6 — InquiryLogList Component Decision
**Gap**: `inquiry-log-list.tsx` (234 lines) exists but is dead code.
**Fix**: Evaluate and either integrate (for table view with pagination) or delete.

### WP-14 Addition: 14.31 — Assessment Review Workflow
**Gap**: DB has `reviewed_by`/`reviewed_at` on assessments but no UI/API.
**Fix**: Add "Assign Reviewer" and "Approve Assessment" UI. New endpoint `PUT /api/assessments/{id}/review`.

### WP-14 Addition: 14.32 — Analysis Note Review Workflow
**Gap**: DB has `reviewer_id`/`reviewed_at` on analysis notes but no UI/API.
**Fix**: Add "Submit for Review" and "Approve Note" buttons. Use existing UpdateAnalysisNote endpoint with status transition.

### WP-14 Addition: 14.33 — Template Instance Approval Backend Fix
**Gap**: `UpdateTemplateInstance` handler doesn't set `approved_by`/`approved_at` during status transitions.
**Fix**: When status changes to `completed`, set `approved_by` = actor and `approved_at` = now.
**File**: `internal/investigation/pg_repository.go` UpdateTemplateInstance method

### WP-14 Addition: 14.34 — Offline Storage Security Warning
**Gap**: WP-14.18 proposes localStorage for offline form queuing, which is legally risky for evidence.
**Fix**: Add prominent warning: "Data saved locally is not part of the official evidence chain. Submit as soon as connectivity is restored." Encrypt localStorage data. Add auto-purge after successful submission.

### WP-14 Addition: 14.35 — Report Builder Mutable Counter Fix
**Gap**: `report-builder.tsx` line 38 has module-level `let sectionCounter = 0` causing ID collisions.
**Fix**: Replace with `useRef(0)` or `crypto.randomUUID()`.

### WP-12 Addition: 12.5 — Verify French Translations Complete
**Gap**: fr.json investigation section may have incomplete translations.
**Fix**: Cross-reference every en.json investigation key exists in fr.json with French text (not English).

---

## FINAL Updated Gap Coverage

| Severity | Original Plan | New Finds | Total Covered |
|----------|--------------|-----------|---------------|
| CRITICAL | 9 | 1 | 10 |
| HIGH | 26 | 4 | 30 |
| MEDIUM | 21 | 7 | 28 |
| LOW | 4 | 3 | 7 |
| Plan corrections | — | 6 | 6 |
| **TOTAL** | **60** | **21** | **81 gaps, all covered** |

