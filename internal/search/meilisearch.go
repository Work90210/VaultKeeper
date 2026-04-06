package search

import "context"

type Document struct {
	ID      string
	Index   string
	Payload map[string]any
}

type SearchResult struct {
	ID      string
	Index   string
	Score   float64
	Payload map[string]any
}

type SearchIndexer interface {
	IndexDocument(ctx context.Context, document Document) error
}

type SearchQuerier interface {
	Search(ctx context.Context, index string, query string, limit int) ([]SearchResult, error)
}
