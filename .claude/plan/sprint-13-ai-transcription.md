# Sprint 13: AI Transcription (Whisper)

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 25-26
**Goal:** Integrate self-hosted Whisper for audio/video transcription. All processing on-premises — no data leaves the server. Transcripts become searchable evidence items linked to the original media.

---

## Prerequisites

- Phase 2 (v2.0.0) complete
- Docker Compose stack operational
- Meilisearch indexing operational

---

## Task Type

- [x] Backend (Go)
- [x] Infrastructure (Docker — Whisper service)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Whisper Service in Docker Compose

**Deliverable:** Self-hosted Whisper inference server as a Docker service.

**Options evaluated:**
- `openai/whisper` via Python HTTP wrapper → simplest
- `ggerganov/whisper.cpp` via HTTP API → faster, lower resource usage
- `fedirz/faster-whisper-server` → production-ready HTTP API

**Recommended:** `faster-whisper-server` — production HTTP API, GPU optional, good language support.

**Docker Compose addition:**
```yaml
whisper:
    image: fedirz/faster-whisper-server:latest
    environment:
        - WHISPER_MODEL=large-v3
        - DEVICE=cpu  # or cuda for GPU instances
    volumes:
        - whisper_models:/root/.cache/huggingface
    networks:
        - vaultkeeper
    deploy:
        resources:
            limits:
                memory: 8G
                cpus: "4"
    healthcheck:
        test: ["CMD", "curl", "-f", "http://localhost:8000/health"]
        interval: 30s
```

**Network isolation:** Whisper service on internal network only, no public exposure.

**Tests:**
- Whisper service starts and passes health check
- Model downloads on first run (cached in volume)
- Service responds to transcription requests
- Resource limits respected

### Step 2: Transcription Service (`internal/ai/transcription.go`)

**Deliverable:** Go service that queues and processes transcription jobs.

**Interface:**
```go
type TranscriptionService interface {
    QueueTranscription(ctx context.Context, evidenceID uuid.UUID) (JobID, error)
    GetTranscriptionStatus(ctx context.Context, jobID JobID) (TranscriptionJob, error)
    GetTranscript(ctx context.Context, evidenceID uuid.UUID) (Transcript, error)
}

type Transcript struct {
    EvidenceID uuid.UUID
    Language   string
    Segments   []TranscriptSegment
    FullText   string
    Duration   time.Duration
    WordCount  int
    CreatedAt  time.Time
}

type TranscriptSegment struct {
    Start    time.Duration  // segment start time
    End      time.Duration  // segment end time
    Text     string
    Language string          // detected language for this segment
}
```

**Transcription flow:**
1. Evidence uploaded (audio/video MIME type detected)
2. Auto-queue transcription job (configurable: auto or manual)
3. Background worker picks up job
4. Extract audio from video if needed (ffmpeg)
5. Send audio to Whisper service API
6. Receive timestamped segments
7. Store transcript in new `transcripts` table
8. Index transcript text in Meilisearch (linked to evidence item)
9. Custody log: `action: "transcribed"`, details include language, duration
10. Notification: "Transcription complete for [evidence_number]"

**New table:**
```sql
CREATE TABLE transcripts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL UNIQUE,
    language        TEXT NOT NULL,
    full_text       TEXT NOT NULL,
    segments        JSONB NOT NULL,  -- array of {start, end, text, language}
    word_count      INT NOT NULL,
    duration_ms     BIGINT NOT NULL,
    model_version   TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_transcripts_evidence ON transcripts(evidence_id);
```

**Job queue:**
- Simple Postgres-based queue (no external queue service needed)
- Jobs table: id, evidence_id, status (pending/processing/completed/failed), created_at, started_at, completed_at, error
- Worker goroutine pool (configurable concurrency, default 1 — transcription is CPU-intensive)
- Retry on failure: 3 attempts with exponential backoff
- Failed after retries → notification to admin

**Supported formats:** MP3, WAV, M4A, FLAC, OGG, MP4, AVI, MKV, MOV, WEBM
**Language detection:** Automatic (Whisper detects language per segment)
**Max duration:** Configurable (default 4 hours)

**Tests:**
- Audio file → transcript with correct text
- Video file → audio extracted, transcript generated
- Timestamped segments match audio timing
- Multi-language audio → languages detected per segment
- Long file (2 hours) → processes without timeout
- Whisper service down → job queued, retried later
- Transcript indexed in Meilisearch (searchable)
- Custody log entry created
- Non-audio/video file → transcription not queued
- Concurrent jobs processed by worker pool
- Job status transitions correct

### Step 3: Transcription Endpoints

```
POST   /api/evidence/:id/transcribe       → Queue transcription job
GET    /api/evidence/:id/transcription     → Get transcript
GET    /api/evidence/:id/transcription/status → Job status
DELETE /api/evidence/:id/transcription     → Delete transcript (re-transcribe)
```

### Step 4: Transcription Frontend

