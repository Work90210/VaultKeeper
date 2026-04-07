# Sprint 4: Evidence Upload, Hashing & Trusted Timestamping

**Phase:** 1 — Foundation
**Duration:** Weeks 7-8
**Goal:** Implement the core value proposition — chunked evidence upload via tus protocol, SHA-256 hashing, RFC 3161 trusted timestamping, MinIO storage with SSE, and automatic custody logging. This is the heart of VaultKeeper.

---

## Prerequisites

- Sprint 3 complete (cases CRUD, custody middleware, hash chain)
- MinIO running with SSE-S3 encryption enabled
- Custody logging middleware operational

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: MinIO Storage Client (`internal/evidence/storage.go`)

**Deliverable:** Abstracted object storage operations with SSE.

**Interface:**
```go
type ObjectStorage interface {
    PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
    GetObject(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
    DeleteObject(ctx context.Context, key string) error
    StatObject(ctx context.Context, key string) (ObjectInfo, error)
    GeneratePresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type ObjectInfo struct {
    Key         string
    Size        int64
    ContentType string
    ETag        string
    LastModified time.Time
}
```

**MinIO configuration:**
- SSE-S3 (server-side encryption) enabled on the `evidence` bucket
- Bucket lifecycle: incomplete multipart uploads expire after 24 hours
- Bucket policy: no public access
- Connection pooling with retry (3 retries, exponential backoff)

**Object key format:**
```
evidence/{case_id}/{evidence_id}/{version}/{original_filename}
```

**Tests:**
- PutObject → file stored, encrypted at rest
- GetObject → file retrieved, content matches what was stored
- DeleteObject → file removed from MinIO
- StatObject → correct size, content type, etag
- Non-existent key → not found error
- Large file (100MB) → handles correctly
- SSE enabled (verify encryption headers)
- Connection failure → retry with backoff
- Bucket not found → clear error message

### Step 2: RFC 3161 Timestamp Client (`internal/integrity/tsa.go`)

**Deliverable:** Client that sends SHA-256 hashes to an RFC 3161 Timestamp Authority.

**Interface:**
```go
type TimestampAuthority interface {
    RequestTimestamp(ctx context.Context, hash []byte) (TimestampResponse, error)
    VerifyTimestamp(ctx context.Context, token []byte, hash []byte) (TimestampVerification, error)
}

type TimestampResponse struct {
    Token     []byte    // DER-encoded RFC 3161 timestamp token
    Timestamp time.Time // Timestamp from TSA response
    TSAName   string    // Name of the TSA that signed
}

type TimestampVerification struct {
    Valid     bool
    Timestamp time.Time
    TSAName   string
    HashMatch bool
}
```

**Implementation:**
1. Create timestamp request (RFC 3161 TimeStampReq) with SHA-256 hash
2. Send HTTP POST to TSA endpoint (`TSA_URL` from config)
3. Parse timestamp response (TimeStampResp)
4. Extract timestamp token and signed time
5. Verify TSA signature against known CA certificates
6. Return token for storage in `evidence_items.tsa_token`

**TSA configuration:**
- Primary: FreeTSA.org (`https://freetsa.org/tsr`)
- Configurable via `TSA_URL` env var
- Timeout: 10 seconds per request
- Retry: 3 attempts with exponential backoff

**Error handling (per spec):**
- TSA unreachable → evidence upload STILL SUCCEEDS
- Set `tsa_token = NULL`, `tsa_timestamp = NULL`
- Custody log entry notes: "TSA unavailable — timestamp pending"
- Background retry job: attempts every 5 minutes for 24 hours
- Admin notification sent on first failure

