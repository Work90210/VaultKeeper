# Sprint 16: Advanced Semantic Search

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 31-32
**Goal:** Replace keyword-only search with meaning-based semantic search. "Find evidence related to attacks on civilian infrastructure" returns relevant items even without those exact words.

---

## Prerequisites

- Sprint 15 complete (entity extraction, text available for all evidence)
- Ollama operational for embedding generation

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Embedding Generation Service

**Deliverable:** Generate vector embeddings for all evidence text using self-hosted model.

**Model:** `nomic-embed-text` via Ollama (384-dimensional, good multilingual support, runs on CPU).

**Interface:**
```go
type EmbeddingService interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}
```

**Embedding pipeline:**
1. On evidence upload/update → extract text (title + description + transcript + OCR + tags)
2. Chunk long text (max 512 tokens per chunk with overlap)
3. Generate embedding for each chunk via Ollama
4. Store embeddings in `pgvector` extension

**pgvector setup:**
```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE evidence_embeddings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL,
    chunk_index     INT NOT NULL DEFAULT 0,
    chunk_text      TEXT NOT NULL,
    embedding       vector(384) NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(evidence_id, chunk_index)
);

CREATE INDEX idx_embeddings_evidence ON evidence_embeddings(evidence_id);
CREATE INDEX idx_embeddings_vector ON evidence_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

**Tests:**
- Text → embedding of correct dimension (384)
- Same text → same embedding (deterministic)
- Similar texts → high cosine similarity
- Different texts → low cosine similarity
- Long text → chunked correctly with overlap
- Batch generation works
- Empty text → skip (no embedding)

### Step 2: Semantic Search Service

**Deliverable:** Hybrid search combining keyword (Meilisearch) + semantic (pgvector).

**Interface:**
```go
type SemanticSearchService interface {
    Search(ctx context.Context, query SemanticQuery) (SemanticSearchResult, error)
    ReindexEvidence(ctx context.Context, evidenceID uuid.UUID) error
    ReindexCase(ctx context.Context, caseID uuid.UUID) error
}

type SemanticQuery struct {
    Query           string
    CaseID          *uuid.UUID
    Mode            string     // keyword, semantic, hybrid (default)
    SemanticWeight  float64    // 0.0-1.0, default 0.5
    Limit           int
    Filters         SearchFilters
}

type SemanticSearchResult struct {
    Hits            []SemanticHit
    TotalHits       int
    Query           string
    Mode            string
    ProcessingTimeMs int
}

type SemanticHit struct {
    EvidenceID      uuid.UUID
    CaseID          uuid.UUID
    Title           string
    Description     string
    EvidenceNumber  string
    RelevantChunk   string     // most relevant text chunk
    KeywordScore    float64
    SemanticScore   float64
    CombinedScore   float64
    Highlights      map[string][]string
}
```

**Hybrid search algorithm:**
1. Generate embedding for search query
2. Run keyword search via Meilisearch (existing)
3. Run vector similarity search via pgvector (`ORDER BY embedding <=> query_embedding`)
4. Combine results using Reciprocal Rank Fusion (RRF):
   ```
   combined_score = (1 / (k + keyword_rank)) + (1 / (k + semantic_rank))
   where k = 60 (standard RRF constant)
   ```
5. Filter by user's case roles
6. Return top N results

**Why hybrid:** Pure semantic misses exact matches (evidence numbers, names). Pure keyword misses conceptual matches. Hybrid gets both.

**Tests:**
- "attacks on civilian infrastructure" → finds evidence about "bombing of hospital" and "shelling residential areas"
- "ICC-UKR-2024-00001" (exact match) → found via keyword
- "witness testimony about checkpoint" → semantic matches relevant transcripts
- Hybrid scores balance keyword and semantic
- Role-based filtering respected
- Defence → only disclosed evidence in results
- Empty query → returns recent items (no semantic search)
- Non-English query → semantic search works (multilingual embeddings)

### Step 3: Search Endpoint Enhancement

**Modify existing:** `GET /api/search`

**New query params:**
- `mode=hybrid|keyword|semantic` (default: hybrid)
- `semantic_weight=0.5` (0-1, balance between keyword and semantic)

**Backward compatible:** Existing keyword-only queries still work unchanged.

### Step 4: Frontend Search Enhancement

**Modifications to existing search UI:**
- Search mode selector: Keyword | Semantic | Hybrid (default)
- Semantic weight slider (advanced option, hidden by default)
- "Why this result?" expansion showing relevant text chunk + match explanation
- Visual indicator for match type (keyword icon, semantic icon, both)

### Step 5: Background Reindexing Job

**Deliverable:** Reindex all existing evidence with embeddings.

- One-time migration job for existing evidence
- Runs as background task
- Progress tracking
- Idempotent (safe to re-run)
- Configurable batch size

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/ai/embeddings.go` | Create | Embedding generation via Ollama |
| `internal/search/semantic.go` | Create | Semantic + hybrid search |
| `internal/search/handler.go` | Modify | Add semantic search params |
| `migrations/013_pgvector.up.sql` | Create | pgvector extension + embeddings table |
| `web/src/components/search/SearchBar.tsx` | Modify | Add search mode selector |

