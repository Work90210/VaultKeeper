# Sprint 19: Multi-Case Analytics & Advanced Workflow Engine

**Phase:** 4 — Enterprise & Scale
**Duration:** Weeks 37-38
**Goal:** Cross-case pattern detection and configurable approval workflows for ICC-scale deployments.

---

## Prerequisites

- Phase 3 (v3.0.0) complete
- Entity extraction operational
- Multiple cases with evidence data

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Cross-Case Analytics Service

**Deliverable:** Pattern detection across multiple cases.

**Interface:**
```go
type AnalyticsService interface {
    FindCrossReferences(ctx context.Context, query CrossRefQuery) ([]CrossReference, error)
    GetEntityOverlap(ctx context.Context, caseIDs []uuid.UUID) ([]EntityOverlap, error)
    GetLocationHeatmap(ctx context.Context, caseIDs []uuid.UUID) ([]LocationPoint, error)
    GetTimelineOverlay(ctx context.Context, caseIDs []uuid.UUID) ([]TimelineEvent, error)
}

type CrossReference struct {
    Entity      Entity
    Cases       []CaseReference
    TotalMentions int
    FirstSeen   time.Time
    LastSeen    time.Time
}

type EntityOverlap struct {
    EntityValue string
    EntityType  string
    CaseCount   int
    Cases       []CaseReference
}
```

**Key queries:**
- "This witness appears in 3 different cases"
- "This location has 47 pieces of evidence across 5 investigations"
- "This organization is mentioned in 8 cases"

**Cross-case entity matching:**
- Use normalized entity values from entity extraction
- Match across cases (same person, location, org)
- Confidence scoring based on exact match vs fuzzy match
- Privacy boundary: cross-case analytics only visible to System Admin or users with roles in ALL referenced cases

**Tests:**
- Entity appearing in 3 cases → identified in overlap query
- Location with evidence across cases → heatmap point
- Timeline overlay shows chronological events across cases
- User with role in only 1 case → only sees that case's data
- System admin → sees all cross-references
- Performance: 10 cases, 50,000 evidence items → query < 5 seconds

### Step 2: Analytics Dashboard Frontend

**Components:**
- `CrossCaseAnalytics` — Admin analytics page
  - Entity overlap matrix (which entities appear across which cases)
  - Location heatmap (map visualization with evidence density)
  - Timeline overlay (multiple case timelines superimposed)
  - Top cross-referenced entities (persons, locations, organizations)
- `EntityOverlapTable` — Table showing entities across cases
  - Entity name, type, case count, cases list
  - Click → drill down to evidence items
- `LocationHeatmap` — Map with evidence locations
  - GPS data from EXIF metadata
  - Heat intensity = evidence count
  - Click location → evidence items at that location
- `TimelineOverlay` — Multi-case timeline
  - Color-coded by case
  - Filterable by entity, location, date range

### Step 3: Configurable Workflow Engine

**Deliverable:** Evidence approval workflows with configurable stages.

**Workflow concept:**
Evidence progresses through stages: `collected → processed → reviewed → approved → disclosed`

Each stage requires sign-off from specific roles.

**Interface:**
```go
type WorkflowService interface {
    CreateWorkflow(ctx context.Context, caseID uuid.UUID, config WorkflowConfig) error
    AdvanceStage(ctx context.Context, evidenceID uuid.UUID, decision Decision) error
    GetWorkflowStatus(ctx context.Context, evidenceID uuid.UUID) (WorkflowStatus, error)
    GetPendingReviews(ctx context.Context, userID string) ([]PendingReview, error)
}

type WorkflowConfig struct {
    Stages []WorkflowStage
}

type WorkflowStage struct {
    Name            string   // "review", "approval", etc.
    RequiredRole    string   // case role required to advance
    RequiredCount   int      // how many sign-offs needed (default 1)
    AutoAdvance     bool     // auto-advance when count met
    NotifyRoles     []string // roles notified when item reaches this stage
}

type Decision struct {
    Action   string  // "approve", "reject", "return"
    Comment  string
    UserID   string
}
```