**Background retry job (`internal/integrity/tsa_retry.go`):**
- On TSA failure: evidence_items row saved with `tsa_token = NULL`, `tsa_timestamp = NULL`
- Custody log entry: `"TSA unavailable — timestamp pending"`
- Admin notification sent on first TSA failure
- Retry job runs every 5 minutes for 24 hours:
  1. Query evidence_items WHERE tsa_token IS NULL AND uploaded_at > now() - 24h
  2. For each: re-request RFC 3161 timestamp
  3. On success: update tsa_token + tsa_timestamp, custody log: "TSA timestamp obtained (delayed)"
  4. On failure: continue to next item, retry again in 5 minutes
  5. After 24 hours: stop retrying, custody log: "TSA timestamp permanently unavailable"
- Retry job uses Postgres advisory lock to prevent duplicate execution across instances

**Tests:**
- Valid hash → timestamp token returned
- Timestamp token verifiable
- Token contains correct hash
- TSA unreachable → nil token returned, no error (graceful degradation)
- TSA unreachable → retry job picks up item within 5 minutes
- TSA recovers → retry job successfully timestamps pending items
- TSA returns error response → handled gracefully
- Timeout → retry with backoff
- Invalid TSA URL → configuration error on startup
- VerifyTimestamp with valid token → valid
- VerifyTimestamp with tampered hash → invalid
- VerifyTimestamp with expired certificate → appropriate handling
- TSA disabled (`TSA_ENABLED=false`) → skip entirely, no errors
- 24-hour retry window → items older than 24h no longer retried

### Step 3: Evidence Number Generator

**Deliverable:** Sequential, gap-free evidence numbers per case.

**Format:** `{case_reference_code}-{sequential_number}` (e.g., `ICC-UKR-2024-00001`)

**Implementation:**
- Postgres sequence per case (or counter column on cases table)
- Atomic increment + fetch (no gaps under concurrent uploads)
- Zero-padded to 5 digits (supports up to 99,999 items per case)
- Advisory lock during generation to prevent duplicates

**Tests:**
- First evidence in case → number ends in 00001
- Sequential uploads → sequential numbers (no gaps)
- Concurrent uploads → unique numbers (no duplicates)
- Different cases → independent sequences
- Number format matches expected pattern

### Step 4: Evidence Upload Flow (`internal/evidence/service.go`)

**Deliverable:** Complete evidence upload pipeline.

**Upload flow (sequential, not parallelizable):**
1. **Receive file** via tus protocol (chunked, resumable)
2. **Validate file**: check size against `MAX_UPLOAD_SIZE`, sanitize filename
3. **Compute SHA-256 hash** over the complete assembled file
4. **Request RFC 3161 timestamp** for the hash (async, non-blocking if TSA down)
5. **Detect MIME type** via `http.DetectContentType` + file extension
6. **Extract EXIF metadata** from images (GPS, timestamp, camera model)
7. **Generate evidence number** (sequential per case)
8. **Store file in MinIO** with SSE encryption
9. **Generate thumbnail** for images (Phase 1 — server-side thumbnail generation)
10. **Write evidence_items row** in Postgres
11. **Write custody_log entry**: action="uploaded", file_hash, evidence_number
12. **Index in Meilisearch** for full-text search
13. **Send notifications** to case members: "New evidence uploaded"

**Filename sanitization:**
```go
func SanitizeFilename(original string) string {
    // Strip path separators (/, \)
    // Remove null bytes and control characters
    // Replace spaces with underscores
    // Limit to 255 characters
    // Preserve original extension
    // Log original filename in metadata
}
```

**EXIF extraction (`internal/evidence/exif.go` — images only):**

Library: `github.com/rwcarlsen/goexif/exif` or `github.com/dsoprea/go-exif`

Fields extracted and stored in `evidence_items.metadata` JSONB:
```json
{
    "exif": {
        "gps_latitude": 48.8566,
        "gps_longitude": 2.3522,
        "gps_altitude": 35.0,
        "date_time_original": "2024-03-15T10:30:00Z",
        "camera_make": "Canon",
        "camera_model": "EOS R5",
        "lens_model": "RF 24-70mm F2.8L",
        "focal_length": "35mm",
        "iso": 400,
        "exposure_time": "1/250",
        "f_number": 5.6,
        "image_width": 8192,
        "image_height": 5464,
        "orientation": 1,
        "software": "Adobe Lightroom 6.0"
    }
}
```

