# Sprint 8: Document Redaction & Disclosure Workflow

**Phase:** 2 — Institutional Features
**Duration:** Weeks 15-16
**Goal:** Implement server-side document redaction (PDF/images) and the disclosure workflow that allows prosecutors to share evidence with defence teams, with full custody logging.

---

## Prerequisites

- Sprint 7 complete (witness management, evidence versioning)
- Evidence upload + version chain operational

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Redaction Engine (`internal/evidence/redaction.go`)

**Deliverable:** Server-side redaction for PDFs and images.

**Approach:**
- **PDFs:** Use `pdfcpu` or `unidoc/unipdf` to overlay black rectangles on specified coordinates. Output is a new PDF (original untouched).
- **Images:** Use Go `image` stdlib + `image/draw` to paint black rectangles. Output is a new image file.
- Redaction is DESTRUCTIVE on the copy — the underlying content is removed, not just covered. For PDFs this means removing text content within the redaction area, not just adding a visual overlay.

**Interface:**
```go
type RedactionService interface {
    ApplyRedactions(ctx context.Context, evidenceID uuid.UUID, redactions []RedactionArea) (RedactedResult, error)
    PreviewRedactions(ctx context.Context, evidenceID uuid.UUID, redactions []RedactionArea) (io.ReadCloser, error)
}

type RedactionArea struct {
    PageNumber int      // 1-based for PDFs, 0 for images
    X          float64  // Top-left X coordinate (percentage of page width, 0-100)
    Y          float64  // Top-left Y coordinate (percentage of page height, 0-100)
    Width      float64  // Width (percentage)
    Height     float64  // Height (percentage)
    Reason     string   // Why this was redacted (e.g., "witness identity protection")
}

type RedactedResult struct {
    NewEvidenceID uuid.UUID  // The redacted copy is a new evidence item
    OriginalID    uuid.UUID  // Link to the original
    RedactionCount int
    NewHash       string     // SHA-256 of the redacted file
}
```

**Flow:**
1. Fetch original file from MinIO
2. Apply redaction areas (remove content, not just overlay)
3. Compute SHA-256 of redacted file
4. Request RFC 3161 timestamp for redacted hash
5. Store redacted file in MinIO as new object
6. Create new evidence_items row with `parent_id` pointing to original
7. Custody log: `action: "redacted"`, details include redaction coordinates and reasons
8. Original file and evidence item completely untouched