**Default workflow template:**
```yaml
stages:
  - name: collected
    required_role: investigator
    required_count: 1
  - name: processed
    required_role: investigator
    required_count: 1
  - name: reviewed
    required_role: prosecutor
    required_count: 1
  - name: approved
    required_role: prosecutor
    required_count: 2  # requires two prosecutors to approve
  - name: ready_for_disclosure
    required_role: prosecutor
    required_count: 1
```

**New tables:**
```sql
CREATE TABLE workflow_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id) NOT NULL UNIQUE,
    stages          JSONB NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE workflow_states (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL,
    current_stage   TEXT NOT NULL,
    decisions       JSONB DEFAULT '[]',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_workflow_states_evidence ON workflow_states(evidence_id);
CREATE INDEX idx_workflow_states_stage ON workflow_states(current_stage);
```

**Rules:**
- Workflow is optional per case (not all cases need approval stages)
- Evidence can only move forward (or be returned to previous stage)
- Each stage transition logged to custody chain
- Rejection returns to previous stage with comment
- Pending reviews visible in user's dashboard
- Workflow config editable by Case Admin (doesn't affect items already past a stage)

**Tests:**
- Evidence advances through all stages
- Wrong role → cannot advance
- Two sign-offs required → only advances after both
- Rejection → returns to previous stage
- Pending reviews for user → correct items listed
- Stage transition → custody log entry
- Notification sent at each stage
- Workflow disabled → evidence has no workflow restrictions
- Edit workflow config → doesn't retroactively affect completed stages

### Step 4: Workflow Frontend

**Components:**
- `WorkflowConfig` — Admin page to configure case workflow
  - Add/remove stages
  - Set required role and sign-off count per stage
  - Preview workflow pipeline
- `WorkflowStatus` — Badge on evidence item
  - Color-coded by stage
  - Click → view workflow history
- `WorkflowAction` — Approve/reject/return panel
  - Comment input (required for rejection)
  - Confirmation dialog
- `PendingReviews` — User dashboard widget
  - Items awaiting user's review
  - Grouped by case
  - Quick approve/reject actions

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/analytics/cross_case.go` | Create | Cross-case analytics |
| `internal/analytics/handler.go` | Create | Analytics endpoints |
| `internal/workflow/engine.go` | Create | Workflow state machine |
| `internal/workflow/service.go` | Create | Workflow business logic |
| `internal/workflow/handler.go` | Create | Workflow endpoints |
| `migrations/016_workflows.up.sql` | Create | Workflow tables |
| `web/src/components/analytics/*` | Create | Analytics dashboard |
| `web/src/components/workflow/*` | Create | Workflow UI |

---

## Definition of Done

- [ ] Cross-case entity overlap detected
- [ ] Location heatmap shows evidence distribution
- [ ] Timeline overlay works across cases
- [ ] Analytics respects role-based access
- [ ] Workflow engine advances evidence through stages
- [ ] Multi-sign-off works (2 prosecutors to approve)
- [ ] Rejection returns to previous stage
- [ ] Pending reviews visible in dashboard
- [ ] All transitions logged to custody chain
- [ ] 100% test coverage

---

## Security Checklist

- [ ] Cross-case analytics respects per-case role assignments
- [ ] User only sees analytics for cases they're assigned to
- [ ] Workflow decisions authenticated and authorized
- [ ] Workflow cannot be bypassed to skip stages
- [ ] Stage transitions immutable in custody chain

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 19 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/analytics/cross_case.go`** — `FindCrossReferences`: entity appearing in 3 cases returns CrossReference with 3 CaseReferences; entity in only 1 case excluded from cross-references; results sorted by total mention count descending
- **`internal/analytics/cross_case.go`** — `GetEntityOverlap`: returns entities shared across specified case IDs; filters by entity type; returns case count and case list per entity
- **`internal/analytics/cross_case.go`** — `GetLocationHeatmap`: returns GPS coordinates with evidence counts; aggregates across specified cases; locations with no GPS data excluded
- **`internal/analytics/cross_case.go`** — `GetTimelineOverlay`: returns chronologically ordered events across cases; each event tagged with case ID and color; date range filter works
- **Cross-case access control** — user with role in cases A and B only sees overlap between A and B; user with role in case A only sees no cross-references; System Admin sees all cases
- **`internal/workflow/engine.go`** — `AdvanceStage` with correct role: evidence moves to next stage; decision recorded with user ID, action, comment, and timestamp
- **`internal/workflow/engine.go`** — `AdvanceStage` with wrong role: returns authorization error; evidence stage unchanged
- **`internal/workflow/engine.go`** — multi-sign-off: stage requiring 2 approvals stays at current stage after 1 approval; advances after 2nd approval from different user; same user signing twice does not count as 2 approvals
- **`internal/workflow/engine.go`** — rejection: evidence returns to previous stage; rejection comment required; re-approval from that stage restarts the approval count
- **`internal/workflow/engine.go`** — auto-advance: when enabled and required count met, evidence advances without manual trigger
- **`internal/workflow/service.go`** — `CreateWorkflow`: valid config persisted; invalid config (empty stages, unknown role) returns validation error
- **`internal/workflow/service.go`** — `GetPendingReviews`: returns only items at stages where the user's role is required; items from cases the user is not assigned to are excluded
- **`internal/workflow/service.go`** — `GetWorkflowStatus`: returns current stage, decision history, and required actions for next stage
- **Workflow disabled** — case without workflow config allows evidence operations without workflow restrictions
- **Workflow config edit** — editing config does not retroactively affect evidence already past modified stages; new evidence follows updated config
- **Custody chain integration** — each stage transition creates custody log entry with stage name, decision, user ID, and timestamp; rejection creates separate entry
- **Notification triggers** — stage transition triggers notification to roles listed in notifyRoles; rejection notifies the original submitter
- **Migration `016_workflows.up.sql`** — tables created; indexes on evidence_id and current_stage created; JSONB columns default correctly

### Integration Tests (testcontainers)

- **Cross-case entity overlap end-to-end** — create 3 cases with overlapping entities (same person in all 3), query overlap, verify entity returned with case count 3 and correct case references
- **Location heatmap data** — create evidence items with GPS metadata across 2 cases, query heatmap, verify correct coordinates and evidence counts returned
- **Timeline overlay** — create evidence items with dates across 3 cases, query timeline, verify chronologically ordered events with correct case attribution
- **Workflow full lifecycle** — create case with 5-stage workflow, advance evidence through all stages with correct roles, verify final state is "ready_for_disclosure"
- **Multi-sign-off workflow** — create workflow requiring 2 prosecutor approvals at "approved" stage, submit 1 approval, verify still at "approved"; submit 2nd approval from different user, verify advances to next stage
- **Workflow rejection and re-approval** — advance evidence to "reviewed" stage, reject it, verify returns to "processed"; re-approve from "processed", verify advances to "reviewed" again
- **Pending reviews dashboard** — create 10 evidence items at various stages, query pending reviews for a prosecutor, verify only items at prosecutor-required stages returned
- **Cross-case analytics performance** — create 10 cases with 5,000 evidence items each, run entity overlap query, verify completes within 5 seconds

### E2E Automated Tests (Playwright)

- **Analytics dashboard loads** — as System Admin, navigate to Analytics, verify cross-case analytics page renders with entity overlap table, location heatmap placeholder, and timeline overlay
- **Entity overlap table** — verify table shows entities appearing across multiple cases with entity name, type, case count, and case names; click an entity to drill down to evidence items
- **Location heatmap renders** — verify map component renders with heat points at GPS coordinates; click a location cluster to see evidence items at that location
- **Timeline overlay** — verify multi-case timeline renders with color-coded events; filter by case or date range; events are chronologically ordered
- **Workflow configuration** — as Case Admin, navigate to case settings > Workflow, add 4 stages with roles and sign-off counts, save; verify stages appear in preview
- **Workflow stage advancement** — as investigator, view evidence item with workflow badge showing "collected", approve to advance to "processed"; verify badge updates
- **Multi-sign-off approval** — as prosecutor, approve evidence at "approved" stage; verify badge still shows "approved" (needs 2); log in as different prosecutor, approve again; verify badge advances to next stage
- **Workflow rejection** — as prosecutor at "reviewed" stage, click reject, enter comment, confirm; verify evidence returns to "processed" stage; verify rejection comment visible in workflow history
- **Pending reviews widget** — on dashboard, verify "Pending Reviews" widget shows items awaiting current user's action; click item to navigate to evidence detail with workflow action panel

---

## Manual E2E Testing Checklist

1. [ ] **Action:** As System Admin, navigate to Analytics > Cross-Case Analysis with 3 active cases that share at least one common person entity (e.g., "Jean-Pierre Bemba" appears in Cases A, B, and C)
   **Expected:** Entity overlap table shows "Jean-Pierre Bemba" with case count of 3, listing Cases A, B, and C
   **Verify:** Click the entity row to drill down; verify evidence items from all 3 cases are listed; verify mention counts per case are accurate

2. [ ] **Action:** View the Location Heatmap with 2 cases that have evidence with GPS metadata from overlapping geographic regions
   **Expected:** Map renders with heat intensity at locations where evidence is concentrated; overlapping regions show higher intensity
   **Verify:** Click a high-intensity area; verify evidence items from both cases appear; verify GPS coordinates in the popup match the evidence metadata

3. [ ] **Action:** As Case Admin, navigate to Case A settings > Workflow and configure: collected (investigator, 1 sign-off) -> processed (investigator, 1) -> reviewed (prosecutor, 1) -> approved (prosecutor, 2 sign-offs) -> ready_for_disclosure (prosecutor, 1)
   **Expected:** Workflow configuration saved; preview shows 5 stages in pipeline view with role requirements
   **Verify:** Save and reload the page; confirm all stages persist with correct roles and sign-off counts

4. [ ] **Action:** Upload a new evidence item to Case A (which now has a workflow configured)
   **Expected:** Evidence item appears with workflow badge showing stage "collected"; workflow history panel shows "Created at 'collected' stage"
   **Verify:** Verify the evidence cannot be disclosed without passing through all workflow stages

5. [ ] **Action:** As an investigator, advance the evidence from "collected" to "processed", then from "processed" to "reviewed"
   **Expected:** Each advancement updates the workflow badge; each transition logged in workflow history with user name, action, and timestamp
   **Verify:** Check custody chain for the evidence; verify two stage transition entries exist with correct stage names and user identities

6. [ ] **Action:** As Prosecutor A, approve the evidence at the "approved" stage (which requires 2 prosecutor sign-offs)
   **Expected:** Workflow history shows Prosecutor A's approval; badge still shows "approved" with "1/2 approvals" indicator
   **Verify:** Verify the evidence has NOT advanced to the next stage; verify Prosecutor A cannot approve a second time (their approval is already recorded)

7. [ ] **Action:** As Prosecutor B (a different prosecutor), approve the same evidence at the "approved" stage
   **Expected:** With 2/2 approvals met, evidence automatically advances to "ready_for_disclosure" stage
   **Verify:** Workflow badge shows "ready_for_disclosure"; workflow history shows both Prosecutor A and Prosecutor B approvals; custody chain records the stage transition

8. [ ] **Action:** Upload another evidence item, advance it to "reviewed" stage, then as a prosecutor, reject it with comment "Insufficient chain of custody documentation"
   **Expected:** Evidence returns to "processed" stage; rejection comment visible in workflow history; notification sent to the investigator who last advanced it
   **Verify:** Workflow badge shows "processed"; workflow history shows rejection entry with comment, rejector name, and timestamp; investigator's pending reviews widget shows the returned item

9. [ ] **Action:** Log in as a user who has an investigator role in Case A only, and navigate to Analytics > Cross-Case Analysis
   **Expected:** Cross-case analytics only shows data from Case A (no data from cases the user is not assigned to); entity overlap shows nothing (single case cannot have cross-case overlap)
   **Verify:** No entity names, locations, or timeline events from other cases are visible; attempting to manually construct API requests for other cases returns 403

10. [ ] **Action:** On the user dashboard, verify the "Pending Reviews" widget for a prosecutor assigned to 2 cases with active workflows
    **Expected:** Widget shows all evidence items across both cases that are at stages requiring prosecutor action, grouped by case
    **Verify:** Click an item in the pending reviews widget; verify it navigates to the evidence detail with the workflow action panel ready for approval/rejection
