# Sprint 9: Classifications, Legal Hold & Retention

**Phase:** 2 — Institutional Features
**Duration:** Weeks 17-18
**Goal:** Implement the four-tier confidentiality classification system, legal hold enforcement, retention policies, and audited evidence destruction.

---

## Prerequisites

- Sprint 8 complete (redaction, disclosure workflow)

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Confidentiality Classifications

**Four levels:**

| Level | Description | Access Rules |
|-------|-------------|-------------|
| Public | No restrictions | All case roles |
| Restricted | Default classification | All case roles (standard) |
| Confidential | Sensitive items | Investigator, Prosecutor, Judge only |
| Ex Parte | One-sided items | Only the side it's assigned to (prosecution OR defence, not both) |

**Implementation:**
- Classification stored on `evidence_items.classification`
- Evidence query filter adds classification-based WHERE clause
- Ex parte requires additional `ex_parte_side` field (new migration)
- Classification change is a custody event

**Migration 006: `006_ex_parte_side.up.sql`**
```sql
ALTER TABLE evidence_items ADD COLUMN ex_parte_side TEXT;
-- Only set when classification = 'ex_parte'
-- Values: 'prosecution', 'defence'
ALTER TABLE evidence_items ADD CONSTRAINT chk_ex_parte_side 
    CHECK (classification != 'ex_parte' OR ex_parte_side IS NOT NULL);
```

**Access matrix per classification:**

| Classification | investigator | prosecutor | defence | judge | observer | victim_rep |
|---------------|:---:|:---:|:---:|:---:|:---:|:---:|
| public | x | x | x | x | x | x |
| restricted | x | x | x (disclosed) | x | x | x (disclosed) |
| confidential | x | x | - | x | - | - |
| ex_parte (prosecution) | x | x | - | x | - | - |
| ex_parte (defence) | - | - | x | x | - | - |

**Tests:**
- Each classification × role combination tested
- Defence cannot see confidential items
- Defence cannot see prosecution's ex parte items
- Prosecution cannot see defence's ex parte items
- Judge sees all classifications
- Classification change logged to custody chain
- Ex parte without side specified → validation error
- Classification upgrade (public → confidential) → custody logged
- Classification downgrade (confidential → public) → custody logged

### Step 2: Legal Hold Enforcement

**Deliverable:** Comprehensive legal hold that blocks destruction and certain modifications.

**When legal hold is active on a case:**
- Evidence destruction → blocked (409 error)
- Evidence file replacement → blocked
- Case archival → blocked
- Case deletion → blocked
- Evidence metadata changes → still allowed (tags, description)
- New evidence uploads → still allowed
- Disclosures → still allowed

**Implementation:**
- `cases.legal_hold` boolean checked at service layer
- Check happens before any destructive operation
- Legal hold toggle logged to custody chain
- Notification sent to all case members on hold change

**Tests:**
- Legal hold = true → destroy evidence → 409
- Legal hold = true → archive case → 409
- Legal hold = true → upload new evidence → 201 (allowed)
- Legal hold = true → update tags → 200 (allowed)
- Legal hold toggle → custody log entry
- Legal hold toggle → notification to all case members
- Concurrent legal hold check → no race condition

### Step 3: Retention Policies

**Deliverable:** Evidence retention periods with expiry warnings.

**Implementation:**
- `evidence_items.retention_until` — earliest date evidence can be destroyed
- `cases.retention_until` — case-level retention (applies to all evidence in case)
- Effective retention = MAX(item retention, case retention)
- Background job runs daily: check for items with retention expiring in 30 days → admin notification
- Destruction blocked if current date < retention_until

**GDPR conflict handling:**
- If GDPR erasure request + active retention → system surfaces warning to admin
- Admin must manually resolve (court order overrides GDPR, or vice versa)
- Resolution logged in custody chain with decision and rationale
- System does NOT auto-decide — this is a legal judgment, not a technical one

**GDPR conflict workflow:**
1. Admin submits erasure request via `POST /api/evidence/:id/erasure-request`
2. System checks: legal hold? Active retention? Part of ongoing case?
3. If any conflict: create `conflict_warning` record with affected evidence IDs
4. Admin dashboard shows pending conflicts with:
   - Evidence item details
   - Conflicting policies (which legal hold, which retention period)
   - Decision options: "Preserve (legal override)" or "Erase (GDPR compliance)"
5. Admin resolves via `POST /api/evidence/:id/erasure-request/resolve`
   - Must provide `decision` and `rationale` (free text, required)
