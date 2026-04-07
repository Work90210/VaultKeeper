# Sprint 15: Entity Extraction & Knowledge Graph

**Phase:** 3 — AI & Advanced Features
**Duration:** Weeks 29-30
**Goal:** AI-powered entity extraction (names, locations, dates, organizations) from evidence text, displayed as a per-case knowledge graph. "Show me every piece of evidence that mentions Colonel X in Bunia between March and June 2024."

---

## Prerequisites

- Sprint 14 complete (translation, OCR — text available for all evidence types)
- Ollama operational for LLM inference

---

## Task Type

- [x] Backend (Go)
- [x] Frontend (Next.js)

---

## Implementation Steps

### Step 1: Entity Extraction Service (`internal/ai/entities.go`)

**Interface:**
```go
type EntityExtractionService interface {
    QueueExtraction(ctx context.Context, evidenceID uuid.UUID) (JobID, error)
    GetEntities(ctx context.Context, evidenceID uuid.UUID) ([]Entity, error)
    SearchEntities(ctx context.Context, caseID uuid.UUID, query EntityQuery) ([]EntitySearchResult, error)
    GetEntityGraph(ctx context.Context, caseID uuid.UUID, filters EntityFilter) (EntityGraph, error)
}

type Entity struct {
    ID           uuid.UUID
    EvidenceID   uuid.UUID
    CaseID       uuid.UUID
    Type         string    // person, location, date, organization, event, weapon, vehicle
    Value        string    // "Colonel Jean-Pierre Bemba"
    Normalized   string    // normalized form for dedup ("jean-pierre bemba")
    Mentions     []Mention
    Confidence   float64
    CreatedAt    time.Time
}

type Mention struct {
    Text       string  // exact text mention
    Context    string  // surrounding sentence
    Position   int     // character offset in source text
    PageNumber *int    // for documents
    Timestamp  *float64 // for transcripts (seconds)
}

type EntityGraph struct {
    Nodes []EntityNode
    Edges []EntityEdge
}

type EntityNode struct {
    ID        string
    Label     string
    Type      string
    MentionCount int
    EvidenceIDs []uuid.UUID
}

type EntityEdge struct {
    Source    string  // entity ID
    Target   string  // entity ID
    Relation string  // co-occurrence, mentioned_together, same_document
    Weight   int     // number of co-occurrences
}
```

**Extraction flow:**
1. Get text from evidence (direct text, transcript, OCR result, or translation)
2. Send text to Ollama with NER prompt
3. Parse extracted entities with types and positions
4. Normalize entities for deduplication (lowercase, trim, alias matching)
5. Store in `entities` table
6. Build co-occurrence edges (entities appearing in same evidence item)
7. Custody log: `action: "entities_extracted"`

**NER prompt strategy:**
- System prompt: "Extract named entities from this evidence text. Return JSON array with type, value, and context for each entity."
- Entity types: person, location, date, organization, event, weapon, vehicle
- Post-processing: validate JSON, filter low-confidence entities, merge duplicates

**Entity deduplication:**
- Normalize: lowercase, strip titles (Mr., Col., Dr.)
- Alias detection: "Jean-Pierre Bemba" = "Bemba" = "J.P. Bemba" = "Colonel Bemba"
- Merge threshold: configurable (default: 0.8 similarity)
- Admin can manually merge/split entities

**New tables:**
```sql
CREATE TABLE entities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id         UUID REFERENCES cases(id) NOT NULL,
    type            TEXT NOT NULL,
    value           TEXT NOT NULL,
    normalized      TEXT NOT NULL,
    mention_count   INT DEFAULT 0,
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    UNIQUE(case_id, normalized, type)
);

CREATE TABLE entity_mentions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id       UUID REFERENCES entities(id) NOT NULL,
    evidence_id     UUID REFERENCES evidence_items(id) NOT NULL,
    mention_text    TEXT NOT NULL,
    context         TEXT,
    position        INT,
    page_number     INT,
    timestamp_sec   FLOAT,
    confidence      FLOAT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_entities_case ON entities(case_id);
CREATE INDEX idx_entities_type ON entities(case_id, type);
CREATE INDEX idx_entities_normalized ON entities(normalized);
CREATE INDEX idx_entity_mentions_entity ON entity_mentions(entity_id);
CREATE INDEX idx_entity_mentions_evidence ON entity_mentions(evidence_id);
```

**Tests:**
- Text with known entities → all extracted correctly
- Entity types classified correctly
- Positions/page numbers accurate
- Duplicate entities merged (normalized matching)
- Alias detection ("Colonel X" = "X")
- Co-occurrence graph built correctly
- Role-based filtering (defence → entities from disclosed evidence only)
- Empty text → no entities (not an error)
- Large text (100 pages) → processed within timeout
- Multiple languages → entities extracted from each