GPS coordinate handling:
- Validate lat/lon ranges (-90/90 and -180/180)
- Convert DMS (degrees/minutes/seconds) to decimal degrees
- Store as decimal degrees for consistency
- Timezone from GPS data if available, else from DateTimeOriginal timezone offset

Security: EXIF data may contain sensitive location info — subject to same access controls as evidence metadata. Defence users don't see EXIF unless evidence is disclosed.

**Thumbnail generation (`internal/evidence/thumbnail.go`):**

Phase 1 thumbnails:
- **Images (JPEG, PNG, TIFF, WebP):** Resize to 300x300 max (maintain aspect ratio) using Go `image` stdlib + `golang.org/x/image/draw` for high-quality downscaling
- **PDFs:** Extract first page as image using `pdfcpu` or `mupdf` (via CGo) — generate 300x300 thumbnail
- **Audio:** Static audio waveform icon (no generation)
- **Video:** Extract first frame using `ffmpeg` (shelled out) — generate 300x300 thumbnail
- **Other files:** Static file-type icon

Thumbnails stored in MinIO at: `thumbnails/{evidence_id}/thumb.jpg`
Served via: `GET /api/evidence/:id/thumbnail`
Generated on upload (background goroutine, non-blocking)
Encrypted at rest (same MinIO SSE as evidence files)

**tus configuration:**
- Chunk size: 5MB (matches spec)
- Max file size: from `MAX_UPLOAD_SIZE` config (default 10GB)
- Incomplete uploads expire after 24 hours (tusd `ExpireUploads` config)
- Upload progress: tus protocol HEAD requests return `Upload-Offset` header — client polls this
- Composition: `tusd` with `filestore` (disk temp) → on completion, move assembled file to MinIO
- Temp directory: `/tmp/vaultkeeper-uploads/` with disk space monitoring
- Cleanup job: every hour, remove incomplete uploads older than 24 hours
- Concurrent uploads: limited by `MAX_CONCURRENT_UPLOADS` config (default 10)

**Upload progress in UI (tus-js-client):**
```typescript
const upload = new tus.Upload(file, {
    endpoint: "/api/cases/{id}/evidence",
    chunkSize: 5 * 1024 * 1024,  // 5MB
    retryDelays: [0, 1000, 3000, 5000],
    onProgress: (bytesUploaded, bytesTotal) => { /* update progress bar */ },
    onSuccess: () => { /* navigate to evidence detail */ },
    onError: (error) => { /* show retry button */ },
});
upload.start();
// Resume: upload.findPreviousUploads().then(...)
```

**Tests (100% coverage):**
- Small file upload → complete flow succeeds
- Large file upload (chunked) → resume works after interruption
- SHA-256 hash matches known test vector
- TSA timestamp obtained and stored
- TSA down → upload succeeds, tsa_token NULL, retry scheduled
- MIME type detected correctly (PDF, JPG, MP4, DOCX)
- EXIF extracted from JPEG with GPS data
- Evidence number generated correctly
- File stored in MinIO at correct key path
- Postgres row created with all fields
- Custody log entry created with correct hash
- Meilisearch indexed (title, description searchable)
- Notification sent to case members
- Filename sanitization strips dangerous characters
- File exceeding MAX_UPLOAD_SIZE → 413 error
- Invalid case_id → 404
- User without upload permission → 403
- Zero-byte file → rejected
- Concurrent uploads to same case → unique evidence numbers
- MinIO disk full → 507 error + admin notification "MinIO storage full" (per spec)
- MinIO unreachable → 503 error + health endpoint reports unhealthy
- Hash computation failure (corrupted assembled file) → 500, cleanup temp file
- EXIF extraction from JPEG → GPS, timestamp, camera model extracted correctly
- EXIF extraction from non-image → skipped gracefully (no error)
- Image without EXIF → metadata.exif = null
- Thumbnail generated for JPEG → stored in MinIO at thumbnails/{id}/thumb.jpg
- Thumbnail for PDF → first page extracted as image
- Thumbnail for video → first frame extracted via ffmpeg
- Thumbnail generation failure → non-blocking, evidence still uploaded, thumbnail = placeholder

