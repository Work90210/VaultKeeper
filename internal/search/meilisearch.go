package search

import (
	"context"
	"encoding/json"
	"fmt"

	ms "github.com/meilisearch/meilisearch-go"
)

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

// NoopSearchIndexer silently discards index operations.
type NoopSearchIndexer struct{}

func (n *NoopSearchIndexer) IndexDocument(_ context.Context, _ Document) error { return nil }
func (n *NoopSearchIndexer) DeleteDocument(_ context.Context, _ string, _ string) error {
	return nil
}
