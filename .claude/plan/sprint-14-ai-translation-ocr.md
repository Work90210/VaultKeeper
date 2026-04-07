# Sprint 14: AI Translation & Multi-Language OCR

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 27-28
**Goal:** Add self-hosted machine translation (via Ollama/Mistral) and OCR for scanned documents. All processing on-premises.

---

## Prerequisites

- Sprint 13 complete (Whisper transcription operational)
- Docker Compose with AI service pattern established

---

## Task Type

- [x] Backend (Go)
- [x] Infrastructure (Docker — Ollama + Tesseract)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Ollama Service in Docker Compose

**Deliverable:** Self-hosted Ollama for LLM inference (translation).

```yaml
ollama:
    image: ollama/ollama:latest
    volumes:
        - ollama_models:/root/.ollama
    networks:
        - vaultkeeper
    deploy:
        resources:
            limits:
                memory: 16G
                cpus: "8"
    healthcheck:
        test: ["CMD", "curl", "-f", "http://localhost:11434/api/tags"]
```

**Model:** Mistral 7B or similar multilingual model. Pulled on first startup.

### Step 2: Translation Service (`internal/ai/translation.go`)

**Interface:**
```go
type TranslationService interface {
    QueueTranslation(ctx context.Context, input TranslationInput) (JobID, error)
    GetTranslation(ctx context.Context, evidenceID uuid.UUID, targetLang string) (Translation, error)
    GetTranslationStatus(ctx context.Context, jobID JobID) (TranslationJob, error)
    DetectLanguage(ctx context.Context, text string) (string, float64, error)
}

type TranslationInput struct {
    EvidenceID  uuid.UUID
    SourceText  string    // or extracted from transcript/OCR
    SourceLang  string    // auto-detected if empty
    TargetLang  string    // ISO 639-1 code
}

type Translation struct {
    EvidenceID  uuid.UUID
    SourceLang  string
    TargetLang  string
    SourceText  string
    TranslatedText string
    Segments    []TranslatedSegment  // for transcripts with timestamps
    ModelVersion string
    Confidence  float64
    CreatedAt   time.Time
}
```

**Translation flow:**
1. User selects evidence item → "Translate to [language]"
2. Source text from: transcript (if audio/video), OCR result (if scanned), or manual input
3. Queue translation job
4. Worker sends text to Ollama in chunks (respect context window)
5. Store translation in `translations` table
6. Index translated text in Meilisearch (searchable in target language)
7. Custody log: `action: "translated"`, details include source/target languages

**New table:**
```sql
CREATE TABLE translations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL,
    source_lang     TEXT NOT NULL,
    target_lang     TEXT NOT NULL,
    source_text     TEXT NOT NULL,
    translated_text TEXT NOT NULL,
    segments        JSONB,
    model_version   TEXT NOT NULL,
    confidence      FLOAT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(evidence_id, target_lang)
);
```

**Supported languages (priority based on ICC needs):**
Arabic, French, English, Ukrainian, Russian, Lingala, Swahili, Spanish, Chinese, German + all major languages supported by the model.

**Tests:**
- French text → English translation correct
- Arabic text → English translation correct
- Language auto-detection works
- Translation of transcript preserves segment timestamps
- Long text chunked correctly (no truncation)
- Ollama down → job queued, retried
- Translation indexed in Meilisearch
- Same evidence + same target language → update existing translation
- Unsupported language → clear error

### Step 3: OCR Service (`internal/ai/ocr.go`)

**Deliverable:** OCR for scanned documents and handwritten notes.

**Approach:** Tesseract OCR (self-hosted, supports 100+ languages).

**Docker addition:**
```yaml
# Tesseract runs as part of the Go API server (library, not separate service)
# Add tesseract-ocr + language packs to Dockerfile
```

**Interface:**
```go
type OCRService interface {
    QueueOCR(ctx context.Context, evidenceID uuid.UUID, languages []string) (JobID, error)
    GetOCRResult(ctx context.Context, evidenceID uuid.UUID) (OCRResult, error)
    GetOCRStatus(ctx context.Context, jobID JobID) (OCRJob, error)
}

type OCRResult struct {
    EvidenceID uuid.UUID
    Pages      []OCRPage
    FullText   string
    Languages  []string
    Confidence float64
    CreatedAt  time.Time
}

type OCRPage struct {
    PageNumber int
    Text       string
    Blocks     []OCRBlock  // position-aware text blocks
    Confidence float64
}
```