### Step 5: Evidence Repository (`internal/evidence/repository.go`)

**Deliverable:** Data access layer for evidence_items table.

**Interface:**
```go
type Repository interface {
    Create(ctx context.Context, item EvidenceItem) (EvidenceItem, error)
    FindByID(ctx context.Context, id uuid.UUID) (EvidenceItem, error)
    FindByCase(ctx context.Context, caseID uuid.UUID, filter EvidenceFilter, pagination Pagination) (PaginatedResult[EvidenceItem], error)
    Update(ctx context.Context, id uuid.UUID, updates EvidenceUpdate) (EvidenceItem, error)
    MarkDestroyed(ctx context.Context, id uuid.UUID, destroyedBy string, authority string) error
    FindByHash(ctx context.Context, hash string) ([]EvidenceItem, error)
}
```

**EvidenceFilter:**
```go
type EvidenceFilter struct {
    MimeTypes       []string  // filter by type (image/*, video/*, etc.)
    Tags            []string  // filter by tags (AND logic)
    Classifications []string  // public, restricted, confidential, ex_parte
    DateFrom        time.Time // source_date range
    DateTo          time.Time
    SearchQuery     string    // ILIKE on title, description
    IsCurrent       *bool     // filter by version status
    UserRole        CaseRole  // determines visibility (defence → disclosed only)
}
```

**Defence visibility filtering:**
- Defence users: JOIN with disclosures table, only return evidence where evidence_id is in any disclosure.evidence_ids for this case
- All other roles: return all evidence matching filters

**Tests:**
- CRUD operations work correctly
- Filtering by each filter field
- Defence role → only disclosed evidence returned
- Pagination works correctly
- Hash lookup returns matching items
- Large result sets perform within 200ms (indexed queries)

### Step 6: Evidence Endpoints (`internal/evidence/handler.go`)

**Deliverable:** HTTP handlers for evidence operations.

**Endpoints:**
```
POST   /api/cases/:id/evidence          → Upload evidence (multipart/tus)
GET    /api/cases/:id/evidence           → List evidence in case
GET    /api/evidence/:id                 → Get evidence metadata
GET    /api/evidence/:id/download        → Download evidence file
GET    /api/evidence/:id/thumbnail       → Get thumbnail/preview
GET    /api/evidence/:id/versions        → Get version history
PATCH  /api/evidence/:id                 → Update metadata/tags/classification
POST   /api/evidence/:id/version         → Upload new version
DELETE /api/evidence/:id                 → Destroy evidence (requires authority)
```

**Download flow:**
1. Validate user has case role with download permission
2. Log custody event: `action: "downloaded"`, `file_hash_at_action: current_hash`
3. Stream file from MinIO (never load entire file in memory)
4. Set `Content-Disposition: attachment; filename="sanitized_name"`
5. Set correct `Content-Type`

**Destroy flow (per spec):**
1. Verify user is Case Admin+ or System Admin
2. Verify case does NOT have legal_hold
3. Require `destruction_authority` in request body (court order reference)
4. Delete file from MinIO
5. Set `destroyed_at`, `destroyed_by`, `destruction_authority` on evidence_items row
6. **Do NOT delete the row** — metadata and custody log preserved permanently
7. Log custody event: `action: "destroyed"`, `details: {authority, hash_at_destruction}`

**Update metadata (PATCH):**
- Allowed fields: title, description, tags, classification, source, source_date, metadata
- Each update logged to custody chain with changed fields
- Tags: alphanumeric + hyphens + underscores, max 100 chars each
- Classification: must be valid enum value

