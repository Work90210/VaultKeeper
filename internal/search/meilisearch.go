package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ms "github.com/meilisearch/meilisearch-go"
)

const evidenceIndex = "evidence"

// Document represents a searchable document.
type Document struct {
	ID      string
	Index   string
	Payload map[string]any
}

// SearchResult represents a single search hit.
type SearchResult struct {
	ID      string
	Index   string
	Score   float64
	Payload map[string]any
}

// SearchIndexer indexes documents for full-text search.
type SearchIndexer interface {
	IndexDocument(ctx context.Context, document Document) error
	DeleteDocument(ctx context.Context, index string, id string) error
}

// SearchQuerier queries indexed documents.
type SearchQuerier interface {
	Search(ctx context.Context, index string, query string, limit int) ([]SearchResult, error)
}

// MeilisearchClient wraps the Meilisearch SDK.
type MeilisearchClient struct {
	client ms.ServiceManager
}

// NewMeilisearchClient creates a new Meilisearch client.
func NewMeilisearchClient(url, apiKey string) *MeilisearchClient {
	client := ms.New(url, ms.WithAPIKey(apiKey))
	return &MeilisearchClient{client: client}
}

func (c *MeilisearchClient) IndexDocument(_ context.Context, doc Document) error {
	index := c.client.Index(doc.Index)

	payload := make(map[string]any, len(doc.Payload)+1)
	for k, v := range doc.Payload {
		payload[k] = v
	}
	payload["id"] = doc.ID

	pk := "id"
	_, err := index.AddDocuments([]map[string]any{payload}, &ms.DocumentOptions{PrimaryKey: &pk})
	if err != nil {
		return fmt.Errorf("index document %s in %s: %w", doc.ID, doc.Index, err)
	}
	return nil
}

func (c *MeilisearchClient) DeleteDocument(_ context.Context, index string, id string) error {
	idx := c.client.Index(index)
	_, err := idx.DeleteDocument(id, nil)
	if err != nil {
		return fmt.Errorf("delete document %s from %s: %w", id, index, err)
	}
	return nil
}