6. Custody log entry: `action: "gdpr_conflict_resolved"`, details include decision + rationale
7. If decision = "Erase": proceed with destruction workflow (Step 4)
8. If decision = "Preserve": mark erasure request as denied with legal basis

**Tests:**
- Set retention on evidence → destruction before date blocked
- Set retention on case → applies to all evidence
- Effective retention is MAX of item and case
- Retention expiring in 30 days → notification sent
- Retention expired → destruction allowed
- GDPR erasure request + active retention → conflict warning surfaced
- GDPR conflict resolution → custody log entry with decision
- GDPR erasure with no conflicts → proceeds to destruction
- Retention date in past → no effect (destruction allowed)

### Step 4: Audited Evidence Destruction

**Deliverable:** Complete destruction workflow with legal authority requirement.

**Destruction flow:**
1. Verify user is Case Admin+ or System Admin
2. Verify case does NOT have legal_hold
3. Verify retention period has expired (or no retention set)
4. Require `destruction_authority` in request (court order reference or legal basis)
5. Delete file from MinIO
6. Set evidence_items: `destroyed_at`, `destroyed_by`, `destruction_authority`
7. **Preserve the evidence_items row permanently** (metadata + hash survive)
8. **Preserve the entire custody chain permanently**
9. Custody log: `action: "destroyed"`, details include authority and hash at destruction
10. Notification to case admins

**The evidence_items row after destruction:**
```
id:                   (preserved)
evidence_number:      (preserved)
sha256_hash:          (preserved — proof the item existed)
tsa_token:            (preserved — proof of when it existed)
file_key:             NULL (file deleted)
file_size:            (preserved)
destroyed_at:         2026-04-06T15:00:00Z
destroyed_by:         user-uuid
destruction_authority: "Court Order ICC-UKR-2024-ORDER-005"
```

**Tests:**
- Full destruction flow → file deleted, metadata preserved
- Without authority string → 400
- With legal hold → 409
- Before retention expires → 409
- After destruction → download returns 410 (Gone)
- After destruction → metadata still accessible
- After destruction → custody chain still queryable
- Custody log records destruction details
- Notification sent to admins

### Step 5: Tagging & Taxonomy

**Deliverable:** Custom tag hierarchies and predefined tag sets.

**Predefined tag categories:**
- Evidence type: document, photo, video, audio, physical, digital
- Source type: witness, open_source, intercept, forensic, field_collection
- Relevance: critical, supporting, background, reference

**Tag validation:**
- Alphanumeric + hyphens + underscores only
- Max 100 characters per tag
- Max 50 tags per evidence item
- Case-insensitive (stored lowercase)

**Custom tags:**
- Users can add custom tags (freeform within validation rules)
- Tags autocomplete from existing tags in the case
- Tag management endpoint for case admins (rename, merge, delete tags)

**Tests:**
- Add valid tag → stored correctly
- Invalid tag (special chars) → 400
- Tag exceeding 100 chars → 400
- 51st tag → 400
- Tag autocomplete returns matching tags
- Case-insensitive storage (MyTag → mytag)
- Tag rename → all evidence items updated
- Tag merge → combined, no duplicates

### Step 6: Frontend for Classifications & Destruction

**Components:**
- `ClassificationSelector` — Dropdown with color coding
  - Public (green), Restricted (blue), Confidential (orange), Ex Parte (red)
  - Ex Parte shows side selector (prosecution/defence)
- `LegalHoldControl` — Toggle with confirmation dialog and notification preview
- `RetentionDatePicker` — Date picker with "no expiry" option
- `DestructionDialog` — Multi-step confirmation
  1. Warning about permanence
  2. Court order / authority reference (required text input)
  3. Final confirmation with evidence number + hash displayed
  4. Success confirmation showing preserved metadata