**OCR flow:**
1. Evidence uploaded (image or PDF with scanned content detected)
2. Auto-queue OCR job (or manual trigger)
3. For PDFs: extract each page as image
4. For images: process directly
5. Run Tesseract OCR with specified languages
6. Store result in `ocr_results` table
7. Index extracted text in Meilisearch
8. Custody log: `action: "ocr_processed"`

**New table:**
```sql
CREATE TABLE ocr_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL UNIQUE,
    full_text       TEXT NOT NULL,
    pages           JSONB NOT NULL,
    languages       TEXT[] NOT NULL,
    confidence      FLOAT,
    engine_version  TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

**Tests:**
- Scanned PDF → text extracted correctly
- Handwritten text (clear) → reasonable extraction
- Multi-language document → both languages processed
- Multi-page PDF → each page processed
- Image (JPEG, PNG, TIFF) → text extracted
- Non-text image (photo) → low confidence, minimal text
- OCR result indexed and searchable
- Page-level text blocks have correct positions

### Step 4: Translation & OCR Frontend

**Components:**
- `TranslationPanel` — Side panel showing translation
  - Source text on left, translation on right
  - Language selector for target language
  - "Translate" button
  - Copy translation
  - For transcripts: synced segment view (source + translation per segment)
- `OCRViewer` — OCR result overlay on document
  - Show extracted text alongside original document/image
  - Page-by-page navigation for multi-page documents
  - Copy text
  - Confidence indicator
- `AIProcessingBadge` — Icon showing AI processing status
  - Transcription, Translation, OCR status indicators
  - Click to view results

### Step 5: Unified AI Job Queue

**Deliverable:** Shared job queue for all AI tasks (transcription, translation, OCR).

**Refactor the transcription job queue (Sprint 13) into a generic AI job queue:**
```go
type AIJobQueue interface {
    Enqueue(ctx context.Context, job AIJob) (JobID, error)
    GetStatus(ctx context.Context, jobID JobID) (AIJobStatus, error)
    Cancel(ctx context.Context, jobID JobID) error
}

type AIJob struct {
    Type       string  // transcription, translation, ocr
    EvidenceID uuid.UUID
    Params     map[string]string
    Priority   int     // higher = processed first
}
```

**Worker pool:** Configurable per job type (e.g., 1 transcription worker, 2 translation workers, 2 OCR workers).

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/ai/translation.go` | Create | Translation service |
| `internal/ai/ollama_client.go` | Create | Ollama HTTP client |
| `internal/ai/ocr.go` | Create | OCR service (Tesseract) |
| `internal/ai/jobs.go` | Modify | Unified AI job queue |
| `migrations/011_translations_ocr.up.sql` | Create | Translations + OCR tables |
| `docker-compose.yml` | Modify | Add Ollama service |
| `Dockerfile` | Modify | Add Tesseract OCR packages |
| `web/src/components/evidence/TranslationPanel.tsx` | Create | Translation UI |
| `web/src/components/evidence/OCRViewer.tsx` | Create | OCR result viewer |

---

## Definition of Done

- [ ] Translation works for major languages (FR→EN, AR→EN tested)
- [ ] OCR extracts text from scanned PDFs and images
- [ ] Both indexed in Meilisearch (searchable)
- [ ] All processing on-premises (Ollama + Tesseract, no external APIs)
- [ ] Unified AI job queue with retry and priority
- [ ] Translation preserves transcript segment timestamps
- [ ] OCR provides page-level and position-aware text blocks
- [ ] 100% test coverage

---

## Security Checklist

- [ ] Ollama on internal network only
- [ ] No data leaves the server for AI processing
- [ ] Translation/OCR results subject to same access controls
- [ ] AI model downloads verified (checksums)
- [ ] Resource limits prevent AI services from starving other services

---

## Test Coverage Requirements (100% Target)