func (c *MeilisearchClient) Search(_ context.Context, index string, query string, limit int) ([]SearchResult, error) {
	idx := c.client.Index(index)

	resp, err := idx.Search(query, &ms.SearchRequest{
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", index, err)
	}

	results := make([]SearchResult, 0, resp.Hits.Len())
	for _, hit := range resp.Hits {
		// Decode Hit (map[string]json.RawMessage) into a generic map
		m := make(map[string]any)
		for k, raw := range hit {
			var v any
			if err := json.Unmarshal(raw, &v); err == nil {
				m[k] = v
			}
		}
		id, _ := m["id"].(string)
		results = append(results, SearchResult{
			ID:      id,
			Index:   index,
			Payload: m,
		})
	}
	return results, nil
}

// EvidenceSearcher provides evidence-specific search operations.
type EvidenceSearcher interface {
	IndexEvidence(ctx context.Context, doc EvidenceSearchDoc) error
	RemoveEvidence(ctx context.Context, id string) error
	SearchEvidence(ctx context.Context, query SearchQuery) (EvidenceSearchResult, error)
	ConfigureEvidenceIndex(ctx context.Context) error
	ReindexAll(ctx context.Context, caseID string, items []EvidenceSearchDoc) error
}

// IndexEvidence indexes an evidence document into the evidence index.
func (c *MeilisearchClient) IndexEvidence(_ context.Context, doc EvidenceSearchDoc) error {
	index := c.client.Index(evidenceIndex)

	payload := map[string]any{
		"id":              doc.ID,
		"case_id":         doc.CaseID,
		"title":           doc.Title,
		"description":     doc.Description,
		"evidence_number": doc.EvidenceNumber,
		"tags":            doc.Tags,
		"source":          doc.Source,
		"file_name":       doc.FileName,
		"mime_type":       doc.MimeType,
		"classification":  doc.Classification,
		"source_date":     doc.SourceDate,
		"uploaded_at":     doc.UploadedAt,
		"is_current":      doc.IsCurrent,
		"is_disclosed":    doc.IsDisclosed,
	}

	pk := "id"
	_, err := index.AddDocuments([]map[string]any{payload}, &ms.DocumentOptions{PrimaryKey: &pk})
	if err != nil {
		return fmt.Errorf("index evidence %s: %w", doc.ID, err)
	}
	return nil
}

// RemoveEvidence removes an evidence document from the evidence index.
func (c *MeilisearchClient) RemoveEvidence(_ context.Context, id string) error {
	index := c.client.Index(evidenceIndex)
	_, err := index.DeleteDocument(id, nil)
	if err != nil {
		return fmt.Errorf("remove evidence %s: %w", id, err)
	}
	return nil
}

// SearchEvidence performs a filtered, faceted search over indexed evidence.
func (c *MeilisearchClient) SearchEvidence(_ context.Context, query SearchQuery) (EvidenceSearchResult, error) {
	index := c.client.Index(evidenceIndex)

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	filter := buildEvidenceFilter(query)

	req := &ms.SearchRequest{
		Limit:  int64(limit),
		Offset: int64(query.Offset),
		AttributesToHighlight: []string{"title", "description", "evidence_number"},
		Facets:                []string{"mime_type", "classification", "tags"},
	}
	if filter != "" {
		req.Filter = filter
	}

	resp, err := index.Search(query.Query, req)
	if err != nil {
		return EvidenceSearchResult{}, fmt.Errorf("search evidence: %w", err)
	}

	hits := make([]EvidenceSearchHit, 0, resp.Hits.Len())
	for _, hit := range resp.Hits {
		hits = append(hits, parseEvidenceHit(hit))
	}

	facets := parseFacetDistribution(resp.FacetDistribution)

	return EvidenceSearchResult{
		Hits:             hits,
		TotalHits:        int(resp.EstimatedTotalHits),
		Query:            query.Query,
		ProcessingTimeMs: int(resp.ProcessingTimeMs),
		Facets:           facets,
	}, nil
}

// ReindexAll deletes all documents for the given case and re-indexes the provided items.
func (c *MeilisearchClient) ReindexAll(_ context.Context, caseID string, items []EvidenceSearchDoc) error {
	index := c.client.Index(evidenceIndex)

	filter := fmt.Sprintf("case_id = '%s'", escapeFilterValue(caseID))
	if _, err := index.DeleteDocumentsByFilter(filter, nil); err != nil {
		return fmt.Errorf("delete documents for case %s: %w", caseID, err)
	}

	if len(items) == 0 {
		return nil
	}

	docs := make([]map[string]any, 0, len(items))
	for _, doc := range items {
		docs = append(docs, map[string]any{
			"id":              doc.ID,
			"case_id":         doc.CaseID,
			"title":           doc.Title,
			"description":     doc.Description,
			"evidence_number": doc.EvidenceNumber,
			"tags":            doc.Tags,
			"source":          doc.Source,
			"file_name":       doc.FileName,
			"mime_type":       doc.MimeType,
			"classification":  doc.Classification,
			"source_date":     doc.SourceDate,
			"uploaded_at":     doc.UploadedAt,
			"is_current":      doc.IsCurrent,
			"is_disclosed":    doc.IsDisclosed,
		})
	}

	pk := "id"
	if _, err := index.AddDocuments(docs, &ms.DocumentOptions{PrimaryKey: &pk}); err != nil {
		return fmt.Errorf("reindex %d documents for case %s: %w", len(docs), caseID, err)
	}

	return nil
}

// ConfigureEvidenceIndex sets up searchable, filterable, and sortable attributes
// along with typo tolerance for the evidence index.
func (c *MeilisearchClient) ConfigureEvidenceIndex(_ context.Context) error {
	index := c.client.Index(evidenceIndex)

	searchable := []string{"title", "description", "tags", "evidence_number", "source", "file_name"}
	if _, err := index.UpdateSearchableAttributes(&searchable); err != nil {
		return fmt.Errorf("configure evidence searchable attributes: %w", err)
	}

	filterable := []interface{}{"case_id", "mime_type", "classification", "tags", "source_date", "uploaded_at", "is_current", "is_disclosed"}
	if _, err := index.UpdateFilterableAttributes(&filterable); err != nil {
		return fmt.Errorf("configure evidence filterable attributes: %w", err)
	}

	sortable := []string{"uploaded_at", "source_date", "evidence_number"}
	if _, err := index.UpdateSortableAttributes(&sortable); err != nil {
		return fmt.Errorf("configure evidence sortable attributes: %w", err)
	}

	typo := ms.TypoTolerance{Enabled: true}
	if _, err := index.UpdateTypoTolerance(&typo); err != nil {
		return fmt.Errorf("configure evidence typo tolerance: %w", err)
	}

	return nil
}

// escapeFilterValue escapes single quotes in a filter value to prevent Meilisearch filter injection.
func escapeFilterValue(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

// buildEvidenceFilter constructs a Meilisearch filter string from the query parameters.
func buildEvidenceFilter(q SearchQuery) string {
	var parts []string

	if q.CaseID != nil && *q.CaseID != "" {
		parts = append(parts, fmt.Sprintf("case_id = '%s'", escapeFilterValue(*q.CaseID)))
	}

	if len(q.UserCaseIDs) > 0 {
		quoted := make([]string, len(q.UserCaseIDs))
		for i, id := range q.UserCaseIDs {
			quoted[i] = fmt.Sprintf("'%s'", escapeFilterValue(id))
		}
		parts = append(parts, fmt.Sprintf("case_id IN [%s]", strings.Join(quoted, ", ")))
	}

	if len(q.MimeTypes) > 0 {
		quoted := make([]string, len(q.MimeTypes))
		for i, mt := range q.MimeTypes {
			quoted[i] = fmt.Sprintf("'%s'", escapeFilterValue(mt))
		}
		parts = append(parts, fmt.Sprintf("mime_type IN [%s]", strings.Join(quoted, ", ")))
	}

	if len(q.Classifications) > 0 {
		quoted := make([]string, len(q.Classifications))
		for i, cl := range q.Classifications {
			quoted[i] = fmt.Sprintf("'%s'", escapeFilterValue(cl))
		}
		parts = append(parts, fmt.Sprintf("classification IN [%s]", strings.Join(quoted, ", ")))
	}

	if len(q.Tags) > 0 {
		tagFilters := make([]string, len(q.Tags))
		for i, tag := range q.Tags {
			tagFilters[i] = fmt.Sprintf("tags = '%s'", escapeFilterValue(tag))
		}
		parts = append(parts, fmt.Sprintf("(%s)", strings.Join(tagFilters, " OR ")))
	}

	if q.DisclosedOnly {
		parts = append(parts, "is_disclosed = true")
	}

	if q.DateFrom != nil && *q.DateFrom != "" {
		parts = append(parts, fmt.Sprintf("source_date >= '%s'", escapeFilterValue(*q.DateFrom)))
	}

	if q.DateTo != nil && *q.DateTo != "" {
		parts = append(parts, fmt.Sprintf("source_date <= '%s'", escapeFilterValue(*q.DateTo)))
	}

	return strings.Join(parts, " AND ")
}

// parseEvidenceHit converts a raw Meilisearch hit into an EvidenceSearchHit.
func parseEvidenceHit(hit map[string]json.RawMessage) EvidenceSearchHit {
	getString := func(key string) string {
		raw, ok := hit[key]
		if !ok {
			return ""
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ""
		}
		return s
	}

	highlights := make(map[string][]string)
	if raw, ok := hit["_formatted"]; ok {
		var formatted map[string]json.RawMessage
		if err := json.Unmarshal(raw, &formatted); err == nil {
			for _, field := range []string{"title", "description", "evidence_number"} {
				if fraw, ok := formatted[field]; ok {
					var val string
					if err := json.Unmarshal(fraw, &val); err == nil && val != "" {
						highlights[field] = []string{val}
					}
				}
			}
		}
	}

	var score float64
	if raw, ok := hit["_rankingScore"]; ok {
		_ = json.Unmarshal(raw, &score)
	}

	return EvidenceSearchHit{
		EvidenceID:     getString("id"),
		CaseID:         getString("case_id"),
		Title:          getString("title"),
		Description:    getString("description"),
		EvidenceNumber: getString("evidence_number"),
		Highlights:     highlights,
		Score:          score,
	}
}

// parseFacetDistribution converts the Meilisearch facet distribution into a typed map.
func parseFacetDistribution(fd interface{}) map[string]map[string]int {
	if fd == nil {
		return nil
	}

	outer, ok := fd.(map[string]interface{})
	if !ok {
		return nil
	}

	result := make(map[string]map[string]int, len(outer))
	for facet, inner := range outer {
		innerMap, ok := inner.(map[string]interface{})
		if !ok {
			continue
		}
		counts := make(map[string]int, len(innerMap))
		for key, val := range innerMap {
			switch v := val.(type) {
			case float64:
				counts[key] = int(v)
			case int:
				counts[key] = v
			}
		}
		result[facet] = counts
	}
	return result
}

// NoopSearchIndexer silently discards index operations.
type NoopSearchIndexer struct{}

func (n *NoopSearchIndexer) IndexDocument(_ context.Context, _ Document) error { return nil }
func (n *NoopSearchIndexer) DeleteDocument(_ context.Context, _ string, _ string) error {
	return nil
}

// NoopEvidenceSearcher silently discards evidence search operations.
type NoopEvidenceSearcher struct{}

func (n *NoopEvidenceSearcher) IndexEvidence(_ context.Context, _ EvidenceSearchDoc) error {
	return nil
}

func (n *NoopEvidenceSearcher) RemoveEvidence(_ context.Context, _ string) error {
	return nil
}

func (n *NoopEvidenceSearcher) SearchEvidence(_ context.Context, _ SearchQuery) (EvidenceSearchResult, error) {
	return EvidenceSearchResult{}, nil
}

func (n *NoopEvidenceSearcher) ConfigureEvidenceIndex(_ context.Context) error {
	return nil
}

func (n *NoopEvidenceSearcher) ReindexAll(_ context.Context, _ string, _ []EvidenceSearchDoc) error {
	return nil
}