- `TagInput` — Multi-tag input with autocomplete
  - Type to search existing tags
  - Create new tag inline
  - Remove tags with X button
  - Color-coded by category

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/evidence/classification.go` | Create | Classification access rules |
| `internal/evidence/retention.go` | Create | Retention policy checks |
| `internal/evidence/destruction.go` | Create | Destruction workflow |
| `internal/evidence/tags.go` | Create | Tag validation + management |
| `internal/cases/service.go` | Modify | Legal hold enforcement |
| `migrations/006_ex_parte_side.up.sql` | Create | Add ex_parte_side column |
| `web/src/components/evidence/ClassificationSelector.tsx` | Create | Classification UI |
| `web/src/components/evidence/DestructionDialog.tsx` | Create | Destruction workflow UI |
| `web/src/components/evidence/TagInput.tsx` | Create | Tag management UI |

---

## Definition of Done

- [ ] Four classification levels enforced per role matrix
- [ ] Ex parte items visible only to correct side + judges
- [ ] Legal hold blocks destruction and archival
- [ ] Retention period blocks premature destruction
- [ ] Destruction requires authority, preserves metadata + custody chain
- [ ] File removed from MinIO on destruction
- [ ] Evidence row preserved permanently after destruction
- [ ] GDPR conflict surfaced as warning (no auto-decision)
- [ ] Tags validated and stored correctly
- [ ] Tag autocomplete works
- [ ] All operations logged to custody chain
- [ ] 100% test coverage

---

## Security Checklist

- [ ] Classification access cannot be bypassed via direct evidence ID query
- [ ] Ex parte side enforced at repository level (not just UI)
- [ ] Legal hold check is atomic (no TOCTOU race)
- [ ] Destruction authority logged immutably
- [ ] Destroyed file actually deleted from MinIO (not just marked)
- [ ] Tag content validated (no XSS via tag names)

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 9 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `classification.CheckAccess` — test every cell in the role x classification matrix (investigator/prosecutor/defence/judge/observer/victim_rep x public/restricted/confidential/ex_parte_prosecution/ex_parte_defence = 24 combinations)
- `classification.SetClassification` — validates ex_parte requires side, rejects unknown classification values, logs custody event
- `classification.validateExParteSide` — rejects nil side when classification is ex_parte, rejects invalid side values
- `legalhold.CheckHold` — returns true/false correctly, atomic check (no TOCTOU)
- `legalhold.ToggleHold` — creates custody log entry, sends notification to all case members
- `retention.EffectiveRetention` — returns MAX of item-level and case-level retention
- `retention.CheckRetention` — blocks destruction before expiry, allows after expiry, handles nil retention
- `retention.ExpiryNotification` — triggers notification at 30-day threshold
- `gdpr.CreateErasureRequest` — detects conflicts with legal hold, active retention, ongoing case
- `gdpr.ResolveConflict` — requires decision + rationale, logs custody event, proceeds with correct action
- `destruction.DestroyEvidence` — validates authority string present, checks legal hold, checks retention, deletes file from MinIO, preserves metadata row, preserves custody chain
- `destruction.DestroyEvidence` — returns 409 on legal hold, 409 on active retention, 400 on missing authority
- `tags.AddTag` — validates format (alphanumeric + hyphens + underscores), max 100 chars, max 50 tags, case-insensitive storage
- `tags.AddTag` — rejects special characters, rejects tags exceeding 100 chars, rejects 51st tag
- `tags.AutoComplete` — returns matching tags from case, case-insensitive matching
- `tags.Rename` — updates all evidence items, no duplicates after rename
- `tags.Merge` — combines tags, removes duplicates

### Integration Tests (testcontainers)

- Classification enforcement end-to-end: create case with evidence at each classification level, query as each role — verify exact visibility matrix matches specification
- Ex parte isolation: create prosecution ex parte evidence and defence ex parte evidence — prosecution sees only their ex parte items, defence sees only theirs, judge sees both
- Legal hold enforcement: enable legal hold on case, attempt destroy evidence (409), attempt archive case (409), attempt upload new evidence (201 allowed), attempt update tags (200 allowed)
- Legal hold toggle notification: toggle hold, verify notification records created for all case members
- Retention + destruction: set retention to future date, attempt destruction (409), advance clock past retention, attempt destruction (200)
- GDPR conflict workflow: create evidence with active retention, submit erasure request, verify conflict_warning record created, resolve with "Preserve" decision, verify custody log entry with rationale
- GDPR erasure without conflict: create evidence with no retention and no legal hold, submit erasure request, verify proceeds to destruction
- Destruction completeness: destroy evidence, verify file deleted from MinIO (GET returns 404), verify evidence_items row preserved with destroyed_at/destroyed_by/destruction_authority, verify custody chain still queryable, verify download returns 410 Gone
- Tag operations across evidence: add tags to multiple evidence items, rename tag, verify all items updated, merge two tags, verify no duplicates

### E2E Automated Tests (Playwright)

- Classification selector: open evidence detail, change classification from "Restricted" to "Confidential" via dropdown, verify color changes to orange, verify custody chain shows classification change event
- Ex parte classification: set evidence to "Ex Parte," verify side selector appears, select "Prosecution," verify defence user cannot see the item
- Legal hold toggle: navigate to case settings, enable legal hold, confirm dialog, verify hold badge appears on case, attempt to delete evidence — verify error message displayed
- Destruction workflow: navigate to evidence item, click destroy, verify multi-step dialog (warning, authority input, final confirmation), enter court order reference, confirm, verify evidence shows "Destroyed" state with preserved metadata
- Retention date picker: set retention date on evidence, verify date displayed, attempt destruction before date — verify blocked with clear message
- Tag management: add tag via TagInput component with autocomplete, verify tag appears, add second tag, verify both display, remove first tag, verify removed, search by tag — verify filtered results
- GDPR conflict resolution UI: trigger erasure request on retained evidence, navigate to admin dashboard, verify conflict warning displayed, resolve with "Preserve" and rationale, verify resolution logged

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Log in as prosecutor, navigate to a case, select an evidence item, and change its classification from "Restricted" to "Confidential" using the ClassificationSelector dropdown.
   **Expected:** Dropdown shows four options color-coded (Public=green, Restricted=blue, Confidential=orange, Ex Parte=red). After selecting Confidential, the badge color changes to orange.
   **Verify:** Check custody chain — a "classified" event appears with old and new classification values. Log in as observer — the item is no longer visible.

2. [ ] **Action:** Set an evidence item's classification to "Ex Parte" and select "Prosecution" as the side.
   **Expected:** Side selector appears when Ex Parte is chosen. After saving, the item shows an "Ex Parte (Prosecution)" badge in red.
   **Verify:** Log in as defence — the item does not appear in the evidence grid. Log in as judge — the item is visible. Log in as prosecution — the item is visible.

3. [ ] **Action:** Attempt to set classification to "Ex Parte" without selecting a side.
   **Expected:** Validation error prevents saving. A clear message indicates the side must be specified.
   **Verify:** The API returns a 400 status with a message about the required ex_parte_side field.

4. [ ] **Action:** Navigate to case settings and toggle Legal Hold to ON. Confirm in the confirmation dialog.
   **Expected:** Legal hold indicator appears on the case header. A notification is sent to all case members.
   **Verify:** Check notification inbox for each case member — legal hold notification present. Case detail page shows legal hold badge.

5. [ ] **Action:** With legal hold active, attempt to destroy an evidence item.
   **Expected:** Destruction is blocked. An error message states: "Cannot destroy evidence — case is under legal hold."
   **Verify:** Evidence item remains intact. Custody chain does not show a destruction attempt. HTTP response is 409 Conflict.

6. [ ] **Action:** With legal hold active, upload a new evidence item and update tags on an existing item.
   **Expected:** Both operations succeed. Upload returns 201. Tag update returns 200.
   **Verify:** New evidence item appears in the grid. Tags are updated on the existing item.

7. [ ] **Action:** Set a retention date of 2027-01-01 on an evidence item. Attempt to destroy it today.
   **Expected:** Destruction is blocked with message: "Cannot destroy evidence — retention period has not expired (expires 2027-01-01)."
   **Verify:** HTTP response is 409. Evidence item is unchanged.

8. [ ] **Action:** Navigate to an evidence item with no legal hold and expired retention (or no retention set). Click Destroy. In the multi-step dialog: read the warning, enter a court order reference ("Court Order ICC-2024-ORDER-005"), and confirm.
   **Expected:** Evidence file is deleted. The evidence item row shows "Destroyed" status with destroyed_at timestamp, destroyed_by user, and the court order reference. Custody chain shows destruction event.
   **Verify:** Attempt to download the file — returns 410 Gone. Evidence metadata (hash, evidence number, timestamps) is still visible. Custody chain is still fully queryable.

9. [ ] **Action:** Submit a GDPR erasure request on an evidence item that has an active retention period and is part of a case under legal hold.
   **Expected:** A conflict warning is surfaced on the admin dashboard showing the evidence item, the conflicting policies (legal hold + retention), and decision options.
   **Verify:** Navigate to admin dashboard — the pending conflict is listed with evidence details, conflicting policy names, and "Preserve" / "Erase" buttons.

10. [ ] **Action:** Resolve the GDPR conflict by selecting "Preserve (legal override)" and entering a rationale: "Court order supersedes GDPR request per Article 17(3)(e)."
    **Expected:** Conflict is marked as resolved. Erasure request is denied. Custody chain shows a "gdpr_conflict_resolved" event with the decision and rationale.
    **Verify:** The evidence item remains intact. The admin dashboard no longer shows the pending conflict. Custody log entry contains the exact rationale text.