Every line of new code in Sprint 14 must be covered. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- `translation.QueueTranslation` — creates job with correct source/target languages, rejects unsupported target language with clear error, auto-detects source language when not specified
- `translation.QueueTranslation` — sources text from transcript (audio/video), OCR result (scanned document), or manual input
- `translation.ProcessTranslation` — sends text to Ollama in chunks (respects context window), reassembles translated chunks
- `translation.ProcessTranslation` — preserves transcript segment timestamps in translated output
- `translation.ProcessTranslation` — creates custody log entry with "translated" action, source/target languages
- `translation.ProcessTranslation` — retries on Ollama failure (3 attempts, exponential backoff), marks failed after retries
- `translation.GetTranslation` — returns source text, translated text, confidence, model version
- `translation.GetTranslation` — same evidence + same target language returns existing translation (upsert behavior)
- `translation.DetectLanguage` — correctly identifies French, Arabic, Ukrainian, Russian, English
- `translation.IndexTranslation` — translated text indexed in Meilisearch, searchable in target language
- `ollama_client.Translate` — sends prompt to Ollama HTTP API, parses response, handles timeout/connection errors
- `ollama_client.ChunkText` — splits text at sentence boundaries respecting context window, no truncation
- `ocr.QueueOCR` — creates job for image or scanned PDF, rejects non-image/non-PDF MIME types
- `ocr.ProcessOCR` — extracts text from image via Tesseract, returns page-level and block-level results
- `ocr.ProcessOCR` — multi-page PDF: extracts each page as image, OCRs each, combines results
- `ocr.ProcessOCR` — multi-language support: processes with specified language packs
- `ocr.ProcessOCR` — creates custody log entry with "ocr_processed" action
- `ocr.GetOCRResult` — returns full_text, pages with page-level text and confidence, block positions
- `ocr.IndexOCRResult` — OCR text indexed in Meilisearch, searchable
- `aijobs.UnifiedQueue` — accepts transcription, translation, and OCR job types, routes to correct worker pool
- `aijobs.UnifiedQueue` — priority ordering (higher priority processed first within same type)
- `aijobs.UnifiedQueue` — configurable worker count per job type (1 transcription, 2 translation, 2 OCR)
- `aijobs.Cancel` — cancels pending job, cannot cancel completed job

### Integration Tests (testcontainers)

- French-to-English translation: upload evidence with French transcript, queue translation to English — translated text is coherent English, segments preserve timestamps, custody log entry created, translation indexed in Meilisearch
- Arabic-to-English translation: upload evidence with Arabic text (via OCR or transcript) — translated text is coherent English
- Language auto-detection: provide text without specifying source language — service correctly detects language, stores in source_lang field
- Translation of transcript with timestamps: transcribe audio (Sprint 13), translate transcript — each translated segment retains original start/end timestamps, synced playback still works
- Long text chunking: translate a 50,000-word document — all chunks translated without truncation, reassembled translation is complete and coherent
- Translation upsert: translate evidence to English, translate same evidence to English again — existing translation updated (not duplicated), UNIQUE constraint on (evidence_id, target_lang) respected
- OCR scanned PDF: upload scanned PDF (3 pages with typed text), queue OCR — full_text contains text from all 3 pages, page-level results correct, confidence > 0.8 for typed text
- OCR handwritten text: upload image with clear handwriting — text extracted with reasonable accuracy, confidence lower than typed text
- OCR multi-language document: upload document with French and Arabic text, specify both language packs — both languages extracted
- OCR image formats: test JPEG, PNG, TIFF inputs — all produce valid OCR results
- OCR search integration: OCR a scanned document, search for a phrase from the extracted text in Meilisearch — evidence item returned in results
- Translation of OCR result: OCR a scanned French document, translate OCR text to English — translation uses OCR-extracted text as source, result searchable in English
- Unified job queue: enqueue transcription, translation, and OCR jobs simultaneously — each routed to correct worker pool, processed with correct concurrency limits, no cross-contamination
- Ollama service recovery: queue translation, stop Ollama container — job retries, restart Ollama — job completes on retry

### E2E Automated Tests (Playwright)

- Translation panel: navigate to evidence with transcript, click "Translate to English" — TranslationPanel opens showing source text on left, translation on right after job completes, language selector works, copy button copies translated text
- Translation of transcript with sync: translate a transcribed audio/video, open TranslationPanel — source and translated segments displayed side by side, click translated segment — media seeks to corresponding timestamp
- Translation search: translate evidence text from French to English, navigate to global search, search for an English phrase from the translation — evidence item appears in results
- OCR viewer: upload scanned PDF, wait for OCR to complete, click evidence item — OCRViewer shows extracted text alongside original document, page navigation for multi-page documents, copy text button works, confidence indicator displayed
- OCR search: OCR a scanned document, search for a word from the extracted text — evidence item appears in search results
- AI processing badges: upload audio (transcription queued), upload scanned PDF (OCR queued), translate evidence (translation queued) — each item shows appropriate AIProcessingBadge with correct status indicators, click badge to view results
- Translation language selection: open TranslationPanel, select different target languages from dropdown (French, Arabic, English) — each triggers a translation job, results appear when complete
- OCR multi-page navigation: OCR a 5-page scanned PDF, open OCRViewer — page selector shows pages 1-5, navigate between pages, each page shows its extracted text alongside the scanned image