### Step 2: Entity Search

**Endpoint:** `GET /api/cases/:id/entities`

**Query params:**
- `type` — filter by entity type
- `q` — search entity values
- `evidence_id` — entities from specific evidence
- `from` / `to` — date entities within range

**Response:**
```json
{
    "data": {
        "entities": [
            {
                "id": "uuid",
                "type": "person",
                "value": "Colonel Jean-Pierre Bemba",
                "mention_count": 47,
                "evidence_count": 12,
                "first_seen": "2024-03-15",
                "last_seen": "2024-08-22"
            }
        ]
    }
}
```

**The key query:** "Show me every piece of evidence that mentions Colonel X in Bunia between March and June 2024"
```
GET /api/cases/:id/entities?q=Colonel+X&type=person
→ returns entity with evidence_ids
→ cross-reference with location entity "Bunia" 
→ filter by date range
```

### Step 3: Knowledge Graph Frontend

**Components:**
- `EntityGraph` — Interactive graph visualization
  - Nodes: entities (color-coded by type)
  - Edges: co-occurrence relationships
  - Node size: proportional to mention count
  - Click node → show entity details + linked evidence
  - Zoom, pan, drag nodes
  - Filter by entity type
  - Library: D3.js force-directed graph or vis-network
- `EntityList` — Searchable table of entities
  - Type, value, mention count, evidence count
  - Click → navigate to evidence items mentioning this entity
- `EntityDetail` — Entity detail panel
  - All mentions with context
  - Linked evidence items
  - Timeline of mentions
  - Co-occurring entities
- `EntityMerge` — Admin tool to merge duplicate entities
  - Select entities to merge
  - Preview merged result
  - Confirm → all mentions reassigned

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `internal/ai/entities.go` | Create | Entity extraction service |
| `internal/ai/entity_graph.go` | Create | Graph building + querying |
| `internal/ai/dedup.go` | Create | Entity deduplication + alias matching |
| `migrations/012_entities.up.sql` | Create | Entities + mentions tables |
| `web/src/components/entities/*` | Create | Graph, list, detail, merge UI |

---

## Definition of Done

- [ ] Entities extracted from text (transcript, OCR, direct text)
- [ ] Entity types classified (person, location, date, org, etc.)
- [ ] Deduplication merges "Colonel X" / "X" / "Col. X"
- [ ] Knowledge graph displays entity relationships
- [ ] Cross-entity queries work ("person X at location Y in date range")
- [ ] Entities linked to source evidence with position/page/timestamp
- [ ] Admin can manually merge/split entities
- [ ] Role-based access respected
- [ ] 100% test coverage

---

## Security Checklist

- [ ] Entity data subject to same case role access controls
- [ ] Defence users only see entities from disclosed evidence
- [ ] Entity search doesn't leak information about undisclosed evidence
- [ ] LLM prompt doesn't include system-internal data
- [ ] Entity merge audit logged

---

## Test Coverage Requirements (100% Target)

Every line of code introduced in Sprint 15 must be covered by automated tests. CI blocks merge if coverage drops below 100% for new code.

### Unit Tests

- **`internal/ai/entities.go`** — `QueueExtraction`: valid evidence ID queues job; missing evidence returns error; already-extracted evidence is idempotent
- **`internal/ai/entities.go`** — `GetEntities`: returns correct entities for evidence ID; empty text yields empty slice (not error); filters by entity type
- **`internal/ai/entities.go`** — `SearchEntities`: keyword match, type filter, date range filter, combined filters, empty query returns all
- **`internal/ai/entity_graph.go`** — `GetEntityGraph`: nodes created per entity; edges created for co-occurring entities; edge weight increments on repeated co-occurrence; single-entity evidence produces node with no edges
- **`internal/ai/dedup.go`** — normalization: lowercase, title stripping ("Colonel X" -> "x"), whitespace collapse
- **`internal/ai/dedup.go`** — alias detection: "Jean-Pierre Bemba" matches "Bemba", "J.P. Bemba", "Colonel Bemba"; configurable similarity threshold; below-threshold pairs not merged
- **`internal/ai/dedup.go`** — merge operation: all mentions reassigned to surviving entity; old entity deleted; mention count recalculated
- **NER prompt parsing** — valid JSON response parsed correctly; malformed JSON returns error; low-confidence entities filtered out; unknown entity type rejected
- **Entity types** — each of person, location, date, organization, event, weapon, vehicle classified correctly from known test inputs
- **Mention extraction** — character offset accurate; page number set for documents; timestamp set for transcripts; context window captures surrounding sentence
- **Role-based filtering** — defence user query excludes entities from undisclosed evidence; prosecutor sees all entities; system admin sees all entities
- **Migration `012_entities.up.sql`** — tables created; indexes created; unique constraint on (case_id, normalized, type) enforced; foreign keys enforced