**Tests:**
- Upload → 201 with evidence metadata
- List → paginated results with correct filtering
- Get metadata → 200 with full evidence details
- Download → file streams correctly, custody logged
- Thumbnail → 200 with image content
- Version history → returns all versions linked by parent_id
- Update tags → 200, custody logged
- Update classification → 200, custody logged
- Destroy → file removed, row preserved, custody logged
- Destroy with legal_hold → 409 error
- Destroy without authority → 400 error
- Defence download disclosed item → 200
- Defence download non-disclosed item → 403

### Step 7: Evidence Upload Frontend

**Deliverable:** Next.js evidence upload UI with progress tracking.

**Components:**
- `EvidenceUploader` — Drag-and-drop zone + file browser
  - Uses tus-js-client for chunked upload
  - Progress bar per file
  - Resume capability (shows "resuming..." for interrupted uploads)
  - File type icon based on MIME type
  - Estimated time remaining
- `EvidenceForm` — Metadata form for uploaded evidence
  - Title (pre-filled from filename)
  - Description (textarea)
  - Tags (multi-select with autocomplete)
  - Classification (dropdown: public, restricted, confidential, ex_parte)
  - Source (text input)
  - Source date (date picker)
- `EvidenceGrid` — Grid/table view of evidence items
  - Thumbnail column for images
  - Sortable by: evidence_number, title, uploaded_at, source_date, file_size
  - Filterable by: type, tags, classification, date range
  - Pagination (cursor-based, load more button)
  - Click to view details
- `EvidenceDetail` — Full evidence detail view
  - Metadata display
  - **File preview (Phase 1 — per spec):**
    - **Images (JPEG, PNG, TIFF, WebP):** Full-size view in browser via `<img>` tag, served from `/api/evidence/:id/download` with `Content-Type` header
    - **PDFs:** Rendered client-side using `react-pdf` (pdf.js wrapper). No server-side PDF interpretation (prevents PDF-based exploits per spec security note)
    - **Audio (MP3, WAV, FLAC, OGG):** HTML5 `<audio>` player with native browser codecs. No transcoding.
    - **Video (MP4, WebM, MOV):** HTML5 `<video>` player with native browser codecs. No transcoding.
    - **All other files:** Metadata display + file type icon + download button. No preview.
  - Download button (with custody log notice: "Downloading will be logged in the chain of custody")
  - Custody log tab (paginated)
  - Version history tab
  - Hash verification display (SHA-256 hash + TSA timestamp + verification status)
  - EXIF metadata display (GPS map embed if coordinates available, camera info)