**Components:**
- `TranscriptViewer` — Display transcript alongside media player
  - Synced scrolling: transcript highlights current segment as media plays
  - Click segment → media jumps to that timestamp
  - Search within transcript
  - Copy transcript text
  - Export transcript as TXT, SRT (subtitles), or PDF
- `TranscriptionStatus` — Badge on evidence grid
  - Pending (yellow), Processing (blue spinner), Complete (green), Failed (red)
- `TranscriptionSettings` — Admin toggle for auto-transcription

**Tests:**
- Transcript renders with segments
- Click segment → media seeks to timestamp
- Search highlights matching segments
- Export as TXT/SRT works
- Status badge updates correctly

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/ai/transcription.go` | Create | Transcription service |
| `internal/ai/whisper_client.go` | Create | HTTP client for Whisper API |
| `internal/ai/jobs.go` | Create | Background job queue |
| `migrations/010_transcripts.up.sql` | Create | Transcripts + jobs tables |
| `docker-compose.yml` | Modify | Add whisper service |
| `web/src/components/evidence/TranscriptViewer.tsx` | Create | Synced transcript UI |

---

## Definition of Done

- [ ] Whisper runs self-hosted in Docker (no external API calls)
- [ ] Audio/video transcribed with timestamped segments
- [ ] Multi-language detection works
- [ ] Transcripts searchable via Meilisearch
- [ ] Transcript viewer syncs with media player
- [ ] Export as TXT, SRT, PDF
- [ ] Job queue with retry handles Whisper downtime
- [ ] All processing on-premises (zero outbound data)
- [ ] 100% test coverage on transcription service

---

## Security Checklist

- [ ] Whisper service on internal network only (no public access)
- [ ] Audio/video data never leaves the server
- [ ] Transcript content subject to same access controls as evidence
- [ ] Defence users only see transcripts of disclosed evidence
- [ ] No telemetry from Whisper model (air-gap compatible)

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 13 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `transcription.QueueTranscription` — creates job record with status "pending," rejects non-audio/video MIME types, rejects evidence IDs that don't exist
- `transcription.QueueTranscription` — rejects files exceeding max duration (configurable, default 4 hours)
- `transcription.ProcessJob` — extracts audio from video via ffmpeg, sends to Whisper API, stores transcript with segments
- `transcription.ProcessJob` — creates custody log entry with "transcribed" action, language, and duration
- `transcription.ProcessJob` — retries on Whisper failure (3 attempts, exponential backoff), marks failed after retries, sends admin notification
- `transcription.GetTranscript` — returns full transcript with segments, word count, duration
- `transcription.GetTranscriptionStatus` — returns correct status at each stage (pending/processing/completed/failed)
- `whisper_client.Transcribe` — sends audio to Whisper HTTP API, parses timestamped segment response
- `whisper_client.Transcribe` — handles Whisper service timeout, connection refused, malformed response
- `jobs.Enqueue` — creates job in Postgres queue, respects concurrency limit
- `jobs.WorkerPool` — processes jobs concurrently up to configured limit (default 1), picks up pending jobs in FIFO order
- `jobs.WorkerPool` — handles server restart (pending/processing jobs picked up on restart)
- `transcript.IndexInMeilisearch` — transcript full_text indexed, linked to evidence_id, searchable
- `transcript.Segments` — each segment has start time, end time, text, and detected language
- `transcript.ExportAsTXT` — plain text output with timestamps
- `transcript.ExportAsSRT` — valid SRT subtitle format with sequential numbering and time codes
- `transcript.ExportAsPDF` — PDF with timestamped segments, evidence metadata header
- Supported format validation: MP3, WAV, M4A, FLAC, OGG, MP4, AVI, MKV, MOV, WEBM all accepted; JPEG, PDF, DOCX rejected

### Integration Tests (testcontainers)

- Full transcription pipeline: upload audio file (MP3 with known speech content), queue transcription, wait for completion — transcript contains expected text, segments have correct timestamps, custody log entry created, transcript indexed in Meilisearch
- Video transcription: upload MP4 video, queue transcription — audio extracted via ffmpeg, transcript generated, segments match audio timing
- Multi-language detection: upload audio with French and English speech segments — transcript segments have correct per-segment language labels
- Long file processing: upload 2-hour audio file, queue transcription — processes without timeout, transcript complete, all segments present
- Whisper service recovery: queue transcription, stop Whisper container, verify job retries — restart Whisper container, job completes on retry
- Search integration: transcribe audio, search for a phrase from the transcript in Meilisearch — evidence item returned in search results
- Concurrent jobs: queue 3 transcription jobs simultaneously with worker pool size 1 — processed sequentially, all complete, no resource contention
- Job status transitions: queue job, verify status "pending" → poll during processing, verify "processing" → wait for completion, verify "completed" with transcript data
- Transcript deletion and re-transcription: delete transcript, re-queue transcription — new transcript generated, old one removed from Meilisearch
- Access control: transcribe disclosed evidence, query as defence — transcript visible; transcribe non-disclosed evidence, query as defence — transcript not visible

### E2E Automated Tests (Playwright)

- Upload audio and trigger transcription: upload MP3 file to case, verify TranscriptionStatus badge shows "Pending" (yellow), wait for processing — badge changes to "Processing" (blue spinner), wait for completion — badge changes to "Complete" (green)
- Transcript viewer: click on transcribed audio evidence, verify TranscriptViewer panel appears alongside media player, transcript shows timestamped segments, current segment highlighted as audio plays
- Synced playback: play audio, verify transcript auto-scrolls to highlight the current segment; click a segment at timestamp 1:30 — verify media player seeks to 1:30 and plays from that point
- Search within transcript: use the transcript search box, type a phrase — matching segments highlighted, click a highlighted segment — media seeks to that timestamp
- Transcript export: click export menu, select TXT — text file downloads with timestamps; select SRT — subtitle file downloads; select PDF — PDF with formatted transcript downloads
- Meilisearch integration: transcribe audio evidence, navigate to global search, search for a phrase from the transcript — evidence item appears in search results with transcript snippet
- Failed transcription: upload a file that will cause Whisper to fail (e.g., silent/empty audio), verify badge changes to "Failed" (red), verify error details accessible
- Auto-transcription toggle: navigate to admin settings, toggle auto-transcription off, upload new audio — verify transcription is NOT auto-queued; toggle back on — verify new uploads auto-queue

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Upload an MP3 audio file (5-10 minutes, with clear speech in English) to a case as evidence.
   **Expected:** File uploads successfully. A transcription job is automatically queued (if auto-transcription is enabled). The evidence item shows a yellow "Pending" badge.
   **Verify:** Check the transcription status endpoint — job exists with status "pending." Evidence grid shows the transcription badge.

2. [ ] **Action:** Wait for the transcription to complete (monitor the status badge).
   **Expected:** Badge transitions from yellow "Pending" to blue "Processing" (with spinner), then to green "Complete." A notification appears: "Transcription complete for [evidence_number]."
   **Verify:** Click the evidence item — TranscriptViewer panel appears with the full transcript. Segments have timestamps. Word count and duration are displayed.

3. [ ] **Action:** Play the audio in the media player while viewing the transcript.
   **Expected:** As audio plays, the transcript auto-scrolls and highlights the current segment. The highlighted segment matches what is being spoken.
   **Verify:** Pause at several points — the highlighted segment matches the audible speech. The sync is within 1-2 seconds of accuracy.

4. [ ] **Action:** Click on a transcript segment that starts at approximately 2:15.
   **Expected:** The media player seeks to 2:15 and begins playing from that point. The transcript highlights the clicked segment.
   **Verify:** The audio output matches the text of the clicked segment.

5. [ ] **Action:** Use the transcript search box to search for a specific word or phrase that appears in the transcript.
   **Expected:** Matching segments are highlighted in the transcript. A count shows "N matches found."
   **Verify:** Click each highlighted match — media player seeks to the corresponding timestamp. Each match is a valid occurrence of the search term.

6. [ ] **Action:** Export the transcript as TXT, SRT, and PDF (one at a time).
   **Expected:** TXT file contains plain text with timestamps (e.g., "[00:01:30] The witness stated..."). SRT file is a valid subtitle file with sequential numbering and timecodes. PDF contains formatted transcript with evidence metadata header.
   **Verify:** Open each file — content matches the on-screen transcript. SRT file can be loaded as subtitles in a video player.

7. [ ] **Action:** Upload a video file (MP4, 3-5 minutes with speech). Wait for transcription to complete.
   **Expected:** Audio is extracted from the video automatically. Transcription completes. Transcript viewer appears alongside the video player.
   **Verify:** Play the video — transcript syncs with the video playback. Segments match the spoken audio in the video.

8. [ ] **Action:** Upload an audio file with speech in two languages (e.g., French and English segments).
   **Expected:** Transcription completes. Each segment has a detected language label. French segments labeled "fr," English segments labeled "en."
   **Verify:** Read through the segments — language labels are correct for each segment. Both languages are transcribed accurately.

9. [ ] **Action:** Navigate to the global search. Search for a distinctive phrase that only appears in a transcript (not in any evidence title or description).
   **Expected:** The search results include the evidence item whose transcript contains the phrase. The result shows a transcript snippet with the matching text highlighted.
   **Verify:** Click the search result — navigates to the evidence detail with the TranscriptViewer showing the matching segment.

10. [ ] **Action:** Stop the Whisper Docker container (`docker stop whisper`). Upload a new audio file. Wait 2 minutes. Restart the Whisper container (`docker start whisper`).
    **Expected:** Transcription job is queued but fails initially (Whisper unavailable). Job retries automatically. After Whisper restarts, the retry succeeds and transcription completes.
    **Verify:** Check job status history — shows retry attempts. Final status is "completed" with valid transcript. No data loss or corruption.