**Security:**
- Redacted content must be truly removed (not recoverable from the output file)
- Redaction runs in isolated process/goroutine (malicious PDFs shouldn't crash the server)
- Original file access requires investigator/prosecutor role
- Redacted file can be shared with defence via disclosure

**Tests:**
- PDF redaction → text under redaction area removed from output
- Image redaction → pixels replaced with black
- Redacted file has different hash than original
- Redacted file has own TSA timestamp
- Parent_id links to original
- Custody log entry with redaction details
- Multiple redaction areas on same page
- Multi-page PDF with redactions on different pages
- Preview returns redacted view without creating permanent copy
- Original file unchanged after redaction
- Redacted PDF text extraction → redacted text not recoverable
- Zero redaction areas → error (don't create empty redaction)
- Invalid coordinates (negative, >100%) → error
- Non-PDF/image file → error (redaction only for supported types)
- **Content removal verification (critical):**
  - Redacted PDF: extract text from redacted area using `pdfcpu` → must return empty
  - Redacted PDF: binary search for redacted text string in output file → must not be found
  - Redacted image: pixel values in redacted area → all black (0,0,0)
  - Redacted PDF: copy-paste from redacted area → yields nothing
  - This prevents "visual-only" redaction where text layer is still intact under the black box

### Step 2: Redaction Frontend

**Deliverable:** UI for drawing redaction areas on documents/images.

**Components:**
- `RedactionEditor` — Full-screen overlay editor
  - PDF: page-by-page view with overlay canvas
  - Images: full image with overlay canvas
  - Draw tool: click-drag to create redaction rectangle
  - Resize/move existing rectangles
  - Delete rectangle
  - Per-rectangle reason input (required)
  - Preview button (shows redacted result without saving)
  - Apply button (creates redacted copy)
  - Page navigation for multi-page PDFs
- `RedactionBadge` — Visual indicator on evidence grid showing item has redacted versions

**UX:**
- Red semi-transparent rectangles while editing
- Black solid rectangles in preview
- Confirmation dialog before applying ("This will create a new copy. Original is preserved.")
- Progress indicator during server-side processing

### Step 3: Disclosure Service (`internal/disclosures/service.go`)

**Deliverable:** Disclosure workflow — prosecutors share evidence with defence.

**Interface:**
```go
type DisclosureService interface {
    CreateDisclosure(ctx context.Context, input CreateDisclosureInput) (Disclosure, error)
    ListDisclosures(ctx context.Context, caseID uuid.UUID, pagination Pagination) (PaginatedResult[Disclosure], error)
    GetDisclosure(ctx context.Context, id uuid.UUID) (Disclosure, error)
}

type CreateDisclosureInput struct {
    CaseID      uuid.UUID
    DisclosedTo string      // Role or specific user ID
    EvidenceIDs []uuid.UUID // Items being disclosed
    Notes       string
    Redacted    bool        // Whether redacted versions are provided
}
```

**Disclosure flow:**
1. Prosecutor selects evidence items for disclosure
2. Optionally marks items as redacted (links to redacted version)
3. Creates disclosure record in Postgres
4. Custody log: `action: "disclosed"`, details include all evidence IDs and recipient
5. Defence user now sees these items in their case view
6. Notification sent to disclosed_to user/role

**Defence visibility after disclosure:**
- Defence user queries evidence → JOIN with disclosures table
- Only evidence_ids present in ANY disclosure for this case are visible
- If both original and redacted versions exist, defence sees only the redacted version
- Defence can see disclosure metadata (when, by whom, what was shared)

**Rules:**
- Only prosecutors can create disclosures (case role check)
- Evidence must belong to the specified case
- Disclosure is irreversible (once disclosed, cannot un-disclose — this is a legal constraint)
- Same evidence can be in multiple disclosures (e.g., disclosed to defence and to judges)
- Empty evidence_ids → error (must disclose at least one item)

**Tests:**
- Create disclosure → record saved, defence can see items
- Non-prosecutor → 403
- Evidence from wrong case → 400
- Disclosure logged in custody chain
- Notification sent to defence
- Defence visibility query returns only disclosed items
- Redacted flag → defence sees redacted version, not original
- Multiple disclosures → union of all disclosed items visible
- List disclosures → paginated, sorted by date
- Empty evidence_ids → error

### Step 4: Disclosure Endpoints

```
POST   /api/cases/:id/disclosures         → Create disclosure (Prosecutor only)
GET    /api/cases/:id/disclosures          → List disclosures
GET    /api/disclosures/:id                → Get disclosure detail
```

### Step 5: Disclosure Frontend

**Components:**
- `DisclosureWizard` — Multi-step disclosure creation
  1. Select evidence items (checkbox list from evidence grid)
  2. Choose recipient (role or specific user)
  3. Mark items as redacted (toggle per item, link to redacted version)
  4. Add notes
  5. Review & confirm
- `DisclosureList` — Table of disclosures for a case
  - Date, by whom, to whom, item count, redacted flag
  - Click to see full disclosure details
- `DisclosureDetail` — Full disclosure with evidence item list
  - Evidence items with thumbnails
  - Download disclosed package (ZIP of disclosed items)

**Tests:**
- Wizard flow completes end-to-end
- Item selection works
- Redaction toggle links correct version
- Confirmation dialog prevents accidental disclosure
- List renders correctly
- Detail shows all items

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/evidence/redaction.go` | Create | PDF/image redaction engine |
| `internal/evidence/redaction_test.go` | Create | Redaction tests with sample files |
| `internal/disclosures/service.go` | Create | Disclosure business logic |
| `internal/disclosures/repository.go` | Create | Disclosure Postgres queries |
| `internal/disclosures/handler.go` | Create | Disclosure HTTP handlers |
| `internal/evidence/repository.go` | Modify | Add defence visibility filtering with disclosures JOIN |
| `web/src/components/redaction/*` | Create | Redaction editor components |
| `web/src/components/disclosures/*` | Create | Disclosure wizard + list |
| `migrations/005_disclosure_indexes.up.sql` | Create | Additional disclosure indexes |

---

## Definition of Done

- [ ] PDF redaction removes content (not just visual overlay)
- [ ] Image redaction replaces pixels with black
- [ ] Redacted copy is new evidence item with own hash + timestamp
- [ ] Original file completely untouched
- [ ] Redaction logged to custody chain with coordinates + reasons
- [ ] Disclosure created by prosecutor, defence now sees items
- [ ] Defence visibility correctly filters to disclosed items only
- [ ] Redacted flag routes defence to redacted version
- [ ] Disclosure is irreversible
- [ ] All operations logged to custody chain
- [ ] 100% test coverage on redaction + disclosure
- [ ] Redaction UI allows draw, resize, delete rectangles
- [ ] Disclosure wizard completes full workflow

---

## Security Checklist

- [ ] Redacted content truly removed (not recoverable from PDF metadata/layers)
- [ ] Redaction processing isolated (malicious PDF doesn't crash server)
- [ ] Only prosecutors can create disclosures
- [ ] Defence never sees original when redacted version exists
- [ ] Disclosure cannot be reversed (legal requirement)
- [ ] Redaction coordinates never expose the content being redacted
- [ ] File processing runs with resource limits (no zip bomb, no PDF bomb)

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 8 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `redaction.ApplyRedactions` — verify black rectangles applied at correct coordinates for both PDF and image inputs
- `redaction.PreviewRedactions` — returns preview stream without persisting to MinIO
- `redaction.validateRedactionAreas` — rejects negative coordinates, values >100%, zero-area rectangles, missing reason
- `redaction.removePDFTextContent` — confirms text layer removed (not just visually overlaid) within redaction bounds
- `redaction.replaceImagePixels` — confirms all pixels in redaction area are (0,0,0) black
- `disclosure.CreateDisclosure` — validates prosecutor role, non-empty evidence list, evidence belongs to case
- `disclosure.CreateDisclosure` — returns error on empty evidence IDs, wrong case evidence, non-prosecutor caller
- `disclosure.ListDisclosures` — pagination, date sorting, correct total count
- `disclosure.GetDisclosure` — returns full disclosure with evidence item details
- `evidence.Repository.ListForDefence` — JOIN with disclosures returns only disclosed items, prefers redacted version over original
- PDF multi-page redaction — redactions on pages 1, 3, 5 of a 6-page PDF each processed independently
- Image format coverage — JPEG, PNG, TIFF inputs all produce valid redacted outputs

### Integration Tests (testcontainers)

- Full redaction pipeline: upload PDF to MinIO, apply redaction via service, verify new evidence_items row with parent_id, verify SHA-256 differs from original, verify RFC 3161 timestamp obtained, verify custody log entry
- Full redaction pipeline (image): upload JPEG, apply redaction, verify pixel replacement, verify new hash + timestamp
- Disclosure end-to-end: create case, upload evidence, apply redaction, create disclosure as prosecutor, query evidence as defence user — only disclosed items visible, redacted version served instead of original
- Redaction content removal verification: upload PDF with known text, redact area containing that text, extract text from output PDF — redacted text must not appear, binary search of output file for redacted string must fail
- Disclosure irreversibility: create disclosure, attempt to delete/reverse it — must fail
- Notification delivery: create disclosure, verify notification record created for defence user
- Concurrent redaction: two simultaneous redaction requests on same evidence — both succeed, produce separate copies

### E2E Automated Tests (Playwright)

- Redaction editor flow: open evidence item, launch redaction editor, draw rectangle on PDF page, enter reason, click preview, verify black rectangle visible, click apply, confirm dialog, verify new evidence item created in grid with redaction badge
- Multi-rectangle redaction: draw 3 rectangles on different pages, apply all, verify each page shows redaction in preview
- Redaction editor for image: open image evidence, draw redaction rectangle, apply, verify redacted image renders with black area
- Disclosure wizard flow: navigate to case, click create disclosure, select 3 evidence items, choose defence as recipient, toggle one item as redacted, add notes, review step shows correct summary, confirm, verify disclosure appears in disclosure list
- Defence view after disclosure: log in as defence user, navigate to case, verify only disclosed items visible in evidence grid, verify redacted item shows redacted version (not original), verify non-disclosed items not visible
- Disclosure list and detail: navigate to disclosure list, verify table shows date/prosecutor/recipient/count, click disclosure, verify detail page shows all evidence items with thumbnails

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Log in as prosecutor, navigate to a case with uploaded PDF evidence, click the evidence item, and launch the redaction editor.
   **Expected:** Redaction editor opens full-screen with PDF rendered page-by-page and a draw tool active.
   **Verify:** PDF pages are navigable, canvas overlay is present, draw tool cursor appears.

2. [ ] **Action:** Click and drag to draw a redaction rectangle over a paragraph of text on page 1. Enter a reason ("witness identity protection") in the reason field.
   **Expected:** A red semi-transparent rectangle appears over the selected area. Reason field accepts text.
   **Verify:** Rectangle is resizable and movable. Reason is displayed next to the rectangle.

3. [ ] **Action:** Click "Preview" to see the redacted result.
   **Expected:** Preview shows the PDF with solid black rectangles replacing the red semi-transparent ones. No new evidence item is created yet.
   **Verify:** The text under the black rectangle is not visible. Check the evidence grid — no new item has been added.

4. [ ] **Action:** Click "Apply" and confirm in the confirmation dialog.
   **Expected:** Confirmation dialog states "This will create a new copy. Original is preserved." After confirming, a progress indicator appears. Upon completion, a new evidence item appears in the grid with a redaction badge.
   **Verify:** The new item has a different evidence number. The original item is unchanged. The custody chain shows a "redacted" entry with coordinates and reason.

5. [ ] **Action:** Download the redacted PDF and attempt to select/copy text from the redacted area using a PDF reader.
   **Expected:** No text is selectable or copyable from the redacted region. The text layer has been removed, not just visually covered.
   **Verify:** Open in Adobe Acrobat or equivalent, use text selection tool on the blacked-out area — nothing is selected. Search for the redacted text string — not found.

6. [ ] **Action:** Open an image evidence item (JPEG), launch redaction editor, draw a rectangle over a face, enter reason, and apply.
   **Expected:** Redacted image created with the face area replaced by solid black pixels.
   **Verify:** Download the redacted image, zoom into the redacted area — all pixels are black. Original image is unchanged.

7. [ ] **Action:** Navigate to the case disclosure page. Click "Create Disclosure." Select 3 evidence items (one original, one redacted copy, one image). Choose "Defence" as recipient. Toggle the redacted copy's "Redacted" flag. Add notes. Review and confirm.
   **Expected:** Disclosure record is created. A notification is sent to the defence team. The disclosure list shows the new entry with date, prosecutor name, "Defence" as recipient, and 3 items.
   **Verify:** Check the disclosure detail page — all 3 items listed with correct thumbnails and metadata.

8. [ ] **Action:** Log out. Log in as a defence user for the same case. Navigate to the case evidence page.
   **Expected:** Only the 3 disclosed evidence items are visible. No other case evidence is shown.
   **Verify:** Count the items in the grid — exactly 3. Attempt to access a non-disclosed evidence item by URL — returns 403.

9. [ ] **Action:** As the defence user, click on the item that was marked as redacted during disclosure.
   **Expected:** The redacted version is displayed, not the original. The redacted areas show solid black rectangles.
   **Verify:** The evidence number corresponds to the redacted copy. The original is not accessible to the defence user.

10. [ ] **Action:** As the prosecutor, attempt to reverse or delete the disclosure.
    **Expected:** No option to reverse or delete the disclosure exists. Disclosure is permanent and irreversible.
    **Verify:** Check the API directly — `DELETE /api/disclosures/:id` returns 405 or does not exist. The disclosure record remains in the database.