**CI enforcement:** CI blocks merge if coverage drops below 100% for new code.

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Navigate to an evidence item that has a transcript (from Sprint 13). Click "Translate" and select "English" as the target language.
   **Expected:** A translation job is queued. The TranslationPanel opens with a loading indicator. After the job completes, the source text appears on the left and the English translation on the right.
   **Verify:** The translation is coherent and accurate. The source language is correctly detected and displayed. Custody chain shows a "translated" event.

2. [ ] **Action:** View the translation of a transcribed audio/video item. The TranslationPanel should show segments with timestamps.
   **Expected:** Each source segment is paired with its translated segment. Timestamps are preserved from the original transcript. Clicking a translated segment seeks the media player to that timestamp.
   **Verify:** Play the audio/video, follow along in the translated text — the translation corresponds to what is being spoken. Timestamps align.

3. [ ] **Action:** Navigate to global search. Search for a distinctive phrase that only exists in a translated text (not in the original language or any other field).
   **Expected:** The search results include the evidence item whose translation contains the phrase. A snippet of the translated text is shown with the match highlighted.
   **Verify:** Click the result — navigates to evidence detail, TranslationPanel shows the relevant translated text.

4. [ ] **Action:** Upload a scanned PDF document (3 pages of typed text in English). Wait for OCR processing to complete.
   **Expected:** OCR job is queued automatically (or triggered manually). After completion, the evidence item shows an OCR badge. Click the item — OCRViewer appears showing extracted text alongside the scanned document.
   **Verify:** Read through the extracted text — it accurately matches the typed text on each page. Confidence indicator shows high confidence (>80%). Page navigation works correctly for all 3 pages.

5. [ ] **Action:** Upload a scanned document with handwritten text (clear handwriting).
   **Expected:** OCR processes the document. Extracted text is displayed in OCRViewer. Confidence indicator is lower than for typed text.
   **Verify:** The extracted text is reasonably accurate for clear handwriting. Some errors are expected and reflected in the lower confidence score.

6. [ ] **Action:** Upload a scanned document containing text in two languages (e.g., French and Arabic). Trigger OCR with both language packs specified.
   **Expected:** OCR processes both languages. Extracted text contains both French and Arabic text in correct order.
   **Verify:** The French portions are accurate. The Arabic portions are extracted (may have lower accuracy depending on script). Both languages are searchable in Meilisearch.

7. [ ] **Action:** OCR a scanned French document, then translate the OCR result to English.
   **Expected:** Translation uses the OCR-extracted French text as its source. English translation appears in TranslationPanel with OCR text on the left and English on the right.
   **Verify:** The English translation is coherent and corresponds to the French document content. Both OCR text (French) and translation (English) are searchable.

8. [ ] **Action:** Copy the translated text from the TranslationPanel using the copy button.
   **Expected:** Translated text is copied to the clipboard. Paste it elsewhere — the full translated text appears.
   **Verify:** The copied text matches what is displayed in the panel, including any formatting or segment separators.

9. [ ] **Action:** Navigate to the evidence grid. Observe the AIProcessingBadge indicators on items that have been transcribed, translated, or OCR-processed.
   **Expected:** Each badge shows the correct status: transcription (audio icon), translation (globe icon), OCR (document scan icon). Completed tasks show green indicators. Click a badge to view the corresponding result.
   **Verify:** Clicking the transcription badge opens TranscriptViewer. Clicking the translation badge opens TranslationPanel. Clicking the OCR badge opens OCRViewer.

10. [ ] **Action:** Stop the Ollama Docker container. Queue a translation job. Wait 2 minutes. Restart Ollama.
    **Expected:** Translation job fails initially and retries. After Ollama restarts, the retry succeeds. Translation completes with valid result.
    **Verify:** Job status history shows retry attempts. Final status is "completed." Translation text is coherent and indexed in Meilisearch.