**Tests:**
- Upload UI renders correctly
- Progress bar updates during upload
- Form validation (required fields)
- Grid renders with mock data
- Sorting/filtering works
- Pagination loads more items
- Detail view shows all metadata
- Preview renders for different file types
- Download triggers correctly
- All text via useTranslations()

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/evidence/storage.go` | Create | MinIO abstraction |
| `internal/evidence/service.go` | Create | Upload pipeline, business logic |
| `internal/evidence/repository.go` | Create | Postgres queries |
| `internal/evidence/handler.go` | Create | HTTP handlers |
| `internal/evidence/models.go` | Create | EvidenceItem, EvidenceFilter types |
| `internal/evidence/thumbnail.go` | Create | Image thumbnail generation |
| `internal/integrity/tsa.go` | Create | RFC 3161 timestamp client |
| `internal/integrity/tsa_retry.go` | Create | Background TSA retry job (5-min intervals, 24hr window) |
| `internal/evidence/exif.go` | Create | EXIF metadata extraction (GPS, camera, timestamp) |
| `internal/evidence/thumbnail.go` | Create | Image/PDF/video thumbnail generation |
| `web/src/components/evidence/*` | Create | Upload, grid, detail components |
| `web/src/app/[locale]/cases/[id]/evidence/page.tsx` | Create | Evidence list page |

---

## Definition of Done

- [ ] Evidence upload works end-to-end (file → hash → TSA → MinIO → Postgres → custody log)
- [ ] Chunked upload resumes after interruption
- [ ] SHA-256 hash computed and stored correctly
- [ ] RFC 3161 timestamp obtained (or gracefully degraded)
- [ ] TSA retry job runs when initial timestamp fails
- [ ] Files encrypted at rest in MinIO (SSE-S3)
- [ ] Evidence numbers sequential and gap-free
- [ ] EXIF metadata extracted from images
- [ ] Filenames sanitized, originals preserved in metadata
- [ ] Custody log entry for every evidence action
- [ ] Download streams file (no full memory load)
- [ ] Evidence destruction preserves metadata + custody log
- [ ] Legal hold blocks destruction
- [ ] Defence visibility filter working
- [ ] Upload UI with progress bar, drag-and-drop, resume
- [ ] Evidence grid with sort/filter/pagination
- [ ] 100% test coverage on all Go code
- [ ] E2E test: upload → view → download → verify hash

---

## Security Checklist

- [ ] File content never executed server-side
- [ ] Filename sanitized (path traversal, null bytes, control chars)
- [ ] MIME type detected server-side (not trusted from client)
- [ ] File size validated against MAX_UPLOAD_SIZE
- [ ] MinIO SSE-S3 encryption verified on stored objects
- [ ] Download logs custody event before streaming
- [ ] Evidence destruction requires explicit authority string
- [ ] Legal hold enforcement cannot be bypassed
- [ ] Hash computation uses crypto/sha256 (not md5, not sha1)
- [ ] TSA token verified against known CA certificates
- [ ] No file content in application logs
- [ ] Incomplete uploads expire (no disk exhaustion)

---

## Test Coverage Requirements (100% Target)

All new code introduced in Sprint 4 must achieve 100% line coverage. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/evidence/storage.go`**: PutObject stores file, GetObject retrieves with matching content, DeleteObject removes, StatObject returns correct size/content-type/etag, non-existent key returns not found, SSE encryption headers verified, connection failure retries with backoff, bucket not found returns clear error
- **`internal/integrity/tsa.go`**: Valid hash returns timestamp token, token is verifiable, token contains correct hash, TSA unreachable returns nil token without error, TSA error response handled gracefully, timeout triggers retry with backoff, invalid TSA URL fails at startup, VerifyTimestamp with valid token passes, VerifyTimestamp with tampered hash fails, VerifyTimestamp with expired cert handled, TSA disabled skips entirely
- **`internal/integrity/tsa_retry.go`**: Retry job picks up items with NULL tsa_token, retry succeeds on TSA recovery, retry respects 24-hour window, items older than 24h no longer retried, advisory lock prevents duplicate execution, custody log entries created for delayed timestamps and permanent failures
- **Evidence number generator**: First evidence returns 00001, sequential uploads produce sequential numbers, concurrent uploads produce unique numbers (no duplicates), different cases have independent sequences, format matches `{case_ref}-{5-digit-number}`
- **`internal/evidence/service.go`** (upload pipeline): Small file upload completes full flow, SHA-256 hash matches known test vectors, MIME type detected correctly for PDF/JPG/MP4/DOCX, evidence number generated correctly, filename sanitization strips path separators/null bytes/control chars, file exceeding MAX_UPLOAD_SIZE returns 413, zero-byte file rejected, MinIO disk full returns 507
- **`internal/evidence/exif.go`**: JPEG with GPS data extracts lat/lon/altitude correctly, DMS to decimal conversion correct, DateTimeOriginal extracted, camera make/model extracted, image without EXIF returns null metadata, non-image file skipped gracefully, invalid EXIF data handled without crash
- **`internal/evidence/thumbnail.go`**: JPEG thumbnail generated at 300x300 max (aspect ratio preserved), PNG thumbnail generated, PDF first page extracted as thumbnail, video first frame extracted, unsupported format returns placeholder, generation failure is non-blocking
- **`internal/evidence/repository.go`**: CRUD operations, filter by MIME type/tags/classification/date range/search query, defence role returns only disclosed evidence, pagination, hash lookup returns matching items, large result sets within 200ms
- **`internal/evidence/handler.go`**: Upload returns 201 with metadata, list returns paginated results, get metadata returns 200, download streams file and logs custody, thumbnail returns image, version history returns all versions, update tags/classification logs custody, destroy removes file but preserves row, destroy with legal_hold returns 409, destroy without authority returns 400, defence download disclosed returns 200, defence download non-disclosed returns 403

### Integration Tests (with testcontainers)

- **MinIO storage (testcontainers/minio)**: Full PutObject/GetObject roundtrip with SSE-S3 encryption verified, large file (100MB) upload and retrieval, object key path matches `evidence/{case_id}/{evidence_id}/{version}/{filename}`, incomplete multipart uploads expire after 24 hours, bucket lifecycle policy enforced
- **RFC 3161 TSA (testcontainers + mock TSA or FreeTSA)**: Timestamp request with real SHA-256 hash returns valid token, token verification against TSA CA certificates, retry job with mock TSA outage then recovery, timestamp token stored and retrievable from evidence_items
- **Postgres evidence + custody (testcontainers/postgres:16-alpine)**: Evidence item created with all fields, evidence number sequence atomic under 10 concurrent uploads, custody_log entry created with valid hash chain on upload, hash chain intact after 50 sequential uploads, defence visibility filter with disclosure JOIN
- **Full upload pipeline (testcontainers: postgres + minio + meilisearch)**: Upload file end-to-end: file in MinIO, row in Postgres, indexed in Meilisearch, custody_log entry with hash, thumbnail generated, evidence number assigned; chunked tus upload with simulated network interruption and resume

### E2E Automated Tests (Playwright)

- **`tests/e2e/evidence/upload.spec.ts`**: Login as investigator, navigate to case evidence page, drag a JPEG file onto the upload zone, verify progress bar appears and completes, verify evidence metadata form appears (title pre-filled from filename), fill in tags and classification, submit, verify evidence appears in grid with correct thumbnail and evidence number
- **`tests/e2e/evidence/upload-resume.spec.ts`**: Start uploading a large file (>10MB), simulate network interruption (throttle to offline), verify upload pauses with "resuming..." indicator, restore network, verify upload resumes and completes from where it left off
- **`tests/e2e/evidence/verify-hash.spec.ts`**: Upload a known test file, navigate to evidence detail, verify SHA-256 hash displayed matches expected hash, verify TSA timestamp displayed with TSA name and signed time
- **`tests/e2e/evidence/download.spec.ts`**: Navigate to evidence detail, click download button, verify custody log notice dialog ("Downloading will be logged"), confirm download, verify file downloads with correct filename and content, check custody log tab shows "downloaded" entry
- **`tests/e2e/evidence/thumbnails.spec.ts`**: Upload JPEG, PDF, and MP4 files, navigate to evidence grid, verify each file type shows appropriate thumbnail (image preview, PDF first page, video frame), verify non-previewable files show type icon
- **`tests/e2e/evidence/exif-display.spec.ts`**: Upload a JPEG with embedded GPS coordinates, navigate to evidence detail, verify EXIF section shows GPS coordinates, camera model, and capture date, verify GPS map embed renders (if coordinates present)
- **`tests/e2e/evidence/evidence-numbering.spec.ts`**: Upload 3 evidence items sequentially to the same case, verify evidence numbers are sequential (00001, 00002, 00003) with correct case reference prefix
- **`tests/e2e/evidence/chunked-large-file.spec.ts`**: Upload a file >50MB, verify progress bar updates incrementally, verify file completes successfully, verify hash and evidence number assigned correctly

### Coverage Enforcement

CI blocks merge if coverage drops below 100% for new code. Coverage reports generated via `go test -coverprofile=coverage.out` and `go tool cover -func=coverage.out`.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Login as investigator, navigate to a case's evidence tab, drag a 5MB JPEG photo onto the upload zone
   **Expected:** Progress bar appears, file uploads with chunked progress updates, metadata form shown on completion
   **Verify:** Evidence grid shows the new item with a thumbnail, evidence number (e.g., ICC-UKR-2024-00001), and correct file size

2. [ ] **Action:** After uploading, navigate to the evidence detail page and check the SHA-256 hash
   **Expected:** SHA-256 hash displayed in hex format, matching the hash of the original file
   **Verify:** Download the file, compute `sha256sum` locally, compare — hashes must be identical

3. [ ] **Action:** On the evidence detail page, check the TSA timestamp section
   **Expected:** RFC 3161 timestamp displayed with TSA name (e.g., FreeTSA.org) and signed timestamp
   **Verify:** Timestamp is within seconds of upload time; if TSA was unavailable, section shows "Timestamp pending — retry in progress"

4. [ ] **Action:** Click the download button on an evidence item
   **Expected:** Custody log notice dialog appears warning that download will be logged; after confirming, file downloads with original filename
   **Verify:** Navigate to custody log tab, verify "downloaded" entry with your user ID, IP address, and file hash at time of download

5. [ ] **Action:** Upload a JPEG with embedded GPS EXIF data (use a test photo with known coordinates)
   **Expected:** Evidence detail page shows EXIF section with GPS coordinates (decimal degrees), camera make/model, capture date
   **Verify:** GPS coordinates match the known values from the test photo; map embed (if present) pins the correct location

6. [ ] **Action:** Upload a PDF document, then check its thumbnail in the evidence grid
   **Expected:** Grid shows a thumbnail of the PDF's first page
   **Verify:** Thumbnail is recognizable as the first page content; clicking navigates to evidence detail with PDF preview via react-pdf

7. [ ] **Action:** Start uploading a 100MB file, then disconnect network (e.g., turn off Wi-Fi) mid-upload
   **Expected:** Upload pauses with a "Connection lost" or "Resuming..." indicator
   **Verify:** Reconnect network, verify upload resumes from the last completed chunk (not from zero), verify file completes with correct hash

8. [ ] **Action:** Upload 3 files rapidly to the same case in quick succession
   **Expected:** All 3 files receive unique, sequential evidence numbers with no gaps
   **Verify:** Evidence numbers are {case-ref}-00001, {case-ref}-00002, {case-ref}-00003 (or incrementing from the last number in the case)

9. [ ] **Action:** Attempt to upload a file larger than the configured MAX_UPLOAD_SIZE
   **Expected:** Upload rejected with 413 error and clear message about the file size limit
   **Verify:** No partial file stored in MinIO; no evidence_items row created; no custody log entry

10. [ ] **Action:** Attempt to upload a zero-byte file
    **Expected:** Upload rejected with 400 error and message indicating empty files are not accepted
    **Verify:** No row in evidence_items; no file in MinIO

11. [ ] **Action:** As case_admin, set legal hold on a case, then attempt to destroy an evidence item in that case
    **Expected:** Destroy operation returns 409 with message that legal hold prevents destruction
    **Verify:** File still exists in MinIO; evidence_items row unchanged; no "destroyed" custody log entry

12. [ ] **Action:** As case_admin (with legal hold off), destroy an evidence item by providing a destruction authority string (e.g., court order number)
    **Expected:** File removed from MinIO, evidence_items row preserved with destroyed_at, destroyed_by, destruction_authority fields set
    **Verify:** Custody log shows "destroyed" entry with authority reference and hash at time of destruction; attempting to download returns 404/410 with message that evidence was destroyed

13. [ ] **Action:** As a defence role user, navigate to the case evidence grid
    **Expected:** Only evidence items included in disclosures are visible; all other items completely absent
    **Verify:** Count of visible items matches the number of disclosed items; no undisclosed metadata, titles, or evidence numbers leaked

14. [ ] **Action:** Upload a video file (MP4) and an audio file (MP3), check their thumbnails and previews
    **Expected:** Video shows first-frame thumbnail in grid; audio shows waveform/audio icon; detail page shows HTML5 video/audio player respectively
    **Verify:** Video plays in browser; audio plays in browser; no server-side transcoding occurs
