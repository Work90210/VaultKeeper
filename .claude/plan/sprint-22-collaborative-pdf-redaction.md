# Sprint 22: Collaborative PDF Redaction Editor

**Phase:** 3 — Advanced Features
**Duration:** Weeks 43-46 (4 weeks)
**Goal:** Build a real-time collaborative PDF redaction editor where multiple prosecutors can simultaneously draw, resize, and annotate redaction areas on PDF documents, with auto-save, undo/redo, live cursors, and destructive redaction on apply.

---

## Prerequisites

- Sprint 7-8 complete (witness management, evidence versioning, redaction engine, disclosure workflow)
- Backend destructive PDF redaction working (MuPDF rasterize → black rectangles → pdfcpu reconstruct)
- Redaction API: `POST /api/evidence/:id/redact` accepts `[{page_number, x, y, width, height, reason}]`
- Image redaction editor UI working (canvas-based draw/resize/delete/reason)

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)
- [x] Infrastructure (WebSocket, Database)

---

## Architecture

```
Browser (Next.js)
├── PDF.js — renders each page as canvas
├── Canvas overlay — redaction rectangles drawn on top
├── Yjs CRDT — shared document state (redaction areas)
├── y-websocket — syncs Yjs state via WebSocket
├── UndoManager — per-user undo/redo
└── Presence — live cursors, user colors

Go Backend
├── WebSocket Hub — y-websocket protocol, room per evidence item
├── Draft Persistence — Yjs binary state saved to Postgres
├── Page Renderer — GET /api/evidence/:id/pages/:num (MuPDF → JPEG)
└── Redaction API — existing POST /api/evidence/:id/redact
```

---

## Implementation Steps

### Step 1: Database — Redaction Drafts

```sql
CREATE TABLE redaction_drafts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id   UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
    case_id       UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    created_by    UUID NOT NULL,
    yjs_state     BYTEA,
    status        TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'applied', 'discarded')),
    last_saved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Step 2: PDF Page Renderer API

**Endpoint:** `GET /api/evidence/:id/pages/:pageNum?dpi=150`

Uses go-fitz (MuPDF) to render a single PDF page as JPEG. Cached in MinIO for subsequent requests.

**Files:**
- `internal/evidence/pages_handler.go`
- `internal/evidence/page_cache.go`

### Step 3: WebSocket Collaboration Hub

Go WebSocket server implementing the y-websocket binary protocol.

**Files:**
- `internal/collaboration/hub.go` — manages rooms, connections
- `internal/collaboration/room.go` — per-document Yjs state
- `internal/collaboration/handler.go` — HTTP→WS upgrade, JWT auth
- `internal/collaboration/persistence.go` — auto-save Yjs state to Postgres

**Endpoint:** `WS /api/evidence/:id/redact/collaborate`

### Step 4: PDF.js Page Renderer Component

Client-side PDF rendering with page navigation and zoom.

**Dependencies:** `pdfjs-dist`

**Features:**
- Render PDF pages as canvas layers
- Page navigation (prev/next/jump/thumbnail strip)
- Zoom: fit-width, fit-page, 50%-200%
- Report page dimensions for coordinate mapping

### Step 5: Collaborative Redaction Editor

The main editor component combining PDF rendering, canvas overlay, and Yjs collaboration.

**Dependencies:** `yjs`, `y-websocket`

**Yjs shared state:**
```typescript
yRedactions: Y.Array<Y.Map>  // {id, page, x, y, w, h, reason, author, color}
yPresence: Y.Map             // {userId: {cursor, color, username}}
```

**Features:**
- Draw rectangles on any page (click-drag)
- Resize via corner/edge handles
- Move via center drag
- Delete via button or keyboard
- Per-area reason input (required, shown in sidebar)
- Sidebar: all areas grouped by page, connected users
- Undo/redo: Yjs UndoManager (per-user, local changes only)
- Auto-save: WebSocket syncs to server, persisted every 5s
- Live cursors: other users' positions shown with colored dots
- Preview mode: toggle solid black rectangles
- Apply: collect all areas → POST to redaction API → destructive copy created

### Step 6: Keyboard Shortcuts & UX Polish

- `Ctrl/Cmd+Z` — Undo
- `Ctrl/Cmd+Shift+Z` — Redo
- `Delete/Backspace` — Delete selected
- `Escape` — Deselect / exit preview
- `PageUp/PageDown` — Navigate pages
- `+/-` — Zoom
- `Ctrl/Cmd+0` — Fit to page

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `migrations/015_redaction_drafts.up.sql` | Create | Drafts table |
| `internal/collaboration/hub.go` | Create | WebSocket hub |
| `internal/collaboration/room.go` | Create | Per-document room |
| `internal/collaboration/handler.go` | Create | WS handler + JWT auth |
| `internal/collaboration/persistence.go` | Create | Draft auto-save |
| `internal/evidence/pages_handler.go` | Create | PDF page renderer API |
| `web/src/components/redaction/collaborative-editor.tsx` | Create | Main editor |
| `web/src/components/redaction/pdf-page-renderer.tsx` | Create | PDF.js renderer |
| `web/src/components/redaction/page-navigator.tsx` | Create | Page nav controls |
| `web/src/components/redaction/zoom-controls.tsx` | Create | Zoom controls |
| `web/src/components/redaction/presence-indicator.tsx` | Create | Live user presence |
| `web/src/hooks/use-yjs.ts` | Create | Yjs + WebSocket hook |
| `web/src/hooks/use-pdf.ts` | Create | PDF.js hook |

---

## Definition of Done

- [ ] PDF pages render client-side via PDF.js
- [ ] Multi-page navigation with zoom/pan
- [ ] Draw, resize, move, delete redaction rectangles on any page
- [ ] Per-area reason annotation (required)
- [ ] Undo/redo works correctly (Yjs UndoManager)
- [ ] Auto-save: draft state persisted via WebSocket
- [ ] Real-time: two+ users can edit simultaneously without conflicts
- [ ] Presence: live cursors and user list visible
- [ ] Preview mode shows solid black rectangles
- [ ] Apply creates destructive redacted copy (text layer destroyed)
- [ ] Custody chain logged
- [ ] Works for PDFs with 50+ pages
- [ ] Keyboard shortcuts functional
- [ ] Draft cleanup after configurable TTL

---

## Security Checklist

- [ ] WebSocket authenticated via Keycloak JWT
- [ ] Room access restricted to case investigators/prosecutors
- [ ] No redacted content in WebSocket messages (coordinates/reasons only)
- [ ] Applied redaction destroys text layer (verified)
- [ ] Abandoned drafts cleaned up

---

## Dependencies

```bash
# Go
go get nhooyr.io/websocket

# Frontend
pnpm add pdfjs-dist yjs y-websocket
```

---

## Estimated Effort

| Step | Days | Risk |
|------|------|------|
| 1. Draft table | 0.5 | Low |
| 2. Page renderer API | 1.5 | Low |
| 3. WebSocket hub + Yjs protocol | 3 | Medium |
| 4. PDF.js renderer component | 2 | Low |
| 5. Collaborative editor | 5 | High |
| 6. Shortcuts + polish | 2 | Low |
| **Total** | **14** | |