### Integration Tests (testcontainers)

- **Postgres + Ollama end-to-end extraction** — upload evidence text, trigger extraction, verify entities persisted in database with correct types and positions
- **Entity search across evidence items** — create 5 evidence items with overlapping entities, verify search returns correct aggregated results
- **Co-occurrence graph building** — insert entities across multiple evidence items, verify edges built with correct weights
- **Deduplication across extraction runs** — extract from two evidence items mentioning same person differently, verify single merged entity
- **Cross-entity query** — "person X at location Y in date range Z" returns correct evidence subset
- **Custody log integration** — extraction creates custody log entry with action "entities_extracted" and correct evidence ID
- **Large text handling** — 100-page document extracts entities within configured timeout
- **Multilingual extraction** — French and Arabic text produce correctly typed entities

### E2E Automated Tests (Playwright)

- **Knowledge graph renders** — navigate to case, open entity graph tab, verify SVG/canvas renders with at least one node
- **Graph interaction** — click entity node, verify detail panel opens with entity value, type, and mention count
- **Entity list search** — type entity name in search box, verify filtered results appear
- **Entity type filter** — select "person" filter on graph, verify only person nodes visible
- **Entity merge flow** — as admin, select two duplicate entities, click merge, confirm, verify single entity remains with combined mention count
- **Cross-entity query** — enter query "Colonel X in Bunia between March and June 2024", verify results show matching evidence items
- **Defence role restriction** — log in as defence user, verify entities from undisclosed evidence are not visible in graph or list

---

## Manual E2E Testing Checklist

1. [ ] **Action:** Upload a 10-page witness statement PDF mentioning 5+ named individuals, 3+ locations, and 2+ organizations
   **Expected:** Entity extraction job completes within 60 seconds; all named individuals, locations, and organizations appear in the entity list
   **Verify:** Check entity list count matches expected entities; verify each entity has correct type classification

2. [ ] **Action:** Navigate to the Knowledge Graph tab for the case
   **Expected:** Interactive graph renders with color-coded nodes (persons, locations, organizations) and edges between co-occurring entities
   **Verify:** Node count matches entity count; edges exist between entities that appear in the same evidence item; node sizes are proportional to mention count

3. [ ] **Action:** Click on a person entity node in the graph
   **Expected:** Detail panel opens showing entity value, type, mention count, all mentions with surrounding context, and linked evidence items
   **Verify:** Each mention links to the correct evidence item; context snippets are readable and accurate

4. [ ] **Action:** Search for an entity by partial name (e.g., "Bemba") in the entity search bar
   **Expected:** All matching entities appear (including aliases like "Colonel Bemba", "J.P. Bemba")
   **Verify:** Search results include all known aliases; clicking a result navigates to the entity detail

5. [ ] **Action:** As System Admin, select two duplicate entities (e.g., "Colonel Bemba" and "Bemba") and initiate merge
   **Expected:** Merge preview shows combined mention count and affected evidence items; after confirmation, only one entity remains
   **Verify:** All mentions from both entities are now under the surviving entity; the custody log records the merge action with both entity IDs

6. [ ] **Action:** Execute a cross-entity query: "Show evidence mentioning [Person X] at [Location Y] between [Date A] and [Date B]"
   **Expected:** Only evidence items containing both the person and location entities within the date range are returned
   **Verify:** Each result links to evidence that genuinely mentions both entities; no false positives from unrelated evidence

7. [ ] **Action:** Upload evidence in French containing named entities, then upload evidence in Arabic containing named entities
   **Expected:** Entities extracted from both languages with correct type classification
   **Verify:** French person names appear as type "person"; Arabic location names appear as type "location"; no garbled text in entity values

8. [ ] **Action:** Log in as a Defence user assigned to the case
   **Expected:** Entity list and graph only show entities derived from disclosed evidence; entities from undisclosed evidence are invisible
   **Verify:** Compare entity count visible to defence user vs prosecutor; defence count is strictly less than or equal to prosecutor count; no undisclosed entity values leak in any UI element

9. [ ] **Action:** Upload a large document (100+ pages) and trigger entity extraction
   **Expected:** Extraction completes within the configured timeout; progress is visible in the UI
   **Verify:** All expected entities from the document are extracted; no timeout error; custody log records successful extraction

10. [ ] **Action:** Upload two evidence items that mention the same person with different name variants (e.g., "Dr. Jean-Pierre Bemba" and "Bemba, J.P.")
    **Expected:** Automatic deduplication merges both into a single entity with combined mention count
    **Verify:** Entity list shows one entity, not two; mention count reflects mentions from both evidence items; both evidence items are linked to the entity