---

## Definition of Done

- [ ] Semantic search finds conceptually related evidence
- [ ] Hybrid search combines keyword + semantic results
- [ ] "Attacks on civilian infrastructure" finds relevant evidence without exact words
- [ ] Exact keyword queries still work (evidence numbers, names)
- [ ] Embeddings generated on upload + stored in pgvector
- [ ] All existing evidence reindexed with embeddings
- [ ] Search mode selector in UI
- [ ] Role-based filtering respected in semantic search
- [ ] 100% test coverage

---

## Security Checklist

- [ ] Embeddings don't leak information (they represent content, same access rules apply)
- [ ] pgvector queries parameterized (no injection)
- [ ] Semantic search filtered by case roles before returning results
- [ ] Defence users never see embeddings of undisclosed evidence
- [ ] Embedding model on internal network only

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 16 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/ai/embeddings.go`** — `GenerateEmbedding`: returns 384-dimensional vector; same text produces identical embedding (deterministic); empty text returns error (not zero vector)
- **`internal/ai/embeddings.go`** — `BatchGenerateEmbeddings`: processes multiple texts; returns correct count of embeddings; handles mix of valid and empty texts
- **Text chunking** — text under 512 tokens produces single chunk; long text split into correct number of chunks; overlap between chunks is correct (configurable window); chunk boundaries respect sentence boundaries where possible
- **`internal/search/semantic.go`** — `Search` with mode "keyword": delegates to Meilisearch only; no embedding generated for query
- **`internal/search/semantic.go`** — `Search` with mode "semantic": generates query embedding; queries pgvector; does not call Meilisearch
- **`internal/search/semantic.go`** — `Search` with mode "hybrid": calls both Meilisearch and pgvector; combines results via Reciprocal Rank Fusion; RRF scores computed correctly with k=60
- **RRF scoring** — item ranked #1 in both lists gets highest combined score; item in only one list still appears; duplicate items deduplicated by evidence ID
- **Semantic weight** — weight=0.0 returns keyword-only ranking; weight=1.0 returns semantic-only ranking; weight=0.5 balances both
- **Role-based filtering** — defence user query filters out undisclosed evidence before returning results; prosecutor sees all matching evidence
- **`ReindexEvidence`** — generates new embedding and upserts into evidence_embeddings; old embedding replaced; idempotent on re-run
- **`ReindexCase`** — processes all evidence items in case; tracks progress; skips items with no text
- **Migration `013_pgvector.up.sql`** — pgvector extension created; evidence_embeddings table created; ivfflat index created; unique constraint on (evidence_id, chunk_index) enforced

### Integration Tests (testcontainers)

- **Postgres (pgvector) + Ollama embedding pipeline** — insert evidence text, generate embedding, store in pgvector, query by cosine similarity, verify correct result returned
- **Semantic search relevance** — insert 10 evidence items with varied content; query "attacks on civilian infrastructure" returns "bombing of hospital" and "shelling residential areas" in top 3
- **Exact keyword match in hybrid mode** — insert evidence with number "ICC-UKR-2024-00001"; hybrid search for that exact string returns it as top result
- **Hybrid search ranking** — insert items that match keyword-only, semantic-only, and both; verify combined RRF ranking places "both" items highest
- **Reindex job end-to-end** — create case with 20 evidence items, run ReindexCase, verify all 20 have embeddings in database
- **Non-English semantic search** — insert French evidence about "attaques contre les civils"; query in English "attacks against civilians" returns it via semantic match
- **Empty query handling** — empty string query returns recent items without error; does not generate embedding
- **Large batch reindex** — 500 evidence items reindexed without timeout or memory issues

### E2E Automated Tests (Playwright)

- **Search mode selector visible** — navigate to search page, verify Keyword / Semantic / Hybrid toggle is present with Hybrid as default
- **Hybrid search returns results** — enter "attacks on civilian infrastructure" in search bar, verify results appear including items without those exact words
- **Keyword search exact match** — switch to Keyword mode, search for an evidence number, verify exact match appears as top result
- **Semantic search conceptual match** — switch to Semantic mode, search for "witness testimony about checkpoint violence", verify relevant transcripts appear
- **"Why this result?" expansion** — click expansion on a semantic result, verify relevant text chunk and match explanation are displayed
- **Match type indicators** — verify keyword matches show keyword icon, semantic matches show semantic icon, hybrid matches show both icons
- **Semantic weight slider** — open advanced options, adjust semantic weight slider, re-search, verify result ordering changes
- **Reindex trigger** — as admin, navigate to admin panel, trigger embedding reindex for a case, verify progress indicator and completion
- **Defence role filtering** — log in as defence user, perform semantic search, verify no undisclosed evidence appears in results

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Upload 5 evidence items: (a) witness statement about "bombing of the hospital in Bucha", (b) photo titled "destroyed medical facility", (c) report on "shelling of residential areas", (d) unrelated financial document, (e) evidence numbered "ICC-UKR-2024-00001"
   **Expected:** All items indexed with embeddings within 30 seconds of upload
   **Verify:** Check evidence_embeddings table has entries for all 5 items; embedding dimensions are 384

2. [ ] **Action:** In Hybrid mode (default), search for "attacks on civilian infrastructure"
   **Expected:** Items (a), (b), and (c) appear in top results despite not containing those exact words; item (d) does not appear in top results
   **Verify:** Review each result's relevance; confirm the "Why this result?" expansion shows meaningful text chunks explaining the match

3. [ ] **Action:** Switch to Keyword mode and search for "ICC-UKR-2024-00001"
   **Expected:** Item (e) appears as the top result via exact keyword match
   **Verify:** Result is ranked #1; no semantic-only matches clutter the results

4. [ ] **Action:** Switch to Semantic mode and search for "destruction of medical facilities during armed conflict"
   **Expected:** Items (a) and (b) appear as top results based on meaning, even though neither contains the phrase "medical facilities during armed conflict"
   **Verify:** Results are conceptually relevant; financial document (d) does not appear

5. [ ] **Action:** Open advanced search options and adjust the semantic weight slider from 0.5 to 0.9, then re-run the hybrid search from step 2
   **Expected:** Result ordering shifts to favor semantically similar items over keyword matches
   **Verify:** Compare result order with default weight; ordering has visibly changed

6. [ ] **Action:** Upload evidence in French: a report titled "Rapport sur les bombardements de Marioupol"
   **Expected:** Semantic search in English for "Mariupol bombings report" returns this French evidence item
   **Verify:** The French item appears in results; the "Why this result?" chunk shows the relevant French text

7. [ ] **Action:** As System Admin, navigate to the admin panel and trigger a full embedding reindex for a case with 50+ evidence items
   **Expected:** Reindex job starts with progress indicator; completes without error; all items have updated embeddings
   **Verify:** Progress bar reaches 100%; evidence_embeddings table shows updated timestamps; search results remain accurate after reindex

8. [ ] **Action:** Log in as a Defence user and perform a semantic search for a term known to appear in both disclosed and undisclosed evidence
   **Expected:** Only evidence items from disclosed evidence appear in results; undisclosed items are completely absent
   **Verify:** Count results visible to defence vs prosecutor for the same query; defence sees fewer or equal results; no undisclosed evidence titles or snippets leak

9. [ ] **Action:** Perform a search with an empty query string
   **Expected:** Recent evidence items are returned (no error, no crash); no embedding is generated for empty input
   **Verify:** Results appear ordered by recency; no 500 error in browser console or server logs

10. [ ] **Action:** Search for a term, then immediately search for a different term before the first search completes
    **Expected:** Second search cancels or supersedes the first; only second search results are displayed
    **Verify:** No stale results from the first search flash on screen; UI shows correct results for the second query
